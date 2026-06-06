package speedtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

func TestMeasureUploadRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), UpURL: srv.URL}
	cfg := Config{Streams: 2, Duration: 600 * time.Millisecond}
	mbps, err := c.measureUpload(cfg, nil)
	if err == nil {
		t.Fatalf("expected an error when rate-limited, got nil (mbps=%v)", mbps)
	}
	if mbps != 0 {
		t.Errorf("rate-limited upload should be 0 mbps, got %v", mbps)
	}
}

func TestMeasureUploadRecoversAfter429(t *testing.T) {
	old := rateLimitBackoff
	rateLimitBackoff = 10 * time.Millisecond
	defer func() { rateLimitBackoff = old }()

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		if atomic.AddInt32(&hits, 1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), UpURL: srv.URL}
	cfg := Config{Streams: 1, Duration: 600 * time.Millisecond}
	mbps, err := c.measureUpload(cfg, nil)
	if err != nil {
		t.Fatalf("expected recovery after transient 429s, got error: %v", err)
	}
	if mbps <= 0 {
		t.Errorf("expected >0 mbps after recovery, got %v", mbps)
	}
}
