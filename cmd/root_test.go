package cmd

import (
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
