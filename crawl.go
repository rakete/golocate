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

	"github.com/gotk3/gotk3/gtk"
)

type FileEntry struct {
	path    string
	name    string
	modtime time.Time
	size    int64
}

type FilesChannel struct {
	byname    chan SortedByName
	bymodtime chan SortedByModTime
	bysize    chan SortedBySize
}

type ResultMemory struct {
	byname    CrawlResult
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
	paths Cache
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
		entries.queue = sortFileEntries(SortedByName(entries.queue)).(SortedByName)
	case SORT_BY_MODTIME:
		entries.queue = sortFileEntries(SortedByModTime(entries.queue)).(SortedByModTime)
	case SORT_BY_SIZE:
		entries.queue = sortFileEntries(SortedBySize(entries.queue)).(SortedBySize)
	}
	entries.sorted = sortMerge(sortcolumn, entries.sorted, entries.queue)
	entries.queue = nil

	l := len(entries.sorted)
	if n > l {
		n = l
	}

	numresults := 0
	aborted := false
	var namecache, pathcache Cache
	if cache.names != nil {
		namecache = cache.names
	} else {
		empty := SimpleCache(map[string]bool{})
		namecache = &empty
	}

	if cache.paths != nil {
		pathcache = cache.paths
	} else {
		empty := SimpleCache(map[string]bool{})
		pathcache = &empty
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
			entrypath := entry.path

			var matchedname, knownname, matchedpath, knownpath bool
			if query != nil {
				matchedname, knownname = namecache.Test(entryname)
				matchedpath, knownpath = pathcache.Test(entrypath)

				if !matchedname && !matchedpath {
					if !knownname {
						matchedname = query.MatchString(entryname)
						namecache.Put(entryname, matchedname)
					}
					if !knownpath && !matchedname {
						matchedpath = query.MatchString(entrypath)
						pathcache.Put(entrypath, matchedpath)
					}
				}
			}

			if query == nil || matchedname || matchedpath {
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
		wg.Add(3)

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
					path:    dir,
					name:    fileinfo.Name(),
					modtime: fileinfo.ModTime(),
					size:    fileinfo.Size(),
				}
				files = append(files, entry)
			}
		}

		if len(files) > 0 {
			collect.byname <- files
			collect.bymodtime <- files
			collect.bysize <- files
		} else {
			defer func() {
				wg.Done()
				wg.Done()
				wg.Done()
			}()
		}
	}

	defer wg.Done()
}

func Crawler(wg *sync.WaitGroup, cores int, mem ResultMemory, newdirs chan string, finish chan struct{}) {
	collect := FilesChannel{make(chan SortedByName), make(chan SortedByModTime), make(chan SortedBySize)}
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

				newbyname = sortFileEntries(SortedByName(newbyname)).(SortedByName)
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
			case files := <-collect.bymodtime:
				newbymodtime := make([]*FileEntry, len(files))
				copy(newbymodtime, files)

				newbymodtime = sortFileEntries(SortedByModTime(newbymodtime)).(SortedByModTime)
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

				newbysize = sortFileEntries(SortedBySize(newbysize)).(SortedBySize)
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
