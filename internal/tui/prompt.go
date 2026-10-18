package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

var reader = bufio.NewReader(os.Stdin)

// Ask prompts for a value with an optional default and validator.
func Ask(label, def string, validate func(string) error) string {
	for {
		if def != "" {
			fmt.Printf("%s %s: ", Bold(label), Dim("["+def+"]"))
		} else {
			fmt.Printf("%s: ", Bold(label))
		}
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return def
		}
		value := strings.TrimSpace(line)
		if value == "" {
			value = def
		}
		if validate != nil {
			if err := validate(value); err != nil {
				Fail(err.Error())
				if !IsInteractive() {
					return value
				}
				continue
			}
		}
		return value
	}
}

// AskSecret reads a value without echo. Falls back to a plain read when stdin
// is not a terminal.
func AskSecret(label string) string {
	fmt.Printf("%s: ", Bold(label))
	if !IsStdinTerminal() {
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(string(b))
}

// AskYesNo asks a yes/no question.
func AskYesNo(label string, def bool) bool {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	for {
		fmt.Printf("%s %s: ", Bold(label), Dim("["+hint+"]"))
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return def
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "":
			return def
		case "y", "yes", "بله", "ب", "1":
			return true
		case "n", "no", "خیر", "خ", "0":
			return false
		default:
			if !IsInteractive() {
				return def
			}
			Fail("لطفاً y یا n وارد کنید")
		}
	}
}
