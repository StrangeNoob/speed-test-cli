package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CheckInterval is the minimum time between real GitHub queries.
const CheckInterval = 24 * time.Hour

// Cache records the last update check, stored next to the history file.
type Cache struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version"`
}

// DefaultCachePath returns ~/.speed-test/update-check.json.
func DefaultCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".speed-test", "update-check.json"), nil
}

// Load reads the cache. A missing or malformed file yields the zero value; this
// is best-effort and never returns an error.
func Load(path string) Cache {
	b, err := os.ReadFile(path)
	if err != nil {
		return Cache{}
	}
	var c Cache
	if err := json.Unmarshal(b, &c); err != nil {
		return Cache{}
	}
	return c
}

// Save writes the cache, creating parent directories as needed.
func Save(path string, c Cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Due reports whether at least interval has passed since the last check.
func Due(c Cache, now time.Time, interval time.Duration) bool {
	return now.Sub(c.LastCheck) >= interval
}
