package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/evechat/evechat/internal/config"
)

// newUpdateFixture prepares a fake installation: a data dir, a "current"
// binary and a "new" binary to install.
func newUpdateFixture(t *testing.T) (cfg *config.Config, target, incoming string) {
	t.Helper()
	dir := t.TempDir()
	cfg = config.Default()
	cfg.Database.Path = filepath.Join(dir, "data", "evechat.db")
	cfg.Storage.UploadsDir = filepath.Join(dir, "data", "uploads")

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("creating bin dir: %v", err)
	}
	target = filepath.Join(binDir, "evechat")
	if err := os.WriteFile(target, []byte("original-binary"), 0o755); err != nil {
		t.Fatalf("writing current binary: %v", err)
	}
	incoming = filepath.Join(dir, "incoming-evechat")
	if err := os.WriteFile(incoming, []byte("broken-binary"), 0o755); err != nil {
		t.Fatalf("writing new binary: %v", err)
	}
	return cfg, target, incoming
}

func stubUpdateHooks(t *testing.T, healthy bool) {
	t.Helper()
	prevRestart, prevWait := restartServiceFn, waitHealthyFn
	t.Cleanup(func() {
		restartServiceFn, waitHealthyFn = prevRestart, prevWait
	})
	restartServiceFn = func() {}
	waitHealthyFn = func(*config.Config, time.Duration) bool { return healthy }
}

// TestUpdateRollsBackWhenNewBinaryIsUnhealthy is the A12 acceptance test: a new
// binary that never serves traffic must leave the previous binary in place.
func TestUpdateRollsBackWhenNewBinaryIsUnhealthy(t *testing.T) {
	cfg, target, incoming := newUpdateFixture(t)
	stubUpdateHooks(t, false)

	err := installUpdate(context.Background(), cfg, "v9.9.9", incoming, target)
	if err == nil {
		t.Fatal("installUpdate succeeded although the new binary never became healthy")
	}
	if !strings.Contains(err.Error(), "بازگردانده") {
		t.Fatalf("error does not mention a rollback: %v", err)
	}
	got, rerr := os.ReadFile(target)
	if rerr != nil {
		t.Fatalf("reading target after rollback: %v", rerr)
	}
	if string(got) != "original-binary" {
		t.Fatalf("target after rollback = %q, want the original binary", got)
	}
	if _, serr := os.Stat(target + ".prev"); serr == nil {
		t.Fatal("stale .prev file was left behind")
	}
}

func TestUpdateKeepsNewBinaryWhenHealthy(t *testing.T) {
	cfg, target, incoming := newUpdateFixture(t)
	stubUpdateHooks(t, true)

	if err := installUpdate(context.Background(), cfg, "v9.9.9", incoming, target); err != nil {
		t.Fatalf("installUpdate: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading target: %v", err)
	}
	if string(got) != "broken-binary" {
		t.Fatalf("target = %q, want the new binary", got)
	}
}

func TestStageBinaryAcceptsArchiveAndPlainFile(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "evechat-linux-amd64")
	if err := os.WriteFile(plain, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "staged")
	if err := stageBinary(plain, out); err != nil {
		t.Fatalf("staging a plain binary: %v", err)
	}
	if data, _ := os.ReadFile(out); string(data) != "bin" {
		t.Fatalf("staged content = %q", data)
	}
	zip := filepath.Join(dir, "x.zip")
	_ = os.WriteFile(zip, []byte("zip"), 0o644)
	if err := stageBinary(zip, out); err == nil {
		t.Fatal("zip archives should be rejected with a hint")
	}
}
