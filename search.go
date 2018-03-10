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
	"runtime"
)

type FileEntry struct {
	path    string
	name    string
	modtime time.Time
	size    int64
}

type ResultChannel struct {
	byname    chan ByName
	bymodtime chan ByModTime
	bysize    chan BySize
}

func visit(wg *sync.WaitGroup, maxproc chan struct{}, dirchan chan string, collect ResultChannel, dir string, query *regexp.Regexp) {
	entries, err := ioutil.ReadDir(dir)
	<-maxproc

	if err != nil {
		//log.Println("Could not read directory:", err)
	} else {
		var matches []string
		for _, entry := range entries {
			entrypath := path.Join(dir, entry.Name())
			if entry.IsDir() {
				dirchan <- entrypath
			} else if query == nil || query.MatchString(entrypath) {
				matches = append(matches, entrypath)
			}
		}

		var files []FileEntry
		for _, entrypath := range matches {
			if fileinfo, err := os.Lstat(entrypath); err != nil {
				log.Println("Could not read file:", err)
			} else {
				files = append(files, FileEntry{
					path:    dir,
					name:    fileinfo.Name(),
					modtime: fileinfo.ModTime(),
					size:    fileinfo.Size(),
				})
			}
		}

		wg.Add(1)
		go func() {
			collect.byname <- sortFileEntries(ByName(files)).(ByName)
		}()
		wg.Add(1)
		go func() {
			collect.bymodtime <- sortFileEntries(ByModTime(files)).(ByModTime)
		}()
		wg.Add(1)
		go func() {
			collect.bysize <- sortFileEntries(BySize(files)).(BySize)
		}()
	}

	defer wg.Done()
}

func Search(display ResultChannel, query *regexp.Regexp) {
	cores := runtime.NumCPU()
	log.Println("start search on", cores, "cores")

	userDirectories := []string{os.Getenv("HOME"), "/usr", "/var", "/sys", "/opt", "/etc", "/bin", "/sbin"}

	var wg sync.WaitGroup

	dirchan := make(chan string)
	collect := ResultChannel{make(chan ByName), make(chan ByModTime), make(chan BySize)}
	finish := make(chan struct{})
	maxproc := make(chan struct{}, cores*2)
	go func() {
		for {
			select {
			case dir := <-dirchan:
				maxproc <- struct{}{}
				wg.Add(1)
				go visit(&wg, maxproc, dirchan, collect, dir, query)
			case <-finish:
				return
			}
		}

	}()

	var resultsbyname []FileEntry
	go func() {
		for {
			select {
			case newbyname := <-collect.byname:
				resultsbyname = sortMerge(SORT_BY_NAME, resultsbyname, newbyname)
				display.byname <- resultsbyname
				wg.Done()
			case <-finish:
				log.Println("resultsbyname", len(resultsbyname))
				return
			}
		}
	}()

	var resultsbymodtime []FileEntry
	go func() {
		for {
			select {
			case newbymodtime := <-collect.bymodtime:
				resultsbymodtime = sortMerge(SORT_BY_MODTIME, resultsbymodtime, newbymodtime)
				display.bymodtime <- resultsbymodtime
				wg.Done()
			case <-finish:
				log.Println("resultsbymodtime", len(resultsbymodtime))
				return
			}
		}
	}()

	var resultsbysize []FileEntry
	go func() {
		for {
			select {
			case newbysize := <-collect.bysize:
				resultsbysize = sortMerge(SORT_BY_SIZE, resultsbysize, newbysize)
				display.bysize <- resultsbysize
				wg.Done()
			case <-finish:
				log.Println("resultsbysize:", len(resultsbysize))
				return
			}
		}
	}()

	for _, dir := range userDirectories {
		dirchan <- dir
	}

	wg.Wait()
	close(finish)
	log.Println("close finish in search")

}
