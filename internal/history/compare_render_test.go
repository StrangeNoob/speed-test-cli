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

func TestRenderCompareUndefinedMetric(t *testing.T) {
	// A prior run with upload 0 → upload median 0 → undefined → renders "—".
	cur := speedtest.Result{DownloadMbps: 100, UploadMbps: 50, Latency: 20 * time.Millisecond, Jitter: 4 * time.Millisecond}
	base := []speedtest.Result{{DownloadMbps: 90, UploadMbps: 0, Latency: 20 * time.Millisecond, Jitter: 4 * time.Millisecond}}
	var buf bytes.Buffer
	RenderCompare(&buf, Compare(cur, base), PlanInfo{}, output.NewStyler(false))
	if !strings.Contains(buf.String(), "—") {
		t.Errorf("undefined metric should render a dash:\n%s", buf.String())
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

func TestRenderCompareJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderCompareJSON(&buf, cmpFixture(), PlanInfo{Set: true, Download: 200, Upload: 100}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["verdict"] == nil || got["summary"] == nil {
		t.Errorf("missing verdict/summary: %v", got)
	}
	if got["baseline"] == nil {
		t.Error("baseline should be present when there is a baseline")
	}
	if got["plan"] == nil {
		t.Error("plan should be present when set")
	}
}

func TestRenderCompareJSONNoBaseline(t *testing.T) {
	var buf bytes.Buffer
	c := Compare(speedtest.Result{DownloadMbps: 100}, nil)
	if err := RenderCompareJSON(&buf, c, PlanInfo{}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if v, ok := got["baseline"]; !ok || v != nil {
		t.Errorf("baseline should be null with no history, got %v", v)
	}
	if _, ok := got["plan"]; ok {
		t.Error("plan should be omitted when unset")
	}
}
