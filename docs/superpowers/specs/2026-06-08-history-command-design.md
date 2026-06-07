# Speed Test CLI — History Command Design

**Date:** 2026-06-08
**Status:** Approved

## Summary

Expose the speed-test history the CLI already records (`~/.speed-test/history.jsonl`)
through a new `speed-test history` subcommand that can:

- print the most recent runs as a colored, aligned table (default: last 20),
- print an avg/min/max summary (`--summary`),
- export records as CSV or JSON (`--export csv|json`, to stdout or `--out <file>`).

The storage layer (`internal/history`: `Append`, `DefaultPath`, records are
`speedtest.Result` JSON lines) already exists, so this is read-and-format work:
a reader + stats functions in `internal/history`, plus a Cobra subcommand.

## Goals

- Make the recorded data visible and analyzable without external tools.
- Pure, offline, unit-testable logic (parse / summarize / format).
- Reuse existing types (`speedtest.Result`) and rendering (`output.Styler`).
- Be script-friendly: CSV/JSON export to stdout, clean empty-data behavior.

## Non-Goals

- No charts/graphs or time-series visualization.
- No filtering by date range or metric thresholds (could come later).
- No editing/deleting history entries.

## Architecture & Components

```
internal/history/
├── log.go     (existing)  Append, DefaultPath
├── read.go    (new)       Load + lastN
├── stats.go   (new)       Summary + Summarize
└── format.go  (new)       Table, CSV, JSON renderers
cmd/
└── history.go (new)       `speed-test history` subcommand
```

Each unit has one responsibility: `read.go` parses, `stats.go` computes,
`format.go` renders, `cmd/history.go` orchestrates. All are testable in
isolation; only `cmd/history.go` touches the terminal/flags.

### read.go
- `Load(path string) ([]speedtest.Result, int, error)` — reads the file line by
  line, JSON-unmarshals each non-empty line into a `speedtest.Result`, and
  returns the valid records **in file order (oldest first)** plus a count of
  malformed/unreadable lines that were skipped. A missing file returns
  `(nil, 0, nil)`; a genuine read error (e.g. permission) returns the error.
  Best-effort parsing: a bad line increments the skip count and is ignored, it
  never aborts the load.
- `lastN(records []speedtest.Result, n int) []speedtest.Result` — returns the
  last `n` records (the most recent, since records are oldest-first). `n <= 0`
  or `n >= len` returns all records. Pure.

### stats.go
- `Summary` struct:
  ```go
  type Summary struct {
      Count           int
      First, Last     time.Time
      Download, Upload metricStats // Mbps
      Ping, Jitter     metricStats // ms
  }
  type metricStats struct{ Avg, Min, Max float64 }
  ```
- `Summarize(records []speedtest.Result) Summary` — computes count, first/last
  timestamps (min/max of `Timestamp`), and avg/min/max for download, upload,
  ping, and jitter. Ping/jitter are converted from `time.Duration` to
  milliseconds (float). An empty slice returns the zero `Summary` (Count 0); the
  caller treats Count 0 as "no data".

### format.go (all rendering; no business logic)
- `Table(w io.Writer, records []speedtest.Result, total int, st *output.Styler)` —
  renders the records newest-first as an aligned table with a header and a
  `showing <len(records)> of <total>` footer (`total` is the full record count
  before the `--last` window was applied). Columns: Date/Time, Download, Upload,
  Ping, Jitter. Right-aligned numbers, 1 decimal for Mbps, integer ms. Header
  cyan/bold and units dim via the styler (a disabled styler emits plain text).
- `CSV(w io.Writer, records []speedtest.Result) error` — writes a header row
  (`timestamp,server_colo,download_mbps,upload_mbps,ping_ms,jitter_ms`) followed
  by one row per record (timestamp in RFC 3339). Empty input still writes the
  header row. Uses `encoding/csv`.
- `JSON(w io.Writer, records []speedtest.Result) error` — writes the records as a
  JSON array (`json.MarshalIndent`, 2-space). Empty input writes `[]`.
- `RenderSummary(w io.Writer, s Summary, st *output.Styler)` — renders the stats
  block below from a computed `Summary`:
```
Speed Test Summary  (134 runs, 12 May – 08 Jun)

            Avg      Min      Max
Download   118.4    42.1     201.3   Mbps
Upload      47.2    18.0      63.5   Mbps
Ping        24.1    14.0      58.0   ms
Jitter       5.8     1.2      19.4   ms
```

### cmd/history.go
`newHistoryCmd() *cobra.Command` building `speed-test history` with flags:

| Flag | Type | Default | Effect |
|------|------|---------|--------|
| `--last` | int | 20 | Most recent N runs (0 = all). |
| `--summary` | bool | false | Print stats block instead of table. |
| `--export` | string | "" | `csv` or `json` export instead of table. |
| `--out` | string | "" | With `--export`, write to this file (else stdout). |
| `--log-file` | string | "" | Read from this path (else `DefaultPath()`). |
| `--no-color` | bool | false | Disable color (also auto-off when piped / `NO_COLOR`). |

`--summary` and `--export` are mutually exclusive
(`cmd.MarkFlagsMutuallyExclusive("summary", "export")`). `--export` only accepts
`csv` or `json` (else an error). Registered in `newRootCmd` via
`cmd.AddCommand(newHistoryCmd())`, next to `newUpdateCmd`.

## Data Flow

1. Resolve path: `--log-file` if set, else `history.DefaultPath()`.
2. `Load(path)` → records (oldest-first) + skipped count.
3. If skipped > 0, print `(skipped N unreadable lines)` to stderr.
4. Apply window: `records = lastN(records, o.last)`.
5. Dispatch by mode:
   - `--export csv|json` → write to `--out` file or stdout (export still emits
     header / `[]` when there are no records).
   - `--summary` → `Summarize(records)`; if Count 0, print the empty message,
     else render the stats block to stdout.
   - default → if no records, print the empty message; else render the table
     (with `showing len(window) of totalCount`).

Color decision (table/summary only): `output.NewStyler(output.ShouldColor(
output.IsTerminal(os.Stdout), o.noColor, os.Getenv("NO_COLOR")))`.

## Error Handling

- Missing or empty history file: not an error. Table/summary print
  `No speed tests recorded yet. Run 'speed-test' to record one.` to stderr and
  exit 0. Export writes just the CSV header / `[]` to keep scripts working.
- Malformed JSON lines: skipped, counted, reported once to stderr; never abort.
- A real read error (permission, etc.) or a bad `--export` value: return the
  error (non-zero exit), printed by Cobra.
- `--out` write failure: return the error (non-zero exit).

## Date / Number Formatting

- Table timestamps: local time, `02 Jan 2006 15:04` (e.g. `08 Jun 2026 10:30`).
- Summary date range: `02 Jan` for first and last (year omitted for brevity).
- Mbps: one decimal (`92.4`). Ping/Jitter ms: integer in the table, one decimal
  in the summary. CSV/JSON: full precision (ms as float, timestamp RFC 3339).

## Testing Strategy

- **`Load`** (`read.go`): temp file with several valid lines + one malformed line
  → returns valid records in order with skipped == 1; missing file → `(nil,0,nil)`;
  empty file → `(nil,0,nil)`.
- **`lastN`**: n in {0, 2, 100} over a 3-record slice → all, last 2, all.
- **`Summarize`** (`stats.go`): fixed records → assert count, first/last, and
  avg/min/max for each metric (hand-computed); empty → Count 0.
- **`CSV` / `JSON`** (`format.go`): fixed records → exact CSV header + rows; JSON
  round-trips back to the same records; empty → header only / `[]`.
- **`Table`** (`format.go`): records + disabled styler → output contains dates,
  values, headers and **no `\x1b`**; enabled styler → contains `\x1b[`.
- **cmd** (`history.go`): `history --help` works with no file; `--summary
  --export csv` errors (mutual exclusion); missing history prints the friendly
  message.

## Out of Scope / Future

- Date-range / threshold filtering, sorting options.
- Sparkline / trend visualization.
- Pruning or rotating the history file.
