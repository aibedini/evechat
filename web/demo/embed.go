// Package demo carries the local-only demonstration pages (/demo*).
//
// They live next to the front-ends instead of inside embed/ because they are
// debugging aids, not part of the shipped SPA bundles: the server fills in the
// live project key and API URL when a page is requested.
package demo

import (
	"embed"
	"strings"
)

//go:embed *.html *.css *.js
var files embed.FS

// Page renders one demo page with the values of the running instance. Every
// value is escaped for a double-quoted JavaScript string so a hostile project
// key cannot break out of the config object.
func Page(name, apiURL, projectKey string) (string, error) {
	raw, err := files.ReadFile(name)
	if err != nil {
		return "", err
	}
	return strings.NewReplacer(
		"__EVECHAT_API_URL__", jsString(strings.TrimRight(apiURL, "/")),
		"__EVECHAT_PROJECT_KEY__", jsString(projectKey),
	).Replace(string(raw)), nil
}

// Asset returns a static demo asset (css/js) unchanged.
func Asset(name string) ([]byte, error) { return files.ReadFile(name) }

// Size reports the total embedded size of the demo assets, used by the budget
// test (the demo bundle must stay under 300 KB).
func Size() int {
	entries, err := files.ReadDir(".")
	if err != nil {
		return 0
	}
	total := 0
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			total += int(info.Size())
		}
	}
	return total
}

func jsString(v string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"<", `\u003c`,
		">", `\u003e`,
		"&", `\u0026`,
	).Replace(v)
}
