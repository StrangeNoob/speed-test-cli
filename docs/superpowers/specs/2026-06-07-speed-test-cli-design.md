# Speed Test CLI — Design

**Date:** 2026-06-07
**Status:** Approved

## Summary

A command-line internet speed test tool written in Go. It measures download
speed, upload speed, latency (ping), and jitter against Cloudflare's public
speed-test endpoints. Built primarily to learn the measurement internals
(HTTP, throughput, concurrency) while being reliable enough for daily use.

## Goals

- Build the measurement logic from scratch (learning goal) — no third-party
  speedtest engine.
- Produce accurate, trustworthy numbers (proper warm-up, ramping, parallel
  streams).
- Be genuinely useful: pretty live output, JSON mode for scripting, and a
  history log to track performance over time.

## Non-Goals

- Competing with Ookla/Cloudflare on absolute accuracy.
- Self-hosted or pluggable test servers (Cloudflare only for v1).
- Scheduling/alerting (deferred — history logging makes it possible later).
- A TUI framework (plain progress output only).

## Approach

Standard library does the interesting work (`net/http`, goroutines for
parallel streams). Minimal, well-understood dependencies: `cobra` for CLI
parsing and one small progress-bar helper for live output. No speedtest
library, no TUI framework.

## Architecture & Package Layout

```
speed-test-cli/
├── main.go                 # entry point, wires CLI → runner
├── cmd/
│   └── root.go             # cobra command, flags, builds Config
├── internal/
│   ├── speedtest/
│   │   ├── client.go       # Cloudflare HTTP client, endpoint config
│   │   ├── latency.go      # ping + jitter measurement
│   │   ├── download.go     # download throughput
│   │   ├── upload.go       # upload throughput
│   │   └── result.go       # Result struct (shared data type)
│   ├── output/
│   │   ├── human.go        # pretty live + summary output
│   │   └── json.go         # --json serialization
│   └── history/
│       └── log.go          # append result to history file
```

Each unit has one responsibility. The `speedtest` package produces a
`Result`; `output` and `history` consume it. Measurement is testable without
a terminal; output is testable with a synthetic `Result`. Measurement code
does NOT import the terminal/progress library — live updates flow through a
callback.

## Cloudflare Endpoints

- Download: `GET https://speed.cloudflare.com/__down?bytes=N`
- Upload:   `POST https://speed.cloudflare.com/__up` (body of N bytes)
- Trace:    `GET https://speed.cloudflare.com/cdn-cgi/trace` (colo/location)

## Measurement Logic

### Latency & Jitter (`latency.go`)
- Issue ~20 small requests.
- For each, measure round-trip time, subtracting Cloudflare's server
  processing time from the `Server-Timing: cfRequestDuration` response header
  to isolate true network RTT.
- `ping` = median of samples (robust against outliers).
- `jitter` = mean of absolute differences between consecutive samples.

### Download / Upload Throughput (`download.go` / `upload.go`)
- **Ramp payload sizes** (e.g. 1MB → 10MB → 25MB → 100MB) so slow links don't
  block on huge files and fast links aren't capped by tiny ones.
- **Parallel streams** (default 6 concurrent connections via goroutines) — a
  single TCP connection rarely saturates a fast link.
- **Throughput** = total bytes ÷ wall-clock duration, aggregated across
  streams.
- **Discard a warm-up window** (~first 1s) so TCP slow-start doesn't drag the
  number down.
- **Time cap** (default 12s per direction) so slow links still finish.

### Shared Result Type (`result.go`)
```go
type Result struct {
    Timestamp    time.Time
    ServerColo   string        // Cloudflare datacenter, e.g. "SIN"
    Latency      time.Duration
    Jitter       time.Duration
    DownloadMbps float64
    UploadMbps   float64
}
```
This is the single contract between measurement and everything downstream.

## CLI Flags

```
speed-test [flags]

  --json              Machine-readable JSON instead of pretty output
  --no-log            Don't append this run to the history file
  --streams int       Parallel connections per direction (default 6)
  --duration duration Max time per direction (default 12s)
  --log-file string   History file path (default ~/.speed-test/history.jsonl)
  --download-only     Skip upload test
  --upload-only       Skip download test
```

All tuning values have sensible defaults; flags override them.

## Data Flow

`cmd/root.go` parses flags → builds `Config` → calls `speedtest.Run(config)`
→ receives a `Result` → routes to `output.Human` or `output.JSON` → unless
`--no-log`, calls `history.Append`. Live progress in human mode is driven by a
callback the measurement code invokes as bytes flow.

## Error Handling

Fail loudly; never silently report a wrong number.

- Network/DNS failure → clear message, non-zero exit.
- Partial failure (e.g. download OK, upload fails) → report what succeeded,
  mark the failed metric explicitly, non-zero exit.
- `--json` mode → errors emitted to stderr as JSON; stdout stays clean for
  piping.
- History write failure → warn to stderr but do not fail the run (the
  measurement itself succeeded).

## History File Format

JSON Lines (`.jsonl`): one serialized `Result` per line, appended per run.
Easy to parse, grep, and graph later. Default location
`~/.speed-test/history.jsonl` (directory created if missing).

## Testing Strategy

- `speedtest`: unit-test the math (throughput, jitter, `Server-Timing`
  parsing) against a local `httptest.Server` — fast, deterministic, offline.
- `output`: feed a fixed `Result`, assert rendered text and JSON.
- `history`: write to a temp file, read back, assert contents.
- Integration test hitting real Cloudflare, gated behind `-short` (skipped in
  normal/CI runs).

## Future (Out of Scope for v1)

- Scheduling (cron-friendly) and threshold alerts.
- Historical summaries / graphs from the `.jsonl` log.
- Configurable/self-hosted endpoints.
