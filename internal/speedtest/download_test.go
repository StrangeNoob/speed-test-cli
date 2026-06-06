package speedtest

import (
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
}
