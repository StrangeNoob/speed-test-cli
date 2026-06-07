# History Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `speed-test history` subcommand that reads `~/.speed-test/history.jsonl` and prints a table (default last 20), an avg/min/max summary (`--summary`), or a CSV/JSON export (`--export`, optional `--out`).

**Architecture:** Pure read/stats/format helpers in `internal/history` (unit-tested offline) plus a Cobra subcommand in `cmd/history.go` that wires flags → load → render. Reuses the existing `speedtest.Result` type and `output.Styler`.

**Tech Stack:** Go, `github.com/spf13/cobra`, stdlib `encoding/csv`/`encoding/json`/`bufio`, the existing `internal/output` styler. Tests via stdlib `testing` + `bytes.Buffer` + `t.TempDir()`.

**IMPORTANT ENV:** Prefix every Go command with `CGO_ENABLED=0` (this machine crashes otherwise with a dyld `LC_UUID` error). The repo requires the Go 1.25 toolchain (auto-installed). Do NOT add a `Co-Authored-By` trailer to any commit.

---

## File Structure

| File | Change | Responsibility |
|------|--------|----------------|
| `internal/history/read.go` | new | `Load(path)` (parse JSONL) + `LastN` windowing |
| `internal/history/read_test.go` | new | load/skip/window tests |
| `internal/history/stats.go` | new | `Summary`, `Summarize`, `msOf` ms helper |
| `internal/history/stats_test.go` | new | summary stats tests |
| `internal/history/format.go` | new | `CSV`, `JSON`, `Table`, `RenderSummary` |
| `internal/history/format_test.go` | new | renderer tests |
| `cmd/history.go` | new | `speed-test history` subcommand |
| `cmd/history_test.go` | new | command wiring tests |
| `cmd/root.go` | modify | register `newHistoryCmd()` |
| `README.md` | modify | document the command |

The existing `speedtest.Result` (fields `Timestamp time.Time`, `ServerColo string`, `Latency time.Duration`, `Jitter time.Duration`, `DownloadMbps float64`, `UploadMbps float64`) is the record type. `internal/history/log.go` already has `DefaultPath()`.

---

## Task 1: History reader (`Load` + `LastN`)

**Files:**
- Create: `internal/history/read.go`
- Test: `internal/history/read_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/history/read_test.go`:
```go
package history

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func TestLoadParsesAndSkips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	content := `{"timestamp":"2026-06-07T10:00:00Z","server_colo":"MAA","latency_ns":15000000,"jitter_ns":2000000,"download_mbps":100.5,"upload_mbps":20.2}
not json
{"timestamp":"2026-06-08T10:00:00Z","server_colo":"CCU","latency_ns":21000000,"jitter_ns":4000000,"download_mbps":92.0,"upload_mbps":48.0}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	recs, skipped, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
	if recs[0].ServerColo != "MAA" || recs[1].DownloadMbps != 92.0 {
		t.Errorf("unexpected records: %+v", recs)
	}
}

func TestLoadMissingFile(t *testing.T) {
	recs, skipped, err := Load(filepath.Join(t.TempDir(), "nope.jsonl"))
	if err != nil || recs != nil || skipped != 0 {
		t.Errorf("missing file = (%v,%d,%v), want (nil,0,nil)", recs, skipped, err)
	}
}

func TestLastN(t *testing.T) {
	rs := []speedtest.Result{{ServerColo: "a"}, {ServerColo: "b"}, {ServerColo: "c"}}
	if len(LastN(rs, 0)) != 3 {
		t.Error("n=0 should return all")
	}
	got := LastN(rs, 2)
	if len(got) != 2 || got[0].ServerColo != "b" {
		t.Errorf("n=2 = %+v, want last two (b,c)", got)
	}
	if len(LastN(rs, 100)) != 3 {
		t.Error("n>len should return all")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run 'Load|LastN' -v`
Expected: FAIL — `undefined: Load`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/history/read.go`:
```go
package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// Load reads the history file at path, returning the records in file order
// (oldest first) and the number of malformed lines skipped. A missing file
// returns (nil, 0, nil); a genuine read error is returned. Parsing is
// best-effort: a bad line is counted and skipped, never fatal.
func Load(path string) ([]speedtest.Result, int, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	defer f.Close()

	var records []speedtest.Result
	skipped := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r speedtest.Result
		if err := json.Unmarshal(line, &r); err != nil {
			skipped++
			continue
		}
		records = append(records, r)
	}
	if err := sc.Err(); err != nil {
		return records, skipped, err
	}
	return records, skipped, nil
}

// LastN returns the most recent n records (records are oldest-first, so the
// tail). n <= 0 or n >= len returns all records.
func LastN(records []speedtest.Result, n int) []speedtest.Result {
	if n <= 0 || n >= len(records) {
		return records
	}
	return records[len(records)-n:]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run 'Load|LastN' -v`
Expected: PASS. Also `CGO_ENABLED=0 go vet ./internal/history/`.

- [ ] **Step 5: Commit**

```bash
git add internal/history/read.go internal/history/read_test.go
git commit -m "feat(history): add JSONL reader and LastN windowing"
```

---

## Task 2: Summary stats (`Summarize`)

**Files:**
- Create: `internal/history/stats.go`
- Test: `internal/history/stats_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/history/stats_test.go`:
```go
package history

import (
	"testing"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func TestSummarize(t *testing.T) {
	t1 := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	recs := []speedtest.Result{
		{Timestamp: t2, DownloadMbps: 100, UploadMbps: 40, Latency: 20 * time.Millisecond, Jitter: 4 * time.Millisecond},
		{Timestamp: t1, DownloadMbps: 80, UploadMbps: 60, Latency: 10 * time.Millisecond, Jitter: 2 * time.Millisecond},
	}
	s := Summarize(recs)
	if s.Count != 2 {
		t.Errorf("Count = %d, want 2", s.Count)
	}
	if !s.First.Equal(t1) || !s.Last.Equal(t2) {
		t.Errorf("range = %v..%v, want %v..%v", s.First, s.Last, t1, t2)
	}
	if s.Download != (metricStats{Avg: 90, Min: 80, Max: 100}) {
		t.Errorf("Download = %+v", s.Download)
	}
	if s.Ping != (metricStats{Avg: 15, Min: 10, Max: 20}) {
		t.Errorf("Ping = %+v", s.Ping)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	if Summarize(nil).Count != 0 {
		t.Error("empty input should give Count 0")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run Summarize -v`
Expected: FAIL — `undefined: Summarize` / `metricStats`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/history/stats.go`:
```go
package history

import (
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// metricStats holds the average, minimum, and maximum of one metric.
type metricStats struct{ Avg, Min, Max float64 }

// Summary is the aggregate view of a set of recorded runs.
type Summary struct {
	Count    int
	First    time.Time
	Last     time.Time
	Download metricStats // Mbps
	Upload   metricStats // Mbps
	Ping     metricStats // ms
	Jitter   metricStats // ms
}

// msOf converts a duration to milliseconds as a float.
func msOf(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// acc accumulates one metric's running sum/min/max.
type acc struct {
	sum, min, max float64
	n             int
}

func (a *acc) add(v float64) {
	if a.n == 0 || v < a.min {
		a.min = v
	}
	if a.n == 0 || v > a.max {
		a.max = v
	}
	a.sum += v
	a.n++
}

func (a acc) result() metricStats {
	if a.n == 0 {
		return metricStats{}
	}
	return metricStats{Avg: a.sum / float64(a.n), Min: a.min, Max: a.max}
}

// Summarize computes the count, time range, and avg/min/max per metric. An
// empty slice yields the zero Summary (Count 0), which callers treat as "no data".
func Summarize(records []speedtest.Result) Summary {
	if len(records) == 0 {
		return Summary{}
	}
	s := Summary{Count: len(records), First: records[0].Timestamp, Last: records[0].Timestamp}
	var dl, ul, pg, jt acc
	for _, r := range records {
		if r.Timestamp.Before(s.First) {
			s.First = r.Timestamp
		}
		if r.Timestamp.After(s.Last) {
			s.Last = r.Timestamp
		}
		dl.add(r.DownloadMbps)
		ul.add(r.UploadMbps)
		pg.add(msOf(r.Latency))
		jt.add(msOf(r.Jitter))
	}
	s.Download, s.Upload, s.Ping, s.Jitter = dl.result(), ul.result(), pg.result(), jt.result()
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run Summarize -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/history/stats.go internal/history/stats_test.go
git commit -m "feat(history): add Summarize (count, range, avg/min/max)"
```

---

## Task 3: CSV & JSON renderers

**Files:**
- Create: `internal/history/format.go`
- Test: `internal/history/format_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/history/format_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run 'CSV|JSON' -v`
Expected: FAIL — `undefined: CSV`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/history/format.go`:
```go
package history

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// CSV writes records as CSV with a header row. Empty input writes just the header.
func CSV(w io.Writer, records []speedtest.Result) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"timestamp", "server_colo", "download_mbps", "upload_mbps", "ping_ms", "jitter_ms"}); err != nil {
		return err
	}
	for _, r := range records {
		if err := cw.Write([]string{
			r.Timestamp.Format(time.RFC3339),
			r.ServerColo,
			strconv.FormatFloat(r.DownloadMbps, 'f', -1, 64),
			strconv.FormatFloat(r.UploadMbps, 'f', -1, 64),
			strconv.FormatFloat(msOf(r.Latency), 'f', -1, 64),
			strconv.FormatFloat(msOf(r.Jitter), 'f', -1, 64),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// JSON writes records as a 2-space-indented JSON array. Empty input writes [].
func JSON(w io.Writer, records []speedtest.Result) error {
	if records == nil {
		records = []speedtest.Result{}
	}
	b, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run 'CSV|JSON' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/history/format.go internal/history/format_test.go
git commit -m "feat(history): add CSV and JSON export renderers"
```

---

## Task 4: Table & summary renderers

**Files:**
- Modify: `internal/history/format.go` (append `Table` + `RenderSummary`, extend imports)
- Modify: `internal/history/format_test.go` (append tests)

- [ ] **Step 1: Write the failing test**

Append to `internal/history/format_test.go` (and add `"github.com/StrangeNoob/speed-test-cli/internal/output"` to its import block):
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run 'Table|RenderSummary' -v`
Expected: FAIL — `undefined: Table` / `RenderSummary`.

- [ ] **Step 3: Write minimal implementation**

In `internal/history/format.go`, change the import block to:
```go
import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/output"
	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)
```
Then append:
```go
// Table renders records newest-first as an aligned table. total is the full
// record count before any --last window, used for the footer. The header is
// colored and the footer dimmed via st; a disabled styler emits plain text.
func Table(w io.Writer, records []speedtest.Result, total int, st *output.Styler) {
	fmt.Fprintf(w, "%s\n\n", st.Bold(fmt.Sprintf("Last %d speed tests", len(records))))
	header := fmt.Sprintf("%-17s  %12s  %12s  %8s  %8s", "Date/Time", "Download", "Upload", "Ping", "Jitter")
	fmt.Fprintln(w, st.Cyan(header))
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		fmt.Fprintf(w, "%-17s  %7.1f Mbps  %7.1f Mbps  %5.0f ms  %5.0f ms\n",
			r.Timestamp.Local().Format("02 Jan 2006 15:04"),
			r.DownloadMbps, r.UploadMbps, msOf(r.Latency), msOf(r.Jitter))
	}
	fmt.Fprintf(w, "\n%s\n", st.Dim(fmt.Sprintf("showing %d of %d", len(records), total)))
}

// RenderSummary renders the avg/min/max stats block from a computed Summary.
func RenderSummary(w io.Writer, s Summary, st *output.Styler) {
	dateRange := fmt.Sprintf("%s – %s", s.First.Local().Format("02 Jan"), s.Last.Local().Format("02 Jan"))
	fmt.Fprintf(w, "%s  %s\n\n",
		st.Bold("Speed Test Summary"),
		st.Dim(fmt.Sprintf("(%d runs, %s)", s.Count, dateRange)))
	fmt.Fprintln(w, st.Cyan(fmt.Sprintf("%-10s %8s %8s %8s", "", "Avg", "Min", "Max")))
	row := func(label string, m metricStats, unit string) {
		fmt.Fprintf(w, "%-10s %8.1f %8.1f %8.1f   %s\n", label, m.Avg, m.Min, m.Max, st.Dim(unit))
	}
	row("Download", s.Download, "Mbps")
	row("Upload", s.Upload, "Mbps")
	row("Ping", s.Ping, "ms")
	row("Jitter", s.Jitter, "ms")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/history/ -v`
Expected: PASS (whole package). Also `CGO_ENABLED=0 go vet ./internal/...`.

- [ ] **Step 5: Commit**

```bash
git add internal/history/format.go internal/history/format_test.go
git commit -m "feat(history): add colored table and summary renderers"
```

---

## Task 5: `speed-test history` subcommand

**Files:**
- Create: `cmd/history.go`
- Modify: `cmd/root.go` (register the subcommand)
- Test: `cmd/history_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/history_test.go`:
```go
package cmd

import (
	"io"
	"path/filepath"
	"testing"
)

func TestHistoryHelp(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("`history --help` should work: %v", err)
	}
}

func TestHistorySummaryExportMutuallyExclusive(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--summary", "--export", "csv"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --summary and --export are combined")
	}
}

func TestHistoryEmptyNoError(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", filepath.Join(t.TempDir(), "none.jsonl")})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("empty history should not error: %v", err)
	}
}

func TestHistoryInvalidExport(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--export", "xml", "--log-file", filepath.Join(t.TempDir(), "none.jsonl")})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for --export xml")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./cmd/ -run TestHistory -v`
Expected: FAIL — `history` is an unknown command (not registered yet) / build failure.

- [ ] **Step 3: Write minimal implementation**

Create `cmd/history.go`:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/StrangeNoob/speed-test-cli/internal/history"
	"github.com/StrangeNoob/speed-test-cli/internal/output"
)

type historyOptions struct {
	last    int
	summary bool
	export  string
	out     string
	logFile string
	noColor bool
}

func newHistoryCmd() *cobra.Command {
	var o historyOptions
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show or export recorded speed-test history",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runHistory(o)
		},
	}
	f := cmd.Flags()
	f.IntVar(&o.last, "last", 20, "Show the most recent N runs (0 = all)")
	f.BoolVar(&o.summary, "summary", false, "Print an avg/min/max summary instead of the table")
	f.StringVar(&o.export, "export", "", "Export as 'csv' or 'json' instead of the table")
	f.StringVar(&o.out, "out", "", "With --export, write to this file instead of stdout")
	f.StringVar(&o.logFile, "log-file", "", "History file to read (default ~/.speed-test/history.jsonl)")
	f.BoolVar(&o.noColor, "no-color", false, "Disable colored output")
	cmd.MarkFlagsMutuallyExclusive("summary", "export")
	return cmd
}

const emptyHistoryMsg = "No speed tests recorded yet. Run 'speed-test' to record one."

func runHistory(o historyOptions) error {
	if o.export != "" && o.export != "csv" && o.export != "json" {
		return fmt.Errorf("invalid --export %q (use csv or json)", o.export)
	}

	path := o.logFile
	if path == "" {
		p, err := history.DefaultPath()
		if err != nil {
			return err
		}
		path = p
	}

	records, skipped, err := history.Load(path)
	if err != nil {
		return err
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "(skipped %d unreadable lines)\n", skipped)
	}
	total := len(records)
	window := history.LastN(records, o.last)

	if o.export != "" {
		w := os.Stdout
		if o.out != "" {
			f, err := os.Create(o.out)
			if err != nil {
				return err
			}
			defer f.Close()
			w = f
		}
		if o.export == "csv" {
			return history.CSV(w, window)
		}
		return history.JSON(w, window)
	}

	styler := func() *output.Styler {
		return output.NewStyler(output.ShouldColor(output.IsTerminal(os.Stdout), o.noColor, os.Getenv("NO_COLOR")))
	}

	if o.summary {
		s := history.Summarize(window)
		if s.Count == 0 {
			fmt.Fprintln(os.Stderr, emptyHistoryMsg)
			return nil
		}
		history.RenderSummary(os.Stdout, s, styler())
		return nil
	}

	if len(window) == 0 {
		fmt.Fprintln(os.Stderr, emptyHistoryMsg)
		return nil
	}
	history.Table(os.Stdout, window, total, styler())
	return nil
}
```

In `cmd/root.go`, register the subcommand. Find the line:
```go
	cmd.AddCommand(newUpdateCmd(versionRaw))
```
and add directly after it:
```go
	cmd.AddCommand(newHistoryCmd())
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./cmd/ -v && CGO_ENABLED=0 go build ./...`
Expected: PASS and clean build. Also `CGO_ENABLED=0 go vet ./...`.

- [ ] **Step 5: Commit**

```bash
git add cmd/history.go cmd/history_test.go cmd/root.go
git commit -m "feat(history): add speed-test history subcommand"
```

---

## Task 6: Final verification, manual smoke test, and README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Full verification**

Run:
```bash
CGO_ENABLED=0 go test ./... -short && CGO_ENABLED=0 go test ./... -race -short && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go build -o speed-test ./cmd/speed-test
```
Expected: all PASS, clean build.

- [ ] **Step 2: Manual smoke test against a sample history file**

Run:
```bash
mkdir -p /tmp/sthist
printf '%s\n' \
  '{"timestamp":"2026-06-07T22:15:00Z","server_colo":"CCU","latency_ns":24000000,"jitter_ns":6000000,"download_mbps":87.0,"upload_mbps":45.3}' \
  '{"timestamp":"2026-06-08T10:30:00Z","server_colo":"MAA","latency_ns":21000000,"jitter_ns":4000000,"download_mbps":92.4,"upload_mbps":48.1}' \
  > /tmp/sthist/history.jsonl

echo "--- table ---";    ./speed-test history --log-file /tmp/sthist/history.jsonl
echo "--- summary ---";  ./speed-test history --log-file /tmp/sthist/history.jsonl --summary
echo "--- csv ---";      ./speed-test history --log-file /tmp/sthist/history.jsonl --export csv
echo "--- json ---";     ./speed-test history --log-file /tmp/sthist/history.jsonl --export json
echo "--- csv piped (must be plain, no escapes) ---"; ./speed-test history --log-file /tmp/sthist/history.jsonl --export csv | cat
echo "--- empty ---";    ./speed-test history --log-file /tmp/sthist/none.jsonl
rm -rf /tmp/sthist
```
Expected: a 2-row table newest-first (08 Jun on top) with `showing 2 of 2`; a summary block with averages; valid CSV (header + 2 rows) and JSON array; the empty case prints `No speed tests recorded yet…`. (Skip nothing — this is all offline.)

- [ ] **Step 3: Update README**

In `README.md`, add a usage line under the Usage code block (after the `speed-test --version` line):
```markdown
speed-test history        # show recent runs (table)
```
And add a new section after the `## Updating` section:
```markdown
## History

Every run is appended to `~/.speed-test/history.jsonl`. View and analyze it:

```bash
speed-test history                 # table of the last 20 runs (newest first)
speed-test history --last 50       # last 50 (use --last 0 for all)
speed-test history --summary       # avg/min/max for download, upload, ping, jitter
speed-test history --export csv     > runs.csv   # or: --out runs.csv
speed-test history --export json    > runs.json
```

`--log-file` reads a different file; `--no-color` (or piping / `NO_COLOR`)
disables coloring.
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: document the history command"
```

---

## Self-Review Notes

- **Spec coverage:** `Load` + skip count + `LastN` (Task 1); `Summary`/`Summarize` count/range/avg-min-max (Task 2); `CSV`/`JSON` incl. empty header/`[]` (Task 3); `Table` (newest-first, `showing X of Y`, 1-decimal Mbps / integer ms, header+footer color) + `RenderSummary` (Task 4); subcommand with `--last`/`--summary`/`--export`/`--out`/`--log-file`/`--no-color`, mutual exclusion, invalid-export error, empty-history message, skipped-line note (Task 5); verification + manual matrix + README (Task 6). All spec sections covered.
- **Type consistency:** `Load(path) ([]speedtest.Result, int, error)`, `LastN(records, n)`, `Summary{Count,First,Last,Download,Upload,Ping,Jitter}`, `metricStats{Avg,Min,Max}`, `Summarize(records)`, `msOf(d)`, `CSV(w,records)`, `JSON(w,records)`, `Table(w,records,total,*output.Styler)`, `RenderSummary(w,Summary,*output.Styler)`, `newHistoryCmd()` are used identically across tasks and match the cmd call sites.
- **Import-cycle check:** `internal/history` importing `internal/output` is safe — `output` imports only `speedtest`/stdlib, never `history`.
- **Ordering:** `msOf` is defined in Task 2 (stats.go) and used by Task 3 (format.go CSV); Task 2 precedes Task 3. Task 4 extends the Task 3 file's imports (adds `fmt`, `output`).
- **No placeholders; no `Co-Authored-By` trailer in any commit.**
