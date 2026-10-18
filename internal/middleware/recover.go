package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/logx"
)

// Recover converts panics into a JSON 500 without killing the process.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logx.WithReq(r.Context()).Error().
					Interface("panic", rec).
					Str("path", r.URL.Path).
					Str("stack", string(debug.Stack())).
					Msg("recovered panic")
				apierr.Write(w, apierr.Internal(nil))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
