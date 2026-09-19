package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/HarjjotSinghh/ritual/internal/install"
	"github.com/HarjjotSinghh/ritual/internal/store"
)

func newInstallCmd() *cobra.Command {
	var (
		targets []string
		dryRun  bool
		force   bool
		project bool
		yes     bool
		author  string
	)
	cmd := &cobra.Command{
		Use:   "install <id>...",
		Short: "Install a finding's artifact into your agents",
		Long: strings.TrimSpace(`
Writes the generated artifact into the directories your agents read.

The plan is printed first and nothing is written until you confirm. An existing
file is never replaced without --force: those directories hold work you wrote by
hand, and a slug collision must not cost you it.
`),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, _, err := loadReport()
			if err != nil {
				return err
			}
			cfg, _, _ := store.LoadConfig()
			if len(targets) == 0 {
				targets = cfg.Install
			}
			if author == "" {
				author = cfg.Author
			}

			resolved := make([]install.Target, 0, len(targets))
			for _, key := range targets {
				t, ok := install.Lookup(key)
				if !ok {
					return fmt.Errorf("unknown target %q — run %s to see the list", key, Bold("ritual install --list"))
				}
				resolved = append(resolved, t)
			}
			if project {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				resolved = append(resolved, install.ProjectTarget(cwd))
			}
			if len(resolved) == 0 {
				return fmt.Errorf("no install target selected; pass --to claude (or set install in config.toml)")
			}

			w := cmd.OutOrStdout()
			plans := make([]install.Plan, 0, len(args)*len(resolved))
			for _, ref := range args {
				bundle, err := bundleFor(rep, ref)
				if err != nil {
					return err
				}
				bundle = improve(cmd.Context(), w, bundle, rep, ref, author)
				for _, t := range resolved {
					plan, err := install.PlanInstall(bundle, t, install.Options{Force: force})
					if err != nil {
						return err
					}
					plans = append(plans, plan)
				}
			}

			fmt.Fprintf(w, "\n%s\n", Bold("Plan"))
			for _, p := range plans {
				fmt.Fprint(w, indentBlock(p.Describe(), "  "))
			}

			if dryRun {
				fmt.Fprintf(w, "\n%s\n", Dim("dry run; nothing was written"))
				return nil
			}
			if !yes && !confirm(cmd.InOrStdin(), w, "Apply this plan?") {
				fmt.Fprintf(w, "%s\n", Dim("cancelled"))
				return nil
			}

			total := 0
			for _, p := range plans {
				changed, err := install.Apply(p)
				if err != nil {
					return err
				}
				total += len(changed)
				for _, path := range changed {
					fmt.Fprintf(w, "%s %s\n", Green("installed"), path)
				}
			}
			if total == 0 {
				fmt.Fprintf(w, "%s\n", Yellow("nothing to do — every file already exists (use --force to replace)"))
				return nil
			}
			fmt.Fprintf(w, "\n%s\n", Dim("Restart or reload your agent so it picks up the new artifact."))
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&targets, "to", nil, "harnesses to install into: claude, codex, cursor, opencode, gemini, grok, qwen, pi, shared")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan and stop")
	cmd.Flags().BoolVar(&force, "force", false, "replace files that already exist")
	cmd.Flags().BoolVar(&project, "project", false, "also install into the current repository")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().StringVar(&author, "author", "", "agent CLI used to improve the prose, or none")

	cmd.AddCommand(&cobra.Command{
		Use:   "targets",
		Short: "List the harnesses ritual can install into",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\n%s\n", Bold("Install targets"))
			for _, t := range install.Targets() {
				status := Dim("not set up")
				if _, err := os.Stat(t.SkillDir); err == nil {
					status = Green("present")
				}
				fmt.Fprintf(w, "  %-10s %-22s %s\n    %s\n", Cyan(t.Key), t.DisplayName, status, Dim(t.SkillDir))
			}
			return nil
		},
	})
	return cmd
}

// confirm asks before writing. Installing into an agent's configuration is a
// change to how that agent behaves from then on, which is not something to do
// on the operator's behalf without asking.
func confirm(in interface{ Read([]byte) (int, error) }, w interface{ Write([]byte) (int, error) }, question string) bool {
	fmt.Fprintf(w, "\n%s %s ", question, Dim("[y/N]"))
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func indentBlock(s, indent string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n") + "\n"
}
