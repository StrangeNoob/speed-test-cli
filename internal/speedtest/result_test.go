package speedtest

import (
	"testing"
	"time"
)

func TestMbps(t *testing.T) {
	// 12_500_000 bytes in 1s = 100 Mbps (bytes*8 / 1e6 / seconds)
	got := Mbps(12_500_000, time.Second)
	if got != 100 {
		t.Fatalf("Mbps = %v, want 100", got)
	}
}

func TestMbpsZeroDuration(t *testing.T) {
	if got := Mbps(1000, 0); got != 0 {
		t.Fatalf("Mbps with zero duration = %v, want 0", got)
	}
}
