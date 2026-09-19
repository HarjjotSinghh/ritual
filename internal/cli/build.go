package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/HarjjotSinghh/ritual/internal/artifact"
	"github.com/HarjjotSinghh/ritual/internal/authoring"
	"github.com/HarjjotSinghh/ritual/internal/classify"
	"github.com/HarjjotSinghh/ritual/internal/report"
	"github.com/HarjjotSinghh/ritual/internal/store"
)

func newBuildCmd() *cobra.Command {
	var (
		outDir   string
		author   string
		stdout   bool
		noAuthor bool
	)
	cmd := &cobra.Command{
		Use:   "build <id>...",
		Short: "Generate the artifact files for one or more findings",
		Long: strings.TrimSpace(`
Writes the files a finding produces — a SKILL.md, a command, a rules block —
into ~/.ritual/out so you can read them before anything is installed.

If an agent CLI is available, ritual will ask it to improve the prose. The
steps, counts, and dates still come from the transcripts; the model only
rewrites the wording, and its output is discarded if it changes anything else.
`),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, _, err := loadReport()
			if err != nil {
				return err
			}
			cfg, _, _ := store.LoadConfig()
			if author == "" {
				author = cfg.Author
			}
			if noAuthor {
				author = "none"
			}

			if outDir == "" {
				outDir, err = store.OutDir()
				if err != nil {
					return err
				}
			}

			w := cmd.OutOrStdout()
			for _, ref := range args {
				bundle, err := bundleFor(rep, ref)
				if err != nil {
					return err
				}
				bundle = improve(cmd.Context(), w, bundle, rep, ref, author)

				if stdout {
					for _, f := range bundle.Files {
						fmt.Fprintf(w, "%s %s %s\n", Dim("─── "), Bold(f.Path), Dim(strings.Repeat("─", max(0, 60-len(f.Path)))))
						fmt.Fprintln(w, f.Content)
					}
					continue
				}

				root := filepath.Join(outDir, bundle.Slug)
				if err := os.MkdirAll(root, 0o700); err != nil {
					return err
				}
				for _, f := range bundle.Files {
					dest := filepath.Join(root, f.Path)
					if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
						return err
					}
					if err := os.WriteFile(dest, []byte(f.Content), 0o600); err != nil {
						return err
					}
					fmt.Fprintf(w, "%s %s\n", Green("wrote"), dest)
				}
				for _, n := range bundle.Notes {
					fmt.Fprintf(w, "  %s %s\n", Dim("note:"), wrapBody(n, 74, ""))
				}
			}
			if !stdout {
				fmt.Fprintf(w, "\n%s %s\n", Dim("install with"), Cyan("ritual install "+strings.Join(args, " ")))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&outDir, "out", "", "directory to write into (default ~/.ritual/out)")
	cmd.Flags().StringVar(&author, "author", "", "agent CLI used to improve the prose: auto, none, claude, codex, opencode, gemini, qwen, ollama")
	cmd.Flags().BoolVar(&noAuthor, "no-author", false, "never call an agent CLI; use the deterministic template")
	cmd.Flags().BoolVar(&stdout, "stdout", false, "print the files instead of writing them")
	return cmd
}

// bundleFor resolves an id to a rendered bundle, accepting both workflow
// findings and rule findings so the operator does not have to remember which
// kind of id they copied.
func bundleFor(rep *report.Report, ref string) (artifact.Bundle, error) {
	if f, ok := rep.Find(ref); ok {
		if f.Decision.Kind == classify.KindIgnore {
			return artifact.Bundle{}, fmt.Errorf("finding %s was classified as not worth an artifact: %s", ref, f.Decision.Rationale)
		}
		return artifact.Build(f), nil
	}
	if r, ok := rep.FindRule(ref); ok {
		if r.Decision.Kind == classify.KindIgnore {
			return artifact.Bundle{}, fmt.Errorf("rule %s is already covered: %s", ref, r.Decision.Rationale)
		}
		return artifact.BuildRule(r), nil
	}
	return artifact.Bundle{}, fmt.Errorf("no finding with id %q — run %s to see the list", ref, Bold("ritual list"))
}

// improve runs the optional authoring pass. A failure here is reported and
// stepped over: the template artifact is already complete, and losing it
// because a model call timed out would be the worse outcome.
func improve(ctx context.Context, w io.Writer, b artifact.Bundle, rep *report.Report, ref, authorKey string) artifact.Bundle {
	a, ok := authoring.Resolve(authorKey)
	if !ok {
		return b
	}
	f, found := rep.Find(ref)
	if !found {
		return b
	}
	fmt.Fprintf(w, "%s\n", Dim("  asking "+a.DisplayName+" to tighten the prose…"))
	improved, err := authoring.Improve(ctx, a, b, f, authoring.DefaultOptions())
	if err != nil {
		fmt.Fprintf(w, "  %s %s\n", Yellow("authoring skipped:"), err.Error())
		return b
	}
	return improved
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
