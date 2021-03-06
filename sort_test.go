package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"

	"testing"
)

func getDirectoryFiles(dirs []string) []*FileEntry {
	var files []*FileEntry
	for _, dir := range dirs {
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
							dir:     dir,
							name:    fileinfo.Name(),
							modtime: fileinfo.ModTime(),
							size:    fileinfo.Size(),
						})
					}
				}
			}

			if len(files) < 2 {
				log.Println("Not enough files in", dir)
			}
		}
	}

	return files
}

func TestSort(t *testing.T) {
	files := getDirectoryFiles([]string{"/tmp"})

	sort.Stable(SortedByName(files))
	if !sort.IsSorted(SortedByName(files)) {
		t.Error("Not sorted by name!")
	}

	sort.Stable(SortedByModTime(files))
	if !sort.IsSorted(SortedByModTime(files)) {
		t.Error("Not sorted by modtime!")
	}

	sort.Stable(SortedBySize(files))
	if !sort.IsSorted(SortedBySize(files)) {
		t.Error("Not sorted by size!")
	}

	log.Println("TestSort finished")
}

func BenchmarkSortedByName(b *testing.B) {
	b.StopTimer()

	files := getDirectoryFiles([]string{"/tmp"})

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sort.Stable(SortedByName(files))
	}
}

func BenchmarkSortedByDir(b *testing.B) {
	b.StopTimer()

	files := getDirectoryFiles([]string{"/tmp"})

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sort.Stable(SortedByDir(files))
	}
}

func BenchmarkSortedByModTime(b *testing.B) {
	b.StopTimer()

	files := getDirectoryFiles([]string{"/tmp"})

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sort.Stable(SortedByModTime(files))
	}
}

func BenchmarkSortedBySize(b *testing.B) {
	b.StopTimer()

	files := getDirectoryFiles([]string{"/tmp"})

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		sort.Stable(SortedBySize(files))
	}
}

func TestSortMerge(t *testing.T) {
	directories := []string{
		os.Getenv("HOME") + "/go/src/golocate/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/github.com/gotk3/gotk3/gtk/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/github.com/gotk3/gotk3/glib/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/github.com/gotk3/gotk3/gdk/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/github.com/gotk3/gotk3/cairo/",
	}
	var allfiles, byname, bymodtime, bysize []*FileEntry
	for _, dir := range directories {
		files := getDirectoryFiles([]string{dir})
		allfiles = append(allfiles, files...)

		bynamefiles := make([]*FileEntry, len(files))
		copy(bynamefiles, files)
		sort.Stable(SortedByName(bynamefiles))
		byname = sortMerge(SORT_BY_NAME, byname, bynamefiles)
		if len(byname) < len(allfiles) {
			t.Error("Result of sortMerge for SORT_BY_NAME contains less entries then its input", len(byname), len(allfiles))
		}

		bymodtimefiles := make([]*FileEntry, len(files))
		copy(bymodtimefiles, files)
		sort.Stable(SortedByModTime(bymodtimefiles))
		bymodtime = sortMerge(SORT_BY_MODTIME, bymodtime, bymodtimefiles)
		if len(bymodtime) < len(allfiles) {
			t.Error("Result of sortMerge for SORT_BY_MODTIME contains less entries then its input", len(bymodtime), len(allfiles))
		}

		bysizefiles := make([]*FileEntry, len(files))
		copy(bysizefiles, files)
		sort.Stable(SortedBySize(bysizefiles))
		bysize = sortMerge(SORT_BY_SIZE, bysize, bysizefiles)
		if len(bysize) < len(allfiles) {
			t.Error("Result of sortMerge for SORT_BY_SIZE contains less entries then its input", len(bysize), len(allfiles))
		}
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

func BenchmarkSortMerge(b *testing.B) {
	directories := []string{
		os.Getenv("HOME") + "/go/src/golocate/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/github.com/gotk3/gotk3/gtk/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/github.com/gotk3/gotk3/glib/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/github.com/gotk3/gotk3/gdk/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/github.com/gotk3/gotk3/cairo/",
	}

	benchmarks := []struct {
		name    string
		sorting SortColumn
	}{
		{"ByName", SORT_BY_NAME},
		{"ByDir", SORT_BY_DIR},
		{"ByModTime", SORT_BY_MODTIME},
		{"BySize", SORT_BY_SIZE},
	}

	for _, bm := range benchmarks {

		var cache [][]*FileEntry
		for _, dir := range directories {
			files := getDirectoryFiles([]string{dir})
			switch bm.sorting {
			case SORT_BY_NAME:
				sort.Stable(SortedByName(files))
				cache = append(cache, files)
			case SORT_BY_DIR:
				sort.Stable(SortedByDir(files))
				cache = append(cache, files)
			case SORT_BY_MODTIME:
				sort.Stable(SortedByModTime(files))
				cache = append(cache, files)
			case SORT_BY_SIZE:
				sort.Stable(SortedBySize(files))
				cache = append(cache, files)
			}
		}
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var merged []*FileEntry
				for _, files := range cache {
					merged = sortMerge(bm.sorting, merged, files)
				}
			}
		})
	}
}
