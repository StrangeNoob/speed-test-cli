package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// liveBarWidth is the cell width of the live progress bar.
const liveBarWidth = 20

// NewProgressPrinter returns a ProgressFunc that renders a live, single-line
// spinner + auto-scaling bar + throughput to w. When animate is false it
// produces no output. The bar scales to the peak Mbps seen so far in the
// current phase; a phase change finalizes the previous line with a newline.
func NewProgressPrinter(w io.Writer, animate bool) speedtest.ProgressFunc {
	st := NewStyler(animate)
	var current speedtest.Phase
	var peak float64
	frame := 0
	label := map[speedtest.Phase]string{
		speedtest.PhaseDownload: "Download",
		speedtest.PhaseUpload:   "Upload  ",
	}
	return func(p speedtest.Progress) {
		if !animate {
			return
		}
		if p.Phase != current {
			if current != "" {
				fmt.Fprint(w, "\n")
			}
			current = p.Phase
			peak = 0
		}
		if p.Mbps > peak {
			peak = p.Mbps
		}
		spin := spinnerFrames[frame%len(spinnerFrames)]
		frame++
		bar := renderBar(p.Mbps, peak, liveBarWidth)
		name := label[p.Phase]
		if name == "" {
			name = string(p.Phase)
		}
		fmt.Fprintf(w, "\r%s  %s  %s%s%s  %.1f %s   ",
			st.Cyan(spin), st.Cyan(name),
			st.Dim("▕"), st.Green(bar), st.Dim("▏"),
			p.Mbps, st.Dim("Mbps"))
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
		fmt.Fprintf(w, "%s  %s  %s\n", st.Cyan(label), st.Dim("▕"+strings.Repeat(" ", barWidth)+"▏"), st.Dim("—"))
		return
	}
	bar := renderBar(mbps, scale, barWidth)
	fmt.Fprintf(w, "%s  %s%s%s  %.1f %s\n",
		st.Cyan(label), st.Dim("▕"), st.Green(bar), st.Dim("▏"), mbps, st.Dim("Mbps"))
}

func fmtMs(d time.Duration) string {
	return fmt.Sprintf("%.1f ms", float64(d.Microseconds())/1000)
}
