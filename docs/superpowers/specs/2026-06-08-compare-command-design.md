# `speed-test compare` Design

**Date:** 2026-06-08
**Status:** Approved

## Summary

Add a `speed-test compare` command that runs a fresh speed test (or reuses the
latest saved one) and tells the user whether their internet is performing
**better, normal, or degraded** compared to their own past results. The baseline
is the **median** of recent runs (robust to spikes); the command reports a
per-metric percentage change, a per-metric label, and an overall verdict.
Optional flags compare against an ISP plan and emit JSON.

Positioning: *"`speed-test compare` tells you whether your current internet
performance is normal, degraded, or better than usual based on your own
speed-test history."*

## Goals

- Turn the existing `history.jsonl` into a genuine "insight" feature.
- Robust baseline (median, not mean).
- Clear, colored human output plus machine-readable JSON.
- Reuse the existing engine, history layer, and styler — no change to the core
  measurement.

## Non-Goals

- No trend charts / sparklines.
- No persistent "plan" config (plan is passed per-invocation via flags).
- No change to the stored history format.

## Components

```
internal/history/compare.go (new)   Median, Compare, verdict classification (pure)
internal/output/compare.go  (new)   RenderCompare (table) + RenderCompareJSON
cmd/compare.go              (new)    newCompareCmd: flags, gather, render
cmd/root.go                 (modify) cmd.AddCommand(newCompareCmd())
```

Reuses: `speedtest.NewClient().Run`, `speedtest.Result`, `history.Load`/`LastN`/
`Filter`/`ParseBound`/`DefaultPath`/`Append`, `output.Styler`/`NewStyler`/
`ShouldColor`/`IsTerminal`/`NewProgressPrinter`.

### internal/history/compare.go

- `Median(values []float64) float64` — sorts a copy, returns the middle value
  (mean of the two middles for even counts); `0` for an empty slice.
- Metric helpers convert a `speedtest.Result` to floats: download/upload Mbps
  directly; ping/jitter via `msOf` (ms).
- `MetricStatus` — `int` enum `StatusNormal`, `StatusBetter`, `StatusWorse`.
- `MetricCompare` struct: `Current, Baseline, DeltaPct float64`, `Defined bool`,
  `Status MetricStatus`. `Defined=false` when the baseline median is 0 (delta
  undefined).
- `Comparison` struct:
  ```go
  type Comparison struct {
      Current     speedtest.Result
      HasBaseline bool
      SampleSize  int
      Download    MetricCompare // higher is better
      Upload      MetricCompare // higher is better
      Ping        MetricCompare // lower is better
      Jitter      MetricCompare // lower is better
      Verdict     string        // code: excellent|normal|degraded|unstable[+_high_latency]
      Summary     string        // human sentence
  }
  ```
- `Compare(current speedtest.Result, baseline []speedtest.Result) Comparison`:
  - If `len(baseline) == 0`: return `{Current: current, HasBaseline: false}`.
  - Else compute the median of each metric over `baseline`, then each
    `MetricCompare`:
    - `DeltaPct = (cur-med)/med*100` when `med != 0` (`Defined=true`), else
      `Defined=false`, `DeltaPct=0`.
    - `improvement` = `+DeltaPct` for throughput, `-DeltaPct` for latency.
    - `Status` = `StatusBetter` if `improvement >= 10`, `StatusWorse` if
      `improvement <= -10`, else `StatusNormal` (only when `Defined`).
  - Overall verdict (download is the headline; latency can override). Let `impr`
    be the per-metric improvement (only for `Defined` metrics):
    - **unstable** if `Ping.improvement <= -25` OR `Jitter.improvement <= -50`.
    - else **excellent** if `Download.improvement >= 15`.
    - else **degraded** if `Download.improvement <= -15`.
    - else **normal**.
    - Suffix `_high_latency` appended to a non-unstable base when `Ping.Status ==
      StatusWorse || Jitter.Status == StatusWorse` (e.g. `normal_high_latency`).
  - `Summary`: a sentence built from the base verdict + an optional latency
    caveat. Examples: excellent → "Your connection is performing better than
    usual."; normal → "Your connection is performing normally."; degraded →
    "Your download is slower than your recent baseline."; unstable → "Your
    latency is much higher than usual — the connection looks unstable."; with the
    `_high_latency` caveat → append " Latency is higher than your recent
    baseline." All constant strings; no data interpolation required for the
    summary (numbers live in the table/JSON).

### internal/output/compare.go

- `RenderCompare(w io.Writer, c Comparison, plan PlanInfo, st *Styler)` — renders:
  - Title `speed-test  •  Compare` (bold).
  - **Current test** block (download/upload/ping/jitter with the existing arrow/
    unit style).
  - If `c.HasBaseline`: **Compared with last N tests** block — one line per metric
    `<label>  ±X.X% <phrase>` where phrase is "better than usual" / "worse than
    usual" (throughput) or "faster than usual" / "slower than usual" (latency) or
    "normal"; an undefined metric shows `—`. Delta colored green (better) / red
    (worse) / dim (normal) via `st`.
  - Else: a dim line "Not enough history to compare yet."
  - If `plan.Set`: **Plan performance** block — `Download <cur> / <plan> Mbps
    <pct>%` and the same for upload (only the ones provided).
  - **Verdict** line — `c.Summary` (bold label coloring optional).
- `RenderCompareJSON(w io.Writer, c Comparison, plan PlanInfo) error` — marshals
  the DTO:
  ```json
  {
    "current": {"download_mbps":…, "upload_mbps":…, "latency_ms":…, "jitter_ms":…},
    "baseline": {"type":"median","sample_size":10,"download_mbps":…,"upload_mbps":…,"latency_ms":…,"jitter_ms":…} | null,
    "delta": {"download_percent":…,"upload_percent":…,"latency_percent":…,"jitter_percent":…},
    "plan": {"download_mbps":200,"upload_mbps":100,"download_percent":…,"upload_percent":…} (omitted if unset),
    "verdict": "normal_high_latency",
    "summary": "…"
  }
  ```
  `baseline` is `null` and `delta` omitted when `!HasBaseline`. Undefined metric
  deltas are emitted as `null`.
- `PlanInfo struct { Set bool; Download, Upload float64 }` (a metric is included
  only when its plan flag > 0).

### cmd/compare.go

`compareOptions`: `last int`, `window string`, `latest bool`, `planDownload
float64`, `planUpload float64`, `json bool`, `noLog bool`, `noColor bool`,
`logFile string`.

`runCompare`:
1. Validate: `--last` and `--window` are mutually exclusive
   (`MarkFlagsMutuallyExclusive`).
2. Resolve `path` (`--log-file` or `DefaultPath`). `history.Load` → `past`.
3. Current + baseline:
   - `--latest`: require `len(past) >= 1`; `current = past[len-1]`; `cand =
     past[:len-1]`.
   - else: run a fresh test — build `speedtest.Config{Streams:6, Duration:12s}`,
     run with a progress printer (TTY-aware, suppressed in `--json`), get
     `current`; `cand = past`. Unless `--no-log`, `history.Append(path, current)`.
   - On a test error, return it (same handling as the main command: JSON-shaped
     to stderr in `--json` mode, else `speed test failed: …`).
4. Select baseline from `cand`:
   - `--window`: `since, _ := ParseBound(o.window, false, now)` (error surfaced);
     `baseline = Filter(cand, since, zero)`.
   - else: `baseline = LastN(cand, o.last)`.
5. `c := history.Compare(current, baseline)` (set `c.SampleSize = len(baseline)`
   inside Compare).
6. `plan := PlanInfo{Set: o.planDownload>0 || o.planUpload>0, Download:
   o.planDownload, Upload: o.planUpload}`.
7. Render: `--json` → `RenderCompareJSON(os.Stdout, c, plan)`; else
   `RenderCompare(os.Stdout, c, plan, styler)`.

Register in `newRootCmd`: `cmd.AddCommand(newCompareCmd())` next to the others.

## Verdict Thresholds (summary)

- Per-metric label threshold: **±10%** improvement → Better / Worse, else Normal.
- Overall verdict throughput threshold: **±15%** download improvement →
  Excellent / Degraded.
- Latency "unstable": ping improvement ≤ −25% or jitter improvement ≤ −50%.
- `_high_latency` suffix: ping or jitter status is Worse (and base not unstable).

## Error Handling

- `--last` + `--window` → cobra error (non-zero exit).
- Invalid `--window` → `ParseBound` error (non-zero exit).
- `--latest` with **zero** saved runs (no "current" to use) → friendly message
  "No saved speed tests to compare. Run 'speed-test' first." (stderr, exit 0).
- `--latest` with exactly **one** saved run (current exists, baseline empty) →
  the empty-baseline case below.
- Live test failure → error to stderr (JSON-shaped in `--json`), non-zero exit.
- Empty baseline (fresh test with no prior history, or the one-record `--latest`
  case) → current-only output + "Not enough history to compare yet." (exit 0);
  JSON `baseline: null`.

## Testing

- **`Median`**: empty→0, single, odd (3 → middle), even (4 → mean of middles),
  unsorted input.
- **`Compare`** (fixed current + baseline slice):
  - medians and `DeltaPct` per metric (hand-computed);
  - per-metric `Status` at boundaries (improvement 9/10/11, −9/−10/−11), latency
    direction flipped;
  - overall verdict for: excellent (download +20%), degraded (download −20%),
    normal, unstable (jitter +80% → improvement −80%), `_high_latency` suffix
    (download normal, ping +15% → improvement −15% → Worse);
  - empty baseline → `HasBaseline=false`;
  - zero-median metric → `Defined=false`, that metric excluded from the verdict.
- **`RenderCompare`** (disabled styler): contains the metric values, the labels,
  the verdict summary, and no `\x1b`; enabled styler → contains `\x1b[`.
  No-baseline → "Not enough history". Plan set → "Plan performance" + percentages.
- **`RenderCompareJSON`**: validates as JSON; `baseline` null when no baseline;
  `verdict`/`summary` present; `plan` present only when set; undefined delta →
  `null`.
- **cmd**: `compare --help`; `--last` + `--window` errors; `--latest` over a temp
  history file (≥2 records) produces output with no network; `--latest` over an
  empty/single-record file prints the friendly message with no error; invalid
  `--window` errors.
- The live test path (`compare` without `--latest`) is not unit-tested (network);
  exercised by the manual smoke test.

## Out of Scope / Future

- Persisted ISP-plan config; trend visualization; per-colo comparison.
