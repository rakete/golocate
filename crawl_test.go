package main

import (
	"os"
	"path"
	//"log"
	"log"
	"regexp"
	"runtime"
	"sync"
	"time"

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
		new(FileEntries),
	}
	config := Configuration{
		cores:       runtime.NumCPU(),
		directories: []string{os.Getenv("HOME"), "/usr", "/var", "/sys", "/opt", "/etc", "/bin", "/sbin"},
		maxinotify:  1024,
	}
	newdirs := make(chan string)
	crawlerquery := make(chan *regexp.Regexp)
	finish := make(chan struct{})

	var wg sync.WaitGroup
	log.Println("starting Crawl on", config.cores, "cores")
	wg.Add(1)
	go Crawler(&wg, mem, config, newdirs, crawlerquery, finish)
	wg.Wait()
	close(finish)
	log.Println("Crawl terminated")

	searchterm := ".*\\.cc$"
	query, _ := regexp.Compile(searchterm)
	cache := MatchCaches{NewSimpleCache(), NewSimpleCache()}
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
		new(FileEntries),
	}
	config := Configuration{
		cores:       runtime.NumCPU(),
		directories: []string{path.Join(os.Getenv("GOPATH"))},
		maxinotify:  1024,
	}
	newdirs := make(chan string)
	crawlerquery := make(chan *regexp.Regexp)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go Crawler(&wg, mem, config, newdirs, crawlerquery, finish)
		time.Sleep(10 * time.Millisecond)
		wg.Wait()
		close(finish)

		cache := MatchCaches{NewSimpleCache(), NewSimpleCache()}
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
		NewDirBucket(),
		NewModTimeBucket(),
		NewSizeBucket(),
	}
	config := Configuration{
		cores:       runtime.NumCPU(),
		directories: []string{path.Join(os.Getenv("GOPATH"))},
		maxinotify:  1024,
	}
	newdirs := make(chan string)
	crawlerquery := make(chan *regexp.Regexp)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go Crawler(&wg, mem, config, newdirs, crawlerquery, finish)
		time.Sleep(10 * time.Millisecond)
		wg.Wait()
		close(finish)

		cache := MatchCaches{NewSimpleCache(), NewSimpleCache()}
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
