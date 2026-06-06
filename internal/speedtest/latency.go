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
