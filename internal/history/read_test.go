package history

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

func TestLoadParsesAndSkips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	content := `{"timestamp":"2026-06-07T10:00:00Z","server_colo":"MAA","latency_ns":15000000,"jitter_ns":2000000,"download_mbps":100.5,"upload_mbps":20.2}
not json
{"timestamp":"2026-06-08T10:00:00Z","server_colo":"CCU","latency_ns":21000000,"jitter_ns":4000000,"download_mbps":92.0,"upload_mbps":48.0}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	recs, skipped, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
	if recs[0].ServerColo != "MAA" || recs[1].DownloadMbps != 92.0 {
		t.Errorf("unexpected records: %+v", recs)
	}
}

func TestLoadMissingFile(t *testing.T) {
	recs, skipped, err := Load(filepath.Join(t.TempDir(), "nope.jsonl"))
	if err != nil || recs != nil || skipped != 0 {
		t.Errorf("missing file = (%v,%d,%v), want (nil,0,nil)", recs, skipped, err)
	}
}

func TestLastN(t *testing.T) {
	rs := []speedtest.Result{{ServerColo: "a"}, {ServerColo: "b"}, {ServerColo: "c"}}
	if len(LastN(rs, 0)) != 3 {
		t.Error("n=0 should return all")
	}
	got := LastN(rs, 2)
	if len(got) != 2 || got[0].ServerColo != "b" {
		t.Errorf("n=2 = %+v, want last two (b,c)", got)
	}
	if len(LastN(rs, 100)) != 3 {
		t.Error("n>len should return all")
	}
	if len(LastN(rs, 3)) != 3 {
		t.Error("n==len should return all")
	}
	if len(LastN(rs, -1)) != 3 {
		t.Error("negative n should return all")
	}
}
