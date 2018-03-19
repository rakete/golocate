package main

import (
	//"os"
	"log"
	"runtime"

	"testing"
)

func BenchmarkCrawl(b *testing.B) {
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
	log.Println("start Crawl on", cores, "cores")
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		Crawl(cores, mem, display, finish, directories, nil)
		<-finish
	}
}
