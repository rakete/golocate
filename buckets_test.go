package main

import (
	//"fmt"
	"log"
	"os"
	//"time"

	"testing"
)

func TestBuckets(t *testing.T) {

	// directories := []string{
	// 	os.Getenv("HOME") + "/.local/share/Zeal/Zeal/docsets/NET_Framework.docset/Contents/Resources/Documents/msdn.microsoft.com/en-us/library/",
	// 	os.Getenv("HOME") + "/go/src/golocate/",
	// 	os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/",
	// 	os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/cairo/",
	// 	os.Getenv("HOME") + "/.local/share/Trash/files",
	// 	//os.Getenv("HOME") + "/.local/share/Zeal/Zeal/docsets/NET_Framework.docset/Contents/Resources/Documents/msdn.microsoft.com/en-us/library/",
	// }

	// buckets := NewSizeBucket()
	// for _, dir := range directories {
	// 	files := getDirectoryFiles([]string{dir})
	// 	bysize := sortFileEntries(SortedBySize(files))

	// 	buckets.Merge(bysize.(SortedBySize))
	// }

	// buckets.Print(0)

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
	directories := []string{os.Getenv("HOME")}
	Crawl(mem, display, finish, directories, nil)

	// bysize := mem.bysize.(*SizeBucket)
	// bysize.Print(0)

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
