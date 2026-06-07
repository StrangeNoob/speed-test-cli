package history

import (
	"testing"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func TestSummarize(t *testing.T) {
	t1 := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	recs := []speedtest.Result{
		{Timestamp: t2, DownloadMbps: 100, UploadMbps: 40, Latency: 20 * time.Millisecond, Jitter: 4 * time.Millisecond},
		{Timestamp: t1, DownloadMbps: 80, UploadMbps: 60, Latency: 10 * time.Millisecond, Jitter: 2 * time.Millisecond},
	}
	s := Summarize(recs)
	if s.Count != 2 {
		t.Errorf("Count = %d, want 2", s.Count)
	}
	if !s.First.Equal(t1) || !s.Last.Equal(t2) {
		t.Errorf("range = %v..%v, want %v..%v", s.First, s.Last, t1, t2)
	}
	if s.Download != (metricStats{Avg: 90, Min: 80, Max: 100}) {
		t.Errorf("Download = %+v", s.Download)
	}
	if s.Ping != (metricStats{Avg: 15, Min: 10, Max: 20}) {
		t.Errorf("Ping = %+v", s.Ping)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	if Summarize(nil).Count != 0 {
		t.Error("empty input should give Count 0")
	}
}
