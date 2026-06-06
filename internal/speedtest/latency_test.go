package speedtest

import (
	"testing"
	"time"
)

func TestMedian(t *testing.T) {
	in := []time.Duration{30, 10, 20} // unsorted
	if got := median(in); got != 20 {
		t.Fatalf("median = %v, want 20", got)
	}
}

func TestMedianEven(t *testing.T) {
	in := []time.Duration{10, 20, 30, 40}
	if got := median(in); got != 25 {
		t.Fatalf("median = %v, want 25", got)
	}
}

func TestMedianEmpty(t *testing.T) {
	if got := median(nil); got != 0 {
		t.Fatalf("median(nil) = %v, want 0", got)
	}
}

func TestJitter(t *testing.T) {
	// diffs: |20-10|=10, |10-20|=10 -> mean 10
	in := []time.Duration{10, 20, 10}
	if got := jitter(in); got != 10 {
		t.Fatalf("jitter = %v, want 10", got)
	}
}

func TestJitterSingle(t *testing.T) {
	if got := jitter([]time.Duration{5}); got != 0 {
		t.Fatalf("jitter single = %v, want 0", got)
	}
}

func TestParseServerTiming(t *testing.T) {
	// cfRequestDuration is in milliseconds
	h := "cfRequestDuration;dur=12.5"
	got := parseServerTiming(h)
	if got != 12500*time.Microsecond {
		t.Fatalf("parseServerTiming = %v, want 12.5ms", got)
	}
}

func TestParseServerTimingMissing(t *testing.T) {
	if got := parseServerTiming("cfCacheStatus;desc=HIT"); got != 0 {
		t.Fatalf("parseServerTiming = %v, want 0", got)
	}
}
