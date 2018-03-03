package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"

	"testing"
)

func getDirectoryFiles(dir string) []FileEntry {
	var files []FileEntry
	if entries, err := ioutil.ReadDir(dir); err != nil {
		log.Println("Could not read dir:", err)
	} else {
		for _, entry := range entries {
			entrypath := path.Join(dir, entry.Name())
			if !entry.IsDir() {
				if fileinfo, err := os.Lstat(entrypath); err != nil {
					log.Println("Could not read file:", err)
				} else {
					files = append(files, FileEntry{path: dir, fileinfo: fileinfo})
				}
			}
		}

		if len(files) < 2 {
			log.Println("Not enough files in", dir)
		}
	}

	return files
}

func TestSort(t *testing.T) {
	files := getDirectoryFiles("/tmp")

	byname := sortFileEntries(SORT_BY_NAME, files)
	if !sort.IsSorted(ByName(byname)) {
		t.Error("Not sorted by name!")
	}

	bymodtime := sortFileEntries(SORT_BY_MODTIME, files)
	if !sort.IsSorted(ByModTime(bymodtime)) {
		t.Error("Not sorted by modtime!")
	}

	bysize := sortFileEntries(SORT_BY_SIZE, files)
	if !sort.IsSorted(BySize(bysize)) {
		t.Error("Not sorted by size!")
	}

	log.Println("TestSort finished")
}

func BenchmarkSortByName(b *testing.B) {
	b.StopTimer()

	files := getDirectoryFiles("/tmp")

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sortFileEntries(SORT_BY_NAME, files)
	}
}

func BenchmarkSortByModTime(b *testing.B) {
	b.StopTimer()

	files := getDirectoryFiles("/tmp")

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sortFileEntries(SORT_BY_MODTIME, files)
	}
}

func BenchmarkSortBySize(b *testing.B) {
	b.StopTimer()

	files := getDirectoryFiles("/tmp")

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sortFileEntries(SORT_BY_SIZE, files)
	}
}

func TestMerge(t *testing.T) {
	directories := []string{os.Getenv("HOME") + "/go/src/golocate/", os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/", os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/cairo/"}
	var byname, bymodtime, bysize []FileEntry
	for _, dir := range directories {
		files := getDirectoryFiles(dir)
		byname = merge(SORT_BY_NAME, byname, sortFileEntries(SORT_BY_NAME, files))
		bymodtime = merge(SORT_BY_MODTIME, bymodtime, sortFileEntries(SORT_BY_MODTIME, files))
		bysize = merge(SORT_BY_SIZE, bysize, sortFileEntries(SORT_BY_SIZE, files))
	}

	if !sort.IsSorted(ByName(byname)) {
		log.Println("---- byname ----")
		for i, entry := range byname {
			log.Println(i, "\t\t", entry.fileinfo.Name())
		}
		log.Println("---- sort.Sort(ByName(byname)) ----")
		sort.Sort(ByName(byname))
		for i, entry := range byname {
			log.Println(i, "\t\t", entry.fileinfo.Name())
		}

		t.Error("Not sorted by name after merging")
	}

	if !sort.IsSorted(ByModTime(bymodtime)) {
		t.Error("Not sorted by modtime after merging")
	}

	if !sort.IsSorted(BySize(bysize)) {
		t.Error("Not sorted by size after merging")
	}

	log.Println("TestMerge finished")
}

func BenchmarkMergeByName(b *testing.B) {
	b.StopTimer()

	directories := []string{os.Getenv("HOME") + "/go/src/golocate/", os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/", os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/cairo/"}
	var cache [][]FileEntry
	for _, dir := range directories {
		files := getDirectoryFiles(dir)
		cache = append(cache, sortFileEntries(SORT_BY_NAME, files))
	}

	var merged []FileEntry
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for _, files := range cache {
			merged = merge(SORT_BY_NAME, merged, files)
		}
	}
}

func BenchmarkMergeByModTime(b *testing.B) {
	b.StopTimer()

	directories := []string{os.Getenv("HOME") + "/go/src/golocate/", os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/", os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/cairo/"}
	var cache [][]FileEntry
	for _, dir := range directories {
		files := getDirectoryFiles(dir)
		cache = append(cache, sortFileEntries(SORT_BY_MODTIME, files))
	}

	var merged []FileEntry
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for _, files := range cache {
			merged = merge(SORT_BY_MODTIME, merged, files)
		}
	}
}

func BenchmarkMergeBySize(b *testing.B) {
	b.StopTimer()

	directories := []string{os.Getenv("HOME") + "/go/src/golocate/", os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/", os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/cairo/"}
	var cache [][]FileEntry
	for _, dir := range directories {
		files := getDirectoryFiles(dir)
		cache = append(cache, sortFileEntries(SORT_BY_SIZE, files))
	}

	var merged []FileEntry
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for _, files := range cache {
			merged = merge(SORT_BY_SIZE, merged, files)
		}
	}
}
