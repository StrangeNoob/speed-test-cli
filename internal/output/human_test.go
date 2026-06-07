package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func TestProgressPrinterAnimates(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressPrinter(&buf, true)
	p(speedtest.Progress{Phase: speedtest.PhaseDownload, Mbps: 20})
	p(speedtest.Progress{Phase: speedtest.PhaseDownload, Mbps: 50})
	p(speedtest.Progress{Phase: speedtest.PhaseUpload, Mbps: 10})
	out := buf.String()

	if !strings.Contains(out, "Download") || !strings.Contains(out, "Upload") {
		t.Errorf("expected both phase labels, got:\n%q", out)
	}
	if !strings.Contains(out, "\n") {
		t.Errorf("expected a newline at the phase boundary, got:\n%q", out)
	}
	if !strings.Contains(out, "\r") {
		t.Errorf("expected carriage-return redraws, got:\n%q", out)
	}
	if !strings.ContainsAny(out, strings.Join(spinnerFrames, "")) {
		t.Errorf("expected a spinner frame, got:\n%q", out)
	}
}

func TestProgressPrinterNoAnimateSilent(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressPrinter(&buf, false)
	p(speedtest.Progress{Phase: speedtest.PhaseDownload, Mbps: 20})
	p(speedtest.Progress{Phase: speedtest.PhaseUpload, Mbps: 5})
	if buf.Len() != 0 {
		t.Errorf("non-animating printer must produce no output, got:\n%q", buf.String())
	}
}

func TestHumanSummaryContainsMetrics(t *testing.T) {
	res := speedtest.Result{
		ServerColo:   "SIN",
		Latency:      15 * time.Millisecond,
		Jitter:       2 * time.Millisecond,
		DownloadMbps: 100.5,
		UploadMbps:   20.2,
	}
	var buf bytes.Buffer
	Human(&buf, res, NewStyler(false))
	out := buf.String()

	for _, want := range []string{"SIN", "100.5", "20.2", "Download", "Upload", "Ping", "Jitter"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q\n---\n%s", want, out)
		}
	}
	// Disabled styler must not emit escape codes.
	if strings.Contains(out, "\x1b") {
		t.Errorf("disabled styler leaked an escape code:\n%s", out)
	}
	// A bar glyph should be present (download is the faster of the two -> full).
	if !strings.Contains(out, "█") {
		t.Errorf("summary missing a bar glyph:\n%s", out)
	}
}

func TestHumanSummaryColorEmitsEscapes(t *testing.T) {
	res := speedtest.Result{ServerColo: "SIN", DownloadMbps: 100, UploadMbps: 50}
	var buf bytes.Buffer
	Human(&buf, res, NewStyler(true))
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("enabled styler should emit ANSI escapes:\n%s", buf.String())
	}
}

func TestHumanSummaryZeroRendersDash(t *testing.T) {
	res := speedtest.Result{ServerColo: "SIN", DownloadMbps: 100, UploadMbps: 0}
	var buf bytes.Buffer
	Human(&buf, res, NewStyler(false))
	out := buf.String()
	if !strings.Contains(out, "—") {
		t.Errorf("a zero direction should render an em-dash, got:\n%s", out)
	}
}
