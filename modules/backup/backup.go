// DigiPanel Backup Engine (modules/backup/backup.go)
// Real tar+gzip archives encrypted with AES-256-GCM, a SHA-256 archive digest,
// a manifest per archive, and restore that refuses any entry escaping the target
// directory. Retention pruning keeps the newest N archives per account.
// Zero emojis, strictly production grade.

package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	manifestVersion = 1
	manifestSuffix  = ".json"
	archiveSuffix   = ".dpb"
	maxLabelLength  = 64
)

var (
	ErrNoPassphrase   = errors.New("backup encryption passphrase is not configured: set DIGIPANEL_BACKUP_PASSPHRASE")
	ErrArchiveMissing = errors.New("backup archive not found")
	ErrSourceMissing  = errors.New("backup source directory does not exist")
	ErrSourceEscaped  = errors.New("backup source directory is outside the configured home base")
	ErrChecksum       = errors.New("backup archive checksum does not match its manifest")
	ErrUnsafeEntry    = errors.New("backup archive contains an entry that escapes the restore target")
	ErrInvalidAccount = errors.New("account name must match ^[a-z][a-z0-9_]{2,31}$")
	ErrInvalidLabel   = errors.New("backup label must be 1-64 printable characters")
)

var (
	accountPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)
	idPattern      = regexp.MustCompile(`^bk-[0-9]{8}T[0-9]{6}-[a-f0-9]{6}$`)
	controlPattern = regexp.MustCompile(`[\x00-\x1f\x7f]`)
)

// Options configures the backup engine.
type Options struct {
	Root       string
	Passphrase string
	HomeBase   string
}

// Archive is the public description of one stored backup.
type Archive struct {
	ID           string     `json:"id"`
	Account      string     `json:"account"`
	Label        string     `json:"label"`
	CreatedAt    time.Time  `json:"createdAt"`
	SizeBytes    int64      `json:"sizeBytes"`
	PlainBytes   int64      `json:"plainBytes"`
	Entries      int        `json:"entries"`
	SHA256       string     `json:"sha256"`
	Encryption   string     `json:"encryption"`
	Format       string     `json:"format"`
	RestoredAt   *time.Time `json:"restoredAt,omitempty"`
	RestoreCount int        `json:"restoreCount"`
}

type manifest struct {
	Version      int        `json:"version"`
	ID           string     `json:"id"`
	Account      string     `json:"account"`
	Label        string     `json:"label"`
	CreatedAt    time.Time  `json:"createdAt"`
	SourceDir    string     `json:"sourceDir"`
	Entries      int        `json:"entries"`
	PlainBytes   int64      `json:"plainBytes"`
	ArchiveBytes int64      `json:"archiveBytes"`
	SHA256       string     `json:"sha256"`
	Encryption   string     `json:"encryption"`
	Format       string     `json:"format"`
	RestoredAt   *time.Time `json:"restoredAt,omitempty"`
	RestoreCount int        `json:"restoreCount"`
}

func (m manifest) archive() Archive {
	return Archive{
		ID:           m.ID,
		Account:      m.Account,
		Label:        m.Label,
		CreatedAt:    m.CreatedAt,
		SizeBytes:    m.ArchiveBytes,
		PlainBytes:   m.PlainBytes,
		Entries:      m.Entries,
		SHA256:       m.SHA256,
		Encryption:   m.Encryption,
		Format:       m.Format,
		RestoredAt:   m.RestoredAt,
		RestoreCount: m.RestoreCount,
	}
}

// RestoreResult reports what a restore produced.
type RestoreResult struct {
	ID       string    `json:"id"`
	Target   string    `json:"target"`
	Files    int       `json:"files"`
	Bytes    int64     `json:"bytes"`
	Restored time.Time `json:"restoredAt"`
}

// Engine stores archives and manifests below Root.
type Engine struct {
	opts Options
	now  func() time.Time
}

// NewEngine builds an engine. A missing passphrase is reported on first use so
// the panel can start without backup configured.
func NewEngine(opts Options) *Engine {
	return &Engine{opts: opts, now: time.Now}
}

// Configured reports whether an encryption passphrase is available.
func (e *Engine) Configured() bool {
	return e != nil && e.opts.Passphrase != ""
}

// Root returns the archive root directory.
func (e *Engine) Root() string {
	if e == nil {
		return ""
	}
	return e.opts.Root
}

func (e *Engine) accountDir(account string) (string, error) {
	if !accountPattern.MatchString(account) {
		return "", ErrInvalidAccount
	}
	return filepath.Join(e.opts.Root, account), nil
}

func newID(now time.Time) (string, error) {
	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	return "bk-" + now.UTC().Format("20060102T150405") + "-" + hex.EncodeToString(suffix), nil
}

// ValidateLabel rejects control characters and oversized labels. An empty label
// is allowed and means the archive keeps its generated identifier only.
func ValidateLabel(label string) error {
	if len(label) > maxLabelLength || controlPattern.MatchString(label) {
		return ErrInvalidLabel
	}
	return nil
}

// Create archives one account directory into an encrypted backup.
func (e *Engine) Create(ctx context.Context, account, sourceDir, label string) (*Archive, error) {
	if e == nil {
		return nil, ErrNoPassphrase
	}
	if !e.Configured() {
		return nil, ErrNoPassphrase
	}
	dir, err := e.accountDir(account)
	if err != nil {
		return nil, err
	}
	if err := ValidateLabel(label); err != nil {
		return nil, err
	}
	sourceDir = filepath.Clean(strings.TrimSpace(sourceDir))
	if sourceDir == "" || sourceDir == "." {
		return nil, ErrSourceMissing
	}
	info, err := os.Stat(sourceDir)
	if err != nil || !info.IsDir() {
		return nil, ErrSourceMissing
	}
	if e.opts.HomeBase != "" {
		base := filepath.Clean(e.opts.HomeBase)
		relative, relErr := filepath.Rel(base, sourceDir)
		if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return nil, ErrSourceEscaped
		}
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	now := e.now()
	id, err := newID(now)
	if err != nil {
		return nil, err
	}

	finalPath := filepath.Join(dir, id+archiveSuffix)
	partialPath := finalPath + ".partial"
	file, err := os.OpenFile(partialPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("create archive file: %w", err)
	}

	encrypted, err := newEncryptedWriter(file, e.opts.Passphrase)
	if err != nil {
		_ = file.Close()
		_ = os.Remove(partialPath)
		return nil, err
	}
	gzipWriter := gzip.NewWriter(encrypted)
	tarWriter := tar.NewWriter(gzipWriter)

	entries, plainBytes, archiveErr := writeArchive(ctx, tarWriter, sourceDir)

	closeErr := tarWriter.Close()
	if archiveErr == nil {
		archiveErr = closeErr
	}
	if err := gzipWriter.Close(); archiveErr == nil {
		archiveErr = err
	}
	if err := encrypted.Close(); archiveErr == nil {
		archiveErr = err
	}
	if err := file.Sync(); archiveErr == nil {
		archiveErr = err
	}
	if err := file.Close(); archiveErr == nil {
		archiveErr = err
	}
	if archiveErr != nil {
		_ = os.Remove(partialPath)
		return nil, archiveErr
	}

	digest, size, err := hashFile(partialPath)
	if err != nil {
		_ = os.Remove(partialPath)
		return nil, err
	}
	if err := os.Rename(partialPath, finalPath); err != nil {
		_ = os.Remove(partialPath)
		return nil, fmt.Errorf("commit archive: %w", err)
	}

	record := manifest{
		Version:      manifestVersion,
		ID:           id,
		Account:      account,
		Label:        label,
		CreatedAt:    now.UTC(),
		SourceDir:    sourceDir,
		Entries:      entries,
		PlainBytes:   plainBytes,
		ArchiveBytes: size,
		SHA256:       digest,
		Encryption:   "aes-256-gcm+pbkdf2-sha256",
		Format:       "tar+gzip",
	}
	if err := writeManifest(filepath.Join(dir, id+manifestSuffix), record); err != nil {
		_ = os.Remove(finalPath)
		return nil, err
	}

	archived := record.archive()
	return &archived, nil
}

func writeArchive(ctx context.Context, tarWriter *tar.Writer, sourceDir string) (int, int64, error) {
	entries := 0
	var plainBytes int64
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(sourceDir, path)
		if err != nil || relative == "." {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, linkErr := os.Readlink(path)
			if linkErr != nil {
				return linkErr
			}
			header, err = tar.FileInfoHeader(info, target)
			if err != nil {
				return err
			}
		}
		if !info.Mode().IsRegular() && !info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			// Sockets, devices, and pipes carry no restorable content.
			return nil
		}
		header.Name = filepath.ToSlash(relative)
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		entries++
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(tarWriter, file)
		closeErr := file.Close()
		plainBytes += written
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return entries, plainBytes, err
	}
	return entries, plainBytes, nil
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), size, nil
}

func writeManifest(path string, record manifest) error {
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	temporary := path + ".partial"
	if err := os.WriteFile(temporary, append(encoded, '\n'), 0600); err != nil {
		return fmt.Errorf("write backup manifest: %w", err)
	}
	return os.Rename(temporary, path)
}

func readManifest(path string) (manifest, error) {
	var record manifest
	content, err := os.ReadFile(path)
	if err != nil {
		return record, err
	}
	if err := json.Unmarshal(content, &record); err != nil {
		return record, fmt.Errorf("parse backup manifest %s: %w", filepath.Base(path), err)
	}
	if record.Version != manifestVersion {
		return record, fmt.Errorf("unsupported backup manifest version %d", record.Version)
	}
	return record, nil
}

// List returns the archives stored for one account, newest first.
func (e *Engine) List(account string) ([]Archive, error) {
	dir, err := e.accountDir(account)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Archive{}, nil
		}
		return nil, err
	}
	archives := make([]Archive, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), manifestSuffix) {
			continue
		}
		record, err := readManifest(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(dir, record.ID+archiveSuffix)); statErr != nil {
			continue
		}
		archives = append(archives, record.archive())
	}
	sort.Slice(archives, func(i, j int) bool { return archives[i].CreatedAt.After(archives[j].CreatedAt) })
	return archives, nil
}

func (e *Engine) lookup(account, id string) (manifest, string, string, error) {
	dir, err := e.accountDir(account)
	if err != nil {
		return manifest{}, "", "", err
	}
	if !idPattern.MatchString(id) {
		return manifest{}, "", "", ErrArchiveMissing
	}
	manifestPath := filepath.Join(dir, id+manifestSuffix)
	archivePath := filepath.Join(dir, id+archiveSuffix)
	record, err := readManifest(manifestPath)
	if err != nil {
		return manifest{}, "", "", ErrArchiveMissing
	}
	if record.Account != account || record.ID != id {
		return manifest{}, "", "", ErrArchiveMissing
	}
	if _, err := os.Stat(archivePath); err != nil {
		return manifest{}, "", "", ErrArchiveMissing
	}
	return record, manifestPath, archivePath, nil
}

// Verify recomputes the archive digest and authenticates every encrypted chunk.
func (e *Engine) Verify(account, id string) error {
	record, _, archivePath, err := e.lookup(account, id)
	if err != nil {
		return err
	}
	digest, _, err := hashFile(archivePath)
	if err != nil {
		return err
	}
	if digest != record.SHA256 {
		return ErrChecksum
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	reader, err := newEncryptedReader(file, e.opts.Passphrase)
	if err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return err
	}
	return nil
}

// Restore decrypts an archive into targetDir. Every entry is validated before
// it is written, so a crafted archive cannot escape the destination.
func (e *Engine) Restore(ctx context.Context, account, id, targetDir string) (*RestoreResult, error) {
	if e == nil || !e.Configured() {
		return nil, ErrNoPassphrase
	}
	record, manifestPath, archivePath, err := e.lookup(account, id)
	if err != nil {
		return nil, err
	}
	if err := e.Verify(account, id); err != nil {
		return nil, err
	}

	targetDir = filepath.Clean(strings.TrimSpace(targetDir))
	if targetDir == "" || targetDir == "." {
		return nil, errors.New("restore target is required")
	}
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		return nil, fmt.Errorf("create restore target: %w", err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(targetDir)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decrypted, err := newEncryptedReader(file, e.opts.Passphrase)
	if err != nil {
		return nil, err
	}
	gzipReader, err := gzip.NewReader(decrypted)
	if err != nil {
		return nil, fmt.Errorf("open backup stream: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)

	files := 0
	var restoredBytes int64
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		name := filepath.Clean(filepath.FromSlash(header.Name))
		if name == "." || filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || name == ".." {
			return nil, ErrUnsafeEntry
		}
		destination := filepath.Join(resolvedTarget, name)
		if !strings.HasPrefix(destination, resolvedTarget+string(os.PathSeparator)) {
			return nil, ErrUnsafeEntry
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0750); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destination), 0750); err != nil {
				return nil, err
			}
			out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
			if err != nil {
				return nil, err
			}
			written, copyErr := io.Copy(out, tarReader)
			closeErr := out.Close()
			if copyErr != nil {
				return nil, copyErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
			restoredBytes += written
			files++
		case tar.TypeSymlink:
			// Symlinks are not recreated: a link target can point outside the
			// account tree and would become a privilege escalation primitive.
			continue
		}
	}

	now := e.now().UTC()
	record.RestoredAt = &now
	record.RestoreCount++
	if err := writeManifest(manifestPath, record); err != nil {
		return nil, err
	}

	return &RestoreResult{
		ID:       id,
		Target:   resolvedTarget,
		Files:    files,
		Bytes:    restoredBytes,
		Restored: now,
	}, nil
}

// Delete removes one archive and its manifest.
func (e *Engine) Delete(account, id string) error {
	_, manifestPath, archivePath, err := e.lookup(account, id)
	if err != nil {
		return err
	}
	if err := os.Remove(archivePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(manifestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Prune keeps the newest keep archives for an account and deletes the rest.
func (e *Engine) Prune(account string, keep int) (int, error) {
	if keep < 1 {
		keep = 1
	}
	archives, err := e.List(account)
	if err != nil {
		return 0, err
	}
	removed := 0
	for index, archive := range archives {
		if index < keep {
			continue
		}
		if err := e.Delete(account, archive.ID); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
