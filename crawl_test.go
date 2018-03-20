package main

import (
	//"os"
	//"log"
	"runtime"

	"testing"
)

func BenchmarkCrawlLargeSlice(b *testing.B) {
	b.StopTimer()

	display := ResultChannel{
		make(chan CrawlResult),
		make(chan CrawlResult),
		make(chan CrawlResult),
	}
	mem := ResultMemory{
		new(NameEntries),
		new(TimeEntries),
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
	directories := []string{"/tmp"}

	cores := runtime.NumCPU()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		Crawl(cores, mem, display, finish, directories, nil)
		<-finish
	}
}

func BenchmarkCrawlBuckets(b *testing.B) {
	b.StopTimer()

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
	go func() {
		for {
			select {
			case <-display.byname:
			case <-display.bymodtime:
			case <-display.bysize:
			}
		}
	}()
	directories := []string{"/tmp"}

	cores := runtime.NumCPU()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		Crawl(cores, mem, display, finish, directories, nil)
		<-finish
	}
}
