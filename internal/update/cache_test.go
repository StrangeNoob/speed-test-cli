package update

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingReturnsZero(t *testing.T) {
	got := Load(filepath.Join(t.TempDir(), "nope.json"))
	if !got.LastCheck.IsZero() || got.LatestVersion != "" {
		t.Errorf("missing file should load zero Cache, got %+v", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "update-check.json")
	want := Cache{LastCheck: time.Unix(1700000000, 0).UTC(), LatestVersion: "v0.2.0"}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got := Load(path)
	if !got.LastCheck.Equal(want.LastCheck) || got.LatestVersion != want.LatestVersion {
		t.Errorf("round-trip = %+v, want %+v", got, want)
	}
}

func TestDue(t *testing.T) {
	now := time.Unix(1700000000, 0)
	if Due(Cache{}, now, CheckInterval) != true {
		t.Error("never-checked should be due")
	}
	recent := Cache{LastCheck: now.Add(-23 * time.Hour)}
	if Due(recent, now, CheckInterval) != false {
		t.Error("checked 23h ago should not be due")
	}
	old := Cache{LastCheck: now.Add(-25 * time.Hour)}
	if Due(old, now, CheckInterval) != true {
		t.Error("checked 25h ago should be due")
	}
}
