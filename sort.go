package main

import (
	"sort"
)

const (
	SORT_BY_NAME = iota
	SORT_BY_MODTIME
	SORT_BY_SIZE
)

type SortedByName []*FileEntry

func (a SortedByName) Len() int      { return len(a) }
func (a SortedByName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a SortedByName) Less(i, j int) bool {
	return a[i].name < a[j].name
}

type SortedByModTime []*FileEntry

func (a SortedByModTime) Len() int      { return len(a) }
func (a SortedByModTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a SortedByModTime) Less(i, j int) bool {
	return a[i].modtime < a[j].modtime
}

type SortedBySize []*FileEntry

func (a SortedBySize) Len() int      { return len(a) }
func (a SortedBySize) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a SortedBySize) Less(i, j int) bool {
	return a[i].size < a[j].size
}

func sortFileEntries(files sort.Interface) sort.Interface {
	sort.Stable(files)
	return files
}

func sortMerge(sorttype int, left, right []*FileEntry) []*FileEntry {
	if len(left) == 0 {
		return right
	}

	if len(right) == 0 {
		return left
	}

	// BEGIN VERSION 0
	// left = append(left, right...)
	// switch sorttype {
	// case SORT_BY_NAME:
	// 	sort.Stable(SortedByName(left))
	// case SORT_BY_MODTIME:
	// 	sort.Stable(SortedByModTime(left))
	// case SORT_BY_SIZE:
	// 	sort.Stable(SortedBySize(left))
	// }
	// return left
	// END VERSION 0

	// BEGIN VERSION 1
	// var result []*FileEntry

	// var queue sort.Interface
	// switch sorttype {
	// case SORT_BY_NAME:
	// 	queue = SortedByName(append(left, right...))
	// case SORT_BY_MODTIME:
	// 	queue = SortedByModTime(append(left, right...))
	// case SORT_BY_SIZE:
	// 	queue = SortedBySize(append(left, right...))
	// }

	// leftqueue := 0
	// rightqueue := len(left)
	// rightindex := 0

	// n := len(left)
	// for n > 0 {
	// 	foundindex := sort.Search(n, func(testindex int) bool {
	// 		return queue.Less(rightqueue, leftqueue+testindex)
	// 	})

	// 	if foundindex == n {
	// 		result = append(result, left[leftqueue:len(left)]...)
	// 		result = append(result, right[rightindex:len(right)]...)
	// 		break
	// 	}

	// 	result = append(result, left[leftqueue:leftqueue+foundindex]...)

	// 	for rightindex < len(right) && queue.Less(rightqueue, leftqueue+foundindex) {
	// 		result = append(result, right[rightindex])
	// 		rightindex += 1
	// 		rightqueue += 1
	// 	}

	// 	if rightindex >= len(right) {
	// 		result = append(result, left[leftqueue+foundindex:len(left)]...)
	// 		break
	// 	} else {
	// 		leftqueue = leftqueue + foundindex
	// 		n = n - foundindex
	// 	}
	// }
	// END VERSION 1

	testRightBeforeLeft, testLeftBeforeRight := false, false
	xs := []*FileEntry{right[len(right)-1], left[0]}
	ys := []*FileEntry{left[len(left)-1], right[0]}
	switch sorttype {
	case SORT_BY_NAME:
		testRightBeforeLeft = SortedByName(xs).Less(0, 1)
		testLeftBeforeRight = SortedByName(ys).Less(0, 1)
	case SORT_BY_MODTIME:
		testRightBeforeLeft = SortedByModTime(xs).Less(0, 1)
		testLeftBeforeRight = SortedByModTime(ys).Less(0, 1)
	case SORT_BY_SIZE:
		testRightBeforeLeft = SortedBySize(xs).Less(0, 1)
		testLeftBeforeRight = SortedBySize(ys).Less(0, 1)
	}

	var result []*FileEntry
	if testRightBeforeLeft { //less(right, len(right)-1, left, 0)
		result = append(right, left...)
	} else if testLeftBeforeRight { //less(left, len(left)-1, right, 0)
		result = append(left, right...)
	} else {
		var queue sort.Interface
		switch sorttype {
		case SORT_BY_NAME:
			queue = SortedByName(append(left, right...))
		case SORT_BY_MODTIME:
			queue = SortedByModTime(append(left, right...))
		case SORT_BY_SIZE:
			queue = SortedBySize(append(left, right...))
		}

		leftqueue := 0
		rightqueue := len(left)
		rightindex := 0

		n := len(left)
		for n > 0 {
			foundindex := sort.Search(n, func(testindex int) bool {
				return queue.Less(rightqueue, leftqueue+testindex)
			})

			if foundindex == n {
				result = append(result, left[leftqueue:len(left)]...)
				result = append(result, right[rightindex:len(right)]...)
				break
			}

			result = append(result, left[leftqueue:leftqueue+foundindex]...)

			for rightindex < len(right) && queue.Less(rightqueue, leftqueue+foundindex) {
				result = append(result, right[rightindex])
				rightindex += 1
				rightqueue += 1
			}

			if rightindex >= len(right) {
				result = append(result, left[leftqueue+foundindex:len(left)]...)
				break
			} else {
				leftqueue = leftqueue + foundindex
				n = n - foundindex
			}
		}
	}

	return result
}
