package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

const testRoot = "build-your-own-grep-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

const mitLicense = `MIT License

Copyright (c) 2026 Example

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction.

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.
`

const testDefinition = `slug: "grep"
name: "Build your own Grep"
short_description_md: "Learn **regex** syntax."
languages:
  - slug: "go"
  - slug: "rust"
stages:
  - slug: "s1"
    name: "Match a literal"
  - slug: "s2"
    name: "Match digits"
`

func courseFiles() map[string]string {
	return map[string]string{
		"course-definition.yml":         testDefinition,
		"LICENSE":                       mitLicense,
		"stage_descriptions/s1.md":      "# Match a literal",
		"starter_templates/go/main.go":  "package main\n",
		"solutions/go/main.go":          "secret solution\n",
		"compiled_starters/go/main":     "binary\n",
		"README.md":                     "readme\n",
		"dockerfiles/go/Dockerfile":     "FROM scratch\n",
		"stage_descriptions/README.txt": "stage docs\n",
	}
}

func tarball(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: testRoot + "/", Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		hdr := &tar.Header{
			Name:     testRoot + "/" + name,
			Mode:     0o644,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testPin() Pin {
	return Pin{
		Slug:    "grep",
		Name:    "Build your own Grep",
		Repo:    "https://github.com/codecrafters-io/build-your-own-grep",
		Commit:  strings.Repeat("a", 40),
		License: "MIT",
	}
}

func TestLoadPins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "courses.json")
	valid := `{"provider":"codecrafters","courses":[{"slug":"grep","name":"Grep","repo":"https://github.com/o/r","commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","license":"MIT"}]}`
	if err := os.WriteFile(path, []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	pins, err := LoadPins(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins.Courses) != 1 || pins.Provider != "codecrafters" {
		t.Fatalf("pins = %+v", pins)
	}

	bad := []string{
		`{"provider":"x","courses":[{"slug":"grep","name":"Grep","repo":"https://github.com/o/r","commit":"not-a-sha","license":"MIT"}]}`,
		`{"provider":"x","courses":[{"slug":"grep","name":"Grep","repo":"https://gitlab.com/o/r","commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","license":"MIT"}]}`,
		`{"provider":"x","courses":[{"slug":"grep","name":"Grep","repo":"https://github.com/o/r","commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","license":"MIT"},{"slug":"grep","name":"Dup","repo":"https://github.com/o/r2","commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","license":"MIT"}]}`,
	}
	for _, body := range bad {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadPins(path); err == nil {
			t.Fatalf("accepted invalid pins: %s", body)
		}
	}
}

func TestRepoParts(t *testing.T) {
	owner, name, err := RepoParts("https://github.com/codecrafters-io/build-your-own-grep")
	if err != nil || owner != "codecrafters-io" || name != "build-your-own-grep" {
		t.Fatalf("got %q %q %v", owner, name, err)
	}
	if _, _, err := RepoParts("https://example.com/o/r"); err == nil {
		t.Fatal("non-github URL accepted")
	}
}

func TestExtractFiltersDisallowedPaths(t *testing.T) {
	dir := t.TempDir()
	out, err := ExtractArchive(tarball(t, courseFiles()), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, allowed := range []string{
		"course-definition.yml", "LICENSE",
		"stage_descriptions/s1.md", "starter_templates/go/main.go",
	} {
		if _, ok := out.Files[allowed]; !ok {
			t.Errorf("missing allowed file %q", allowed)
		}
	}
	for _, denied := range []string{
		"solutions/go/main.go", "compiled_starters/go/main", "README.md",
		"dockerfiles/go/Dockerfile",
	} {
		if _, ok := out.Files[denied]; ok {
			t.Errorf("bundled disallowed file %q", denied)
		}
	}
	if out.Skipped == 0 {
		t.Error("expected skipped entries")
	}
	if _, err := os.Stat(filepath.Join(dir, "course-definition.yml")); err != nil {
		t.Fatal("allowed file not written")
	}
}

func TestExtractRejectsUnsafeEntries(t *testing.T) {
	cases := [][]tar.Header{
		{{Name: "../evil", Typeflag: tar.TypeReg, Size: 1}},
		{{Name: "/etc/passwd", Typeflag: tar.TypeReg, Size: 1}},
		{{Name: testRoot + "/../evil", Typeflag: tar.TypeReg, Size: 1}},
		{{Name: testRoot + "/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}},
	}
	for _, headers := range cases {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		for _, hdr := range headers {
			if err := tw.WriteHeader(&hdr); err != nil {
				t.Fatal(err)
			}
			if hdr.Typeflag == tar.TypeReg {
				tw.Write([]byte("x"))
			}
		}
		tw.Close()
		gz.Close()
		if _, err := ExtractArchive(buf.Bytes(), t.TempDir()); err == nil {
			t.Errorf("accepted unsafe archive %+v", headers)
		}
	}
}

func TestExtractRejectsDuplicate(t *testing.T) {
	files := courseFiles()
	body := tarball(t, files)
	// Append the same allowed path a second time by rebuilding with a custom tar.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"course-definition.yml", "course-definition.yml"} {
		data := testDefinition
		hdr := &tar.Header{Name: testRoot + "/" + name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg}
		tw.WriteHeader(hdr)
		tw.Write([]byte(data))
	}
	tw.Close()
	gz.Close()
	if _, err := ExtractArchive(body, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractArchive(buf.Bytes(), t.TempDir()); err == nil {
		t.Fatal("duplicate entry accepted")
	}
}

func TestValidateLicense(t *testing.T) {
	if err := ValidateLicense(testPin(), []byte(mitLicense)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLicense(testPin(), []byte("All rights reserved")); err == nil {
		t.Fatal("non-MIT license accepted")
	}
	if err := ValidateLicense(Pin{Slug: "x", License: "GPL-3.0"}, []byte(mitLicense)); err == nil {
		t.Fatal("unsupported SPDX license accepted")
	}
}

func TestParseDefinition(t *testing.T) {
	def, err := ParseDefinition([]byte(testDefinition))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateDefinition(testPin(), def); err != nil {
		t.Fatal(err)
	}
	if def.StageCount() != 2 || len(def.LanguagesList()) != 2 {
		t.Fatalf("definition = %+v", def)
	}
	if err := ValidateDefinition(Pin{Slug: "redis"}, def); err == nil {
		t.Fatal("slug mismatch accepted")
	}
}

func TestWriteArchiveDeterministic(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"a.txt": "a", "sub/b.txt": "b"} {
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var first, second bytes.Buffer
	if err := WriteArchive(dir, &first); err != nil {
		t.Fatal(err)
	}
	if err := WriteArchive(dir, &second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("archive output is not deterministic")
	}
}

func newFetchServer(t *testing.T, files map[string]string, wantCommit string) *httptest.Server {
	t.Helper()
	body := tarball(t, files)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := "/codecrafters-io/build-your-own-grep/tar.gz/" + wantCommit
		if r.URL.Path != want {
			t.Errorf("fetch path = %q, want %q", r.URL.Path, want)
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		w.Write(body)
	}))
}

func TestBuild(t *testing.T) {
	pin := testPin()
	srv := newFetchServer(t, courseFiles(), pin.Commit)
	defer srv.Close()

	dist := filepath.Join(t.TempDir(), "dist")
	idx, err := Build(context.Background(), Options{
		Pins:    Pins{Provider: "codecrafters", Courses: []Pin{pin}},
		Dist:    dist,
		Release: "content-test",
		Builder: Builder{Repo: "example/quest-content", Commit: "bbbb"},
		Fetcher: Fetcher{BaseURL: srv.URL, Client: srv.Client()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Courses) != 1 {
		t.Fatalf("courses = %d", len(idx.Courses))
	}
	entry := idx.Courses[0]
	if entry.Slug != "grep" || entry.Stages != 2 || entry.License != "MIT" {
		t.Fatalf("entry = %+v", entry)
	}
	if entry.Bundle.Asset != "codecrafters-grep-content-test.tar.gz" {
		t.Fatalf("asset = %q", entry.Bundle.Asset)
	}

	bundlePath := filepath.Join(dist, entry.Bundle.Asset)
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != entry.Bundle.SHA256 {
		t.Fatal("index digest does not match bundle")
	}

	extracted, err := ExtractArchive(data, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := extracted.Files["solutions/go/main.go"]; ok {
		t.Fatal("bundle contains solutions")
	}
	manifestData := readTarEntry(t, data, "manifest.json")
	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Spec.Commit != pin.Commit || manifest.Spec.License != "MIT" || manifest.Format != Format {
		t.Fatalf("manifest = %+v", manifest)
	}
	if !strings.Contains(string(manifestData), "course-definition.yml") {
		t.Fatal("manifest does not list files")
	}

	checksums, err := os.ReadFile(filepath.Join(dist, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(checksums)), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Fatalf("bad checksum line %q", line)
		}
		fileData, err := os.ReadFile(filepath.Join(dist, parts[1]))
		if err != nil {
			t.Fatal(err)
		}
		fileSum := sha256.Sum256(fileData)
		if hex.EncodeToString(fileSum[:]) != parts[0] {
			t.Fatalf("checksum mismatch for %s", parts[1])
		}
	}
}

func readTarEntry(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if path.Clean(strings.TrimPrefix(hdr.Name, "./")) == name {
			body, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			return body
		}
	}
	t.Fatalf("entry %q not found in bundle", name)
	return nil
}

func TestBuildDeterministic(t *testing.T) {
	pin := testPin()
	srv := newFetchServer(t, courseFiles(), pin.Commit)
	defer srv.Close()
	opts := Options{
		Pins:    Pins{Provider: "codecrafters", Courses: []Pin{pin}},
		Release: "content-test",
		Builder: Builder{Repo: "example/quest-content", Commit: "bbbb"},
		Fetcher: Fetcher{BaseURL: srv.URL, Client: srv.Client()},
	}
	opts.Dist = filepath.Join(t.TempDir(), "one")
	first, err := Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Dist = filepath.Join(t.TempDir(), "two")
	second, err := Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if first.Courses[0].Bundle.SHA256 != second.Courses[0].Bundle.SHA256 {
		t.Fatal("bundle bytes differ between builds")
	}
}

func TestBuildRejectsBadLicense(t *testing.T) {
	files := courseFiles()
	files["LICENSE"] = "All rights reserved.\n"
	pin := testPin()
	srv := newFetchServer(t, files, pin.Commit)
	defer srv.Close()

	_, err := Build(context.Background(), Options{
		Pins:    Pins{Provider: "codecrafters", Courses: []Pin{pin}},
		Dist:    filepath.Join(t.TempDir(), "dist"),
		Release: "content-test",
		Fetcher: Fetcher{BaseURL: srv.URL, Client: srv.Client()},
	})
	if err == nil || !strings.Contains(err.Error(), "MIT") {
		t.Fatalf("expected MIT rejection, got %v", err)
	}
}

func TestBuildRejectsSlugMismatch(t *testing.T) {
	files := courseFiles()
	files["course-definition.yml"] = strings.Replace(testDefinition, `slug: "grep"`, `slug: "redis"`, 1)
	pin := testPin()
	srv := newFetchServer(t, files, pin.Commit)
	defer srv.Close()

	_, err := Build(context.Background(), Options{
		Pins:    Pins{Provider: "codecrafters", Courses: []Pin{pin}},
		Dist:    filepath.Join(t.TempDir(), "dist"),
		Release: "content-test",
		Fetcher: Fetcher{BaseURL: srv.URL, Client: srv.Client()},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match pin") {
		t.Fatalf("expected slug mismatch, got %v", err)
	}
}

func TestBuildMissingLicense(t *testing.T) {
	files := courseFiles()
	delete(files, "LICENSE")
	pin := testPin()
	srv := newFetchServer(t, files, pin.Commit)
	defer srv.Close()

	_, err := Build(context.Background(), Options{
		Pins:    Pins{Provider: "codecrafters", Courses: []Pin{pin}},
		Dist:    filepath.Join(t.TempDir(), "dist"),
		Release: "content-test",
		Fetcher: Fetcher{BaseURL: srv.URL, Client: srv.Client()},
	})
	if err == nil || !strings.Contains(err.Error(), "LICENSE") {
		t.Fatalf("expected missing LICENSE, got %v", err)
	}
}
