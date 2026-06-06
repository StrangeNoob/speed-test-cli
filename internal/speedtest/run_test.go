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
