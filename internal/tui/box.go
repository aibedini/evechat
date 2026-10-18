package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const minBoxWidth = 46

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func padRight(s string, width int) string {
	n := width - runeLen(s)
	if n <= 0 {
		return s
	}
	return s + strings.Repeat(" ", n)
}

// Title prints the wizard banner.
func Title(s string) {
	width := runeLen(s) + 8
	if width < minBoxWidth {
		width = minBoxWidth
	}
	fmt.Println()
	fmt.Println("╔" + strings.Repeat("═", width) + "╗")
	left := (width - runeLen(s)) / 2
	right := width - runeLen(s) - left
	fmt.Println("║" + strings.Repeat(" ", left) + Bold(s) + strings.Repeat(" ", right) + "║")
	fmt.Println("╚" + strings.Repeat("═", width) + "╝")
	fmt.Println()
}

// Section prints a section heading.
func Section(s string) {
	fmt.Println()
	fmt.Println(Bold("▌ " + s))
}

// Step prints "گام ۲ از ۸ — پایگاه داده".
func Step(n, total int, title string) {
	fmt.Println()
	fmt.Println(Bold(fmt.Sprintf("گام %s از %s — %s", faNum(n), faNum(total), title)))
	fmt.Println(Dim(strings.Repeat("─", 48)))
}

// Box prints a bordered summary box.
func Box(title string, lines []string) {
	width := runeLen(title) + 6
	for _, l := range lines {
		if w := runeLen(l) + 4; w > width {
			width = w
		}
	}
	if width < minBoxWidth {
		width = minBoxWidth
	}
	fmt.Println("╔" + strings.Repeat("═", width) + "╗")
	if title != "" {
		left := (width - runeLen(title)) / 2
		if left < 0 {
			left = 0
		}
		right := width - runeLen(title) - left
		if right < 0 {
			right = 0
		}
		fmt.Println("║" + strings.Repeat(" ", left) + Bold(title) + strings.Repeat(" ", right) + "║")
		fmt.Println("╠" + strings.Repeat("═", width) + "╣")
	}
	for _, l := range lines {
		fmt.Println("║ " + padRight(l, width-1) + "║")
	}
	fmt.Println("╚" + strings.Repeat("═", width) + "╝")
}

// faNum renders an integer with Persian digits (used for step counters).
func faNum(n int) string {
	digits := []rune("۰۱۲۳۴۵۶۷۸۹")
	s := fmt.Sprint(n)
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			out = append(out, digits[r-'0'])
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}
