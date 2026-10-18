// Package pii masks personally identifying values before they reach logs,
// audit rows, error strings or outbound payloads.
package pii

import "strings"

// MaskPhone keeps the first three and last two characters of a phone number and
// replaces everything in between, e.g. 09121234567 -> "091***67". Values too
// short to hide anything are replaced entirely.
func MaskPhone(p string) string {
	p = strings.TrimSpace(p)
	r := []rune(p)
	if len(r) < 7 {
		return "***"
	}
	return string(r[:3]) + "***" + string(r[len(r)-2:])
}
