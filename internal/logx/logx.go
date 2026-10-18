package logx

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var logger = zerolog.New(os.Stderr).With().Timestamp().Logger()

// Init configures the global logger. pretty enables human friendly console
// output (development); production uses JSON lines.
func Init(level string, pretty bool) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil || level == "" {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	var out = os.Stderr
	if pretty {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: out, TimeFormat: time.RFC3339}).With().Timestamp().Logger()
	} else {
		logger = zerolog.New(out).With().Timestamp().Logger()
	}
}

// L returns the global logger.
func L() *zerolog.Logger { return &logger }

type ctxKey struct{}

// WithRequestID returns a context carrying the request id.
func WithRequestID(ctx context.Context, rid string) context.Context {
	return context.WithValue(ctx, ctxKey{}, rid)
}

// RequestID extracts the request id from a context.
func RequestID(ctx context.Context) string {
	s, _ := ctx.Value(ctxKey{}).(string)
	return s
}

// WithReq pulls request_id (and any other ambient fields) into the logger.
func WithReq(ctx context.Context) *zerolog.Logger {
	l := logger.With()
	if rid := RequestID(ctx); rid != "" {
		l = l.Str("request_id", rid)
	}
	lg := l.Logger()
	return &lg
}
