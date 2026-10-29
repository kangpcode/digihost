// DigiPanel backup engine tests.
// These create real encrypted archives on disk, verify the digest and every
// authentication tag, restore into a fresh directory, and prove that a tampered
// archive and a traversal entry are both refused.

package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestEngine(t *testing.T) (*Engine, string, string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "homes", "acme_client")
	if err := os.MkdirAll(filepath.Join(source, "public_html", "assets"), 0750); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "public_html", "index.php"), []byte("<?php echo 'ok';\n"), 0640); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "public_html", "assets", "app.css"), []byte("body{color:#111}\n"), 0640); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	engine := NewEngine(Options{
		Root:       filepath.Join(root, "archives"),
		Passphrase: "correct horse battery staple",
		HomeBase:   filepath.Join(root, "homes"),
	})
	return engine, source, filepath.Join(root, "restore")
}

func TestCreateListVerifyRestoreRoundTrip(t *testing.T) {
	engine, source, restoreDir := newTestEngine(t)
	ctx := context.Background()

	archive, err := engine.Create(ctx, "acme_client", source, "nightly")
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	if archive.Entries < 3 {
		t.Fatalf("expected the directory tree in the archive, got %d entries", archive.Entries)
	}
	if archive.SHA256 == "" || archive.SizeBytes == 0 || archive.PlainBytes == 0 {
		t.Fatalf("manifest is incomplete: %+v", archive)
	}

	listed, err := engine.List("acme_client")
	if err != nil || len(listed) != 1 || listed[0].ID != archive.ID {
		t.Fatalf("list: %v %+v", err, listed)
	}

	if err := engine.Verify("acme_client", archive.ID); err != nil {
		t.Fatalf("verify: %v", err)
	}

	result, err := engine.Restore(ctx, "acme_client", archive.ID, restoreDir)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if result.Files != 2 {
		t.Fatalf("expected 2 restored files, got %d", result.Files)
	}

	restored, err := os.ReadFile(filepath.Join(restoreDir, "public_html", "index.php"))
	if err != nil || string(restored) != "<?php echo 'ok';\n" {
		t.Fatalf("restored content mismatch: %q %v", restored, err)
	}
	if _, err := os.Stat(filepath.Join(restoreDir, "public_html", "assets", "app.css")); err != nil {
		t.Fatalf("nested file missing after restore: %v", err)
	}

	after, err := engine.List("acme_client")
	if err != nil || after[0].RestoreCount != 1 {
		t.Fatalf("restore was not recorded in the manifest: %+v %v", after, err)
	}
}

func TestArchiveIsEncryptedAtRest(t *testing.T) {
	engine, source, _ := newTestEngine(t)
	archive, err := engine.Create(context.Background(), "acme_client", source, "")
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(engine.Root(), "acme_client", archive.ID+archiveSuffix))
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if len(raw) < headerSize || string(raw[:len(magicPrefix)]) != magicPrefix {
		t.Fatal("archive is not wrapped in the encrypted container")
	}
	// The plaintext marker must not appear anywhere in the encrypted bytes.
	if containsBytes(raw, []byte("echo 'ok'")) {
		t.Fatal("archive leaks plaintext content")
	}
	if containsBytes(raw, []byte("public_html/index.php")) {
		t.Fatal("archive leaks the file names in plaintext")
	}
}

func containsBytes(haystack, needle []byte) bool {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return false
	}
	for index := 0; index+len(needle) <= len(haystack); index++ {
		match := true
		for offset := range needle {
			if haystack[index+offset] != needle[offset] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestTamperedArchiveFailsVerification(t *testing.T) {
	engine, source, _ := newTestEngine(t)
	archive, err := engine.Create(context.Background(), "acme_client", source, "")
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	path := filepath.Join(engine.Root(), "acme_client", archive.ID+archiveSuffix)
	file, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	if _, err := file.WriteAt([]byte{0xFF, 0x00, 0x11}, int64(headerSize+8)); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	_ = file.Close()

	if err := engine.Verify("acme_client", archive.ID); err == nil {
		t.Fatal("tampered archive passed verification")
	}
	if _, err := engine.Restore(context.Background(), "acme_client", archive.ID, t.TempDir()); err == nil {
		t.Fatal("tampered archive was restored")
	}
}

func TestRestoreRefusesTraversalEntry(t *testing.T) {
	engine, _, restoreDir := newTestEngine(t)
	root := engine.Root()
	dir := filepath.Join(root, "acme_client")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	id := "bk-20260101T000000-abcdef"
	path := filepath.Join(dir, id+archiveSuffix)

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create malicious archive: %v", err)
	}
	writer, err := newEncryptedWriter(file, engine.opts.Passphrase)
	if err != nil {
		t.Fatalf("encrypting writer: %v", err)
	}
	gzipWriter := gzip.NewWriter(writer)
	tarWriter := tar.NewWriter(gzipWriter)
	body := []byte("escaped")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "../escaped.txt", Mode: 0640, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tarWriter.Write(body); err != nil {
		t.Fatalf("write body: %v", err)
	}
	_ = tarWriter.Close()
	_ = gzipWriter.Close()
	_ = writer.Close()
	_ = file.Close()

	digest, size, err := hashFile(path)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	record := manifest{
		Version:      manifestVersion,
		ID:           id,
		Account:      "acme_client",
		CreatedAt:    time.Now().UTC(),
		ArchiveBytes: size,
		SHA256:       digest,
		Encryption:   "aes-256-gcm+pbkdf2-sha256",
		Format:       "tar+gzip",
	}
	if err := writeManifest(filepath.Join(dir, id+manifestSuffix), record); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, err = engine.Restore(context.Background(), "acme_client", id, restoreDir)
	if !errors.Is(err, ErrUnsafeEntry) {
		t.Fatalf("expected ErrUnsafeEntry, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(restoreDir), "escaped.txt")); err == nil {
		t.Fatal("traversal entry escaped the restore target")
	}
}

func TestCreateRequiresPassphraseAndConfinedSource(t *testing.T) {
	root := t.TempDir()
	unconfigured := NewEngine(Options{Root: filepath.Join(root, "archives")})
	source := filepath.Join(root, "homes", "acme_client")
	_ = os.MkdirAll(source, 0750)
	if _, err := unconfigured.Create(context.Background(), "acme_client", source, ""); !errors.Is(err, ErrNoPassphrase) {
		t.Fatalf("expected ErrNoPassphrase, got %v", err)
	}

	engine := NewEngine(Options{
		Root:       filepath.Join(root, "archives"),
		Passphrase: "passphrase",
		HomeBase:   filepath.Join(root, "homes"),
	})
	outside := filepath.Join(root, "elsewhere")
	_ = os.MkdirAll(outside, 0750)
	if _, err := engine.Create(context.Background(), "acme_client", outside, ""); !errors.Is(err, ErrSourceEscaped) {
		t.Fatalf("expected ErrSourceEscaped, got %v", err)
	}
}

func TestPruneKeepsNewest(t *testing.T) {
	engine, source, _ := newTestEngine(t)
	ctx := context.Background()
	for index := 0; index < 3; index++ {
		if _, err := engine.Create(ctx, "acme_client", source, ""); err != nil {
			t.Fatalf("create %d: %v", index, err)
		}
	}
	removed, err := engine.Prune("acme_client", 1)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 pruned archives, got %d", removed)
	}
	remaining, err := engine.List("acme_client")
	if err != nil || len(remaining) != 1 {
		t.Fatalf("expected one archive left, got %d (%v)", len(remaining), err)
	}
}

func TestScheduleComputesWindowsAndPersists(t *testing.T) {
	engine, _, _ := newTestEngine(t)
	base := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC) // Wednesday
	engine.now = func() time.Time { return base }

	schedule, err := engine.SaveSchedule(Schedule{
		Account:   "acme_client",
		Label:     "nightly",
		Frequency: "daily",
		Hour:      2,
		Retention: 7,
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("save schedule: %v", err)
	}
	if schedule.NextRun.Hour() != 2 || !schedule.NextRun.After(base) {
		t.Fatalf("unexpected next run %s", schedule.NextRun)
	}

	stored, err := engine.Schedules()
	if err != nil || len(stored) != 1 || stored[0].ID != schedule.ID {
		t.Fatalf("schedule was not persisted: %+v %v", stored, err)
	}

	due, err := engine.Due(schedule.NextRun.Add(time.Minute))
	if err != nil || len(due) != 1 {
		t.Fatalf("schedule should be due: %+v %v", due, err)
	}
	if due, _ := engine.Due(base); len(due) != 0 {
		t.Fatal("schedule fired before its window")
	}

	advanced, err := engine.MarkScheduleRun(schedule.ID, schedule.NextRun)
	if err != nil {
		t.Fatalf("mark run: %v", err)
	}
	if advanced.LastRun == nil || !advanced.NextRun.After(schedule.NextRun) {
		t.Fatalf("schedule did not advance: %+v", advanced)
	}

	weekly, err := ComputeNextRun("weekly", 3, base)
	if err != nil || weekly.Weekday() != time.Monday {
		t.Fatalf("weekly schedule must land on Monday, got %s (%v)", weekly, err)
	}
	monthly, err := ComputeNextRun("monthly", 0, base)
	if err != nil || monthly.Day() != 1 || !monthly.After(base) {
		t.Fatalf("monthly schedule must land on the first, got %s (%v)", monthly, err)
	}
	if _, err := ComputeNextRun("hourly", 0, base); err == nil {
		t.Fatal("unsupported frequency was accepted")
	}
}

func TestRestoreRejectsUnknownArchive(t *testing.T) {
	engine, _, restoreDir := newTestEngine(t)
	if _, err := engine.Restore(context.Background(), "acme_client", "bk-20260101T000000-ffffff", restoreDir); !errors.Is(err, ErrArchiveMissing) {
		t.Fatalf("expected ErrArchiveMissing, got %v", err)
	}
}
