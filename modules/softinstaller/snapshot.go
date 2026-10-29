// DigiPanel installer snapshot and restore (modules/softinstaller/snapshot.go)
// Update operations capture a real tar.gz backup of the instance tree before
// swapping files, so a failed upgrade can always be rolled back from disk.
// Symlinks and special files are rejected rather than archived.
// Zero emojis.

package softinstaller

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// snapshot writes StateDir/backups/<instance>-<timestamp>.tar.gz and returns
// its absolute path. The archive contains relative paths only.
func (m *Manager) snapshot(inst *Instance) (string, error) {
	if inst == nil {
		return "", ErrInstanceNotFound
	}
	if _, err := os.Stat(inst.Path); err != nil {
		return "", fmt.Errorf("snapshot source missing: %w", err)
	}
	dir := filepath.Join(m.cfg.StateDir, "backups")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%s.tar.gz", inst.ID, m.now().UTC().Format("20060102T150405Z"))
	path := filepath.Join(dir, name)
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	writeErr := filepath.WalkDir(inst.Path, func(current string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(inst.Path, current)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symlink %s", ErrArchiveUnsafe, rel)
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if info.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		src, err := os.Open(current)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, src)
		closeErr := src.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if writeErr == nil {
		writeErr = tw.Close()
	} else {
		_ = tw.Close()
	}
	if err := gz.Close(); err != nil && writeErr == nil {
		writeErr = err
	}
	if err := f.Close(); err != nil && writeErr == nil {
		writeErr = err
	}
	if writeErr != nil {
		_ = os.Remove(tmp)
		return "", writeErr
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	m.logf(inst.ID, "snapshot written: %s", filepath.Base(path))
	return path, nil
}

// restoreSnapshot extracts a snapshot over an empty target directory.
func (m *Manager) restoreSnapshot(archivePath, target string) error {
	if !strings.HasPrefix(filepath.Clean(archivePath), filepath.Clean(filepath.Join(m.cfg.StateDir, "backups"))+string(os.PathSeparator)) {
		return fmt.Errorf("%w: snapshot path outside backup directory", ErrInvalidInput)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("snapshot is not gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		name, err := safeArchiveName(hdr.Name, "")
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}
		out := filepath.Join(target, name)
		if !strings.HasPrefix(out, filepath.Clean(target)+string(os.PathSeparator)) {
			return fmt.Errorf("%w: snapshot entry escapes target", ErrArchiveUnsafe)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(out, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeArchiveFile(out, tr, hdr.Size, hdr.Mode); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: unsupported snapshot entry type", ErrArchiveUnsafe)
		}
	}
	return nil
}

// pruneSnapshots deletes all but the newest keep snapshots for an instance.
func (m *Manager) pruneSnapshots(instID string, keep int) {
	if keep < 0 {
		keep = 0
	}
	dir := filepath.Join(m.cfg.StateDir, "backups")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type item struct {
		name string
		mod  time.Time
	}
	var items []item
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), instID+"-") || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, item{name: entry.Name(), mod: info.ModTime()})
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].mod.After(items[i].mod) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	for i := keep; i < len(items); i++ {
		_ = os.Remove(filepath.Join(dir, items[i].name))
	}
}
