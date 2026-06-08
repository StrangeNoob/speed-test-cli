package history

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/output"
	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func cmpFixture() Comparison {
	cur := speedtest.Result{DownloadMbps: 150, UploadMbps: 136, Latency: 56 * time.Millisecond, Jitter: 6 * time.Millisecond}
	base := []speedtest.Result{
		{DownloadMbps: 130, UploadMbps: 140, Latency: 47 * time.Millisecond, Jitter: 5 * time.Millisecond},
		{DownloadMbps: 134, UploadMbps: 142, Latency: 44 * time.Millisecond, Jitter: 6 * time.Millisecond},
	}
	return Compare(cur, base)
}

func TestRenderComparePlain(t *testing.T) {
	var buf bytes.Buffer
	RenderCompare(&buf, cmpFixture(), PlanInfo{}, output.NewStyler(false))
	out := buf.String()
	for _, want := range []string{"Compare", "Current test", "Download", "Compared with last 2 tests", "Verdict"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("disabled styler leaked an escape:\n%s", out)
	}
}

func TestRenderCompareColor(t *testing.T) {
	var buf bytes.Buffer
	RenderCompare(&buf, cmpFixture(), PlanInfo{}, output.NewStyler(true))
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Error("enabled styler should emit escapes")
	}
}

func TestRenderCompareNoBaseline(t *testing.T) {
	var buf bytes.Buffer
	c := Compare(speedtest.Result{DownloadMbps: 100}, nil)
	RenderCompare(&buf, c, PlanInfo{}, output.NewStyler(false))
	if !strings.Contains(buf.String(), "Not enough history") {
		t.Errorf("missing no-baseline message:\n%s", buf.String())
	}
}

func TestRenderComparePlan(t *testing.T) {
	var buf bytes.Buffer
	RenderCompare(&buf, cmpFixture(), PlanInfo{Set: true, Download: 200, Upload: 100}, output.NewStyler(false))
	out := buf.String()
	if !strings.Contains(out, "Plan performance") || !strings.Contains(out, "/ 200 Mbps") {
		t.Errorf("missing plan section:\n%s", out)
	}
}
