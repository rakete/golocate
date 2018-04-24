package main

import (
	"sort"
)

type SortColumn int

const (
	SORT_BY_NAME SortColumn = iota
	SORT_BY_DIR
	SORT_BY_MODTIME
	SORT_BY_SIZE
)

type SortedByName []*FileEntry

func (entries SortedByName) Len() int      { return len(entries) }
func (entries SortedByName) Swap(i, j int) { entries[i], entries[j] = entries[j], entries[i] }
func (entries SortedByName) Less(i, j int) bool {
	return entries[i].name < entries[j].name
}

type SortedByDir []*FileEntry

func (entries SortedByDir) Len() int      { return len(entries) }
func (entries SortedByDir) Swap(i, j int) { entries[i], entries[j] = entries[j], entries[i] }
func (entries SortedByDir) Less(i, j int) bool {
	return entries[i].dir[1:] < entries[j].dir[1:]
}

type SortedByModTime []*FileEntry

func (entries SortedByModTime) Len() int      { return len(entries) }
func (entries SortedByModTime) Swap(i, j int) { entries[i], entries[j] = entries[j], entries[i] }
func (entries SortedByModTime) Less(i, j int) bool {
	return entries[i].modtime.After(entries[j].modtime)
}

type SortedBySize []*FileEntry

func (entries SortedBySize) Len() int      { return len(entries) }
func (entries SortedBySize) Swap(i, j int) { entries[i], entries[j] = entries[j], entries[i] }
func (entries SortedBySize) Less(i, j int) bool {
	return entries[i].size > entries[j].size
}


func sortMerge(sortcolumn SortColumn, left, right []*FileEntry) []*FileEntry {
	if len(left) == 0 {
		return right
	}

	if len(right) == 0 {
		return left
	}

	// BEGIN VERSION 0
	// left = append(left, right...)
	// switch sortcolumn {
	// case SORT_BY_NAME:
	// 	sort.Stable(SortedByName(left))
	// case SORT_BY_MODTIME:
	// 	sort.Stable(SortedByModTime(left))
	// case SORT_BY_SIZE:
	// 	sort.Stable(SortedBySize(left))
	// }
	// return left
	// END VERSION 0

	// - I want to use the sort.Interface functions that I defined above for SortedByName, etc, so
	// I had to come up with this convoluted way to use them to compare elements from two different
	// slices, I make a small slice of only the two elements I want to compare, then call Less with
	// 0 and 1 as indices
	testRightBeforeLeft, testLeftBeforeRight := false, false
	xs := []*FileEntry{right[len(right)-1], left[0]}
	ys := []*FileEntry{left[len(left)-1], right[0]}
	switch sortcolumn {
	case SORT_BY_NAME:
		testRightBeforeLeft = SortedByName(xs).Less(0, 1)
		testLeftBeforeRight = SortedByName(ys).Less(0, 1)
	case SORT_BY_DIR:
		testRightBeforeLeft = SortedByDir(xs).Less(0, 1)
		testLeftBeforeRight = SortedByDir(ys).Less(0, 1)
	case SORT_BY_MODTIME:
		testRightBeforeLeft = SortedByModTime(xs).Less(0, 1)
		testLeftBeforeRight = SortedByModTime(ys).Less(0, 1)
	case SORT_BY_SIZE:
		testRightBeforeLeft = SortedBySize(xs).Less(0, 1)
		testLeftBeforeRight = SortedBySize(ys).Less(0, 1)
	}

	// - first two conditions are just early out if the two slices to merge happen to be
	// completely in front or behind each other, then we can just append them and are done
	// - this is so rare it might make sense to just not test it at all
	var result []*FileEntry
	if testRightBeforeLeft { //less(right, len(right)-1, left, 0)
		result = right
		result = append(result, left...)
	} else if testLeftBeforeRight { //less(left, len(left)-1, right, 0)
		result = left
		result = append(result, right...)
	} else {
		resultmem := make([]*FileEntry, len(left)+len(right))
		result = resultmem[:0]

		// - append left and right together into queue, then operate only using indices into queue,
		// that way I can reuse the above sort.Interface functions
		// - notice that queue will stay the same throughout the whole process, the merging is done
		// by building the merged slice in result by appending elements from queue to result, queue
		// is only used to compare elements with the sort.Interface Less function
		var queue sort.Interface
		switch sortcolumn {
		case SORT_BY_NAME:
			queue = SortedByName(append(left, right...))
		case SORT_BY_DIR:
			queue = SortedByDir(append(left, right...))
		case SORT_BY_MODTIME:
			queue = SortedByModTime(append(left, right...))
		case SORT_BY_SIZE:
			queue = SortedBySize(append(left, right...))
		}

		// - these are the indices we'll need, leftqueue marks the start of the (remaining) left slice in
		// queue, rightqueue marks the start of the (remaining) right slice in queue, and rightindex marks
		// how much from the right slice (the orginal one not the one in the queue, so it needs to go from
		// 0 to len(right)) we have already merged into result
		leftqueue := 0
		rightqueue := len(left)
		rightindex := 0

		// - the following loop does the merging, the general idea is that I can use sort.Search binary
		// search to search at which index of the left slice an element from the right slice needs to be
		// inserted, then insert elements from the right slice while they satisfy the less condition, and
		// do this in a loop until all elements from the rigth slice are merged into the left slice
		// - sort.Search takes an amount n and a function that takes an index as argument and returns a bool,
		// then calls that function in a 'binary search pattern', and returns the smallest index for which
		// that function returns true, so n=10 -> f(1) == true, f(10) == false, f(5) == true, f(7) == true,
		// f(8) == false -> returns 7
		// - loop condition is n > 0 which starts as len(left), keeps track how much of the left slice is
		// yet tested with sort.Search and decreases until it is 0 in the worst case (meaning we had to binary
		// search through all of the left slice), or we break early from the loop once all of the right slice
		// is merged
		n := len(left)
		for n > 0 {
			// - rightqueue points to the first element of the right slice in queue, leftqueue to the first
			// element of the left slice in the queue, this construct returns the smallest index where
			// the first element of the right slice is less then an element of the left slice as foundindex
			foundindex := sort.Search(n, func(testindex int) bool {
				return queue.Less(rightqueue, leftqueue+testindex)
			})

			// - when the found index is n then the remaining right slice lies completely behind the remaining
			// left slice, therefore we can just append the remaining left slice to the result, then append
			// the remaining right slice to the result and then we are done
			if foundindex == n {
				result = append(result, left[leftqueue:len(left)]...)
				result = append(result, right[rightindex:len(right)]...)
				break
			}

			// - if we did not early out because foundindex == n, then foundindex points to somewhere
			// in left slice, so we append everything from the left slice up until that foundindex
			// to the result, because those elements are all smaller then the first element of the
			// remaining right slice
			// - in the rare case that foundindex == 0, then the first element of the remaining right
			// slice is less then all of the remaining left slice, so nothing from left needs to be
			// appended to the result
			if foundindex > 0 {
				result = append(result, left[leftqueue:leftqueue+foundindex]...)
			}

			// - now append elements from the remaining right slice one by one as long as they are less
			// then the element at foundindex, increase rightindex AND rightqueue by one each time because
			// we need to keep track of the position of the right slice in queue AND where we are in the
			// right slice (the one outside of queue) because we have to take elements from that one when
			// appending (we can't use the [] operator on queue because its type is sort.Interface, so we
			// use right[rightindex] instead)
			// - append until either an element is not less then the element at foundindex, or the whole
			// right slice has been appended
			for rightindex < len(right) && queue.Less(rightqueue, leftqueue+foundindex) {
				result = append(result, right[rightindex])
				rightindex += 1
				rightqueue += 1
			}

			if rightindex >= len(right) {
				// - when the whole right slice has been merged, then all that is left to do is append the
				// remaining left slice and we are done
				result = append(result, left[leftqueue+foundindex:len(left)]...)
				break
			} else {
				// - otherwise increase the leftqueue index so that it points to the element where we inserted
				// from the right slice so that next iteration we start searching from there, and decrease
				// n accordingly because we only have to search within the remaining elements of the left slice
				leftqueue = leftqueue + foundindex
				n = n - foundindex
			}
		}
	}

	return result
}
