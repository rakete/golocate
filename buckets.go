package main

import (
	"fmt"
	//"sort"
	"time"
)

type Threshold interface {
	Less(than Threshold) bool
	String() string
}

type NameThreshold string
type TimeThreshold time.Time
type SizeThreshold int64

func (a NameThreshold) Less(b Threshold) bool {
	if a[0] == '.' {
		return a[1:] < b.(NameThreshold)
	} else {
		return a < b.(NameThreshold)
	}
}

func (a NameThreshold) String() string {
	return string(a)
}

func (a TimeThreshold) Less(b Threshold) bool {
	return time.Time(a).After(time.Time(b.(TimeThreshold)))
}

func (a TimeThreshold) String() string {
	return time.Time(a).Format("2006-01-02 15:04:05")
}

func (a SizeThreshold) Less(b Threshold) bool {
	return a < b.(SizeThreshold)
}

func (a SizeThreshold) String() string {
	if a >= (1000 * 1000 * 1000 * 1000 * 1000) {
		return fmt.Sprintf("%.3fPb", float64(a)/(1000*1000*1000*1000*1000))
	} else if a >= (1000 * 1000 * 1000 * 1000) {
		return fmt.Sprintf("%.3fTb", float64(a)/(1000*1000*1000*1000))
	} else if a >= (1000 * 1000 * 1000) {
		return fmt.Sprintf("%.3fGb", float64(a)/(1000*1000*1000))
	} else if a >= (1000 * 1000) {
		return fmt.Sprintf("%.3fMb", float64(a)/(1000*1000))
	} else if a >= 1000 {
		return fmt.Sprintf("%.3fKb", float64(a)/1000)
	} else {
		return fmt.Sprintf("%db", a)
	}
}

type Node struct {
	threshold Threshold
	queue     []*FileEntry
	sorted    []*FileEntry
	children  []Bucket
}

type Bucket interface {
	Less(entry *FileEntry) bool
	Sort()
	Branch(threshold Threshold, entries []*FileEntry)
	Threshold(i int) Threshold
	Node() *Node
}

type NameBucket Node
type ModTimeBucket Node
type SizeBucket Node

func NewNameBucket() *NameBucket {
	bucket := new(NameBucket)

	for _, char := range "9abcdefghijklmnopqrstuvwxyz" {
		bucket.children = append(bucket.children, &NameBucket{threshold: NameThreshold(char)})
	}
	bucket.children = append(bucket.children, &NameBucket{})

	return bucket
}

func (node *NameBucket) Merge(files []*FileEntry) {
	Insert(node, 0, files)
}

func (node *NameBucket) NumFiles() int {
	num := 0
	for _, child := range node.children {
		num += child.(*NameBucket).NumFiles()
	}
	num += len(node.queue)
	num += len(node.sorted)
	return num
}

func (node *NameBucket) Less(entry *FileEntry) bool {
	return (node.threshold == nil || NameThreshold(entry.name).Less(node.threshold))
}

func (node *NameBucket) Sort() {
	if len(node.queue) > 0 {
		node.queue = sortFileEntries(SortedByName(node.queue)).(SortedByName)
		node.sorted = sortMerge(SORT_BY_NAME, node.sorted, node.queue)
		node.queue = nil
	}

	for _, child := range node.children {
		child.Sort()
	}
}

func (node *NameBucket) Branch(threshold Threshold, entries []*FileEntry) {
	newnode := &NameBucket{
		threshold: threshold,
		sorted:    make([]*FileEntry, len(entries)),
	}
	copy(newnode.sorted, entries)
	node.children = append(node.children, newnode)
}

func (node *NameBucket) Threshold(i int) Threshold {
	if i >= len(node.sorted) {
		return node.threshold
	} else {
		return NameThreshold(node.sorted[i].name)
	}
}

func (node *NameBucket) Node() *Node {
	return (*Node)(node)
}

func NewModTimeBucket() *ModTimeBucket {
	bucket := new(ModTimeBucket)

	now := time.Now()
	second := time.Second
	minute := time.Minute
	hour := time.Hour
	day := time.Hour * 24
	week := day * 7
	year := week * 52
	decade := year * 10

	bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now)})
	bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-second * 10))})
	bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-minute))})
	bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-minute * 10))})
	bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-minute * 20))})
	bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-minute * 30))})
	bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-minute * 40))})
	bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-minute * 50))})

	for i := 1; i < 24; i++ {
		bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-hour * time.Duration(i)))})
	}

	for i := 1; i < 7; i++ {
		bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-day * time.Duration(i)))})
	}

	for i := 1; i < 52; i++ {
		bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-week * time.Duration(i)))})
	}

	for i := 1; i < 10; i++ {
		bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-year * time.Duration(i)))})
	}

	bucket.children = append(bucket.children, &ModTimeBucket{threshold: TimeThreshold(now.Add(-decade))})
	bucket.children = append(bucket.children, &ModTimeBucket{})

	return bucket
}

func (node *ModTimeBucket) Merge(files []*FileEntry) {
	Insert(node, 0, files)
}

func (node *ModTimeBucket) NumFiles() int {
	num := 0
	for _, child := range node.children {
		num += child.(*ModTimeBucket).NumFiles()
	}
	num += len(node.queue)
	num += len(node.sorted)
	return num
}

func (node *ModTimeBucket) Less(entry *FileEntry) bool {
	return (node.threshold == nil || TimeThreshold(entry.modtime).Less(node.threshold))
}

func (node *ModTimeBucket) Sort() {
	if len(node.queue) > 0 {
		node.queue = sortFileEntries(SortedByModTime(node.queue)).(SortedByModTime)
		node.sorted = sortMerge(SORT_BY_MODTIME, node.sorted, node.queue)
		node.queue = nil
	}

	for _, child := range node.children {
		child.Sort()
	}
}

func (node *ModTimeBucket) Branch(threshold Threshold, entries []*FileEntry) {
	newnode := &ModTimeBucket{
		threshold: threshold,
		sorted:    make([]*FileEntry, len(entries)),
	}
	copy(newnode.sorted, entries)
	node.children = append(node.children, newnode)
}

func (node *ModTimeBucket) Threshold(i int) Threshold {
	if i >= len(node.sorted) {
		return node.threshold
	} else {
		return TimeThreshold(node.sorted[i].modtime)
	}
}

func (node *ModTimeBucket) Node() *Node {
	return (*Node)(node)
}

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
	Insert(node, 0, files)
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

func (node *SizeBucket) Less(entry *FileEntry) bool {
	return (node.threshold == nil || SizeThreshold(entry.size).Less(node.threshold))
}

func (node *SizeBucket) Sort() {
	if len(node.queue) > 0 {
		node.queue = sortFileEntries(SortedBySize(node.queue)).(SortedBySize)
		node.sorted = sortMerge(SORT_BY_SIZE, node.sorted, node.queue)
		node.queue = nil
	}

	for _, child := range node.children {
		child.Sort()
	}
}

func (node *SizeBucket) Branch(threshold Threshold, entries []*FileEntry) {
	newnode := &SizeBucket{
		threshold: threshold,
		sorted:    make([]*FileEntry, len(entries)),
	}
	copy(newnode.sorted, entries)
	node.children = append(node.children, newnode)
}

func (node *SizeBucket) Threshold(i int) Threshold {
	if i >= len(node.sorted) {
		return node.threshold.(SizeThreshold)
	} else {
		return SizeThreshold(node.sorted[i].size)
	}
}

func (node *SizeBucket) Node() *Node {
	return (*Node)(node)
}

func WalkEntries(bucket Bucket, direction int, f func(entry *FileEntry) bool) bool {
	node := bucket.Node()

	var indexfunc func(int, int) int
	switch direction {
	case DIRECTION_ASCENDING:
		indexfunc = func(l, i int) int { return i }
	case DIRECTION_DESCENDING:
		indexfunc = func(l, j int) int { return l - 1 - j }
	}

	for i := range node.children {
		child := node.children[indexfunc(len(node.children), i)]
		if len(child.(*SizeBucket).children) > 0 {
			if !WalkEntries(child, direction, f) {
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

func WalkNodes(bucket Bucket, direction int, f func(direction int, node *Node) bool) bool {
	node := bucket.Node()

	var indexfunc func(int, int) int
	switch direction {
	case DIRECTION_ASCENDING:
		indexfunc = func(l, i int) int { return i }
	case DIRECTION_DESCENDING:
		indexfunc = func(l, j int) int { return l - 1 - j }
	}

	for i := range node.children {
		child := node.children[indexfunc(len(node.children), i)]
		if len(child.Node().children) > 0 {
			if !WalkNodes(child, direction, f) {
				return false
			}
		} else {
			f(direction, child.Node())
		}
	}

	return true
}

func Print(bucket Bucket, level int) {
	node := bucket.Node()

	for _, child := range node.children {
		childnode := child.Node()
		for i := 0; i < level; i++ {
			fmt.Print(" ")
		}

		if len(childnode.children) > 0 {
			fmt.Println("parent:", childnode.threshold)
			Print(child, level+1)
		} else {
			if childnode.threshold == nil {
				fmt.Println("maximum", "numfiles:", len(childnode.queue)+len(childnode.sorted))
			} else {
				fmt.Println(childnode.threshold.String(), "numfiles:", len(childnode.queue)+len(childnode.sorted))
			}
		}
	}
}

func Insert(bucket Bucket, first int, files []*FileEntry) int {
	node := bucket.Node()

	i := first
	for _, child := range node.children {
		childnode := child.Node()
		for i < len(files) && child.Less(files[i]) {
			if len(childnode.children) > 0 {
				i = Insert(child, i, files)
			} else {
				childnode.queue = append(childnode.queue, files[i])
				i += 1
			}
		}

		if len(childnode.queue) >= 10000 {
			Split(child, 10)
		}

		if i >= len(files) {
			break
		}
	}

	return i
}

func Split(bucket Bucket, numparts int) {
	// - we want to split this node.sorted slice into numparts parts and create a childnode
	// in node.children for each of them
	node := bucket.Node()

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
	bucket.Sort()

	// - below is an algorithm that tries to split the sorted slice into roughly uniform parts,
	// the general idea is that we can use the entries in the slice itself as new thresholds for
	// the new child nodes that we want to create, the complexity comes mostly from handling edge
	// case where there are lots of file entries with similar sizes, so that we need to adjust the
	// selected thresholds so that the resulting child nodes fulfill the property that all their
	// entries are less then their threshold (not equal!)
	// - endthreshold is needed to decide if entries are the same as the last entry, meaning there is
	// no other threshold to be found among the entries at which the sorted slice can be split and we can
	// just put all those entries in a child and finish
	endthreshold := bucket.Threshold(len(node.sorted) - 1)

	// - we compute inc with which we can increase an index numparts times and split
	// the slice in inc sized parts
	// - then we deal with an edge case where most of the slice contains entries that have the
	// same size: if we cannot split even one inc sized part off from the beginning without
	// its last element not being less then endthreshold, then we conclude that trying to split this
	// slice is pointless and just return, not doing so would result in a very unbalanced subtree
	// with nodes containing very few entries and then one node containing almost all of them which
	// would be further divided, resulting in a very deep subtree
	inc := len(node.sorted) / numparts
	if !bucket.Threshold(inc).Less(endthreshold) {
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

		// - the += inc above may increase b beyond len(node.sorted), this will cause an error
		// when we finally use b in node.sorted[a:b], if we had to increase b so much that it goes
		// beyond len(node.sorted), then we just set it to len(node.sorted) here manually so the
		// Branch call below does not panic
		if b >= len(node.sorted) {
			b = len(node.sorted)
		}

		// - the above edge case when the sorted slice contains almost only entries with the same
		// size can be handled differently then just returning early and not splitting, this if
		// sets b to the smallest possible value if we are not at the end of numparts yet but the
		// entries size at the b index is already not less then endthreshold
		// - I leave it in because it works, but the resulting subtree is quite unbalanced as well,
		// the above early return is simpler and seems to be better
		// if i < numparts-1 && !bucket.Threshold(b).Less(endthreshold) {
		// 	b = a + 1
		// }

		// - to make sure that the b index seperates two slice parts such that there are no entries
		// with equal size that end up in both resulting parts, we increase b when the sizes of the
		// entries at b-1 and b are not less, until they are
		for b < len(node.sorted) && (!bucket.Threshold(b - 1).Less(bucket.Threshold(b))) {
			b += 1
		}

		// - if we are in the last loop iteration, or if all remaining entries have the same size as
		// the last entry, then we set b to len(node.sorted) so that all remaining entries end up
		// in the last part
		if b < len(node.sorted) && (i == numparts-1 || !bucket.Threshold(b).Less(endthreshold)) {
			b = len(node.sorted)
		}

		// - if b is at the end of node.sorted we use the parents node.threshold, the last child
		// always gets its parents threshold
		threshold := bucket.Threshold(b)

		// - create new child, copy entries, set a = b
		bucket.Branch(threshold, node.sorted[a:b])

		a = b
	}

	// - after splitting into parts, we don't need to keep this nodes entries around, they are all in
	// the children now, so clear node.sorted and let it be gc'ed
	if len(node.children) > 0 {
		node.sorted = nil
	}
}
