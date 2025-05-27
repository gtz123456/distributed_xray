package order

import "testing"

func TestIntervalSet(t *testing.T) {
	// Create a new interval set
	intervalSet := NewIntervalSet()

	// Add intervals to the set
	intervalSet.Add(1)
	intervalSet.Add(2)
	intervalSet.Add(3)
	intervalSet.Add(5)
	intervalSet.Add(6)
	intervalSet.Add(7)
	intervalSet.Add(9)
	intervalSet.Add(10)

	nextMissing := intervalSet.NextMissing(1)
	if nextMissing != 4 {
		t.Errorf("expected next missing number: 4, got: %d", nextMissing)
	}

	nextMissing = intervalSet.NextMissing(4)
	if nextMissing != 4 {
		t.Errorf("expected next missing number: 4, got: %d", nextMissing)
	}

	nextMissing = intervalSet.NextMissing(5)
	if nextMissing != 8 {
		t.Errorf("expected next missing number: 8, got: %d", nextMissing)
	}

	nextMissing = intervalSet.NextMissing(7)
	if nextMissing != 8 {
		t.Errorf("expected next missing number: 8, got: %d", nextMissing)
	}

	nextMissing = intervalSet.NextMissing(8)
	if nextMissing != 8 {
		t.Errorf("expected next missing number: 8, got: %d", nextMissing)
	}

	nextMissing = intervalSet.NextMissing(11)
	if nextMissing != 11 {
		t.Errorf("expected next missing number: 11, got: %d", nextMissing)
	}

	nextMissing = intervalSet.NextMissing(12)
	if nextMissing != 12 {
		t.Errorf("expected next missing number: 12, got: %d", nextMissing)
	}

}
