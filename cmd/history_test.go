package cmd

import (
	"io"
	"path/filepath"
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
