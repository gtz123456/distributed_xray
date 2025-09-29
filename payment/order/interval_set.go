package order

import (
	"github.com/google/btree"
)

// Data structure to store a set of integers and can do Add, Remove, NextMissing operations at O(log n) time
// nextMissing(x) returns the smallest integer >= x that is not in the set
// we use a B-tree to store the intervals of consecutive integers in the set

type interval struct {
	l, r int
}

func (a interval) Less(b btree.Item) bool {
	return a.l < b.(interval).l
}

type IntervalSet struct {
	tree *btree.BTree
}

func NewIntervalSet() *IntervalSet {
	return &IntervalSet{
		tree: btree.New(2),
	}
}

// Add a number to the set
func (s *IntervalSet) Add(x int) {
	iv := interval{x, x}
	// Find the largest interval <= x
	var prev *interval
	s.tree.DescendLessOrEqual(iv, func(it btree.Item) bool {
		p := it.(interval)
		if p.r+1 >= x { // can merge
			prev = &p
		}
		return false
	})

	if prev != nil {
		// remove the previous interval
		s.tree.Delete(*prev)
		// expand the right boundary
		if x > prev.r {
			prev.r = x
		}
		// Check if we can merge with the successor
		var next *interval
		s.tree.AscendGreaterOrEqual(*prev, func(it btree.Item) bool {
			n := it.(interval)
			if prev.r+1 >= n.l {
				next = &n
			}
			return false
		})
		if next != nil {
			s.tree.Delete(*next)
			if next.r > prev.r {
				prev.r = next.r
			}
		}
		s.tree.ReplaceOrInsert(*prev)
	} else {
		// Check if we can merge with the successor
		var next *interval
		s.tree.AscendGreaterOrEqual(iv, func(it btree.Item) bool {
			n := it.(interval)
			if n.l <= x+1 {
				next = &n
			}
			return false
		})
		if next != nil {
			s.tree.Delete(*next)
			if x < next.l {
				next.l = x
			}
			if x > next.r {
				next.r = x
			}
			s.tree.ReplaceOrInsert(*next)
		} else {
			s.tree.ReplaceOrInsert(iv)
		}
	}
}

// Remove a number from the set
func (s *IntervalSet) Remove(x int) {
	iv := interval{x, x}
	var target *interval
	s.tree.DescendLessOrEqual(iv, func(it btree.Item) bool {
		p := it.(interval)
		if p.l <= x && x <= p.r {
			target = &p
		}
		return false
	})
	if target == nil {
		return
	}
	s.tree.Delete(*target)
	if target.l < x {
		s.tree.ReplaceOrInsert(interval{target.l, x - 1})
	}
	if x < target.r {
		s.tree.ReplaceOrInsert(interval{x + 1, target.r})
	}
}

// NextMissing returns the smallest integer >= x that is not in the set
func (s *IntervalSet) NextMissing(x int) int {
	iv := interval{x, x}
	var res int
	found := false
	s.tree.DescendLessOrEqual(iv, func(it btree.Item) bool {
		p := it.(interval)
		if p.l <= x && x <= p.r {
			res = p.r + 1
			found = true
		}
		return false
	})
	if found {
		return res
	}
	// Check if the predecessor interval covers x
	var nxt interval
	got := false
	s.tree.AscendGreaterOrEqual(iv, func(it btree.Item) bool {
		nxt = it.(interval)
		got = true
		return false
	})
	if !got || x < nxt.l {
		return x
	}
	return x
}
