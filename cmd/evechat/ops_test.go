package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/crypto"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// TestBackupRestoreKeepsOldMessagesDecryptable is the A11 acceptance test: a
// message written before the backup must still decrypt after the database is
// deleted and restored from the archive. A row count alone would pass even when
// the ciphertext or the key version was mangled on the way through.
func TestBackupRestoreKeepsOldMessagesDecryptable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generating key: %v", err)
	}
	t.Setenv("EVECHAT_ENCRYPTION_KEY_V1", base64.StdEncoding.EncodeToString(key))

	cfg := config.Default()
	cfg.Database.Path = filepath.Join(dir, "data", "evechat.db")
	cfg.Storage.UploadsDir = filepath.Join(dir, "data", "uploads")
	if err := os.MkdirAll(cfg.Storage.UploadsDir, 0o750); err != nil {
		t.Fatalf("creating data dir: %v", err)
	}

	enc, err := crypto.New(map[int][]byte{1: key}, 1)
	if err != nil {
		t.Fatalf("building cipher: %v", err)
	}
	const plaintext = "این پیام پیش از پشتیبان‌گیری نوشته شده است"

	writeFixture := func() {
		d, err := db.Open(cfg)
		if err != nil {
			t.Fatalf("opening db: %v", err)
		}
		defer d.Close()
		if err := db.Migrate(ctx, d, zerolog.Nop()); err != nil {
			t.Fatalf("migrating: %v", err)
		}
		r := repo.New(d)
		project := &models.Project{
			ID: "proj_a11", Name: "A11", APIKey: "pk_a11", SecretKey: "sk_a11",
			Domains: models.StringList{}, Settings: models.JSONMap{}, IsActive: true,
		}
		if err := r.Projects.Create(ctx, project); err != nil {
			t.Fatalf("creating project: %v", err)
		}
		visitor := &models.Visitor{
			ID: "vis_a11", ProjectID: project.ID,
			SubscriptionInfo: models.JSONMap{}, Metadata: models.JSONMap{},
		}
		if err := r.Visitors.Upsert(ctx, visitor); err != nil {
			t.Fatalf("creating visitor: %v", err)
		}
		conv := &models.Conversation{
			ID: "conv_a11", ProjectID: project.ID, VisitorID: visitor.ID,
			Status: models.StatusPending, Channel: "web",
		}
		if err := r.Conversations.Create(ctx, conv); err != nil {
			t.Fatalf("creating conversation: %v", err)
		}
		ct, iv, version, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encrypting: %v", err)
		}
		msg := &models.Message{
			ID: "msg_a11", ConversationID: conv.ID, SenderType: models.SenderVisitor,
			ContentType: "text", ContentEncrypted: ct, ContentIV: iv, KeyVersion: version,
		}
		if err := r.Messages.Create(ctx, msg); err != nil {
			t.Fatalf("storing message: %v", err)
		}
		if err := os.WriteFile(filepath.Join(cfg.Storage.UploadsDir, "note.txt"), []byte("attachment"), 0o640); err != nil {
			t.Fatalf("writing upload: %v", err)
		}
	}

	writeFixture()

	archive := filepath.Join(dir, "backup", "evechat")
	if err := doBackup(ctx, cfg, archive, true, false); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if _, err := os.Stat(archive + ".tar.gz"); err != nil {
		t.Fatalf("backup archive missing: %v", err)
	}

	// Move the database out of the way: the restore must bring the data back.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(cfg.Database.Path + suffix)
	}
	if _, err := os.Stat(cfg.Database.Path); err == nil {
		t.Fatal("database was not removed before restore")
	}

	if err := doRestore(ctx, cfg, archive+".tar.gz", false); err != nil {
		t.Fatalf("restore: %v", err)
	}

	d, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("opening restored db: %v", err)
	}
	defer d.Close()
	r := repo.New(d)

	got, err := r.Messages.GetByID(ctx, "msg_a11")
	if err != nil {
		t.Fatalf("old message is missing after restore: %v", err)
	}
	decrypted, err := enc.Decrypt(got.ContentEncrypted, got.ContentIV, got.KeyVersion)
	if err != nil {
		t.Fatalf("old message does not decrypt after restore: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("decrypted message = %q, want %q", decrypted, plaintext)
	}
	if got.KeyVersion != 1 {
		t.Fatalf("key version changed during backup/restore: %d", got.KeyVersion)
	}

	if _, err := os.Stat(filepath.Join(cfg.Storage.UploadsDir, "note.txt")); err != nil {
		t.Fatalf("upload was not restored: %v", err)
	}
}
