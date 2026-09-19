package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/HarjjotSinghh/ritual/internal/classify"
	"github.com/HarjjotSinghh/ritual/internal/report"
	"github.com/HarjjotSinghh/ritual/internal/score"
	"github.com/HarjjotSinghh/ritual/internal/store"
)

type scanFlags struct {
	days        int
	agents      []string
	workspace   string
	minScore    float64
	threshold   float64
	minRuns     int
	minSessions int
	limit       int
	jsonOut     bool
	noSave      bool
	noInventory bool
	keepEmails  bool
	all         bool
	unprompted  bool
	quiet       bool
}

func newScanCmd() *cobra.Command {
	var f scanFlags
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Read your agent history and find the workflows you repeat",
		Long: strings.TrimSpace(`
Reads every installed agent's session store, segments the sessions into task
arcs, groups the arcs that are the same work done again, and ranks what it
finds.

The report is saved so later commands can act on it without rescanning.
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(cmd.OutOrStdout(), f)
		},
	}
	cfg, _, _ := store.LoadConfig()
	cmd.Flags().IntVar(&f.days, "days", cfg.Days, "how far back to read, in days (0 for everything)")
	cmd.Flags().StringSliceVar(&f.agents, "agents", cfg.Agents, "limit to these agents (default: every installed agent)")
	cmd.Flags().StringVar(&f.workspace, "workspace", "", "limit to sessions whose path contains this text")
	cmd.Flags().Float64Var(&f.minScore, "min-score", cfg.MinScore, "score below which a finding is marked ignore")
	cmd.Flags().Float64Var(&f.threshold, "threshold", cfg.Threshold, "clustering similarity cutoff, 0..1")
	cmd.Flags().IntVar(&f.minRuns, "min-runs", cfg.MinOccurrences, "how many runs before a workflow is proposed")
	cmd.Flags().IntVar(&f.minSessions, "min-sessions", cfg.MinSessions, "how many distinct sessions a workflow must span")
	cmd.Flags().IntVar(&f.limit, "limit", 15, "how many findings to print")
	cmd.Flags().BoolVar(&f.jsonOut, "json", false, "emit the full report as JSON")
	cmd.Flags().BoolVar(&f.noSave, "no-save", false, "do not write the report to disk")
	cmd.Flags().BoolVar(&f.noInventory, "no-inventory", false, "skip the scan of already-installed skills and rules")
	cmd.Flags().BoolVar(&f.keepEmails, "keep-emails", cfg.KeepEmails, "keep email addresses instead of redacting them")
	cmd.Flags().BoolVar(&f.unprompted, "include-automated", false, "include sessions with no human turn (scheduled runs, SDK agents)")
	cmd.Flags().BoolVar(&f.all, "all", false, "include findings classified as not worth an artifact")
	cmd.Flags().BoolVar(&f.quiet, "quiet", false, "suppress progress output")
	return cmd
}

func runScan(w io.Writer, f scanFlags) error {
	opts := report.DefaultOptions()
	opts.Ingest.Since = sinceFrom(f.days)
	opts.Ingest.Agents = f.agents
	opts.Ingest.Workspace = f.workspace
	opts.Ingest.KeepEmails = f.keepEmails
	opts.Ingest.KeepUnprompted = f.unprompted
	opts.MinScore = f.minScore
	opts.SkipInventory = f.noInventory
	if f.threshold > 0 {
		opts.Mine.Cluster.Threshold = f.threshold
	}
	if f.minRuns > 0 {
		opts.Mine.Cluster.MinOccurrences = f.minRuns
	}
	if f.minSessions > 0 {
		opts.Mine.MinSessions = f.minSessions
	}
	opts.Weights = score.DefaultWeights()

	// Progress redraws one line, which is right in a terminal and is noise in
	// a CI log or a pipe.
	showProgress := !f.quiet && !f.jsonOut && IsTerminal()
	if showProgress {
		opts.Ingest.Progress = progressPrinter(w)
	}

	started := time.Now()
	rep, err := report.Build(opts)
	if err != nil {
		return err
	}
	if showProgress {
		fmt.Fprint(w, "\r\033[K")
	}

	if !f.noSave {
		if _, err := store.SaveReport(rep); err != nil {
			return fmt.Errorf("save report: %w", err)
		}
	}

	if f.jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}

	fmt.Fprint(w, RenderReport(rep, f.limit, f.all, time.Since(started)))
	return nil
}

// progressPrinter keeps the operator informed during what can be a minute-long
// read of a year of history. It writes to a single line so a finished scan
// leaves no trail.
func progressPrinter(w io.Writer) func(string, int, int) {
	last := ""
	return func(agent string, total, done int) {
		line := fmt.Sprintf("  reading %s… %d/%d", agent, done, total)
		if line == last {
			return
		}
		last = line
		fmt.Fprintf(w, "\r\033[K%s", Dim(line))
	}
}

// RenderReport is the terminal view of a report. It is exported so `ritual
// list` renders a saved report identically to a fresh scan.
func RenderReport(rep *report.Report, limit int, includeIgnored bool, elapsed time.Duration) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n%s\n", Bold("Scanned"))
	for _, s := range rep.Stats {
		fmt.Fprintf(&b, "  %-10s %4d sessions  %6d turns  %6d tool calls  %s\n",
			s.Agent, s.Sessions, s.Turns, s.ToolCalls, Dim(rangeOf(s.Earliest, s.Latest)))
	}
	if len(rep.Stats) == 0 {
		b.WriteString(Dim("  no agent session stores found on this machine\n"))
		b.WriteString("  " + Dim("run `ritual agents` to see where ritual looked") + "\n")
		return b.String()
	}
	fmt.Fprintf(&b, "  %s\n", Dim(fmt.Sprintf("%d task arcs, %d of them part of a repeating workflow, in %s",
		rep.Arcs, rep.Clustered, elapsed.Round(time.Millisecond))))

	findings := rep.Findings
	if !includeIgnored {
		findings = rep.Actionable()
	}
	if len(findings) == 0 {
		b.WriteString("\n" + Yellow("No repeating workflows cleared the bar.") + "\n")
		b.WriteString(Dim("  Try `ritual scan --all` to see what was found and rejected,\n  or lower the bar with `--min-runs 2 --min-score 15`.\n"))
		return b.String()
	}

	fmt.Fprintf(&b, "\n%s\n", Bold(fmt.Sprintf("Found %d recurring workflows", len(findings))))
	fmt.Fprintf(&b, "%s\n", Dim("  id         score  kind      runs  days  workflow"))
	shown := findings
	if limit > 0 && len(shown) > limit {
		shown = shown[:limit]
	}
	for _, f := range shown {
		fmt.Fprintf(&b, "  %s  %s   %s %4d  %4d  %s\n",
			Cyan(f.ID), ScoreColor(f.Score.Total), KindBadge(string(f.Decision.Kind)),
			f.Occurrences, f.Cadence.Days, f.Title)
		fmt.Fprintf(&b, "  %s\n", Dim("           "+Truncate(f.Summary, 96)))
	}
	if len(findings) > len(shown) {
		fmt.Fprintf(&b, "  %s\n", Dim(fmt.Sprintf("… and %d more (use --limit 0 to see all)", len(findings)-len(shown))))
	}

	if rules := actionableRules(rep); len(rules) > 0 {
		fmt.Fprintf(&b, "\n%s\n", Bold(fmt.Sprintf("Found %d standing preferences you keep restating", len(rules))))
		for _, r := range rules {
			fmt.Fprintf(&b, "  %s  %s %s\n", Cyan(r.ID), Dim(fmt.Sprintf("x%d", r.Occurrences)), Truncate(oneLine(r.Text), 88))
		}
	}

	if len(rep.Warnings) > 0 {
		fmt.Fprintf(&b, "\n%s\n", Dim(fmt.Sprintf("%d files were skipped; run `ritual doctor` for details", len(rep.Warnings))))
	}

	b.WriteString("\n" + Bold("Next") + "\n")
	fmt.Fprintf(&b, "  %s   see the evidence behind a finding\n", Cyan("ritual show <id>"))
	fmt.Fprintf(&b, "  %s  write the artifact to ~/.ritual/out\n", Cyan("ritual build <id>"))
	fmt.Fprintf(&b, "  %s  install it into your agents\n", Cyan("ritual install <id>"))
	return b.String()
}

func actionableRules(rep *report.Report) []report.RuleFinding {
	out := make([]report.RuleFinding, 0, len(rep.Rules))
	for _, r := range rep.Rules {
		if r.Decision.Kind == classify.KindIgnore {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Occurrences > out[j].Occurrences })
	return out
}

func rangeOf(start, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return "undated"
	}
	return start.Format("2006-01-02") + " → " + end.Format("2006-01-02")
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
