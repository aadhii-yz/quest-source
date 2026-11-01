// Package bundle builds per-course content bundles from pinned upstream
// course repositories.
//
// Bundles contain only MIT-licensed course content (course-definition.yml,
// stage descriptions, starter templates and the upstream LICENSE). Tester
// source, tester binaries, solutions and compiled starters are never included.
package bundle

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// Format is the bundle/index format version written into manifests and the
// index. Bump it when the format changes incompatibly.
const Format = 1

// Pin is one upstream course repository pinned to a commit.
type Pin struct {
	// Slug is the course identifier, e.g. "grep".
	Slug string `json:"slug"`
	// Name is the display name, e.g. "Build your own Grep".
	Name string `json:"name"`
	// Repo is the https://github.com/owner/repo URL.
	Repo string `json:"repo"`
	// Commit is the full 40-character SHA to build from.
	Commit string `json:"commit"`
	// License is the SPDX identifier expected for this repository.
	License string `json:"license"`
}

// Pins is the content repository's pin file (courses.json).
type Pins struct {
	Provider string `json:"provider"`
	Courses  []Pin  `json:"courses"`
}

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// LoadPins reads and validates a pins file.
func LoadPins(path string) (Pins, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Pins{}, err
	}
	var pins Pins
	if err := json.Unmarshal(data, &pins); err != nil {
		return Pins{}, fmt.Errorf("%s: %w", path, err)
	}
	if pins.Provider == "" {
		return Pins{}, fmt.Errorf("%s: provider is required", path)
	}
	if len(pins.Courses) == 0 {
		return Pins{}, fmt.Errorf("%s: no courses pinned", path)
	}
	seen := map[string]bool{}
	for i := range pins.Courses {
		pin := &pins.Courses[i]
		if err := pin.Validate(); err != nil {
			return Pins{}, fmt.Errorf("%s: course %d: %w", path, i, err)
		}
		if seen[pin.Slug] {
			return Pins{}, fmt.Errorf("%s: duplicate course %q", path, pin.Slug)
		}
		seen[pin.Slug] = true
	}
	return pins, nil
}

// Validate checks that a pin is complete and buildable.
func (p Pin) Validate() error {
	if p.Slug == "" {
		return fmt.Errorf("slug is required")
	}
	if p.Name == "" {
		return fmt.Errorf("%s: name is required", p.Slug)
	}
	if _, _, err := RepoParts(p.Repo); err != nil {
		return fmt.Errorf("%s: %w", p.Slug, err)
	}
	if !commitPattern.MatchString(p.Commit) {
		return fmt.Errorf("%s: commit must be a full lowercase 40-character SHA, got %q", p.Slug, p.Commit)
	}
	if p.License == "" {
		return fmt.Errorf("%s: license is required", p.Slug)
	}
	return nil
}

// RepoParts splits an https://github.com/owner/repo URL into its parts.
func RepoParts(repo string) (owner, name string, err error) {
	u, err := url.Parse(strings.TrimSpace(repo))
	if err != nil {
		return "", "", fmt.Errorf("invalid repo URL %q: %w", repo, err)
	}
	if u.Scheme != "https" || u.Host != "github.com" {
		return "", "", fmt.Errorf("repo must be an https://github.com URL, got %q", repo)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repo must look like https://github.com/owner/repo, got %q", repo)
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}
