package main

import (
	"sort"
)

const (
	SORT_BY_NAME = iota
	SORT_BY_MODTIME
	SORT_BY_SIZE
)

type ByName []FileEntry

func (a ByName) Len() int      { return len(a) }
func (a ByName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool {
	return a[i].name < a[j].name
}

type ByModTime []FileEntry

func (a ByModTime) Len() int      { return len(a) }
func (a ByModTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByModTime) Less(i, j int) bool {
	return a[i].modtime.Before(a[j].modtime)
}

type BySize []FileEntry

func (a BySize) Len() int      { return len(a) }
func (a BySize) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a BySize) Less(i, j int) bool {
	return a[i].size < a[j].size
}

func sortFileEntries(files sort.Interface) sort.Interface {
	sort.Stable(files)
	return files
}

func sortMerge(sorttype int, left, right []FileEntry) []FileEntry {
	if len(left) == 0 {
		return right
	}

	if len(right) == 0 {
		return left
	}

	testRightBeforeLeft, testLeftBeforeRight := false, false
	xs := []FileEntry{right[len(right)-1], left[0]}
	ys := []FileEntry{left[len(left)-1], right[0]}
	switch sorttype {
	case SORT_BY_NAME:
		testRightBeforeLeft = ByName(xs).Less(0, 1)
		testLeftBeforeRight = ByName(ys).Less(0, 1)
	case SORT_BY_MODTIME:
		testRightBeforeLeft = ByModTime(xs).Less(0, 1)
		testLeftBeforeRight = ByModTime(ys).Less(0, 1)
	case SORT_BY_SIZE:
		testRightBeforeLeft = BySize(xs).Less(0, 1)
		testLeftBeforeRight = BySize(ys).Less(0, 1)
	}

	var result []FileEntry

	if testRightBeforeLeft { //less(right, len(right)-1, left, 0)
		result = append(right, left...)
	} else if testLeftBeforeRight { //less(left, len(left)-1, right, 0)
		result = append(left, right...)
	} else {
		// result = append(left, right...)
		// sort.Stable(ByName(result))

		var queue sort.Interface
		switch sorttype {
		case SORT_BY_NAME:
			queue = ByName(append(left, right...))
		case SORT_BY_MODTIME:
			queue = ByModTime(append(left, right...))
		case SORT_BY_SIZE:
			queue = BySize(append(left, right...))
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
