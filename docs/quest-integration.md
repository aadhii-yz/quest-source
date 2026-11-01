# Consuming content from quest

This describes the intended app-side integration (quest `ROADMAP.md`, Phase 3).
Nothing here is implemented in quest yet.

## Trust chain

```
quest source pin  ->  content release tag + index.json sha256
index.json        ->  bundle asset name + sha256 + size
bundle manifest   ->  spec repo + commit + license
quest sources.go  ->  must equal manifest spec.commit
```

The app pins a **release tag and an index digest**. That is the only value that
must be updated to move content forward. Everything else is verified from the
index.

Recommended pin file in quest (e.g. `internal/provider/codecrafters/content.go`):

```go
const (
    ContentRelease = "content-2026.11"
    ContentIndexSHA256 = "<sha256 of index.json>"
    ContentRepo = "aadhii-yz/quest-source" // owner/repo, no scheme
)
```

URLs are built as:

```
https://github.com/<ContentRepo>/releases/download/<ContentRelease>/index.json
https://github.com/<ContentRepo>/releases/download/<ContentRelease>/<bundle.asset>
```

## Cache layout

Content lives beside tester assets, not inside the repo:

```
<cache>/content/<release>/index.json
<cache>/content/<release>/<asset>
<cache>/content/<release>/<slug>/       extracted bundle
<cache>/testers/...                     existing tester assets
```

The cache directory is created 0700 by quest's `internal/cache`. Content and
tester assets share the same verification helpers; do not add a second
downloader.

## Loading rules

1. **Index first, bundle on demand.** The catalog needs only `index.json`. Fetch
   a bundle when the course is opened or a project is created. Never fetch all
   bundles at startup.
2. **Cache-first.** If a cached index matches the pinned digest, use it without
   network. If a cached bundle matches the index digest, use it. A digest
   mismatch deletes and refetches; it is never a warning.
3. **Local override for development.** `QUEST_DEFS_DIR` (or a future
   `QUEST_CONTENT_DIR`) wins over remote content so contributors can work on
   `temp/` checkouts offline. Document the precedence in the UI when a course
   has no spec.
4. **No bundle at startup.** Startup must not block on the network. Today
   `provider.Provider.Courses()` is synchronous, so integration needs one of:
   - a `Load(ctx) ([]Course, error)` style method invoked from a `tea.Cmd`, with
     a loading state in the catalog, or
   - a content store that returns cached/index data synchronously and refreshes
     in the background.

   Pick one before implementing; see quest `ROADMAP.md` open decisions.

## Validation checklist

- [ ] index digest equals `ContentIndexSHA256`
- [ ] bundle digest and size equal the index entry
- [ ] manifest `format` is supported
- [ ] manifest `provider`/`course` equal the requested course
- [ ] manifest `spec.commit` equals the pinned commit in `sources.go`
- [ ] manifest `spec.license` equals the pinned `RepoLicense`
- [ ] `LICENSE` exists in the extracted bundle
- [ ] extractions are traversal-safe (reuse `internal/cache` extraction)
- [ ] nothing from the bundle is executed

## Mapping to the app model

- `provider.Course.Name` ← index `name` (fall back to definition name).
- `provider.Course.Summary` ← `short_description_md`, stripped like the current
  adapter's `plain()`.
- `provider.Course.Languages` ← index `languages` / definition languages.
- `provider.Course.Stages` ← stages from `course-definition.yml` (ordered,
  numbered 1..n).
- `provider.Course.Tester` ← existing tester pins in `sources.go`; content
  bundles never carry tester information.
- Stage description: today the app renders `marketing_md` from the definition.
  The bundle also ships `stage_descriptions/<slug>.md`; decide which is the
  canonical display text and document it.

## What not to do

- Do not fetch per-course files from `raw.githubusercontent.com` once bundles
  exist; that bypasses the digest chain.
- Do not trust the manifest without comparing `spec.commit` to quest's pins.
- Do not treat a bundle as executable, a starter template as trusted code, or a
  cached extraction as immutable without verifying before use.
- Do not add a "content channel" or "latest" resolution before the pinned path
  works end to end.
