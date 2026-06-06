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
