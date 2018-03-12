package main

import (
	"fmt"
	"time"
)

type Threshold interface {
	Less(than Threshold) bool
}

type NameThreshold string

func (a NameThreshold) Less(b Threshold) bool {
	return a < b.(NameThreshold)
}

type TimeThreshold time.Time

func (a TimeThreshold) Less(b Threshold) bool {
	return time.Time(a).Before(time.Time(b.(TimeThreshold)))
}

type SizeThreshold int64

func (a SizeThreshold) Less(b Threshold) bool {
	return a < b.(SizeThreshold)
}

type Node struct {
	threshold Threshold
	queue     []FileEntry
	sorted    []FileEntry
	children  []Node
}

type Bucket interface {
	Insert(values []*FileEntry)
	Print()
}

type NameBucket Node
type TimeBucket Node
type SizeBucket Node

func NewBucket(sorttype int) Bucket {
	var children []Node
	var bucket Bucket
	switch sorttype {
	case SORT_BY_SIZE:
		children = append(children, Node{threshold: SizeThreshold(1)})
		children = append(children, Node{threshold: SizeThreshold(10)})
		children = append(children, Node{threshold: SizeThreshold(100)})
		children = append(children, Node{threshold: SizeThreshold(1000)})
		children = append(children, Node{threshold: SizeThreshold(10000)})
		children = append(children, Node{threshold: SizeThreshold(100000)})
		children = append(children, Node{threshold: SizeThreshold(1000000)})
		children = append(children, Node{threshold: SizeThreshold(10000000)})
		children = append(children, Node{threshold: SizeThreshold(100000000)})
		children = append(children, Node{threshold: SizeThreshold(1000000000)})
		children = append(children, Node{threshold: SizeThreshold(10000000000)})
		children = append(children, Node{})

		bucket = SizeBucket(Node{
			children: children,
		})
	}

	return bucket
}

func (node SizeBucket) Insert(values []*FileEntry) {
	j := 0
	for i, bucket := range node.children {
		for j < len(values) && (bucket.threshold == nil || SizeThreshold(values[j].size).Less(bucket.threshold)) {
			node.children[i].queue = append(node.children[i].queue, *values[j])
			j += 1
		}

		if j == len(values) {
			break
		}
	}
}

func (node SizeBucket) Print() {
	for _, child := range node.children {
		if child.threshold == nil {
			fmt.Println("size: < maximum", "files:", len(child.queue)+len(child.sorted))
		} else {
			fmt.Println("size: <", child.threshold, "files:", len(child.queue)+len(child.sorted))
		}
	}
}
