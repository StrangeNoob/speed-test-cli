package history

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"time"

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
