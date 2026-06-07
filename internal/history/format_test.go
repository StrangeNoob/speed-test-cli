package history

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/output"
	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func sampleRecords() []speedtest.Result {
	return []speedtest.Result{{
		Timestamp:    time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC),
		ServerColo:   "MAA",
		DownloadMbps: 100.5,
		UploadMbps:   20,
		Latency:      15 * time.Millisecond,
		Jitter:       2 * time.Millisecond,
	}}
}

func TestCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := CSV(&buf, sampleRecords()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "timestamp,server_colo,download_mbps,upload_mbps,ping_ms,jitter_ms\n") {
		t.Errorf("bad header:\n%s", out)
	}
	if !strings.Contains(out, "2026-06-07T10:00:00Z,MAA,100.5,20,15,2") {
		t.Errorf("bad row:\n%s", out)
	}
}

func TestCSVEmptyHeaderOnly(t *testing.T) {
	var buf bytes.Buffer
	if err := CSV(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "timestamp,server_colo,download_mbps,upload_mbps,ping_ms,jitter_ms\n" {
		t.Errorf("empty CSV = %q", buf.String())
	}
}

func TestJSONRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sampleRecords()); err != nil {
		t.Fatal(err)
	}
	var got []speedtest.Result
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 1 || got[0].ServerColo != "MAA" || got[0].DownloadMbps != 100.5 {
		t.Errorf("round-trip = %+v", got)
	}
}

func TestJSONEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("empty JSON = %q", buf.String())
	}
}

func TestTablePlainNoEscapes(t *testing.T) {
	recs := []speedtest.Result{{
		Timestamp: time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC),
		DownloadMbps: 92.4, UploadMbps: 48.1,
		Latency: 21 * time.Millisecond, Jitter: 4 * time.Millisecond,
	}}
	var buf bytes.Buffer
	Table(&buf, recs, 5, output.NewStyler(false))
	out := buf.String()
	for _, want := range []string{"Date/Time", "Download", "Jitter", "92.4", "48.1", "21 ms", "showing 1 of 5"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("disabled styler leaked an escape:\n%s", out)
	}
}

func TestTableColorEscapes(t *testing.T) {
	recs := []speedtest.Result{{Timestamp: time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)}}
	var buf bytes.Buffer
	Table(&buf, recs, 1, output.NewStyler(true))
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Error("enabled styler should emit escapes")
	}
}

func TestRenderSummary(t *testing.T) {
	s := Summary{
		Count: 3,
		First: time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC),
		Last:  time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		Download: metricStats{Avg: 118.4, Min: 42.1, Max: 201.3},
		Upload:   metricStats{Avg: 47.2, Min: 18, Max: 63.5},
		Ping:     metricStats{Avg: 24.1, Min: 14, Max: 58},
		Jitter:   metricStats{Avg: 5.8, Min: 1.2, Max: 19.4},
	}
	var buf bytes.Buffer
	RenderSummary(&buf, s, output.NewStyler(false))
	for _, want := range []string{"Speed Test Summary", "3 runs", "Download", "118.4", "201.3", "Mbps"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("summary missing %q\n%s", want, buf.String())
		}
	}
}
