package cmd

import (
	"bytes"
	"io"
	"strings"
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
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"--download-only", "--upload-only"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when both --download-only and --upload-only are set")
	}
}

func TestNoColorFlagParses(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"--no-color", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--no-color should be a valid flag, got: %v", err)
	}
}

func TestVersionFlag(t *testing.T) {
	cmd := newRootCmd("1.2.3 (commit abc, built 2026-06-07)", "v1.2.3")
	cmd.SetArgs([]string{"--version"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--version should not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "speed-test 1.2.3") || !strings.Contains(out, "commit abc") {
		t.Errorf("unexpected --version output: %q", out)
	}
}

func TestNoUpdateCheckFlagParses(t *testing.T) {
	cmd := newRootCmd("test", "v0.1.0")
	cmd.SetArgs([]string{"--no-update-check", "--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--no-update-check should be a valid flag, got: %v", err)
	}
}

func TestBuildVersionFormatsComponents(t *testing.T) {
	got := buildVersion("1.2.3", "deadbeef", "2026-06-07T00:00:00Z")
	want := "1.2.3 (commit deadbeef, built 2026-06-07T00:00:00Z)"
	if got != want {
		t.Errorf("buildVersion = %q, want %q", got, want)
	}
}

func TestBuildVersionBarePlainBuild(t *testing.T) {
	// With the sentinel defaults and no build info override, the version is bare.
	if got := buildVersion("9.9.9", "none", "unknown"); got != "9.9.9" {
		t.Errorf("buildVersion with default commit/date = %q, want 9.9.9", got)
	}
}
