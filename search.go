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
	"sort"

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

		collect <- sortFileEntries(sorttype, files)
	}

	defer wg.Done()
}

type SortFunc func([]FileEntry) []FileEntry

type ByName []FileEntry

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].fileinfo.Name() <= a[j].fileinfo.Name() }

type ByModTime []FileEntry

func (a ByModTime) Len() int      { return len(a) }
func (a ByModTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByModTime) Less(i, j int) bool {
	return a[i].fileinfo.ModTime().Before(a[j].fileinfo.ModTime()) || a[i].fileinfo.ModTime().Equal(a[j].fileinfo.ModTime())
}

const (
	SORT_BY_NAME = iota
	SORT_BY_MODTIME
)

func sortFileEntries(sorttype int, files []FileEntry) []FileEntry {
	switch sorttype {
	case SORT_BY_NAME:
		sort.Sort(ByName(files))
	case SORT_BY_MODTIME:
		sort.Sort(ByModTime(files))
	}
	return files
}

func merge(sorttype int, left, right []FileEntry) []FileEntry {
	if len(left) == 0 {
		return right
	}

	if len(right) == 0 {
		return left
	}

	var result []FileEntry

	type LessCompareFunc func([]FileEntry, int, []FileEntry, int) bool
	var lesscomparefunc LessCompareFunc

	switch sorttype {
	case SORT_BY_NAME:
		lesscomparefunc = func(a []FileEntry, i int, b []FileEntry, j int) bool {
			return a[i].fileinfo.Name() <= b[j].fileinfo.Name()
		}
	case SORT_BY_MODTIME:
		lesscomparefunc = func(a []FileEntry, i int, b []FileEntry, j int) bool {
			return a[i].fileinfo.ModTime().Before(b[j].fileinfo.ModTime()) || a[i].fileinfo.ModTime().Equal(b[j].fileinfo.ModTime())
		}
	}

	if lesscomparefunc(left, 0, right, len(right)-1) {
		result = append(left, right...)
	} else if lesscomparefunc(left, len(left)-1, right, 0) {
		result = append(right, left...)
	} else {
		i, j := 0, 0
		for i < len(left) && j < len(right) {
			if lesscomparefunc(left, i, right, j) {
				result = append(result, left[i])
				i += 1
			} else {
				result = append(result, right[j])
				j += 1
			}
		}

		if i < len(left) {
			result = append(result, left[i:]...)
		}
		if j < len(right) {
			result = append(result, right[j:]...)
		}
	}

	return result
}

func Search(liststore *gtk.ListStore) {
	log.Println("start search")

	userDirectories := []string{os.Getenv("HOME"), "/usr", "/sys", "/opt", "/etc", "/bin", "/sbin"}

	sorttype := SORT_BY_MODTIME
	query, err := regexp.Compile(".*")
	if err != nil {
		log.Fatal("Error compiling regexp:", err)
	}
	var wg sync.WaitGroup

	dirchan := make(chan string)
	collect := make(chan []FileEntry)
	finish := make(chan struct{})
	maxproc := make(chan struct{}, 16)
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
			case newentries := <-collect:
				results = merge(sorttype, results, newentries)
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
