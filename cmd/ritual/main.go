// Command ritual mines coding-agent session history for the workflows you keep
// repeating and turns them into skills, rules, commands, and hooks.
//
// Everything runs locally. See https://github.com/HarjjotSinghh/ritual.
package main

import (
	"os"

	"github.com/HarjjotSinghh/ritual/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
