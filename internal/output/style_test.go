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

func TestSpinnerFramesNonEmpty(t *testing.T) {
	if len(spinnerFrames) == 0 {
		t.Fatal("spinnerFrames must not be empty")
	}
	for i, f := range spinnerFrames {
		if f == "" {
			t.Errorf("spinnerFrames[%d] is empty", i)
		}
	}
}

func TestStylerEnabledWraps(t *testing.T) {
	st := NewStyler(true)
	got := st.Cyan("hi")
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("enabled Cyan should contain an ANSI escape, got %q", got)
	}
	if !strings.Contains(got, "hi") {
		t.Errorf("enabled Cyan should still contain the text, got %q", got)
	}
}

func TestStylerDisabledPlain(t *testing.T) {
	st := NewStyler(false)
	for _, got := range []string{st.Cyan("x"), st.Green("x"), st.Red("x"), st.Dim("x"), st.Bold("x")} {
		if got != "x" {
			t.Errorf("disabled styler should return raw text, got %q", got)
		}
	}
}

func TestShouldColor(t *testing.T) {
	for _, tc := range []struct {
		name      string
		isTTY     bool
		noColor   bool
		noColorNV string
		want      bool
	}{
		{"tty no flags", true, false, "", true},
		{"not a tty", false, false, "", false},
		{"flag set", true, true, "", false},
		{"NO_COLOR set", true, false, "1", false},
		{"NO_COLOR empty value but present treated as unset", true, false, "", true},
		{"all off", false, true, "1", false},
	} {
		if got := ShouldColor(tc.isTTY, tc.noColor, tc.noColorNV); got != tc.want {
			t.Errorf("%s: ShouldColor(%v,%v,%q) = %v, want %v",
				tc.name, tc.isTTY, tc.noColor, tc.noColorNV, got, tc.want)
		}
	}
}
