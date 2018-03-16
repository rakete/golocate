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

type Bucket interface {
	Insert(first int, files []*FileEntry) int
	Split(numparts int)
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
func (node *NameBucket) Print(level int)                          {}

func NewTimeBucket() *TimeBucket {
	bucket := new(TimeBucket)

	return bucket
}

func (node *TimeBucket) Merge(files []*FileEntry)                 {}
func (node *TimeBucket) Len() int                                 { return 0 }
func (node *TimeBucket) Insert(first int, files []*FileEntry) int { return 0 }
func (node *TimeBucket) Split(numparts int)                       {}
func (node *TimeBucket) Print(level int)                          {}

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
	//fmt.Println("Insert")
	// if !sort.IsSorted(SortedBySize(files)) {
	// 	panic("sort")
	// }

	i := first
	for _, childnode := range node.children {
		childthreshold := childnode.(*SizeBucket).threshold
		for i < len(files) && (childthreshold == nil || (SizeThreshold{files[i].size, files[i].name}).Less(childthreshold)) {
			if len(childnode.(*SizeBucket).children) > 0 {
				i = childnode.Insert(i, files)
			} else {
				childnode.(*SizeBucket).queue = append(childnode.(*SizeBucket).queue, files[i])
				//fmt.Println("append:", files[i].size, childthreshold)
				// for test := 0; test < ci; test++ {
				// 	testthreshold := node.children[test].(*SizeBucket).threshold
				// 	if (SizeThreshold{files[i].size, files[i].name}).Less(testthreshold) {
				// 		fmt.Println(files[i].size, test, ci, i, testthreshold, childthreshold)
				// 		panic("insert")
				// 	}
				// }
				i += 1
			}

			if len(childnode.(*SizeBucket).queue) >= 10000 {
				childnode.Split(10)
				// if len(childnode.(*SizeBucket).queue) > 0 || len(childnode.(*SizeBucket).sorted) > 0 {
				// 	panic("len")
				// }
				// if ci > 0 && len(childnode.(*SizeBucket).children) > 0 {
				// 	ta := childnode.(*SizeBucket).children[0].(*SizeBucket).threshold
				// 	tb := node.children[ci-1].(*SizeBucket).threshold
				// 	if ta.Less(tb) {
				// 		fmt.Println(ta, tb)
				// 		panic("lala")
				// 	} else {
				// 		fmt.Println("split", childthreshold)
				// 	}
				// }
			}
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
	// if !sort.IsSorted(SortedBySize(node.queue)) {
	// 	panic("sort2")
	// }
	// if !sort.IsSorted(SortedBySize(node.sorted)) {
	// 	panic("sort3")
	// }
	node.sorted = sortMerge(SORT_BY_SIZE, node.sorted, node.queue)
	// if !sort.IsSorted(SortedBySize(node.sorted)) {
	// 	panic("sort4")
	// }
	// node.sorted = append(node.sorted, node.queue...)
	// node.sorted = sortFileEntries(SortedBySize(node.sorted)).(SortedBySize)
	node.queue = nil

	inc := len(node.sorted) / numparts
	a := 0
	b := inc
	for i := 0; i < numparts-1; i++ {
		pivot := node.sorted[b]
		node.children = append(node.children, &SizeBucket{
			threshold: SizeThreshold{pivot.size, pivot.name},
			queue:     nil,
			sorted:    node.sorted[a:b],
			children:  nil,
		})
		//fmt.Println("newchild:", SizeThreshold{pivot.size, pivot.name}, node.sorted[a].size, node.sorted[a].name)
		a = b
		b += inc
	}
	node.children = append(node.children, &SizeBucket{
		threshold: node.threshold,
		queue:     nil,
		sorted:    node.sorted[a:len(node.sorted)],
		children:  nil,
	})
	//fmt.Println("lastchild:", node.threshold, a, len(node.sorted))
	node.sorted = nil

	// lensum := 0
	// for _, child := range node.children {
	// 	lensum += len(child.(*SizeBucket).sorted)
	// }
	// fmt.Println("lensum:", lensum)
	// panic("test")
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
