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

type CrawlResult interface {
	Merge(sortcolumn SortColumn, files []*FileEntry)
	Take(sortcolumn SortColumn, direction gtk.SortType, query *regexp.Regexp, n int) ([]*FileEntry, []Bucket)
	NumFiles() int
}

type FileEntries []*FileEntry

func (entries *FileEntries) Merge(_ SortColumn, files []*FileEntry) {
	*entries = append(*entries, files...)
}

func (entries *FileEntries) Take(sortcolumn SortColumn, direction gtk.SortType, query *regexp.Regexp, n int) ([]*FileEntry, []Bucket) {
	var indexfunc func(int, int) int
	switch direction {
	case gtk.SORT_ASCENDING:
		indexfunc = func(l, i int) int { return i }
	case gtk.SORT_DESCENDING:
		indexfunc = func(l, j int) int { return l - 1 - j }
	}

	var sorted FileEntries
	switch sortcolumn {
	case SORT_BY_NAME:
		sorted = FileEntries(sortFileEntries(SortedByName(*entries)).(SortedByName))
	case SORT_BY_MODTIME:
		sorted = FileEntries(sortFileEntries(SortedByModTime(*entries)).(SortedByModTime))
	case SORT_BY_SIZE:
		sorted = FileEntries(sortFileEntries(SortedBySize(*entries)).(SortedBySize))
	}

	l := len(sorted)
	if n > l {
		n = l
	}

	var result []*FileEntry
	for i := 0; i < len(sorted); i++ {
		index := indexfunc(l, i)
		if query == nil || query.MatchString(sorted[index].name) || query.MatchString(sorted[index].path) {
			result = append(result, sorted[index])
		}

		if len(result) >= n {
			break
		}
	}

	return result, nil
}

func (entries *FileEntries) NumFiles() int { return len(*entries) }

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
