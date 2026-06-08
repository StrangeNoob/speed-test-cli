package history

import (
	"fmt"
	"io"

	"github.com/StrangeNoob/speed-test-cli/internal/output"
)

// PlanInfo carries optional ISP-plan targets for the comparison output.
type PlanInfo struct {
	Set      bool
	Download float64
	Upload   float64
}

// RenderCompare writes the human-readable comparison to w.
func RenderCompare(w io.Writer, c Comparison, plan PlanInfo, st *output.Styler) {
	fmt.Fprintf(w, "%s\n\n", st.Bold("speed-test  •  Compare"))

	fmt.Fprintln(w, st.Cyan("Current test"))
	fmt.Fprintf(w, "  %-9s %7.1f %s\n", "Download", c.Current.DownloadMbps, st.Dim("Mbps"))
	fmt.Fprintf(w, "  %-9s %7.1f %s\n", "Upload", c.Current.UploadMbps, st.Dim("Mbps"))
	fmt.Fprintf(w, "  %-9s %7.1f %s\n", "Ping", msOf(c.Current.Latency), st.Dim("ms"))
	fmt.Fprintf(w, "  %-9s %7.1f %s\n\n", "Jitter", msOf(c.Current.Jitter), st.Dim("ms"))

	if c.HasBaseline {
		fmt.Fprintln(w, st.Cyan(fmt.Sprintf("Compared with last %d tests", c.SampleSize)))
		renderMetricLine(w, st, "Download", c.Download, true)
		renderMetricLine(w, st, "Upload", c.Upload, true)
		renderMetricLine(w, st, "Ping", c.Ping, false)
		renderMetricLine(w, st, "Jitter", c.Jitter, false)
		fmt.Fprintln(w)
	} else {
		fmt.Fprintf(w, "%s\n\n", st.Dim(c.Summary))
	}

	if plan.Set {
		fmt.Fprintln(w, st.Cyan("Plan performance"))
		if plan.Download > 0 {
			fmt.Fprintf(w, "  %-9s %7.1f / %.0f Mbps   %.1f%%\n", "Download", c.Current.DownloadMbps, plan.Download, c.Current.DownloadMbps/plan.Download*100)
		}
		if plan.Upload > 0 {
			fmt.Fprintf(w, "  %-9s %7.1f / %.0f Mbps   %.1f%%\n", "Upload", c.Current.UploadMbps, plan.Upload, c.Current.UploadMbps/plan.Upload*100)
		}
		fmt.Fprintln(w)
	}

	if c.HasBaseline {
		fmt.Fprintln(w, st.Cyan("Verdict"))
		fmt.Fprintf(w, "  %s\n", c.Summary)
	}
}

// renderMetricLine prints "Label  +X.X% phrase", colored by status.
func renderMetricLine(w io.Writer, st *output.Styler, label string, mc MetricCompare, higherBetter bool) {
	if !mc.Defined {
		fmt.Fprintf(w, "  %-9s %s\n", label, st.Dim("—"))
		return
	}
	delta := fmt.Sprintf("%+.1f%%", mc.DeltaPct)
	var colored, phrase string
	switch mc.Status {
	case StatusBetter:
		colored = st.Green(delta)
		if higherBetter {
			phrase = "better than usual"
		} else {
			phrase = "faster than usual"
		}
	case StatusWorse:
		colored = st.Red(delta)
		if higherBetter {
			phrase = "worse than usual"
		} else {
			phrase = "slower than usual"
		}
	default:
		colored = st.Dim(delta)
		phrase = "normal"
	}
	fmt.Fprintf(w, "  %-9s %8s  %s\n", label, colored, st.Dim(phrase))
}
