// Command build produces per-course content bundles, index.json and
// checksums.txt for a content release.
//
// Usage:
//
//	go run ./cmd/build -release content-2026.11
//
// The workflow sets -builder-repo and -builder-commit from the GitHub Actions
// environment so each manifest records its provenance.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aadhii-yz/quest-content/internal/bundle"
)

func main() {
	var (
		pinsPath      = flag.String("pins", "courses.json", "pinned course repositories")
		dist          = flag.String("dist", "dist", "output directory")
		release       = flag.String("release", envOr("CONTENT_RELEASE", "dev"), "content release tag")
		builderRepo   = flag.String("builder-repo", os.Getenv("GITHUB_REPOSITORY"), "builder repository")
		builderCommit = flag.String("builder-commit", os.Getenv("GITHUB_SHA"), "builder commit")
		only          = flag.String("courses", "", "comma-separated course slugs (default: all)")
		fetchBase     = flag.String("fetch-base", "", "override the codeload base URL (testing)")
	)
	flag.Parse()

	if err := run(*pinsPath, *dist, *release, *builderRepo, *builderCommit, *only, *fetchBase); err != nil {
		fmt.Fprintf(os.Stderr, "quest-content: %v\n", err)
		os.Exit(1)
	}
}

func run(pinsPath, dist, release, builderRepo, builderCommit, only, fetchBase string) error {
	pins, err := bundle.LoadPins(pinsPath)
	if err != nil {
		return err
	}
	var selected []string
	if only != "" {
		for _, slug := range strings.Split(only, ",") {
			if slug = strings.TrimSpace(slug); slug != "" {
				selected = append(selected, slug)
			}
		}
	}

	index, err := bundle.Build(context.Background(), bundle.Options{
		Pins:    pins,
		Dist:    dist,
		Release: release,
		Builder: bundle.Builder{Repo: builderRepo, Commit: builderCommit},
		Fetcher: bundle.Fetcher{BaseURL: fetchBase},
		Only:    selected,
	})
	if err != nil {
		return err
	}

	fmt.Printf("built %d course bundles for %s into %s\n", len(index.Courses), release, dist)
	for _, c := range index.Courses {
		fmt.Printf("  %-14s %-44s %8d bytes\n", c.Slug, c.Bundle.Asset, c.Bundle.Size)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
