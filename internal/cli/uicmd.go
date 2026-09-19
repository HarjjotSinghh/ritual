package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/HarjjotSinghh/ritual/internal/ui"
)

func newUICmd() *cobra.Command {
	var (
		addr         string
		allowInstall bool
		noOpen       bool
	)
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Open the local dashboard for the last scan",
		Long: strings.TrimSpace(`
Serves the last report as a page in your browser.

The server binds to the loopback interface and requires a token that is printed
here, so the page is reachable from this machine and nowhere else. It is
read-only unless you pass --allow-install.
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, _, err := loadReport()
			if err != nil {
				return err
			}
			srv, err := ui.New(rep, allowInstall)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			return srv.Serve(addr, func(url string) {
				fmt.Fprintf(w, "\n%s %s\n", Bold("ritual ui"), Dim("serving the last report"))
				fmt.Fprintf(w, "  %s\n", Cyan(url))
				if allowInstall {
					fmt.Fprintf(w, "  %s\n", Yellow("install is enabled: this page can write to your agent directories"))
				} else {
					fmt.Fprintf(w, "  %s\n", Dim("read-only; pass --allow-install to install from the browser"))
				}
				fmt.Fprintf(w, "  %s\n", Dim("press ctrl-c to stop"))
				if !noOpen {
					openBrowser(url)
				}
			})
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:4783", "address to bind (loopback only)")
	cmd.Flags().BoolVar(&allowInstall, "allow-install", false, "let the page install artifacts")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "do not open a browser")
	return cmd
}

// openBrowser is best-effort. A failure to launch a browser is not a failure of
// the command: the URL is already printed.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
