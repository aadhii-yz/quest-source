package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// BundleRef points at one course bundle asset.
type BundleRef struct {
	Asset  string `json:"asset"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// CourseEntry is the catalog-facing summary of one course. It is deliberately
// small: the app can render the catalog from index.json without downloading
// any bundle.
type CourseEntry struct {
	Slug               string    `json:"slug"`
	Name               string    `json:"name"`
	ShortDescriptionMD string    `json:"short_description_md"`
	License            string    `json:"license"`
	SpecCommit         string    `json:"spec_commit"`
	Stages             int       `json:"stages"`
	Languages          []string  `json:"languages"`
	Bundle             BundleRef `json:"bundle"`
	ManifestSHA256     string    `json:"manifest_sha256"`
}

// Index is the release-level catalog of bundles.
type Index struct {
	Format         int           `json:"format"`
	ContentRelease string        `json:"content_release"`
	Provider       string        `json:"provider"`
	Courses        []CourseEntry `json:"courses"`
}

// WriteIndex writes index.json, with courses sorted by slug for stable output.
func WriteIndex(path string, idx Index) error {
	if idx.Format != Format {
		return fmt.Errorf("index: format %d, want %d", idx.Format, Format)
	}
	if idx.ContentRelease == "" {
		return fmt.Errorf("index: content_release is required")
	}
	sort.Slice(idx.Courses, func(i, j int) bool { return idx.Courses[i].Slug < idx.Courses[j].Slug })
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
