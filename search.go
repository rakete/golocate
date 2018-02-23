package main

import (
	"log"
	"os"
	"path"
	"regexp"
	//"strconv"
	"sync"
	//"time"
	"io/ioutil"
	"syscall"

	"gotk3/gtk"
)

type FileEntry struct {
	path     string
	fileinfo os.FileInfo
	urgency  int
}

func visit(wg *sync.WaitGroup, maxproc chan struct{}, dirchan chan string, collect chan []FileEntry, dir string, query *regexp.Regexp) {
	entries, err := ioutil.ReadDir(dir)
	<-maxproc

	if err != nil {
		log.Println("Could not read directory:", err)
	} else {
		var matches []string
		for _, entry := range entries {
			entrypath := path.Join(dir, entry.Name())
			if entry.IsDir() {
				dirchan <- entrypath
			} else if query.MatchString(entrypath) {
				matches = append(matches, entrypath)
			}
		}

		var files []FileEntry
		for _, entrypath := range matches {
			if fileinfo, err := os.Lstat(entrypath); err != nil {
				log.Println("Could not read file:", err)
			} else {
				files = append(files, FileEntry{path: entrypath, fileinfo: fileinfo, urgency: 0})
			}
		}
		collect <- files
	}

	defer wg.Done()
}

func Search(liststore *gtk.ListStore) {
	log.Println("start search")

	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		log.Println("Error Getting Rlimit ", err)
	}

	userDirectories := []string{os.Getenv("HOME"), "/usr"}

	query, err := regexp.Compile(".*")
	if err != nil {
		log.Fatal("Error compiling regexp:", err)
	}
	var wg sync.WaitGroup

	dirchan := make(chan string)
	collect := make(chan []FileEntry)
	finish := make(chan struct{})
	maxproc := make(chan struct{}, rLimit.Cur/2)
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

	var results []FileEntry
	go func() {
		for {
			select {
			case newEntries := <-collect:
				results = append(results, newEntries...)
			case <-finish:
				log.Println(len(results))
				return
			}
		}
	}()

	for _, dir := range userDirectories {
		dirchan <- dir
	}

	wg.Wait()
	close(finish)
}
