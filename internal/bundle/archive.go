package bundle

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// WriteArchive writes a deterministic tar.gz of the files under dir.
//
// Paths are sorted, timestamps and ownership are zeroed, and the gzip header is
// fixed, so building the same content twice produces identical bytes.
func WriteArchive(dir string, out io.Writer) error {
	paths := make([]string, 0, 64)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("archive: %s contains no files", dir)
	}
	sort.Strings(paths)

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	gz.ModTime = time.Unix(0, 0)
	gz.Name = ""
	gz.Comment = ""

	tw := tar.NewWriter(gz)
	for _, rel := range paths {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		info, err := os.Stat(full)
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Name:     rel,
			Mode:     0o644,
			Size:     info.Size(),
			ModTime:  time.Unix(0, 0),
			Typeflag: tar.TypeReg,
			Format:   tar.FormatPAX,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(full)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}
