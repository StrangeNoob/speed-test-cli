package cmd

import (
	"io"
	"testing"
	"time"
)

func TestBuildConfigDefaults(t *testing.T) {
	o := options{streams: 6, duration: 12 * time.Second}
	cfg := o.toConfig()
	if cfg.Streams != 6 {
		t.Errorf("Streams = %d, want 6", cfg.Streams)
	}
	if cfg.Duration != 12*time.Second {
		t.Errorf("Duration = %v, want 12s", cfg.Duration)
	}
}

func TestBuildConfigOnlyFlags(t *testing.T) {
	o := options{streams: 4, duration: time.Second, downloadOnly: true}
	cfg := o.toConfig()
	if !cfg.DownloadOnly {
		t.Errorf("DownloadOnly not propagated")
	}
}

func TestMutuallyExclusiveOnlyFlags(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--download-only", "--upload-only"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when both --download-only and --upload-only are set")
	}
}

func TestNoColorFlagParses(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--no-color", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--no-color should be a valid flag, got: %v", err)
	}
}
