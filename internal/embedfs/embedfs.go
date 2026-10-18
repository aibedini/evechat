// Package embedfs exposes the compiled front-end bundles and static assets to
// the HTTP layer. When a bundle is missing (source checkout before `make
// build`), a friendly placeholder is served instead of failing at startup.
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

// Assets returns the static assets rooted at embed/assets (fonts live under
// "fonts/").
func Assets() fs.FS {
	sub, err := fs.Sub(assets.Assets(), "assets")
	if err != nil {
		return assets.Assets()
	}
	return sub
}

// Setup returns the first-run wizard bundle rooted at embed/setup.
func Setup() fs.FS {
	sub, err := fs.Sub(assets.Setup(), "setup")
	if err != nil {
		return assets.Setup()
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

// ReadWidget returns the embedded widget bundle bytes.
func ReadWidget() ([]byte, error) {
	return fs.ReadFile(Widget(), "widget.js")
}

// HasFont reports whether the self-hosted Vazirmatn variable font is embedded.
func HasFont() bool {
	f, err := Assets().Open("fonts/vazirmatn-variable.woff2")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// HasSetup reports whether the web setup wizard is embedded.
func HasSetup() bool {
	f, err := Setup().Open("index.html")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
