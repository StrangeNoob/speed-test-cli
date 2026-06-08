package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/StrangeNoob/speed-test-cli/internal/history"
	"github.com/StrangeNoob/speed-test-cli/internal/output"
)

type historyOptions struct {
	last    int
	summary bool
	export  string
	out     string
	logFile string
	noColor bool
	since   string
	until   string
}

func newHistoryCmd() *cobra.Command {
	var o historyOptions
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show or export recorded speed-test history",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runHistory(o)
		},
	}
	f := cmd.Flags()
	f.IntVar(&o.last, "last", 20, "Show the most recent N runs (0 = all)")
	f.BoolVar(&o.summary, "summary", false, "Print an avg/min/max summary instead of the table")
	f.StringVar(&o.export, "export", "", "Export as 'csv' or 'json' instead of the table")
	f.StringVar(&o.out, "out", "", "With --export, write to this file instead of stdout")
	f.StringVar(&o.logFile, "log-file", "", "History file to read (default ~/.speed-test/history.jsonl)")
	f.BoolVar(&o.noColor, "no-color", false, "Disable colored output")
	f.StringVar(&o.since, "since", "", "Only runs at/after this time (YYYY-MM-DD, 'YYYY-MM-DD HH:MM', or 7d/24h/30m)")
	f.StringVar(&o.until, "until", "", "Only runs at/before this time (same formats; a bare date includes the whole day)")
	cmd.MarkFlagsMutuallyExclusive("summary", "export")
	return cmd
}

const emptyHistoryMsg = "No speed tests recorded yet. Run 'speed-test' to record one."
const noMatchMsg = "No speed tests match that range."

func runHistory(o historyOptions) error {
	if o.export != "" && o.export != "csv" && o.export != "json" {
		return fmt.Errorf("invalid --export %q (use csv or json)", o.export)
	}

	path := o.logFile
	if path == "" {
		p, err := history.DefaultPath()
		if err != nil {
			return err
		}
		path = p
	}

	records, skipped, err := history.Load(path)
	if err != nil {
		return err
	}
	if skipped > 0 {
		noun := "lines"
		if skipped == 1 {
			noun = "line"
		}
		fmt.Fprintf(os.Stderr, "(skipped %d unreadable %s)\n", skipped, noun)
	}

	now := time.Now()
	since, err := history.ParseBound(o.since, false, now)
	if err != nil {
		return err
	}
	until, err := history.ParseBound(o.until, true, now)
	if err != nil {
		return err
	}

	filtered := history.Filter(records, since, until)
	total := len(filtered)
	window := history.LastN(filtered, o.last)

	if o.export != "" {
		w := os.Stdout
		if o.out != "" {
			f, err := os.Create(o.out)
			if err != nil {
				return err
			}
			defer f.Close()
			w = f
		}
		if o.export == "csv" {
			return history.CSV(w, window)
		}
		return history.JSON(w, window)
	}

	styler := func() *output.Styler {
		return output.NewStyler(output.ShouldColor(output.IsTerminal(os.Stdout), o.noColor, os.Getenv("NO_COLOR")))
	}

	// Distinguish "no history at all" from "none in the chosen range".
	emptyMsg := emptyHistoryMsg
	if len(records) > 0 {
		emptyMsg = noMatchMsg
	}

	if o.summary {
		s := history.Summarize(window)
		if s.Count == 0 {
			fmt.Fprintln(os.Stderr, emptyMsg)
			return nil
		}
		history.RenderSummary(os.Stdout, s, styler())
		return nil
	}

	if len(window) == 0 {
		fmt.Fprintln(os.Stderr, emptyMsg)
		return nil
	}
	history.Table(os.Stdout, window, total, styler())
	return nil
}
