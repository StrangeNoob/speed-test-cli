package history

import "testing"

func TestMedian(t *testing.T) {
	if Median(nil) != 0 {
		t.Error("empty should be 0")
	}
	if Median([]float64{5}) != 5 {
		t.Error("single should be itself")
	}
	if Median([]float64{3, 1, 2}) != 2 {
		t.Error("odd unsorted should be 2")
	}
	if Median([]float64{4, 1, 3, 2}) != 2.5 {
		t.Error("even should be mean of two middles (2.5)")
	}
}
