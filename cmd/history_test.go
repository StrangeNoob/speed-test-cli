package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryHelp(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("`history --help` should work: %v", err)
	}
}

func TestHistorySummaryExportMutuallyExclusive(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--summary", "--export", "csv"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --summary and --export are combined")
	}
}

func TestHistoryEmptyNoError(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", filepath.Join(t.TempDir(), "none.jsonl")})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("empty history should not error: %v", err)
	}
}

func TestHistoryInvalidExport(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--export", "xml", "--log-file", filepath.Join(t.TempDir(), "none.jsonl")})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for --export xml")
	}
}

func writeHistory(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	content := `{"timestamp":"2026-06-01T12:00:00Z","download_mbps":10}
{"timestamp":"2026-06-05T12:00:00Z","download_mbps":20}
{"timestamp":"2026-06-09T12:00:00Z","download_mbps":30}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHistorySinceFiltersExport(t *testing.T) {
	hist := writeHistory(t)
	out := filepath.Join(filepath.Dir(hist), "out.csv")
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", hist, "--since", "2026-06-05", "--export", "csv", "--out", out})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("history --since --export: %v", err)
	}
	s := string(mustRead(t, out))
	if strings.Contains(s, "2026-06-01") {
		t.Errorf("June 1 should be filtered out:\n%s", s)
	}
	if !strings.Contains(s, "2026-06-05") || !strings.Contains(s, "2026-06-09") {
		t.Errorf("June 5 and 9 should be present:\n%s", s)
	}
}

func TestHistoryUntilFiltersExport(t *testing.T) {
	hist := writeHistory(t)
	out := filepath.Join(filepath.Dir(hist), "out.csv")
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", hist, "--until", "2026-06-05", "--export", "csv", "--out", out})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := string(mustRead(t, out))
	if strings.Contains(s, "2026-06-09") {
		t.Errorf("June 9 should be filtered out by --until:\n%s", s)
	}
	if !strings.Contains(s, "2026-06-01") || !strings.Contains(s, "2026-06-05") {
		t.Errorf("June 1 and 5 should be present (until is end-of-day inclusive):\n%s", s)
	}
}

func TestHistoryInvalidSince(t *testing.T) {
	hist := writeHistory(t)
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", hist, "--since", "nonsense"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for --since nonsense")
	}
}

func TestHistorySinceNoMatchNoError(t *testing.T) {
	hist := writeHistory(t)
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", hist, "--since", "2030-01-01"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("no-match range should not error: %v", err)
	}
}

func TestHistorySinceSummary(t *testing.T) {
	hist := writeHistory(t)
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"history", "--log-file", hist, "--since", "2026-06-05", "--summary"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--since with --summary should work: %v", err)
	}
}
