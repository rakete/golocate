package main

import (
	//"os"

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

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		finish := make(chan struct{})
		Crawl(mem, display, finish, directories, nil)
		<-finish
	}
}
