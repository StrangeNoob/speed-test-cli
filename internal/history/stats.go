package history

import (
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// metricStats holds the average, minimum, and maximum of one metric.
type metricStats struct{ Avg, Min, Max float64 }

// Summary is the aggregate view of a set of recorded runs.
type Summary struct {
	Count    int
	First    time.Time
	Last     time.Time
	Download metricStats // Mbps
	Upload   metricStats // Mbps
	Ping     metricStats // ms
	Jitter   metricStats // ms
}

// msOf converts a duration to milliseconds as a float.
func msOf(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// acc accumulates one metric's running sum/min/max.
type acc struct {
	sum, min, max float64
	n             int
}

func (a *acc) add(v float64) {
	if a.n == 0 || v < a.min {
		a.min = v
	}
	if a.n == 0 || v > a.max {
		a.max = v
	}
	a.sum += v
	a.n++
}

func (a acc) result() metricStats {
	if a.n == 0 {
		return metricStats{}
	}
	return metricStats{Avg: a.sum / float64(a.n), Min: a.min, Max: a.max}
}

// Summarize computes the count, time range, and avg/min/max per metric. An
// empty slice yields the zero Summary (Count 0), which callers treat as "no data".
func Summarize(records []speedtest.Result) Summary {
	if len(records) == 0 {
		return Summary{}
	}
	s := Summary{Count: len(records), First: records[0].Timestamp, Last: records[0].Timestamp}
	var dl, ul, pg, jt acc
	for _, r := range records {
		if r.Timestamp.Before(s.First) {
			s.First = r.Timestamp
		}
		if r.Timestamp.After(s.Last) {
			s.Last = r.Timestamp
		}
		dl.add(r.DownloadMbps)
		ul.add(r.UploadMbps)
		pg.add(msOf(r.Latency))
		jt.add(msOf(r.Jitter))
	}
	s.Download, s.Upload, s.Ping, s.Jitter = dl.result(), ul.result(), pg.result(), jt.result()
	return s
}
