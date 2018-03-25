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

type DisplayChannel struct {
	size      chan int
	direction chan int
	byname    chan CrawlResult
	bymodtime chan CrawlResult
	bysize    chan CrawlResult
}

type ResultMemory struct {
	byname    CrawlResult
	bymodtime CrawlResult
	bysize    CrawlResult
}

const (
	DIRECTION_ASCENDING = iota
	DIRECTION_DESCENDING
)

type CrawlResult interface {
	Merge(files []*FileEntry)
	Take(direction, n int) []*FileEntry
	NumFiles() int
}

type NameEntries []*FileEntry
type ModTimeEntries []*FileEntry
type SizeEntries []*FileEntry

func (entries *NameEntries) Merge(files []*FileEntry) {
	*entries = NameEntries(append(*entries, files...))
}

func (entries *NameEntries) Take(direction, n int) []*FileEntry {
	sorted := sortFileEntries(SortedByName(*entries)).(SortedByName)
	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

func (entries *NameEntries) NumFiles() int { return len(*entries) }

func (entries *ModTimeEntries) Merge(files []*FileEntry) {
	*entries = ModTimeEntries(append(*entries, files...))
}

func (entries *ModTimeEntries) Take(direction, n int) []*FileEntry {
	sorted := sortFileEntries(SortedByModTime(*entries)).(SortedByModTime)
	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

func (entries *ModTimeEntries) NumFiles() int { return len(*entries) }

func (entries *SizeEntries) Merge(files []*FileEntry) {
	*entries = SizeEntries(append(*entries, files...))
}

func (entries *SizeEntries) Take(direction, n int) []*FileEntry {
	sorted := sortFileEntries(SortedBySize(*entries)).(SortedBySize)
	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

func (entries *SizeEntries) NumFiles() int { return len(*entries) }

func visit(wg *sync.WaitGroup, maxproc chan struct{}, newdirs chan string, collect FilesChannel, dir string, query *regexp.Regexp) {
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
			} else if query == nil || query.MatchString(entrypath) {
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

func Crawl(wg *sync.WaitGroup, cores int, mem ResultMemory, display DisplayChannel, newdirs chan string, finish chan struct{}, query *regexp.Regexp) {
	wg.Add(1)

	collect := FilesChannel{make(chan SortedByName), make(chan SortedByModTime), make(chan SortedBySize)}
	maxproc := make(chan struct{}, cores)
	go func() {
		for {
			select {
			case dir := <-newdirs:
				wg.Add(1)
				maxproc <- struct{}{}
				go visit(wg, maxproc, newdirs, collect, dir, query)
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
				mem.byname.Merge(newbyname)

				display.byname <- mem.byname
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
				mem.bymodtime.Merge(newbymodtime)

				display.bymodtime <- mem.bymodtime
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
				mem.bysize.Merge(newbysize)

				display.bysize <- mem.bysize
				wg.Done()
			case <-finish:
				return
			}
		}
	}()

	wg.Done()
	<-finish
}
