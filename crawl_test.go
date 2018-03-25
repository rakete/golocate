package main

import (
	"os"
	"path"
	//"log"
	"log"
	"runtime"
	"sync"

	"testing"
)

func TestFileEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TestFileEntries in short mode.")
	}
	log.Println("running TestFileEntries")

	display := DisplayChannel{
		make(chan int),
		make(chan int),
		make(chan CrawlResult),
		make(chan CrawlResult),
		make(chan CrawlResult),
	}
	mem := ResultMemory{
		new(NameEntries),
		new(ModTimeEntries),
		new(SizeEntries),
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
	newdirs := make(chan string)
	cores := runtime.NumCPU()

	var wg sync.WaitGroup
	log.Println("starting Crawl on", cores, "cores")
	go Crawl(&wg, cores, mem, display, newdirs, finish, nil)
	for _, dir := range directories {
		newdirs <- dir
	}
	wg.Wait()
	close(finish)
	log.Println("Crawl terminated")

	byname := mem.byname.Take(DIRECTION_ASCENDING, 1000)
	bymodtime := mem.bymodtime.Take(DIRECTION_ASCENDING, 1000)
	bysize := mem.bysize.Take(DIRECTION_ASCENDING, 1000)

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

	display := DisplayChannel{
		make(chan int),
		make(chan int),
		make(chan CrawlResult),
		make(chan CrawlResult),
		make(chan CrawlResult),
	}
	mem := ResultMemory{
		new(NameEntries),
		new(ModTimeEntries),
		new(SizeEntries),
	}
	go func() {
		for {
			select {
			case <-display.byname:
			case <-display.bymodtime:
			case <-display.bysize:
			}
		}
	}()
	directories := []string{path.Join(os.Getenv("HOME"), "/go/src/golocate")}
	newdirs := make(chan string)

	cores := runtime.NumCPU()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		var wg sync.WaitGroup
		go Crawl(&wg, cores, mem, display, newdirs, finish, nil)
		for _, dir := range directories {
			newdirs <- dir
		}
		wg.Wait()
		close(finish)
		mem.byname.Take(DIRECTION_ASCENDING, 1000)
		mem.bymodtime.Take(DIRECTION_ASCENDING, 1000)
		mem.bysize.Take(DIRECTION_ASCENDING, 1000)
	}
}

func BenchmarkCrawlBuckets(b *testing.B) {
	b.StopTimer()

	display := DisplayChannel{
		make(chan int),
		make(chan int),
		make(chan CrawlResult),
		make(chan CrawlResult),
		make(chan CrawlResult),
	}
	mem := ResultMemory{
		NewNameBucket(),
		NewModTimeBucket(),
		NewSizeBucket(),
	}
	go func() {
		for {
			select {
			case <-display.byname:
			case <-display.bymodtime:
			case <-display.bysize:
			}
		}
	}()
	directories := []string{path.Join(os.Getenv("HOME"), "/go/src/golocate")}
	newdirs := make(chan string)

	cores := runtime.NumCPU()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		var wg sync.WaitGroup
		go Crawl(&wg, cores, mem, display, newdirs, finish, nil)
		for _, dir := range directories {
			newdirs <- dir
		}
		wg.Wait()
		close(finish)
		mem.byname.Take(DIRECTION_ASCENDING, 1000)
		mem.bymodtime.Take(DIRECTION_ASCENDING, 1000)
		mem.bysize.Take(DIRECTION_ASCENDING, 1000)
	}
}