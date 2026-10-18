package logx

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var logger = zerolog.New(os.Stderr).With().Timestamp().Logger()

// Init configures the global logger. pretty enables human friendly console
// output (development); production uses JSON lines.
func Init(level string, pretty bool) {
	setup(level, pretty, os.Stderr)
}

// InitFile configures the logger and mirrors output into a size-rotated file.
// Rotation is hand-rolled so no extra dependency is needed.
func InitFile(level string, pretty bool, file string, maxSizeMB, maxBackups int) error {
	base := io.Writer(os.Stderr)
	if file != "" {
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil && filepath.Dir(file) != "." {
			return fmt.Errorf("creating log directory: %w", err)
		}
		rw, err := newRotatingWriter(file, maxSizeMB, maxBackups)
		if err != nil {
			return err
		}
		base = io.MultiWriter(os.Stderr, &sampleWriter{inner: rw, seen: map[uint64]time.Time{}})
	}
	setup(level, pretty, base)
	return nil
}

func setup(level string, pretty bool, out io.Writer) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil || level == "" {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = time.RFC3339Nano

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

// ---- size rotated file writer ----

type rotatingWriter struct {
	mu         sync.Mutex
	path       string
	maxSize    int64
	maxBackups int
	file       *os.File
	size       int64
}

func newRotatingWriter(path string, maxSizeMB, maxBackups int) (*rotatingWriter, error) {
	if maxSizeMB <= 0 {
		maxSizeMB = 50
	}
	if maxBackups <= 0 {
		maxBackups = 5
	}
	w := &rotatingWriter{path: path, maxSize: int64(maxSizeMB) << 20, maxBackups: maxBackups}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) rotate() {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	for i := w.maxBackups - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", w.path, i)
		next := fmt.Sprintf("%s.%d", w.path, i+1)
		_ = os.Rename(old, next)
	}
	_ = os.Rename(w.path, w.path+".1")
	_ = w.open()
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.size+int64(len(p)) > w.maxSize {
		w.rotate()
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// ---- duplicate line sampler ----

type sampleWriter struct {
	inner io.Writer
	mu    sync.Mutex
	seen  map[uint64]time.Time
}

func (s *sampleWriter) Write(p []byte) (int, error) {
	h := fnv.New64a()
	_, _ = h.Write(p)
	key := h.Sum64()
	now := time.Now()
	s.mu.Lock()
	last, ok := s.seen[key]
	duplicate := ok && now.Sub(last) < time.Second
	if len(s.seen) > 4096 {
		for k, t := range s.seen {
			if now.Sub(t) > time.Minute {
				delete(s.seen, k)
			}
		}
	}
	if !duplicate {
		s.seen[key] = now
	}
	s.mu.Unlock()
	if duplicate {
		return len(p), nil
	}
	return s.inner.Write(p)
}
