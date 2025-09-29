package order_test

import (
	"go-distributed/payment/order"
	"testing"
)

func TestIntervalSet(t *testing.T) {
	s := order.NewIntervalSet()
	s.Add(5000000)
	if s.NextMissing(5000000) != 5000001 {
		t.Error("Expected 5000001, got", s.NextMissing(5000000))
	}

	s.Add(5000001)
	if s.NextMissing(5000000) != 5000002 {
		t.Error("Expected 5000002, got", s.NextMissing(5000000))
	}

	s.Add(5000003)
	if s.NextMissing(5000000) != 5000002 {
		t.Error("Expected 5000002, got", s.NextMissing(5000000))
	}

	s.Remove(5000001)
	if s.NextMissing(5000000) != 5000001 {
		t.Error("Expected 5000001, got", s.NextMissing(5000000))
	}

	s.Remove(5000000)
	if s.NextMissing(5000000) != 5000000 {
		t.Error("Expected 5000000, got", s.NextMissing(5000000))
	}

	s.Add(5000000)
	if s.NextMissing(5000000) != 5000001 {
		t.Error("Expected 5000001, got", s.NextMissing(5000000))
	}

	s.Add(5000001)
	if s.NextMissing(5000000) != 5000002 {
		t.Error("Expected 5000002, got", s.NextMissing(5000000))
	}

	s.Add(5000002)
	if s.NextMissing(5000000) != 5000004 {
		t.Error("Expected 5000004, got", s.NextMissing(5000000))
	}
}
