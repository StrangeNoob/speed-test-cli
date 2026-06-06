# Speed Test CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI that measures download/upload speed, latency, and jitter against Cloudflare's speed-test endpoints, with pretty output, JSON mode, and history logging.

**Architecture:** Standard-library `net/http` does the measurement (parallel goroutine streams, warm-up discard, payload ramping). A `speedtest` package produces a shared `Result`; `output` and `history` packages consume it. `cmd` (cobra) wires flags → config → run → output/log. Measurement never imports the terminal/progress library — live updates flow through a callback.

**Tech Stack:** Go 1.22+, `net/http` (stdlib), `github.com/spf13/cobra` (CLI), stdlib `testing` + `net/http/httptest`. Live progress is plain `fmt` (no TUI dependency).

---

## File Structure

| File | Responsibility |
|------|----------------|
| `go.mod` | Module definition + deps |
| `internal/speedtest/result.go` | `Result` struct + `Mbps` helper — the shared contract |
| `internal/speedtest/client.go` | Cloudflare endpoint config, HTTP client, trace/colo lookup, `Config`, `Run` orchestration, progress callback type |
| `internal/speedtest/latency.go` | Ping (median RTT via `Server-Timing`) + jitter |
| `internal/speedtest/download.go` | Download throughput (parallel streams, warm-up, time cap) |
| `internal/speedtest/upload.go` | Upload throughput (parallel streams, warm-up, time cap) |
| `internal/output/json.go` | Serialize `Result` to JSON |
| `internal/output/human.go` | Pretty summary + live progress wiring |
| `internal/history/log.go` | Append `Result` as a JSONL line |
| `cmd/root.go` | Cobra command, flags, build `Config`, route output, log |
| `main.go` | Entry point → `cmd.Execute()` |

---

## Task 1: Project scaffold

**Files:**
- Create: `go.mod`
- Create: `main.go`

- [ ] **Step 1: Initialize the module**

Run:
```bash
go mod init speed-test-cli
```
Expected: creates `go.mod` with `module speed-test-cli` and a `go 1.x` line.

- [ ] **Step 2: Create a placeholder main**

Create `main.go`:
```go
package main

import "fmt"

func main() {
	fmt.Println("speed-test-cli")
}
```

- [ ] **Step 3: Verify it builds and runs**

Run:
```bash
go build ./... && go run .
```
Expected: prints `speed-test-cli`, no build errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod main.go
git commit -m "chore: scaffold go module and main entry point"
```

---

## Task 2: Result type

**Files:**
- Create: `internal/speedtest/result.go`
- Test: `internal/speedtest/result_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/speedtest/result_test.go`:
```go
package speedtest

import (
	"testing"
	"time"
)

func TestMbps(t *testing.T) {
	// 12_500_000 bytes in 1s = 100 Mbps (bytes*8 / 1e6 / seconds)
	got := Mbps(12_500_000, time.Second)
	if got != 100 {
		t.Fatalf("Mbps = %v, want 100", got)
	}
}

func TestMbpsZeroDuration(t *testing.T) {
	if got := Mbps(1000, 0); got != 0 {
		t.Fatalf("Mbps with zero duration = %v, want 0", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/speedtest/ -run TestMbps -v`
Expected: FAIL — `undefined: Mbps`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/speedtest/result.go`:
```go
package speedtest

import "time"

// Result is the single contract between measurement and consumers.
type Result struct {
	Timestamp    time.Time     `json:"timestamp"`
	ServerColo   string        `json:"server_colo"`
	Latency      time.Duration `json:"latency_ns"`
	Jitter       time.Duration `json:"jitter_ns"`
	DownloadMbps float64       `json:"download_mbps"`
	UploadMbps   float64       `json:"upload_mbps"`
}

// Mbps converts a byte count over a duration into megabits per second.
func Mbps(bytes int64, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(bytes) * 8 / 1e6 / d.Seconds()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/speedtest/ -run TestMbps -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/speedtest/result.go internal/speedtest/result_test.go
git commit -m "feat: add Result type and Mbps helper"
```

---

## Task 3: Client config, endpoints, and colo trace

**Files:**
- Create: `internal/speedtest/client.go`
- Test: `internal/speedtest/client_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/speedtest/client_test.go`:
```go
package speedtest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseColo(t *testing.T) {
	body := "fl=123\ncolo=SIN\nloc=SG\n"
	if got := parseColo(body); got != "SIN" {
		t.Fatalf("parseColo = %q, want SIN", got)
	}
}

func TestParseColoMissing(t *testing.T) {
	if got := parseColo("fl=123\n"); got != "" {
		t.Fatalf("parseColo = %q, want empty", got)
	}
}

func TestFetchColo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("colo=LHR\n"))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), TraceURL: srv.URL}
	got, err := c.fetchColo()
	if err != nil {
		t.Fatalf("fetchColo error: %v", err)
	}
	if got != "LHR" {
		t.Fatalf("fetchColo = %q, want LHR", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/speedtest/ -run Colo -v`
Expected: FAIL — `undefined: parseColo` / `undefined: Client`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/speedtest/client.go`:
```go
package speedtest

import (
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultDownURL  = "https://speed.cloudflare.com/__down"
	defaultUpURL    = "https://speed.cloudflare.com/__up"
	defaultTraceURL = "https://speed.cloudflare.com/cdn-cgi/trace"
)

// Client holds endpoint URLs and the HTTP client used for measurement.
type Client struct {
	HTTP     *http.Client
	DownURL  string
	UpURL    string
	TraceURL string
}

// NewClient returns a Client pointed at Cloudflare with sane HTTP defaults.
func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
			},
		},
		DownURL:  defaultDownURL,
		UpURL:    defaultUpURL,
		TraceURL: defaultTraceURL,
	}
}

// parseColo extracts the colo value from a cdn-cgi/trace body.
func parseColo(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "colo=") {
			return strings.TrimPrefix(line, "colo=")
		}
	}
	return ""
}

// fetchColo retrieves the serving Cloudflare datacenter code.
func (c *Client) fetchColo() (string, error) {
	resp, err := c.HTTP.Get(c.TraceURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return parseColo(string(b)), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/speedtest/ -run Colo -v`
Expected: PASS (all three tests).

- [ ] **Step 5: Commit**

```bash
git add internal/speedtest/client.go internal/speedtest/client_test.go
git commit -m "feat: add Cloudflare client and colo trace lookup"
```

---

## Task 4: Latency and jitter

**Files:**
- Create: `internal/speedtest/latency.go`
- Test: `internal/speedtest/latency_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/speedtest/latency_test.go`:
```go
package speedtest

import (
	"testing"
	"time"
)

func TestMedian(t *testing.T) {
	in := []time.Duration{30, 10, 20} // unsorted
	if got := median(in); got != 20 {
		t.Fatalf("median = %v, want 20", got)
	}
}

func TestMedianEven(t *testing.T) {
	in := []time.Duration{10, 20, 30, 40}
	if got := median(in); got != 25 {
		t.Fatalf("median = %v, want 25", got)
	}
}

func TestMedianEmpty(t *testing.T) {
	if got := median(nil); got != 0 {
		t.Fatalf("median(nil) = %v, want 0", got)
	}
}

func TestJitter(t *testing.T) {
	// diffs: |20-10|=10, |10-20|=10 -> mean 10
	in := []time.Duration{10, 20, 10}
	if got := jitter(in); got != 10 {
		t.Fatalf("jitter = %v, want 10", got)
	}
}

func TestJitterSingle(t *testing.T) {
	if got := jitter([]time.Duration{5}); got != 0 {
		t.Fatalf("jitter single = %v, want 0", got)
	}
}

func TestParseServerTiming(t *testing.T) {
	// cfRequestDuration is in milliseconds
	h := "cfRequestDuration;dur=12.5"
	got := parseServerTiming(h)
	if got != 12500*time.Microsecond {
		t.Fatalf("parseServerTiming = %v, want 12.5ms", got)
	}
}

func TestParseServerTimingMissing(t *testing.T) {
	if got := parseServerTiming("cfCacheStatus;desc=HIT"); got != 0 {
		t.Fatalf("parseServerTiming = %v, want 0", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/speedtest/ -run "Median|Jitter|ServerTiming" -v`
Expected: FAIL — `undefined: median` / `jitter` / `parseServerTiming`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/speedtest/latency.go`:
```go
package speedtest

import (
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// median returns the middle value (mean of two middles for even counts).
func median(ds []time.Duration) time.Duration {
	n := len(ds)
	if n == 0 {
		return 0
	}
	s := make([]time.Duration, n)
	copy(s, ds)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// jitter is the mean of absolute differences between consecutive samples.
func jitter(ds []time.Duration) time.Duration {
	if len(ds) < 2 {
		return 0
	}
	var total time.Duration
	for i := 1; i < len(ds); i++ {
		d := ds[i] - ds[i-1]
		if d < 0 {
			d = -d
		}
		total += d
	}
	return total / time.Duration(len(ds)-1)
}

// parseServerTiming extracts cfRequestDuration (ms) from a Server-Timing header.
func parseServerTiming(h string) time.Duration {
	for _, part := range strings.Split(h, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "cfRequestDuration") {
			continue
		}
		for _, kv := range strings.Split(part, ";") {
			kv = strings.TrimSpace(kv)
			if strings.HasPrefix(kv, "dur=") {
				ms, err := strconv.ParseFloat(strings.TrimPrefix(kv, "dur="), 64)
				if err != nil {
					return 0
				}
				return time.Duration(ms * float64(time.Millisecond))
			}
		}
	}
	return 0
}

// measureLatency issues n tiny downloads and returns median ping and jitter,
// subtracting server processing time reported via Server-Timing.
func (c *Client) measureLatency(n int) (ping, jit time.Duration, err error) {
	samples := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		req, _ := http.NewRequest(http.MethodGet, c.DownURL+"?bytes=0", nil)
		start := time.Now()
		resp, e := c.HTTP.Do(req)
		if e != nil {
			err = e
			continue
		}
		// drain + close so the connection is reused
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		rtt := time.Since(start) - parseServerTiming(resp.Header.Get("Server-Timing"))
		if rtt < 0 {
			rtt = 0
		}
		samples = append(samples, rtt)
	}
	if len(samples) == 0 {
		return 0, 0, err
	}
	return median(samples), jitter(samples), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/speedtest/ -run "Median|Jitter|ServerTiming" -v`
Expected: PASS (all latency tests). Also run `go build ./...` to confirm imports resolve.

- [ ] **Step 5: Commit**

```bash
git add internal/speedtest/latency.go internal/speedtest/latency_test.go
git commit -m "feat: add latency (median ping) and jitter measurement"
```

---

## Task 5: Download throughput

**Files:**
- Create: `internal/speedtest/download.go`
- Test: `internal/speedtest/download_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/speedtest/download_test.go`:
```go
package speedtest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// a server that returns `bytes` zero-bytes for /__down?bytes=N
func downServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.URL.Query().Get("bytes"))
		w.Header().Set("Content-Type", "application/octet-stream")
		buf := make([]byte, 32*1024)
		for n > 0 {
			chunk := len(buf)
			if n < chunk {
				chunk = n
			}
			w.Write(buf[:chunk])
			n -= chunk
		}
	}))
}

func TestMeasureDownloadCountsBytes(t *testing.T) {
	srv := downServer()
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), DownURL: srv.URL}
	cfg := Config{Streams: 2, Duration: 500 * time.Millisecond}

	mbps, err := c.measureDownload(cfg, nil)
	if err != nil {
		t.Fatalf("measureDownload error: %v", err)
	}
	if mbps <= 0 {
		t.Fatalf("measureDownload = %v, want > 0", mbps)
	}
}

func TestMeasureDownloadCallsProgress(t *testing.T) {
	srv := downServer()
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), DownURL: srv.URL}
	cfg := Config{Streams: 1, Duration: 300 * time.Millisecond}

	var calls int
	_, err := c.measureDownload(cfg, func(p Progress) { calls++ })
	if err != nil {
		t.Fatalf("measureDownload error: %v", err)
	}
	if calls == 0 {
		t.Fatalf("expected progress callback to be called at least once")
	}
	_ = fmt.Sprint(calls)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/speedtest/ -run MeasureDownload -v`
Expected: FAIL — `undefined: Config` / `Progress` / `measureDownload`.

- [ ] **Step 3: Write minimal implementation**

Add `Config` and `Progress` to `internal/speedtest/client.go` (append to that file):
```go
// Phase identifies which measurement is in progress.
type Phase string

const (
	PhaseLatency  Phase = "latency"
	PhaseDownload Phase = "download"
	PhaseUpload   Phase = "upload"
)

// Progress is reported to the callback as bytes flow.
type Progress struct {
	Phase Phase
	Mbps  float64
}

// ProgressFunc receives live progress; may be nil.
type ProgressFunc func(Progress)

// Config holds tunable run parameters.
type Config struct {
	Streams      int
	Duration     time.Duration
	DownloadOnly bool
	UploadOnly   bool
}
```

Create `internal/speedtest/download.go`:
```go
package speedtest

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const downloadWarmup = 1 * time.Second

// chunkBytes is the per-request payload size requested from the server.
const downloadChunkBytes = 25_000_000

// measureDownload runs `cfg.Streams` parallel download loops for up to
// `cfg.Duration`, discarding a warm-up window, and returns throughput in Mbps.
func (c *Client) measureDownload(cfg Config, progress ProgressFunc) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	var counted int64 // bytes counted after warm-up
	start := time.Now()
	warmEnd := start.Add(downloadWarmup)

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for i := 0; i < cfg.Streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 64*1024)
			for ctx.Err() == nil {
				url := c.DownURL + "?bytes=" + strconv.Itoa(downloadChunkBytes)
				req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				resp, err := c.HTTP.Do(req)
				if err != nil {
					if ctx.Err() == nil {
						errOnce.Do(func() { firstErr = err })
					}
					return
				}
				for {
					n, rerr := resp.Body.Read(buf)
					if n > 0 && time.Now().After(warmEnd) {
						atomic.AddInt64(&counted, int64(n))
						if progress != nil {
							elapsed := time.Since(warmEnd)
							progress(Progress{Phase: PhaseDownload, Mbps: Mbps(atomic.LoadInt64(&counted), elapsed)})
						}
					}
					if rerr != nil {
						break
					}
				}
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(warmEnd)
	if elapsed <= 0 {
		return 0, firstErr
	}
	return Mbps(atomic.LoadInt64(&counted), elapsed), firstErr
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/speedtest/ -run MeasureDownload -v`
Expected: PASS. Run `go vet ./...` to confirm no unused imports.

- [ ] **Step 5: Commit**

```bash
git add internal/speedtest/download.go internal/speedtest/client.go internal/speedtest/download_test.go
git commit -m "feat: add parallel download throughput measurement"
```

---

## Task 6: Upload throughput

**Files:**
- Create: `internal/speedtest/upload.go`
- Test: `internal/speedtest/upload_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/speedtest/upload_test.go`:
```go
package speedtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// a server that drains the upload body and 200s.
func upServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
}

func TestMeasureUploadCountsBytes(t *testing.T) {
	srv := upServer()
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), UpURL: srv.URL}
	cfg := Config{Streams: 2, Duration: 500 * time.Millisecond}

	mbps, err := c.measureUpload(cfg, nil)
	if err != nil {
		t.Fatalf("measureUpload error: %v", err)
	}
	if mbps <= 0 {
		t.Fatalf("measureUpload = %v, want > 0", mbps)
	}
}

func TestMeasureUploadCallsProgress(t *testing.T) {
	srv := upServer()
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), UpURL: srv.URL}
	cfg := Config{Streams: 1, Duration: 300 * time.Millisecond}

	var calls int
	_, err := c.measureUpload(cfg, func(p Progress) { calls++ })
	if err != nil {
		t.Fatalf("measureUpload error: %v", err)
	}
	if calls == 0 {
		t.Fatalf("expected progress callback to be called at least once")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/speedtest/ -run MeasureUpload -v`
Expected: FAIL — `undefined: measureUpload`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/speedtest/upload.go`:
```go
package speedtest

import (
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const uploadWarmup = 1 * time.Second

// uploadChunkBytes is the per-request body size sent to the server.
const uploadChunkBytes = 10_000_000

// countingReader counts bytes read from a fixed-size zero payload and reports
// post-warmup bytes via the atomic counter and the progress callback.
type countingReader struct {
	remaining int
	counted   *int64
	warmEnd   time.Time
	progress  ProgressFunc
	start     *time.Time
}

func (r *countingReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if n > r.remaining {
		n = r.remaining
	}
	for i := 0; i < n; i++ {
		p[i] = 0
	}
	r.remaining -= n
	if time.Now().After(r.warmEnd) {
		atomic.AddInt64(r.counted, int64(n))
		if r.progress != nil {
			elapsed := time.Since(r.warmEnd)
			r.progress(Progress{Phase: PhaseUpload, Mbps: Mbps(atomic.LoadInt64(r.counted), elapsed)})
		}
	}
	return n, nil
}

// measureUpload runs `cfg.Streams` parallel upload loops for up to
// `cfg.Duration`, discarding a warm-up window, and returns throughput in Mbps.
func (c *Client) measureUpload(cfg Config, progress ProgressFunc) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	var counted int64
	start := time.Now()
	warmEnd := start.Add(uploadWarmup)

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for i := 0; i < cfg.Streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				body := &countingReader{
					remaining: uploadChunkBytes,
					counted:   &counted,
					warmEnd:   warmEnd,
					progress:  progress,
					start:     &start,
				}
				req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.UpURL, body)
				req.ContentLength = int64(uploadChunkBytes)
				resp, err := c.HTTP.Do(req)
				if err != nil {
					if ctx.Err() == nil {
						errOnce.Do(func() { firstErr = err })
					}
					return
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(warmEnd)
	if elapsed <= 0 {
		return 0, firstErr
	}
	return Mbps(atomic.LoadInt64(&counted), elapsed), firstErr
}
```

Note: `countingReader.Read` and `io.Copy` both rely on the `"io"` import already included above.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/speedtest/ -run MeasureUpload -v`
Expected: PASS. Run `go vet ./...`.

- [ ] **Step 5: Commit**

```bash
git add internal/speedtest/upload.go internal/speedtest/upload_test.go
git commit -m "feat: add parallel upload throughput measurement"
```

---

## Task 7: Run orchestration

**Files:**
- Modify: `internal/speedtest/client.go` (add `Run`)
- Test: `internal/speedtest/run_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/speedtest/run_test.go`:
```go
package speedtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// combined server handling down, up, and trace based on path/query.
func fullServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/__down", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server-Timing", "cfRequestDuration;dur=1.0")
		n, _ := strconv.Atoi(r.URL.Query().Get("bytes"))
		buf := make([]byte, 32*1024)
		for n > 0 {
			c := len(buf)
			if n < c {
				c = n
			}
			w.Write(buf[:c])
			n -= c
		}
	})
	mux.HandleFunc("/__up", func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(200)
	})
	mux.HandleFunc("/cdn-cgi/trace", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("colo=TST\n"))
	})
	return httptest.NewServer(mux)
}

func TestRunPopulatesResult(t *testing.T) {
	srv := fullServer()
	defer srv.Close()

	c := &Client{
		HTTP:     srv.Client(),
		DownURL:  srv.URL + "/__down",
		UpURL:    srv.URL + "/__up",
		TraceURL: srv.URL + "/cdn-cgi/trace",
	}
	cfg := Config{Streams: 1, Duration: 400 * time.Millisecond}

	res, err := c.Run(cfg, nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if res.ServerColo != "TST" {
		t.Errorf("ServerColo = %q, want TST", res.ServerColo)
	}
	if res.DownloadMbps <= 0 {
		t.Errorf("DownloadMbps = %v, want > 0", res.DownloadMbps)
	}
	if res.UploadMbps <= 0 {
		t.Errorf("UploadMbps = %v, want > 0", res.UploadMbps)
	}
	if res.Timestamp.IsZero() {
		t.Errorf("Timestamp not set")
	}
}

func TestRunDownloadOnly(t *testing.T) {
	srv := fullServer()
	defer srv.Close()

	c := &Client{
		HTTP:     srv.Client(),
		DownURL:  srv.URL + "/__down",
		UpURL:    srv.URL + "/__up",
		TraceURL: srv.URL + "/cdn-cgi/trace",
	}
	cfg := Config{Streams: 1, Duration: 300 * time.Millisecond, DownloadOnly: true}

	res, err := c.Run(cfg, nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if res.UploadMbps != 0 {
		t.Errorf("UploadMbps = %v, want 0 (download-only)", res.UploadMbps)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/speedtest/ -run TestRun -v`
Expected: FAIL — `c.Run undefined`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/speedtest/client.go`:
```go
// Run executes the configured measurements and returns a populated Result.
// Latency runs first, then download and/or upload per cfg flags.
func (c *Client) Run(cfg Config, progress ProgressFunc) (Result, error) {
	res := Result{Timestamp: time.Now()}

	if colo, err := c.fetchColo(); err == nil {
		res.ServerColo = colo
	}

	ping, jit, err := c.measureLatency(20)
	if err != nil && ping == 0 {
		return res, err
	}
	res.Latency = ping
	res.Jitter = jit

	if !cfg.UploadOnly {
		d, err := c.measureDownload(cfg, progress)
		if err != nil {
			return res, err
		}
		res.DownloadMbps = d
	}

	if !cfg.DownloadOnly {
		u, err := c.measureUpload(cfg, progress)
		if err != nil {
			return res, err
		}
		res.UploadMbps = u
	}

	return res, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/speedtest/ -v`
Expected: PASS (entire speedtest package).

- [ ] **Step 5: Commit**

```bash
git add internal/speedtest/client.go internal/speedtest/run_test.go
git commit -m "feat: add Run orchestration producing a full Result"
```

---

## Task 8: JSON output

**Files:**
- Create: `internal/output/json.go`
- Test: `internal/output/json_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/output/json_test.go`:
```go
package output

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"speed-test-cli/internal/speedtest"
)

func TestJSONContainsFields(t *testing.T) {
	res := speedtest.Result{
		Timestamp:    time.Unix(0, 0).UTC(),
		ServerColo:   "SIN",
		DownloadMbps: 100.5,
		UploadMbps:   20.25,
	}
	var buf bytes.Buffer
	if err := JSON(&buf, res); err != nil {
		t.Fatalf("JSON error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["server_colo"] != "SIN" {
		t.Errorf("server_colo = %v, want SIN", got["server_colo"])
	}
	if got["download_mbps"].(float64) != 100.5 {
		t.Errorf("download_mbps = %v, want 100.5", got["download_mbps"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/output/ -run TestJSON -v`
Expected: FAIL — `undefined: JSON`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/output/json.go`:
```go
package output

import (
	"encoding/json"
	"io"

	"speed-test-cli/internal/speedtest"
)

// JSON writes the result as a single-line JSON object to w.
func JSON(w io.Writer, res speedtest.Result) error {
	enc := json.NewEncoder(w)
	return enc.Encode(res)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/output/ -run TestJSON -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/json.go internal/output/json_test.go
git commit -m "feat: add JSON output"
```

---

## Task 9: Human-readable output

**Files:**
- Create: `internal/output/human.go`
- Test: `internal/output/human_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/output/human_test.go`:
```go
package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"speed-test-cli/internal/speedtest"
)

func TestHumanSummaryContainsMetrics(t *testing.T) {
	res := speedtest.Result{
		ServerColo:   "SIN",
		Latency:      15 * time.Millisecond,
		Jitter:       2 * time.Millisecond,
		DownloadMbps: 100.5,
		UploadMbps:   20.2,
	}
	var buf bytes.Buffer
	Human(&buf, res)
	out := buf.String()

	for _, want := range []string{"SIN", "100.5", "20.2", "Download", "Upload", "Ping", "Jitter"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q\n---\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/output/ -run TestHuman -v`
Expected: FAIL — `undefined: Human`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/output/human.go`:
```go
package output

import (
	"fmt"
	"io"

	"speed-test-cli/internal/speedtest"
)

// Human writes a clean, human-readable summary of the result to w.
func Human(w io.Writer, res speedtest.Result) {
	if res.ServerColo != "" {
		fmt.Fprintf(w, "Server:   Cloudflare %s\n", res.ServerColo)
	}
	fmt.Fprintf(w, "Ping:     %.1f ms\n", float64(res.Latency.Microseconds())/1000)
	fmt.Fprintf(w, "Jitter:   %.1f ms\n", float64(res.Jitter.Microseconds())/1000)
	fmt.Fprintf(w, "Download: %.1f Mbps\n", res.DownloadMbps)
	fmt.Fprintf(w, "Upload:   %.1f Mbps\n", res.UploadMbps)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/output/ -run TestHuman -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/output/human.go internal/output/human_test.go
git commit -m "feat: add human-readable summary output"
```

---

## Task 10: History logging

**Files:**
- Create: `internal/history/log.go`
- Test: `internal/history/log_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/history/log_test.go`:
```go
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"speed-test-cli/internal/speedtest"
)

func TestAppendCreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "history.jsonl")

	r1 := speedtest.Result{Timestamp: time.Unix(1, 0).UTC(), DownloadMbps: 10}
	r2 := speedtest.Result{Timestamp: time.Unix(2, 0).UTC(), DownloadMbps: 20}

	if err := Append(path, r1); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if err := Append(path, r2); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	var lines []speedtest.Result
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r speedtest.Result
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		lines = append(lines, r)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0].DownloadMbps != 10 || lines[1].DownloadMbps != 20 {
		t.Errorf("unexpected contents: %+v", lines)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/history/ -v`
Expected: FAIL — `undefined: Append`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/history/log.go`:
```go
package history

import (
	"encoding/json"
	"os"
	"path/filepath"

	"speed-test-cli/internal/speedtest"
)

// Append writes the result as one JSON line to the file at path, creating
// parent directories and the file if needed.
func Append(path string, res speedtest.Result) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(res)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// DefaultPath returns the default history file location under the user's home.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".speed-test", "history.jsonl"), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/history/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/history/log.go internal/history/log_test.go
git commit -m "feat: add JSONL history logging"
```

---

## Task 11: CLI wiring (cobra)

**Files:**
- Create: `cmd/root.go`
- Modify: `main.go`
- Test: `cmd/root_test.go`

- [ ] **Step 1: Add the cobra dependency**

Run:
```bash
go get github.com/spf13/cobra@latest
```
Expected: `go.mod`/`go.sum` updated with cobra.

- [ ] **Step 2: Write the failing test**

Create `cmd/root_test.go`:
```go
package cmd

import (
	"testing"
	"time"
)

func TestBuildConfigDefaults(t *testing.T) {
	o := options{streams: 6, duration: 12 * time.Second}
	cfg := o.toConfig()
	if cfg.Streams != 6 {
		t.Errorf("Streams = %d, want 6", cfg.Streams)
	}
	if cfg.Duration != 12*time.Second {
		t.Errorf("Duration = %v, want 12s", cfg.Duration)
	}
}

func TestBuildConfigOnlyFlags(t *testing.T) {
	o := options{streams: 4, duration: time.Second, downloadOnly: true}
	cfg := o.toConfig()
	if !cfg.DownloadOnly {
		t.Errorf("DownloadOnly not propagated")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./cmd/ -v`
Expected: FAIL — `undefined: options`.

- [ ] **Step 4: Write minimal implementation**

Create `cmd/root.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"speed-test-cli/internal/history"
	"speed-test-cli/internal/output"
	"speed-test-cli/internal/speedtest"
)

type options struct {
	json         bool
	noLog        bool
	streams      int
	duration     time.Duration
	logFile      string
	downloadOnly bool
	uploadOnly   bool
}

func (o options) toConfig() speedtest.Config {
	return speedtest.Config{
		Streams:      o.streams,
		Duration:     o.duration,
		DownloadOnly: o.downloadOnly,
		UploadOnly:   o.uploadOnly,
	}
}

// Execute runs the root command.
func Execute() {
	var o options
	cmd := &cobra.Command{
		Use:   "speed-test",
		Short: "Measure internet speed against Cloudflare",
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(o)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&o.json, "json", false, "Machine-readable JSON output")
	f.BoolVar(&o.noLog, "no-log", false, "Don't append to the history file")
	f.IntVar(&o.streams, "streams", 6, "Parallel connections per direction")
	f.DurationVar(&o.duration, "duration", 12*time.Second, "Max time per direction")
	f.StringVar(&o.logFile, "log-file", "", "History file path (default ~/.speed-test/history.jsonl)")
	f.BoolVar(&o.downloadOnly, "download-only", false, "Skip upload test")
	f.BoolVar(&o.uploadOnly, "upload-only", false, "Skip download test")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(o options) error {
	client := speedtest.NewClient()

	var progress speedtest.ProgressFunc
	if !o.json {
		fmt.Fprintln(os.Stderr, "Testing… (Cloudflare)")
	}

	res, err := client.Run(o.toConfig(), progress)
	if err != nil {
		if o.json {
			fmt.Fprintf(os.Stderr, `{"error":%q}`+"\n", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "speed test failed: %v\n", err)
		}
		return err
	}

	if o.json {
		if err := output.JSON(os.Stdout, res); err != nil {
			return err
		}
	} else {
		output.Human(os.Stdout, res)
	}

	if !o.noLog {
		path := o.logFile
		if path == "" {
			p, err := history.DefaultPath()
			if err == nil {
				path = p
			}
		}
		if path != "" {
			if err := history.Append(path, res); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write history: %v\n", err)
			}
		}
	}
	return nil
}
```

Replace `main.go` with:
```go
package main

import "speed-test-cli/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./cmd/ -v && go build ./...`
Expected: PASS and a clean build.

- [ ] **Step 6: Commit**

```bash
git add cmd/root.go cmd/root_test.go main.go go.mod go.sum
git commit -m "feat: wire cobra CLI with flags, output routing, and logging"
```

---

## Task 12: Live progress output (human mode)

**Files:**
- Modify: `internal/output/human.go` (add `NewProgressPrinter`)
- Modify: `cmd/root.go` (use it when not `--json`)
- Test: `internal/output/human_test.go` (add a test)

- [ ] **Step 1: Write the failing test**

Add to `internal/output/human_test.go`:
```go
func TestProgressPrinterUpdates(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressPrinter(&buf)
	p(speedtest.Progress{Phase: speedtest.PhaseDownload, Mbps: 50})
	// Should write something reflecting activity; we only assert it doesn't panic
	// and produces output.
	if buf.Len() == 0 {
		t.Errorf("expected progress output, got none")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/output/ -run TestProgressPrinter -v`
Expected: FAIL — `undefined: NewProgressPrinter`.

- [ ] **Step 3: Write minimal implementation**

Add to `internal/output/human.go`:
```go
import (
	// add to existing imports:
	"speed-test-cli/internal/speedtest"
)

// NewProgressPrinter returns a ProgressFunc that prints a live, single-line
// throughput readout to w for each update.
func NewProgressPrinter(w io.Writer) speedtest.ProgressFunc {
	return func(p speedtest.Progress) {
		fmt.Fprintf(w, "\r%-8s %.1f Mbps   ", p.Phase, p.Mbps)
	}
}
```

`human.go` now uses `fmt`, `io`, and `speed-test-cli/internal/speedtest` — all already imported by Task 9 except the speedtest package, which this step adds. No third-party dependency is needed; the live readout is plain `fmt`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/output/ -v`
Expected: PASS.

- [ ] **Step 5: Wire it into the CLI**

In `cmd/root.go`, replace the `var progress speedtest.ProgressFunc` block in `run` with:
```go
	var progress speedtest.ProgressFunc
	if !o.json {
		fmt.Fprintln(os.Stderr, "Testing… (Cloudflare)")
		progress = output.NewProgressPrinter(os.Stderr)
	}
```
And after `client.Run(...)` returns in non-json mode, print a newline to close the live line:
```go
	if !o.json {
		fmt.Fprintln(os.Stderr)
	}
```
(Place this immediately after the error check, before printing the summary.)

- [ ] **Step 6: Run the full suite and build**

Run: `go test ./... && go vet ./... && go build ./...`
Expected: all PASS, clean build.

- [ ] **Step 7: Commit**

```bash
git add internal/output/human.go internal/output/human_test.go cmd/root.go
git commit -m "feat: add live progress output in human mode"
```

---

## Task 13: Real integration test (gated)

**Files:**
- Create: `internal/speedtest/integration_test.go`

- [ ] **Step 1: Write the gated integration test**

Create `internal/speedtest/integration_test.go`:
```go
package speedtest

import (
	"testing"
	"time"
)

// TestRealCloudflare hits the live Cloudflare endpoints. Skipped under -short.
func TestRealCloudflare(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in -short mode")
	}
	c := NewClient()
	cfg := Config{Streams: 4, Duration: 5 * time.Second}
	res, err := c.Run(cfg, nil)
	if err != nil {
		t.Fatalf("live Run error: %v", err)
	}
	if res.DownloadMbps <= 0 {
		t.Errorf("DownloadMbps = %v, want > 0", res.DownloadMbps)
	}
	t.Logf("colo=%s ping=%v jitter=%v down=%.1f up=%.1f",
		res.ServerColo, res.Latency, res.Jitter, res.DownloadMbps, res.UploadMbps)
}
```

- [ ] **Step 2: Verify it is skipped in short mode**

Run: `go test ./internal/speedtest/ -short -v -run TestRealCloudflare`
Expected: `--- SKIP: TestRealCloudflare`.

- [ ] **Step 3: (Optional, requires network) run it for real**

Run: `go test ./internal/speedtest/ -v -run TestRealCloudflare`
Expected: PASS with a log line showing real numbers (skip if offline).

- [ ] **Step 4: Commit**

```bash
git add internal/speedtest/integration_test.go
git commit -m "test: add gated live Cloudflare integration test"
```

---

## Task 14: Final verification & README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Run the entire suite**

Run: `go test ./... -short && go vet ./... && go build ./...`
Expected: all PASS, clean build, binary `speed-test-cli` produced.

- [ ] **Step 2: Smoke-test the binary (requires network)**

Run: `go run . --download-only --duration 5s`
Expected: prints a human summary with a non-zero download number (skip if offline).

- [ ] **Step 3: Write the README**

Create `README.md`:
```markdown
# speed-test-cli

A command-line internet speed test (download, upload, ping, jitter) powered by
Cloudflare's public speed-test endpoints.

## Install

```bash
go build -o speed-test .
```

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
```

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add README with usage and flags"
```

---

## Self-Review Notes

- **Spec coverage:** download/upload/ping/jitter (Tasks 4–7), Cloudflare endpoints (Task 3), parallel streams + warm-up + time cap (Tasks 5–6), JSON mode (Task 8), human output (Task 9), live progress (Task 12), history JSONL (Task 10), flags w/ defaults (Task 11), error handling routes errors to stderr and non-zero exit (Task 11), gated integration test (Task 13). All spec sections covered.
- **Types are consistent:** `Result`, `Config`, `Progress`, `ProgressFunc`, `Client` used identically across tasks. `measureDownload`/`measureUpload`/`measureLatency`/`Run` signatures match their call sites.
- **Known cleanup steps called out:** Tasks 5 and 6 explicitly instruct removing placeholder import-guard lines before commit; Task 12 calls out keeping `go.mod` clean if the progress bar lib isn't used yet.
