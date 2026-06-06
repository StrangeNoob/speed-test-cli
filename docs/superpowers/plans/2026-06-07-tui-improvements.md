# TUI Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the speed-test CLI a polished, colored, Unicode-bar terminal UI (live progress + summary) using only the standard library, auto-disabling color/animation when output is not a terminal.

**Architecture:** All work stays in `internal/output` plus flag wiring in `cmd`. A new `style.go` holds the rendering toolkit (`Styler`, `renderBar`, `ShouldColor`, `IsTerminal`, spinner). `human.go` uses it for the summary and a stateful live progress printer. No new dependencies; TTY detection via `os.ModeCharDevice`.

**Tech Stack:** Go 1.22+, stdlib only (`fmt`, `os`, `strings`). ANSI escape codes for color, Unicode 1/8-block glyphs for bars, braille spinner. Tests via stdlib `testing` + `bytes.Buffer`.

**IMPORTANT ENV:** This machine requires `CGO_ENABLED=0` for all `go test`/`go build`/`go vet` commands (otherwise binaries crash with a dyld `LC_UUID` error). Prefix every Go command with `CGO_ENABLED=0`. A `Makefile` exists with this baked in.

---

## File Structure

| File | Change | Responsibility |
|------|--------|----------------|
| `internal/output/style.go` | new | `Styler` (ANSI on/off), `renderBar`, `ShouldColor`, `IsTerminal`, `spinnerFrames` |
| `internal/output/style_test.go` | new | unit tests for the toolkit |
| `internal/output/human.go` | modified | `Human(w, res, *Styler)` summary + `NewProgressPrinter(w, animate)` live printer |
| `internal/output/human_test.go` | modified | updated for new signatures + live-printer test |
| `cmd/root.go` | modified | `--no-color` flag, per-stream color decision, wiring |
| `cmd/root_test.go` | modified | assert `--no-color` flag parses |

---

## Task 1: Bar renderer

**Files:**
- Create: `internal/output/style.go`
- Test: `internal/output/style_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/output/style_test.go`:
```go
package output

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRenderBarWidth(t *testing.T) {
	// Every result must be exactly `width` runes regardless of value.
	for _, tc := range []struct {
		value, max float64
	}{
		{0, 100}, {50, 100}, {100, 100}, {150, 100}, {10, 0},
	} {
		got := renderBar(tc.value, tc.max, 10)
		if n := utf8.RuneCountInString(got); n != 10 {
			t.Errorf("renderBar(%v,%v,10) width = %d runes, want 10 (%q)", tc.value, tc.max, n, got)
		}
	}
}

func TestRenderBarFull(t *testing.T) {
	got := renderBar(100, 100, 8)
	if got != strings.Repeat("█", 8) {
		t.Errorf("full bar = %q, want 8 full blocks", got)
	}
}

func TestRenderBarEmpty(t *testing.T) {
	got := renderBar(0, 100, 8)
	if got != strings.Repeat(" ", 8) {
		t.Errorf("empty bar = %q, want 8 spaces", got)
	}
}

func TestRenderBarClampsOverMax(t *testing.T) {
	if renderBar(999, 100, 6) != strings.Repeat("█", 6) {
		t.Errorf("value over max should clamp to full bar")
	}
}

func TestRenderBarZeroMax(t *testing.T) {
	if renderBar(50, 0, 6) != strings.Repeat(" ", 6) {
		t.Errorf("max<=0 should render an all-space bar")
	}
}

func TestRenderBarHalf(t *testing.T) {
	// 50% of 8 cells = 4 full blocks then 4 spaces.
	got := renderBar(50, 100, 8)
	if got != strings.Repeat("█", 4)+strings.Repeat(" ", 4) {
		t.Errorf("half bar = %q, want 4 full + 4 spaces", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestRenderBar -v`
Expected: FAIL — `undefined: renderBar`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/output/style.go`:
```go
package output

import "strings"

// barGlyphs are the eight fractional fill levels (1/8 .. 8/8 of a cell).
var barGlyphs = []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

// renderBar returns a bar of exactly `width` runes representing value/max.
// Filled cells use '█', the final partial cell uses a fractional glyph, and
// empty cells use a space. value>max clamps to full; max<=0 renders all spaces.
func renderBar(value, max float64, width int) string {
	if width <= 0 {
		return ""
	}
	if max <= 0 {
		return strings.Repeat(" ", width)
	}
	frac := value / max
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}

	// Total eighths of a cell to fill across the whole bar.
	totalEighths := int(frac * float64(width) * 8)
	full := totalEighths / 8
	rem := totalEighths % 8

	var b strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i < full:
			b.WriteRune('█')
		case i == full && rem > 0:
			b.WriteRune(barGlyphs[rem-1])
		default:
			b.WriteRune(' ')
		}
	}
	return b.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestRenderBar -v`
Expected: PASS (all six). Also `CGO_ENABLED=0 go vet ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/output/style.go internal/output/style_test.go
git commit -m "feat: add Unicode progress bar renderer"
```

---

## Task 2: Color decision (ShouldColor) and TTY detection

**Files:**
- Modify: `internal/output/style.go`
- Test: `internal/output/style_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/output/style_test.go`:
```go
func TestShouldColor(t *testing.T) {
	for _, tc := range []struct {
		name      string
		isTTY     bool
		noColor   bool
		noColorNV string
		want      bool
	}{
		{"tty no flags", true, false, "", true},
		{"not a tty", false, false, "", false},
		{"flag set", true, true, "", false},
		{"NO_COLOR set", true, false, "1", false},
		{"NO_COLOR empty value but present treated as unset", true, false, "", true},
		{"all off", false, true, "1", false},
	} {
		if got := ShouldColor(tc.isTTY, tc.noColor, tc.noColorNV); got != tc.want {
			t.Errorf("%s: ShouldColor(%v,%v,%q) = %v, want %v",
				tc.name, tc.isTTY, tc.noColor, tc.noColorNV, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestShouldColor -v`
Expected: FAIL — `undefined: ShouldColor`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/output/style.go` (add `"os"` to the import block — change `import "strings"` to a grouped import):
```go
import (
	"os"
	"strings"
)
```
Then add:
```go
// ShouldColor reports whether colored/animated output should be used.
// noColorEnv is the raw value of the NO_COLOR environment variable; per the
// NO_COLOR convention, any non-empty value disables color.
func ShouldColor(isTTY, noColorFlag bool, noColorEnv string) bool {
	return isTTY && !noColorFlag && noColorEnv == ""
}

// IsTerminal reports whether f refers to a character device (a terminal).
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestShouldColor -v`
Expected: PASS. Also `CGO_ENABLED=0 go vet ./...` (confirm import group is valid).

- [ ] **Step 5: Commit**

```bash
git add internal/output/style.go internal/output/style_test.go
git commit -m "feat: add ShouldColor decision and TTY detection"
```

---

## Task 3: Styler (ANSI color on/off)

**Files:**
- Modify: `internal/output/style.go`
- Test: `internal/output/style_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/output/style_test.go`:
```go
func TestStylerEnabledWraps(t *testing.T) {
	st := NewStyler(true)
	got := st.Cyan("hi")
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("enabled Cyan should contain an ANSI escape, got %q", got)
	}
	if !strings.Contains(got, "hi") {
		t.Errorf("enabled Cyan should still contain the text, got %q", got)
	}
}

func TestStylerDisabledPlain(t *testing.T) {
	st := NewStyler(false)
	for _, got := range []string{st.Cyan("x"), st.Green("x"), st.Dim("x"), st.Bold("x")} {
		if got != "x" {
			t.Errorf("disabled styler should return raw text, got %q", got)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestStyler -v`
Expected: FAIL — `undefined: NewStyler`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/output/style.go`:
```go
// Styler wraps text in ANSI color codes when enabled, or returns it unchanged.
type Styler struct {
	enabled bool
}

// NewStyler returns a Styler that emits color codes only when enabled.
func NewStyler(enabled bool) *Styler {
	return &Styler{enabled: enabled}
}

func (s *Styler) wrap(code, text string) string {
	if !s.enabled {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func (s *Styler) Cyan(text string) string  { return s.wrap("36", text) }
func (s *Styler) Green(text string) string { return s.wrap("32", text) }
func (s *Styler) Dim(text string) string   { return s.wrap("2", text) }
func (s *Styler) Bold(text string) string  { return s.wrap("1", text) }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestStyler -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/style.go internal/output/style_test.go
git commit -m "feat: add Styler for ANSI color on/off"
```

---

## Task 4: Spinner frames

**Files:**
- Modify: `internal/output/style.go`
- Test: `internal/output/style_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/output/style_test.go`:
```go
func TestSpinnerFramesNonEmpty(t *testing.T) {
	if len(spinnerFrames) == 0 {
		t.Fatal("spinnerFrames must not be empty")
	}
	for i, f := range spinnerFrames {
		if f == "" {
			t.Errorf("spinnerFrames[%d] is empty", i)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestSpinnerFrames -v`
Expected: FAIL — `undefined: spinnerFrames`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/output/style.go`:
```go
// spinnerFrames is a braille spinner animation cycle.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestSpinnerFrames -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/style.go internal/output/style_test.go
git commit -m "feat: add braille spinner frames"
```

---

## Task 5: Summary rendering (Human) with bars + color

**Files:**
- Modify: `internal/output/human.go`
- Modify: `internal/output/human_test.go`

- [ ] **Step 1: Update the existing summary test and add a color test**

The current `internal/output/human_test.go` has `TestHumanSummaryContainsMetrics` calling `Human(&buf, res)`. REPLACE that test function with the two below (keep the file's package clause and imports; add `"strings"` if not present — it is already imported). The existing `TestProgressPrinterUpdates` test stays untouched in this task (Task 6 handles it).

Replace `TestHumanSummaryContainsMetrics` with:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestHumanSummary -v`
Expected: FAIL — too many arguments to `Human` (signature mismatch / build failure).

- [ ] **Step 3: Write minimal implementation**

In `internal/output/human.go`, REPLACE the existing `Human` function (lines defining `func Human(w io.Writer, res speedtest.Result)`) with:
```go
// barWidth is the cell width of summary throughput bars.
const barWidth = 16

// Human writes a clean, colored, human-readable summary of the result to w.
// Pass NewStyler(false) for plain output.
func Human(w io.Writer, res speedtest.Result, st *Styler) {
	if res.ServerColo != "" {
		fmt.Fprintf(w, "%s  %s  %s %s\n\n",
			st.Bold("speed-test"), st.Dim("•"), st.Dim("Cloudflare"), st.Bold(res.ServerColo))
	}
	fmt.Fprintf(w, "%s  %s    %s  %s\n\n",
		st.Cyan("Ping   "), fmtMs(res.Latency),
		st.Cyan("Jitter"), fmtMs(res.Jitter))

	scale := res.DownloadMbps
	if res.UploadMbps > scale {
		scale = res.UploadMbps
	}

	// A Result can't distinguish a skipped direction from a measured zero
	// (both are 0.0), so any zero renders a dash instead of a bar.
	writeRate(w, st, "Download", res.DownloadMbps, scale)
	writeRate(w, st, "Upload  ", res.UploadMbps, scale)
}

// writeRate renders one labeled throughput row with a scaled bar.
func writeRate(w io.Writer, st *Styler, label string, mbps, scale float64) {
	if mbps <= 0 {
		fmt.Fprintf(w, "%s  %s  %s\n", st.Cyan(label), st.Dim("▕"+spaces(barWidth)+"▏"), st.Dim("—"))
		return
	}
	bar := renderBar(mbps, scale, barWidth)
	fmt.Fprintf(w, "%s  %s%s%s  %.1f %s\n",
		st.Cyan(label), st.Dim("▕"), st.Green(bar), st.Dim("▏"), mbps, st.Dim("Mbps"))
}

func fmtMs(d time.Duration) string {
	return fmt.Sprintf("%.1f ms", float64(d.Microseconds())/1000)
}

func spaces(n int) string {
	return strings.Repeat(" ", n)
}
```
Update the `import` block of `human.go` to include `time` and `strings` (it currently imports `fmt`, `io`, and the speedtest package):
```go
import (
	"fmt"
	"io"
	"strings"
	"time"

	"speed-test-cli/internal/speedtest"
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestHumanSummary -v`
Expected: PASS (both). Also `CGO_ENABLED=0 go vet ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/output/human.go internal/output/human_test.go
git commit -m "feat: render colored summary with scaled bars"
```

---

## Task 6: Live progress printer (spinner + auto-scaling bar)

**Files:**
- Modify: `internal/output/human.go`
- Modify: `internal/output/human_test.go`

- [ ] **Step 1: Update the progress test**

In `internal/output/human_test.go`, REPLACE the existing `TestProgressPrinterUpdates` with the following two tests:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/output/ -run TestProgressPrinter -v`
Expected: FAIL — `NewProgressPrinter` signature mismatch (now needs a second arg) / build failure.

- [ ] **Step 3: Write minimal implementation**

In `internal/output/human.go`, REPLACE the existing `NewProgressPrinter` function with:
```go
// liveBarWidth is the cell width of the live progress bar.
const liveBarWidth = 20

// NewProgressPrinter returns a ProgressFunc that renders a live, single-line
// spinner + auto-scaling bar + throughput to w. When animate is false it
// produces no output. The bar scales to the peak Mbps seen so far in the
// current phase; a phase change finalizes the previous line with a newline.
func NewProgressPrinter(w io.Writer, animate bool) speedtest.ProgressFunc {
	st := NewStyler(true)
	var current speedtest.Phase
	var peak float64
	frame := 0
	label := map[speedtest.Phase]string{
		speedtest.PhaseDownload: "Download",
		speedtest.PhaseUpload:   "Upload  ",
	}
	return func(p speedtest.Progress) {
		if !animate {
			return
		}
		if p.Phase != current {
			if current != "" {
				fmt.Fprint(w, "\n")
			}
			current = p.Phase
			peak = 0
		}
		if p.Mbps > peak {
			peak = p.Mbps
		}
		spin := spinnerFrames[frame%len(spinnerFrames)]
		frame++
		bar := renderBar(p.Mbps, peak, liveBarWidth)
		name := label[p.Phase]
		if name == "" {
			name = string(p.Phase)
		}
		fmt.Fprintf(w, "\r%s  %s  %s%s%s  %.1f %s   ",
			st.Cyan(spin), st.Cyan(name),
			st.Dim("▕"), st.Green(bar), st.Dim("▏"),
			p.Mbps, st.Dim("Mbps"))
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/output/ -v`
Expected: PASS (entire output package). Also `CGO_ENABLED=0 go vet ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/output/human.go internal/output/human_test.go
git commit -m "feat: animated live progress printer with auto-scaling bar"
```

---

## Task 7: Wire color/animation into the CLI

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/root_test.go`

- [ ] **Step 1: Write the failing test**

Add to `cmd/root_test.go`:
```go
func TestNoColorFlagParses(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--no-color", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--no-color should be a valid flag, got: %v", err)
	}
}
```
(`io` is already imported in `cmd/root_test.go` from the mutually-exclusive test added earlier.)

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./cmd/ -run TestNoColorFlag -v`
Expected: FAIL — unknown flag `--no-color`.

- [ ] **Step 3: Write minimal implementation**

In `cmd/root.go`:

(a) Add the field to the `options` struct (after `uploadOnly`):
```go
	noColor      bool
```

(b) Register the flag inside `newRootCmd()` alongside the other `f.BoolVar` calls:
```go
	f.BoolVar(&o.noColor, "no-color", false, "Disable colored output")
```

(c) Replace the body of `run`'s progress/output section. The current code is:
```go
	var progress speedtest.ProgressFunc
	if !o.json {
		fmt.Fprintln(os.Stderr, "Testing… (Cloudflare)")
		progress = output.NewProgressPrinter(os.Stderr)
	}

	res, err := client.Run(o.toConfig(), progress)
	if err != nil {
		if o.json {
			enc := json.NewEncoder(os.Stderr)
			_ = enc.Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Fprintf(os.Stderr, "speed test failed: %v\n", err)
		}
		return err
	}

	if !o.json {
		fmt.Fprintln(os.Stderr)
	}

	if o.json {
		if err := output.JSON(os.Stdout, res); err != nil {
			return err
		}
	} else {
		output.Human(os.Stdout, res)
	}
```
Replace it with:
```go
	noColorEnv := os.Getenv("NO_COLOR")
	animate := output.ShouldColor(output.IsTerminal(os.Stderr), o.noColor, noColorEnv)

	var progress speedtest.ProgressFunc
	if !o.json {
		fmt.Fprintln(os.Stderr, "Testing… (Cloudflare)")
		progress = output.NewProgressPrinter(os.Stderr, animate)
	}

	res, err := client.Run(o.toConfig(), progress)
	if err != nil {
		if o.json {
			enc := json.NewEncoder(os.Stderr)
			_ = enc.Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Fprintf(os.Stderr, "speed test failed: %v\n", err)
		}
		return err
	}

	if o.json {
		if err := output.JSON(os.Stdout, res); err != nil {
			return err
		}
	} else {
		if animate {
			fmt.Fprintln(os.Stderr)
		}
		summarySt := output.NewStyler(output.ShouldColor(output.IsTerminal(os.Stdout), o.noColor, noColorEnv))
		output.Human(os.Stdout, res, summarySt)
	}
```
(The trailing-newline-after-Run is now inside the non-json branch and only printed when `animate` was on, so a piped run doesn't emit a stray blank line.)

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./cmd/ -v && CGO_ENABLED=0 go build ./...`
Expected: PASS and clean build. Also `CGO_ENABLED=0 go vet ./...`.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go cmd/root_test.go
git commit -m "feat: wire --no-color and TTY-aware color into the CLI"
```

---

## Task 8: Final verification, manual smoke test, README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Run the full suite (short) + race + vet + build**

Run:
```bash
CGO_ENABLED=0 go test ./... -short && CGO_ENABLED=0 go test ./... -race -short && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go build -o speed-test .
```
Expected: all PASS, clean build.

- [ ] **Step 2: Smoke-test the new UI (requires network)**

Run (TTY — colored, animated):
```bash
CGO_ENABLED=0 ./speed-test --download-only --duration 5s --no-log
```
Expected: animated spinner + green bar while testing; a colored summary with a full Download bar. (Skip if offline.)

Run (piped — must be plain, no escape codes, no stray blank line):
```bash
CGO_ENABLED=0 ./speed-test --download-only --duration 5s --no-log | cat -v
```
Expected: no `^[` escape sequences in the output. (`cat -v` makes escapes visible.)

Run (explicit flag):
```bash
CGO_ENABLED=0 ./speed-test --download-only --duration 5s --no-log --no-color
```
Expected: plain summary even in a terminal.

- [ ] **Step 3: Update the README flags table and a UI note**

In `README.md`, add a `--no-color` row to the Flags table (after the `--upload-only` row):
```markdown
| `--no-color` | false | Disable colored output |
```
And add this line directly under the Usage code block (after the existing history sentence):
```markdown
Output is colored with live progress bars when run in a terminal; colors and
animation are disabled automatically when piped/redirected, when `NO_COLOR` is
set, or with `--no-color`.
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: document --no-color and colored TUI"
```

---

## Self-Review Notes

- **Spec coverage:** `renderBar` (Task 1), `ShouldColor`/`IsTerminal` (Task 2), `Styler` (Task 3), spinner (Task 4), colored summary with `max(down,up)` scale + skipped/zero handling (Task 5), animated live printer with peak-scaling + phase-boundary newline + non-animate silence (Task 6), `--no-color` flag + per-stream TTY decision + json untouched (Task 7), verification/piped-plain check/README (Task 8). All spec sections covered.
- **Type consistency:** `Styler`/`NewStyler`/`Cyan/Green/Dim/Bold`, `renderBar(value,max,width)`, `ShouldColor(isTTY,noColorFlag,noColorEnv)`, `IsTerminal(*os.File)`, `spinnerFrames`, `Human(w,res,*Styler)`, `NewProgressPrinter(w,animate)` are used identically across tasks and match the cmd call sites in Task 7.
- **No placeholders:** every code step shows full code; commands include expected output.
- **Existing-test updates are explicit:** Task 5 replaces `TestHumanSummaryContainsMetrics`; Task 6 replaces `TestProgressPrinterUpdates`; both note exactly what changes.
