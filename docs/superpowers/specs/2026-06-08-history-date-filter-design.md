# History Date-Range Filtering Design

**Date:** 2026-06-08
**Status:** Approved

## Summary

Add `--since` and `--until` flags to the `speed-test history` command so users can
view, summarize, or export only the runs within a date range. Both absolute dates
(`YYYY-MM-DD`, `YYYY-MM-DD HH:MM`) and relative durations (`7d`, `24h`, `30m`,
meaning "N ago") are accepted. Filtering composes with the existing `--last`,
`--summary`, and `--export` flags.

## Goals

- Let users scope history to a time window for any output mode.
- Accept both calendar dates and convenient relative shorthand.
- Keep parsing/filtering pure and unit-testable (no network, deterministic via an
  injected `now`).
- Compose cleanly with the existing flags; no surprising precedence.

## Non-Goals

- No timezone flag (uses the machine's local time, consistent with the table).
- No named ranges (`today`, `this-week`) — relative durations cover the need.
- No change to the stored history format.

## Components

```
internal/history/
└── filter.go (new)   ParseBound + Filter (pure)
cmd/history.go        + --since / --until flags; parse → Filter → LastN
```

### filter.go

- `ParseBound(s string, end bool, now time.Time) (time.Time, error)`:
  - `""` → zero `time.Time` (caller treats as "unbounded"), nil error.
  - **Relative** matching `^\d+(d|h|m)$` (`7d`, `24h`, `30m`) → `now.Add(-dur)` where
    `d=24h`, `h=time.Hour`, `m=time.Minute`.
  - **Datetime** `YYYY-MM-DD HH:MM` (layout `2006-01-02 15:04`, parsed in
    `time.Local`) → that exact local time.
  - **Date** `YYYY-MM-DD` (layout `2006-01-02`, `time.Local`): for `end == false`
    the parsed start-of-day; for `end == true` the **end of that day**
    (`+24h - 1ns`, i.e. `23:59:59.999999999` local) so `--until 2026-06-07`
    includes all of June 7.
  - Anything else → `fmt.Errorf("invalid date %q: use YYYY-MM-DD, \"YYYY-MM-DD HH:MM\", or a duration like 7d/24h/30m", s)`.
  - Parsing order: empty → relative → datetime → date → error.
- `Filter(records []speedtest.Result, since, until time.Time) []speedtest.Result`:
  - Returns records whose `Timestamp` is within `[since, until]` **inclusive**.
  - A zero `since` means no lower bound; a zero `until` means no upper bound.
  - Implemented as: skip when `!since.IsZero() && t.Before(since)` or
    `!until.IsZero() && t.After(until)`. Order-preserving; returns a new slice
    (may be empty, never nil-vs-empty-sensitive for callers).

### cmd/history.go

- Add to `historyOptions`: `since string`, `until string`.
- Register flags:
  - `--since` — "Only runs at/after this time (YYYY-MM-DD, 'YYYY-MM-DD HH:MM', or 7d/24h/30m)".
  - `--until` — "Only runs at/before this time (same formats; a bare date includes the whole day)".
- In `runHistory`, after `Load` and before windowing:
  ```
  now := time.Now()
  since, err := history.ParseBound(o.since, false, now)  // return err
  until, err := history.ParseBound(o.until, true, now)   // return err
  filtered := history.Filter(records, since, until)
  total := len(filtered)
  window := history.LastN(filtered, o.last)
  ```
  The skipped-lines note (from `Load`) is unchanged. `total` (the post-filter
  count) feeds the table footer and the export/summary windows.

## Behavior

- `history --since 7d` → all runs in the last 7 days, then the default `--last 20`.
- `history --since 7d --last 5` → the 5 newest runs within the last 7 days.
- `history --since 2026-06-01 --until 2026-06-07 --summary` → avg/min/max for that
  inclusive week.
- `history --since 2026-06-01 --export csv` → CSV of runs on/after June 1.
- Filtering applies to **all** modes (table, summary, csv, json). `--summary` and
  `--export` remain mutually exclusive; `--since`/`--until` conflict with neither.

## Error Handling

- Invalid `--since` or `--until` value → return the `ParseBound` error (non-zero
  exit, printed by Cobra). The since error is checked before the until error.
- `since > until` → `Filter` returns no records (not an error).
- Empty results:
  - File had **no records at all** → existing message
    `No speed tests recorded yet. Run 'speed-test' to record one.` (stderr, exit 0).
  - Records exist but **none match the range** → `No speed tests match that range.`
    (stderr, exit 0) for table/summary. Export still emits the CSV header / `[]`.
  - The distinction is: `len(records) == 0` (pre-filter) → the first message;
    `len(records) > 0 && len(window) == 0` → the second.

## Testing

- **`ParseBound`** (injected `now`):
  - `"2026-06-01"` with `end=false` → `2026-06-01 00:00` local;
    with `end=true` → `2026-06-01 23:59:59.999999999` local.
  - `"2026-06-01 15:30"` → exact local time, identical for both `end` values.
  - `"7d"`, `"24h"`, `"30m"` → `now` minus the duration.
  - `""` → zero time (`IsZero()`), no error.
  - `"nonsense"`, `"2026-13-99"`, `"5x"` → error.
- **`Filter`**: records spanning several days →
  - both bounds → only in-range; `since`-only and `until`-only halves;
  - both zero → all records unchanged;
  - a record whose `Timestamp` equals `since` or `until` exactly is **included**;
  - `since > until` → empty.
- **cmd** (`history_test.go`): a temp file spanning a week →
  `--since/--until` returns only the window (assert count/content); `--since nonsense`
  errors; in-range-empty prints `No speed tests match that range.` with no error;
  `--since … --summary` and `--since … --export csv` work without flag conflict.

## Out of Scope / Future

- Timezone selection, named windows, `--last` semantics changes.
