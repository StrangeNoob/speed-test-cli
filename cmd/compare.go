package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/StrangeNoob/speed-test-cli/internal/history"
	"github.com/StrangeNoob/speed-test-cli/internal/output"
	"github.com/StrangeNoob/speed-test-cli/internal/speedtest"
)

type compareOptions struct {
	last         int
	window       string
	latest       bool
	planDownload float64
	planUpload   float64
	json         bool
	noLog        bool
	noColor      bool
	logFile      string
}

func newCompareCmd() *cobra.Command {
	var o compareOptions
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare a speed test against your past results",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCompare(o)
		},
	}
	f := cmd.Flags()
	f.IntVar(&o.last, "last", 10, "Baseline = median of the last N runs")
	f.StringVar(&o.window, "window", "", "Baseline = runs within this window (7d/24h/30m, or a date)")
	f.BoolVar(&o.latest, "latest", false, "Compare the latest saved run instead of running a new test")
	f.Float64Var(&o.planDownload, "plan-download", 0, "Show performance vs an ISP download plan (Mbps)")
	f.Float64Var(&o.planUpload, "plan-upload", 0, "Show performance vs an ISP upload plan (Mbps)")
	f.BoolVar(&o.json, "json", false, "Machine-readable JSON output")
	f.BoolVar(&o.noLog, "no-log", false, "Don't append the fresh test to history")
	f.BoolVar(&o.noColor, "no-color", false, "Disable colored output")
	f.StringVar(&o.logFile, "log-file", "", "History file (default ~/.speed-test/history.jsonl)")
	cmd.MarkFlagsMutuallyExclusive("last", "window")
	return cmd
}

func runCompare(o compareOptions) error {
	path := o.logFile
	if path == "" {
		p, err := history.DefaultPath()
		if err != nil {
			return err
		}
		path = p
	}
	past, _, err := history.Load(path)
	if err != nil {
		return err
	}

	var current speedtest.Result
	cand := past
	if o.latest {
		if len(past) == 0 {
			fmt.Fprintln(os.Stderr, "No saved speed tests to compare. Run 'speed-test' first.")
			return nil
		}
		current = past[len(past)-1]
		cand = past[:len(past)-1]
	} else {
		res, err := runCompareTest(o)
		if err != nil {
			if o.json {
				_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
			} else {
				fmt.Fprintf(os.Stderr, "speed test failed: %v\n", err)
			}
			return err
		}
		current = res
		if !o.noLog {
			if err := history.Append(path, current); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write history: %v\n", err)
			}
		}
	}

	var baseline []speedtest.Result
	if o.window != "" {
		since, err := history.ParseBound(o.window, false, time.Now())
		if err != nil {
			return err
		}
		baseline = history.Filter(cand, since, time.Time{})
	} else {
		baseline = history.LastN(cand, o.last)
	}

	c := history.Compare(current, baseline)
	plan := history.PlanInfo{Set: o.planDownload > 0 || o.planUpload > 0, Download: o.planDownload, Upload: o.planUpload}

	if o.json {
		return history.RenderCompareJSON(os.Stdout, c, plan)
	}
	st := output.NewStyler(output.ShouldColor(output.IsTerminal(os.Stdout), o.noColor, os.Getenv("NO_COLOR")))
	history.RenderCompare(os.Stdout, c, plan, st)
	return nil
}

// runCompareTest runs a fresh speed test, showing live progress unless --json.
func runCompareTest(o compareOptions) (speedtest.Result, error) {
	client := speedtest.NewClient()
	cfg := speedtest.Config{Streams: 6, Duration: 12 * time.Second}

	animate := !o.json && output.ShouldColor(output.IsTerminal(os.Stderr), o.noColor, os.Getenv("NO_COLOR"))
	var progress speedtest.ProgressFunc
	if !o.json {
		fmt.Fprintln(os.Stderr, "Testing… (Cloudflare)")
		progress = output.NewProgressPrinter(os.Stderr, animate)
	}
	res, err := client.Run(cfg, progress)
	if animate {
		fmt.Fprintln(os.Stderr)
	}
	return res, err
}
