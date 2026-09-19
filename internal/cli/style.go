package cli

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Styling is hand-rolled rather than delegated to a framework. The output is a
// handful of colours and one bar chart; a dependency that renders tables would
// be more code than it saves, and ritual should stay quick to audit.

var useColor = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}()

// SetColor forces colour on or off, used by --no-color.
func SetColor(on bool) { useColor = on }

// IsTerminal reports whether stdout is a terminal. Progress that redraws a line
// with carriage returns is helpful in a terminal and unreadable in a log file,
// so anything that redraws checks this first.
func IsTerminal() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	cyan   = "\033[36m"
)

func paint(code, s string) string {
	if !useColor || s == "" {
		return s
	}
	return code + s + reset
}

// Bold renders emphasis.
func Bold(s string) string { return paint(bold, s) }

// Dim renders secondary text.
func Dim(s string) string { return paint(dim, s) }

// Red renders a problem.
func Red(s string) string { return paint(red, s) }

// Green renders a success.
func Green(s string) string { return paint(green, s) }

// Yellow renders a caution.
func Yellow(s string) string { return paint(yellow, s) }

// Blue renders a label.
func Blue(s string) string { return paint(blue, s) }

// Cyan renders an identifier.
func Cyan(s string) string { return paint(cyan, s) }

// Heading prints a section title.
func Heading(w *strings.Builder, text string) {
	fmt.Fprintf(w, "\n%s\n", Bold(text))
}

// Bar renders a 0..1 value as a fixed-width meter.
func Bar(value float64, width int) string {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	filled := int(value*float64(width) + 0.5)
	return strings.Repeat("█", filled) + Dim(strings.Repeat("·", width-filled))
}

// ScoreColor picks a colour for a 0..100 score.
func ScoreColor(total float64) string {
	switch {
	case total >= 60:
		return Green(fmt.Sprintf("%3.0f", total))
	case total >= 35:
		return Yellow(fmt.Sprintf("%3.0f", total))
	default:
		return Dim(fmt.Sprintf("%3.0f", total))
	}
}

// KindBadge renders a classification as a short, aligned label.
func KindBadge(kind string) string {
	label := fmt.Sprintf("%-9s", kind)
	switch kind {
	case "skill":
		return Green(label)
	case "rule":
		return Blue(label)
	case "command":
		return Cyan(label)
	case "hook":
		return Yellow(label)
	case "update":
		return Yellow(label)
	case "reference":
		return Dim(label)
	default:
		return Dim(label)
	}
}

// Truncate shortens a display string to a column width.
func Truncate(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s + strings.Repeat(" ", width-len(r))
	}
	if width <= 1 {
		return string(r[:width])
	}
	return string(r[:width-1]) + "…"
}
