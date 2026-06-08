package history

import (
	"encoding/json"
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

// RenderCompareJSON writes the comparison as a JSON object to w. baseline is
// null and delta omitted when there is no baseline; undefined metric deltas and
// plan percentages are emitted as null.
func RenderCompareJSON(w io.Writer, c Comparison, plan PlanInfo) error {
	type metricsDTO struct {
		Download float64 `json:"download_mbps"`
		Upload   float64 `json:"upload_mbps"`
		Latency  float64 `json:"latency_ms"`
		Jitter   float64 `json:"jitter_ms"`
	}
	type baselineDTO struct {
		Type       string  `json:"type"`
		SampleSize int     `json:"sample_size"`
		Download   float64 `json:"download_mbps"`
		Upload     float64 `json:"upload_mbps"`
		Latency    float64 `json:"latency_ms"`
		Jitter     float64 `json:"jitter_ms"`
	}
	type deltaDTO struct {
		Download *float64 `json:"download_percent"`
		Upload   *float64 `json:"upload_percent"`
		Latency  *float64 `json:"latency_percent"`
		Jitter   *float64 `json:"jitter_percent"`
	}
	type planDTO struct {
		Download        float64  `json:"download_mbps"`
		Upload          float64  `json:"upload_mbps"`
		DownloadPercent *float64 `json:"download_percent"`
		UploadPercent   *float64 `json:"upload_percent"`
	}
	out := struct {
		Current  metricsDTO   `json:"current"`
		Baseline *baselineDTO `json:"baseline"`
		Delta    *deltaDTO    `json:"delta,omitempty"`
		Plan     *planDTO     `json:"plan,omitempty"`
		Verdict  string       `json:"verdict"`
		Summary  string       `json:"summary"`
	}{
		Current: metricsDTO{c.Current.DownloadMbps, c.Current.UploadMbps, msOf(c.Current.Latency), msOf(c.Current.Jitter)},
		Verdict: c.Verdict,
		Summary: c.Summary,
	}
	if c.HasBaseline {
		out.Baseline = &baselineDTO{
			Type: "median", SampleSize: c.SampleSize,
			Download: c.Download.Baseline, Upload: c.Upload.Baseline,
			Latency: c.Ping.Baseline, Jitter: c.Jitter.Baseline,
		}
		out.Delta = &deltaDTO{
			Download: definedPtr(c.Download),
			Upload:   definedPtr(c.Upload),
			Latency:  definedPtr(c.Ping),
			Jitter:   definedPtr(c.Jitter),
		}
	}
	if plan.Set {
		p := &planDTO{Download: plan.Download, Upload: plan.Upload}
		if plan.Download > 0 {
			v := c.Current.DownloadMbps / plan.Download * 100
			p.DownloadPercent = &v
		}
		if plan.Upload > 0 {
			v := c.Current.UploadMbps / plan.Upload * 100
			p.UploadPercent = &v
		}
		out.Plan = p
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// definedPtr returns a pointer to the delta percent, or nil when undefined.
func definedPtr(mc MetricCompare) *float64 {
	if !mc.Defined {
		return nil
	}
	v := mc.DeltaPct
	return &v
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
