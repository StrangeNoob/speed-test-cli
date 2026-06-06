# Speed Test CLI — TUI Improvements Design

**Date:** 2026-06-07
**Status:** Approved

## Summary

Improve the terminal UI of the speed-test CLI: replace the plain single-line
`download 99.1 Mbps` progress and the flat text summary with a colored,
Unicode-bar interface — while keeping the zero-dependency, standard-library-only
design. Colors and animation auto-disable when output is not a terminal.

## Goals

- A polished live view: spinner + auto-scaling progress bar + climbing number
  during the download and upload phases.
- A polished summary: aligned metrics with proportional Unicode bars and color.
- No new dependencies (stdlib only, including TTY detection).
- Correct behavior when piped/redirected: no escape codes, no animation.
- Honor `NO_COLOR` and a new `--no-color` flag. `--json` output unchanged.

## Non-Goals

- No TUI framework (bubbletea/lipgloss) — explicitly rejected to preserve the
  stdlib-only design.
- No new metrics or measurement changes; this is presentation only.
- No configurable themes/palettes beyond on/off.

## Approach

Enhanced standard-library rendering. ANSI escape codes for color, Unicode
1/8-block glyphs for sub-cell bar precision, a braille spinner. TTY detection
uses `os.File.Stat().Mode()&os.ModeCharDevice` (no `golang.org/x/term`).

## Architecture & Components

All work stays in `internal/output` (plus flag wiring in `cmd`). No new packages.

| File | Change | Responsibility |
|------|--------|----------------|
| `internal/output/style.go` | new | `Styler`, `renderBar`, `shouldColor`, `IsTerminal`, spinner frames |
| `internal/output/human.go` | modified | `Human` summary + stateful live progress printer |
| `internal/output/json.go` | unchanged | JSON serialization |
| `cmd/root.go` | modified | `--no-color` flag; per-stream color decision; wiring |

### style.go

- `Styler` — holds an `enabled bool`. Methods `Cyan/Green/Dim/Bold(s string) string`
  wrap `s` in the matching ANSI code when enabled, return `s` unchanged when
  disabled. Constructor `NewStyler(enabled bool) *Styler`.
- `renderBar(value, max float64, width int) string` — returns a bar of exactly
  `width` runes: filled cells use `█`, the final partial cell uses one of
  `▏▎▍▌▋▊▉` for fractional precision, and empty cells use a space. Clamps
  `value > max` to full. Returns an all-space bar of `width` runes when
  `max <= 0`. The bar content is bracketed by `▕`…`▏` end-caps at the call sites
  (the end-caps are not counted in `width`).
- `ShouldColor(isTTY bool, noColorFlag bool, noColorEnv string) bool` — returns
  `isTTY && !noColorFlag && noColorEnv == ""`. Pure; unit-tested. Exported so
  `cmd` can call it.
- `IsTerminal(f *os.File) bool` — `fi, err := f.Stat(); err == nil &&
  fi.Mode()&os.ModeCharDevice != 0`.
- Spinner: a `[]string` of braille frames (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`).

### human.go

- `Human(w io.Writer, res speedtest.Result, st *Styler)` — writes the summary:
  - Header line: `speed-test • Cloudflare <colo>` (colo bold; omitted if empty).
  - Ping / Jitter line (ms, one decimal).
  - Download and Upload rows, each a label + `renderBar(value, scale, width)` +
    value, where `scale = max(DownloadMbps, UploadMbps)`.
  - A direction whose value is 0 because it was skipped
    (`--download-only`/`--upload-only`) is not drawn. If a measured value is
    genuinely 0, it renders `—` instead of a bar.
  - Labels cyan, bars green, units dim, via `st`.
- Live progress printer — `NewProgressPrinter(w io.Writer, animate bool)
  speedtest.ProgressFunc`:
  - Closure state: `currentPhase speedtest.Phase`, `peak float64`,
    `frame int`.
  - On each `Progress p`:
    - If `!animate`: return immediately (no output).
    - If `p.Phase != currentPhase`: if `currentPhase != ""`, write `"\n"` to lock
      in the previous phase's line; set `currentPhase = p.Phase`; reset `peak = 0`.
    - `peak = max(peak, p.Mbps)`.
    - Write `"\r" + spinner[frame%len] + " " + label + " " +
      renderBar(p.Mbps, peak, width) + " " + value`; increment `frame`.
  - The live printer always uses color when `animate` is true (animation implies
    a TTY by construction — see cmd wiring).

### cmd/root.go

- Add flag: `--no-color` (bool, default false) → `options.noColor`.
- In `run`:
  - `animate := output.ShouldColor(output.IsTerminal(os.Stderr), o.noColor, os.Getenv("NO_COLOR"))`
    — gates the live printer (`NewProgressPrinter(os.Stderr, animate)`), and also
    whether animation runs (the `Testing…` header still prints in non-json mode
    regardless; only the animated line is gated by `animate`).
  - `summarySt := output.NewStyler(output.ShouldColor(output.IsTerminal(os.Stdout), o.noColor, os.Getenv("NO_COLOR")))`
    — passed to `output.Human(os.Stdout, res, summarySt)`.
  - The `--json` branch is unchanged (no styler, no progress).

  Note: `ShouldColor`, `IsTerminal`, and `NewStyler` are exported from the
  `output` package so `cmd` can call them.

## Data Flow

`cmd` decides color/animation from TTY + env + flag, constructs a `*Styler` for
the summary and an `animate` bool for the live printer, then runs as before. The
`speedtest` package is untouched; it still emits `Progress{Phase, Mbps}`
callbacks. The live printer renders them; `Human` renders the final `Result`.

## Error Handling

Unchanged. Rendering is best-effort to the given writer; write errors from
`fmt.Fprintf` are ignored (consistent with existing output code). No new failure
modes.

## Testing Strategy

- `renderBar`: zero, half, full, `value>max` clamp, `max==0`; assert exact rune
  width equals `width` in each case.
- `ShouldColor`: full truth table over (isTTY, noColorFlag, NO_COLOR set/unset).
- `Styler`: enabled wraps with `\x1b[…`; disabled returns the raw string.
- `Human`: with a disabled styler, output contains metrics + bar glyphs and no
  `\x1b` byte; with an enabled styler, output contains `\x1b[`.
- Live printer: feed a Phase-changing sequence into a `bytes.Buffer`; assert it
  contains both phase labels, a spinner frame, a bar glyph, and a `\n` at the
  phase boundary; assert `animate=false` produces no output.
- Update existing tests for new signatures: `TestHumanSummaryContainsMetrics`
  (pass `NewStyler(false)`), `TestProgressPrinterUpdates` (pass `animate=true`).

## Out of Scope / Future

- Themes/palettes, 256-color or truecolor gradients.
- A persistent full-screen dashboard or historical sparkline from the JSONL log.
