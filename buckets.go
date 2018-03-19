package main

import (
	"fmt"
	//"sort"
)

type Threshold interface {
	Less(than Threshold) bool
}

type NameThreshold string
type TimeThreshold int64
type SizeThreshold struct {
	size int64
	name string
}

func (a NameThreshold) Less(b Threshold) bool {
	return a < b.(NameThreshold)
}

func (a TimeThreshold) Less(b Threshold) bool {
	return a < b.(TimeThreshold)
}

func (a SizeThreshold) Less(b Threshold) bool {
	if a.size > b.(SizeThreshold).size {
		return false
	} else if a.size < b.(SizeThreshold).size {
		return true
	} else {
		return a.name < b.(SizeThreshold).name
	}
}

type Node struct {
	threshold Threshold
	queue     []*FileEntry
	sorted    []*FileEntry
	children  []Bucket
}

const (
	BUCKET_ASCENDING = iota
	BUCKET_DESCENDING
)

type Bucket interface {
	Insert(first int, files []*FileEntry) int
	Split(numparts int)
	//Sort()
	Walk(direction int, f func(entry *FileEntry) bool) bool
	//Take(direction int, n int) []*FileEntry
	Print(level int)
}

type NameBucket Node
type TimeBucket Node
type SizeBucket Node

func NewNameBucket() *NameBucket {
	bucket := new(NameBucket)

	return bucket
}

func (node *NameBucket) Merge(files []*FileEntry)                 {}
func (node *NameBucket) Len() int                                 { return 0 }
func (node *NameBucket) Insert(first int, files []*FileEntry) int { return 0 }
func (node *NameBucket) Split(numparts int)                       {}
func (node *NameBucket) Walk(direction int, f func(entry *FileEntry) bool) bool {
	return false
}
func (node *NameBucket) Print(level int) {}

func NewTimeBucket() *TimeBucket {
	bucket := new(TimeBucket)

	return bucket
}

func (node *TimeBucket) Merge(files []*FileEntry)                 {}
func (node *TimeBucket) Len() int                                 { return 0 }
func (node *TimeBucket) Insert(first int, files []*FileEntry) int { return 0 }
func (node *TimeBucket) Split(numparts int)                       {}
func (node *TimeBucket) Walk(direction int, f func(entry *FileEntry) bool) bool {
	return false
}
func (node *TimeBucket) Print(level int) {}

func NewSizeBucket() *SizeBucket {
	bucket := new(SizeBucket)

	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold{10, ""}})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold{100, ""}})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold{1000, ""}})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold{10000, ""}})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold{100000, ""}})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold{1000000, ""}})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold{10000000, ""}})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold{100000000, ""}})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold{1000000000, ""}})
	bucket.children = append(bucket.children, &SizeBucket{})

	return bucket
}

func (node *SizeBucket) Merge(files []*FileEntry) {
	node.Insert(0, files)
}

func (node *SizeBucket) Len() int {
	length := 0
	for _, child := range node.children {
		length += child.(*SizeBucket).Len()
	}
	length += len(node.queue)
	length += len(node.sorted)
	return length
}

func (node *SizeBucket) Insert(first int, files []*FileEntry) int {
	i := first
	for _, childnode := range node.children {
		childthreshold := childnode.(*SizeBucket).threshold
		for i < len(files) && (childthreshold == nil || (SizeThreshold{files[i].size, files[i].name}).Less(childthreshold)) {
			if len(childnode.(*SizeBucket).children) > 0 {
				i = childnode.Insert(i, files)
			} else {
				childnode.(*SizeBucket).queue = append(childnode.(*SizeBucket).queue, files[i])
				i += 1
			}
		}

		if len(childnode.(*SizeBucket).queue) >= 100000 {
			childnode.Split(10)
		}

		if i >= len(files) {
			break
		}
	}

	return i
}

func (node *SizeBucket) Split(numparts int) {
	if node.threshold == nil {
		return
	}

	if len(node.queue) < numparts {
		return
	}

	if len(node.children) > 0 {
		return
	}

	node.queue = sortFileEntries(SortedBySize(node.queue)).(SortedBySize)
	node.sorted = sortMerge(SORT_BY_SIZE, node.sorted, node.queue)
	node.queue = nil

	if node.sorted[0].size == node.sorted[len(node.sorted)-1].size {
		return
	}

	inc := len(node.sorted) / numparts
	a := 0
	b := inc
	for i := 0; i < numparts-1; i++ {
		pivot := node.sorted[b]
		newnode := &SizeBucket{
			threshold: SizeThreshold{pivot.size, pivot.name},
			queue:     nil,
			sorted:    make([]*FileEntry, b-a),
			children:  nil,
		}
		copy(newnode.sorted, node.sorted[a:b])
		node.children = append(node.children, newnode)
		a = b
		b += inc
	}
	lastnode := &SizeBucket{
		threshold: node.threshold,
		queue:     nil,
		sorted:    make([]*FileEntry, len(node.sorted)-a),
		children:  nil,
	}
	copy(lastnode.sorted, node.sorted[a:len(node.sorted)])
	node.children = append(node.children, lastnode)
	node.sorted = nil
}

func (node *SizeBucket) Walk(direction int, f func(entry *FileEntry) bool) bool {
	var indexfunc func(int, int) int
	switch direction {
	case BUCKET_ASCENDING:
		indexfunc = func(l, i int) int { return i }
	case BUCKET_DESCENDING:
		indexfunc = func(l, j int) int { return l - 1 - j }
	}

	for i := range node.children {
		child := node.children[indexfunc(len(node.children), i)]
		if len(child.(*SizeBucket).children) > 0 {
			if !child.Walk(direction, f) {
				return false
			}
		} else {
			sorted := child.(*SizeBucket).sorted
			for j := range sorted {
				entry := sorted[indexfunc(len(sorted), j)]
				if !f(entry) {
					return false
				}
			}
		}
	}

	return true
}

func (node *SizeBucket) Print(level int) {
	for _, child := range node.children {
		fmt.Print(level, ":")
		for i := 0; i < level; i++ {
			fmt.Print(" ")
		}
		if len(child.(*SizeBucket).children) > 0 {
			fmt.Println("parent:", child.(*SizeBucket).threshold.(SizeThreshold).size)
			child.Print(level + 1)
		} else {
			if child.(*SizeBucket).threshold == nil {
				fmt.Println("maximum", "numfiles:", len(child.(*SizeBucket).queue)+len(child.(*SizeBucket).sorted))
			} else {
				fmt.Println(child.(*SizeBucket).threshold.(SizeThreshold).size, "numfiles:", len(child.(*SizeBucket).queue)+len(child.(*SizeBucket).sorted))
			}
		}
	}
}
