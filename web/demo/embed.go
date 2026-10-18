// Package demo carries the manual widget test page served at /demo/test.
//
// The page lives next to the front-ends instead of inside embed/ because it is
// a debugging aid, not part of the shipped SPA bundles: the server fills in the
// live project key and API URL when it is requested.
package demo

import (
	_ "embed"
	"strings"
)

//go:embed test.html
var testPage string

// Render fills the placeholders with the values of the running instance. Every
// value is escaped for a double-quoted JavaScript string so a hostile project
// key cannot break out of the config object.
func Render(apiURL, projectKey string) string {
	return strings.NewReplacer(
		"__EVECHAT_API_URL__", jsString(strings.TrimRight(apiURL, "/")),
		"__EVECHAT_PROJECT_KEY__", jsString(projectKey),
	).Replace(testPage)
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
