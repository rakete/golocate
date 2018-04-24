package main

import (
	//"log"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"sort"
	"sync"
	"time"

	gtk "github.com/gotk3/gotk3/gtk"
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

func testMatchCaches(dircache Cache, namecache Cache, entry *FileEntry, query *regexp.Regexp) (bool, bool) {
	matchedname, knownname, matcheddir, knowndir := true, true, false, false
	if query != nil {
		matchedname, knownname = namecache.Test(entry.name)
		matcheddir, knowndir = dircache.Test(entry.dir)

		if !matchedname && !matcheddir {
			if !knownname {
				matchedname = query.MatchString(entry.name)
				namecache.Put(entry.name, matchedname)
			}
			if !knowndir && !matchedname {
				matcheddir = query.MatchString(entry.dir)
				dircache.Put(entry.dir, matcheddir)
			}
		}
	}

	return matchedname, matcheddir
}

type CrawlResult interface {
	Merge(sortcolumn SortColumn, files []*FileEntry)
	Take(cache MatchCaches, sortcolumn SortColumn, direction gtk.SortType, query *regexp.Regexp, n int, abort chan struct{}, results chan *FileEntry)
	Remove(sortcolumn SortColumn, files []*FileEntry)
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
			matchedname, matcheddir := testMatchCaches(dircache, namecache, entry, query)

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

func (entries *FileEntries) Remove(sortcolumn SortColumn, files []*FileEntry) {
}

func (entries *FileEntries) NumFiles() int {
	return len(entries.queue) + len(entries.sorted)
}

func visit(wg *sync.WaitGroup, maxproc chan struct{}, newdirs chan string, collect FilesChannel, dir string) {
	infos, err := ioutil.ReadDir(dir)
	<-maxproc

	if err != nil {
		//log.Println("Could not read directory:", err)
	} else {
		wg.Add(4)
		fileentries := make([]*FileEntry, len(infos))
		numfiles := 0

		for _, fileinfo := range infos {
			entrypath := path.Join(dir, fileinfo.Name())
			if fileinfo.IsDir() {
				newdirs <- entrypath
			} else {
				entry := &FileEntry{
					dir:     dir,
					name:    fileinfo.Name(),
					modtime: fileinfo.ModTime(),
					size:    fileinfo.Size(),
				}
				fileentries[numfiles] = entry
				numfiles += 1
			}
		}

		if numfiles > 0 {
			collect.byname <- fileentries[:numfiles]
			collect.bydir <- fileentries[:numfiles]
			collect.bymodtime <- fileentries[:numfiles]
			collect.bysize <- fileentries[:numfiles]
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

type CrawlUpdate struct {
	dirs  []string
	files []*FileEntry
}

type DirAge struct {
	dir string
	age time.Time
}

func Crawler(wg *sync.WaitGroup, mem ResultMemory, config Configuration, newdirs chan string, query chan *regexp.Regexp, updates chan CrawlUpdate, finish chan struct{}) {
	wg.Add(len(config.directories))

	collect := FilesChannel{make(chan SortedByName), make(chan SortedByDir), make(chan SortedByModTime), make(chan SortedBySize)}
	maxproc := make(chan struct{}, config.cores)
	dirages := make(map[string]time.Time)
	var currentquery *regexp.Regexp
	go func() {
		for {
			select {
			case dir := <-newdirs:
				wg.Add(1)
				maxproc <- struct{}{}

				_, ageknown := dirages[dir]
				if !ageknown {
					if dirinfo, err := os.Lstat(dir); err == nil {
						dirages[dir] = dirinfo.ModTime()
					}
				}

				go visit(wg, maxproc, newdirs, collect, dir)
			case currentquery = <-query:
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

				//sort.Stable(SortedByDir(newbydir))
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

	dirnumfiles := make(map[string]int)
	latestmodtime := time.Unix(0, 0)
	go func() {
		for {
			select {
			case files := <-collect.bysize:
				dirnumfiles[files[0].dir] += len(files)
				for _, entry := range files {
					if entry.modtime.After(latestmodtime) {
						latestmodtime = entry.modtime
					}
				}

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

	for _, dir := range config.directories {
		newdirs <- dir
		wg.Done()
	}

	wg.Done()
	wg.Wait()

	for {
		select {
		case <-finish:
			return
		case <-time.After(1000 * time.Millisecond):
			//log.Println("crawl some more")
		}
	}
}
