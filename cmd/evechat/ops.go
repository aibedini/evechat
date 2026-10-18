package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/crypto"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/logx"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/setup"
)

// ---- migrate ----

func runMigrateCommand(args []string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	_ = fs.Parse(args)
	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	cfg := mustConfig(cfgPath)
	d := mustDB(cfg)
	defer d.Close()
	if err := db.Migrate(context.Background(), d, *logx.L()); err != nil {
		fatalf("%v", err)
	}
	fmt.Println("مهاجرت‌ها با موفقیت اجرا شدند.")
}

// ---- backup ----

type backupManifest struct {
	Version       string         `json:"version"`
	SchemaVersion int            `json:"schema_version"`
	CreatedAt     time.Time      `json:"created_at"`
	InstanceID    string         `json:"instance_id"`
	DBSHA256      string         `json:"db_sha256"`
	Counts        map[string]int `json:"counts"`
	WithUploads   bool           `json:"with_uploads"`
	Encrypted     bool           `json:"encrypted"`
}

func runBackupCommand(args []string) {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	withUploads := fs.Bool("with-uploads", false, "شامل پیوست‌ها")
	keep := fs.Int("keep", 0, "نگه‌داشتن N نسخه اخیر")
	encrypt := fs.Bool("encrypt", false, "رمزگذاری با عبارت عبور")
	rest := positional(fs, args)
	if len(rest) < 1 {
		fatalf("مسیر مقصد لازم است: evechat backup /backup/evechat-2026-10-18")
	}
	cfg := mustConfig(cfgPath)
	if err := doBackup(context.Background(), cfg, rest[0], *withUploads, *encrypt); err != nil {
		fatalf("%v", err)
	}
	if *keep > 0 {
		if err := pruneBackups(filepath.Dir(rest[0]), *keep); err != nil {
			fmt.Fprintln(os.Stderr, "هشدار در هرس نسخه‌های قدیمی:", err)
		}
	}
}

func doBackup(ctx context.Context, cfg *config.Config, dest string, withUploads, encrypt bool) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	d, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer d.Close()

	tmpDB := dest + ".db"
	_ = os.Remove(tmpDB)
	if err := d.Backup(ctx, tmpDB); err != nil {
		return fmt.Errorf("creating consistent copy: %w", err)
	}
	manifest := backupManifest{
		Version:     version,
		CreatedAt:   time.Now().UTC(),
		WithUploads: withUploads,
		Encrypted:   encrypt,
		Counts:      map[string]int{},
	}
	if latest, err := db.LatestMigrationVersion(); err == nil {
		manifest.SchemaVersion = latest
	}
	if v, ok, _ := repo.New(d).Settings.Get(ctx, "instance_id"); ok {
		manifest.InstanceID = v
	}
	var projectCount, agentCount, convCount, msgCount int
	_ = d.R.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&projectCount)
	_ = d.R.QueryRowContext(ctx, "SELECT COUNT(*) FROM agents").Scan(&agentCount)
	_ = d.R.QueryRowContext(ctx, "SELECT COUNT(*) FROM conversations").Scan(&convCount)
	_ = d.R.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages").Scan(&msgCount)
	manifest.Counts["projects"] = projectCount
	manifest.Counts["agents"] = agentCount
	manifest.Counts["conversations"] = convCount
	manifest.Counts["messages"] = msgCount

	dbBytes, err := os.ReadFile(tmpDB)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(dbBytes)
	manifest.DBSHA256 = hex.EncodeToString(sum[:])

	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")

	if !withUploads {
		_ = os.Remove(tmpDB)
		if err := os.WriteFile(dest+".db", dbBytes, 0o640); err != nil {
			return err
		}
		if err := os.WriteFile(dest+".manifest.json", manifestBytes, 0o640); err != nil {
			return err
		}
		fmt.Printf("پشتیبان ساخته شد: %s.db (%d پروژه، %d پیام)\n", dest, manifest.Counts["projects"], manifest.Counts["messages"])
		fmt.Println("یادآوری: کلید رمزنگاری جداگانه ذخیره شده است و برای خواندن پیام‌ها لازم است.")
		return nil
	}

	// Build the tar.gz in memory-friendly streaming mode.
	if err := writeArchive(dest+".tar.gz", tmpDB, cfg.Storage.UploadsDir, manifestBytes); err != nil {
		return err
	}
	_ = os.Remove(tmpDB)
	if encrypt {
		if err := encryptFile(dest + ".tar.gz"); err != nil {
			return err
		}
	}
	fmt.Printf("پشتیبان کامل ساخته شد: %s.tar.gz\n", dest)
	fmt.Println("یادآوری: کلید رمزنگاری جداگانه ذخیره شده است و برای خواندن پیام‌ها لازم است.")
	return nil
}

func writeArchive(outPath, dbPath, uploadsDir string, manifest []byte) error {
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	add := func(name string, data []byte, mode int64) error {
		hdr := &tar.Header{Name: name, Mode: mode, Size: int64(len(data)), ModTime: time.Now()}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	if err := add("manifest.json", manifest, 0o640); err != nil {
		return err
	}
	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		return err
	}
	if err := add("evechat.db", dbData, 0o640); err != nil {
		return err
	}
	root := strings.TrimRight(uploadsDir, string(os.PathSeparator))
	_ = filepath.Walk(uploadsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		return add(filepath.ToSlash(filepath.Join("uploads", rel)), data, 0o640)
	})
	return nil
}

func encryptFile(path string) error {
	pass := os.Getenv("EVECHAT_BACKUP_PASSPHRASE")
	if pass == "" {
		fmt.Print("عبارت عبور پشتیبان: ")
		var input string
		fmt.Scanln(&input)
		pass = input
	}
	if len(pass) < 8 {
		return fmt.Errorf("عبارت عبور باید حداقل ۸ کاراکتر باشد")
	}
	plain, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	keySum := sha256.Sum256(append(salt, []byte(pass)...))
	block, err := aes.NewCipher(keySum[:])
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	sealed := gcm.Seal(nil, nonce, plain, nil)
	final := append([]byte("EVECHAT1"), salt...)
	final = append(final, nonce...)
	final = append(final, sealed...)
	if err := os.WriteFile(path, final, 0o600); err != nil {
		return err
	}
	return nil
}

func pruneBackups(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type fileInfo struct {
		path string
		mod  time.Time
	}
	var backups []fileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".db") && !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, fileInfo{path: filepath.Join(dir, name), mod: info.ModTime()})
	}
	if len(backups) <= keep {
		return nil
	}
	// Keep the newest `keep` files.
	for i := 0; i < len(backups); i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[j].mod.After(backups[i].mod) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}
	for _, f := range backups[keep:] {
		_ = os.Remove(f.path)
	}
	return nil
}

// ---- restore ----

func runRestoreCommand(args []string) {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	dryRun := fs.Bool("dry-run", false, "فقط نمایش تغییرات")
	rest := positional(fs, args)
	if len(rest) < 1 {
		fatalf("مسیر آرشیو لازم است: evechat restore /backup/evechat-2026-10-18.tar.gz")
	}
	cfg := mustConfig(cfgPath)
	if err := doRestore(context.Background(), cfg, rest[0], *dryRun); err != nil {
		fatalf("%v", err)
	}
}

func doRestore(ctx context.Context, cfg *config.Config, archive string, dryRun bool) error {
	if lockExists(cfg.DataDir()) {
		return fmt.Errorf("سرور در حال اجراست؛ ابتدا سرویس را متوقف کنید (systemctl stop evechat)")
	}
	ext := filepath.Ext(archive)
	var manifestBytes, dbBytes []byte
	if ext == ".gz" {
		r, closer, err := openArchive(archive)
		if err != nil {
			return err
		}
		defer closer()
		for {
			hdr, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			switch hdr.Name {
			case "manifest.json":
				manifestBytes, _ = io.ReadAll(r)
			case "evechat.db":
				dbBytes, _ = io.ReadAll(r)
			default:
				if dryRun {
					fmt.Println("  +", hdr.Name)
				}
			}
		}
	} else {
		data, err := os.ReadFile(archive)
		if err != nil {
			return err
		}
		dbBytes = data
		manifestBytes, _ = os.ReadFile(strings.TrimSuffix(archive, ".db") + ".manifest.json")
	}
	if len(dbBytes) == 0 {
		return fmt.Errorf("آرشیو حاوی evechat.db نیست")
	}
	var manifest backupManifest
	if len(manifestBytes) > 0 {
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			return fmt.Errorf("manifest نامعتبر است: %w", err)
		}
		if manifest.DBSHA256 != "" {
			sum := sha256.Sum256(dbBytes)
			if hex.EncodeToString(sum[:]) != manifest.DBSHA256 {
				return fmt.Errorf("checksum پایگاه داده مطابقت ندارد")
			}
		}
		if latest, _ := db.LatestMigrationVersion(); manifest.SchemaVersion > latest {
			return fmt.Errorf("نسخه schema پشتیبان (%d) جدیدتر از این باینری (%d) است", manifest.SchemaVersion, latest)
		}
	}
	if dryRun {
		fmt.Printf("بازیابی %s به %s\n", archive, cfg.Database.Path)
		fmt.Printf("پروژه‌ها: %d، پیام‌ها: %d\n", manifest.Counts["projects"], manifest.Counts["messages"])
		fmt.Println("هیچ تغییری اعمال نشد (--dry-run).")
		return nil
	}

	if err := os.MkdirAll(cfg.DataDir(), 0o750); err != nil {
		return err
	}
	if _, err := os.Stat(cfg.Database.Path); err == nil {
		backupPath := filepath.Join(cfg.DataDir(), fmt.Sprintf("evechat.pre-restore-%s.db", time.Now().UTC().Format("20060102-150405")))
		if err := os.WriteFile(backupPath, dbBytes, 0o640); err != nil {
			return err
		}
		_ = os.Rename(cfg.Database.Path, backupPath+".current")
		fmt.Println("نسخه فعلی در", backupPath+".current", "نگهداری شد")
	}
	if err := os.WriteFile(cfg.Database.Path, dbBytes, 0o640); err != nil {
		return err
	}
	if ext == ".gz" {
		if err := extractUploads(archive, cfg.Storage.UploadsDir); err != nil {
			fmt.Fprintln(os.Stderr, "هشدار در استخراج پیوست‌ها:", err)
		}
	}
	d, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := db.Migrate(ctx, d, *logx.L()); err != nil {
		return err
	}
	fmt.Println("بازیابی با موفقیت انجام شد؛ سرویس را دوباره اجرا کنید.")
	return nil
}

func openArchive(path string) (*tar.Reader, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	return tar.NewReader(gz), func() { _ = gz.Close(); _ = f.Close() }, nil
}

func extractUploads(archive, uploadsDir string) error {
	r, closer, err := openArchive(archive)
	if err != nil {
		return err
	}
	defer closer()
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !strings.HasPrefix(hdr.Name, "uploads/") || hdr.FileInfo().IsDir() {
			continue
		}
		clean := filepath.Clean(hdr.Name)
		if strings.Contains(clean, "..") {
			return fmt.Errorf("مسیر ناامن در آرشیو: %s", hdr.Name)
		}
		dest := filepath.Join(filepath.Dir(uploadsDir), clean)
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, r)
		_ = out.Close()
		if err != nil {
			return err
		}
	}
}

func lockExists(dataDir string) bool {
	path := filepath.Join(dataDir, "evechat.lock")
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < 30*time.Second
}

// ---- update ----

func runUpdateCommand(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	checkOnly := fs.Bool("check", false, "فقط بررسی وجود نسخه جدید")
	target := fs.String("version", "", "نسخه هدف (مثلا v1.1.0)")
	from := fs.String("from", "", "آرشیو یا باینری محلی نسخه جدید (بدون شبکه)")
	_ = fs.Parse(args)

	// --from installs a release that was fetched by hand: no GitHub call, so it
	// also works on air-gapped installations.
	if *from != "" {
		if *checkOnly {
			fatalf("--check با --from معنی ندارد")
		}
		tag := *target
		if tag == "" {
			tag = "local"
		}
		if err := doUpdateFromFile(context.Background(), mustConfig(cfgPath), tag, *from); err != nil {
			fatalf("%v", err)
		}
		return
	}

	latest, err := latestRelease(*target)
	if err != nil {
		fatalf("دریافت اطلاعات نسخه: %v", err)
	}
	if *checkOnly {
		fmt.Printf("نسخه فعلی: %s\nآخرین نسخه: %s\n", version, latest.Tag)
		if latest.Tag == version || "v"+version == latest.Tag {
			fmt.Println("شما از آخرین نسخه استفاده می‌کنید.")
		}
		return
	}
	if latest.Tag == version || "v"+version == latest.Tag {
		fmt.Println("نسخه جدیدی موجود نیست.")
		return
	}
	cfg := mustConfig(cfgPath)
	if err := doUpdate(context.Background(), cfg, latest); err != nil {
		fatalf("%v", err)
	}
}

type releaseInfo struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func latestRelease(explicit string) (*releaseInfo, error) {
	url := "https://api.github.com/repos/aibedini/evechat/releases/latest"
	tag := explicit
	if tag != "" {
		url = "https://api.github.com/repos/aibedini/evechat/releases/tags/" + tag
	}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API status %d", resp.StatusCode)
	}
	var rel releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	if rel.Tag == "" {
		return nil, fmt.Errorf("نسخه‌ای یافت نشد")
	}
	return &rel, nil
}

func doUpdate(ctx context.Context, cfg *config.Config, rel *releaseInfo) error {
	assetName := fmt.Sprintf("evechat_%s_%s_%s", strings.TrimPrefix(rel.Tag, "v"), runtimeGOOS(), runtimeGOARCH())
	if runtimeGOOS() == "windows" {
		assetName += ".zip"
	} else {
		assetName += ".tar.gz"
	}
	var assetURL, checksumURL string
	for _, a := range rel.Assets {
		if a.Name == assetName {
			assetURL = a.URL
		}
		if a.Name == "checksums.txt" {
			checksumURL = a.URL
		}
	}
	if assetURL == "" {
		return fmt.Errorf("فایل %s در نسخه %s پیدا نشد", assetName, rel.Tag)
	}
	tmp, err := os.MkdirTemp("", "evechat-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	archivePath := filepath.Join(tmp, assetName)
	if err := download(assetURL, archivePath); err != nil {
		return err
	}
	if checksumURL != "" {
		sumPath := filepath.Join(tmp, "checksums.txt")
		if err := download(checksumURL, sumPath); err == nil {
			if err := verifyChecksum(archivePath, sumPath); err != nil {
				return err
			}
		}
	}
	current, err := os.Executable()
	if err != nil {
		return err
	}
	return installUpdate(ctx, cfg, rel.Tag, archivePath, current)
}

// doUpdateFromFile installs a release that was fetched by hand. It is the
// offline/air-gapped path and the one the rollback test drives.
func doUpdateFromFile(ctx context.Context, cfg *config.Config, tag, path string) error {
	current, err := os.Executable()
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	return installUpdate(ctx, cfg, tag, abs, current)
}

// installUpdate backs the database up, swaps targetPath for the new binary,
// restarts the service and restores the previous binary when the new one never
// becomes healthy.
func installUpdate(ctx context.Context, cfg *config.Config, tag, sourcePath, targetPath string) error {
	fmt.Println("پشتیبان‌گیری قبل از به‌روزرسانی…")
	backupBase := filepath.Join(cfg.DataDir(), "pre-update-"+time.Now().UTC().Format("20060102-150405"))
	if err := doBackup(ctx, cfg, backupBase, false, false); err != nil {
		return fmt.Errorf("پشتیبان‌گیری ناموفق؛ به‌روزرسانی متوقف شد: %w", err)
	}

	newBinary := sourcePath + ".new"
	if err := stageBinary(sourcePath, newBinary); err != nil {
		return err
	}
	defer os.Remove(newBinary)

	prev := targetPath + ".prev"
	_ = os.Remove(prev)
	if err := os.Rename(targetPath, prev); err != nil {
		return fmt.Errorf("جابه‌جایی باینری فعلی: %w", err)
	}
	if err := copyFile(newBinary, targetPath); err != nil {
		_ = os.Rename(prev, targetPath)
		return err
	}
	_ = os.Chmod(targetPath, 0o755)
	fmt.Println("باینری جایگزین شد؛ ری‌استارت سرویس…")
	restartServiceFn()
	if !waitHealthyFn(cfg, 20*time.Second) {
		fmt.Fprintln(os.Stderr, "سلامت پس از به‌روزرسانی برقرار نشد؛ بازگردانی نسخه قبلی…")
		_ = os.Remove(targetPath)
		_ = os.Rename(prev, targetPath)
		restartServiceFn()
		return fmt.Errorf("به‌روزرسانی ناموفق بود و نسخه قبلی بازگردانده شد")
	}
	fmt.Printf("به‌روزرسانی به %s با موفقیت انجام شد.\n", tag)
	return nil
}

// stageBinary accepts a release archive or a plain binary file.
func stageBinary(sourcePath, dest string) error {
	if strings.HasSuffix(sourcePath, ".tar.gz") || strings.HasSuffix(sourcePath, ".tgz") {
		return extractBinary(sourcePath, dest)
	}
	if strings.HasSuffix(sourcePath, ".zip") {
		return fmt.Errorf("آرشیو zip پشتیبانی نمی‌شود؛ فایل اجرایی را مستقیم بدهید")
	}
	return copyFile(sourcePath, dest)
}

// restartServiceFn and waitHealthyFn are seams for the rollback tests.
var (
	restartServiceFn = restartService
	waitHealthyFn    = waitHealthy
)

func extractBinary(archivePath, dest string) error {
	if strings.HasSuffix(archivePath, ".zip") {
		return fmt.Errorf("به‌روزرسانی خودکار در ویندوز پشتیبانی نمی‌شود؛ آرشیو را دستی جایگزین کنید")
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) != "evechat" {
			continue
		}
		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, tr)
		_ = out.Close()
		if err != nil {
			return err
		}
		return os.Chmod(dest, 0o755)
	}
	return fmt.Errorf("باینری در آرشیو یافت نشد")
}

func download(url, dest string) error {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+os.Getenv("GITHUB_TOKEN"))
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("دانلود %s: status %d", url, resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func verifyChecksum(file, sumsPath string) error {
	data, err := os.ReadFile(sumsPath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(mustRead(file))
	actual := hex.EncodeToString(sum[:])
	base := filepath.Base(file)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == base {
			if fields[0] != actual {
				return fmt.Errorf("checksum مطابقت ندارد: %s", base)
			}
			return nil
		}
	}
	return fmt.Errorf("checksum برای %s یافت نشد", base)
}

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

func waitHealthy(cfg *config.Config, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if probeHealth(cfg) == 0 {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

// ---- encryption keys ----

func runKeyCommand(args []string) {
	fs := flag.NewFlagSet("key", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	_ = fs.Parse(args)
	sub := "status"
	if remaining := fs.Args(); len(remaining) > 0 {
		sub = remaining[0]
	}
	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	cfg := mustConfig(cfgPath)
	d := mustDB(cfg)
	defer d.Close()

	switch sub {
	case "status":
		rows, err := d.R.Query(`SELECT key_version, COUNT(*) FROM messages GROUP BY key_version ORDER BY key_version`)
		if err != nil {
			fatalf("%v", err)
		}
		defer rows.Close()
		keys, _ := cfg.EncryptionKeys()
		fmt.Println("نسخه جاری:", cfg.Encryption.CurrentVersion)
		fmt.Println("کلیدهای بارگذاری‌شده:")
		for v := range keys {
			fmt.Println("  - v" + strconv.Itoa(v))
		}
		fmt.Println("پیام‌ها به تفکیک نسخه:")
		for rows.Next() {
			var v, n int
			_ = rows.Scan(&v, &n)
			fmt.Printf("  - v%d: %d پیام\n", v, n)
		}
	case "rotate":
		if err := rotateKeys(context.Background(), cfg, d); err != nil {
			fatalf("%v", err)
		}
	default:
		fatalf("زیردستور نامعتبر: %s (status|rotate)", sub)
	}
}

func rotateKeys(ctx context.Context, cfg *config.Config, d *db.DB) error {
	keys, err := cfg.EncryptionKeys()
	if err != nil {
		return err
	}
	current := cfg.Encryption.CurrentVersion
	next := current + 1
	nextKey, ok := keys[next]
	if !ok {
		return fmt.Errorf("کلید نسخه %d تنظیم نشده است. ابتدا EVECHAT_ENCRYPTION_KEY_V%d را در محیط قرار دهید", next, next)
	}
	if len(nextKey) != 32 {
		return fmt.Errorf("کلید نسخه %d باید ۳۲ بایت باشد", next)
	}
	enc, err := crypto.New(keys, next)
	if err != nil {
		return err
	}
	r := repo.New(d)
	rows, err := d.R.QueryContext(ctx, `SELECT id, content_encrypted, content_iv, key_version FROM messages WHERE key_version != ?`, next)
	if err != nil {
		return err
	}
	type row struct {
		id, ct, iv string
		ver        int
	}
	var batch []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.ct, &x.iv, &x.ver); err != nil {
			return err
		}
		batch = append(batch, x)
	}
	rows.Close()
	fmt.Printf("بازرمزگذاری %d پیام از نسخه %d به %d…\n", len(batch), current, next)
	for _, x := range batch {
		newCT, newIV, newVer, err := enc.ReEncrypt(x.ct, x.iv, x.ver)
		if err != nil {
			return fmt.Errorf("re-encrypt %s: %w", x.id, err)
		}
		if _, err := d.W.ExecContext(ctx, `UPDATE messages SET content_encrypted = ?, content_iv = ?, key_version = ? WHERE id = ?`,
			newCT, newIV, newVer, x.id); err != nil {
			return err
		}
	}
	_ = r.Settings.Set(ctx, "encryption.current_version", strconv.Itoa(next))
	fmt.Printf("بازرمزگذاری کامل شد. اکنون encryption.current_version را در YAML به %d تغییر دهید و EVECHAT_ENCRYPTION_KEY_V%d را حذف کنید.\n", next, current)
	return nil
}

// ---- misc ----

func runHealthcheckCommand(args []string) {
	fs := flag.NewFlagSet("healthcheck", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	_ = fs.Parse(args)
	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	os.Exit(probeHealth(cfg))
}

func mustConfig(path string) *config.Config {
	cfg, err := config.Load(path)
	if err != nil {
		fatalf("%v", err)
	}
	return cfg
}

func mustDB(cfg *config.Config) *db.DB {
	d, err := db.Open(cfg)
	if err != nil {
		fatalf("%v", err)
	}
	return d
}

// positional separates positional arguments from flags. Go's flag package stops
// at the first non-flag token, which would silently ignore flags written after a
// path (e.g. `evechat backup /backup --with-uploads`), so parse in rounds.
func positional(fs *flag.FlagSet, args []string) []string {
	var out []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			fatalf("%v", err)
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		out = append(out, rest[0])
		args = rest[1:]
	}
	return out
}

// buildInfoForManifest keeps go vet happy about unused imports in some builds.
var _ = models.NowStr
var _ = base64.StdEncoding
