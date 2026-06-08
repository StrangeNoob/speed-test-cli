package cmd

import (
	"io"
	"path/filepath"
	"testing"
)

func TestCompareHelp(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"compare", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compare --help: %v", err)
	}
}

func TestCompareLastWindowConflict(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"compare", "--latest", "--last", "5", "--window", "7d"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when --last and --window are combined")
	}
}

func TestCompareLatestProducesOutput(t *testing.T) {
	hist := writeHistory(t)
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"compare", "--latest", "--log-file", hist})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compare --latest: %v", err)
	}
}

func TestCompareLatestEmptyNoError(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"compare", "--latest", "--log-file", filepath.Join(t.TempDir(), "none.jsonl")})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("empty --latest should not error: %v", err)
	}
}

func TestCompareInvalidWindow(t *testing.T) {
	hist := writeHistory(t)
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"compare", "--latest", "--window", "bogus", "--log-file", hist})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for --window bogus")
	}
}
