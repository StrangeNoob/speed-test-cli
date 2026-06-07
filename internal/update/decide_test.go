package update

import (
	"bytes"
	"strings"
	"testing"
)

func TestShouldCheck(t *testing.T) {
	for _, tc := range []struct {
		name    string
		json    bool
		noFlag  bool
		env     string
		version string
		want    bool
	}{
		{"normal release", false, false, "", "v0.1.5", true},
		{"json mode", true, false, "", "v0.1.5", false},
		{"flag set", false, true, "", "v0.1.5", false},
		{"env set", false, false, "1", "v0.1.5", false},
		{"dev build", false, false, "", "dev", false},
	} {
		if got := ShouldCheck(tc.json, tc.noFlag, tc.env, tc.version); got != tc.want {
			t.Errorf("%s: ShouldCheck = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestShouldPrompt(t *testing.T) {
	if !ShouldPrompt(true) || ShouldPrompt(false) {
		t.Error("ShouldPrompt should mirror isTTY")
	}
}

func TestPromptYesNo(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"y\n", true}, {"Y\n", true}, {"yes\n", true}, {"YES\n", true},
		{"n\n", false}, {"\n", false}, {"nope\n", false},
	} {
		var out bytes.Buffer
		got, _ := PromptYesNo(strings.NewReader(tc.in), &out, "Update now? [y/N] ")
		if got != tc.want {
			t.Errorf("PromptYesNo(%q) = %v, want %v", tc.in, got, tc.want)
		}
		if !strings.Contains(out.String(), "Update now?") {
			t.Errorf("prompt text not written: %q", out.String())
		}
	}
}
