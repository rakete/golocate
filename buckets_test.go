package main

import (
	"log"

	"testing"
)

func TestBuckets(t *testing.T) {

	files := getDirectoryFiles("/tmp")
	bysize := sortFileEntries(SortedBySize(files))

	buckets := NewBucket(SORT_BY_SIZE)
	buckets.Insert(bysize.(SortedBySize))
	buckets.Print()

	log.Println("TestBuckets finished")
}

func BenchBucketInsertSortedBySize(b *testing.B) {
}
