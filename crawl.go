package main

import (
	"log"
	"os"
	"path"
	"regexp"
	//"strconv"
	"io/ioutil"
	"sync"
	//"gotk3/gtk"
	//"fmt"
)

type FileEntry struct {
	path    string
	name    string
	modtime int64
	size    int64
}

type FilesChannel struct {
	byname    chan SortedByName
	bymodtime chan SortedByModTime
	bysize    chan SortedBySize
}

type ResultChannel struct {
	byname    chan CrawlResult
	bymodtime chan CrawlResult
	bysize    chan CrawlResult
}

type ResultMemory struct {
	byname    CrawlResult
	bymodtime CrawlResult
	bysize    CrawlResult
}

type CrawlResult interface {
	Merge(files []*FileEntry)
	Len() int
}

type FileEntries []*FileEntry

func NewFileEntries() *FileEntries {
	return new(FileEntries)
}

func (entries *FileEntries) Merge(files []*FileEntry) {
	*entries = FileEntries(append(*entries, files...))
}

func (entries *FileEntries) Len() int {
	return len(*entries)
}

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
				wg.Add(1)
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
					modtime: fileinfo.ModTime().Unix(),
					size:    fileinfo.Size(),
				}
				files = append(files, entry)
			}
		}

		if len(files) > 0 {
			go func() {
				byname := make([]*FileEntry, len(files))
				copy(byname, files)
				collect.byname <- sortFileEntries(SortedByName(byname)).(SortedByName)
			}()
			go func() {
				bymodtime := make([]*FileEntry, len(files))
				copy(bymodtime, files)
				collect.bymodtime <- sortFileEntries(SortedByModTime(bymodtime)).(SortedByModTime)
			}()
			go func() {
				bysize := make([]*FileEntry, len(files))
				copy(bysize, files)
				collect.bysize <- sortFileEntries(SortedBySize(bysize)).(SortedBySize)
			}()
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

func Crawl(cores int, mem ResultMemory, display ResultChannel, finish chan struct{}, directories []string, query *regexp.Regexp) {

	var wg sync.WaitGroup

	newdirs := make(chan string)
	collect := FilesChannel{make(chan SortedByName), make(chan SortedByModTime), make(chan SortedBySize)}
	maxproc := make(chan struct{}, cores*2)
	go func() {
		for {
			select {
			case dir := <-newdirs:
				maxproc <- struct{}{}
				go visit(&wg, maxproc, newdirs, collect, dir, query)
			case <-finish:
				return
			}
		}

	}()

	go func() {
		for {
			select {
			case newbyname := <-collect.byname:
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
			case newbymodtime := <-collect.bymodtime:
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
			case newbysize := <-collect.bysize:
				mem.bysize.Merge(newbysize)
				display.bysize <- mem.bysize
				wg.Done()
			case <-finish:
				return
			}
		}
	}()

	for _, dir := range directories {
		wg.Add(1)
		newdirs <- dir
	}

	wg.Wait()
	close(finish)

}
