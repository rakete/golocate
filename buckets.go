package main

import (
	"fmt"

	"regexp"
	"sync"

	"sort"
	"time"

	gtk "github.com/gotk3/gotk3/gtk"
)

type Threshold interface {
	Less(than Threshold) bool
	Equal(than Threshold) bool
	String() string
}

type NameThreshold string
type DirThreshold string
type ModTimeThreshold time.Time
type SizeThreshold int64

func (a NameThreshold) Less(b Threshold) bool {
	return a < b.(NameThreshold)
}

func (a NameThreshold) Equal(b Threshold) bool {
	return a == b.(NameThreshold)
}

func (a NameThreshold) String() string {
	return string(a)
}

func (a DirThreshold) Less(b Threshold) bool {
	return a[1:] < b.(DirThreshold)[1:]
}

func (a DirThreshold) Equal(b Threshold) bool {
	return a[1:] == b.(DirThreshold)[1:]
}

func (a DirThreshold) String() string {
	return string(a)
}

func (a ModTimeThreshold) Less(b Threshold) bool {
	return time.Time(a).After(time.Time(b.(ModTimeThreshold)))
}

func (a ModTimeThreshold) Equal(b Threshold) bool {
	return time.Time(a).Equal(time.Time(b.(ModTimeThreshold)))
}

func (a ModTimeThreshold) String() string {
	return time.Time(a).Format("2006-01-02 15:04:05")
}

func (a SizeThreshold) Less(b Threshold) bool {
	return a > b.(SizeThreshold)
}

func (a SizeThreshold) Equal(b Threshold) bool {
	return a == b.(SizeThreshold)
}

func (a SizeThreshold) String() string {
	if a >= (1000 * 1000 * 1000 * 1000 * 1000) {
		return fmt.Sprintf("%.3fP", float64(a)/(1000*1000*1000*1000*1000))
	} else if a >= (1000 * 1000 * 1000 * 1000) {
		return fmt.Sprintf("%.3fT", float64(a)/(1000*1000*1000*1000))
	} else if a >= (1000 * 1000 * 1000) {
		return fmt.Sprintf("%.3fG", float64(a)/(1000*1000*1000))
	} else if a >= (1000 * 1000) {
		return fmt.Sprintf("%.3fM", float64(a)/(1000*1000))
	} else if a >= 1000 {
		return fmt.Sprintf("%.3fK", float64(a)/1000)
	} else {
		return fmt.Sprintf("%db", a)
	}
}

type Node struct {
	threshold   Threshold
	queuemutex  sync.Mutex
	sortedmutex sync.Mutex
	lastchange  time.Time
	queue       []*FileEntry
	sorted      []*FileEntry
	children    []Bucket
}

type Bucket interface {
	Less(entry *FileEntry) bool
	Sort(sortcolumn SortColumn)
	AddBranch(threshold Threshold, entries []*FileEntry)
	ThresholdSplit(i int) Threshold
	Node() *Node
}

func NewNameBucket() *Node {
	bucket := new(Node)

	for _, char := range "@abcdefghijklmnopqrstuvwxyz" {
		bucket.children = append(bucket.children, &Node{
			threshold: NameThreshold(char),
		})
	}

	bucket.children = append(bucket.children, &Node{
		threshold: nil,
	})

	return bucket
}

func NewDirBucket() *Node {
	bucket := new(Node)

	var thresholds []string

	//homedir := os.Getenv("HOME")
	//thresholds = append(thresholds, "/h")
	//thresholds = append(thresholds, path.Join(homedir, ".l"))
	//thresholds = append(thresholds, path.Join(homedir, ".local", "s"))
	//if _, err := os.Stat(path.Join(homedir, ".local", "share", "Zeal", "Zeal", "docsets")); err == nil {
	//	for _, char := range "ELNOSz" {
	//		thresholds = append(thresholds, path.Join(homedir, ".local", "share", "Zeal", "Zeal", "docsets", string(char)))
	//	}
	//}
	//
	//thresholds = append(thresholds, path.Join(homedir, ".local", "share", "z"))
	//thresholds = append(thresholds, path.Join(homedir, ".z"))
	//for _, char := range "Zmz" {
	//	thresholds = append(thresholds, path.Join(homedir, string(char)))
	//}
	//thresholds = append(thresholds, []string{"/t", "/v", "/w"}...)

	for _, char := range thresholds {
		bucket.children = append(bucket.children, &Node{
			threshold: DirThreshold(char),
		})
	}

	bucket.children = append(bucket.children, &Node{
		threshold: nil,
	})

	return bucket
}

func NewModTimeBucket() *Node {
	bucket := new(Node)

	now := time.Now()
	hour := time.Hour
	day := time.Hour * 24
	week := day * 7
	month := week * 4
	year := week * 52
	decade := year * 10

	bucket.children = append(bucket.children, &Node{
		threshold: ModTimeThreshold(now),
	})

	bucket.children = append(bucket.children, &Node{
		threshold: ModTimeThreshold(now.Add(-hour)),
	})

	bucket.children = append(bucket.children, &Node{
		threshold: ModTimeThreshold(now.Add(-day)),
	})

	bucket.children = append(bucket.children, &Node{
		threshold: ModTimeThreshold(now.Add(-week)),
	})

	bucket.children = append(bucket.children, &Node{
		threshold: ModTimeThreshold(now.Add(-month)),
	})

	bucket.children = append(bucket.children, &Node{
		threshold: ModTimeThreshold(now.Add(-year)),
	})

	bucket.children = append(bucket.children, &Node{
		threshold: ModTimeThreshold(now.Add(-decade)),
	})

	bucket.children = append(bucket.children, &Node{
		threshold: nil,
	})

	return bucket
}

func NewSizeBucket() *Node {
	bucket := new(Node)

	bucket.children = append(bucket.children, &Node{
		threshold: SizeThreshold(100000000),
	})
	bucket.children = append(bucket.children, &Node{
		threshold: SizeThreshold(10000000),
	})
	bucket.children = append(bucket.children, &Node{
		threshold: SizeThreshold(1000000),
	})
	bucket.children = append(bucket.children, &Node{
		threshold: SizeThreshold(100000),
	})
	bucket.children = append(bucket.children, &Node{
		threshold: SizeThreshold(10000),
	})
	// <4097 are very popular file sizes
	bucket.children = append(bucket.children, &Node{
		threshold: SizeThreshold(4097),
	})
	bucket.children = append(bucket.children, &Node{
		threshold: SizeThreshold(1000),
	})
	bucket.children = append(bucket.children, &Node{
		threshold: SizeThreshold(100),
	})
	bucket.children = append(bucket.children, &Node{
		threshold: SizeThreshold(1),
	})

	bucket.children = append(bucket.children, &Node{
		threshold: nil,
	})

	return bucket
}

func (node *Node) Merge(sortcolumn SortColumn, files []*FileEntry) {
	Insert(sortcolumn, node, 0, files)
}

func (node *Node) Take(cache MatchCaches, sortcolumn SortColumn, direction gtk.SortType, query *regexp.Regexp, n int, abort chan struct{}, results chan *FileEntry) {
	var indexfunc func(int, int) int
	switch direction {
	case gtk.SORT_ASCENDING:
		indexfunc = func(l, i int) int { return i }
	case gtk.SORT_DESCENDING:
		indexfunc = func(l, j int) int { return l - 1 - j }
	}

	numresults := 0
	var namecache, dircache Cache
	if cache.names != nil {
		namecache = cache.names
	} else {
		namecache = NewSimpleCache()
	}

	if cache.dirs != nil {
		dircache = cache.dirs
	} else {
		dircache = NewSimpleCache()
	}

	WalkNodes(node, direction, func(child Bucket) bool {
		if child == nil {
			results <- nil
			return true
		}

		childnode := child.Node()

		defer childnode.sortedmutex.Unlock()
		childnode.sortedmutex.Lock()

		child.Sort(sortcolumn)

		sorted := childnode.sorted
		l := len(sorted)

		for i := 0; i < len(sorted); i++ {
			select {
			case <-abort:
				return false
			default:
				index := indexfunc(l, i)
				entry := sorted[index]

				matchedname, matcheddir := testMatchCaches(dircache, namecache, entry, query)

				if query == nil || matchedname || matcheddir {
					results <- entry
					numresults += 1
				}

				if numresults >= n {
					results <- nil
					return false
				}
			}
		}

		return true
	})
}

func (node *Node) Remove(sortcolumn SortColumn, files []*FileEntry) {
	Delete(sortcolumn, node, 0, files)
}

func (node *Node) NumFiles() int {
	num := 0
	for _, child := range node.children {
		num += child.(*Node).NumFiles()
	}
	num += len(node.queue)
	num += len(node.sorted)
	return num
}

func (node *Node) Less(entry *FileEntry) bool {
	if node.threshold != nil {
		switch node.threshold.(type) {
		case NameThreshold:
			return NameThreshold(entry.name).Less(node.threshold)
		case DirThreshold:
			return DirThreshold(entry.dir).Less(node.threshold)
		case ModTimeThreshold:
			return ModTimeThreshold(entry.modtime).Less(node.threshold)
		case SizeThreshold:
			return SizeThreshold(entry.size).Less(node.threshold)
		}
	}

	return true
}

func (node *Node) Sort(sortcolumn SortColumn) {
	if len(node.queue) > 0 {
		switch sortcolumn {
		case SORT_BY_NAME:
			sort.Stable(SortedByName(node.queue))
		case SORT_BY_DIR:
			sort.Stable(SortedByDir(node.queue))
		case SORT_BY_MODTIME:
			sort.Stable(SortedByModTime(node.queue))
		case SORT_BY_SIZE:
			sort.Stable(SortedBySize(node.queue))
		}
		node.sorted = sortMerge(sortcolumn, node.sorted, node.queue)
		node.queue = node.queue[:0]
	}

	for _, child := range node.children {
		child.Sort(sortcolumn)
	}
}

func (node *Node) AddBranch(threshold Threshold, entries []*FileEntry) {
	newnode := &Node{
		threshold:  threshold,
		lastchange: time.Now(),
		sorted:     make([]*FileEntry, len(entries)),
	}
	copy(newnode.sorted, entries)
	node.children = append(node.children, newnode)
}

func (node *Node) ThresholdSplit(i int) Threshold {
	if i >= len(node.sorted) {
		return node.threshold
	}

	switch node.threshold.(type) {
	case NameThreshold:
		return NameThreshold(node.sorted[i].name)
	case DirThreshold:
		return DirThreshold(node.sorted[i].dir)
	case ModTimeThreshold:
		return ModTimeThreshold(node.sorted[i].modtime)
	case SizeThreshold:
		return SizeThreshold(node.sorted[i].size)
	}

	return node.threshold
}

func (node *Node) Node() *Node {
	return node
}

func WalkEntries(bucket Bucket, direction gtk.SortType, f func(entry *FileEntry) bool) bool {
	return WalkEntriesRecur(nil, bucket, direction, f)
}

func WalkEntriesRecur(parent Bucket, bucket Bucket, direction gtk.SortType, f func(entry *FileEntry) bool) bool {
	node := bucket.Node()

	var indexfunc func(int, int) int
	switch direction {
	case gtk.SORT_ASCENDING:
		indexfunc = func(l, i int) int { return i }
	case gtk.SORT_DESCENDING:
		indexfunc = func(l, j int) int { return l - 1 - j }
	}

	node.queuemutex.Lock()
	if len(node.children) > 0 {
		for i := range node.children {
			child := node.children[indexfunc(len(node.children), i)]
			if len(child.(*Node).children) > 0 {
				node.queuemutex.Unlock()
				if !WalkEntriesRecur(node, child, direction, f) {
					return false
				}
				node.queuemutex.Lock()
			} else {
				sorted := child.(*Node).sorted
				for j := range sorted {
					entry := sorted[indexfunc(len(sorted), j)]
					if !f(entry) {
						node.queuemutex.Unlock()
						return false
					}
				}
			}
		}
	} else {
		sorted := node.sorted
		for j := range sorted {
			entry := sorted[indexfunc(len(sorted), j)]
			if !f(entry) {
				node.queuemutex.Unlock()
				return false
			}
		}
	}

	ret := true
	if parent == nil {
		ret = f(nil)
	}
	node.queuemutex.Unlock()
	return ret
}

func WalkNodes(bucket Bucket, direction gtk.SortType, f func(bucket Bucket) bool) bool {
	return WalkNodesRecur(nil, bucket, direction, f)
}

func WalkNodesRecur(parent Bucket, bucket Bucket, direction gtk.SortType, f func(bucket Bucket) bool) bool {
	node := bucket.Node()

	var indexfunc func(int, int) int
	switch direction {
	case gtk.SORT_ASCENDING:
		indexfunc = func(l, i int) int { return i }
	case gtk.SORT_DESCENDING:
		indexfunc = func(l, j int) int { return l - 1 - j }
	}

	node.queuemutex.Lock()
	if len(node.children) > 0 {
		for i := range node.children {
			child := node.children[indexfunc(len(node.children), i)]
			if len(child.Node().children) > 0 {
				node.queuemutex.Unlock()
				if !WalkNodesRecur(node, child, direction, f) {
					return false
				}
				node.queuemutex.Lock()
			} else {
				if !f(child) {
					node.queuemutex.Unlock()
					return false
				}
			}
		}
	} else {
		if !f(node) {
			node.queuemutex.Unlock()
			return false
		}
	}

	ret := true
	if parent == nil {
		ret = f(nil)
	}
	node.queuemutex.Unlock()
	return ret
}

func PrintBucket(bucket Bucket, level int) {
	node := bucket.Node()

	for _, child := range node.children {
		childnode := child.Node()

		if level < 0 {
			if childnode.threshold == nil {
				fmt.Println("maximum", "numfiles:", childnode.NumFiles())
			} else {
				fmt.Println(childnode.threshold.String(), "numfiles:", childnode.NumFiles())
			}
		} else {
			for i := 0; i < level; i++ {
				fmt.Print(" ")
			}

			if len(childnode.children) > 0 {
				fmt.Println("parent:", childnode.threshold, "numfiles:", childnode.NumFiles())
				PrintBucket(child, level+1)
			} else {
				if childnode.threshold == nil {
					fmt.Println("maximum", "numfiles:", len(childnode.queue)+len(childnode.sorted))
				} else {
					fmt.Println(childnode.threshold.String(), "numfiles:", len(childnode.queue)+len(childnode.sorted))
				}
			}
		}

	}
}

const (
	SPLIT_ENTRYTHRESHOLD int = 10000
	SPLIT_NUMPARTS       int = 10
)

func Insert(sortcolumn SortColumn, bucket Bucket, first int, files []*FileEntry) int {
	node := bucket.Node()
	node.lastchange = time.Now()

	i := first
childrenloop:
	for _, child := range node.children {
		childnode := child.Node()

		childnode.queuemutex.Lock()

		for i < len(files) && child.Less(files[i]) {
			if len(childnode.children) > 0 {
				i = Insert(sortcolumn, child, i, files)
			} else {
				childnode.lastchange = time.Now()
				childnode.queue = append(childnode.queue, files[i])
				i += 1
			}
		}

		if len(childnode.queue) >= SPLIT_ENTRYTHRESHOLD {
			childnode.sortedmutex.Lock()
			Split(sortcolumn, child, SPLIT_NUMPARTS)
			childnode.sortedmutex.Unlock()
		}
		childnode.queuemutex.Unlock()

		if i >= len(files) {
			break childrenloop
		}
	}

	return i
}

func Delete(sortcolumn SortColumn, bucket Bucket, first int, files []*FileEntry) int {
	node := bucket.Node()
	node.lastchange = time.Now()

	searchfunc := func(a, b *FileEntry) bool {
		aname := NameThreshold(a.name)
		bname := NameThreshold(b.name)
		adir := DirThreshold(a.dir)
		bdir := DirThreshold(b.dir)

		switch sortcolumn {
		case SORT_BY_NAME:
			return !aname.Less(bname) || (aname.Equal(bname) && adir.Equal(bdir))
		case SORT_BY_DIR:
			return !adir.Less(bdir) || (aname.Equal(bname) && adir.Equal(bdir))
		case SORT_BY_MODTIME:
			amodtime := ModTimeThreshold(a.modtime)
			bmodtime := ModTimeThreshold(b.modtime)
			return (amodtime.Equal(bmodtime) || !amodtime.Less(bmodtime)) || (aname.Equal(bname) && adir.Equal(bdir))
		case SORT_BY_SIZE:
			asize := SizeThreshold(a.size)
			bsize := SizeThreshold(b.size)
			return (asize.Equal(bsize) || !asize.Less(bsize)) || (aname.Equal(bname) && adir.Equal(bdir))
		}

		return true
	}

	i := first
childrenloop:
	for _, child := range node.children {
		childnode := child.Node()

		childnode.queuemutex.Lock()
		child.Sort(sortcolumn)
		childnode.queuemutex.Unlock()

		childnode.sortedmutex.Lock()
		start := 0
		newsorted := childnode.sorted[:0]
		for i < len(files) && child.Less(files[i]) {
			if len(childnode.children) > 0 {
				i = Delete(sortcolumn, child, i, files)
			} else {
				n := len(childnode.sorted) - start
				amount := sort.Search(n, func(testindex int) bool {
					a := childnode.sorted[start+testindex]
					b := files[i]
					ret := searchfunc(a, b)
					return ret
				})

				if amount == n {
					panic("this should not happen")
				}

				if amount >= 0 {
					childnode.lastchange = time.Now()
					newsorted = append(newsorted, childnode.sorted[start:start+amount]...)
				}

				start += amount + 1
				i += 1
			}
		}

		if start > 0 {
			newsorted = append(newsorted, childnode.sorted[start:len(childnode.sorted)]...)
			childnode.sorted = newsorted
		}
		childnode.sortedmutex.Unlock()

		if i >= len(files) {
			break childrenloop
		}
	}

	return i
}

func Split(sortcolumn SortColumn, bucket Bucket, numparts int) {
	// - we want to split this node.sorted slice into numparts parts and create a childnode
	// in node.children for each of them
	node := bucket.Node()
	node.lastchange = time.Now()

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
	bucket.Sort(sortcolumn)

	// - below is an algorithm that tries to split the sorted slice into roughly uniform parts,
	// the general idea is that we can use the entries in the slice itself as new thresholds for
	// the new child nodes that we want to create, the complexity comes mostly from handling edge
	// case where there are lots of file entries with similar sizes, so that we need to adjust the
	// selected thresholds so that the resulting child nodes fulfill the property that all their
	// entries are less then their threshold (not equal!)
	// - endthreshold is needed to decide if entries are the same as the last entry, meaning there is
	// no other threshold to be found among the entries at which the sorted slice can be split and we can
	// just put all those entries in a child and finish
	endthreshold := bucket.ThresholdSplit(len(node.sorted) - 1)

	// - we compute inc with which we can increase an index numparts times and split
	// the slice in inc sized parts
	// - then we deal with an edge case where most of the slice contains entries that have the
	// same size: if we cannot split even one inc sized part off from the beginning without
	// its last element not being less then endthreshold, then we conclude that trying to split this
	// slice is pointless and just return, not doing so would result in a very unbalanced subtree
	// with nodes containing very few entries and then one node containing almost all of them which
	// would be further divided, resulting in a very deep subtree
	inc := len(node.sorted) / numparts
	incthreshold := bucket.ThresholdSplit(inc)
	if (incthreshold != nil && incthreshold.Equal(endthreshold)) || (incthreshold == nil && endthreshold == nil) {
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
		for b < len(node.sorted) && (!bucket.ThresholdSplit(b - 1).Less(bucket.ThresholdSplit(b))) {
			b += 1
		}

		// - if we are in the last loop iteration, or if all remaining entries have the same size as
		// the last entry, then we set b to len(node.sorted) so that all remaining entries end up
		// in the last part
		if b < len(node.sorted) && (i == numparts-1 || !bucket.ThresholdSplit(b).Less(endthreshold)) {
			b = len(node.sorted)
		}

		// - if b is at the end of node.sorted we use the parents node.threshold, the last child
		// always gets its parents threshold
		threshold := bucket.ThresholdSplit(b)

		// - create new child, copy entries, set a = b
		bucket.AddBranch(threshold, node.sorted[a:b])

		a = b
	}

	// - after splitting into parts, we don't need to keep this nodes entries around, they are all in
	// the children now, so clear node.sorted and let it be gc'ed
	if len(node.children) > 0 {
		node.sorted = nil
	}
}
