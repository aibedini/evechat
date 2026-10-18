package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/cors"
)

// OpenCORS allows every origin: the widget is embedded on arbitrary customer
// domains and is authenticated by project key, not cookies.
func OpenCORS() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Project-Key", "X-Request-ID", "X-EveChat-Signature", "X-EveChat-Timestamp"},
		ExposedHeaders:   []string{"X-Request-ID", "Retry-After", "ETag"},
		AllowCredentials: false,
		MaxAge:           600,
	})
}

// StrictCORS only allows the dashboard origin (from server.public_url).
func StrictCORS(publicURL string) func(http.Handler) http.Handler {
	origins := []string{}
	if publicURL != "" {
		origins = append(origins, strings.TrimRight(publicURL, "/"))
		if u, err := url.Parse(publicURL); err == nil && u.Host != "" {
			origins = append(origins,
				"http://"+u.Host,
				"https://"+u.Host,
			)
		}
	}
	origins = append(origins, "http://localhost:5173", "http://127.0.0.1:5173")
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           600,
	})
}
