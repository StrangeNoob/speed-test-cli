# `speed-test compare` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `speed-test compare` — run (or reuse) a speed test and report whether the connection is better/normal/degraded versus the median of the user's past results, with `--last`/`--window`/`--latest`/`--plan-*`/`--json`.

**Architecture:** Pure comparison logic (`Median`, `Compare`, verdict) plus the comparison renderers live in `internal/history` — rendering must live there (not `internal/output`) because `internal/history` already imports `internal/output` for the `Styler`, so putting it in `output` would create an import cycle. `cmd/compare.go` orchestrates: gather current + baseline, then render.

**Tech Stack:** Go, `github.com/spf13/cobra`, stdlib `sort`/`encoding/json`/`time`. Reuses `speedtest`, `internal/history`, `internal/output`.

**IMPORTANT ENV:** Prefix every Go command with `CGO_ENABLED=0` (this machine crashes otherwise with a dyld `LC_UUID` error; Go 1.25 toolchain auto-installs). Do NOT add a `Co-Authored-By` trailer (or any Claude attribution) to commit messages.

---

## File Structure

| File | Change | Responsibility |
|------|--------|----------------|
| `internal/history/compare.go` | new | `Median`, comparison types, `Compare`, verdict (pure) |
| `internal/history/compare_test.go` | new | median + compare/verdict tests |
| `internal/output/style.go` | modify | add a `Red` method to `Styler` |
| `internal/output/style_test.go` | modify | cover `Red` |
| `internal/history/compare_render.go` | new | `PlanInfo`, `RenderCompare`, `RenderCompareJSON` |
| `internal/history/compare_render_test.go` | new | renderer tests |
| `cmd/compare.go` | new | `compare` subcommand |
| `cmd/compare_test.go` | new | command wiring tests (offline via `--latest`) |
| `cmd/root.go` | modify | register `newCompareCmd()` |
| `README.md` | modify | document the command |

> **Spec deviation (intentional):** the spec listed `internal/output/compare.go`; the
> renderers go in `internal/history/compare_render.go` instead to avoid an import
> cycle (`output` ← `history`). This matches where `Table`/`RenderSummary` already
> live.

The record type is `speedtest.Result` (`Timestamp`, `ServerColo`, `Latency time.Duration`, `Jitter time.Duration`, `DownloadMbps float64`, `UploadMbps float64`). `internal/history` already exports `Load`, `LastN`, `Filter`, `ParseBound`, `DefaultPath`, `Append`, and the unexported `msOf(d time.Duration) float64`.

---

## Task 1: `Median`

**Files:**
- Create: `internal/history/compare.go`
- Test: `internal/history/compare_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/history/compare_test.go`:
```go
package history

import "testing"

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run TestMedian -v`
Expected: FAIL — `undefined: Median`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/history/compare.go`:
```go
package history

import "sort"

// Median returns the middle value of values (the mean of the two middles for an
// even count). An empty slice returns 0.
func Median(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	s := make([]float64, n)
	copy(s, values)
	sort.Float64s(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run TestMedian -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/history/compare.go internal/history/compare_test.go
git commit -m "feat(compare): add Median helper"
```

---

## Task 2: `Compare` + verdict

**Files:**
- Modify: `internal/history/compare.go`
- Modify: `internal/history/compare_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/history/compare_test.go` (add imports `time` and the speedtest package to its block — final block: `"testing"`, `"time"`, `"github.com/StrangeNoob/speed-test-cli/internal/speedtest"`):
```go
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
	// medians: download 120, upload 50, ping 30, jitter 6
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
	base := []speedtest.Result{mkResult(100, 0, 20, 4)} // upload median 0
	c := Compare(mkResult(100, 50, 20, 4), base)
	if c.Upload.Defined {
		t.Error("upload median 0 -> Defined false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run TestCompare -v`
Expected: FAIL — `undefined: Compare` / `StatusBetter`.

- [ ] **Step 3: Write minimal implementation**

In `internal/history/compare.go`, change the import to a group and add the speedtest import:
```go
import (
	"sort"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)
```
Then append:
```go
// MetricStatus classifies a metric versus its baseline.
type MetricStatus int

const (
	StatusNormal MetricStatus = iota
	StatusBetter
	StatusWorse
)

// MetricCompare is one metric's current value, baseline median, and change.
type MetricCompare struct {
	Current  float64
	Baseline float64 // median of the baseline
	DeltaPct float64 // (current-baseline)/baseline*100; valid only when Defined
	Defined  bool    // false when the baseline median is 0
	Status   MetricStatus
}

// Comparison is the full current-vs-baseline result.
type Comparison struct {
	Current     speedtest.Result
	HasBaseline bool
	SampleSize  int
	Download    MetricCompare // higher is better
	Upload      MetricCompare // higher is better
	Ping        MetricCompare // lower is better
	Jitter      MetricCompare // lower is better
	Verdict     string        // excellent|normal|degraded|unstable, optionally +_high_latency
	Summary     string        // human sentence
}

const (
	labelThreshold   = 10.0 // per-metric Better/Worse cutoff (improvement %)
	verdictThreshold = 15.0 // download Excellent/Degraded cutoff (improvement %)
)

// metricCompare builds a MetricCompare. higherBetter is true for throughput
// (download/upload) and false for latency (ping/jitter).
func metricCompare(current float64, baselineVals []float64, higherBetter bool) MetricCompare {
	med := Median(baselineVals)
	mc := MetricCompare{Current: current, Baseline: med}
	if med == 0 {
		return mc // Defined stays false
	}
	mc.Defined = true
	mc.DeltaPct = (current - med) / med * 100
	improvement := mc.DeltaPct
	if !higherBetter {
		improvement = -improvement
	}
	switch {
	case improvement >= labelThreshold:
		mc.Status = StatusBetter
	case improvement <= -labelThreshold:
		mc.Status = StatusWorse
	}
	return mc
}

// improvementOf returns the direction-corrected improvement % and whether it is
// defined.
func improvementOf(mc MetricCompare, higherBetter bool) (float64, bool) {
	if !mc.Defined {
		return 0, false
	}
	if higherBetter {
		return mc.DeltaPct, true
	}
	return -mc.DeltaPct, true
}

// Compare computes the comparison of current against the baseline records.
func Compare(current speedtest.Result, baseline []speedtest.Result) Comparison {
	c := Comparison{Current: current, SampleSize: len(baseline)}
	if len(baseline) == 0 {
		c.Verdict = "insufficient_history"
		c.Summary = "Not enough history to compare yet."
		return c
	}
	c.HasBaseline = true

	var dl, ul, pg, jt []float64
	for _, r := range baseline {
		dl = append(dl, r.DownloadMbps)
		ul = append(ul, r.UploadMbps)
		pg = append(pg, msOf(r.Latency))
		jt = append(jt, msOf(r.Jitter))
	}
	c.Download = metricCompare(current.DownloadMbps, dl, true)
	c.Upload = metricCompare(current.UploadMbps, ul, true)
	c.Ping = metricCompare(msOf(current.Latency), pg, false)
	c.Jitter = metricCompare(msOf(current.Jitter), jt, false)
	c.Verdict, c.Summary = verdict(c)
	return c
}

// verdict produces the overall code and human summary from a populated comparison.
func verdict(c Comparison) (code, summary string) {
	dlImpr, dlOk := improvementOf(c.Download, true)
	pgImpr, pgOk := improvementOf(c.Ping, false)
	jtImpr, jtOk := improvementOf(c.Jitter, false)

	if (pgOk && pgImpr <= -25) || (jtOk && jtImpr <= -50) {
		return "unstable", "Your latency is much higher than usual — the connection looks unstable."
	}

	switch {
	case dlOk && dlImpr >= verdictThreshold:
		code, summary = "excellent", "Your connection is performing better than usual."
	case dlOk && dlImpr <= -verdictThreshold:
		code, summary = "degraded", "Your download is slower than your recent baseline."
	default:
		code, summary = "normal", "Your connection is performing normally."
	}
	if c.Ping.Status == StatusWorse || c.Jitter.Status == StatusWorse {
		code += "_high_latency"
		summary += " Latency is higher than your recent baseline."
	}
	return code, summary
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run 'TestCompare|TestMedian' -v`
Expected: PASS (all). Also `CGO_ENABLED=0 go vet ./internal/history/`.

- [ ] **Step 5: Commit**

```bash
git add internal/history/compare.go internal/history/compare_test.go
git commit -m "feat(compare): add Compare with median baseline and verdict"
```

---

## Task 3: `Styler.Red` + table renderer

**Files:**
- Modify: `internal/output/style.go`
- Modify: `internal/output/style_test.go`
- Create: `internal/history/compare_render.go`
- Create: `internal/history/compare_render_test.go`

- [ ] **Step 1: Write the failing test**

(a) In `internal/output/style_test.go`, REPLACE `TestStylerDisabledPlain` with a version that also covers `Red`:
```go
func TestStylerDisabledPlain(t *testing.T) {
	st := NewStyler(false)
	for _, got := range []string{st.Cyan("x"), st.Green("x"), st.Red("x"), st.Dim("x"), st.Bold("x")} {
		if got != "x" {
			t.Errorf("disabled styler should return raw text, got %q", got)
		}
	}
}
```

(b) Create `internal/history/compare_render_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestStyler -v` and `CGO_ENABLED=0 go test ./internal/history/ -run TestRenderCompare -v`
Expected: FAIL — `st.Red undefined` and `undefined: RenderCompare`.

- [ ] **Step 3: Write minimal implementation**

(a) In `internal/output/style.go`, add a `Red` method next to the others:
```go
func (s *Styler) Red(text string) string { return s.wrap("31", text) }
```

(b) Create `internal/history/compare_render.go`:
```go
package history

import (
	"fmt"
	"io"

	"github.com/StrangeNoob/speed-test-cli/internal/output"
)

// PlanInfo carries optional ISP-plan targets for the comparison output.
type PlanInfo struct {
	Set      bool
	Download float64
	Upload   float64
}

// RenderCompare writes the human-readable comparison to w.
func RenderCompare(w io.Writer, c Comparison, plan PlanInfo, st *output.Styler) {
	fmt.Fprintf(w, "%s\n\n", st.Bold("speed-test  •  Compare"))

	fmt.Fprintln(w, st.Cyan("Current test"))
	fmt.Fprintf(w, "  %-9s %7.1f %s\n", "Download", c.Current.DownloadMbps, st.Dim("Mbps"))
	fmt.Fprintf(w, "  %-9s %7.1f %s\n", "Upload", c.Current.UploadMbps, st.Dim("Mbps"))
	fmt.Fprintf(w, "  %-9s %7.1f %s\n", "Ping", msOf(c.Current.Latency), st.Dim("ms"))
	fmt.Fprintf(w, "  %-9s %7.1f %s\n\n", "Jitter", msOf(c.Current.Jitter), st.Dim("ms"))

	if c.HasBaseline {
		fmt.Fprintln(w, st.Cyan(fmt.Sprintf("Compared with last %d tests", c.SampleSize)))
		renderMetricLine(w, st, "Download", c.Download, true)
		renderMetricLine(w, st, "Upload", c.Upload, true)
		renderMetricLine(w, st, "Ping", c.Ping, false)
		renderMetricLine(w, st, "Jitter", c.Jitter, false)
		fmt.Fprintln(w)
	} else {
		fmt.Fprintf(w, "%s\n\n", st.Dim(c.Summary))
	}

	if plan.Set {
		fmt.Fprintln(w, st.Cyan("Plan performance"))
		if plan.Download > 0 {
			fmt.Fprintf(w, "  %-9s %7.1f / %.0f Mbps   %.1f%%\n", "Download", c.Current.DownloadMbps, plan.Download, c.Current.DownloadMbps/plan.Download*100)
		}
		if plan.Upload > 0 {
			fmt.Fprintf(w, "  %-9s %7.1f / %.0f Mbps   %.1f%%\n", "Upload", c.Current.UploadMbps, plan.Upload, c.Current.UploadMbps/plan.Upload*100)
		}
		fmt.Fprintln(w)
	}

	if c.HasBaseline {
		fmt.Fprintln(w, st.Cyan("Verdict"))
		fmt.Fprintf(w, "  %s\n", c.Summary)
	}
}

// renderMetricLine prints "Label  +X.X% phrase", colored by status.
func renderMetricLine(w io.Writer, st *output.Styler, label string, mc MetricCompare, higherBetter bool) {
	if !mc.Defined {
		fmt.Fprintf(w, "  %-9s %s\n", label, st.Dim("—"))
		return
	}
	delta := fmt.Sprintf("%+.1f%%", mc.DeltaPct)
	var colored, phrase string
	switch mc.Status {
	case StatusBetter:
		colored = st.Green(delta)
		if higherBetter {
			phrase = "better than usual"
		} else {
			phrase = "faster than usual"
		}
	case StatusWorse:
		colored = st.Red(delta)
		if higherBetter {
			phrase = "worse than usual"
		} else {
			phrase = "slower than usual"
		}
	default:
		colored = st.Dim(delta)
		phrase = "normal"
	}
	fmt.Fprintf(w, "  %-9s %8s  %s\n", label, colored, st.Dim(phrase))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/output/ ./internal/history/ -v 2>&1 | tail -20`
Expected: PASS (both packages). Also `CGO_ENABLED=0 go vet ./internal/...`.

- [ ] **Step 5: Commit**

```bash
git add internal/output/style.go internal/output/style_test.go internal/history/compare_render.go internal/history/compare_render_test.go
git commit -m "feat(compare): add Red styler color and the comparison table renderer"
```

---

## Task 4: JSON renderer

**Files:**
- Modify: `internal/history/compare_render.go`
- Modify: `internal/history/compare_render_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/history/compare_render_test.go` (add `"encoding/json"` to its import block):
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run TestRenderCompareJSON -v`
Expected: FAIL — `undefined: RenderCompareJSON`.

- [ ] **Step 3: Write minimal implementation**

In `internal/history/compare_render.go`, add `"encoding/json"` to the import block and append:
```go
// RenderCompareJSON writes the comparison as a JSON object to w. baseline is
// null and delta omitted when there is no baseline; undefined metric deltas and
// plan percentages are emitted as null.
func RenderCompareJSON(w io.Writer, c Comparison, plan PlanInfo) error {
	type metricsDTO struct {
		Download float64 `json:"download_mbps"`
		Upload   float64 `json:"upload_mbps"`
		Latency  float64 `json:"latency_ms"`
		Jitter   float64 `json:"jitter_ms"`
	}
	type baselineDTO struct {
		Type       string  `json:"type"`
		SampleSize int     `json:"sample_size"`
		Download   float64 `json:"download_mbps"`
		Upload     float64 `json:"upload_mbps"`
		Latency    float64 `json:"latency_ms"`
		Jitter     float64 `json:"jitter_ms"`
	}
	type deltaDTO struct {
		Download *float64 `json:"download_percent"`
		Upload   *float64 `json:"upload_percent"`
		Latency  *float64 `json:"latency_percent"`
		Jitter   *float64 `json:"jitter_percent"`
	}
	type planDTO struct {
		Download        float64  `json:"download_mbps"`
		Upload          float64  `json:"upload_mbps"`
		DownloadPercent *float64 `json:"download_percent"`
		UploadPercent   *float64 `json:"upload_percent"`
	}
	out := struct {
		Current  metricsDTO   `json:"current"`
		Baseline *baselineDTO `json:"baseline"`
		Delta    *deltaDTO    `json:"delta,omitempty"`
		Plan     *planDTO     `json:"plan,omitempty"`
		Verdict  string       `json:"verdict"`
		Summary  string       `json:"summary"`
	}{
		Current: metricsDTO{c.Current.DownloadMbps, c.Current.UploadMbps, msOf(c.Current.Latency), msOf(c.Current.Jitter)},
		Verdict: c.Verdict,
		Summary: c.Summary,
	}
	if c.HasBaseline {
		out.Baseline = &baselineDTO{
			Type: "median", SampleSize: c.SampleSize,
			Download: c.Download.Baseline, Upload: c.Upload.Baseline,
			Latency: c.Ping.Baseline, Jitter: c.Jitter.Baseline,
		}
		out.Delta = &deltaDTO{
			Download: definedPtr(c.Download),
			Upload:   definedPtr(c.Upload),
			Latency:  definedPtr(c.Ping),
			Jitter:   definedPtr(c.Jitter),
		}
	}
	if plan.Set {
		p := &planDTO{Download: plan.Download, Upload: plan.Upload}
		if plan.Download > 0 {
			v := c.Current.DownloadMbps / plan.Download * 100
			p.DownloadPercent = &v
		}
		if plan.Upload > 0 {
			v := c.Current.UploadMbps / plan.Upload * 100
			p.UploadPercent = &v
		}
		out.Plan = p
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// definedPtr returns a pointer to the delta percent, or nil when undefined.
func definedPtr(mc MetricCompare) *float64 {
	if !mc.Defined {
		return nil
	}
	v := mc.DeltaPct
	return &v
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run TestRenderCompare -v`
Expected: PASS. Also `CGO_ENABLED=0 go vet ./internal/history/`.

- [ ] **Step 5: Commit**

```bash
git add internal/history/compare_render.go internal/history/compare_render_test.go
git commit -m "feat(compare): add JSON comparison renderer"
```

---

## Task 5: `compare` subcommand

**Files:**
- Create: `cmd/compare.go`
- Modify: `cmd/root.go`
- Test: `cmd/compare_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/compare_test.go` (reuses the `writeHistory` helper already defined in `cmd/history_test.go`, same package):
```go
package cmd

import (
	"io"
	"path/filepath"
	"testing"
)

func TestCompareHelp(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"compare", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compare --help: %v", err)
	}
}

func TestCompareLastWindowConflict(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"compare", "--latest", "--last", "5", "--window", "7d"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --last and --window are combined")
	}
}

func TestCompareLatestProducesOutput(t *testing.T) {
	hist := writeHistory(t) // 3 dated records, package cmd helper
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"compare", "--latest", "--log-file", hist})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compare --latest: %v", err)
	}
}

func TestCompareLatestEmptyNoError(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"compare", "--latest", "--log-file", filepath.Join(t.TempDir(), "none.jsonl")})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("empty --latest should not error: %v", err)
	}
}

func TestCompareInvalidWindow(t *testing.T) {
	hist := writeHistory(t)
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"compare", "--latest", "--window", "bogus", "--log-file", hist})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for --window bogus")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./cmd/ -run TestCompare -v`
Expected: FAIL — `compare` is an unknown command / build failure.

- [ ] **Step 3: Write minimal implementation**

Create `cmd/compare.go`:
```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/StrangeNoob/speed-test-cli/internal/history"
	"github.com/StrangeNoob/speed-test-cli/internal/output"
	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

type compareOptions struct {
	last         int
	window       string
	latest       bool
	planDownload float64
	planUpload   float64
	json         bool
	noLog        bool
	noColor      bool
	logFile      string
}

func newCompareCmd() *cobra.Command {
	var o compareOptions
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare a speed test against your past results",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCompare(o)
		},
	}
	f := cmd.Flags()
	f.IntVar(&o.last, "last", 10, "Baseline = median of the last N runs")
	f.StringVar(&o.window, "window", "", "Baseline = runs within this window (7d/24h/30m, or a date)")
	f.BoolVar(&o.latest, "latest", false, "Compare the latest saved run instead of running a new test")
	f.Float64Var(&o.planDownload, "plan-download", 0, "Show performance vs an ISP download plan (Mbps)")
	f.Float64Var(&o.planUpload, "plan-upload", 0, "Show performance vs an ISP upload plan (Mbps)")
	f.BoolVar(&o.json, "json", false, "Machine-readable JSON output")
	f.BoolVar(&o.noLog, "no-log", false, "Don't append the fresh test to history")
	f.BoolVar(&o.noColor, "no-color", false, "Disable colored output")
	f.StringVar(&o.logFile, "log-file", "", "History file (default ~/.speed-test/history.jsonl)")
	cmd.MarkFlagsMutuallyExclusive("last", "window")
	return cmd
}

func runCompare(o compareOptions) error {
	path := o.logFile
	if path == "" {
		p, err := history.DefaultPath()
		if err != nil {
			return err
		}
		path = p
	}
	past, _, err := history.Load(path)
	if err != nil {
		return err
	}

	var current speedtest.Result
	cand := past
	if o.latest {
		if len(past) == 0 {
			fmt.Fprintln(os.Stderr, "No saved speed tests to compare. Run 'speed-test' first.")
			return nil
		}
		current = past[len(past)-1]
		cand = past[:len(past)-1]
	} else {
		res, err := runCompareTest(o)
		if err != nil {
			if o.json {
				_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
			} else {
				fmt.Fprintf(os.Stderr, "speed test failed: %v\n", err)
			}
			return err
		}
		current = res
		if !o.noLog {
			if err := history.Append(path, current); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write history: %v\n", err)
			}
		}
	}

	var baseline []speedtest.Result
	if o.window != "" {
		since, err := history.ParseBound(o.window, false, time.Now())
		if err != nil {
			return err
		}
		baseline = history.Filter(cand, since, time.Time{})
	} else {
		baseline = history.LastN(cand, o.last)
	}

	c := history.Compare(current, baseline)
	plan := history.PlanInfo{Set: o.planDownload > 0 || o.planUpload > 0, Download: o.planDownload, Upload: o.planUpload}

	if o.json {
		return history.RenderCompareJSON(os.Stdout, c, plan)
	}
	st := output.NewStyler(output.ShouldColor(output.IsTerminal(os.Stdout), o.noColor, os.Getenv("NO_COLOR")))
	history.RenderCompare(os.Stdout, c, plan, st)
	return nil
}

// runCompareTest runs a fresh speed test, showing live progress unless --json.
func runCompareTest(o compareOptions) (speedtest.Result, error) {
	client := speedtest.NewClient()
	cfg := speedtest.Config{Streams: 6, Duration: 12 * time.Second}

	animate := !o.json && output.ShouldColor(output.IsTerminal(os.Stderr), o.noColor, os.Getenv("NO_COLOR"))
	var progress speedtest.ProgressFunc
	if !o.json {
		fmt.Fprintln(os.Stderr, "Testing… (Cloudflare)")
		progress = output.NewProgressPrinter(os.Stderr, animate)
	}
	res, err := client.Run(cfg, progress)
	if animate {
		fmt.Fprintln(os.Stderr)
	}
	return res, err
}
```

In `cmd/root.go`, register the command. Find:
```go
	cmd.AddCommand(newHistoryCmd())
```
and add directly after it:
```go
	cmd.AddCommand(newCompareCmd())
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./cmd/ -v && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./...`
Expected: PASS and clean build/vet.

- [ ] **Step 5: Commit**

```bash
git add cmd/compare.go cmd/compare_test.go cmd/root.go
git commit -m "feat(compare): add the compare subcommand"
```

---

## Task 6: Final verification, manual smoke test, README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Full verification**

Run:
```bash
CGO_ENABLED=0 go test ./... -short && CGO_ENABLED=0 go test ./... -race -short && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go build -o speed-test ./cmd/speed-test
```
Expected: all PASS, clean build.

- [ ] **Step 2: Manual smoke test (offline via `--latest`)**

Run:
```bash
mkdir -p /tmp/stcmp
printf '%s\n' \
  '{"timestamp":"2026-06-07T10:00:00Z","server_colo":"AAA","latency_ns":44000000,"jitter_ns":5000000,"download_mbps":130,"upload_mbps":140}' \
  '{"timestamp":"2026-06-07T18:00:00Z","server_colo":"BBB","latency_ns":47000000,"jitter_ns":6000000,"download_mbps":134,"upload_mbps":142}' \
  '{"timestamp":"2026-06-08T09:00:00Z","server_colo":"CCC","latency_ns":56000000,"jitter_ns":6000000,"download_mbps":151,"upload_mbps":136}' \
  > /tmp/stcmp/history.jsonl
L=/tmp/stcmp/history.jsonl
echo "=== compare --latest (table) ==="; ./speed-test compare --latest --log-file "$L"
echo "=== compare --latest --json ==="; ./speed-test compare --latest --json --log-file "$L"
echo "=== with plan ==="; ./speed-test compare --latest --log-file "$L" --plan-download 200 --plan-upload 100
echo "=== window ==="; ./speed-test compare --latest --window 30d --log-file "$L"
echo "=== empty ==="; ./speed-test compare --latest --log-file /tmp/stcmp/none.jsonl
rm -rf /tmp/stcmp
```
Expected: the `--latest` run compares the newest record (151/136, 56ms) against the median of the prior two (132 down, 141 up, 45.5 ping); the table shows a "Compared with last 2 tests" block with per-metric deltas (download better, upload ~normal, ping slower) and a verdict; `--json` is valid JSON with `verdict`/`summary`; the plan section shows `/ 200 Mbps` and `/ 100 Mbps`; the empty case prints the "No saved speed tests" message. (Optionally run a live `./speed-test compare` once if online.)

- [ ] **Step 3: Update README**

In `README.md`, add a usage line under the Usage code block (after the `speed-test history` line):
```markdown
speed-test compare         # is my connection better/normal/worse than usual?
```
And add a new section after the `## History` section:
```markdown
## Compare

`speed-test compare` runs a test and tells you whether your connection is
**better, normal, or degraded** versus the median of your own recent results.

```bash
speed-test compare                       # run a test, compare to the last 10
speed-test compare --last 30             # baseline = median of the last 30
speed-test compare --window 7d           # baseline = runs in the last 7 days
speed-test compare --latest              # compare the most recent saved run (no new test)
speed-test compare --plan-download 200 --plan-upload 100   # vs your ISP plan
speed-test compare --json
```

The baseline uses the **median** (so one bad test doesn't skew it). `--last` and
`--window` are mutually exclusive; `--no-log` skips saving the fresh test.
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: document the compare command"
```

---

## Self-Review Notes

- **Spec coverage:** `Median` (Task 1); `Compare` medians/deltas/per-metric status/overall verdict/`_high_latency`/unstable/empty-baseline/zero-median (Task 2); `Red` + table renderer incl. plan + no-baseline (Task 3); JSON DTO incl. null baseline/delta/plan-omit (Task 4); `compare` command — run-or-`--latest`, `--last`/`--window` exclusive, `--plan-*`, `--json`, `--no-log`, append-on-run, error paths, register (Task 5); verification + offline smoke matrix + README (Task 6). All spec sections covered.
- **Import cycle avoided:** renderers placed in `internal/history` (which imports `output`), not `internal/output`; documented as an intentional spec deviation.
- **Type consistency:** `Median`, `MetricStatus`/`StatusNormal/Better/Worse`, `MetricCompare{Current,Baseline,DeltaPct,Defined,Status}`, `Comparison{...}`, `Compare`, `PlanInfo{Set,Download,Upload}`, `RenderCompare(w,c,plan,*Styler)`, `RenderCompareJSON(w,c,plan)`, `compareOptions`, `runCompare`/`runCompareTest`, `Styler.Red` are used identically across tasks and match the existing `Load`/`LastN`/`Filter`/`ParseBound`/`Append`/`msOf`/`NewProgressPrinter` signatures.
- **No placeholders; no `Co-Authored-By` trailer in any commit.**
