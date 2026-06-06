# speed-test-cli

A command-line internet speed test (download, upload, ping, jitter) powered by
Cloudflare's public speed-test endpoints.

## Install

```bash
CGO_ENABLED=0 go build -o speed-test .
```

> **Note:** `CGO_ENABLED=0` is required on macOS 26 (Tahoe) with Go 1.22, where
> CGO-enabled binaries crash at launch with `missing LC_UUID load command`. This
> project is pure Go, so disabling CGO is safe. On unaffected toolchains a plain
> `go build -o speed-test .` works too.

## Usage

```bash
speed-test                 # run full test, pretty output
speed-test --json          # machine-readable JSON
speed-test --download-only # skip upload
speed-test --streams 8 --duration 15s
speed-test --no-log        # don't append to history
```

Results are appended to `~/.speed-test/history.jsonl` (one JSON object per line)
unless `--no-log` is passed.

## Development

A `Makefile` wraps the common tasks (all with `CGO_ENABLED=0` set):

```bash
make build       # compile ./speed-test
make run ARGS="--json --duration 5s"
make test        # full suite (includes the live network test)
make test-short  # unit tests only
make test-race   # full suite with the race detector
make check       # fmt + vet + race tests (run before committing)
make help        # list all targets
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | false | Machine-readable JSON output |
| `--no-log` | false | Don't append to the history file |
| `--streams` | 6 | Parallel connections per direction |
| `--duration` | 12s | Max time per direction |
| `--log-file` | `~/.speed-test/history.jsonl` | History file path |
| `--download-only` | false | Skip upload test |
| `--upload-only` | false | Skip download test |
