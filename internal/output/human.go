package output

import (
	"fmt"
	"io"

	"speed-test-cli/internal/speedtest"
)

// Human writes a clean, human-readable summary of the result to w.
func Human(w io.Writer, res speedtest.Result) {
	if res.ServerColo != "" {
		fmt.Fprintf(w, "Server:   Cloudflare %s\n", res.ServerColo)
	}
	fmt.Fprintf(w, "Ping:     %.1f ms\n", float64(res.Latency.Microseconds())/1000)
	fmt.Fprintf(w, "Jitter:   %.1f ms\n", float64(res.Jitter.Microseconds())/1000)
	fmt.Fprintf(w, "Download: %.1f Mbps\n", res.DownloadMbps)
	fmt.Fprintf(w, "Upload:   %.1f Mbps\n", res.UploadMbps)
}
