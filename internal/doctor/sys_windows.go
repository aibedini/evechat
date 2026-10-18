//go:build windows

package doctor

// openFileLimit is not exposed on Windows by the Go runtime.
func openFileLimit() (uint64, bool) { return 0, false }

// freeBytes is best-effort on Windows; returning 0 disables the disk check.
func freeBytes(path string) uint64 { return 0 }
