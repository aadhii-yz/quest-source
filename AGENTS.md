# AGENTS.md — quest-content

Guidance for agents working in this repository.

## What this repo is

A content build pipeline for QuesTUI. It pins upstream MIT course repositories,
extracts an allow-listed subset, and publishes per-course bundles plus an index
and checksums. The app consumes those release assets; it never clones upstream
repos at runtime.

This repository is checked out at `quest/content/` and is gitignored by the
quest repo. It is its own git repository with its own CI and releases.

## Layout

```
courses.json              pinned upstream repos: slug, repo URL, commit, license
cmd/build/main.go         CLI: load pins -> fetch -> validate -> bundle -> index
internal/bundle/
  pins.go                 Pins/Pin types, repo URL parsing, pin validation
  fetch.go                codeload tarball download by commit SHA
  extract.go              filtered, traversal-safe tar.gz extraction
  license.go              supported licenses + LICENSE text validation
  definition.go           course-definition.yml subset parsing/validation
  manifest.go             per-bundle manifest.json
  archive.go              deterministic tar.gz writer
  index.go                index.json catalog
  build.go                orchestration, asset naming, checksums
  bundle_test.go          offline tests (httptest tarballs only)
docs/bundle-format.md     the contract quest depends on
docs/quest-integration.md how the app consumes a release
.github/workflows/        ci.yml, release.yml
```

## Commands

```sh
go build ./...
go vet ./...
gofmt -l .                # must print nothing
go test ./...
go run ./cmd/build -release dev               # network; downloads pinned tarballs
go run ./cmd/build -release dev -courses grep # subset
```

Tests must stay offline: serve synthetic tarballs with `httptest`, never hit
GitHub.

## Rules

Pinning:

- Every pin is a full lowercase 40-character commit SHA. Never a branch, tag or
  `latest`.
- `courses.json` must match the spec commits pinned in quest's
  `internal/provider/codecrafters/sources.go`. When quest bumps a spec commit,
  bump this repository in the same change window and cut a content release.
- Never change a published release's assets. Cut a new tag.

Licensing:

- Bundle only MIT-licensed upstream content. Only `MIT` is in
  `SupportedLicenses` today; extend deliberately, with the license text checks
  in `license.go`.
- Every bundle must contain the upstream `LICENSE` at `LICENSE`. The build fails
  if it is missing or does not contain the MIT grant and notice text.
- Never add tester source, tester binaries, decompiled tests, solutions,
  compiled starters, dockerfiles, testimonials or avatars to a bundle.
- Keep `AllowedPaths` minimal. Adding a path is a licensing decision: document
  it here and in `docs/bundle-format.md`.
- Do not add Advent of Code or other non-redistributable content.

Build:

- Builds must be deterministic: sorted archive entries, zeroed timestamps and
  ownership, fixed gzip header. `TestBuildDeterministic` guards this.
- Extraction must stay fail-closed: reject absolute paths, traversal,
  backslashes, duplicate entries, links and oversized files.
- Never add a "skip checksum" or "skip license" option.
- Do not fetch anything except the pinned codeload tarball. No GitHub API
  pagination, no scraping.

Repository:

- This is not the app. Do not add TUI code here.
- Keep `docs/bundle-format.md` and `docs/quest-integration.md` in sync with the
  code and with quest's `ROADMAP.md`.

## Handoff: next tasks

1. Run the builder against the real pins and fix anything the synthetic fixtures
   did not catch:
   ```sh
   go run ./cmd/build -release dev
   ```
   Check bundle sizes, languages/stage counts in `dist/index.json`, and that
   every bundle's `manifest.json` digest matches `index.json`.
2. Inspect one extracted bundle by hand (grep or shell) for unexpected files.
3. Publish the first `content-0.1.0` release once (1) and (2) pass.
4. Add a CI job that fails if `courses.json` commits drift from quest's pins.
   Parse `sources.go` with a small Go program or run the check from a quest
   checkout in the quest repo instead; do not guess.
5. In quest (`ROADMAP.md`, Phase 3), implement the consumer:
   pin this release tag + `index.json` digest, fetch through `internal/cache`,
   validate manifest `spec.commit` against `sources.go`, and extract into the
   content cache.
6. Optional later: signing the index/manifest, or a user-selectable content
   channel. Both are out of scope until the pinned path works end to end.

Commit messages: lowercase, semicolon-separated clauses, no Conventional
Commits prefix. End with:

```
Co-Authored-By: DeepSeek-V4.1-Flash via Pi Agent
```
