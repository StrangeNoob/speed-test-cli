# History Date-Range Filtering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--since`/`--until` date-range filtering to `speed-test history`, accepting absolute dates, `YYYY-MM-DD HH:MM`, and relative durations (7d/24h/30m), composing with `--last`/`--summary`/`--export`.

**Architecture:** A pure `filter.go` in `internal/history` (`ParseBound` + `Filter`, unit-tested with an injected `now`); `cmd/history.go` parses the flags and applies the filter before `--last` windowing, with a distinct "none in range" message.

**Tech Stack:** Go, `github.com/spf13/cobra`, stdlib `time`/`regexp`. Tests via stdlib `testing`.

**IMPORTANT ENV:** Prefix every Go command with `CGO_ENABLED=0` (this machine crashes otherwise with a dyld `LC_UUID` error; the Go 1.25 toolchain auto-installs). Do NOT add a `Co-Authored-By` trailer (or any Claude attribution) to commit messages.

---

## File Structure

| File | Change | Responsibility |
|------|--------|----------------|
| `internal/history/filter.go` | new | `ParseBound(s, end, now)` + `Filter(records, since, until)` |
| `internal/history/filter_test.go` | new | parse + filter tests |
| `cmd/history.go` | modify | `--since`/`--until` flags, parse → Filter → LastN, "no match" message |
| `cmd/history_test.go` | modify | flag/filter wiring tests |
| `README.md` | modify | document the date flags |

The record type is `speedtest.Result` with field `Timestamp time.Time`. Existing
helpers: `history.Load`, `history.LastN`, `history.Summarize`, `history.CSV`,
`history.JSON`, `history.Table`, `history.RenderSummary`, `history.DefaultPath`.

---

## Task 1: `ParseBound` + `Filter`

**Files:**
- Create: `internal/history/filter.go`
- Test: `internal/history/filter_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/history/filter_test.go`:
```go
package history

import (
	"testing"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func TestParseBoundEmpty(t *testing.T) {
	got, err := ParseBound("", false, time.Now())
	if err != nil || !got.IsZero() {
		t.Errorf("empty = (%v,%v), want (zero,nil)", got, err)
	}
}

func TestParseBoundRelative(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.Local)
	cases := map[string]time.Duration{"7d": 7 * 24 * time.Hour, "24h": 24 * time.Hour, "30m": 30 * time.Minute}
	for in, want := range cases {
		got, err := ParseBound(in, false, now)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if !got.Equal(now.Add(-want)) {
			t.Errorf("%s = %v, want %v", in, got, now.Add(-want))
		}
	}
}

func TestParseBoundDate(t *testing.T) {
	now := time.Now()
	start, err := ParseBound("2026-06-01", false, now)
	if err != nil || !start.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)) {
		t.Errorf("since date = %v (%v), want start of day", start, err)
	}
	end, err := ParseBound("2026-06-01", true, now)
	wantEnd := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local).Add(24*time.Hour - time.Nanosecond)
	if err != nil || !end.Equal(wantEnd) {
		t.Errorf("until date = %v (%v), want end of day", end, err)
	}
}

func TestParseBoundDateTime(t *testing.T) {
	now := time.Now()
	want := time.Date(2026, 6, 1, 15, 30, 0, 0, time.Local)
	for _, end := range []bool{false, true} {
		got, err := ParseBound("2026-06-01 15:30", end, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("datetime end=%v = %v (%v), want %v", end, got, err, want)
		}
	}
}

func TestParseBoundInvalid(t *testing.T) {
	for _, s := range []string{"nonsense", "5x", "2026-13-99"} {
		if _, err := ParseBound(s, false, time.Now()); err == nil {
			t.Errorf("%q should error", s)
		}
	}
}

func TestFilter(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2026, 6, day, 12, 0, 0, 0, time.UTC) }
	recs := []speedtest.Result{{Timestamp: d(1)}, {Timestamp: d(5)}, {Timestamp: d(9)}}

	if got := Filter(recs, time.Time{}, time.Time{}); len(got) != 3 {
		t.Errorf("no bounds = %d, want 3", len(got))
	}
	if got := Filter(recs, d(5), time.Time{}); len(got) != 2 {
		t.Errorf("since=5 = %d, want 2 (5,9)", len(got))
	}
	if got := Filter(recs, time.Time{}, d(5)); len(got) != 2 {
		t.Errorf("until=5 = %d, want 2 (1,5)", len(got))
	}
	if got := Filter(recs, d(2), d(6)); len(got) != 1 || !got[0].Timestamp.Equal(d(5)) {
		t.Errorf("[2,6] = %+v, want only day 5", got)
	}
	if got := Filter(recs, d(5), d(5)); len(got) != 1 {
		t.Errorf("[5,5] = %d, want 1 (inclusive)", len(got))
	}
	if got := Filter(recs, d(9), d(1)); len(got) != 0 {
		t.Errorf("since>until = %d, want 0", len(got))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run 'ParseBound|Filter' -v`
Expected: FAIL — `undefined: ParseBound`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/history/filter.go`:
```go
package history

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// relativeRe matches relative durations like "7d", "24h", "30m".
var relativeRe = regexp.MustCompile(`^(\d+)(d|h|m)$`)

// ParseBound parses a date-range bound. An empty string returns the zero time
// (unbounded). Relative durations (7d/24h/30m) return now minus that duration.
// "YYYY-MM-DD HH:MM" returns that exact local time. "YYYY-MM-DD" returns the
// start of that day for a lower bound (end=false) or the end of that day
// (end=true), so a bare --until date includes the whole day. Local time is used.
func ParseBound(s string, end bool, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if m := relativeRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		var unit time.Duration
		switch m[2] {
		case "d":
			unit = 24 * time.Hour
		case "h":
			unit = time.Hour
		case "m":
			unit = time.Minute
		}
		return now.Add(-time.Duration(n) * unit), nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		if end {
			return t.Add(24*time.Hour - time.Nanosecond), nil
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD, \"YYYY-MM-DD HH:MM\", or a duration like 7d/24h/30m", s)
}

// Filter returns the records whose Timestamp falls within [since, until],
// inclusive. A zero since or until means unbounded on that side. The result is
// a new slice preserving the original order.
func Filter(records []speedtest.Result, since, until time.Time) []speedtest.Result {
	out := make([]speedtest.Result, 0, len(records))
	for _, r := range records {
		if !since.IsZero() && r.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && r.Timestamp.After(until) {
			continue
		}
		out = append(out, r)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/history/ -run 'ParseBound|Filter' -v`
Expected: PASS. Also `CGO_ENABLED=0 go vet ./internal/history/`.

- [ ] **Step 5: Commit**

```bash
git add internal/history/filter.go internal/history/filter_test.go
git commit -m "feat(history): add ParseBound and Filter for date ranges"
```

---

## Task 2: Wire `--since`/`--until` into the command

**Files:**
- Modify: `cmd/history.go`
- Modify: `cmd/history_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/history_test.go` (and ensure its import block is exactly
`"io"`, `"os"`, `"path/filepath"`, `"strings"`, `"testing"`):
```go
func writeHistory(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	content := `{"timestamp":"2026-06-01T12:00:00Z","download_mbps":10}
{"timestamp":"2026-06-05T12:00:00Z","download_mbps":20}
{"timestamp":"2026-06-09T12:00:00Z","download_mbps":30}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHistorySinceFiltersExport(t *testing.T) {
	hist := writeHistory(t)
	out := filepath.Join(filepath.Dir(hist), "out.csv")
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", hist, "--since", "2026-06-05", "--export", "csv", "--out", out})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("history --since --export: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "2026-06-01") {
		t.Errorf("June 1 should be filtered out:\n%s", s)
	}
	if !strings.Contains(s, "2026-06-05") || !strings.Contains(s, "2026-06-09") {
		t.Errorf("June 5 and 9 should be present:\n%s", s)
	}
}

func TestHistoryUntilFiltersExport(t *testing.T) {
	hist := writeHistory(t)
	out := filepath.Join(filepath.Dir(hist), "out.csv")
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", hist, "--until", "2026-06-05", "--export", "csv", "--out", out})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := string(mustRead(t, out))
	if strings.Contains(s, "2026-06-09") {
		t.Errorf("June 9 should be filtered out by --until:\n%s", s)
	}
	if !strings.Contains(s, "2026-06-01") || !strings.Contains(s, "2026-06-05") {
		t.Errorf("June 1 and 5 should be present (until is end-of-day inclusive):\n%s", s)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHistoryInvalidSince(t *testing.T) {
	hist := writeHistory(t)
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", hist, "--since", "nonsense"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for --since nonsense")
	}
}

func TestHistorySinceNoMatchNoError(t *testing.T) {
	hist := writeHistory(t)
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", hist, "--since", "2030-01-01"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("no-match range should not error: %v", err)
	}
}

func TestHistorySinceSummary(t *testing.T) {
	hist := writeHistory(t)
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", hist, "--since", "2026-06-05", "--summary"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--since with --summary should work: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./cmd/ -run TestHistory -v`
Expected: FAIL — `unknown flag: --since` (and build errors for the new helpers).

- [ ] **Step 3: Write minimal implementation**

In `cmd/history.go`:

(a) Add `"time"` to the import block:
```go
import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/StrangeNoob/speed-test-cli/internal/history"
	"github.com/StrangeNoob/speed-test-cli/internal/output"
)
```

(b) Add two fields to `historyOptions` (after `noColor bool`):
```go
	since   string
	until   string
```

(c) In `newHistoryCmd`, register the flags alongside the others (before
`cmd.MarkFlagsMutuallyExclusive(...)`):
```go
	f.StringVar(&o.since, "since", "", "Only runs at/after this time (YYYY-MM-DD, 'YYYY-MM-DD HH:MM', or 7d/24h/30m)")
	f.StringVar(&o.until, "until", "", "Only runs at/before this time (same formats; a bare date includes the whole day)")
```

(d) Add the no-match message constant next to `emptyHistoryMsg`:
```go
const noMatchMsg = "No speed tests match that range."
```

(e) REPLACE the entire `runHistory` function with:
```go
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
		noun := "lines"
		if skipped == 1 {
			noun = "line"
		}
		fmt.Fprintf(os.Stderr, "(skipped %d unreadable %s)\n", skipped, noun)
	}

	now := time.Now()
	since, err := history.ParseBound(o.since, false, now)
	if err != nil {
		return err
	}
	until, err := history.ParseBound(o.until, true, now)
	if err != nil {
		return err
	}

	filtered := history.Filter(records, since, until)
	total := len(filtered)
	window := history.LastN(filtered, o.last)

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

	// Distinguish "no history at all" from "none in the chosen range".
	emptyMsg := emptyHistoryMsg
	if len(records) > 0 {
		emptyMsg = noMatchMsg
	}

	if o.summary {
		s := history.Summarize(window)
		if s.Count == 0 {
			fmt.Fprintln(os.Stderr, emptyMsg)
			return nil
		}
		history.RenderSummary(os.Stdout, s, styler())
		return nil
	}

	if len(window) == 0 {
		fmt.Fprintln(os.Stderr, emptyMsg)
		return nil
	}
	history.Table(os.Stdout, window, total, styler())
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./cmd/ -v && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./...`
Expected: PASS and clean build/vet.

- [ ] **Step 5: Commit**

```bash
git add cmd/history.go cmd/history_test.go
git commit -m "feat(history): add --since/--until date-range filtering"
```

---

## Task 3: Final verification, manual smoke test, README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Full verification**

Run:
```bash
CGO_ENABLED=0 go test ./... -short && CGO_ENABLED=0 go test ./... -race -short && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go build -o speed-test ./cmd/speed-test
```
Expected: all PASS, clean build.

- [ ] **Step 2: Manual smoke test**

Run:
```bash
mkdir -p /tmp/sthist
printf '%s\n' \
  '{"timestamp":"2026-06-01T12:00:00Z","server_colo":"AAA","latency_ns":20000000,"jitter_ns":3000000,"download_mbps":10,"upload_mbps":5}' \
  '{"timestamp":"2026-06-05T12:00:00Z","server_colo":"BBB","latency_ns":21000000,"jitter_ns":4000000,"download_mbps":20,"upload_mbps":9}' \
  '{"timestamp":"2026-06-09T12:00:00Z","server_colo":"CCC","latency_ns":22000000,"jitter_ns":5000000,"download_mbps":30,"upload_mbps":12}' \
  > /tmp/sthist/history.jsonl
L=/tmp/sthist/history.jsonl
echo "=== since 2026-06-05 (expect 2 rows: 05, 09) ==="; ./speed-test history --log-file "$L" --since 2026-06-05
echo "=== until 2026-06-05 (expect 2 rows: 01, 05 — whole day) ==="; ./speed-test history --log-file "$L" --until 2026-06-05
echo "=== range 06-02..06-06 (expect 1 row: 05) ==="; ./speed-test history --log-file "$L" --since 2026-06-02 --until 2026-06-06
echo "=== range summary ==="; ./speed-test history --log-file "$L" --since 2026-06-01 --until 2026-06-09 --summary
echo "=== no match (expect message, exit 0) ==="; ./speed-test history --log-file "$L" --since 2030-01-01; echo "exit: $?"
echo "=== invalid (expect error, non-zero) ==="; ./speed-test history --log-file "$L" --since bogus; echo "exit: $?"
rm -rf /tmp/sthist
```
Expected: since/until/range filter correctly; the until-by-date includes the whole
of June 5; the no-match case prints `No speed tests match that range.` and exits 0;
the invalid case prints an `invalid date "bogus"…` error and exits non-zero.

- [ ] **Step 3: Update README**

In `README.md`, in the `## History` section, REPLACE the example code block with:
```markdown
```bash
speed-test history                 # table of the last 20 runs (newest first)
speed-test history --last 50       # last 50 (use --last 0 for all)
speed-test history --since 7d      # only the last 7 days (also 24h, 30m)
speed-test history --since 2026-06-01 --until 2026-06-07   # a date range
speed-test history --summary       # avg/min/max for download, upload, ping, jitter
speed-test history --export csv  > runs.csv    # or: --out runs.csv
speed-test history --export json > runs.json
```
```
And after that block's closing fence, REPLACE the trailing sentence with:
```markdown
`--since`/`--until` accept `YYYY-MM-DD`, `YYYY-MM-DD HH:MM`, or a relative
duration (`7d`/`24h`/`30m`); a bare `--until` date includes the whole day. They
combine with `--last`, `--summary`, and `--export`. `--log-file` reads a
different file; `--no-color` (or piping / `NO_COLOR`) disables coloring.
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: document history --since/--until date filters"
```

---

## Self-Review Notes

- **Spec coverage:** `ParseBound` formats (empty/relative/datetime/date, end-of-day for `--until`) and `Filter` inclusive `[since,until]` with zero=unbounded (Task 1); `--since`/`--until` flags, parse-before-`LastN`, footer/window over the filtered total, distinct "no match" message, invalid-value error, composition with summary/export (Task 2); verification + smoke matrix + README (Task 3). All spec sections covered.
- **Type consistency:** `ParseBound(s string, end bool, now time.Time) (time.Time, error)`, `Filter(records []speedtest.Result, since, until time.Time) []speedtest.Result`, `historyOptions.since/until`, `noMatchMsg`, and the `runHistory` flow match across tasks and the existing `Load`/`LastN`/`Table`/`Summarize` signatures.
- **Timezone-robustness:** the cmd tests use UTC record timestamps far from the local-midnight boundaries (June 1/5/9 at 12:00 UTC vs `--since 2026-06-05` local), so results hold in any machine timezone.
- **No placeholders; no `Co-Authored-By` trailer in any commit.**
