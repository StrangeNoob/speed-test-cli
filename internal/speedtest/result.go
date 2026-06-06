package speedtest

import "time"

// Result is the single contract between measurement and consumers.
type Result struct {
	Timestamp    time.Time     `json:"timestamp"`
	ServerColo   string        `json:"server_colo"`
	Latency      time.Duration `json:"latency_ns"`
	Jitter       time.Duration `json:"jitter_ns"`
	DownloadMbps float64       `json:"download_mbps"`
	UploadMbps   float64       `json:"upload_mbps"`
}

// Mbps converts a byte count over a duration into megabits per second.
func Mbps(bytes int64, d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(bytes) * 8 / 1e6 / d.Seconds()
}
