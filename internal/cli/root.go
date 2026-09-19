// Package cli is ritual's command surface.
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/HarjjotSinghh/ritual/internal/report"
	"github.com/HarjjotSinghh/ritual/internal/store"
	"github.com/HarjjotSinghh/ritual/internal/version"
)

type globalFlags struct {
	noColor bool
	json    bool
}

var globals globalFlags

// NewRoot builds the command tree.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "ritual",
		Short: "Mine your coding-agent history for the workflows you keep repeating",
		Long: strings.TrimSpace(`
ritual reads the session history your coding agents already write to disk —
Claude Code, Codex, Cursor, OpenCode, Gemini, Grok, and others — finds the work
you do over and over, and turns it into skills, rules, commands, and hooks.

Everything runs on this machine. Nothing is uploaded, and there is no account.
`),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if globals.noColor {
				SetColor(false)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	root.PersistentFlags().BoolVar(&globals.noColor, "no-color", false, "disable coloured output")

	root.AddCommand(
		newScanCmd(),
		newListCmd(),
		newShowCmd(),
		newBuildCmd(),
		newInstallCmd(),
		newRulesCmd(),
		newAgentsCmd(),
		newInventoryCmd(),
		newDoctorCmd(),
		newConfigCmd(),
		newUICmd(),
		newEvalCmd(),
		newVersionCmd(),
	)
	return root
}

// Execute runs the CLI and returns a process exit code.
func Execute() int {
	if err := NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, Red("error: ")+err.Error())
		return 1
	}
	return 0
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), version.String())
			return nil
		},
	}
}

// loadReport reads the saved report, with an error that tells the operator what
// to do rather than what failed.
func loadReport() (*report.Report, string, error) {
	rep, path, err := store.LoadReport()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, path, fmt.Errorf("no saved report yet — run %s first", Bold("ritual scan"))
		}
		return nil, path, err
	}
	return rep, path, nil
}

func sinceFrom(days int) time.Time {
	if days <= 0 {
		return time.Time{}
	}
	return time.Now().AddDate(0, 0, -days).UTC()
}
