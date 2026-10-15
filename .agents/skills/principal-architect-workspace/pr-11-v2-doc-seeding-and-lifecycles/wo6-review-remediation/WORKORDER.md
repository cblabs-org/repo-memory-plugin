# Work Order: wo6-review-remediation

> **PR:** pr-11-v2-doc-seeding-and-lifecycles | **WO:** 6 of 6 (Review Round 1 Remediation)
> **Branch:** `pr-11-v2-doc-seeding-and-lifecycles/wo5-dogfood-doc-seed-and-verification`
> **Base:** `pr-11-v2-doc-seeding-and-lifecycles/wo5-dogfood-doc-seed-and-verification` (`80b9c2e`)
> **Complexity:** `standard`

## Objective

Resolve all 3 findings (`[MAJOR-1]`, `[MAJOR-2]`, `[MINOR-1]`) from the Cold-Firewall PR Audit Round 1 on GitHub PR #19, re-seed `.memory/` cleanly so `repo-memory seed-docs` and `repo-memory ops gc` are mutually idempotent with zero polluted scopes, and verify all tests pass.

## Context & Architectural Constraints

- **Strictly zero internal references:** Never include `google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or local `/usr/local/google/...` paths in any files, comments, or commit messages.
- **Finding `[MAJOR-1]` (`internal/core/seed/doc_seed.go`, `.memory/`):**
  - `isPollutedScope(scope string) bool` in `internal/core/seed/gc.go` (same package `seed`) defines polluted scopes (dot-prefixed scopes like `.memory`, `.agents`, `.agents/agents`, root files like `Makefile`, `Dockerfile`, `LICENSE`, etc.).
  - In `SeedDocs` (`internal/core/seed/doc_seed.go:407-413`), `collectRepoFileAnchors(opts.RepoRoot)` walks `.agents` and root files (`Makefile`), causing `config.DeriveModuleScopes(repoFiles)` to create polluted module scopes.
  - In `ParseDocToOps` (`internal/core/seed/doc_seed.go:272-298`), `anchors` containing `.memory/...`, `.agents/...`, or `Makefile` are passed directly into `resolver.ResolveScope(anchors, keywords)` and `config.DeriveModuleScopes(anchors)` without filtering out dot-prefixed or slashless root-file anchors (`strings.Contains(a, "/") && !strings.HasPrefix(a, ".")`) or checking `isPollutedScope(dossierScope)`. Note: `anchors` themselves (`".memory/config.yaml"`, `"Makefile"`, etc.) can remain in `op.Fields["anchors"]`, `scopeAnchors` used for scope resolution must exclude dot-prefixed and slashless paths, and any resulting `dossierScope` where `dossierScope == "" || isPollutedScope(dossierScope)` must fall back to `"root"`.
- **Finding `[MAJOR-2]` (`internal/core/dossiers/writer.go`, `dossiers_test.go`, `dream_freshness_test.go`):**
  - In `Writer.WriteAll` (`internal/core/dossiers/writer.go:48-76`), when all active entries in a dossier are retired (`len(d.Entries) == 0` and `len(d.History) > 0`), `WriteAll` currently calls `WriteDossier(d)` (which writes an empty `# <scope>\n` file to `.memory/dossiers/<scope>.md`) and appends an `AtlasEntry` with `EntryCount: 0` and `LastUpdated: 0001-01-01T00:00:00Z` to `.memory/ATLAS.md`. Furthermore, stale `.md` files in `.memory/dossiers/` from scopes that now have `0` active entries or are absent from `dossiers` are left behind on disk.
- **Finding `[MINOR-1]` (`internal/core/seed/doc_seed.go`):**
  - In `SeedDocs` (`internal/core/seed/doc_seed.go:468-536`), when a scanned markdown file produces `len(candidateOps) == 0` and has `len(prevEntries) == 0` (`stat.Added == 0 && stat.Amended == 0 && stat.Retired == 0`), `SeedDocs` does not increment `res.FilesSkipped++` (or set `stat.Skipped = true`).

## Checklist of Changes

- [x] **1. Fix `[MAJOR-1]` in `internal/core/seed/doc_seed.go` & `internal/core/seed/doc_seed_test.go`**:
  - In `collectRepoFileAnchors(repoRoot string) []string` (`internal/core/seed/doc_seed.go:806-835`):
    - Skip any dot-prefixed directory (`if strings.HasPrefix(base, ".") && base != "." { return filepath.SkipDir }`).
    - Only append regular files that contain `/` and do not start with `.` (`if d.Type().IsRegular() && strings.Contains(relSlash, "/") && !strings.HasPrefix(relSlash, ".")`).
  - In `SeedDocs` (`internal/core/seed/doc_seed.go:407-413`):
    - Filter `moduleScopes := config.DeriveModuleScopes(repoFiles)` to exclude any `m` where `isPollutedScope(m.Name)` is true before passing to `config.NewScopeResolver`.
  - In `ParseDocToOps` (`internal/core/seed/doc_seed.go:272-299`):
    - Build `scopeAnchors` from `anchors` by keeping only anchors with `strings.Contains(a, "/") && !strings.HasPrefix(a, ".")` (matching `RunOpsGC` in `internal/core/seed/gc.go:220-225`).
    - Pass `scopeAnchors` (instead of raw `anchors`) to `resolver.ResolveScope(scopeAnchors, keywords)` and `config.DeriveModuleScopes(scopeAnchors)`.
    - If `dossierScope == "" || isPollutedScope(dossierScope)`:
      - Try `defScope := deriveDocDefaultScope(relPath)` if `isSkill` and `!isPollutedScope(defScope)`.
      - Otherwise set `dossierScope = "root"`.
    - Keep `anchors` in `op.Fields["anchors"]` intact so valid file anchors like `.memory/config.yaml` or `Makefile` are still recorded on the entry inside `root.md` (or the matched declared scope).
  - Add a unit test `TestParseDocToOps_NoPollutedScopes` in `internal/core/seed/doc_seed_test.go` verifying that doc bullets referencing `.memory/config.yaml`, `.agents/skills`, and `Makefile` resolve to `"root"` (never `.memory`, `.agents`, or `Makefile`) and that running `RunOpsGC` after `SeedDocs` reports `ScopesRepaired == 0`.
- [x] **2. Fix `[MAJOR-2]` in `internal/core/dossiers/writer.go`, `dossiers_test.go`, and `internal/core/tick/dream_freshness_test.go`**:
  - In `Writer.WriteDossier(d *Dossier) error` (`internal/core/dossiers/writer.go:19-40`):
    - When `len(d.Entries) == 0`:
      - Do NOT write `scopePath` (`filepath.Join(dossiersDir, d.Scope+".md")`); instead, remove `scopePath` if it exists (`_ = os.Remove(scopePath)`).
      - If `len(d.History) > 0`, `os.MkdirAll(filepath.Dir(historyPath), 0755)` and write `historyPath` (`filepath.Join(dossiersDir, d.Scope+".history.md")`).
      - Return `nil`.
  - In `Writer.WriteAll(dossiers map[string]*Dossier) error` (`internal/core/dossiers/writer.go:48-76`):
    - Track expected `.md` and `.history.md` file paths written during `WriteAll`.
    - When `len(d.Entries) == 0`:
      - If `len(d.History) > 0`, call `w.WriteDossier(d)` (which writes `<scope>.history.md` and removes `<scope>.md`), and record `<scope>.history.md` in the expected file set.
      - Do NOT append an `AtlasEntry` to `atlas.Dossiers` when `len(d.Entries) == 0` (so `ATLAS.md` never has `0`-entry or `0001-01-01T00:00:00Z` rows).
    - Walk `filepath.Join(w.baseDir, "dossiers")` (if it exists) and remove any `.md` files not in the expected file set (cleaning up orphaned `.md` files and any newly-empty subdirectories).
  - Add unit test coverage in `internal/core/dossiers/dossiers_test.go` and `internal/core/tick/dream_freshness_test.go` verifying that when all entries in a scope are retired (e.g. when `docs/arch.md` is deleted in `TestCheckDocFreshness_DetectsModifiedAndDeletedDocs`), `<scope>.md` does not exist on disk, `<scope>.history.md` exists on disk, and `.memory/ATLAS.md` contains no `0001-01-01T00:00:00Z` or `| 0 |` entry row.
- [x] **3. Fix `[MINOR-1]` in `internal/core/seed/doc_seed.go`**:
  - In `SeedDocs` (`internal/core/seed/doc_seed.go:533-536`), when `stat.Added > 0 || stat.Amended > 0 || stat.Retired > 0`, increment `res.FilesSeeded++`; `else` (when `stat.Added == 0 && stat.Amended == 0 && stat.Retired == 0`), set `stat.Skipped = true` and increment `res.FilesSkipped++`.
- [x] **4. Clean & Re-Seed `.memory/`, Rebuild `~/.local/bin/repo-memory` & `index.db`**:
  - Remove the previous `.memory/ops/2026-10/01M50MAEJDR5SPBR8M771BEMF0.json` batch.
  - Rebuild `~/.local/bin/repo-memory`:
    ```bash
    go build -o ~/.local/bin/repo-memory ./cmd/repo-memory
    ```
  - Run `~/.local/bin/repo-memory ops gc --apply` first to repair any pre-existing polluted scopes in `.memory/ops` (if any).
  - Run `~/.local/bin/repo-memory seed-docs` (and verify a 2nd run reports `0 added, 0 amended, 0 retired` and `3 unchanged skipped`).
  - Run `~/.local/bin/repo-memory ops gc` (dry-run) and verify it reports `0` scopes needing repair (`ScopesRepaired == 0`), proving `seed-docs` and `ops gc` are mutually idempotent.
  - Verify `.memory/dossiers/.agents.md`, `.memory/dossiers/.agents/agents.md`, `.memory/dossiers/.memory.md`, and `.memory/dossiers/Makefile.md` do NOT exist on disk and are NOT listed in `.memory/ATLAS.md`.
  - Run `~/.local/bin/repo-memory reindex` to rebuild `<data-dir>/<repo-id>/index.db`.

## Verification Criteria

Run:
```bash
go build ./cmd/... ./internal/... ./evaluation/cmd/build-index/... && \
go vet ./cmd/... ./internal/... ./evaluation/cmd/build-index/... && \
GOOS=windows go build ./cmd/... ./internal/... ./evaluation/cmd/build-index/... && \
go test -race -count=1 ./cmd/... ./internal/...
```
All commands must exit 0.

---

## Completion Report

- **Status:** Complete
- **Commits:**
  - `23f082d`: `fix(seed,dossiers): prevent polluted doc-seed scopes and clean retired dossiers`
- **Summary of Changes:**
  - **`[MAJOR-1]` (`internal/core/seed/doc_seed.go`, `internal/core/seed/doc_seed_test.go`, `.memory/`)**: Filtered dot-prefixed directories and root-level files in `collectRepoFileAnchors`, filtered `DeriveModuleScopes(repoFiles)` against `isPollutedScope`, restricted `scopeAnchors` in `ParseDocToOps` to `strings.Contains(a, "/") && !strings.HasPrefix(a, ".")` with fallback to `"root"` when `dossierScope == "" || isPollutedScope(dossierScope)`, and added `TestParseDocToOps_NoPollutedScopes`.
  - **`[MAJOR-2]` (`internal/core/dossiers/writer.go`, `internal/core/dossiers/dossiers_test.go`, `internal/core/tick/dream_freshness_test.go`)**: Updated `Writer.WriteDossier` to remove `<scope>.md` and only write `<scope>.history.md` when `len(d.Entries) == 0`; updated `Writer.WriteAll` to exclude `0`-entry dossiers from `ATLAS.md` and prune orphaned `.md` files and empty directories under `.memory/dossiers/`.
  - **`[MINOR-1]` (`internal/core/seed/doc_seed.go`)**: Incremented `res.FilesSkipped++` and set `stat.Skipped = true` when a scanned markdown file yields zero added/amended/retired ops.
  - **Dogfood Re-Seed (`.memory/`)**: Replaced `.memory/ops/2026-10/01M50MAEJDR5SPBR8M771BEMF0.json` with `01M50N8B38698HRTF64HBG369H.json` (12 entries across `core/seed`, `eval`, and `root`), removed `.memory/dossiers/{.agents,.memory,Makefile}.md`, verified `seed-docs` + `ops gc --dry-run` idempotence (`scopes_repaired=0`), and rebuilt `~/.local/bin/repo-memory` and `index.db`.
- **Verification Output:**
  - `go build ./cmd/... ./internal/... ./evaluation/cmd/build-index/...`: exit 0
  - `go vet ./cmd/... ./internal/... ./evaluation/cmd/build-index/...`: exit 0
  - `GOOS=windows go build ./cmd/... ./internal/... ./evaluation/cmd/build-index/...`: exit 0
  - `go test -race -count=1 ./cmd/... ./internal/...`: exit 0 (all packages passed)
