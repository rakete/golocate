package main

import (
	//"fmt"
	"log"
	"os"
	//"time"
	"runtime"

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
		NewTimeBucket(),
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
	log.Println("mem.byname", mem.byname.Len())
	log.Println("mem.bymodtime", mem.bymodtime.Len())
	log.Println("mem.bysize:", mem.bysize.Len())

	//mem.bysize.(*SizeBucket).Print(0)

	var lastentry *FileEntry
	mem.bysize.(*SizeBucket).Walk(BUCKET_ASCENDING, func(entry *FileEntry) bool {
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
	mem.bysize.(*SizeBucket).Walk(BUCKET_DESCENDING, func(entry *FileEntry) bool {
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

	log.Println("TestLess finished")
}
