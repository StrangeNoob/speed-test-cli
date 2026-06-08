package history

import (
	"testing"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func TestParseBoundEmpty(t *testing.T) {
	got, err := ParseBound("", false, time.Now())
	if err != nil || !got.IsZero() {
		t.Errorf("empty = (%v,%v), want (zero,nil)", got, err)
	}
}

func TestParseBoundRelative(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.Local)
	cases := map[string]time.Duration{"7d": 7 * 24 * time.Hour, "24h": 24 * time.Hour, "30m": 30 * time.Minute}
	for in, want := range cases {
		got, err := ParseBound(in, false, now)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if !got.Equal(now.Add(-want)) {
			t.Errorf("%s = %v, want %v", in, got, now.Add(-want))
		}
	}
}

func TestParseBoundDate(t *testing.T) {
	now := time.Now()
	start, err := ParseBound("2026-06-01", false, now)
	if err != nil || !start.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)) {
		t.Errorf("since date = %v (%v), want start of day", start, err)
	}
	end, err := ParseBound("2026-06-01", true, now)
	wantEnd := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local).Add(24*time.Hour - time.Nanosecond)
	if err != nil || !end.Equal(wantEnd) {
		t.Errorf("until date = %v (%v), want end of day", end, err)
	}
}

func TestParseBoundDateTime(t *testing.T) {
	now := time.Now()
	want := time.Date(2026, 6, 1, 15, 30, 0, 0, time.Local)
	for _, end := range []bool{false, true} {
		got, err := ParseBound("2026-06-01 15:30", end, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("datetime end=%v = %v (%v), want %v", end, got, err, want)
		}
	}
}

func TestParseBoundInvalid(t *testing.T) {
	for _, s := range []string{"nonsense", "5x", "2026-13-99"} {
		if _, err := ParseBound(s, false, time.Now()); err == nil {
			t.Errorf("%q should error", s)
		}
	}
}

func TestFilter(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2026, 6, day, 12, 0, 0, 0, time.UTC) }
	recs := []speedtest.Result{{Timestamp: d(1)}, {Timestamp: d(5)}, {Timestamp: d(9)}}

	if got := Filter(recs, time.Time{}, time.Time{}); len(got) != 3 {
		t.Errorf("no bounds = %d, want 3", len(got))
	}
	if got := Filter(recs, d(5), time.Time{}); len(got) != 2 {
		t.Errorf("since=5 = %d, want 2 (5,9)", len(got))
	}
	if got := Filter(recs, time.Time{}, d(5)); len(got) != 2 {
		t.Errorf("until=5 = %d, want 2 (1,5)", len(got))
	}
	if got := Filter(recs, d(2), d(6)); len(got) != 1 || !got[0].Timestamp.Equal(d(5)) {
		t.Errorf("[2,6] = %+v, want only day 5", got)
	}
	if got := Filter(recs, d(5), d(5)); len(got) != 1 {
		t.Errorf("[5,5] = %d, want 1 (inclusive)", len(got))
	}
	if got := Filter(recs, d(9), d(1)); len(got) != 0 {
		t.Errorf("since>until = %d, want 0", len(got))
	}
}
