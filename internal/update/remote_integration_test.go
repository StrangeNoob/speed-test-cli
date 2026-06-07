package update

import (
	"context"
	"testing"
	"time"
)

// TestLatestLive hits the real GitHub API. Skipped under -short.
func TestLatestLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live GitHub test in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	v, err := Latest(ctx)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if v == "" {
		t.Fatal("expected a non-empty latest version")
	}
	t.Logf("latest = %s", v)
}
