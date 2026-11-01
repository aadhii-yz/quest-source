# quest-content

Per-course content bundles for QuesTUI (`quest`), built from pinned
CodeCrafters course repositories.

## Why

The app needs course definitions, stage descriptions and starter templates
without cloning upstream repositories on the user's machine. This repository
does that work once per content release and publishes small, immutable,
per-course bundles. The app downloads one course at a time and verifies its
SHA256 before extracting.

Bundles contain **only MIT-licensed course content**; see [Licensing](#licensing).

## Status

Skeleton with a working, tested builder. No content release has been published
yet.

- `courses.json` — pinned upstream repositories and commits (12 CodeCrafters
  courses; must match `internal/provider/codecrafters/sources.go` in quest).
- `internal/bundle` — pin validation, tarball download, filtered extraction,
  license validation, deterministic archive, manifest/index/checksums.
- `cmd/build` — CLI that produces a release directory.
- `docs/bundle-format.md` — the contract quest depends on.
- `docs/quest-integration.md` — how the app will consume releases.
- `.github/workflows` — CI and tag-driven release.

## Commands

```sh
go test ./...
go vet ./...
gofmt -l .

# Build everything (network: downloads pinned tarballs from codeload.github.com)
go run ./cmd/build -release dev

# Build a subset while iterating
go run ./cmd/build -release dev -courses grep,shell
```

Output lands in `dist/`:

```
dist/
  index.json
  checksums.txt
  codecrafters-grep-dev.tar.gz
  codecrafters-shell-dev.tar.gz
  ...
```

## Releasing

Tag the repository with a `content-*` tag and push it:

```sh
git tag content-2026.11
git push origin content-2026.11
```

The release workflow runs the tests, builds every bundle with the tag as
`content_release`, verifies `checksums.txt`, and publishes:

- every `codecrafters-<course>-<release>.tar.gz`
- `index.json`
- `checksums.txt`

Release assets are immutable: never re-upload an asset for an existing tag.
Cut a new tag instead.

## Licensing

Course definitions, stage descriptions and starter templates come from
`codecrafters-io/build-your-own-*` repositories, which are MIT licensed.

- Every bundle embeds the upstream `LICENSE` file.
- `manifest.json` records the source repository, pinned commit and license.
- The builder **fails** if the upstream `LICENSE` is missing, is not supported
  (currently MIT only), or does not contain the expected MIT text.
- Bundles never include tester source, tester binaries, solutions or compiled
  starters. `solutions/`, `compiled_starters/`, `dockerfiles/` and `README.md`
  are filtered out.
- Bundles do not include Advent of Code puzzle text; that content is not
  redistributable.

This repository's own tooling and docs are MIT licensed (see `LICENSE`); the
bundled upstream content remains under its upstream MIT license, attributed in
each bundle.

## Repository rules

See [AGENTS.md](AGENTS.md) for the rules agents must follow (pin discipline,
license checks, determinism, exclusions) and for the current handoff state.
