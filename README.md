# speed-test-cli

[![CI](https://github.com/StrangeNoob/speed-test-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/StrangeNoob/speed-test-cli/actions/workflows/ci.yml)

A command-line internet speed test (download, upload, ping, jitter) powered by
Cloudflare's public speed-test endpoints.

## Install

### Go toolchain (any platform)

```bash
go install github.com/StrangeNoob/speed-test-cli/cmd/speed-test@latest
```

This installs the `speed-test` command into your Go bin directory
(`$(go env GOBIN)`, or `$(go env GOPATH)/bin` — make sure it's on your `PATH`,
e.g. add `export PATH="$(go env GOPATH)/bin:$PATH"` to your shell profile).

> **macOS 26 (Tahoe) + Go 1.22:** prefix the command with `CGO_ENABLED=0`, i.e.
> `CGO_ENABLED=0 go install github.com/StrangeNoob/speed-test-cli/cmd/speed-test@latest`.
> Otherwise the installed binary crashes at launch with
> `missing LC_UUID load command`. The project is pure Go, so disabling CGO is safe.

### Prebuilt binaries

Download the archive for your platform from the
[latest release](https://github.com/StrangeNoob/speed-test-cli/releases/latest),
extract it, and move the `speed-test` binary onto your `PATH`:

```bash
tar xzf speed-test_<version>_<os>_<arch>.tar.gz   # or unzip on Windows
sudo mv speed-test /usr/local/bin/
```

Verify the download against `checksums.txt` from the release.

### From source (with make)

```bash
make build            # compile ./speed-test in the repo
make install          # copy it to /usr/local/bin (run from anywhere as `speed-test`)
make go-install       # or install via the Go toolchain (also `speed-test`)
make uninstall        # remove the binary installed by `make install`
```

`make install` defaults to `/usr/local/bin`; if that needs root, either run
`sudo make install` or pick a user-writable dir: `make install PREFIX=$HOME/.local`.

> **Note:** the Makefile sets `CGO_ENABLED=0` for every target. This is required
> on macOS 26 (Tahoe) with Go 1.22, where CGO-enabled binaries crash at launch
> with `missing LC_UUID load command`. The project is pure Go, so disabling CGO
> is safe; on unaffected toolchains a plain `go build -o speed-test .` works too.

## Usage

```bash
speed-test                 # run full test, pretty output
speed-test --json          # machine-readable JSON
speed-test --download-only # skip upload
speed-test --streams 8 --duration 15s
speed-test --no-log        # don't append to history
speed-test --version       # print version, commit, and build date
speed-test history         # show recent runs (table)
speed-test compare         # is my connection better/normal/worse than usual?
```

Results are appended to `~/.speed-test/history.jsonl` (one JSON object per line)
unless `--no-log` is passed.

Output is colored with live progress bars when run in a terminal; colors and
animation are disabled automatically when piped/redirected, when `NO_COLOR` is
set, or with `--no-color`.

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
| `--no-color` | false | Disable colored output |
| `--no-update-check` | false | Disable the GitHub update check |

## Updating

`speed-test` checks GitHub for a newer release at most once a day and, on an
interactive terminal, offers to update in place. To update explicitly:

```bash
speed-test update
```

Disable the passive check with `--no-update-check` or `SPEEDTEST_NO_UPDATE_CHECK=1`.
If the binary lives in a directory you can't write to (e.g. a system path or a
Homebrew install), `update` will tell you to use your install method or rerun
with elevated permissions.

## History

Every run is appended to `~/.speed-test/history.jsonl`. View and analyze it:

```bash
speed-test history                 # table of the last 20 runs (newest first)
speed-test history --last 50       # last 50 (use --last 0 for all)
speed-test history --since 7d      # only the last 7 days (also 24h, 30m)
speed-test history --since 2026-06-01 --until 2026-06-07   # a date range
speed-test history --summary       # avg/min/max for download, upload, ping, jitter
speed-test history --export csv  > runs.csv    # or: --out runs.csv
speed-test history --export json > runs.json
```

`--since`/`--until` accept `YYYY-MM-DD`, `YYYY-MM-DD HH:MM`, or a relative
duration (`7d`/`24h`/`30m`); a bare `--until` date includes the whole day. They
combine with `--last`, `--summary`, and `--export`. `--log-file` reads a
different file; `--no-color` (or piping / `NO_COLOR`) disables coloring.

## Compare

`speed-test compare` runs a test and tells you whether your connection is
**better, normal, or degraded** versus the median of your own recent results.

```bash
speed-test compare                       # run a test, compare to the last 10
speed-test compare --last 30             # baseline = median of the last 30
speed-test compare --window 7d           # baseline = runs in the last 7 days
speed-test compare --latest              # compare the most recent saved run (no new test)
speed-test compare --plan-download 200 --plan-upload 100   # vs your ISP plan
speed-test compare --json
```

The baseline uses the **median** (so one bad test doesn't skew it). `--last` and
`--window` are mutually exclusive; `--no-log` skips saving the fresh test.
