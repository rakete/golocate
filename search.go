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
	//"gotk3/gtk"
	"runtime"
)

const (
	SORT_BY_NAME = iota
	SORT_BY_MODTIME
	SORT_BY_SIZE
)

type FileEntry struct {
	path     string
	fileinfo os.FileInfo
}

type ResultChannel struct {
	byname    chan ByName
	bymodtime chan ByModTime
	bysize    chan BySize
}

type ByName []FileEntry

func (a ByName) Len() int      { return len(a) }
func (a ByName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool {
	return a[i].fileinfo.Name() < a[j].fileinfo.Name()
}

type ByModTime []FileEntry

func (a ByModTime) Len() int      { return len(a) }
func (a ByModTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByModTime) Less(i, j int) bool {
	return a[i].fileinfo.ModTime().Before(a[j].fileinfo.ModTime())
}

type BySize []FileEntry

func (a BySize) Len() int      { return len(a) }
func (a BySize) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BySize) Less(i, j int) bool {
	return a[i].fileinfo.Size() < a[j].fileinfo.Size()
}

func sortFileEntries(files sort.Interface) sort.Interface {
	sort.Stable(files)
	return files
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
				files = append(files, FileEntry{path: dir, fileinfo: fileinfo})
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

func merge(sorttype int, left, right []FileEntry) []FileEntry {
	if len(left) == 0 {
		return right
	}

	if len(right) == 0 {
		return left
	}

	type LessFunc func([]FileEntry, int, []FileEntry, int) bool
	var less LessFunc

	switch sorttype {
	case SORT_BY_NAME:
		less = func(a []FileEntry, i int, b []FileEntry, j int) bool {
			return a[i].fileinfo.Name() < b[j].fileinfo.Name()
		}
	case SORT_BY_MODTIME:
		less = func(a []FileEntry, i int, b []FileEntry, j int) bool {
			return a[i].fileinfo.ModTime().Before(b[j].fileinfo.ModTime())
		}
	case SORT_BY_SIZE:
		less = func(a []FileEntry, i int, b []FileEntry, j int) bool {
			return a[i].fileinfo.Size() < b[j].fileinfo.Size()
		}
	}

	var result []FileEntry
	if less(right, len(right)-1, left, 0) {
		result = append(right, left...)
	} else if less(left, len(left)-1, right, 0) {
		result = append(left, right...)
	} else {
		// i, j := 0, 0
		// for i < len(left) && j < len(right) {
		// 	if less(left, i, right, j) {
		// 		result = append(result, left[i])
		// 		i += 1
		// 	} else {
		// 		result = append(result, right[j])
		// 		j += 1
		// 	}
		// }

		// if i < len(left) {
		// 	result = append(result, left[i:]...)
		// }
		// if j < len(right) {
		// 	result = append(result, right[j:]...)
		// }
		result = append(left, right...)
		switch sorttype {
		case SORT_BY_NAME:
			sort.Stable(ByName(result))
		case SORT_BY_MODTIME:
			sort.Stable(ByModTime(result))
		case SORT_BY_SIZE:
			sort.Stable(BySize(result))
		}
	}

	return result
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
				resultsbyname = merge(SORT_BY_NAME, resultsbyname, newbyname)
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
				resultsbymodtime = merge(SORT_BY_MODTIME, resultsbymodtime, newbymodtime)
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
				resultsbysize = merge(SORT_BY_SIZE, resultsbysize, newbysize)
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
