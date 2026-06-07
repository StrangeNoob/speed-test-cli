package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

// Load reads the history file at path, returning the records in file order
// (oldest first) and the number of malformed lines skipped. A missing file
// returns (nil, 0, nil); a genuine read error is returned. Parsing is
// best-effort: a bad line is counted and skipped, never fatal.
func Load(path string) ([]speedtest.Result, int, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	defer f.Close()

	var records []speedtest.Result
	skipped := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r speedtest.Result
		if err := json.Unmarshal(line, &r); err != nil {
			skipped++
			continue
		}
		records = append(records, r)
	}
	if err := sc.Err(); err != nil {
		return records, skipped, err
	}
	return records, skipped, nil
}

// LastN returns the most recent n records (records are oldest-first, so the
// tail). n <= 0 or n >= len returns all records.
func LastN(records []speedtest.Result, n int) []speedtest.Result {
	if n <= 0 || n >= len(records) {
		return records
	}
	return records[len(records)-n:]
}
