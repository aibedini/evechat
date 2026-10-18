// Package assets carries the compiled widget, dashboard, setup wizard and the
// self-hosted Vazirmatn font.
//
// The go:embed directive must live in (or above) the directory it embeds, so it
// lives here at the root of the embed/ tree and internal/embedfs re-exports
// sub-FS views. Dev stubs keep the build green before the front-ends compile.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:dashboard
var dashboardFS embed.FS

//go:embed all:widget
var widgetFS embed.FS

//go:embed all:assets
var assetsFS embed.FS

//go:embed all:setup
var setupFS embed.FS

// Dashboard returns the embedded dashboard build rooted at embed/dashboard.
func Dashboard() fs.FS { return dashboardFS }

// Widget returns the embedded widget build rooted at embed/widget.
func Widget() fs.FS { return widgetFS }

// Assets returns the embedded static assets rooted at embed/assets.
func Assets() fs.FS { return assetsFS }

// Setup returns the embedded setup wizard rooted at embed/setup.
func Setup() fs.FS { return setupFS }
