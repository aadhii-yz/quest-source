package bundle

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultCodeloadBase is GitHub's tarball endpoint. Fetching by commit SHA is
// deterministic and avoids cloning repository history.
const DefaultCodeloadBase = "https://codeload.github.com"

// maxArchiveSize caps a single downloaded repository tarball.
const maxArchiveSize = 256 << 20

// Fetcher downloads pinned repository tarballs.
type Fetcher struct {
	// Client is optional; a client with a 10 minute timeout is used otherwise.
	Client *http.Client
	// BaseURL overrides the codeload endpoint (used by tests).
	BaseURL string
}

// Fetch downloads the tarball for a pinned course repository at its commit.
func (f Fetcher) Fetch(ctx context.Context, pin Pin) ([]byte, error) {
	owner, name, err := RepoParts(pin.Repo)
	if err != nil {
		return nil, err
	}
	if !commitPattern.MatchString(pin.Commit) {
		return nil, fmt.Errorf("%s: invalid commit %q", pin.Slug, pin.Commit)
	}

	base := strings.TrimSuffix(f.BaseURL, "/")
	if base == "" {
		base = DefaultCodeloadBase
	}
	url := fmt.Sprintf("%s/%s/%s/tar.gz/%s", base, owner, name, pin.Commit)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "quest-content/1 (+https://github.com/aadhii-yz/quest)")

	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: fetch %s: %w", pin.Slug, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("%s: fetch %s: %s: %s", pin.Slug, url, resp.Status, strings.TrimSpace(string(body)))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveSize+1))
	if err != nil {
		return nil, fmt.Errorf("%s: read %s: %w", pin.Slug, url, err)
	}
	if len(data) > maxArchiveSize {
		return nil, fmt.Errorf("%s: archive for %s exceeds %d bytes", pin.Slug, pin.Repo, maxArchiveSize)
	}
	return data, nil
}
