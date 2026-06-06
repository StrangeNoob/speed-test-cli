package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"speed-test-cli/internal/speedtest"
)

func TestAppendCreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "history.jsonl")

	r1 := speedtest.Result{Timestamp: time.Unix(1, 0).UTC(), DownloadMbps: 10}
	r2 := speedtest.Result{Timestamp: time.Unix(2, 0).UTC(), DownloadMbps: 20}

	if err := Append(path, r1); err != nil {
		t.Fatalf("Append 1: %v", err)
	}
	if err := Append(path, r2); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	var lines []speedtest.Result
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r speedtest.Result
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		lines = append(lines, r)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0].DownloadMbps != 10 || lines[1].DownloadMbps != 20 {
		t.Errorf("unexpected contents: %+v", lines)
	}
}
