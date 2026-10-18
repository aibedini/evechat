package tui

import (
	"fmt"
	"os"
)

var colorEnabled = detectColor()

func detectColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return IsInteractive()
}

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiCyan   = "\033[36m"
)

func paint(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + ansiReset
}

// Bold renders bold text when colours are enabled.
func Bold(s string) string { return paint(ansiBold, s) }

// Dim renders dim text when colours are enabled.
func Dim(s string) string { return paint(ansiDim, s) }

// Cyan renders cyan text when colours are enabled.
func Cyan(s string) string { return paint(ansiCyan, s) }

// Green renders green text when colours are enabled.
func Green(s string) string { return paint(ansiGreen, s) }

// Yellow renders yellow text when colours are enabled.
func Yellow(s string) string { return paint(ansiYellow, s) }

// Red renders red text when colours are enabled.
func Red(s string) string { return paint(ansiRed, s) }

// OK prints a success line.
func OK(s string) {
	fmt.Printf("%s %s\n", Green("✓"), s)
}

// Warn prints a warning line.
func Warn(s string) {
	fmt.Printf("%s %s\n", Yellow("!"), s)
}

// Fail prints an error line.
func Fail(s string) {
	fmt.Printf("%s %s\n", Red("✗"), s)
}

// Info prints a neutral line.
func Info(s string) {
	fmt.Printf("  %s\n", s)
}
