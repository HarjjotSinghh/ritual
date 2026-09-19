package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/HarjjotSinghh/ritual/internal/eval"
	"github.com/HarjjotSinghh/ritual/internal/ingest"
)

func newEvalCmd() *cobra.Command {
	var (
		days     int
		only     string
		minSess  int
		jsonOut  bool
		matchMin float64
	)
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Check whether ritual would have found the skills you wrote by hand",
		Long: strings.TrimSpace(`
Uses your own installed skills as an answer key.

For each skill, ritual is re-run over only the sessions that predate it and
asked what it would have proposed. If the workflow shows up in the ranking
before the skill existed, ritual would have saved you the trouble of noticing
it yourself.

This is the honest test of the tool. It is also the harder one: the skills were
written by someone with full context, and ritual only sees transcripts.
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			opts := eval.DefaultOptions()
			opts.Only = only
			if minSess > 0 {
				opts.MinSessions = minSess
			}
			if matchMin > 0 {
				opts.MatchThreshold = matchMin
			}

			fmt.Fprintf(w, "%s\n", Dim("reading history…"))
			scan, err := ingest.Scan(ingest.Options{Since: sinceFrom(days), Limits: ingest.DefaultLimits()})
			if err != nil {
				return err
			}

			roots := make([]string, 0, 8)
			truths, err := eval.LoadTruth(roots)
			if err != nil {
				return err
			}
			if len(truths) == 0 {
				return fmt.Errorf("no installed skills to use as ground truth — write one first, or run %s", Bold("ritual inventory"))
			}

			opts.Progress = func(name string, i, total int) {
				fmt.Fprintf(w, "\r\033[K%s", Dim(fmt.Sprintf("  evaluating %d/%d: %s", i+1, total, name)))
			}
			started := time.Now()
			res, err := eval.Run(scan, truths, opts)
			if err != nil {
				return err
			}
			fmt.Fprint(w, "\r\033[K")

			if jsonOut {
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(res)
			}

			fmt.Fprintf(w, "\n%s\n", Bold("Benchmark"))
			fmt.Fprintf(w, "  %d skills evaluated, %d skipped for want of history, in %s\n\n",
				res.Evaluated, res.Skipped, time.Since(started).Round(time.Second))

			for _, cutoff := range opts.Cutoffs {
				recall := res.RecallAt[cutoff]
				fmt.Fprintf(w, "  recall@%-3d %s %.0f%%\n", cutoff, Bar(recall, 24), recall*100)
			}
			fmt.Fprintf(w, "  %-10s %s %.2f\n", "MRR", Bar(res.MRR, 24), res.MRR)

			fmt.Fprintf(w, "\n%s\n", Bold("Per skill"))
			for _, o := range res.Outcomes {
				switch {
				case o.Note != "" && o.Rank == 0 && o.Sessions < opts.MinSessions:
					fmt.Fprintf(w, "  %s %-34s %s\n", Dim("·"), Truncate(o.Truth.Name, 34), Dim(o.Note))
				case o.Rank == 0:
					fmt.Fprintf(w, "  %s %-34s %s\n", Red("✗"), Truncate(o.Truth.Name, 34),
						Dim(fmt.Sprintf("not surfaced (%d sessions of history)", o.Sessions)))
				default:
					fmt.Fprintf(w, "  %s %-34s %s\n", Green("✓"), Truncate(o.Truth.Name, 34),
						Dim(fmt.Sprintf("rank %d, %.0f%% match — %s", o.Rank, o.Similarity*100, Truncate(o.MatchTitle, 40))))
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 0, "how far back to read, in days (0 for everything)")
	cmd.Flags().StringVar(&only, "only", "", "evaluate only skills whose name contains this text")
	cmd.Flags().IntVar(&minSess, "min-sessions", 0, "history a skill needs before a miss counts")
	cmd.Flags().Float64Var(&matchMin, "match", 0, "vocabulary overlap at which a finding counts as the same workflow")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit the benchmark as JSON")
	return cmd
}
