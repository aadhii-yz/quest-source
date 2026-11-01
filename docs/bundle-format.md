# Bundle format

This is the contract between `quest-source` (producer) and quest (consumer).
Treat every field and filename as a wire format.

## Release assets

For a release tag such as `content-2026.11`:

```
index.json                                  catalog + digests for every course
checksums.txt                               sha256sum -c compatible, all assets
codecrafters-<slug>-content-2026.11.tar.gz  one bundle per course
```

Asset name: `codecrafters-<slug>-<sanitized-release>.tar.gz`, where the release
is lowercased and any character outside `[A-Za-z0-9._-]` becomes `-`.

Assets are immutable. Never replace an asset for an existing tag.

## Bundle contents

```
manifest.json                metadata (see below)
course-definition.yml        verbatim from the pinned commit
LICENSE                      verbatim upstream MIT license
stage_descriptions/**        verbatim
starter_templates/**         verbatim
```

Nothing else. The builder filters everything outside `AllowedPaths`:
`solutions/`, `compiled_starters/`, `dockerfiles/`, `README.md`, tester source
and tester binaries are excluded.

The archive is deterministic: entries are sorted, mtimes and ownership are
zeroed, and the gzip header is fixed, so the same inputs always produce the same
bytes and digest.

## manifest.json

```json
{
  "format": 1,
  "content_release": "content-2026.11",
  "provider": "codecrafters",
  "course": "grep",
  "name": "Build your own Grep",
  "spec": {
    "repo": "https://github.com/codecrafters-io/build-your-own-grep",
    "commit": "6d54daaa...(40 hex)",
    "license": "MIT",
    "license_file": "LICENSE"
  },
  "builder": {
    "repo": "aadhii-yz/quest-source",
    "commit": "<40 hex or empty for local dev builds>"
  },
  "files": {
    "LICENSE": "<sha256>",
    "course-definition.yml": "<sha256>",
    "stage_descriptions/<stage>.md": "<sha256>",
    "starter_templates/<language>/...": "<sha256>"
  }
}
```

Rules:

- `format` is `1`. Consumers must reject unknown major formats with a clear
  message.
- `spec.commit` must equal the commit quest has pinned for that course.
- `spec.license` is the SPDX identifier checked against the supported list.
- `files` lists every bundled file except `manifest.json` itself.
- `builder` is provenance only; it is not used for trust decisions.

## index.json

```json
{
  "format": 1,
  "content_release": "content-2026.11",
  "provider": "codecrafters",
  "courses": [
    {
      "slug": "grep",
      "name": "Build your own Grep",
      "short_description_md": "Learn about regex syntax: ...",
      "license": "MIT",
      "spec_commit": "6d54daaa...(40 hex)",
      "stages": 33,
      "languages": ["c", "cpp", "go", "..."],
      "bundle": {
        "asset": "codecrafters-grep-content-2026.11.tar.gz",
        "sha256": "<bundle sha256>",
        "size": 123456
      },
      "manifest_sha256": "<manifest.json sha256>"
    }
  ]
}
```

Courses are sorted by slug. The index is intentionally small: the app can
render its catalog from it without downloading bundles. `short_description_md`
is raw markdown; consumers strip it for one-line display.

## checksums.txt

`sha256sum` format, one line per asset in the release except `checksums.txt`
itself:

```
<sha256>  <asset>
```

## Consumer verification (required)

1. Pin the release tag and the `index.json` digest in the app. Never resolve
   `latest`.
2. Fetch `index.json`, verify its digest, cache it.
3. Pick the course entry, then fetch that bundle and verify the digest and size
   from the index before extracting.
4. Extract safely (no absolute paths, traversal, duplicate entries, links or
   oversized files).
5. Read `manifest.json`; reject if `format` is unknown, the provider/course do
   not match, or `spec.commit` differs from the app's pinned spec commit.
6. Verify that the bundle contains `LICENSE` and (optionally) that the file
   digests match the manifest.
7. Never execute anything from a content bundle. Bundles are data only.
