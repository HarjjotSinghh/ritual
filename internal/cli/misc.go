package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/HarjjotSinghh/ritual/internal/agentspec"
	"github.com/HarjjotSinghh/ritual/internal/authoring"
	"github.com/HarjjotSinghh/ritual/internal/classify"
	"github.com/HarjjotSinghh/ritual/internal/ingest"
	"github.com/HarjjotSinghh/ritual/internal/install"
	"github.com/HarjjotSinghh/ritual/internal/inventory"
	"github.com/HarjjotSinghh/ritual/internal/redact"
	"github.com/HarjjotSinghh/ritual/internal/store"
	"github.com/HarjjotSinghh/ritual/internal/version"
)

func newAgentsCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Show which agent session stores ritual can see",
		Long: strings.TrimSpace(`
Lists every agent in ritual's catalog, where its sessions live on this machine,
and how many session files were found.

An agent showing zero files is not a bug on its own: it may not be installed, or
it may keep its history somewhere this build does not know about yet. The path
is printed either way so the difference is visible.
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\n%s\n", Bold("Agent session stores"))
			found := 0
			for _, spec := range agentspec.Catalog() {
				d, installed, err := agentspec.Discover(spec)
				switch {
				case err != nil:
					fmt.Fprintf(w, "  %-10s %s %s\n", Cyan(spec.Key), Red("error"), err)
					continue
				case !installed:
					if all {
						fmt.Fprintf(w, "  %-10s %s %s\n", Dim(spec.Key), Dim("not installed"), Dim(defaultRoot(spec)))
					}
					continue
				}
				found++
				// A file is not a session. Most stores write one file per
				// session, but a database holds thousands in one file, and
				// reporting "1 session" for a 500 MB store was simply wrong.
				unit := "session files"
				if spec.WholeStore {
					unit = "store file (many sessions inside)"
				}
				count := Green(fmt.Sprintf("%d %s", len(d.Files), unit))
				if len(d.Files) == 0 {
					count = Yellow("no session files found")
				}
				fmt.Fprintf(w, "  %-10s %-24s %s\n", Cyan(spec.Key), spec.DisplayName, count)
				for _, root := range d.Roots {
					fmt.Fprintf(w, "    %s\n", Dim(filepath.ToSlash(redact.Path(root))+"/"+spec.Glob))
				}
				if spec.Note != "" {
					fmt.Fprintf(w, "    %s\n", Dim(spec.Note))
				}
			}
			if found == 0 {
				fmt.Fprintf(w, "  %s\n", Yellow("no agent session stores found"))
				fmt.Fprintf(w, "  %s\n", Dim("ritual reads history that already exists; it cannot create any"))
			}
			if !all {
				fmt.Fprintf(w, "\n%s\n", Dim("pass --all to include agents that are not installed here"))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include agents that are not installed on this machine")
	return cmd
}

func defaultRoot(spec agentspec.Spec) string {
	if len(spec.Roots) == 0 {
		return ""
	}
	return redact.Path(spec.Roots[0])
}

func newInventoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "inventory",
		Aliases: []string{"installed"},
		Short:   "Show the skills, commands, and rules already installed",
		Long: strings.TrimSpace(`
Lists what is already set up on this machine. ritual uses this to avoid
proposing a skill for something you automated months ago.
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			roots := projectRootsFromReport()
			inv, err := inventory.Scan(roots)
			if err != nil {
				return err
			}
			if len(inv.Items) == 0 {
				fmt.Fprintf(w, "\n%s\n", Yellow("nothing installed yet"))
				return nil
			}
			byKind := map[inventory.Kind][]inventory.Item{}
			for _, item := range inv.Items {
				byKind[item.Kind] = append(byKind[item.Kind], item)
			}
			for _, kind := range []inventory.Kind{inventory.KindSkill, inventory.KindCommand, inventory.KindRules} {
				items := byKind[kind]
				if len(items) == 0 {
					continue
				}
				fmt.Fprintf(w, "\n%s %s\n", Bold(strings.ToUpper(string(kind))), Dim(fmt.Sprintf("(%d)", len(items))))
				for _, item := range items {
					fmt.Fprintf(w, "  %-30s %-9s %s\n", Truncate(item.Name, 30), Dim(item.Harness), Truncate(oneLine(item.Description), 56))
				}
			}
			return nil
		},
	}
}

// projectRootsFromReport reuses the last scan's workspaces so the inventory
// looks in the repositories the operator actually works in, rather than walking
// the disk.
func projectRootsFromReport() []string {
	rep, _, err := store.LoadReport()
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	roots := make([]string, 0, 8)
	home, _ := os.UserHomeDir()
	for _, f := range rep.Findings {
		for _, e := range f.Evidence {
			if e.Repo == "" {
				continue
			}
			if _, ok := seen[e.Repo]; ok {
				continue
			}
			seen[e.Repo] = struct{}{}
		}
	}
	// The report stores redacted workspaces on the evidence's source paths;
	// reconstructing them is best-effort and never fails the command.
	for repo := range seen {
		for _, base := range []string{"Documents/Projects", "Documents", "code", "src", "dev", "projects"} {
			candidate := filepath.Join(home, base, repo)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				roots = append(roots, candidate)
				break
			}
		}
	}
	sort.Strings(roots)
	return roots
}

func newRulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Work with the standing preferences you keep restating",
		Long: strings.TrimSpace(`
A preference you have stated more than once — "always run the mobile check",
"never touch the generated schema" — is not a workflow. It belongs in a rules
file, where it applies to every turn without being invoked.
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, _, err := loadReport()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			rules := actionableRules(rep)
			if len(rules) == 0 {
				fmt.Fprintf(w, "\n%s\n", Yellow("no repeated standing preferences found"))
				fmt.Fprintf(w, "%s\n", Dim("ritual only proposes a rule you have stated more than once"))
				return nil
			}
			fmt.Fprintf(w, "\n%s\n", Bold(fmt.Sprintf("%d standing preferences", len(rules))))
			for _, r := range rules {
				fmt.Fprintf(w, "\n  %s %s\n", Cyan(r.ID), Dim(fmt.Sprintf("stated %s across %s", plural(r.Occurrences, "time", "times"), plural(r.Sessions, "session", "sessions"))))
				fmt.Fprintf(w, "%s\n", wrapBody(oneLine(r.Text), 74, "    "))
			}
			fmt.Fprintf(w, "\n%s %s\n", Dim("install one with"), Cyan("ritual rules install <id>"))
			return nil
		},
	}

	var targets []string
	var dryRun, yes bool
	installCmd := &cobra.Command{
		Use:   "install <id>...",
		Short: "Append a standing preference to your rules files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, _, err := loadReport()
			if err != nil {
				return err
			}
			cfg, _, _ := store.LoadConfig()
			if len(targets) == 0 {
				targets = cfg.Install
			}
			w := cmd.OutOrStdout()
			plans := make([]install.Plan, 0, len(args))
			for _, ref := range args {
				r, ok := rep.FindRule(ref)
				if !ok {
					return fmt.Errorf("no rule with id %q", ref)
				}
				if r.Decision.Kind == classify.KindIgnore {
					return fmt.Errorf("rule %s is already covered: %s", ref, r.Decision.Rationale)
				}
				bundle, err := bundleFor(rep, ref)
				if err != nil {
					return err
				}
				for _, key := range targets {
					t, ok := install.Lookup(key)
					if !ok {
						return fmt.Errorf("unknown target %q", key)
					}
					plan, err := install.PlanInstall(bundle, t, install.Options{})
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
			if !yes && !confirm(cmd.InOrStdin(), w, "Append these rules?") {
				fmt.Fprintf(w, "%s\n", Dim("cancelled"))
				return nil
			}
			for _, p := range plans {
				changed, err := install.Apply(p)
				if err != nil {
					return err
				}
				for _, path := range changed {
					fmt.Fprintf(w, "%s %s\n", Green("appended to"), path)
				}
			}
			return nil
		},
	}
	installCmd.Flags().StringSliceVar(&targets, "to", nil, "harnesses to append to")
	installCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan and stop")
	installCmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	cmd.AddCommand(installCmd)
	return cmd
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check what ritual can see and what it cannot",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "\n%s\n", Bold(version.String()))

			dir, err := store.Dir()
			if err != nil {
				fmt.Fprintf(w, "  %s ritual directory: %v\n", Red("✗"), err)
			} else {
				fmt.Fprintf(w, "  %s ritual directory %s\n", Green("✓"), Dim(redact.Path(dir)))
			}

			cfg, cfgPath, err := store.LoadConfig()
			if err != nil {
				fmt.Fprintf(w, "  %s config: %v\n", Red("✗"), err)
			} else if _, statErr := os.Stat(cfgPath); statErr == nil {
				fmt.Fprintf(w, "  %s config %s\n", Green("✓"), Dim(redact.Path(cfgPath)))
			} else {
				fmt.Fprintf(w, "  %s config not written yet, using defaults %s\n", Dim("·"), Dim("(ritual config --init)"))
			}

			// One scan, cross-referenced against the file counts. An agent
			// whose files all parse to nothing used to print a tick, which is
			// the failure mode hardest to notice and most worth reporting.
			kept := map[string]int{}
			parsed := map[string]int{}
			scan, scanErr := ingest.Scan(ingest.Options{Limits: ingest.DefaultLimits()})
			if scanErr == nil {
				for _, st := range scan.Stats {
					kept[st.Agent] = st.Sessions
				}
				parsed = scan.Parsed
			}

			fmt.Fprintf(w, "\n%s\n", Bold("Agents"))
			installed := 0
			for _, spec := range agentspec.Catalog() {
				d, ok, discErr := agentspec.Discover(spec)
				switch {
				case discErr != nil:
					fmt.Fprintf(w, "  %s %-10s %v\n", Red("✗"), spec.Key, discErr)
				case !ok:
					continue
				default:
					installed++
					files, read, used := len(d.Files), parsed[spec.Key], kept[spec.Key]
					mark := Green("✓")
					switch {
					case files == 0:
						mark = Yellow("!")
					case read == 0:
						// Nothing came out of the reader at all. That is a bug
						// in ritual, and the only case worth alarming about.
						mark = Red("✗")
					case used == 0:
						mark = Yellow("!")
					}
					fmt.Fprintf(w, "  %s %-10s %d files → %d parsed → %d in scan%s\n",
						mark, spec.Key, files, read, used, skippedNote(d.Skipped))
					switch {
					case files > 0 && read == 0:
						fmt.Fprintf(w, "      %s\n", Red("every file parsed to nothing — the reader or the layout is wrong; please report this"))
					case read > 0 && used == 0:
						fmt.Fprintf(w, "      %s\n", Dim("parsed fine, then filtered out: no human turn, or outside the scan window — try `ritual scan --days 0 --include-automated`"))
					case files > 0 && read*3 < files:
						fmt.Fprintf(w, "      %s\n", Dim("most files are subagent transcripts, which are read as part of their parent rather than on their own"))
					}
					for _, root := range d.Roots[1:] {
						fmt.Fprintf(w, "      %s\n", Dim("also reading "+filepath.ToSlash(redact.Path(root))))
					}
				}
			}
			if installed == 0 {
				fmt.Fprintf(w, "  %s no agents found; ritual has nothing to read\n", Yellow("!"))
			}
			if scanErr != nil {
				fmt.Fprintf(w, "  %s could not parse: %v\n", Red("✗"), scanErr)
			}

			fmt.Fprintf(w, "\n%s\n", Bold("Authoring"))
			detected := authoring.Detect()
			if len(detected) == 0 {
				fmt.Fprintf(w, "  %s no agent CLI found; artifacts use the deterministic template\n", Dim("·"))
			} else {
				for _, a := range detected {
					fmt.Fprintf(w, "  %s %s\n", Green("✓"), a.DisplayName)
				}
			}

			fmt.Fprintf(w, "\n%s\n", Bold("Last report"))
			rep, path, loadErr := store.LoadReport()
			if loadErr != nil {
				fmt.Fprintf(w, "  %s none yet — run %s\n", Dim("·"), Cyan("ritual scan"))
			} else {
				fmt.Fprintf(w, "  %s %s, %d findings %s\n", Green("✓"), store.Age(rep.GeneratedAt), len(rep.Findings), Dim(redact.Path(path)))
				if len(rep.Warnings) > 0 {
					fmt.Fprintf(w, "\n%s\n", Bold("Warnings from that scan"))
					for i, warning := range rep.Warnings {
						if i >= 10 {
							fmt.Fprintf(w, "  %s\n", Dim(fmt.Sprintf("… and %d more", len(rep.Warnings)-10)))
							break
						}
						fmt.Fprintf(w, "  %s %s\n", Yellow("!"), warning)
					}
				}
			}
			_ = cfg
			return nil
		},
	}
}

func skippedNote(skipped int) string {
	if skipped == 0 {
		return ""
	}
	return Dim(fmt.Sprintf(" (%d entries skipped)", skipped))
}

func newConfigCmd() *cobra.Command {
	var initialize bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or create ritual's configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			cfg, path, err := store.LoadConfig()
			if err != nil {
				return err
			}
			if initialize {
				written, err := store.SaveConfig(cfg)
				if err != nil {
					return err
				}
				fmt.Fprintf(w, "%s %s\n", Green("wrote"), written)
				return nil
			}
			fmt.Fprintf(w, "\n%s %s\n\n", Bold("Configuration"), Dim(path))
			fmt.Fprintf(w, "  days            %d\n", cfg.Days)
			fmt.Fprintf(w, "  min_score       %.0f\n", cfg.MinScore)
			fmt.Fprintf(w, "  min_occurrences %d\n", cfg.MinOccurrences)
			fmt.Fprintf(w, "  min_sessions    %d\n", cfg.MinSessions)
			fmt.Fprintf(w, "  threshold       %.2f\n", cfg.Threshold)
			fmt.Fprintf(w, "  keep_emails     %t\n", cfg.KeepEmails)
			fmt.Fprintf(w, "  author          %s\n", cfg.Author)
			fmt.Fprintf(w, "  install         %s\n", strings.Join(cfg.Install, ", "))
			if len(cfg.Agents) > 0 {
				fmt.Fprintf(w, "  agents          %s\n", strings.Join(cfg.Agents, ", "))
			}
			fmt.Fprintf(w, "\n%s\n", Dim("edit the file directly, or run `ritual config --init` to create it"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&initialize, "init", false, "write the current settings to config.toml")
	return cmd
}
