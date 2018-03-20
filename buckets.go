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
type SizeThreshold int64

func (a NameThreshold) Less(b Threshold) bool {
	return a < b.(NameThreshold)
}

func (a TimeThreshold) Less(b Threshold) bool {
	return a < b.(TimeThreshold)
}

func (a SizeThreshold) Less(b Threshold) bool {
	return a < b.(SizeThreshold)
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
	WalkEntries(direction int, f func(entry *FileEntry) bool) bool
	WalkNodes(direction int, f func(direction int, node *Node) bool) bool
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
func (node *NameBucket) NumFiles() int                            { return 0 }
func (node *NameBucket) Insert(first int, files []*FileEntry) int { return 0 }
func (node *NameBucket) Split(numparts int)                       {}
func (node *NameBucket) WalkEntries(direction int, f func(entry *FileEntry) bool) bool {
	return false
}
func (node *NameBucket) WalkNodes(direction int, f func(direction int, node *Node) bool) bool {
	return false
}
func (node *NameBucket) Print(level int) {}

func NewTimeBucket() *TimeBucket {
	bucket := new(TimeBucket)

	return bucket
}

func (node *TimeBucket) Merge(files []*FileEntry)                 {}
func (node *TimeBucket) NumFiles() int                            { return 0 }
func (node *TimeBucket) Insert(first int, files []*FileEntry) int { return 0 }
func (node *TimeBucket) Split(numparts int)                       {}
func (node *TimeBucket) WalkEntries(direction int, f func(entry *FileEntry) bool) bool {
	return false
}
func (node *TimeBucket) WalkNodes(direction int, f func(direction int, node *Node) bool) bool {
	return false
}
func (node *TimeBucket) Print(level int) {}

func NewSizeBucket() *SizeBucket {
	bucket := new(SizeBucket)

	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(1)})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(10)})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(100)})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(1000)})
	// <4096 and <4097 are very popular file sizes
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(4096)})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(4097)})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(10000)})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(100000)})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(1000000)})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(10000000)})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(100000000)})
	bucket.children = append(bucket.children, &SizeBucket{threshold: SizeThreshold(1000000000)})
	bucket.children = append(bucket.children, &SizeBucket{})

	return bucket
}

func (node *SizeBucket) Merge(files []*FileEntry) {
	node.Insert(0, files)
}

func (node *SizeBucket) NumFiles() int {
	num := 0
	for _, child := range node.children {
		num += child.(*SizeBucket).NumFiles()
	}
	num += len(node.queue)
	num += len(node.sorted)
	return num
}

func (node *SizeBucket) Insert(first int, files []*FileEntry) int {
	i := first
	for _, childnode := range node.children {
		childthreshold := childnode.(*SizeBucket).threshold
		for i < len(files) && (childthreshold == nil || SizeThreshold(files[i].size).Less(childthreshold)) {
			if len(childnode.(*SizeBucket).children) > 0 {
				i = childnode.Insert(i, files)
			} else {
				childnode.(*SizeBucket).queue = append(childnode.(*SizeBucket).queue, files[i])
				i += 1
			}
		}

		if len(childnode.(*SizeBucket).queue) >= 10000 {
			childnode.Split(10)
		}

		if i >= len(files) {
			break
		}
	}

	return i
}

func (node *SizeBucket) Split(numparts int) {
	// - we want to split this node.sorted slice into numparts parts and create a childnode
	// in node.children for each of them

	// - its an expensive operation, so we only do it when we have to, in node.queue we accumulate
	// entries and split once we have enough entries accumulated (this check is done outside of this
	// function), but we test if there are at least as many entries in the queue as as there are
	// parts to split into
	if len(node.queue) < numparts {
		return
	}

	// - a node that already has children, has already been split, and does not need to be split
	// again
	if len(node.children) > 0 {
		return
	}

	// - sorted is already sorted, but queue is just appended to, so we sort queue and then merge
	// sorted and queue, afterwards we can discard the queue
	node.queue = sortFileEntries(SortedBySize(node.queue)).(SortedBySize)
	node.sorted = sortMerge(SORT_BY_SIZE, node.sorted, node.queue)
	node.queue = nil

	// - below is an algorithm that tries to split the sorted slice into roughly uniform parts,
	// the general idea is that we can use the entries in the slice itself as new thresholds for
	// the new child nodes that we want to create, the complexity comes mostly from handling edge
	// case where there are lots of file entries with similar sizes, so that we need to adjust the
	// selected thresholds so that the resulting child nodes fulfill the property that all their
	// entries are less then their threshold (not equal!)
	// - endthreshold is needed to decide if entries are the same as the last entry, meaning there is
	// no other threshold to be found among the entries at which the sorted slice can be split and we can
	// just put all those entries in a child and finish
	endsize := node.sorted[len(node.sorted)-1].size
	endthreshold := SizeThreshold(endsize)

	// - we compute inc with which we can increase an index numparts times and split
	// the slice in inc sized parts
	// - then we deal with an edge case where most of the slice contains entries that have the
	// same size: if we cannot split even one inc sized part off from the beginning without
	// its last element not being less then endthreshold, then we conclude that trying to split this
	// slice is pointless and just return, not doing so would result in a very unbalanced subtree
	// with nodes containing very few entries and then one node containing almost all of them which
	// would be further divided, resulting in a very deep subtree
	inc := len(node.sorted) / numparts
	if !SizeThreshold(node.sorted[inc].size).Less(endthreshold) {
		return
	}

	// - we keep track of two indices, a which marks the front of a part and b which marks the end
	// - the loop condition is either we split sorted into numparts or b is beyond the end node.sorted
	a, b := 0, 1
	for i := 0; i < numparts && b < len(node.sorted); i++ {
		// - a can end up being much larger then this loops b because it is set to previous loops b,
		// so we'll have to make sure that this loops b is actually larger then a
		b = (i + 1) * inc
		for b <= a {
			b += inc
		}

		// - the above edge case when the sorted slice contains almost only entries with the same
		// size can be handled differently then just returning early and not splitting, this if
		// sets b to the smallest possible value if we are not at the end of numparts yet but the
		// entries size at the b index is already not less then endthreshold
		// - I leave it in because it works, but the resulting subtree is quite unbalanced as well,
		// the above early return is simpler and seems to be better
		// if i < numparts-1 && !SizeThreshold(node.sorted[b].size).Less(endthreshold) {
		// 	b = a + 1
		// }

		// - to make sure that the b index seperates two slice parts such that there are no entries
		// with equal size that end up in both resulting parts, we increase b when the sizes of the
		// entries at b-1 and b are not less, until they are
		for b < len(node.sorted) && (!SizeThreshold(node.sorted[b-1].size).Less(SizeThreshold(node.sorted[b].size))) {
			b += 1
		}

		// - if we are in the last loop iteration, or if all remaining entries have the same size as
		// the last entry, then we set b to len(node.sorted) so that all remaining entries end up
		// in the last part
		if b < len(node.sorted) && (i == numparts-1 || !SizeThreshold(node.sorted[b].size).Less(endthreshold)) {
			b = len(node.sorted)
		}

		// - if b is at the end of node.sorted we use the parents node.threshold, the last child
		// always gets its parents threshold
		var threshold SizeThreshold
		if b >= len(node.sorted) {
			threshold = node.threshold.(SizeThreshold)
		} else {
			threshold = SizeThreshold(node.sorted[b].size)
		}

		// - create new child, copy entries, set a = b
		newnode := &SizeBucket{
			threshold: threshold,
			sorted:    make([]*FileEntry, b-a),
		}
		copy(newnode.sorted, node.sorted[a:b])
		node.children = append(node.children, newnode)

		a = b
	}

	// - after splitting into parts, we don't need to keep this nodes entries around, they are all in
	// the children now, so clear node.sorted and let it be gc'ed
	node.sorted = nil
}

func (node *SizeBucket) WalkEntries(direction int, f func(entry *FileEntry) bool) bool {
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
			if !child.WalkEntries(direction, f) {
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

func (node *SizeBucket) WalkNodes(direction int, f func(direction int, node *Node) bool) bool {
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
			if !child.WalkNodes(direction, f) {
				return false
			}
		} else {
			f(direction, (*Node)(child.(*SizeBucket)))
		}
	}

	return true
}

func (node *SizeBucket) Print(level int) {
	for _, child := range node.children {
		for i := 0; i < level; i++ {
			fmt.Print(" ")
		}
		if len(child.(*SizeBucket).children) > 0 {
			fmt.Println("parent:", child.(*SizeBucket).threshold.(SizeThreshold))
			child.Print(level + 1)
		} else {
			if child.(*SizeBucket).threshold == nil {
				fmt.Println("maximum", "numfiles:", len(child.(*SizeBucket).queue)+len(child.(*SizeBucket).sorted))
			} else {
				fmt.Println(child.(*SizeBucket).threshold.(SizeThreshold), "numfiles:", len(child.(*SizeBucket).queue)+len(child.(*SizeBucket).sorted))
			}
		}
	}
}
