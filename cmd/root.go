package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"speed-test-cli/internal/history"
	"speed-test-cli/internal/output"
	"speed-test-cli/internal/speedtest"
)

type options struct {
	json         bool
	noLog        bool
	streams      int
	duration     time.Duration
	logFile      string
	downloadOnly bool
	uploadOnly   bool
}

func (o options) toConfig() speedtest.Config {
	return speedtest.Config{
		Streams:      o.streams,
		Duration:     o.duration,
		DownloadOnly: o.downloadOnly,
		UploadOnly:   o.uploadOnly,
	}
}

// Execute runs the root command.
func Execute() {
	var o options
	cmd := &cobra.Command{
		Use:   "speed-test",
		Short: "Measure internet speed against Cloudflare",
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(o)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&o.json, "json", false, "Machine-readable JSON output")
	f.BoolVar(&o.noLog, "no-log", false, "Don't append to the history file")
	f.IntVar(&o.streams, "streams", 6, "Parallel connections per direction")
	f.DurationVar(&o.duration, "duration", 12*time.Second, "Max time per direction")
	f.StringVar(&o.logFile, "log-file", "", "History file path (default ~/.speed-test/history.jsonl)")
	f.BoolVar(&o.downloadOnly, "download-only", false, "Skip upload test")
	f.BoolVar(&o.uploadOnly, "upload-only", false, "Skip download test")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(o options) error {
	client := speedtest.NewClient()

	var progress speedtest.ProgressFunc
	if !o.json {
		fmt.Fprintln(os.Stderr, "Testing… (Cloudflare)")
		progress = output.NewProgressPrinter(os.Stderr)
	}

	res, err := client.Run(o.toConfig(), progress)
	if err != nil {
		if o.json {
			enc := json.NewEncoder(os.Stderr)
			_ = enc.Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Fprintf(os.Stderr, "speed test failed: %v\n", err)
		}
		return err
	}

	if !o.json {
		fmt.Fprintln(os.Stderr)
	}

	if o.json {
		if err := output.JSON(os.Stdout, res); err != nil {
			return err
		}
	} else {
		output.Human(os.Stdout, res)
	}

	if !o.noLog {
		path := o.logFile
		if path == "" {
			p, err := history.DefaultPath()
			if err == nil {
				path = p
			}
		}
		if path != "" {
			if err := history.Append(path, res); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write history: %v\n", err)
			}
		}
	}
	return nil
}
