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
