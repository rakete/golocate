package main

import (
	//"fmt"
	"log"
	"os"
	//"time"
	"runtime"
	"sort"

	"testing"
)

func TestBuckets(t *testing.T) {
	display := ResultChannel{
		make(chan CrawlResult),
		make(chan CrawlResult),
		make(chan CrawlResult),
	}
	mem := ResultMemory{
		NewNameBucket(),
		NewModTimeBucket(),
		NewSizeBucket(),
	}
	finish := make(chan struct{})
	go func() {
		for {
			select {
			case <-display.byname:
			case <-display.bymodtime:
			case <-display.bysize:
			}
		}
	}()

	//directories := []string{os.Getenv("HOME")}
	directories := []string{os.Getenv("HOME"), "/usr", "/var", "/sys", "/opt", "/etc", "/bin", "/sbin"}
	cores := runtime.NumCPU()

	log.Println("start Crawl on", cores, "cores")
	Crawl(cores, mem, display, finish, directories, nil)

	<-finish
	log.Println("closed finish in Crawl")
	log.Println("mem.byname", mem.byname.NumFiles())
	log.Println("mem.bymodtime", mem.bymodtime.NumFiles())
	log.Println("mem.bysize:", mem.bysize.NumFiles())

	//Print(mem.byname.(*NameBucket), 0)
	//Print(mem.bymodtime.(*ModTimeBucket), 0)
	//Print(mem.bysize.(*SizeBucket), 0)

	var lastentry *FileEntry
	WalkEntries(mem.bysize.(*SizeBucket), BUCKET_ASCENDING, func(entry *FileEntry) bool {
		if lastentry == nil {
			lastentry = entry
			return true
		} else {
			if lastentry.size > entry.size {
				t.Error("SizeBucket Walk could not assert ASCENDING sorting")
				return false
			}
			lastentry = entry
			return true
		}
	})

	lastentry = nil
	WalkEntries(mem.bysize.(*SizeBucket), BUCKET_DESCENDING, func(entry *FileEntry) bool {
		if lastentry == nil {
			lastentry = entry
			return true
		} else {
			if lastentry.size < entry.size {
				t.Error("SizeBucket Walk could not assert DESCENDING sorting")
				return false
			}
			lastentry = entry
		}
		return true
	})

	WalkNodes(mem.bysize.(*SizeBucket), BUCKET_ASCENDING, func(direction int, node *Node) bool {
		if !sort.IsSorted(SortedBySize(node.sorted)) {
			t.Error("Found a node.sorted that is not sorted")
			return false
		}

		if len(node.children) == 1 {
			t.Error("Found a node with only one children")
			return false
		}

		for _, entry := range node.sorted {
			if !SizeThreshold(entry.size).Less(node.threshold) {
				t.Error("Found an entry.size that is not less then its threshold")
				return false
			}
		}
		return true
	})

	log.Println("TestBuckets finished")
}

func TestLess(t *testing.T) {
	if "=.html" < "9" {
		log.Println("=.html < 9")
	}

	if "=.html" < "1" {
		log.Println("=.html < 1")
	}

	if "1.h" < "1" {
		log.Println("1.h < 1")
	}

	if "b" < "aaaaaaaaaaaaaaaaaaaaaaaaaa" {
		log.Println("b < aaaaaaaaaaaaaaaaaaaaaaaaa")
	}

	if "a" < "0" {
		log.Println("a < 0")
	}

	if "a" < "A" {
		log.Println("a < A")
	}

	log.Println("TestLess finished")
}
