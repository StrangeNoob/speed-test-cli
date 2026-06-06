package speedtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// uploadChunkBytes is the per-request body size sent to the server.
const uploadChunkBytes = 10_000_000

// countingReader streams a fixed-size zero payload, tracking how many bytes were
// sent after the warm-up window (sentAfterWarm) so the caller can commit them to
// the shared total only on a successful (2xx) response. It also drives throttled
// live progress using committed-plus-in-flight bytes.
type countingReader struct {
	remaining     int
	sentAfterWarm int64
	committed     *int64
	warmEnd       time.Time
	progress      ProgressFunc
	lastProgress  *time.Time
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
		r.sentAfterWarm += int64(n)
		if r.progress != nil && time.Since(*r.lastProgress) > 100*time.Millisecond {
			*r.lastProgress = time.Now()
			elapsed := time.Since(r.warmEnd)
			total := atomic.LoadInt64(r.committed) + r.sentAfterWarm
			r.progress(Progress{Phase: PhaseUpload, Mbps: Mbps(total, elapsed)})
		}
	}
	return n, nil
}

// measureUpload runs `cfg.Streams` parallel upload loops for up to `cfg.Duration`,
// discarding a warm-up window of cfg.Duration/5, and returns throughput in Mbps.
// Bytes count toward the total only after a successful (200) response, so
// rate-limited (429) or failed uploads never inflate the result; an HTTP 429
// triggers a short backoff and retry, and a phase that commits zero bytes
// returns an error rather than a bogus value.
func (c *Client) measureUpload(cfg Config, progress ProgressFunc) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	warmDuration := cfg.Duration / 5
	var counted int64
	var rateLimited int32
	warmEnd := time.Now().Add(warmDuration)

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for i := 0; i < cfg.Streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var lastProgress time.Time
			for ctx.Err() == nil {
				body := &countingReader{
					remaining:    uploadChunkBytes,
					committed:    &counted,
					warmEnd:      warmEnd,
					progress:     progress,
					lastProgress: &lastProgress,
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.UpURL, body)
				if err != nil {
					if ctx.Err() == nil {
						errOnce.Do(func() { firstErr = err })
					}
					return
				}
				req.ContentLength = int64(uploadChunkBytes)
				resp, err := c.HTTP.Do(req)
				if err != nil {
					if ctx.Err() == nil {
						errOnce.Do(func() { firstErr = err })
					}
					return
				}
				status := resp.StatusCode
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if status != http.StatusOK {
					if status == http.StatusTooManyRequests {
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
							firstErr = fmt.Errorf("upload: unexpected status %s", resp.Status)
						})
					}
					return
				}
				// Commit only the bytes sent after warm-up, and only on success.
				atomic.AddInt64(&counted, body.sentAfterWarm)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(warmEnd)
	total := atomic.LoadInt64(&counted)
	if firstErr == nil && total == 0 {
		if atomic.LoadInt32(&rateLimited) == 1 {
			firstErr = errors.New("upload: rate limited by Cloudflare (wait ~30s and retry)")
		} else {
			firstErr = errors.New("upload: no data received")
		}
	}
	if elapsed <= 0 {
		return 0, firstErr
	}
	return Mbps(total, elapsed), firstErr
}
