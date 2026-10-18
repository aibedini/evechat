// Package tui is a dependency-light terminal UI for the EveChat setup wizard.
// It never assumes a TTY: when stdin/stdout are redirected it degrades to plain
// numbered prompts with no escape sequences.
package tui

import (
	"os"

	"golang.org/x/term"
)

// IsInteractive reports whether both stdin and stdout are terminals.
func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// IsStdinTerminal reports whether prompts can read raw key presses.
func IsStdinTerminal() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
