package output

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRenderBarWidth(t *testing.T) {
	// Every result must be exactly `width` runes regardless of value.
	for _, tc := range []struct {
		value, max float64
	}{
		{0, 100}, {50, 100}, {100, 100}, {150, 100}, {10, 0},
	} {
		got := renderBar(tc.value, tc.max, 10)
		if n := utf8.RuneCountInString(got); n != 10 {
			t.Errorf("renderBar(%v,%v,10) width = %d runes, want 10 (%q)", tc.value, tc.max, n, got)
		}
	}
}

func TestRenderBarFull(t *testing.T) {
	got := renderBar(100, 100, 8)
	if got != strings.Repeat("█", 8) {
		t.Errorf("full bar = %q, want 8 full blocks", got)
	}
}

func TestRenderBarEmpty(t *testing.T) {
	got := renderBar(0, 100, 8)
	if got != strings.Repeat(" ", 8) {
		t.Errorf("empty bar = %q, want 8 spaces", got)
	}
}

func TestRenderBarClampsOverMax(t *testing.T) {
	if renderBar(999, 100, 6) != strings.Repeat("█", 6) {
		t.Errorf("value over max should clamp to full bar")
	}
}

func TestRenderBarZeroMax(t *testing.T) {
	if renderBar(50, 0, 6) != strings.Repeat(" ", 6) {
		t.Errorf("max<=0 should render an all-space bar")
	}
}

func TestRenderBarHalf(t *testing.T) {
	// 50% of 8 cells = 4 full blocks then 4 spaces.
	got := renderBar(50, 100, 8)
	if got != strings.Repeat("█", 4)+strings.Repeat(" ", 4) {
		t.Errorf("half bar = %q, want 4 full + 4 spaces", got)
	}
}
