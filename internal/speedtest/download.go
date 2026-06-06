package speedtest

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// downloadChunkBytes is the per-request payload size requested from the server.
const downloadChunkBytes = 25_000_000

// measureDownload runs `cfg.Streams` parallel download loops for up to
// `cfg.Duration`, discarding a warm-up window of cfg.Duration/5, and
// returns throughput in Mbps.
func (c *Client) measureDownload(cfg Config, progress ProgressFunc) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	var counted int64 // bytes counted after warm-up
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
