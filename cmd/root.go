package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"

	"github.com/StrangeNoob/speed-test-cli/internal/history"
	"github.com/StrangeNoob/speed-test-cli/internal/output"
	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

type options struct {
	json         bool
	noLog        bool
	streams      int
	duration     time.Duration
	logFile      string
	downloadOnly bool
	uploadOnly   bool
	noColor      bool
}

func (o options) toConfig() speedtest.Config {
	return speedtest.Config{
		Streams:      o.streams,
		Duration:     o.duration,
		DownloadOnly: o.downloadOnly,
		UploadOnly:   o.uploadOnly,
	}
}

func newRootCmd(version string) *cobra.Command {
	var o options
	cmd := &cobra.Command{
		Use:     "speed-test",
		Short:   "Measure internet speed against Cloudflare",
		Version: version,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(o)
		},
	}
	cmd.SetVersionTemplate("speed-test {{.Version}}\n")
	f := cmd.Flags()
	f.BoolVar(&o.json, "json", false, "Machine-readable JSON output")
	f.BoolVar(&o.noLog, "no-log", false, "Don't append to the history file")
	f.IntVar(&o.streams, "streams", 6, "Parallel connections per direction")
	f.DurationVar(&o.duration, "duration", 12*time.Second, "Max time per direction")
	f.StringVar(&o.logFile, "log-file", "", "History file path (default ~/.speed-test/history.jsonl)")
	f.BoolVar(&o.downloadOnly, "download-only", false, "Skip upload test")
	f.BoolVar(&o.uploadOnly, "upload-only", false, "Skip download test")
	f.BoolVar(&o.noColor, "no-color", false, "Disable colored output")
	cmd.MarkFlagsMutuallyExclusive("download-only", "upload-only")
	return cmd
}

// Execute runs the root command. version/commit/date are injected at build time
// (via -ldflags by GoReleaser); pass the defaults otherwise.
func Execute(version, commit, date string) {
	if err := newRootCmd(buildVersion(version, commit, date)).Execute(); err != nil {
		os.Exit(1)
	}
}

// buildVersion assembles the string shown by `--version`. When no version was
// stamped in (a plain `go build`), it falls back to the module version recorded
// in the binary's build info, so `go install ...@vX.Y.Z` still reports correctly.
func buildVersion(version, commit, date string) string {
	v := version
	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if mv := info.Main.Version; mv != "" && mv != "(devel)" {
				v = mv
			}
		}
	}
	if commit != "none" || date != "unknown" {
		return fmt.Sprintf("%s (commit %s, built %s)", v, commit, date)
	}
	return v
}

func run(o options) error {
	client := speedtest.NewClient()

	noColorEnv := os.Getenv("NO_COLOR")
	animate := output.ShouldColor(output.IsTerminal(os.Stderr), o.noColor, noColorEnv)

	var progress speedtest.ProgressFunc
	if !o.json {
		fmt.Fprintln(os.Stderr, "Testing… (Cloudflare)")
		progress = output.NewProgressPrinter(os.Stderr, animate)
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

	if o.json {
		if err := output.JSON(os.Stdout, res); err != nil {
			return err
		}
	} else {
		if animate {
			fmt.Fprintln(os.Stderr)
		}
		summarySt := output.NewStyler(output.ShouldColor(output.IsTerminal(os.Stdout), o.noColor, noColorEnv))
		output.Human(os.Stdout, res, summarySt)
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
