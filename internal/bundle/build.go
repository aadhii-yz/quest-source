package bundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Options configures a bundle build.
type Options struct {
	Pins    Pins
	Dist    string
	Release string
	Builder Builder
	Fetcher Fetcher
	// Only limits the build to the given course slugs (empty means all).
	Only []string
}

// Build downloads every pinned course, validates licensing and definitions,
// and writes per-course bundles, index.json and checksums.txt into Dist.
func Build(ctx context.Context, opts Options) (Index, error) {
	if opts.Dist == "" {
		return Index{}, fmt.Errorf("build: dist directory is required")
	}
	if opts.Release == "" {
		return Index{}, fmt.Errorf("build: content release is required")
	}
	if opts.Pins.Provider == "" {
		return Index{}, fmt.Errorf("build: pins have no provider")
	}
	if err := os.MkdirAll(opts.Dist, 0o755); err != nil {
		return Index{}, err
	}

	only := map[string]bool{}
	for _, slug := range opts.Only {
		only[slug] = true
	}

	idx := Index{
		Format:         Format,
		ContentRelease: opts.Release,
		Provider:       opts.Pins.Provider,
	}
	for _, pin := range opts.Pins.Courses {
		if len(only) > 0 && !only[pin.Slug] {
			continue
		}
		entry, err := buildCourse(ctx, opts, pin)
		if err != nil {
			return Index{}, fmt.Errorf("%s: %w", pin.Slug, err)
		}
		idx.Courses = append(idx.Courses, entry)
	}
	if len(idx.Courses) == 0 {
		return Index{}, fmt.Errorf("build: no courses selected")
	}

	if err := WriteIndex(filepath.Join(opts.Dist, "index.json"), idx); err != nil {
		return Index{}, err
	}
	if err := writeChecksums(opts.Dist); err != nil {
		return Index{}, err
	}
	return idx, nil
}

func buildCourse(ctx context.Context, opts Options, pin Pin) (CourseEntry, error) {
	dir, err := os.MkdirTemp("", "quest-source-"+pin.Slug+"-*")
	if err != nil {
		return CourseEntry{}, err
	}
	defer os.RemoveAll(dir)

	data, err := opts.Fetcher.Fetch(ctx, pin)
	if err != nil {
		return CourseEntry{}, err
	}
	extracted, err := ExtractArchive(data, dir)
	if err != nil {
		return CourseEntry{}, err
	}
	for _, required := range []string{"course-definition.yml", "LICENSE"} {
		if _, ok := extracted.Files[required]; !ok {
			return CourseEntry{}, fmt.Errorf("%s: bundle is missing %s", pin.Slug, required)
		}
	}

	licenseText, err := os.ReadFile(filepath.Join(dir, "LICENSE"))
	if err != nil {
		return CourseEntry{}, err
	}
	if err := ValidateLicense(pin, licenseText); err != nil {
		return CourseEntry{}, err
	}

	definitionData, err := os.ReadFile(filepath.Join(dir, "course-definition.yml"))
	if err != nil {
		return CourseEntry{}, err
	}
	def, err := ParseDefinition(definitionData)
	if err != nil {
		return CourseEntry{}, err
	}
	if err := ValidateDefinition(pin, def); err != nil {
		return CourseEntry{}, err
	}

	manifest := Manifest{
		Format:         Format,
		ContentRelease: opts.Release,
		Provider:       opts.Pins.Provider,
		Course:         pin.Slug,
		Name:           def.Name,
		Spec: Spec{
			Repo:        pin.Repo,
			Commit:      pin.Commit,
			License:     pin.License,
			LicenseFile: "LICENSE",
		},
		Builder: opts.Builder,
		Files:   extracted.Files,
	}
	manifestSHA, err := WriteManifest(dir, manifest)
	if err != nil {
		return CourseEntry{}, err
	}

	asset := assetName(opts.Pins.Provider, pin.Slug, opts.Release)
	assetPath := filepath.Join(opts.Dist, asset)
	f, err := os.Create(assetPath)
	if err != nil {
		return CourseEntry{}, err
	}
	archiveErr := WriteArchive(dir, f)
	closeErr := f.Close()
	if archiveErr != nil {
		return CourseEntry{}, archiveErr
	}
	if closeErr != nil {
		return CourseEntry{}, closeErr
	}

	digest, size, err := hashFile(assetPath)
	if err != nil {
		return CourseEntry{}, err
	}

	return CourseEntry{
		Slug:               pin.Slug,
		Name:               def.Name,
		ShortDescriptionMD: def.ShortDescriptionMD,
		License:            pin.License,
		SpecCommit:         pin.Commit,
		Stages:             def.StageCount(),
		Languages:          def.LanguagesList(),
		Bundle: BundleRef{
			Asset:  asset,
			SHA256: digest,
			Size:   size,
		},
		ManifestSHA256: manifestSHA,
	}, nil
}

var unsafeAssetChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func assetName(provider, slug, release string) string {
	return fmt.Sprintf("%s-%s-%s.tar.gz", provider, slug, unsafeAssetChars.ReplaceAllString(release, "-"))
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// writeChecksums writes sha256sum-compatible checksums for every file in dir,
// excluding checksums.txt itself.
func writeChecksums(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "checksums.txt" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		digest, _, err := hashFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s  %s\n", digest, name)
	}
	return os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(b.String()), 0o644)
}
