// DigiPanel archive extraction sandbox (modules/softinstaller/archive.go)
// Supports tar.gz, tar.bz2, and zip with hard rejection of traversal, links,
// device nodes, oversized entries, and setuid modes.
// Zero shell execution, zero emojis.

package softinstaller

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxArchiveFileBytes  = 512 << 20 // 512 MiB per file
	maxArchiveTotalBytes = 2 << 30   // 2 GiB extracted total
	maxArchiveEntries    = 200000
)

var ErrArchiveUnsafe = errors.New("archive rejected by extraction sandbox")

// extractArchive extracts into dest, applying stripPrefix to every entry.
func extractArchive(archivePath, dest, archiveType, stripPrefix string) error {
	switch archiveType {
	case ArchiveTarGz, ArchiveTarBz2:
		return extractTar(archivePath, dest, archiveType, stripPrefix)
	case ArchiveZip:
		return extractZip(archivePath, dest, stripPrefix)
	default:
		return fmt.Errorf("%w: unsupported archive type %q", ErrArchiveUnsafe, archiveType)
	}
}

func extractTar(archivePath, dest, archiveType, stripPrefix string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader
	switch archiveType {
	case ArchiveTarGz:
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("not a gzip archive: %w", err)
		}
		defer gz.Close()
		reader = gz
	case ArchiveTarBz2:
		reader = bzip2.NewReader(f)
	}
	return consumeTar(tar.NewReader(reader), dest, stripPrefix)
}

func consumeTar(tr *tar.Reader, dest, stripPrefix string) error {
	entries := 0
	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: tar read failed: %v", ErrArchiveUnsafe, err)
		}
		entries++
		if entries > maxArchiveEntries {
			return fmt.Errorf("%w: archive contains too many entries", ErrArchiveUnsafe)
		}
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			return fmt.Errorf("%w: link entry %q", ErrArchiveUnsafe, hdr.Name)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeDir {
			continue
		}
		name, err := safeArchiveName(hdr.Name, stripPrefix)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}
		out := filepath.Join(dest, name)
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(out, 0755); err != nil {
				return err
			}
			continue
		}
		if hdr.Size < 0 || hdr.Size > maxArchiveFileBytes {
			return fmt.Errorf("%w: file %q size out of range", ErrArchiveUnsafe, hdr.Name)
		}
		total += hdr.Size
		if total > maxArchiveTotalBytes {
			return fmt.Errorf("%w: extracted size exceeds cap", ErrArchiveUnsafe)
		}
		if err := writeArchiveFile(out, tr, hdr.Size, hdr.Mode); err != nil {
			return err
		}
	}
}

func extractZip(archivePath, dest, stripPrefix string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("%w: zip open failed: %v", ErrArchiveUnsafe, err)
	}
	defer zr.Close()
	if len(zr.File) > maxArchiveEntries {
		return fmt.Errorf("%w: archive contains too many entries", ErrArchiveUnsafe)
	}
	var total int64
	for _, zf := range zr.File {
		if zf.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: link entry %q", ErrArchiveUnsafe, zf.Name)
		}
		name, err := safeArchiveName(zf.Name, stripPrefix)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}
		out := filepath.Join(dest, name)
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(out, 0755); err != nil {
				return err
			}
			continue
		}
		if !zf.Mode().IsRegular() {
			continue
		}
		if zf.UncompressedSize64 > maxArchiveFileBytes {
			return fmt.Errorf("%w: file %q size out of range", ErrArchiveUnsafe, zf.Name)
		}
		total += int64(zf.UncompressedSize64)
		if total > maxArchiveTotalBytes {
			return fmt.Errorf("%w: extracted size exceeds cap", ErrArchiveUnsafe)
		}
		rc, err := zf.Open()
		if err != nil {
			return fmt.Errorf("%w: zip entry %q: %v", ErrArchiveUnsafe, zf.Name, err)
		}
		err = writeArchiveFile(out, rc, int64(zf.UncompressedSize64), int64(zf.Mode().Perm()))
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// safeArchiveName normalizes an entry name, strips the configured top-level
// directory, and rejects absolute paths, traversal, and empty segments.
func safeArchiveName(raw, stripPrefix string) (string, error) {
	name := strings.ReplaceAll(raw, "\\", "/")
	if name == "" {
		return "", nil
	}
	if strings.HasPrefix(name, "/") || strings.Contains(name, "\x00") {
		return "", fmt.Errorf("%w: absolute or NUL entry %q", ErrArchiveUnsafe, raw)
	}
	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("%w: traversal entry %q", ErrArchiveUnsafe, raw)
	}
	if stripPrefix != "" {
		prefix := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(stripPrefix)), "/") + "/"
		if clean == strings.TrimSuffix(prefix, "/") {
			return "", nil
		}
		if !strings.HasPrefix(clean, prefix) {
			return "", nil // entries outside the declared application root are ignored
		}
		clean = strings.TrimPrefix(clean, prefix)
		if clean == "" {
			return "", nil
		}
	}
	if strings.HasPrefix(clean, "../") || clean == ".." {
		return "", fmt.Errorf("%w: traversal entry %q", ErrArchiveUnsafe, raw)
	}
	return clean, nil
}

func writeArchiveFile(out string, src io.Reader, declared int64, mode int64) error {
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	perm := os.FileMode(mode) & 0777
	if perm&0111 != 0 {
		perm = 0755
	} else {
		perm = 0644
	}
	f, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(f, io.LimitReader(src, maxArchiveFileBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("%w: write %q: %v", ErrArchiveUnsafe, out, copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if n > maxArchiveFileBytes {
		_ = os.Remove(out)
		return fmt.Errorf("%w: entry %q exceeds per-file cap", ErrArchiveUnsafe, out)
	}
	if declared > 0 && n != declared {
		_ = os.Remove(out)
		return fmt.Errorf("%w: entry %q size mismatch (declared %d, read %d)", ErrArchiveUnsafe, out, declared, n)
	}
	return nil
}
