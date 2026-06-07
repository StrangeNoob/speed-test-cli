package update

import "testing"

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            bool
	}{
		{"v0.1.5", "v0.2.0", true},
		{"0.1.5", "0.2.0", true},
		{"v0.1.5", "v0.1.5", false},
		{"v0.1.5", "0.1.5", false}, // mixed prefix: go-selfupdate strips the v
		{"v0.2.0", "v0.1.5", false},
		{"dev", "v0.2.0", false},
		{"v0.1.5", "garbage", false},
		{"garbage", "v0.2.0", false},
		{"v0.1.5", "v0.1.6-next", true},
	} {
		if got := Newer(tc.current, tc.latest); got != tc.want {
			t.Errorf("Newer(%q,%q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestUpToDate(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            bool
	}{
		{"v0.2.0", "v0.2.0", true},
		{"v0.1.5", "0.1.5", true}, // mixed prefix: go-selfupdate strips the v
		{"v0.3.0", "v0.2.0", true},
		{"v0.1.0", "v0.2.0", false},
		{"dev", "v0.2.0", false},
		{"garbage", "v0.2.0", false},
	} {
		if got := upToDate(tc.current, tc.latest); got != tc.want {
			t.Errorf("upToDate(%q,%q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}
