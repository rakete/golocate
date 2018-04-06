package main

import (
	"os"
	"path"
	//"log"
	"log"
	"regexp"
	"runtime"
	"sync"

	"github.com/gotk3/gotk3/gtk"

	"testing"
)

func TestFileEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TestFileEntries in short mode.")
	}
	log.Println("running TestFileEntries")

	mem := ResultMemory{
		new(FileEntries),
		new(FileEntries),
		new(FileEntries),
	}
	finish := make(chan struct{})

	//directories := []string{os.Getenv("HOME")}
	directories := []string{os.Getenv("HOME"), "/usr", "/var", "/sys", "/opt", "/etc", "/bin", "/sbin"}
	newdirs := make(chan string)
	cores := runtime.NumCPU()

	var wg sync.WaitGroup
	log.Println("starting Crawl on", cores, "cores")
	wg.Add(1)
	go Crawler(&wg, cores*2, mem, newdirs, finish)
	for _, dir := range directories {
		newdirs <- dir
	}
	wg.Wait()
	close(finish)
	log.Println("Crawl terminated")

	query, _ := regexp.Compile("golocate")
	byname, _ := mem.byname.Take(SORT_BY_NAME, gtk.SORT_ASCENDING, query, 1000)
	bymodtime, _ := mem.bymodtime.Take(SORT_BY_MODTIME, gtk.SORT_ASCENDING, query, 1000)
	bysize, _ := mem.bysize.Take(SORT_BY_SIZE, gtk.SORT_ASCENDING, query, 1000)

	log.Println("len(byname):", len(byname))
	log.Println("len(bymodtime):", len(bymodtime))
	log.Println("len(bysize):", len(bysize))

	//Print(mem.byname.(*NameBucket), 0)
	//Print(mem.bymodtime.(*ModTimeBucket), 0)
	//Print(mem.bysize.(*SizeBucket), 0)

	log.Println("TestFileEntries finished")
}

func BenchmarkCrawlLargeSlice(b *testing.B) {
	b.StopTimer()

	mem := ResultMemory{
		new(FileEntries),
		new(FileEntries),
		new(FileEntries),
	}
	directories := []string{path.Join(os.Getenv("HOME"), "/go/src/golocate")}
	newdirs := make(chan string)

	cores := runtime.NumCPU()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go Crawler(&wg, cores, mem, newdirs, finish)
		for _, dir := range directories {
			newdirs <- dir
		}
		wg.Wait()
		close(finish)

		query, _ := regexp.Compile(".*")
		mem.byname.Take(SORT_BY_NAME, gtk.SORT_ASCENDING, query, 1000)
		mem.bymodtime.Take(SORT_BY_MODTIME, gtk.SORT_ASCENDING, query, 1000)
		mem.bysize.Take(SORT_BY_SIZE, gtk.SORT_ASCENDING, query, 1000)
	}
}

func BenchmarkCrawlBuckets(b *testing.B) {
	b.StopTimer()

	mem := ResultMemory{
		NewNameBucket(),
		NewModTimeBucket(),
		NewSizeBucket(),
	}
	directories := []string{path.Join(os.Getenv("HOME"), "/go/src/golocate")}
	newdirs := make(chan string)

	cores := runtime.NumCPU()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go Crawler(&wg, cores, mem, newdirs, finish)
		for _, dir := range directories {
			newdirs <- dir
		}
		wg.Wait()
		close(finish)

		query, _ := regexp.Compile(".*")
		mem.byname.Take(SORT_BY_NAME, gtk.SORT_ASCENDING, query, 1000)
		mem.bymodtime.Take(SORT_BY_MODTIME, gtk.SORT_ASCENDING, query, 1000)
		mem.bysize.Take(SORT_BY_SIZE, gtk.SORT_ASCENDING, query, 1000)
	}
}
