# Work Order: wo4-async-doc-freshness-dreaming

> **PR:** pr-11-v2-doc-seeding-and-lifecycles | **WO:** 4 of 5
> **Branch:** `pr-11-v2-doc-seeding-and-lifecycles/wo4-async-doc-freshness-dreaming`
> **Base:** `pr-11-v2-doc-seeding-and-lifecycles/wo3-doc-seed-source`
> **Complexity:** `standard`

## Objective

Implement Async Doc Freshness ("Dreaming", `SPEC.md` §6.3.5, `docs/design-v2-distillation-retrieval.md` §7.6, `docs/addendum-a-provenance-and-hardening.md` §11.1) in `internal/core/tick/dream_freshness.go` and wire it into `PipelineExecutor.Run` (`internal/core/tick/pipeline.go`). During `tick`, compare each `source: "doc-seed"` entry's `source_doc_blob_sha` against the current git blob SHA of `source_doc_path` on disk; when a seeded markdown file is modified or deleted in the repository (or when `doc_seeds` / `KindDreamFreshness` / `KindSeedDocs` triggers), deterministically re-seed/amend/retire the affected `doc-seed` entries, re-project `.memory/dossiers/` and `.memory/ATLAS.md`, and rebuild `index.db`.

## Context & Architectural Constraints

- **Strictly zero internal references:** Never include `google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or local `/usr/local/google/...` paths in any files, comments, or commit messages.
- **Deterministic, zero-LLM freshness check:** Per `SPEC.md` §6.3.5 and `design-v2` §7.6, doc freshness during `tick` is deterministic when using `DocSeedSource`:
  - If a seeded markdown document's git blob SHA (`seed.ComputeGitBlobSHA`) matches `source_doc_blob_sha` on all its active `doc-seed` entries, it is skipped in O(1) without emitting ops.
  - If a seeded markdown document was modified on disk (blob SHA changed), `seed.SeedDocs` re-parses the file, emits `OpAmend` (and `OpMove` if scope changed) for updated entries, `OpAdd` for new entries, and `OpRetire` (`reason: "source_doc_updated: <path> (<sha>)"`) for removed bullets.
  - If a seeded markdown document was deleted from the repo, `seed.SeedDocs` emits `OpRetire` (`reason: "source_doc_deleted: <path>"`) for all active `doc-seed` entries originating from that file.
- **Fast no-op for unseeded workspaces:** If `.memory/ops` does not exist, or if there are no active `source: "doc-seed"` entries, no configured `cfg.DocSeeds`, and no queued `jobs.KindDreamFreshness` / `jobs.KindSeedDocs` jobs, `checkDocFreshness` must return rapidly without mutating `.memory/`.

## Checklist of Changes

- [x] **1. Implement `CheckDocFreshness` in `internal/core/tick/dream_freshness.go`**:
  - Create `internal/core/tick/dream_freshness.go` in package `tick`.
  - Add exported method `func (p *PipelineExecutor) CheckDocFreshness(ctx context.Context) (*seed.DocSeedResult, error)` and internal helper `func (p *PipelineExecutor) checkDocFreshness(ctx context.Context) error`.
  - Drain any queued `jobs.KindDreamFreshness` and `jobs.KindSeedDocs` jobs from `p.store` (if `p.store != nil`), completing them once the freshness check succeeds (or failing them if it returns an error).
  - Resolve `opsDir := filepath.Join(p.repoRoot, ".memory", "ops")` and `configPath := filepath.Join(p.repoRoot, ".memory", "config.yaml")`.
  - Load `repoCfg` via `config.LoadRepoConfig(p.repoRoot)` (merging with `p.cfg` if `len(p.cfg.DocSeeds) > 0`) and `scopeCfg` via `config.LoadScopeConfig(configPath)` (ignoring `os.IsNotExist`).
  - Inspect active `source: "doc-seed"` entries via `ops.NewReader(opsDir)` / `dossiers.NewProjector(opsReader).Project(ctx)` if `opsDir` exists.
  - If there are no active `source: "doc-seed"` entries, `len(repoCfg.DocSeeds) == 0`, and no `KindDreamFreshness` / `KindSeedDocs` job was acquired, return `&seed.DocSeedResult{}, nil` without creating `.memory/dossiers` or writing ops.
  - Otherwise, run `seed.SeedDocs(ctx, seed.DocSeedOptions{RepoRoot: p.repoRoot, OpsDir: opsDir, RepoCfg: repoCfg}, scopeCfg, opsReader, ops.NewWriter(opsDir))`.
  - When `res.EntriesAdded + res.EntriesAmend + res.EntriesRetire > 0`:
    - Re-project `.memory/dossiers/` and `.memory/ATLAS.md` via `dossiers.NewProjector(opsReader)` and `dossiers.NewWriter(filepath.Join(p.repoRoot, ".memory")).WriteAll(dossiersMap)`.
    - If `p.idx != nil`, call `p.idx.RebuildFromOpLog(ctx, opsReader)`; if `p.idx == nil` and `p.dataDir != ""` and `p.repoID != ""`, open `index.NewIndex(index.IndexPath(p.dataDir, p.repoID))` (closing via `defer`), and call `idx.RebuildFromOpLog(ctx, opsReader)`.
- [x] **2. Wire `checkDocFreshness` into `PipelineExecutor.Run` (`internal/core/tick/pipeline.go`)**:
  - In `PipelineExecutor.Run(ctx context.Context) error`, call `if err := p.checkDocFreshness(ctx); err != nil { return err }` right after `convertCapturesToEvidence(ctx)` and before the index rebuild step.
- [x] **3. Add Unit Tests in `internal/core/tick/dream_freshness_test.go`**:
  - `TestCheckDocFreshness_NoOpWhenUnseeded`: Verify that running `exec.Run(ctx)` on a workspace without `doc-seed` entries or `doc_seeds` config succeeds as a fast no-op.
  - `TestCheckDocFreshness_DetectsModifiedAndDeletedDocs`:
    - Set up a temp repo with a valid anchor file (`internal/foo/bar.go`), `.memory/config.yaml` with `doc_seeds: ["docs/arch.md"]`, and `docs/arch.md` containing 2 anchored bullets.
    - Run `exec.CheckDocFreshness(ctx)` (or `exec.Run(ctx)`): verify `EntriesAdded == 2`, `.memory/dossiers/` is projected with `source_doc: docs/arch.md` and `kind:`, and `index.db` contains the 2 `doc-seed` entries (`idx.ListEntriesBySource(ctx, ops.SourceDocSeed)`).
    - Run `exec.CheckDocFreshness(ctx)` again without modifying `docs/arch.md`: verify `FilesSkipped == 1` and `EntriesAdded == 0, EntriesAmend == 0, EntriesRetire == 0`.
    - Modify `docs/arch.md` (change 1 bullet's text and remove the 2nd bullet): run `exec.CheckDocFreshness(ctx)` and verify 1 entry is amended/updated with the new `source_doc_blob_sha` and 1 entry is retired (`EntriesRetire == 1`), and both `.memory/dossiers/` and `index.db` reflect the updated state.
    - Delete `docs/arch.md` from disk: run `exec.CheckDocFreshness(ctx)` and verify the remaining entry is retired (`EntriesRetire == 1`) with reason `"source_doc_deleted: docs/arch.md"`, and `idx.ListEntriesBySource(ctx, ops.SourceDocSeed)` returns 0 active entries.
  - `TestCheckDocFreshness_DrainsDreamFreshnessJob`:
    - Verify that enqueuing a `jobs.KindDreamFreshness` job in `jobStore` and running `exec.Run(ctx)` transitions the job to `jobs.StateDone`.

## Verification Criteria

Run:
```bash
go test -race ./internal/core/tick/... ./internal/core/seed/... ./cmd/repo-memory/...
```
All tests must pass with zero race warnings.

---

## Completion Report

- **Status:** Complete
- **Commits:** `f54c27f`
- **Summary of Changes:**
  - Implemented `PipelineExecutor.CheckDocFreshness` and `checkDocFreshness` in `internal/core/tick/dream_freshness.go` to drain queued `KindDreamFreshness` and `KindSeedDocs` jobs, perform a fast no-op on unseeded workspaces, reconcile modified/deleted markdown docs via `seed.SeedDocs`, re-project `.memory/dossiers/` and `.memory/ATLAS.md`, and rebuild `index.db`.
  - Wired `p.checkDocFreshness(ctx)` into `PipelineExecutor.Run` in `internal/core/tick/pipeline.go`.
  - Added unit tests in `internal/core/tick/dream_freshness_test.go` covering unseeded no-op, modified/deleted doc detection (`source_doc_updated` / `source_doc_deleted`), and `KindDreamFreshness` job completion.
- **Verification Output:**
  - `go test -race -v ./internal/core/tick/... ./internal/core/seed/... ./cmd/repo-memory/...` — PASS (0 race warnings).
