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

	searchterm := "golocate"
	query, _ := regexp.Compile(searchterm)
	cache := MatchCaches{NewSyncCache(), NewSyncCache()}
	abort := make(chan struct{})
	taken := make(chan *FileEntry)

	var byname, bymodtime, bysize []*FileEntry

	taker := func(xs *[]*FileEntry) {
		for {
			entry := <-taken
			if entry == nil {
				return
			}
			*xs = append(*xs, entry)
		}
	}

	go taker(&byname)
	mem.byname.Take(cache, SORT_BY_NAME, gtk.SORT_ASCENDING, query, 1000, abort, taken)

	go taker(&bymodtime)
	mem.bymodtime.Take(cache, SORT_BY_MODTIME, gtk.SORT_ASCENDING, query, 1000, abort, taken)

	go taker(&bysize)
	mem.bysize.Take(cache, SORT_BY_SIZE, gtk.SORT_ASCENDING, query, 1000, abort, taken)

	log.Println("len(byname):", len(byname), mem.byname.NumFiles())
	log.Println("len(bymodtime):", len(bymodtime), mem.bymodtime.NumFiles())
	log.Println("len(bysize):", len(bysize), mem.bysize.NumFiles())

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
	directories := []string{path.Join(os.Getenv("GOPATH"))}
	newdirs := make(chan string)

	cores := runtime.NumCPU()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go Crawler(&wg, cores, mem, newdirs, finish)
		for _, dir := range directories {
			newdirs <- dir
		}
		wg.Done()
		wg.Wait()
		close(finish)

		cache := MatchCaches{NewSyncCache(), NewSyncCache()}
		abort := make(chan struct{})
		taken := make(chan *FileEntry)

		var bymodtime []*FileEntry

		taker := func(xs *[]*FileEntry) {
			for {
				entry := <-taken
				*xs = append(*xs, entry)
			}
		}

		go taker(&bymodtime)
		mem.bymodtime.Take(cache, SORT_BY_MODTIME, gtk.SORT_ASCENDING, nil, 10, abort, taken)
		mem.bymodtime.Take(cache, SORT_BY_MODTIME, gtk.SORT_ASCENDING, nil, 100, abort, taken)
		mem.bymodtime.Take(cache, SORT_BY_MODTIME, gtk.SORT_ASCENDING, nil, 1000, abort, taken)
	}
}

func BenchmarkCrawlBuckets(b *testing.B) {
	b.StopTimer()

	mem := ResultMemory{
		NewNameBucket(),
		NewModTimeBucket(),
		NewSizeBucket(),
	}
	directories := []string{path.Join(os.Getenv("GOPATH"))}
	newdirs := make(chan string)

	cores := runtime.NumCPU()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go Crawler(&wg, cores, mem, newdirs, finish)
		for _, dir := range directories {
			newdirs <- dir
		}
		wg.Done()
		wg.Wait()
		close(finish)

		cache := MatchCaches{NewSyncCache(), NewSyncCache()}
		abort := make(chan struct{})
		taken := make(chan *FileEntry)

		var bymodtime []*FileEntry

		taker := func(xs *[]*FileEntry) {
			for {
				entry := <-taken
				*xs = append(*xs, entry)
			}
		}

		go taker(&bymodtime)
		mem.bymodtime.Take(cache, SORT_BY_MODTIME, gtk.SORT_ASCENDING, nil, 10, abort, taken)
		mem.bymodtime.Take(cache, SORT_BY_MODTIME, gtk.SORT_ASCENDING, nil, 100, abort, taken)
		mem.bymodtime.Take(cache, SORT_BY_MODTIME, gtk.SORT_ASCENDING, nil, 1000, abort, taken)
	}
}
