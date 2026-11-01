package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Spec records the upstream repository a bundle was built from.
type Spec struct {
	Repo        string `json:"repo"`
	Commit      string `json:"commit"`
	License     string `json:"license"`
	LicenseFile string `json:"license_file"`
}

// Builder records which build produced a bundle.
type Builder struct {
	Repo   string `json:"repo"`
	Commit string `json:"commit"`
}

// Manifest describes one course bundle. It is included in the bundle at
// manifest.json.
type Manifest struct {
	Format         int               `json:"format"`
	ContentRelease string            `json:"content_release"`
	Provider       string            `json:"provider"`
	Course         string            `json:"course"`
	Name           string            `json:"name"`
	Spec           Spec              `json:"spec"`
	Builder        Builder           `json:"builder"`
	Files          map[string]string `json:"files"`
}

// WriteManifest writes manifest.json into dir and returns its digest.
func WriteManifest(dir string, m Manifest) (string, error) {
	if m.Format != Format {
		return "", fmt.Errorf("manifest: format %d, want %d", m.Format, Format)
	}
	if m.Course == "" || m.Provider == "" {
		return "", fmt.Errorf("manifest: provider and course are required")
	}
	if m.Spec.License == "" {
		return "", fmt.Errorf("manifest: spec license is required")
	}
	if len(m.Files) == 0 {
		return "", fmt.Errorf("manifest: no files recorded")
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
