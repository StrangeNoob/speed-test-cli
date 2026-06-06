package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"speed-test-cli/internal/speedtest"
)

func TestHumanSummaryContainsMetrics(t *testing.T) {
	res := speedtest.Result{
		ServerColo:   "SIN",
		Latency:      15 * time.Millisecond,
		Jitter:       2 * time.Millisecond,
		DownloadMbps: 100.5,
		UploadMbps:   20.2,
	}
	var buf bytes.Buffer
	Human(&buf, res)
	out := buf.String()

	for _, want := range []string{"SIN", "100.5", "20.2", "Download", "Upload", "Ping", "Jitter"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q\n---\n%s", want, out)
		}
	}
}
