# speed-test-cli Makefile
#
# CGO_ENABLED=0 is exported for all targets: on macOS 26 (Tahoe) with Go 1.22,
# CGO-enabled binaries crash at launch/test with "missing LC_UUID load command".
# This project is pure Go, so disabling CGO is safe everywhere.
export CGO_ENABLED := 0

BINARY := speed-test
PKG    := ./...

# Install location for `make install`. Override e.g. `make install PREFIX=$HOME/.local`.
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.DEFAULT_GOAL := build

## build: compile the CLI binary
.PHONY: build
build:
	go build -o $(BINARY) .

## run: build and run the CLI (pass args via ARGS="--json --duration 5s")
.PHONY: run
run: build
	./$(BINARY) $(ARGS)

## install: build and copy the binary into BINDIR (default /usr/local/bin) to run from anywhere
.PHONY: install
install: build
	install -d "$(BINDIR)"
	install -m 0755 $(BINARY) "$(BINDIR)/$(BINARY)"
	@echo "Installed $(BINARY) -> $(BINDIR)/$(BINARY)"
	@echo "(if /usr/local/bin is not writable, re-run with sudo, or: make install PREFIX=$$HOME/.local)"

## uninstall: remove the binary installed by `make install`
.PHONY: uninstall
uninstall:
	rm -f "$(BINDIR)/$(BINARY)"
	@echo "Removed $(BINDIR)/$(BINARY)"

## go-install: install via the Go toolchain into GOBIN/GOPATH bin (command: speed-test-cli)
.PHONY: go-install
go-install:
	go install .
	@dir=$$(go env GOBIN); [ -n "$$dir" ] || dir=$$(go env GOPATH)/bin; \
		echo "Installed 'speed-test-cli' -> $$dir (ensure it is on your PATH)"

## test: run the full test suite (includes the live Cloudflare network test)
.PHONY: test
test:
	go test $(PKG) -v

## test-short: run unit tests only, skipping the live network test
.PHONY: test-short
test-short:
	go test $(PKG) -short

## test-race: run the full suite with the race detector
.PHONY: test-race
test-race:
	go test $(PKG) -race

## vet: run go vet static analysis
.PHONY: vet
vet:
	go vet $(PKG)

## fmt: format all Go source
.PHONY: fmt
fmt:
	go fmt $(PKG)

## tidy: prune and sync go.mod / go.sum
.PHONY: tidy
tidy:
	go mod tidy

## check: fmt + vet + race tests (run before committing)
.PHONY: check
check: fmt vet test-race

## clean: remove the built binary
.PHONY: clean
clean:
	rm -f $(BINARY)

## help: list available targets
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
