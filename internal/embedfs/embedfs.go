// Package embedfs exposes the compiled front-end bundles to the HTTP layer.
// When the bundles are missing (source checkout before `make build`), a friendly
// placeholder is served instead of failing at startup.
package embedfs

import (
	"io/fs"

	assets "github.com/evechat/evechat/embed"
)

// Dashboard returns the dashboard SPA rooted at the bundle root.
func Dashboard() fs.FS {
	sub, err := fs.Sub(assets.Dashboard(), "dashboard")
	if err != nil {
		return assets.Dashboard()
	}
	return sub
}

// Widget returns the widget bundle rooted at the bundle root.
func Widget() fs.FS {
	sub, err := fs.Sub(assets.Widget(), "widget")
	if err != nil {
		return assets.Widget()
	}
	return sub
}

// HasDashboardIndex reports whether a real dashboard build is embedded.
func HasDashboardIndex() bool {
	f, err := Dashboard().Open("index.html")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
