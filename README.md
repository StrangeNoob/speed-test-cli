# speed-test-cli

[![CI](https://github.com/StrangeNoob/speed-test-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/StrangeNoob/speed-test-cli/actions/workflows/ci.yml)

A command-line internet speed test (download, upload, ping, jitter) powered by
Cloudflare's public speed-test endpoints.

## Install

### Go toolchain (any platform)

```bash
go install github.com/StrangeNoob/speed-test-cli@latest
```

This installs the `speed-test-cli` command into your Go bin directory
(`$(go env GOBIN)`, or `$(go env GOPATH)/bin` — make sure it's on your `PATH`).

### Prebuilt binaries

Download the binary for your platform from the
[latest release](https://github.com/StrangeNoob/speed-test-cli/releases/latest),
then `chmod +x` it and move it onto your `PATH`.

### From source (with make)

```bash
make build            # compile ./speed-test in the repo
make install          # copy it to /usr/local/bin (run from anywhere as `speed-test`)
make go-install       # or install via the Go toolchain (command: speed-test-cli)
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
