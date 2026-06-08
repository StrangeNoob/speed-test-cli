package history

import (
	"testing"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func TestMedian(t *testing.T) {
	if Median(nil) != 0 {
		t.Error("empty should be 0")
	}
	if Median([]float64{5}) != 5 {
		t.Error("single should be itself")
	}
	if Median([]float64{3, 1, 2}) != 2 {
		t.Error("odd unsorted should be 2")
	}
	if Median([]float64{4, 1, 3, 2}) != 2.5 {
		t.Error("even should be mean of two middles (2.5)")
	}
}

func mkResult(dl, ul, pingMs, jitMs float64) speedtest.Result {
	return speedtest.Result{
		DownloadMbps: dl,
		UploadMbps:   ul,
		Latency:      time.Duration(pingMs * float64(time.Millisecond)),
		Jitter:       time.Duration(jitMs * float64(time.Millisecond)),
	}
}

func TestCompareEmptyBaseline(t *testing.T) {
	c := Compare(mkResult(100, 50, 20, 4), nil)
	if c.HasBaseline {
		t.Error("empty baseline -> HasBaseline false")
	}
	if c.SampleSize != 0 {
		t.Errorf("SampleSize = %d, want 0", c.SampleSize)
	}
	if c.Verdict != "insufficient_history" {
		t.Errorf("Verdict = %q, want insufficient_history", c.Verdict)
	}
}

func TestCompareMediansAndStatus(t *testing.T) {
	base := []speedtest.Result{mkResult(100, 40, 20, 4), mkResult(120, 60, 30, 6), mkResult(140, 50, 40, 8)}
	c := Compare(mkResult(150, 50, 24, 6), base)
	if !c.HasBaseline || c.SampleSize != 3 {
		t.Fatalf("HasBaseline/SampleSize = %v/%d", c.HasBaseline, c.SampleSize)
	}
	if c.Download.Baseline != 120 {
		t.Errorf("download median = %v, want 120", c.Download.Baseline)
	}
	if d := c.Download.DeltaPct; d < 24.99 || d > 25.01 {
		t.Errorf("download delta = %v, want 25", d)
	}
	if c.Download.Status != StatusBetter {
		t.Error("download +25% -> Better")
	}
	if c.Upload.Status != StatusNormal {
		t.Error("upload 50 vs 50 -> Normal")
	}
	if c.Ping.Status != StatusBetter {
		t.Error("ping 24 vs 30 = -20% -> +20% improvement -> Better")
	}
}

func TestCompareVerdicts(t *testing.T) {
	base := []speedtest.Result{mkResult(100, 50, 20, 4)}
	cases := []struct {
		name    string
		cur     speedtest.Result
		verdict string
	}{
		{"excellent", mkResult(130, 50, 20, 4), "excellent"},
		{"degraded", mkResult(80, 50, 20, 4), "degraded"},
		{"normal", mkResult(100, 50, 20, 4), "normal"},
		{"unstable from jitter", mkResult(100, 50, 20, 8), "unstable"},
		{"normal high latency", mkResult(100, 50, 23, 4), "normal_high_latency"},
	}
	for _, tc := range cases {
		if got := Compare(tc.cur, base).Verdict; got != tc.verdict {
			t.Errorf("%s: verdict = %q, want %q", tc.name, got, tc.verdict)
		}
	}
}

func TestCompareZeroMedianUndefined(t *testing.T) {
	base := []speedtest.Result{mkResult(100, 0, 20, 4)}
	c := Compare(mkResult(100, 50, 20, 4), base)
	if c.Upload.Defined {
		t.Error("upload median 0 -> Defined false")
	}
}
