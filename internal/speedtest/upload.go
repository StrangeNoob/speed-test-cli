package speedtest

import (
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// uploadChunkBytes is the per-request body size sent to the server.
const uploadChunkBytes = 10_000_000

// countingReader streams a fixed-size zero payload, counting post-warmup bytes
// into the shared atomic counter and reporting throttled progress.
type countingReader struct {
	remaining    int
	counted      *int64
	warmEnd      time.Time
	progress     ProgressFunc
	lastProgress *time.Time
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
		if r.progress != nil && time.Since(*r.lastProgress) > 100*time.Millisecond {
			*r.lastProgress = time.Now()
			elapsed := time.Since(r.warmEnd)
			r.progress(Progress{Phase: PhaseUpload, Mbps: Mbps(atomic.LoadInt64(r.counted), elapsed)})
		}
	}
	return n, nil
}

// measureUpload runs `cfg.Streams` parallel upload loops for up to
// `cfg.Duration`, discarding a proportional warm-up window, and returns
// throughput in Mbps.
func (c *Client) measureUpload(cfg Config, progress ProgressFunc) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	warmDuration := cfg.Duration / 5
	var counted int64
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
					counted:      &counted,
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
