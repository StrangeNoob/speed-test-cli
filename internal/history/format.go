package history

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/StrangeNoob/speed-test-cli/internal/output"
	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// CSV writes records as CSV with a header row. Empty input writes just the header.
func CSV(w io.Writer, records []speedtest.Result) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"timestamp", "server_colo", "download_mbps", "upload_mbps", "ping_ms", "jitter_ms"}); err != nil {
		return err
	}
	for _, r := range records {
		if err := cw.Write([]string{
			r.Timestamp.Format(time.RFC3339),
			r.ServerColo,
			strconv.FormatFloat(r.DownloadMbps, 'f', -1, 64),
			strconv.FormatFloat(r.UploadMbps, 'f', -1, 64),
			strconv.FormatFloat(msOf(r.Latency), 'f', -1, 64),
			strconv.FormatFloat(msOf(r.Jitter), 'f', -1, 64),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// JSON writes records as a 2-space-indented JSON array. Empty input writes [].
func JSON(w io.Writer, records []speedtest.Result) error {
	if records == nil {
		records = []speedtest.Result{}
	}
	b, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// Table renders records newest-first as an aligned table. total is the full
// record count before any --last window, used for the footer. The header is
// colored and the footer dimmed via st; a disabled styler emits plain text.
func Table(w io.Writer, records []speedtest.Result, total int, st *output.Styler) {
	noun := "speed tests"
	if len(records) == 1 {
		noun = "speed test"
	}
	fmt.Fprintf(w, "%s\n\n", st.Bold(fmt.Sprintf("Last %d %s", len(records), noun)))
	header := fmt.Sprintf("%-17s  %12s  %12s  %8s  %8s", "Date/Time", "Download", "Upload", "Ping", "Jitter")
	fmt.Fprintln(w, st.Cyan(header))
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		fmt.Fprintf(w, "%-17s  %7.1f Mbps  %7.1f Mbps  %5.0f ms  %5.0f ms\n",
			r.Timestamp.Local().Format("02 Jan 2006 15:04"),
			r.DownloadMbps, r.UploadMbps, msOf(r.Latency), msOf(r.Jitter))
	}
	fmt.Fprintf(w, "\n%s\n", st.Dim(fmt.Sprintf("showing %d of %d", len(records), total)))
}

// RenderSummary renders the avg/min/max stats block from a computed Summary.
func RenderSummary(w io.Writer, s Summary, st *output.Styler) {
	dateRange := fmt.Sprintf("%s – %s", s.First.Local().Format("02 Jan"), s.Last.Local().Format("02 Jan"))
	fmt.Fprintf(w, "%s  %s\n\n",
		st.Bold("Speed Test Summary"),
		st.Dim(fmt.Sprintf("(%d runs, %s)", s.Count, dateRange)))
	fmt.Fprintln(w, st.Cyan(fmt.Sprintf("%-10s %8s %8s %8s", "", "Avg", "Min", "Max")))
	row := func(label string, m metricStats, unit string) {
		fmt.Fprintf(w, "%-10s %8.1f %8.1f %8.1f   %s\n", label, m.Avg, m.Min, m.Max, st.Dim(unit))
	}
	row("Download", s.Download, "Mbps")
	row("Upload", s.Upload, "Mbps")
	row("Ping", s.Ping, "ms")
	row("Jitter", s.Jitter, "ms")
}
