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
