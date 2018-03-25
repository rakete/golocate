package main

import (
	"os"
	"path"
	//"log"
	"runtime"
	"sync"

	"testing"
)

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
	}
}
