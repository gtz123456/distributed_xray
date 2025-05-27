package order

import "sort"

// store the the actual amounts as intervals, to binarily search for the next missing amount
type Interval struct {
	start, end int
}

type IntervalSet struct {
	intervals []Interval
}

func NewIntervalSet() *IntervalSet {
	return &IntervalSet{
		intervals: []Interval{},
	}
}

func (s *IntervalSet) Add(x int) {
	pos := sort.Search(len(s.intervals), func(i int) bool {
		return s.intervals[i].start > x
	})

	// Check for possible merging with left and right intervals
	mergeLeft := pos > 0 && s.intervals[pos-1].end+1 >= x
	mergeRight := pos < len(s.intervals) && s.intervals[pos].start-1 <= x

	if mergeLeft && mergeRight {
		left := s.intervals[pos-1]
		right := s.intervals[pos]
		newInterval := Interval{start: left.start, end: max(right.end, x)}
		// Remove left and right
		s.intervals = append(s.intervals[:pos-1], s.intervals[pos+1:]...)
		// Insert merged
		s.intervals = insertInterval(s.intervals, newInterval)
	} else if mergeLeft {
		left := s.intervals[pos-1]
		newInterval := Interval{start: left.start, end: max(left.end, x)}
		s.intervals = append(s.intervals[:pos-1], s.intervals[pos:]...)
		s.intervals = insertInterval(s.intervals, newInterval)
	} else if mergeRight {
		right := s.intervals[pos]
		newInterval := Interval{start: min(right.start, x), end: right.end}
		s.intervals = append(s.intervals[:pos], s.intervals[pos+1:]...)
		s.intervals = insertInterval(s.intervals, newInterval)
	} else {
		newInterval := Interval{start: x, end: x}
		s.intervals = insertInterval(s.intervals, newInterval)
	}
}

// find the first available amount greater than or equal to x
func (s *IntervalSet) NextMissing(x int) int {
	pos := sort.Search(len(s.intervals), func(i int) bool {
		return s.intervals[i].start > x
	})

	if pos > 0 {
		prev := s.intervals[pos-1]
		if x >= prev.start && x <= prev.end {
			return prev.end + 1
		}
	}
	return x
}

func insertInterval(arr []Interval, val Interval) []Interval {
	pos := sort.Search(len(arr), func(i int) bool {
		return arr[i].start > val.start
	})
	arr = append(arr, Interval{}) // extend slice
	copy(arr[pos+1:], arr[pos:])  // shift right
	arr[pos] = val                // insert
	return arr
}
