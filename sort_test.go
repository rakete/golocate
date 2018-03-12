package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"time"

	"testing"
)

func getDirectoryFiles(dir string) []*FileEntry {
	var files []*FileEntry
	if entries, err := ioutil.ReadDir(dir); err != nil {
		log.Println("Could not read dir:", err)
	} else {
		for _, entry := range entries {
			entrypath := path.Join(dir, entry.Name())
			if !entry.IsDir() {
				if fileinfo, err := os.Lstat(entrypath); err != nil {
					log.Println("Could not read file:", err)
				} else {
					files = append(files, &FileEntry{
						path:    dir,
						name:    fileinfo.Name(),
						modtime: fileinfo.ModTime().Unix(),
						size:    fileinfo.Size(),
					})
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

	byname := sortFileEntries(SortedByName(files))
	if !sort.IsSorted(byname) {
		t.Error("Not sorted by name!")
	}

	bymodtime := sortFileEntries(SortedByModTime(files))
	if !sort.IsSorted(bymodtime) {
		t.Error("Not sorted by modtime!")
	}

	bysize := sortFileEntries(SortedBySize(files))
	if !sort.IsSorted(bysize) {
		t.Error("Not sorted by size!")
	}

	log.Println("TestSort finished")
}

func BenchmarkSortedByName(b *testing.B) {
	b.StopTimer()

	files := getDirectoryFiles("/tmp")

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sortFileEntries(SortedByName(files))
	}
}

func BenchmarkSortedByModTime(b *testing.B) {
	b.StopTimer()

	files := getDirectoryFiles("/tmp")

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sortFileEntries(SortedByModTime(files))
	}
}

func BenchmarkSortedBySize(b *testing.B) {
	b.StopTimer()

	files := getDirectoryFiles("/tmp")

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sortFileEntries(SortedBySize(files))
	}
}

func TestSortMerge(t *testing.T) {
	directories := []string{os.Getenv("HOME") + "/go/src/golocate/", os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/", os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/cairo/"}
	var allfiles, byname, bymodtime, bysize []*FileEntry
	for _, dir := range directories {
		files := getDirectoryFiles(dir)
		allfiles = append(allfiles, files...)

		var temp []*FileEntry
		copy(files, temp)
		byname = sortMerge(SORT_BY_NAME, byname, sortFileEntries(SortedByName(temp)).(SortedByName))
		copy(files, temp)
		bymodtime = sortMerge(SORT_BY_MODTIME, bymodtime, sortFileEntries(SortedByModTime(temp)).(SortedByModTime))
		copy(files, temp)
		bysize = sortMerge(SORT_BY_SIZE, bysize, sortFileEntries(SortedBySize(temp)).(SortedBySize))
	}

	if !sort.IsSorted(SortedByName(byname)) {
		log.Println("---- byname ----")
		for i, entry := range byname {
			log.Println(i, "\t\t", entry.name)
		}
		log.Println("---- sort.Sort(SortedByName(byname)) ----")
		sort.Sort(SortedByName(allfiles))
		for i, entry := range allfiles {
			log.Println(i, "\t\t", entry.name)
		}

		t.Error("Not sorted by name after merging")
	}

	if !sort.IsSorted(SortedByModTime(bymodtime)) {
		t.Error("Not sorted by modtime after merging")
	}

	if !sort.IsSorted(SortedBySize(bysize)) {
		t.Error("Not sorted by size after merging")
	}

	log.Println("TestSortMerge finished")
}

func BenchmarkSortMergeByName(b *testing.B) {
	b.StopTimer()

	directories := []string{
		os.Getenv("HOME") + "/.local/share/Zeal/Zeal/docsets/NET_Framework.docset/Contents/Resources/Documents/msdn.microsoft.com/en-us/library/",
		os.Getenv("HOME") + "/go/src/golocate/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/cairo/",
		os.Getenv("HOME") + "/.local/share/Trash/files",
	}
	var cache [][]*FileEntry
	for _, dir := range directories {
		files := getDirectoryFiles(dir)
		cache = append(cache, sortFileEntries(SortedByName(files)).(SortedByName))
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		var merged []*FileEntry
		for _, files := range cache {
			merged = sortMerge(SORT_BY_NAME, merged, files)
		}
	}
}

func BenchmarkSortMergeByModTime(b *testing.B) {
	b.StopTimer()

	directories := []string{
		os.Getenv("HOME") + "/.local/share/Zeal/Zeal/docsets/NET_Framework.docset/Contents/Resources/Documents/msdn.microsoft.com/en-us/library/",
		os.Getenv("HOME") + "/go/src/golocate/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/cairo/",
		os.Getenv("HOME") + "/.local/share/Trash/files",
	}
	var cache [][]*FileEntry
	for _, dir := range directories {
		files := getDirectoryFiles(dir)
		cache = append(cache, sortFileEntries(SortedByModTime(files)).(SortedByModTime))
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		var merged []*FileEntry
		for _, files := range cache {
			merged = sortMerge(SORT_BY_MODTIME, merged, files)
		}
	}
}

func BenchmarkSortMergeBySize(b *testing.B) {
	b.StopTimer()

	directories := []string{
		os.Getenv("HOME") + "/.local/share/Zeal/Zeal/docsets/NET_Framework.docset/Contents/Resources/Documents/msdn.microsoft.com/en-us/library/",
		os.Getenv("HOME") + "/go/src/golocate/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/cairo/",
		os.Getenv("HOME") + "/.local/share/Trash/files",
	}
	var cache [][]*FileEntry
	for _, dir := range directories {
		files := getDirectoryFiles(dir)
		cache = append(cache, sortFileEntries(SortedBySize(files)).(SortedBySize))
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		var merged []*FileEntry
		for _, files := range cache {
			merged = sortMerge(SORT_BY_SIZE, merged, files)
		}
	}
}

func BenchmarkInt64(b *testing.B) {
	u := int64(time.Now().Unix())
	v := int64(time.Now().Unix())
	var results []bool
	for i := 0; i < b.N; i++ {
		results = append(results, u < v)
	}
}
