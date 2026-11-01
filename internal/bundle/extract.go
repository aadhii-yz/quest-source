package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// AllowedPaths are the only paths that may enter a bundle. Everything else in
// an upstream repository (solutions, compiled starters, dockerfiles, tester
// code) is skipped.
var AllowedPaths = []string{
	"course-definition.yml",
	"LICENSE",
	"stage_descriptions/",
	"starter_templates/",
}

const (
	maxFileSize  = 32 << 20
	maxTotalSize = 512 << 20
)

// Allowed reports whether a cleaned relative path may be bundled.
func Allowed(p string) bool {
	for _, allowed := range AllowedPaths {
		if p == allowed || strings.HasPrefix(p, allowed) {
			return true
		}
	}
	return false
}

// Extracted is the result of unpacking an upstream tarball.
type Extracted struct {
	// Files maps bundled relative paths to hex SHA256 digests.
	Files map[string]string
	// Skipped counts entries outside AllowedPaths.
	Skipped int
}

// ExtractArchive unpacks a GitHub tarball into dest, keeping only AllowedPaths.
//
// It rejects absolute paths, parent traversal, backslashes, duplicate entries,
// links and oversized files. The tarball's single top-level directory is
// stripped.
func ExtractArchive(data []byte, dest string) (Extracted, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return Extracted{}, fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	out := Extracted{Files: map[string]string{}}
	root := ""
	var total int64

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Extracted{}, fmt.Errorf("tar: %w", err)
		}

		name := strings.TrimPrefix(hdr.Name, "./")
		if name == "" || name == "." {
			continue
		}
		if strings.ContainsRune(name, '\\') {
			return Extracted{}, fmt.Errorf("tar: backslash in path %q", hdr.Name)
		}
		name = path.Clean(name)
		if path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
			return Extracted{}, fmt.Errorf("tar: path escapes archive: %q", hdr.Name)
		}

		// GitHub tarballs begin with a single <repo>-<sha>/ directory. Bundles
		// produced by this repository are flat, so root stays empty for them.
		if root == "" && hdr.Typeflag == tar.TypeDir && !strings.Contains(name, "/") {
			root = name
			continue
		}
		rel := name
		if root != "" {
			if name == root {
				continue
			}
			if !strings.HasPrefix(name, root+"/") {
				return Extracted{}, fmt.Errorf("tar: entry %q is outside root %q", hdr.Name, root)
			}
			rel = strings.TrimPrefix(name, root+"/")
		}

		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			// GitHub tarballs carry a global PAX header; it is metadata only.
			continue
		case tar.TypeDir:
			if !Allowed(rel + "/") {
				out.Skipped++
				continue
			}
			if err := os.MkdirAll(filepath.Join(dest, filepath.FromSlash(rel)), 0o755); err != nil {
				return Extracted{}, err
			}
			continue
		case tar.TypeReg, tar.TypeRegA:
			if !Allowed(rel) {
				out.Skipped++
				continue
			}
			if _, exists := out.Files[rel]; exists {
				return Extracted{}, fmt.Errorf("tar: duplicate entry %q", rel)
			}
			if hdr.Size > maxFileSize {
				return Extracted{}, fmt.Errorf("tar: %q is %d bytes, limit is %d", rel, hdr.Size, maxFileSize)
			}
			total += hdr.Size
			if total > maxTotalSize {
				return Extracted{}, fmt.Errorf("tar: archive exceeds %d bytes", maxTotalSize)
			}
			digest, err := writeEntry(dest, rel, tr, hdr.Size)
			if err != nil {
				return Extracted{}, err
			}
			out.Files[rel] = digest
		default:
			return Extracted{}, fmt.Errorf("tar: unsupported entry type for %q", hdr.Name)
		}
	}

	if len(out.Files) == 0 {
		return Extracted{}, errors.New("tar: no bundleable files found")
	}
	return out, nil
}

func writeEntry(dest, rel string, r io.Reader, size int64) (string, error) {
	target := filepath.Join(dest, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(r, size))
	if err != nil {
		return "", err
	}
	if n != size {
		return "", fmt.Errorf("tar: %q truncated (read %d of %d bytes)", rel, n, size)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
