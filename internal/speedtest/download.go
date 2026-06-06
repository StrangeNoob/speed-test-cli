package speedtest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// downloadChunkBytes is the per-request payload size requested from the server.
const downloadChunkBytes = 25_000_000

// rateLimitBackoff is how long a stream waits before retrying after an HTTP 429.
// It is a var (not const) so tests can shorten it. Shared by download and upload.
var rateLimitBackoff = 1 * time.Second

// measureDownload runs `cfg.Streams` parallel download loops for up to
// `cfg.Duration`, discarding a warm-up window of cfg.Duration/5, and returns
// throughput in Mbps. Non-2xx responses are never counted; an HTTP 429 triggers
// a short backoff and retry; if no data is measured the call returns an error
// rather than a bogus 0 Mbps.
func (c *Client) measureDownload(cfg Config, progress ProgressFunc) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	var counted int64 // bytes counted after warm-up
	var rateLimited int32
	start := time.Now()
	warmDuration := cfg.Duration / 5
	warmEnd := start.Add(warmDuration)

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for i := 0; i < cfg.Streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 64*1024)
			var lastProgress time.Time
			for ctx.Err() == nil {
				reqURL := c.DownURL + "?bytes=" + strconv.Itoa(downloadChunkBytes)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
				if err != nil {
					if ctx.Err() == nil {
						errOnce.Do(func() { firstErr = err })
					}
					return
				}
				resp, err := c.HTTP.Do(req)
				if err != nil {
					if ctx.Err() == nil {
						errOnce.Do(func() { firstErr = err })
					}
					return
				}
				if resp.StatusCode != http.StatusOK {
					resp.Body.Close()
					if resp.StatusCode == http.StatusTooManyRequests {
						atomic.StoreInt32(&rateLimited, 1)
						select {
						case <-ctx.Done():
							return
						case <-time.After(rateLimitBackoff):
						}
						continue
					}
					if ctx.Err() == nil {
						errOnce.Do(func() {
							firstErr = fmt.Errorf("download: unexpected status %s", resp.Status)
						})
					}
					return
				}
				for {
					n, rerr := resp.Body.Read(buf)
					if n > 0 && time.Now().After(warmEnd) {
						atomic.AddInt64(&counted, int64(n))
						if progress != nil && time.Since(lastProgress) > 100*time.Millisecond {
							lastProgress = time.Now()
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
	total := atomic.LoadInt64(&counted)
	if firstErr == nil && total == 0 {
		if atomic.LoadInt32(&rateLimited) == 1 {
			firstErr = errors.New("download: rate limited by Cloudflare (wait ~30s and retry)")
		} else {
			firstErr = errors.New("download: no data received")
		}
	}
	if elapsed <= 0 {
		return 0, firstErr
	}
	return Mbps(total, elapsed), firstErr
}
