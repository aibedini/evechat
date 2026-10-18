// Package assets carries the compiled widget and dashboard bundles.
//
// The go:embed directive must live in (or above) the directory it embeds, so it
// lives here at the repository root of the embed/ tree and internal/embedfs
// re-exports a sub-FS. Dev stubs (embed/*/index.html, embed/widget/widget.js)
// keep the build green before the front-ends are compiled.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:dashboard
var dashboardFS embed.FS

//go:embed all:widget
var widgetFS embed.FS

// Dashboard returns the embedded dashboard build rooted at embed/dashboard.
func Dashboard() fs.FS { return dashboardFS }

// Widget returns the embedded widget build rooted at embed/widget.
func Widget() fs.FS { return widgetFS }
