package history

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// relativeRe matches relative durations like "7d", "24h", "30m".
var relativeRe = regexp.MustCompile(`^(\d+)(d|h|m)$`)

// ParseBound parses a date-range bound. An empty string returns the zero time
// (unbounded). Relative durations (7d/24h/30m) return now minus that duration.
// "YYYY-MM-DD HH:MM" returns that exact local time. "YYYY-MM-DD" returns the
// start of that day for a lower bound (end=false) or the end of that day
// (end=true), so a bare --until date includes the whole day. Local time is used.
func ParseBound(s string, end bool, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if m := relativeRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		var unit time.Duration
		switch m[2] {
		case "d":
			unit = 24 * time.Hour
		case "h":
			unit = time.Hour
		case "m":
			unit = time.Minute
		}
		return now.Add(-time.Duration(n) * unit), nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		if end {
			return t.Add(24*time.Hour - time.Nanosecond), nil
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD, \"YYYY-MM-DD HH:MM\", or a duration like 7d/24h/30m", s)
}

// Filter returns the records whose Timestamp falls within [since, until],
// inclusive. A zero since or until means unbounded on that side. The result is
// a new slice preserving the original order.
func Filter(records []speedtest.Result, since, until time.Time) []speedtest.Result {
	out := make([]speedtest.Result, 0, len(records))
	for _, r := range records {
		if !since.IsZero() && r.Timestamp.Before(since) {
			continue
		}
		if !until.IsZero() && r.Timestamp.After(until) {
			continue
		}
		out = append(out, r)
	}
	return out
}
