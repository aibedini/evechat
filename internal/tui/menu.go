package tui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// MenuItem is one menu entry.
type MenuItem struct {
	Label string
	Hint  string
}

// Menu renders an arrow-key menu and returns the chosen index.
// When the terminal is not interactive it falls back to numbered input.
func Menu(title string, items []MenuItem) int {
	if len(items) == 0 {
		return -1
	}
	if !IsInteractive() {
		return numberedMenu(title, items)
	}
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return numberedMenu(title, items)
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	selected := 0
	render := func() {
		fmt.Fprintf(os.Stdout, "\r\n%s\r\n", Bold(title))
		for i, it := range items {
			prefix := "   "
			label := it.Label
			if i == selected {
				prefix = " ❯ "
				label = Bold(Cyan(label))
			}
			hint := ""
			if it.Hint != "" {
				hint = " " + Dim("("+it.Hint+")")
			}
			fmt.Fprintf(os.Stdout, "\r\n%s%s%s", prefix, label, hint)
		}
		fmt.Fprint(os.Stdout, "\r\n\r\n"+Dim("↑/↓ انتخاب · Enter تأیید")+"\r\n")
	}
	render()
	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return 0
		}
		if n == 1 {
			switch buf[0] {
			case 3: // Ctrl+C
				fmt.Fprint(os.Stdout, "\r\n")
				return -1
			case 13, 10: // Enter
				return selected
			case 'k':
				if selected > 0 {
					selected--
				}
			case 'j':
				if selected < len(items)-1 {
					selected++
				}
			}
		}
		if n >= 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 'A':
				if selected > 0 {
					selected--
				}
			case 'B':
				if selected < len(items)-1 {
					selected++
				}
			}
		}
		if n == 1 && buf[0] >= '1' && buf[0] <= '9' {
			idx := int(buf[0] - '1')
			if idx < len(items) {
				selected = idx
				return selected
			}
		}
		// Clear and repaint: move cursor up len(items)+3 lines.
		fmt.Fprintf(os.Stdout, "\033[%dA\033[J", len(items)+5)
		render()
	}
}

func numberedMenu(title string, items []MenuItem) int {
	fmt.Println(Bold(title))
	for i, it := range items {
		hint := ""
		if it.Hint != "" {
			hint = Dim(" (" + it.Hint + ")")
		}
		fmt.Printf("  %d) %s%s\n", i+1, it.Label, hint)
	}
	for {
		fmt.Print("شماره گزینه: ")
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return 0
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && n >= 1 && n <= len(items) {
			return n - 1
		}
		if !IsInteractive() {
			return 0
		}
		Fail("شماره نامعتبر است")
	}
}

// MultiSelect renders a checkbox list and returns the selected indexes.
func MultiSelect(title string, items []string, preselected []int) []int {
	selected := make([]bool, len(items))
	for _, idx := range preselected {
		if idx >= 0 && idx < len(items) {
			selected[idx] = true
		}
	}
	if !IsInteractive() {
		return preselected
	}
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return preselected
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	cursor := 0
	render := func() {
		fmt.Fprintf(os.Stdout, "\r\n%s\r\n", Bold(title))
		for i, item := range items {
			box := "[ ]"
			if selected[i] {
				box = Green("[x]")
			}
			prefix := "   "
			if i == cursor {
				prefix = " ❯ "
			}
			fmt.Fprintf(os.Stdout, "\r\n%s%s %s", prefix, box, item)
		}
		fmt.Fprint(os.Stdout, "\r\n\r\n"+Dim("↑/↓ حرکت · Space انتخاب · Enter تأیید")+"\r\n")
	}
	render()
	buf := make([]byte, 3)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 1 {
			switch buf[0] {
			case 3:
				fmt.Fprint(os.Stdout, "\r\n")
				return nil
			case 13, 10:
				out := make([]int, 0, len(items))
				for i, on := range selected {
					if on {
						out = append(out, i)
					}
				}
				fmt.Fprint(os.Stdout, "\r\n")
				return out
			case ' ':
				selected[cursor] = !selected[cursor]
			case 'k':
				if cursor > 0 {
					cursor--
				}
			case 'j':
				if cursor < len(items)-1 {
					cursor++
				}
			}
		}
		if n >= 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 'A':
				if cursor > 0 {
					cursor--
				}
			case 'B':
				if cursor < len(items)-1 {
					cursor++
				}
			}
		}
		fmt.Fprintf(os.Stdout, "\033[%dA\033[J", len(items)+5)
		render()
	}
}

// Select is the single-choice convenience wrapper.
func Select(title string, options []string, def int) int {
	items := make([]MenuItem, len(options))
	for i, o := range options {
		items[i] = MenuItem{Label: o}
	}
	idx := Menu(title, items)
	if idx < 0 {
		return def
	}
	return idx
}

var _ = bufio.NewReader
