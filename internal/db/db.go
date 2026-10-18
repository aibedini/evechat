package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/evechat/evechat/internal/config"
	"github.com/rs/zerolog"

	_ "modernc.org/sqlite"
)

// DB holds two pools: a single-writer pool (SQLite allows one writer) and a
// read pool sized for concurrent readers under WAL.
type DB struct {
	W    *sql.DB
	R    *sql.DB
	path string

	stopOnce sync.Once
	stop     chan struct{}
	log      zerolog.Logger
}

func dsn(path string, readOnly bool, busyTimeoutMS int) string {
	p := strings.ReplaceAll(path, `\`, "/")
	if !strings.HasPrefix(p, "file:") {
		p = "file:" + p
	}
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMS))
	q.Add("_pragma", "cache_size(-8000)")
	q.Add("_pragma", "temp_store(MEMORY)")
	if readOnly {
		q.Set("mode", "ro")
	}
	return p + "?" + q.Encode()
}

// Open opens both pools and verifies connectivity.
func Open(cfg *config.Config) (*DB, error) {
	path := cfg.Database.Path
	if path != ":memory:" && strings.Contains(path, ":memory:") {
		path = "file::memory:?cache=shared"
	}
	busy := cfg.Database.BusyTimeoutMS
	if busy <= 0 {
		busy = 5000
	}

	writerDSN := dsn(path, false, busy) + "&_txlock=immediate"
	w, err := sql.Open("sqlite", writerDSN)
	if err != nil {
		return nil, fmt.Errorf("opening writer pool: %w", err)
	}
	maxW := cfg.Database.MaxOpenConns
	if maxW <= 0 {
		maxW = 1
	}
	w.SetMaxOpenConns(maxW)
	w.SetMaxIdleConns(1)
	w.SetConnMaxLifetime(0)

	readerDSN := dsn(path, false, busy)
	r, err := sql.Open("sqlite", readerDSN)
	if err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("opening reader pool: %w", err)
	}
	maxR := cfg.Database.MaxReadConns
	if maxR <= 0 {
		maxR = 4
	}
	r.SetMaxOpenConns(maxR)
	r.SetMaxIdleConns(maxR)
	r.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := w.PingContext(ctx); err != nil {
		_ = w.Close()
		_ = r.Close()
		return nil, fmt.Errorf("pinging writer pool: %w", err)
	}
	if err := r.PingContext(ctx); err != nil {
		_ = w.Close()
		_ = r.Close()
		return nil, fmt.Errorf("pinging reader pool: %w", err)
	}

	d := &DB{W: w, R: r, path: path, stop: make(chan struct{})}
	d.optimize()
	go d.optimizeLoop()
	return d, nil
}

// Path returns the database path.
func (d *DB) Path() string { return d.path }

// Tx runs fn inside a write transaction. BEGIN IMMEDIATE is requested through
// the driver's _txlock=immediate connection option, so concurrent writers
// fail fast instead of deadlocking on a lock upgrade.
func (d *DB) Tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.W.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func (d *DB) optimize() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := d.W.ExecContext(ctx, "PRAGMA optimize"); err != nil {
		// Optimize is best-effort.
		return
	}
}

func (d *DB) optimizeLoop() {
	t := time.NewTicker(6 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			d.optimize()
		case <-d.stop:
			return
		}
	}
}

// Close stops background work and closes both pools.
func (d *DB) Close() error {
	d.stopOnce.Do(func() { close(d.stop) })
	d.optimize()
	errW := d.W.Close()
	errR := d.R.Close()
	if errW != nil {
		return errW
	}
	return errR
}

// Backup writes a consistent copy of the database using VACUUM INTO.
func (d *DB) Backup(ctx context.Context, dest string) error {
	if _, err := d.W.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		return fmt.Errorf("vacuum into %s: %w", dest, err)
	}
	return nil
}

// Dir returns the directory that contains the database file.
func (d *DB) Dir() string {
	if strings.HasPrefix(d.path, "file:") {
		return ""
	}
	return filepath.Dir(d.path)
}
