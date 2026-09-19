package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/HarjjotSinghh/ritual/internal/mine"
	"github.com/HarjjotSinghh/ritual/internal/report"
	"github.com/HarjjotSinghh/ritual/internal/store"
)

func newListCmd() *cobra.Command {
	var limit int
	var all bool
	var jsonOut bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"suggest", "ls"},
		Short:   "Show the findings from the last scan",
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, path, err := loadReport()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if jsonOut {
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}
			fmt.Fprintf(w, "%s\n", Dim(fmt.Sprintf("report from %s · %s", store.Age(rep.GeneratedAt), path)))
			fmt.Fprint(w, RenderReport(rep, limit, all, 0))
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 15, "how many findings to print")
	cmd.Flags().BoolVar(&all, "all", false, "include findings classified as not worth an artifact")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit the saved report as JSON")
	return cmd
}

func newShowCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show the full evidence behind one finding",
		Long: strings.TrimSpace(`
Prints everything ritual used to make a suggestion: the score breakdown, the
canonical steps with how many runs performed each, the corrections issued
during those runs, and the sessions the finding cites.

A suggestion you cannot check is a suggestion you should not act on, so this
command exists to make checking cheap.
`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, _, err := loadReport()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()

			if f, ok := rep.Find(args[0]); ok {
				if jsonOut {
					enc := json.NewEncoder(w)
					enc.SetIndent("", "  ")
					return enc.Encode(f)
				}
				fmt.Fprint(w, renderFinding(f))
				return nil
			}
			if r, ok := rep.FindRule(args[0]); ok {
				if jsonOut {
					enc := json.NewEncoder(w)
					enc.SetIndent("", "  ")
					return enc.Encode(r)
				}
				fmt.Fprint(w, renderRule(r))
				return nil
			}
			return fmt.Errorf("no finding with id %q — run %s to see the list", args[0], Bold("ritual list"))
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit the finding as JSON")
	return cmd
}

func renderFinding(f report.Finding) string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n%s  %s\n", Bold(f.Title), Dim("("+f.ID+")"))
	fmt.Fprintf(&b, "%s\n\n", f.Summary)

	fmt.Fprintf(&b, "%s %s   %s %s   %s %.0f/100\n",
		Dim("verdict"), KindBadge(string(f.Decision.Kind)),
		Dim("confidence"), fmt.Sprintf("%.0f%%", f.Decision.Confidence*100),
		Dim("score"), f.Score.Total)
	fmt.Fprintf(&b, "%s\n", wrap(f.Decision.Rationale, 78, "  "))

	fmt.Fprintf(&b, "\n%s\n", Bold("Recurrence"))
	fmt.Fprintf(&b, "  %d runs across %d sessions, %d separate days (%s)\n",
		f.Occurrences, f.Sessions, f.Cadence.Days, f.Cadence.Label)
	if !f.Cadence.FirstSeen.IsZero() {
		fmt.Fprintf(&b, "  %s → %s, spanning %d days\n",
			f.Cadence.FirstSeen.Format("2006-01-02"), f.Cadence.LastSeen.Format("2006-01-02"), f.Cadence.SpanDays)
	}
	if f.Cadence.MedianIntervalHours > 0 {
		fmt.Fprintf(&b, "  median gap %.0f hours, regularity %.0f%%\n",
			f.Cadence.MedianIntervalHours, f.Cadence.Regularity*100)
	}
	if len(f.Repos) > 0 {
		fmt.Fprintf(&b, "  repositories: %s\n", strings.Join(f.Repos, ", "))
	}
	fmt.Fprintf(&b, "  agents: %s\n", strings.Join(f.Agents, ", "))

	if len(f.Steps) > 0 {
		fmt.Fprintf(&b, "\n%s\n", Bold("Canonical sequence"))
		for i, s := range f.Steps {
			fmt.Fprintf(&b, "  %2d. %-46s %s %s\n",
				i+1, Truncate(mine.DescribeStep(s.Action), 46),
				Bar(s.Support, 10), Dim(fmt.Sprintf("%d/%d runs", s.Count, f.Occurrences)))
		}
	}

	if len(f.Commands) > 0 {
		fmt.Fprintf(&b, "\n%s\n  %s\n", Bold("Commands"), strings.Join(f.Commands, ", "))
	}

	if len(f.Corrections) > 0 {
		fmt.Fprintf(&b, "\n%s\n", Bold("Corrections issued during these runs"))
		for _, c := range f.Corrections {
			fmt.Fprintf(&b, "  %s %s\n", Dim("·"), Truncate(oneLine(c.Text), 86))
		}
	}

	fmt.Fprintf(&b, "\n%s\n", Bold("Score breakdown"))
	b.WriteString(f.Score.Describe())
	for _, p := range f.Score.Penalties {
		fmt.Fprintf(&b, "  %s %s — %s\n", Red("penalty"), p.Name, p.Reason)
	}

	fmt.Fprintf(&b, "\n%s\n", Bold("Evidence"))
	for _, e := range f.Evidence {
		when := "undated"
		if !e.At.IsZero() {
			when = e.At.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(&b, "  %s %s %s\n", Dim(when), Cyan(fmt.Sprintf("%-8s", e.Agent)), Truncate(oneLine(e.Intent), 70))
		fmt.Fprintf(&b, "  %s\n", Dim("    "+e.Source))
	}

	fmt.Fprintf(&b, "\n%s\n", Bold("Next"))
	fmt.Fprintf(&b, "  %s\n", Cyan("ritual build "+f.ID))
	fmt.Fprintf(&b, "  %s\n", Cyan("ritual install "+f.ID))
	return b.String()
}

func renderRule(r report.RuleFinding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s  %s\n", Bold("Standing preference"), Dim("("+r.ID+")"))
	fmt.Fprintf(&b, "  %s\n\n", wrapBody(r.Text, 76, "  "))
	fmt.Fprintf(&b, "%s %s   %s\n", Dim("verdict"), KindBadge(string(r.Decision.Kind)), Dim(fmt.Sprintf("stated %d times in %d sessions", r.Occurrences, r.Sessions)))
	fmt.Fprintf(&b, "%s\n", wrap(r.Decision.Rationale, 78, "  "))
	if len(r.Repos) > 0 {
		fmt.Fprintf(&b, "\n  repositories: %s\n", strings.Join(r.Repos, ", "))
	}
	fmt.Fprintf(&b, "  agents: %s\n", strings.Join(r.Agents, ", "))

	fmt.Fprintf(&b, "\n%s\n", Bold("Every time it was said"))
	for _, e := range r.Examples {
		when := "undated"
		if !e.At.IsZero() {
			when = e.At.Format("2006-01-02")
		}
		fmt.Fprintf(&b, "  %s %s\n", Dim(when), Truncate(oneLine(e.Text), 80))
	}
	fmt.Fprintf(&b, "\n%s\n  %s\n", Bold("Next"), Cyan("ritual rules install "+r.ID))
	return b.String()
}

// wrap renders a paragraph at a fixed width with a hanging indent, because a
// rationale that runs off the terminal is a rationale nobody reads.
func wrap(text string, width int, indent string) string {
	return wrapBody(text, width-len(indent), indent)
}

func wrapBody(text string, width int, indent string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	line := indent
	for _, w := range words {
		if len(line)+len(w)+1 > width+len(indent) && strings.TrimSpace(line) != "" {
			b.WriteString(strings.TrimRight(line, " ") + "\n")
			line = indent
		}
		line += w + " "
	}
	b.WriteString(strings.TrimRight(line, " "))
	return b.String()
}
