package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Injected at build time through -ldflags.
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

// resolveBuildInfo falls back to the VCS metadata the Go toolchain embeds, so a
// plain `go build` still reports the commit (and whether the tree was dirty)
// instead of "dev (none, unknown)".
func resolveBuildInfo() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "none" && len(s.Value) >= 7 {
				commit = s.Value[:7]
			}
		case "vcs.time":
			if buildDate == "unknown" && s.Value != "" {
				buildDate = s.Value
			}
		case "vcs.modified":
			if s.Value == "true" && commit != "none" && !strings.HasSuffix(commit, "-dirty") {
				commit += "-dirty"
			}
		}
	}
	if version == "dev" && commit != "none" {
		version = "dev-" + commit
	}
}

// versionString is printed by `evechat version` and used in release notes.
func versionString() string {
	resolveBuildInfo()
	return fmt.Sprintf("evechat %s (%s, %s, %s, %s/%s)",
		version, commit, buildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// shortVersion is the bare semantic version.
func shortVersion() string { resolveBuildInfo(); return version }
