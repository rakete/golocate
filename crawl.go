package main

import (
	"log"
	"os"
	"path"
	"regexp"
	//"strconv"
	"io/ioutil"
	"sync"
	"time"
	//"gotk3/gtk"
	//"fmt"
	"sort"

	"github.com/gotk3/gotk3/gtk"
)

type FileEntry struct {
	dir     string
	name    string
	modtime time.Time
	size    int64
}

type FilesChannel struct {
	byname    chan SortedByName
	bydir     chan SortedByDir
	bymodtime chan SortedByModTime
	bysize    chan SortedBySize
}

type ResultMemory struct {
	byname    CrawlResult
	bydir     CrawlResult
	bymodtime CrawlResult
	bysize    CrawlResult
}

type Cache interface {
	Test(k string) (bool, bool)
	Put(k string, v bool)
}

type SyncCache sync.Map

func NewSyncCache() *SyncCache {
	return new(SyncCache)
}

func (c *SyncCache) Test(k string) (bool, bool) {
	v, ok := (*sync.Map)(c).Load(k)
	ret, _ := v.(bool)
	return ret, ok
}

func (c *SyncCache) Put(k string, v bool) {
	(*sync.Map)(c).Store(k, v)
}

type SimpleCache map[string]bool

func NewSimpleCache() *SimpleCache {
	empty := SimpleCache(map[string]bool{})
	return &empty
}

func (c *SimpleCache) Test(k string) (bool, bool) {
	v, ok := (*c)[k]
	return v, ok
}

func (c *SimpleCache) Put(k string, v bool) {
	(*c)[k] = v
}

type MatchCaches struct {
	dirs  Cache
	names Cache
}

type CrawlResult interface {
	Merge(sortcolumn SortColumn, files []*FileEntry)
	Take(cache MatchCaches, sortcolumn SortColumn, direction gtk.SortType, query *regexp.Regexp, n int, abort chan struct{}, results chan *FileEntry)
	NumFiles() int
}

type FileEntries struct {
	queue  []*FileEntry
	sorted []*FileEntry
}

func (entries *FileEntries) Merge(_ SortColumn, files []*FileEntry) {
	entries.queue = append(entries.queue, files...)
}

func (entries *FileEntries) Take(cache MatchCaches, sortcolumn SortColumn, direction gtk.SortType, query *regexp.Regexp, n int, abort chan struct{}, results chan *FileEntry) {
	var indexfunc func(int, int) int
	switch direction {
	case gtk.SORT_ASCENDING:
		indexfunc = func(l, i int) int { return i }
	case gtk.SORT_DESCENDING:
		indexfunc = func(l, j int) int { return l - 1 - j }
	}

	switch sortcolumn {
	case SORT_BY_NAME:
		sort.Stable(SortedByName(entries.queue))
	case SORT_BY_MODTIME:
		sort.Stable(SortedByModTime(entries.queue))
	case SORT_BY_SIZE:
		sort.Stable(SortedBySize(entries.queue))
	}
	entries.sorted = sortMerge(sortcolumn, entries.sorted, entries.queue)
	entries.queue = nil

	l := len(entries.sorted)
	if n > l {
		n = l
	}

	numresults := 0
	aborted := false
	var namecache, dircache Cache
	if cache.names != nil {
		namecache = cache.names
	} else {
		empty := SimpleCache(map[string]bool{})
		namecache = &empty
	}

	if cache.dirs != nil {
		dircache = cache.dirs
	} else {
		empty := SimpleCache(map[string]bool{})
		dircache = &empty
	}

sortedloop:
	for i := 0; i < len(entries.sorted); i++ {
		select {
		case <-abort:
			aborted = true
			break sortedloop
		default:

			index := indexfunc(l, i)

			entry := entries.sorted[index]
			entryname := entry.name
			entrydir := entry.dir

			var matchedname, knownname, matcheddir, knowndir bool
			if query != nil {
				matchedname, knownname = namecache.Test(entryname)
				matcheddir, knowndir = dircache.Test(entrydir)

				if !matchedname && !matcheddir {
					if !knownname {
						matchedname = query.MatchString(entryname)
						namecache.Put(entryname, matchedname)
					}
					if !knowndir && !matchedname {
						matcheddir = query.MatchString(entrydir)
						dircache.Put(entrydir, matcheddir)
					}
				}
			}

			if query == nil || matchedname || matcheddir {
				results <- entry
				numresults += 1
			}

			if numresults >= n {
				break sortedloop
			}
		}

	}

	if !aborted {
		results <- nil
	}
}

func (entries *FileEntries) NumFiles() int {
	return len(entries.queue) + len(entries.sorted)
}

func visit(wg *sync.WaitGroup, maxproc chan struct{}, newdirs chan string, collect FilesChannel, dir string) {
	entries, err := ioutil.ReadDir(dir)
	<-maxproc

	if err != nil {
		//log.Println("Could not read directory:", err)
	} else {
		wg.Add(4)

		var matches []string
		for _, entry := range entries {
			entrypath := path.Join(dir, entry.Name())
			if entry.IsDir() {
				newdirs <- entrypath
			} else {
				matches = append(matches, entrypath)
			}
		}

		var files []*FileEntry
		for _, entrypath := range matches {
			if fileinfo, err := os.Lstat(entrypath); err != nil {
				log.Println("Could not read file:", err)
			} else {
				entry := &FileEntry{
					dir:     dir,
					name:    fileinfo.Name(),
					modtime: fileinfo.ModTime(),
					size:    fileinfo.Size(),
				}
				files = append(files, entry)
			}
		}

		if len(files) > 0 {
			collect.byname <- files
			collect.bydir <- files
			collect.bymodtime <- files
			collect.bysize <- files
		} else {
			defer func() {
				wg.Done()
				wg.Done()
				wg.Done()
				wg.Done()
			}()
		}
	}

	defer wg.Done()
}

func Crawler(wg *sync.WaitGroup, cores int, mem ResultMemory, newdirs chan string, finish chan struct{}, directories []string) {
	wg.Add(len(directories))

	collect := FilesChannel{make(chan SortedByName), make(chan SortedByDir), make(chan SortedByModTime), make(chan SortedBySize)}
	maxproc := make(chan struct{}, cores)
	go func() {
		for {
			select {
			case dir := <-newdirs:
				wg.Add(1)
				maxproc <- struct{}{}
				go visit(wg, maxproc, newdirs, collect, dir)
			case <-finish:
				return
			}
		}

	}()

	go func() {
		for {
			select {
			case files := <-collect.byname:
				newbyname := make([]*FileEntry, len(files))
				copy(newbyname, files)

				sort.Stable(SortedByName(newbyname))
				mem.byname.Merge(SORT_BY_NAME, newbyname)

				wg.Done()
			case <-finish:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case files := <-collect.bydir:
				newbydir := make([]*FileEntry, len(files))
				copy(newbydir, files)

				sort.Stable(SortedByDir(newbydir))
				mem.bydir.Merge(SORT_BY_DIR, newbydir)

				wg.Done()
			case <-finish:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case files := <-collect.bymodtime:
				newbymodtime := make([]*FileEntry, len(files))
				copy(newbymodtime, files)

				sort.Stable(SortedByModTime(newbymodtime))
				mem.bymodtime.Merge(SORT_BY_MODTIME, newbymodtime)

				wg.Done()
			case <-finish:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case files := <-collect.bysize:
				newbysize := make([]*FileEntry, len(files))
				copy(newbysize, files)

				sort.Stable(SortedBySize(newbysize))
				mem.bysize.Merge(SORT_BY_SIZE, newbysize)

				wg.Done()
			case <-finish:
				return
			}
		}
	}()

	wg.Done()
	<-finish
}
