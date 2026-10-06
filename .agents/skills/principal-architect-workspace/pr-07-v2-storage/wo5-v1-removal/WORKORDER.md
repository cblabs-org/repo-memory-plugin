---
name: pr-07-v2-storage-wo5
description: Handoff instructions for Work Order 5 — v1 Removal
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Remove the v1 pipeline stages (triage, extract, reconcile, verify), the v1 claims package, the v1 events package, and the v1 views generator. **Keep** the executor/job chaining, local-publish worktree, 429 handling, and the compact stage. Update all consumers to remove dead imports.

# Context

## What to REMOVE
| Package | Files | Why |
|---|---|---|
| `internal/core/pipeline/triage/` | 6 files | Superseded by turn-note distillation in PR-10+ |
| `internal/core/pipeline/extract/` | 3 files | Superseded by turn-note distillation |
| `internal/core/pipeline/reconcile/` | 3 files | Superseded by consolidation in PR-11 |
| `internal/core/pipeline/verify/` | 3 files | Superseded by dossier cap linting |
| `internal/core/claims/` | 7 files (models, parser, store, ulid) | Superseded by `internal/core/dossiers/` entries |
| `internal/core/events/` | 4 files (models, store) | Superseded by `internal/core/ops/` op log |
| `internal/core/views/` | 2 files (generator) | Superseded by dossier renderer |

## What to KEEP
| Package | Why |
|---|---|
| `internal/core/pipeline/publish/` | local-publish worktree still used by v2 |
| `internal/core/pipeline/compact/` | compact still useful for evidence management |
| `internal/core/tick/pipeline.go` | **Must be gutted** — remove references to triage/extract/reconcile/verify/claims, but keep the executor shell, 429 handling, and publish integration |
| `cmd/repo-memory/cmd_reindex.go` | Update: remove claims/store import, wire to v2 `RebuildFromOpLog` |
| `cmd/repo-memory/cmd_views.go` | **Remove** — superseded by dossier rendering |

## Consumers to Update
- `internal/core/tick/pipeline.go` — imports triage, extract, reconcile, verify, claims/store. **Gut the v1 pipeline stages.** Keep the executor structure, publish, and 429 handling. The pipeline runner should become a no-op placeholder that logs "v1 pipeline stages removed, v2 distillation not yet implemented".
- `cmd/repo-memory/cmd_reindex.go` — imports claims/store. Update to use v2 `index.RebuildFromOpLog`.
- `cmd/repo-memory/cmd_views.go` — imports views. **Delete this file.**
- `internal/core/index/rebuild.go` — has `RebuildV1Index` or `Rebuild` that imports claims. Remove the v1 rebuild function. Keep `RebuildFromOpLog`.
- `internal/core/index/index.go` — remove v1 tables (`claims`, `anchors`, `relations`, `conditions`) from `initSchema()`. Keep `entities`, `entity_relations`, `episodes_fts`.

# Strict Boundary Box

**DO NOT touch:**
- `internal/core/ops/` — op log (WO1)
- `internal/core/dossiers/` — dossiers (WO2)
- `internal/core/config/scopes.go` — scopes (WO3)
- `internal/core/index/entries.go` — v2 index entries (WO4)
- `internal/core/index/entry_anchors.go` — v2 anchor lookups (WO4)
- `internal/core/index/v2_test.go` — v2 index tests (WO4)
- `internal/core/delivery/` — delivery (PR-08)
- `internal/harness/` — harness code
- `.agents/` — PA workspace

**DO remove/modify:**
- Files listed above in "What to REMOVE"
- Consumer files listed above
- `internal/core/index/index.go`, `rebuild.go`, `populate.go` — schema and rebuild changes

# Tooling Constraints

- **Build:** `go build ./...` must pass — this is the critical check. All dead imports must be resolved.
- **Test:** `go test ./...` must pass (full suite)
- **Vet:** `go vet ./...` must pass

# Line-Item Instructions

## 1. Remove v1 Packages
- [ ] Delete `internal/core/pipeline/triage/` (entire directory)
- [ ] Delete `internal/core/pipeline/extract/` (entire directory)
- [ ] Delete `internal/core/pipeline/reconcile/` (entire directory)
- [ ] Delete `internal/core/pipeline/verify/` (entire directory)
- [ ] Delete `internal/core/claims/` (entire directory, including `store/`)
- [ ] Delete `internal/core/events/` (entire directory)
- [ ] Delete `internal/core/views/` (entire directory)

## 2. Update tick/pipeline.go
- [ ] Remove imports: triage, extract, reconcile, verify, claims, claims/store
- [ ] Gut the `PipelineExecutor` struct: remove claimStore and stage fields
- [ ] Gut the `Run` method: replace v1 stage execution with a log message: `"v1 pipeline stages removed; v2 distillation not yet implemented"`
- [ ] **Keep:** the `PipelineExecutor` struct shell, `NewPipelineExecutor` constructor, error handling, 429 detection, and any publish-related code

## 3. Update cmd/
- [ ] Delete `cmd/repo-memory/cmd_views.go`
- [ ] Update `cmd/repo-memory/cmd_reindex.go`: remove claims/store import, update to call `index.RebuildFromOpLog` with an `ops.NewReader` instead of the v1 rebuild
- [ ] Verify all CLI subcommand registration still compiles (check `main.go` or `root.go` for views command registration and remove it)

## 4. Update index schema
- [ ] In `internal/core/index/index.go` `initSchema()`: remove the v1 tables: `claims`, `anchors`, `relations`, `conditions`
- [ ] In `internal/core/index/rebuild.go`: remove the v1 `Rebuild` / `RebuildV1Index` function(s) and any claims imports. Keep `RebuildFromOpLog`.
- [ ] In `internal/core/index/populate.go`: check if it imports claims. If so, remove dead code. If it only depends on episodes, keep it.
- [ ] In `internal/core/index/index_test.go`: remove any tests that import claims or use v1 functions. Update as needed.

## 5. Verify No Dangling Imports
- [ ] Run `go build ./...` — must compile cleanly
- [ ] Run `grep -rn "core/claims\|core/events\|core/views\|pipeline/triage\|pipeline/extract\|pipeline/reconcile\|pipeline/verify" internal/ cmd/` — must return zero results (except possibly in comments/docs, which is fine)
- [ ] Run `go test ./...` — full test suite must pass

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./...` — full test suite must pass (some pre-existing failures are acceptable if documented)
3. Run `go vet ./...` — must pass
4. Verify no dangling imports to removed packages

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

<!-- Template — append this section when done -->
