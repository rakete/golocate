package main

import (
	//"fmt"
	"log"
	"os"
	//"time"
	"regexp"
	"runtime"
	"sort"
	"sync"
	"time"

	"testing"
)

func TestBuckets(t *testing.T) {
	log.Println("running TestBuckets")

	display := DisplayChannel{
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
	if testing.Short() {
		directories = []string{os.Getenv("HOME")}
	}

	newdirs := make(chan string)
	cores := runtime.NumCPU()

	var wg sync.WaitGroup
	log.Println("starting Crawl on", cores, "cores")
	wg.Add(1)
	go Crawler(&wg, cores*2, mem, display, newdirs, finish)
	for _, dir := range directories {
		newdirs <- dir
	}
	wg.Wait()
	close(finish)
	log.Println("Crawl terminated")

	query, _ := regexp.Compile("golocate")
	byname := mem.byname.Take(SORT_BY_NAME, DIRECTION_ASCENDING, query, 1000)
	bymodtime := mem.bymodtime.Take(SORT_BY_MODTIME, DIRECTION_ASCENDING, query, 1000)
	bysize := mem.bysize.Take(SORT_BY_SIZE, DIRECTION_ASCENDING, query, 1000)

	log.Println("len(byname):", len(byname))
	log.Println("len(bymodtime):", len(bymodtime))
	log.Println("len(bysize):", len(bysize))

	//Print(mem.byname.(*NameBucket), 0)
	//Print(mem.bymodtime.(*ModTimeBucket), 0)
	//Print(mem.bysize.(*SizeBucket), 0)

	var lastentry *FileEntry
	WalkEntries(mem.bysize.(*Node), DIRECTION_ASCENDING, func(entry *FileEntry) bool {
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
	WalkEntries(mem.bysize.(*Node), DIRECTION_DESCENDING, func(entry *FileEntry) bool {
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

	WalkNodes(mem.bysize.(*Node), DIRECTION_ASCENDING, func(child Bucket) bool {
		node := child.Node()

		if !sort.IsSorted(SortedBySize(node.sorted)) {
			t.Error("Found a node.sorted that is not sorted")
			return false
		}

		if len(node.children) == 1 {
			t.Error("Found a node with only one children")
			return false
		}

		for _, entry := range node.sorted {
			if node.threshold != nil {
				if !SizeThreshold(entry.size).Less(node.threshold) {
					t.Error("Found an entry.size that is not less then its threshold")
					return false
				}
			}
		}
		return true
	})

	log.Println("TestBuckets finished")
}

func TestLess(t *testing.T) {
	if NameThreshold("=.html").Less(NameThreshold("9")) {
		t.Error("=.html < 9")
	}

	if NameThreshold("=.html").Less(NameThreshold("1")) {
		t.Error("=.html < 1")
	}

	if NameThreshold("b").Less(NameThreshold("aaaaaaaaaaaaaaaaaaaaaaaaa")) {
		t.Error("b < aaaaaaaaaaaaaaaaaaaaaaaaa")
	}

	if NameThreshold("a").Less(NameThreshold("0")) {
		t.Error("a < 0")
	}

	if NameThreshold("a").Less(NameThreshold("A")) {
		t.Error("a < A")
	}

	if ModTimeThreshold(time.Now().Add(-time.Minute)).Less(ModTimeThreshold(time.Now())) {
		t.Error("time.Now().Add(-time.Minute) < time.Now")
	}

	if SizeThreshold(1).Less(SizeThreshold(0)) {
		t.Error("1 < 0")
	}

	log.Println("TestLess finished")
}
