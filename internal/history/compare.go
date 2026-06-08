package history

import (
	"sort"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// Median returns the middle value of values (the mean of the two middles for an
// even count). An empty slice returns 0.
func Median(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	s := make([]float64, n)
	copy(s, values)
	sort.Float64s(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// MetricStatus classifies a metric versus its baseline.
type MetricStatus int

const (
	StatusNormal MetricStatus = iota
	StatusBetter
	StatusWorse
)

// MetricCompare is one metric's current value, baseline median, and change.
type MetricCompare struct {
	Current  float64
	Baseline float64 // median of the baseline
	DeltaPct float64 // (current-baseline)/baseline*100; valid only when Defined
	Defined  bool    // false when the baseline median is 0
	Status   MetricStatus
}

// Comparison is the full current-vs-baseline result.
type Comparison struct {
	Current     speedtest.Result
	HasBaseline bool
	SampleSize  int
	Download    MetricCompare // higher is better
	Upload      MetricCompare // higher is better
	Ping        MetricCompare // lower is better
	Jitter      MetricCompare // lower is better
	Verdict     string        // excellent|normal|degraded|unstable, optionally +_high_latency
	Summary     string        // human sentence
}

const (
	labelThreshold   = 10.0 // per-metric Better/Worse cutoff (improvement %)
	verdictThreshold = 15.0 // download Excellent/Degraded cutoff (improvement %)
)

// metricCompare builds a MetricCompare. higherBetter is true for throughput
// (download/upload) and false for latency (ping/jitter).
func metricCompare(current float64, baselineVals []float64, higherBetter bool) MetricCompare {
	med := Median(baselineVals)
	mc := MetricCompare{Current: current, Baseline: med}
	if med == 0 {
		return mc // Defined stays false
	}
	mc.Defined = true
	mc.DeltaPct = (current - med) / med * 100
	improvement := mc.DeltaPct
	if !higherBetter {
		improvement = -improvement
	}
	switch {
	case improvement >= labelThreshold:
		mc.Status = StatusBetter
	case improvement <= -labelThreshold:
		mc.Status = StatusWorse
	}
	return mc
}

// improvementOf returns the direction-corrected improvement % and whether it is defined.
func improvementOf(mc MetricCompare, higherBetter bool) (float64, bool) {
	if !mc.Defined {
		return 0, false
	}
	if higherBetter {
		return mc.DeltaPct, true
	}
	return -mc.DeltaPct, true
}

// Compare computes the comparison of current against the baseline records.
func Compare(current speedtest.Result, baseline []speedtest.Result) Comparison {
	c := Comparison{Current: current, SampleSize: len(baseline)}
	if len(baseline) == 0 {
		c.Verdict = "insufficient_history"
		c.Summary = "Not enough history to compare yet."
		return c
	}
	c.HasBaseline = true

	var dl, ul, pg, jt []float64
	for _, r := range baseline {
		dl = append(dl, r.DownloadMbps)
		ul = append(ul, r.UploadMbps)
		pg = append(pg, msOf(r.Latency))
		jt = append(jt, msOf(r.Jitter))
	}
	c.Download = metricCompare(current.DownloadMbps, dl, true)
	c.Upload = metricCompare(current.UploadMbps, ul, true)
	c.Ping = metricCompare(msOf(current.Latency), pg, false)
	c.Jitter = metricCompare(msOf(current.Jitter), jt, false)
	c.Verdict, c.Summary = verdict(c)
	return c
}

// verdict produces the overall code and human summary from a populated comparison.
func verdict(c Comparison) (code, summary string) {
	dlImpr, dlOk := improvementOf(c.Download, true)
	pgImpr, pgOk := improvementOf(c.Ping, false)
	jtImpr, jtOk := improvementOf(c.Jitter, false)

	if (pgOk && pgImpr <= -25) || (jtOk && jtImpr <= -50) {
		return "unstable", "Your latency is much higher than usual — the connection looks unstable."
	}

	switch {
	case dlOk && dlImpr >= verdictThreshold:
		code, summary = "excellent", "Your connection is performing better than usual."
	case dlOk && dlImpr <= -verdictThreshold:
		code, summary = "degraded", "Your download is slower than your recent baseline."
	default:
		code, summary = "normal", "Your connection is performing normally."
	}
	if c.Ping.Status == StatusWorse || c.Jitter.Status == StatusWorse {
		code += "_high_latency"
		summary += " Latency is higher than your recent baseline."
	}
	return code, summary
}
