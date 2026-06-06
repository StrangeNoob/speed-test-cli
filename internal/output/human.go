package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"speed-test-cli/internal/speedtest"
)

// NewProgressPrinter returns a ProgressFunc that prints a live, single-line
// throughput readout to w for each update.
func NewProgressPrinter(w io.Writer) speedtest.ProgressFunc {
	return func(p speedtest.Progress) {
		fmt.Fprintf(w, "\r%-8s %.1f Mbps   ", p.Phase, p.Mbps)
	}
}

// barWidth is the cell width of summary throughput bars.
const barWidth = 16

// Human writes a clean, colored, human-readable summary of the result to w.
// Pass NewStyler(false) for plain output.
func Human(w io.Writer, res speedtest.Result, st *Styler) {
	if res.ServerColo != "" {
		fmt.Fprintf(w, "%s  %s  %s %s\n\n",
			st.Bold("speed-test"), st.Dim("•"), st.Dim("Cloudflare"), st.Bold(res.ServerColo))
	}
	fmt.Fprintf(w, "%s  %s    %s  %s\n\n",
		st.Cyan("Ping   "), fmtMs(res.Latency),
		st.Cyan("Jitter"), fmtMs(res.Jitter))

	scale := res.DownloadMbps
	if res.UploadMbps > scale {
		scale = res.UploadMbps
	}

	// A Result can't distinguish a skipped direction from a measured zero
	// (both are 0.0), so any zero renders a dash instead of a bar.
	writeRate(w, st, "Download", res.DownloadMbps, scale)
	writeRate(w, st, "Upload  ", res.UploadMbps, scale)
}

// writeRate renders one labeled throughput row with a scaled bar.
func writeRate(w io.Writer, st *Styler, label string, mbps, scale float64) {
	if mbps <= 0 {
		fmt.Fprintf(w, "%s  %s  %s\n", st.Cyan(label), st.Dim("▕"+spaces(barWidth)+"▏"), st.Dim("—"))
		return
	}
	bar := renderBar(mbps, scale, barWidth)
	fmt.Fprintf(w, "%s  %s%s%s  %.1f %s\n",
		st.Cyan(label), st.Dim("▕"), st.Green(bar), st.Dim("▏"), mbps, st.Dim("Mbps"))
}

func fmtMs(d time.Duration) string {
	return fmt.Sprintf("%.1f ms", float64(d.Microseconds())/1000)
}

func spaces(n int) string {
	return strings.Repeat(" ", n)
}
