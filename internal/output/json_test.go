package output

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func TestJSONContainsFields(t *testing.T) {
	res := speedtest.Result{
		Timestamp:    time.Unix(0, 0).UTC(),
		ServerColo:   "SIN",
		DownloadMbps: 100.5,
		UploadMbps:   20.25,
	}
	var buf bytes.Buffer
	if err := JSON(&buf, res); err != nil {
		t.Fatalf("JSON error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["server_colo"] != "SIN" {
		t.Errorf("server_colo = %v, want SIN", got["server_colo"])
	}
	if got["download_mbps"].(float64) != 100.5 {
		t.Errorf("download_mbps = %v, want 100.5", got["download_mbps"])
	}
}
