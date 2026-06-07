# Update Check & Self-Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a throttled, non-blocking GitHub update check that notifies (and on a TTY, prompts to self-update) during a normal run, plus an explicit `speed-test update` subcommand that downloads, verifies, and replaces the running binary.

**Architecture:** A new `internal/update` package holds all policy (pure, unit-tested: version compare, throttle, opt-out, prompt) and thin wrappers over `go-selfupdate` for the network/replace work. `cmd/root.go` runs the check in a goroutine during the test and notifies/prompts after the summary; `cmd/update.go` adds the explicit subcommand.

**Tech Stack:** Go 1.22, `github.com/creativeprojects/go-selfupdate` v1.5.2 (release detection, checksum validation against `checksums.txt`, atomic replace), `golang.org/x/mod/semver` (version comparison), `github.com/spf13/cobra`.

**IMPORTANT ENV:** This machine requires `CGO_ENABLED=0` for all `go` commands (otherwise binaries crash with a dyld `LC_UUID` error). Prefix every Go command with `CGO_ENABLED=0`.

---

## File Structure

| File | Change | Responsibility |
|------|--------|----------------|
| `go.mod` / `go.sum` | modify | add go-selfupdate + x/mod deps |
| `internal/update/decide.go` | new | `ShouldCheck`, `ShouldPrompt`, `PromptYesNo` (pure policy + prompt I/O) |
| `internal/update/decide_test.go` | new | tests for the above |
| `internal/update/version.go` | new | `Newer`, `upToDate`, `canon` (semver comparison) |
| `internal/update/version_test.go` | new | tests for comparison |
| `internal/update/cache.go` | new | `Cache`, `Load`, `Save`, `Due`, `DefaultCachePath`, `CheckInterval` |
| `internal/update/cache_test.go` | new | cache round-trip + throttle tests |
| `internal/update/remote.go` | new | `Latest`, `Apply` (thin go-selfupdate wrappers) + gated integration test |
| `internal/update/remote_integration_test.go` | new | `-short`-gated live `Latest` test |
| `cmd/root.go` | modify | version threading, `--no-update-check`, background check + notify/prompt, `runUpdate` |
| `cmd/update.go` | new | `speed-test update` subcommand |
| `cmd/root_test.go` | modify | update `newRootCmd` call sites; add `--no-update-check` + update-cmd tests |
| `README.md` | modify | document the feature |

---

## Task 1: Add dependencies and the decision/prompt policy

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/update/decide.go`
- Test: `internal/update/decide_test.go`

- [ ] **Step 1: Add the dependencies**

Run:
```bash
CGO_ENABLED=0 go get github.com/creativeprojects/go-selfupdate@v1.5.2
CGO_ENABLED=0 go get golang.org/x/mod@latest
```
Expected: `go.mod`/`go.sum` updated.

- [ ] **Step 2: Write the failing test**

Create `internal/update/decide_test.go`:
```go
package update

import (
	"bytes"
	"strings"
	"testing"
)

func TestShouldCheck(t *testing.T) {
	for _, tc := range []struct {
		name    string
		json    bool
		noFlag  bool
		env     string
		version string
		want    bool
	}{
		{"normal release", false, false, "", "v0.1.5", true},
		{"json mode", true, false, "", "v0.1.5", false},
		{"flag set", false, true, "", "v0.1.5", false},
		{"env set", false, false, "1", "v0.1.5", false},
		{"dev build", false, false, "", "dev", false},
	} {
		if got := ShouldCheck(tc.json, tc.noFlag, tc.env, tc.version); got != tc.want {
			t.Errorf("%s: ShouldCheck = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestShouldPrompt(t *testing.T) {
	if !ShouldPrompt(true) || ShouldPrompt(false) {
		t.Error("ShouldPrompt should mirror isTTY")
	}
}

func TestPromptYesNo(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"y\n", true}, {"Y\n", true}, {"yes\n", true}, {"YES\n", true},
		{"n\n", false}, {"\n", false}, {"nope\n", false},
	} {
		var out bytes.Buffer
		got, _ := PromptYesNo(strings.NewReader(tc.in), &out, "Update now? [y/N] ")
		if got != tc.want {
			t.Errorf("PromptYesNo(%q) = %v, want %v", tc.in, got, tc.want)
		}
		if !strings.Contains(out.String(), "Update now?") {
			t.Errorf("prompt text not written: %q", out.String())
		}
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/update/ -v`
Expected: FAIL — `undefined: ShouldCheck` (package doesn't compile yet).

- [ ] **Step 4: Write minimal implementation**

Create `internal/update/decide.go`:
```go
// Package update checks GitHub for newer releases and can replace the running
// binary in place. All policy here is pure and offline; network/replace work
// lives in remote.go.
package update

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ShouldCheck reports whether the passive update check should run. It is skipped
// for machine output, when explicitly disabled, and for unversioned dev builds.
func ShouldCheck(jsonMode, noFlag bool, env, version string) bool {
	if jsonMode || noFlag || env != "" || version == "dev" {
		return false
	}
	return true
}

// ShouldPrompt reports whether to interactively prompt (only on a TTY).
func ShouldPrompt(isTTY bool) bool { return isTTY }

// PromptYesNo writes prompt to w and reads a yes/no answer from r. Empty or
// anything other than y/yes (case-insensitive) is false.
func PromptYesNo(r io.Reader, w io.Writer, prompt string) (bool, error) {
	fmt.Fprint(w, prompt)
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/update/ -v`
Expected: PASS. Also `CGO_ENABLED=0 go vet ./...`.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/update/decide.go internal/update/decide_test.go
git commit -m "feat(update): add deps and check/prompt policy"
```

---

## Task 2: Version comparison

**Files:**
- Create: `internal/update/version.go`
- Test: `internal/update/version_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/update/version_test.go`:
```go
package update

import "testing"

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            bool
	}{
		{"v0.1.5", "v0.2.0", true},
		{"0.1.5", "0.2.0", true},   // tolerate missing leading v
		{"v0.1.5", "v0.1.5", false},
		{"v0.2.0", "v0.1.5", false},
		{"dev", "v0.2.0", false},   // never nag dev builds
		{"v0.1.5", "garbage", false},
		{"garbage", "v0.2.0", false},
		{"v0.1.5", "v0.1.6-next", true}, // prerelease still newer than release
	} {
		if got := Newer(tc.current, tc.latest); got != tc.want {
			t.Errorf("Newer(%q,%q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestUpToDate(t *testing.T) {
	// Explicit-update logic: dev/invalid current is NOT up to date (should update).
	for _, tc := range []struct {
		current, latest string
		want            bool
	}{
		{"v0.2.0", "v0.2.0", true},
		{"v0.3.0", "v0.2.0", true},
		{"v0.1.0", "v0.2.0", false},
		{"dev", "v0.2.0", false},
		{"garbage", "v0.2.0", false},
	} {
		if got := upToDate(tc.current, tc.latest); got != tc.want {
			t.Errorf("upToDate(%q,%q) = %v, want %v", tc.current, tc.latest, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/update/ -run 'Newer|UpToDate' -v`
Expected: FAIL — `undefined: Newer`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/update/version.go`:
```go
package update

import "golang.org/x/mod/semver"

// canon ensures a leading "v" so golang.org/x/mod/semver can parse it.
func canon(v string) string {
	if v == "" || v[0] == 'v' {
		return v
	}
	return "v" + v
}

// Newer reports whether latest is strictly newer than current. It returns false
// for a "dev" build or any unparseable version (used by the passive notice).
func Newer(current, latest string) bool {
	if current == "dev" {
		return false
	}
	c, l := canon(current), canon(latest)
	if !semver.IsValid(c) || !semver.IsValid(l) {
		return false
	}
	return semver.Compare(l, c) > 0
}

// upToDate reports whether current is a valid version that is >= latest. A
// dev/unparseable current is treated as NOT up to date so an explicit `update`
// still upgrades it.
func upToDate(current, latest string) bool {
	c, l := canon(current), canon(latest)
	if !semver.IsValid(c) {
		return false
	}
	if !semver.IsValid(l) {
		return true
	}
	return semver.Compare(c, l) >= 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/update/ -run 'Newer|UpToDate' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/update/version.go internal/update/version_test.go
git commit -m "feat(update): add semver comparison (Newer/upToDate)"
```

---

## Task 3: Cache and throttle

**Files:**
- Create: `internal/update/cache.go`
- Test: `internal/update/cache_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/update/cache_test.go`:
```go
package update

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingReturnsZero(t *testing.T) {
	got := Load(filepath.Join(t.TempDir(), "nope.json"))
	if !got.LastCheck.IsZero() || got.LatestVersion != "" {
		t.Errorf("missing file should load zero Cache, got %+v", got)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "update-check.json")
	want := Cache{LastCheck: time.Unix(1700000000, 0).UTC(), LatestVersion: "v0.2.0"}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got := Load(path)
	if !got.LastCheck.Equal(want.LastCheck) || got.LatestVersion != want.LatestVersion {
		t.Errorf("round-trip = %+v, want %+v", got, want)
	}
}

func TestDue(t *testing.T) {
	now := time.Unix(1700000000, 0)
	if Due(Cache{}, now, CheckInterval) != true {
		t.Error("never-checked should be due")
	}
	recent := Cache{LastCheck: now.Add(-23 * time.Hour)}
	if Due(recent, now, CheckInterval) != false {
		t.Error("checked 23h ago should not be due")
	}
	old := Cache{LastCheck: now.Add(-25 * time.Hour)}
	if Due(old, now, CheckInterval) != true {
		t.Error("checked 25h ago should be due")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/update/ -run 'Load|Save|Due' -v`
Expected: FAIL — `undefined: Load`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/update/cache.go`:
```go
package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CheckInterval is the minimum time between real GitHub queries.
const CheckInterval = 24 * time.Hour

// Cache records the last update check, stored next to the history file.
type Cache struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version"`
}

// DefaultCachePath returns ~/.speed-test/update-check.json.
func DefaultCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".speed-test", "update-check.json"), nil
}

// Load reads the cache. A missing or malformed file yields the zero value; this
// is best-effort and never returns an error.
func Load(path string) Cache {
	b, err := os.ReadFile(path)
	if err != nil {
		return Cache{}
	}
	var c Cache
	if err := json.Unmarshal(b, &c); err != nil {
		return Cache{}
	}
	return c
}

// Save writes the cache, creating parent directories as needed.
func Save(path string, c Cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Due reports whether at least interval has passed since the last check.
func Due(c Cache, now time.Time, interval time.Duration) bool {
	return now.Sub(c.LastCheck) >= interval
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./internal/update/ -run 'Load|Save|Due' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/update/cache.go internal/update/cache_test.go
git commit -m "feat(update): add update-check cache and 24h throttle"
```

---

## Task 4: Remote lookup and self-update (go-selfupdate wrappers)

**Files:**
- Create: `internal/update/remote.go`
- Test: `internal/update/remote_integration_test.go`

- [ ] **Step 1: Write the gated integration test**

Create `internal/update/remote_integration_test.go`:
```go
package update

import (
	"context"
	"testing"
	"time"
)

// TestLatestLive hits the real GitHub API. Skipped under -short.
func TestLatestLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live GitHub test in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	v, err := Latest(ctx)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if v == "" {
		t.Fatal("expected a non-empty latest version")
	}
	t.Logf("latest = %s", v)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./internal/update/ -run TestLatestLive -v`
Expected: FAIL — `undefined: Latest`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/update/remote.go`:
```go
package update

import (
	"context"
	"os"

	"github.com/creativeprojects/go-selfupdate"
)

// repoSlug is the GitHub repository releases are fetched from.
const repoSlug = "StrangeNoob/speed-test-cli"

// newUpdater builds an updater that validates downloads against checksums.txt.
func newUpdater() (*selfupdate.Updater, error) {
	return selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
}

// Latest returns the newest release version tag (e.g. "v0.2.0"), or "" if none
// is found.
func Latest(ctx context.Context) (string, error) {
	up, err := newUpdater()
	if err != nil {
		return "", err
	}
	rel, found, err := up.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
	if err != nil {
		return "", err
	}
	if !found || rel == nil {
		return "", nil
	}
	return rel.Version(), nil
}

// Apply updates the running binary to the latest release if it is newer than
// current. It returns the new version on success, or ("", nil) when already up
// to date. The running executable is replaced atomically by go-selfupdate.
func Apply(ctx context.Context, current string) (string, error) {
	up, err := newUpdater()
	if err != nil {
		return "", err
	}
	rel, found, err := up.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
	if err != nil {
		return "", err
	}
	if !found || rel == nil {
		return "", nil
	}
	if upToDate(current, rel.Version()) {
		return "", nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := up.UpdateTo(ctx, rel, exe); err != nil {
		return "", err
	}
	return rel.Version(), nil
}
```

- [ ] **Step 4: Verify it builds and the live test passes (or skips offline)**

Run: `CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./...`
Expected: clean build.
Run: `CGO_ENABLED=0 go test ./internal/update/ -short` → PASS (live test skipped).
Run (network): `CGO_ENABLED=0 go test ./internal/update/ -run TestLatestLive -v` → PASS with a logged version, OR a network error if offline (report which; do not mask).

- [ ] **Step 5: Commit**

```bash
git add internal/update/remote.go internal/update/remote_integration_test.go
git commit -m "feat(update): add Latest and Apply over go-selfupdate"
```

---

## Task 5: Thread the raw version and add the `--no-update-check` flag

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/root_test.go`

- [ ] **Step 1: Update the tests for the new signatures**

In `cmd/root_test.go`, the calls `newRootCmd("test")`, `newRootCmd("1.2.3 (commit abc, built 2026-06-07)")`, etc. become two-argument `newRootCmd(display, raw)`. REPLACE the three tests that call `newRootCmd` and ADD a flag test:
```go
func TestMutuallyExclusiveOnlyFlags(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"--download-only", "--upload-only"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when both --download-only and --upload-only are set")
	}
}

func TestNoColorFlagParses(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"--no-color", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--no-color should be a valid flag, got: %v", err)
	}
}

func TestVersionFlag(t *testing.T) {
	cmd := newRootCmd("1.2.3 (commit abc, built 2026-06-07)", "v1.2.3")
	cmd.SetArgs([]string{"--version"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--version should not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "speed-test 1.2.3") || !strings.Contains(out, "commit abc") {
		t.Errorf("unexpected --version output: %q", out)
	}
}

func TestNoUpdateCheckFlagParses(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"--no-update-check", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--no-update-check should be a valid flag, got: %v", err)
	}
}
```
(The existing `TestBuildVersionFormatsComponents` and `TestBuildVersionBarePlainBuild` call `buildVersion(...)` and are unchanged — keep them.)

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./cmd/ -v`
Expected: FAIL — not enough arguments to `newRootCmd` / `undefined` flag.

- [ ] **Step 3: Write minimal implementation**

In `cmd/root.go`:

(a) Add the field to `options` (after `noColor bool`):
```go
	noUpdateCheck bool
```

(b) Add `resolveVersion` and route `buildVersion` through it. REPLACE the existing `buildVersion` function with:
```go
// resolveVersion returns the bare version, falling back to the module version
// recorded in the binary's build info for `go install`-built binaries.
func resolveVersion(version string) string {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if mv := info.Main.Version; mv != "" && mv != "(devel)" {
				return mv
			}
		}
	}
	return version
}

// buildVersion assembles the string shown by `--version`.
func buildVersion(version, commit, date string) string {
	v := resolveVersion(version)
	if commit != "none" || date != "unknown" {
		return fmt.Sprintf("%s (commit %s, built %s)", v, commit, date)
	}
	return v
}
```

(c) Change `newRootCmd` to take both the display string and the raw version, register the flag, wire `run` and the update subcommand. REPLACE the `newRootCmd` signature/body header:
```go
func newRootCmd(versionDisplay, versionRaw string) *cobra.Command {
	var o options
	cmd := &cobra.Command{
		Use:     "speed-test",
		Short:   "Measure internet speed against Cloudflare",
		Version: versionDisplay,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(o, versionRaw)
		},
	}
	cmd.SetVersionTemplate("speed-test {{.Version}}\n")
```
and, immediately before `return cmd`, add (alongside the other flag registrations and `MarkFlagsMutuallyExclusive`):
```go
	f.BoolVar(&o.noUpdateCheck, "no-update-check", false, "Disable the GitHub update check")
	cmd.AddCommand(newUpdateCmd(versionRaw))
```

(d) Update `Execute` to pass both forms:
```go
func Execute(version, commit, date string) {
	if err := newRootCmd(buildVersion(version, commit, date), resolveVersion(version)).Execute(); err != nil {
		os.Exit(1)
	}
}
```

(e) Change `run`'s signature to accept the raw version (the body is extended in Task 6; for now just thread the parameter):
```go
func run(o options, versionRaw string) error {
```
Leave the rest of `run` unchanged for this task. `newUpdateCmd` is added in Task 7; to keep the package compiling between tasks, add this temporary stub at the bottom of `cmd/root.go` now (it is replaced by the real subcommand in Task 7):
```go
// newUpdateCmd is implemented in update.go (Task 7). Temporary stub so the
// package compiles; replaced in the next task.
func newUpdateCmd(versionRaw string) *cobra.Command {
	return &cobra.Command{Use: "update", Short: "Update speed-test to the latest release", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./cmd/ -v && CGO_ENABLED=0 go build ./...`
Expected: PASS and clean build. Also `CGO_ENABLED=0 go vet ./...`.

- [ ] **Step 5: Commit**

```bash
git add cmd/root.go cmd/root_test.go
git commit -m "feat(update): thread raw version and add --no-update-check flag"
```

---

## Task 6: Background check + notify/prompt in `run()`

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Add imports**

In `cmd/root.go`, ensure the import block includes `context` and the update package:
```go
	"context"
	...
	"github.com/StrangeNoob/speed-test-cli/internal/update"
```
(`time`, `os`, `fmt`, and `output` are already imported.)

- [ ] **Step 2: Start the background check at the top of `run`**

In `run(o options, versionRaw string)`, immediately after `client := speedtest.NewClient()`, add:
```go
	updCh := startUpdateCheck(o, versionRaw)
```

- [ ] **Step 3: Notify/prompt after the summary**

In `run`, in the non-JSON branch, AFTER `output.Human(os.Stdout, res, summarySt)` (the last statement before the history-logging block, or after it — place it just before `return nil` at the end of the function), add:
```go
	maybeReportUpdate(updCh, versionRaw)
```

- [ ] **Step 4: Implement the helpers**

Add to `cmd/root.go`:
```go
// startUpdateCheck launches the throttled GitHub check in the background so it
// overlaps the speed test. It returns nil when checking is disabled.
func startUpdateCheck(o options, versionRaw string) chan string {
	if !update.ShouldCheck(o.json, o.noUpdateCheck, os.Getenv("SPEEDTEST_NO_UPDATE_CHECK"), versionRaw) {
		return nil
	}
	ch := make(chan string, 1)
	go func() {
		path, err := update.DefaultCachePath()
		if err != nil {
			ch <- ""
			return
		}
		c := update.Load(path)
		if !update.Due(c, time.Now(), update.CheckInterval) {
			ch <- c.LatestVersion
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		latest, err := update.Latest(ctx)
		if err != nil || latest == "" {
			ch <- c.LatestVersion // fall back to the cached value
			return
		}
		_ = update.Save(path, update.Cache{LastCheck: time.Now(), LatestVersion: latest})
		ch <- latest
	}()
	return ch
}

// maybeReportUpdate reads the background check result (with a short guard) and,
// if a newer version exists, notifies or prompts per interactivity.
func maybeReportUpdate(ch chan string, versionRaw string) {
	if ch == nil {
		return
	}
	var latest string
	select {
	case latest = <-ch:
	case <-time.After(2 * time.Second):
		return
	}
	if latest == "" || !update.Newer(versionRaw, latest) {
		return
	}
	interactive := update.ShouldPrompt(output.IsTerminal(os.Stdin) && output.IsTerminal(os.Stdout))
	if !interactive {
		fmt.Fprintf(os.Stderr, "\nUpdate available: %s — run 'speed-test update'.\n", latest)
		return
	}
	fmt.Fprintf(os.Stderr, "\nA new version %s is available (you have %s).\n", latest, versionRaw)
	yes, _ := update.PromptYesNo(os.Stdin, os.Stderr, "Update now? [y/N] ")
	if !yes {
		fmt.Fprintln(os.Stderr, "Run 'speed-test update' to upgrade.")
		return
	}
	_ = runUpdate(versionRaw)
}

// runUpdate performs the self-update and prints the outcome. Shared by the
// prompt path and the `update` subcommand.
func runUpdate(versionRaw string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	newV, err := update.Apply(ctx, versionRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not update: %v.\nIf you installed via go install or a package manager, update with that instead; otherwise re-run with sufficient permissions (e.g. sudo).\n", err)
		return err
	}
	if newV == "" {
		fmt.Fprintf(os.Stderr, "speed-test is up to date (%s).\n", versionRaw)
		return nil
	}
	fmt.Fprintf(os.Stderr, "Updated speed-test %s → %s.\n", versionRaw, newV)
	return nil
}
```

- [ ] **Step 5: Run the suite and build**

Run: `CGO_ENABLED=0 go test ./... -short && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go build ./...`
Expected: all PASS, clean build. (No new unit test here — the logic is covered by the `internal/update` tests; this task wires them together.)

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go
git commit -m "feat(update): background check with notify/prompt after the summary"
```

---

## Task 7: `speed-test update` subcommand

**Files:**
- Create: `cmd/update.go`
- Modify: `cmd/root.go` (remove the temporary stub)
- Modify: `cmd/root_test.go` (add a subcommand test)

- [ ] **Step 1: Write the failing test**

Add to `cmd/root_test.go`:
```go
func TestUpdateSubcommandRegistered(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"update", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("`update --help` should work without network, got: %v", err)
	}
	sub, _, err := cmd.Find([]string{"update"})
	if err != nil || sub.Name() != "update" {
		t.Fatalf("update subcommand not registered: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `CGO_ENABLED=0 go test ./cmd/ -run TestUpdateSubcommand -v`
Expected: it passes against the stub OR fails; either way, proceed to replace the stub with the real command. (The stub returns nil for `RunE`; `--help` already works, so this test may pass against the stub — that is fine. The point of this task is the real implementation below.)

- [ ] **Step 3: Replace the stub with the real subcommand**

In `cmd/root.go`, DELETE the temporary `newUpdateCmd` stub function added in Task 5.

Create `cmd/update.go`:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newUpdateCmd builds the `speed-test update` subcommand. It always checks
// GitHub (ignoring the 24h throttle and --no-update-check) and self-updates.
func newUpdateCmd(versionRaw string) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update speed-test to the latest release",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprintln(os.Stderr, "Checking for updates…")
			return runUpdate(versionRaw)
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `CGO_ENABLED=0 go test ./cmd/ -v && CGO_ENABLED=0 go build ./...`
Expected: PASS and clean build. Also `CGO_ENABLED=0 go vet ./...`.

- [ ] **Step 5: Commit**

```bash
git add cmd/update.go cmd/root.go cmd/root_test.go
git commit -m "feat(update): add `speed-test update` subcommand"
```

---

## Task 8: Final verification, manual smoke test, and README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Full verification**

Run:
```bash
CGO_ENABLED=0 go mod tidy && CGO_ENABLED=0 go test ./... -short && CGO_ENABLED=0 go test ./... -race -short && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go build -o speed-test ./cmd/speed-test
```
Expected: all PASS, clean build. (`go mod tidy` promotes go-selfupdate and x/mod from indirect to direct dependencies now that they're imported.)

- [ ] **Step 2: Manual smoke test (requires network)**

The current build's version is older than the latest release, so the passive notice should appear:
```bash
CGO_ENABLED=0 go build -ldflags "-X main.version=v0.0.1" -o /tmp/st-old ./cmd/speed-test
/tmp/st-old --download-only --duration 5s --no-log
```
Expected: after the summary, a notice/prompt on a TTY (`A new version vX.Y.Z is available (you have v0.0.1).`). When piped, only the one-line notice:
```bash
/tmp/st-old --download-only --duration 5s --no-log | cat
```
Expected: stderr shows `Update available: vX.Y.Z — run 'speed-test update'.`, stdout is just the summary.

Opt-out works:
```bash
/tmp/st-old --download-only --duration 5s --no-log --no-update-check
SPEEDTEST_NO_UPDATE_CHECK=1 /tmp/st-old --download-only --duration 5s --no-log
```
Expected: no update notice in either. JSON mode is silent too:
```bash
/tmp/st-old --json --download-only --duration 5s --no-log
```
Expected: a single clean JSON object, no update text. Clean up: `rm -f /tmp/st-old`.

- [ ] **Step 3: Update README**

In `README.md`, add a row to the Flags table after `--no-color`:
```markdown
| `--no-update-check` | false | Disable the GitHub update check |
```
And add a new section after the Flags table:
```markdown
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
```

- [ ] **Step 4: Commit**

```bash
git add README.md go.mod go.sum
git commit -m "docs: document update check, update command, and --no-update-check"
```
(Include `go.mod`/`go.sum` if `go mod tidy` in Step 1 promoted the new deps from indirect to direct.)

---

## Self-Review Notes

- **Spec coverage:** dependencies (Task 1), `ShouldCheck`/`ShouldPrompt`/`PromptYesNo` (Task 1), `Newer`/`upToDate` semver (Task 2), cache + `Due` throttle (Task 3), `Latest`/`Apply` over go-selfupdate with checksum validation + gated live test (Task 4), version threading + `--no-update-check` + env opt-out (Tasks 5–6), background non-blocking check + 2s guard + notify/prompt split + stderr-only + json-skip (Task 6), `speed-test update` subcommand (Task 7), README + manual smoke covering TTY/piped/opt-out/json (Task 8). All spec sections covered.
- **Type consistency:** `ShouldCheck(json,noFlag,env,version)`, `ShouldPrompt(isTTY)`, `PromptYesNo(r,w,prompt)`, `Newer(current,latest)`, `upToDate(current,latest)`, `Cache{LastCheck,LatestVersion}`, `Load(path) Cache`, `Save(path,Cache) error`, `Due(c,now,interval)`, `CheckInterval`, `Latest(ctx)`, `Apply(ctx,current)`, `newRootCmd(versionDisplay,versionRaw)`, `run(o,versionRaw)`, `runUpdate(versionRaw)`, `newUpdateCmd(versionRaw)` are used identically across tasks.
- **Inter-task compile safety:** Task 5 adds a temporary `newUpdateCmd` stub so the package compiles before Task 7 provides the real one; Task 7 explicitly deletes the stub. No task leaves the tree uncompilable.
- **No placeholders:** every code step is complete; commands include expected output.
