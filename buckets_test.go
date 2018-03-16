package main

import (
	//"fmt"
	"log"
	"os"
	"time"

	"testing"
)

func TestBuckets(t *testing.T) {

	directories := []string{
		os.Getenv("HOME") + "/.local/share/Zeal/Zeal/docsets/NET_Framework.docset/Contents/Resources/Documents/msdn.microsoft.com/en-us/library/",
		os.Getenv("HOME") + "/go/src/golocate/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/",
		os.Getenv("HOME") + "/go/src/golocate/vendor/gotk3/cairo/",
		os.Getenv("HOME") + "/.local/share/Trash/files",
		//os.Getenv("HOME") + "/.local/share/Zeal/Zeal/docsets/NET_Framework.docset/Contents/Resources/Documents/msdn.microsoft.com/en-us/library/",
	}

	buckets := NewSizeBucket()
	for _, dir := range directories {
		files := getDirectoryFiles([]string{dir})
		bysize := sortFileEntries(SortedBySize(files))

		buckets.Merge(bysize.(SortedBySize))
	}

	buckets.Print(0)

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

func TestBucketCrawl(t *testing.T) {
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
	directories := []string{os.Getenv("HOME"), "/usr", "/var", "/sys", "/opt", "/etc", "/bin", "/sbin"}
	Crawl(mem, display, finish, directories, nil)

	time.Sleep(3000)
	bysize := mem.bysize.(*SizeBucket)
	bysize.Print(0)

	// node := mem.bysize.(*SizeBucket)
	// err := false
	// for i, child := range node.children {
	// 	if len(child.queue) > 0 && child.threshold.(SizeThreshold).size == 0 {
	// 		firstchar := child.queue[0].name[0]
	// 		for _, entry := range child.queue {
	// 			if firstchar != entry.name[0] && i != 0 {
	// 				fmt.Println(i, string(firstchar), entry.path, entry.name, entry.size)
	// 				err = true
	// 			}
	// 		}
	// 	}
	// 	if err {
	// 		t.Error("Bucket", i, "contains names with different starting chars.")
	// 		err = false
	// 	}
	// }
}
