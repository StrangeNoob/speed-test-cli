package speedtest

import (
	"strings"
	"testing"
	"time"
)

// TestRealCloudflare hits the live Cloudflare endpoints. Skipped under -short.
func TestRealCloudflare(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in -short mode")
	}
	c := NewClient()
	cfg := Config{Streams: 4, Duration: 5 * time.Second}
	res, err := c.Run(cfg, nil)
	if err != nil {
		// Cloudflare rate-limits aggressive back-to-back runs; that's a transient
		// network condition, not a code defect, so skip rather than fail.
		if strings.Contains(err.Error(), "rate limited") {
			t.Skipf("skipping: Cloudflare rate-limited this run (%v)", err)
		}
		t.Fatalf("live Run error: %v", err)
	}
	if res.DownloadMbps <= 0 {
		t.Errorf("DownloadMbps = %v, want > 0", res.DownloadMbps)
	}
	t.Logf("colo=%s ping=%v jitter=%v down=%.1f up=%.1f",
		res.ServerColo, res.Latency, res.Jitter, res.DownloadMbps, res.UploadMbps)
}
