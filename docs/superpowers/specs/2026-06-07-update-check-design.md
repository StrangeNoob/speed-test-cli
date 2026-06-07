# Speed Test CLI — Update Check & Self-Update Design

**Date:** 2026-06-07
**Status:** Approved

## Summary

Add a GitHub-backed update feature to the speed-test CLI:

1. A **passive check** during a normal run that, throttled to once per 24h and
   without adding latency, notifies the user when a newer release exists — and on
   an interactive terminal prompts to update in place.
2. An explicit **`speed-test update`** subcommand that downloads the matching
   release asset, verifies it, and atomically replaces the running binary.

Self-update is delegated to the vetted `github.com/creativeprojects/go-selfupdate`
library (GitHub release lookup, OS/arch asset matching, checksum verification
against the release's `checksums.txt`, atomic replace + rollback).

## Goals

- Tell users when they're behind, with minimal friction.
- Let users update in place (passively via a prompt, or explicitly via `update`).
- Never slow down or break the speed test; never pollute `--json` output.
- Be quiet and safe: offline or any error → silent on the passive path; clear,
  actionable messages on the explicit path.

## Non-Goals

- No auto-update without consent.
- No package-manager-specific logic (Homebrew, etc.) beyond surfacing a helpful
  error when in-place replacement isn't possible.
- No background daemon or scheduled checks outside of CLI invocations.

## Dependencies

- `github.com/creativeprojects/go-selfupdate` — release detection, asset
  selection by OS/arch, checksum validation, and safe binary replacement.
- `golang.org/x/mod/semver` — the version comparison in `Newer` (Go-team
  maintained, tiny). Used directly so our comparison logic is pure and
  unit-testable without the network.

These are deliberate, approved exceptions to the project's otherwise
near-zero-dependency stance, because hand-rolling secure self-update and robust
semver comparison is error-prone.

## Architecture & Components

A new `internal/update` package owns all update logic; `cmd` wires it in.

```
internal/update/
├── cache.go        # update-check cache + throttle decision
├── check.go        # latest-version lookup + semver comparison
├── selfupdate.go   # download + replace the running binary
└── decide.go       # pure policy helpers (no I/O)
cmd/
├── root.go         # --no-update-check flag; background check; notify/prompt
└── update.go       # `speed-test update` subcommand
```

### cache.go
- `Cache` struct: `LastCheck time.Time (json:"last_check")`,
  `LatestVersion string (json:"latest_version")`.
- `DefaultCachePath() (string, error)` → `~/.speed-test/update-check.json`
  (alongside the existing history file).
- `Load(path) (Cache, error)` — a missing file returns the zero value and no
  error; malformed JSON returns the zero value and no error (best-effort).
- `Save(path, Cache) error` — creates the parent dir, writes atomically.
- `Due(c Cache, now time.Time, interval time.Duration) bool` — true when
  `now.Sub(c.LastCheck) >= interval`. `checkInterval = 24h` (constant).

### check.go
- `const repoSlug = "StrangeNoob/speed-test-cli"`.
- `Latest(ctx) (version string, err error)` — wraps
  `selfupdate.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))`; returns the
  latest release tag (e.g. `v0.2.0`), or an error.
- `Newer(current, latest string) bool` — uses `golang.org/x/mod/semver`.
  Normalizes each operand to a leading `v` (`semver.Compare` requires it),
  returns false when `current == "dev"`, when either value is not
  `semver.IsValid`, or when `latest` is not strictly greater than `current`.

### selfupdate.go
- `Apply(ctx, current string) (newVersion string, err error)`:
  - `DetectLatest`; if not newer than `current`, return `("", nil)` (caller treats
    as "already current").
  - Resolve `os.Executable()`, then `selfupdate.UpdateTo(...)` to replace it.
  - Returns the new version on success. On replacement failure (permission
    denied, etc.) returns a wrapped error the caller renders helpfully.

### decide.go (pure, no I/O — the testable policy core)
- `ShouldCheck(jsonMode, noFlag bool, env, version string) bool` — false when
  `jsonMode`, `noFlag`, `env != ""` (any non-empty `SPEEDTEST_NO_UPDATE_CHECK`),
  or `version == "dev"`; true otherwise.
- `ShouldPrompt(isTTY bool) bool` — returns `isTTY`.

## Passive Check Flow (in `cmd/root.go run()`)

1. Compute `check := update.ShouldCheck(o.json, o.noUpdateCheck,
   os.Getenv("SPEEDTEST_NO_UPDATE_CHECK"), currentVersion)`. If false, behave
   exactly as today.
2. If checking: load the cache. Start a goroutine that delivers a `latest string`
   on a buffered channel:
   - If `Due`: call `Latest(ctx)`. On success, `Save` the cache
     (`{LastCheck: now, LatestVersion: latest}`) and send `latest`.
   - Else (throttled): send `cache.LatestVersion`.
   - On any error: send `""` (no notice).
3. Run the speed test as usual (the goroutine overlaps the ~12s test).
4. After the summary prints, read the channel with a guard:
   `select { case v := <-ch: latest = v; case <-time.After(2s): }`. A hung
   network call is simply ignored.
5. If `latest != "" && Newer(currentVersion, latest)`:
   - **Interactive** (`ShouldPrompt(IsTerminal(os.Stdin) && IsTerminal(os.Stdout))`):
     print `A new version <latest> is available (you have <current>).`, then
     prompt `Update now? [y/N] `. Read one line from stdin; on `y`/`yes`
     (case-insensitive), run the self-update path (see below). Otherwise print
     `Run 'speed-test update' to upgrade.`
   - **Non-interactive**: print one line —
     `Update available: <latest> — run 'speed-test update'.` No prompt.

All passive output goes to **stderr** (stdout stays for results), and is never
emitted in `--json` mode.

## `speed-test update` Subcommand (`cmd/update.go`)

- Always runs (ignores the 24h throttle and `--no-update-check`; it's explicit).
- Prints `Checking for updates…` then calls `update.Apply(ctx, currentVersion)`:
  - Returns `("", nil)` → print `speed-test is up to date (<current>).`
  - Returns `(new, nil)` → print `Updated speed-test <current> → <new>.`
  - Returns error → print
    `Could not update: <reason>. If you installed via go install or a package
    manager, update with that instead; otherwise re-run with sufficient
    permissions (e.g. sudo).` and exit non-zero.
- The passive "yes" path calls the same `update.Apply` and renders the same
  messages.

## Flags & Config

- `--no-update-check` (bool, default false) on the root command → `options.noUpdateCheck`.
- `SPEEDTEST_NO_UPDATE_CHECK` env var (any non-empty value disables the passive check).
- Cache: `~/.speed-test/update-check.json`, 24h interval (constant).
- `current version` is the same value `--version` reports (from `buildVersion`),
  passed into `run()` / the update subcommand.

## Error Handling

- Passive path: every error (network, cache I/O, parse) is swallowed; the result
  is simply "no notice". Offline runs print nothing extra.
- Explicit `update`: surfaces clear, actionable messages (no stack traces);
  non-zero exit on failure.
- Cache writes failing never affect the run (best-effort).

## Testing Strategy

- **Pure logic (unit, the core value):**
  - `Newer`: `v0.1.5`<`v0.2.0`, equal, older, `v`-prefix vs none, `dev` (never
    newer), pre-release tags.
  - `ShouldCheck`: truth table over (jsonMode, noFlag, env set, `dev` version).
  - `ShouldPrompt`: TTY true/false.
  - `Due`: never-checked, 23h ago, 25h ago.
  - Cache `Load`/`Save` round-trip in a temp dir; missing file → zero value, no error.
- **Network / self-update:** thin wrappers over go-selfupdate, not unit-tested
  against the live API. One `-short`-gated integration test calls the real
  `Latest(ctx)` against `StrangeNoob/speed-test-cli` (skipped in CI / `-short`).
- **cmd:** `--no-update-check` parses; `speed-test update --help` works with no
  network; the mutually-exclusive/other existing tests still pass.

## Out of Scope / Future

- Self-update for Homebrew/other managers (we only surface a helpful error).
- Configurable check interval or channel (stable vs pre-release).
- Showing release notes/changelog in the notice.
