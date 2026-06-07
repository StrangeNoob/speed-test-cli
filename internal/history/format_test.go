package history

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

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
