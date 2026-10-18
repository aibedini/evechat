package main

import (
	"fmt"
	"runtime"
)

// Injected at build time through -ldflags.
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

// versionString is printed by `evechat version` and used in release notes.
func versionString() string {
	return fmt.Sprintf("evechat %s (%s, %s, %s, %s/%s)",
		version, commit, buildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// shortVersion is the bare semantic version.
func shortVersion() string { return version }
