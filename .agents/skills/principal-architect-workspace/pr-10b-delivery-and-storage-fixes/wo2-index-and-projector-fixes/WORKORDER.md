---
name: pr-10b-delivery-and-storage-fixes-wo2
description: Handoff instructions for Work Order 2 (Index FTS/Child Row Cleanup & Projector Cross-Scope Deduplication)
complexity: standard
---

# WORKORDER 2: Index FTS/Child Row Cleanup (`#13`) & Projector Cross-Scope Deduplication (`#11`)

> **Parent Project:** [pr-10b-delivery-and-storage-fixes](../PROJECT.md) (`PROGRAM-PLAN.md` v2.2 human approval recorded 2026-10-15)
> **Branch:** `pr-10b-delivery-and-storage-fixes/wo2-index-and-projector-fixes`
> **Base Branch:** `pr-10b-delivery-and-storage-fixes/wo1-persistent-usermessage-and-rewind`

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Fix GitHub Issue **#13** (prevent orphan FTS5 documents and duplicate child rows in `entry_anchors` and `error_patterns` when replacing or updating entries in SQLite) and GitHub Issue **#11** (properly move and deduplicate entries across dossier scopes in `internal/core/dossiers/projector.go` when an existing entry ID is re-added under a new scope or moved via `OpMove`).

## Strict Boundary Box
- **ONLY touch:**
  - `internal/core/index/index.go`
  - `internal/core/index/entries.go`
  - `internal/core/index/rebuild.go`
  - `internal/core/index/v2_test.go`
  - `internal/core/dossiers/projector.go`
  - `internal/core/dossiers/dossiers_test.go`
  - `.agents/skills/principal-architect-workspace/pr-10b-delivery-and-storage-fixes/wo2-index-and-projector-fixes/WORKORDER.md`
- **DO NOT touch:**
  - `internal/core/seed/` (reserved for WO3)
  - `.memory/` (reserved for WO4)
  - `internal/harness/antigravity/` (completed in WO1 / reserved for WO5)
- **External GitHub Hygiene:** Never include any internal references or absolute OS paths in code, comments, or commit messages. Use git author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.

## Tooling Constraints
- Run tests and static analysis via `run_command`: `go test -v ./internal/core/index/... ./internal/core/dossiers/...` and `go vet ./cmd/... ./internal/...`.

## Line-Item Instructions

- [ ] `internal/core/index/index.go` (GitHub Issue **#13**):
  - In `NewIndex(dbPath string)`, append `&_pragma=recursive_triggers(1)&_pragma=foreign_keys(1)` to the SQLite DSN (`file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=recursive_triggers(1)&_pragma=foreign_keys(1)`) so SQLite fires `entries_ad` and `ON DELETE CASCADE` even if `INSERT OR REPLACE` is ever executed.

- [ ] `internal/core/index/rebuild.go` (GitHub Issue **#13**):
  - Replace `insertEntryQuery` (`INSERT OR REPLACE INTO entries ...`) with `INSERT INTO entries (id, type, dossier_scope, text, source, verified, closes_when, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET type = excluded.type, dossier_scope = excluded.dossier_scope, text = excluded.text, source = excluded.source, verified = excluded.verified, closes_when = excluded.closes_when, created_at = CASE WHEN excluded.created_at != '' THEN excluded.created_at ELSE entries.created_at END, updated_at = excluded.updated_at` so `entries_au` fires cleanly.
  - Before inserting into `entry_anchors` and `error_patterns` for `entry.ID` inside the `RebuildFromOpLog` loop, execute `DELETE FROM entry_anchors WHERE entry_id = ?` and `DELETE FROM error_patterns WHERE entry_id = ?` so any duplicate entry IDs across dossier scopes can never duplicate anchor or error-pattern rows.

- [ ] `internal/core/index/entries.go` (GitHub Issue **#13**):
  - Add an `InsertEntry(ctx context.Context, entry dossiers.Entry) error` method (or update `UpsertEntry`) that deletes existing child rows in `entry_anchors` (when `len(entry.Anchors) > 0` in `UpsertEntry`, or unconditionally in `InsertEntry`) and `error_patterns` (and removes any existing FTS row via `ON CONFLICT(id) DO UPDATE` trigger `entries_au`), and if `entry.Type == dossiers.EntryTypeFailureSignature` in `InsertEntry`, populates `error_patterns`.
  - In `InsertErrorPattern(ctx, entryID, commandPrefix, pattern, scope)`, delete any existing identical row (`DELETE FROM error_patterns WHERE entry_id = ? AND COALESCE(command_prefix, '') = ? AND pattern = ?`) before inserting so repeated calls are idempotent.

- [ ] `internal/core/dossiers/projector.go` (GitHub Issue **#11**):
  - Extract a helper `moveEntry(dossiers map[string]*Dossier, entryIndex map[string]string, entryID, fromScope, toScope string, ts time.Time)` that:
    1. Returns immediately if `fromScope == "" || toScope == "" || fromScope == toScope`.
    2. Finds `entryID` in `fromD := getOrCreateDossier(dossiers, fromScope)`, removes it from `fromD.Entries`, updates its `DossierScope = toScope` and `UpdatedAt = ts`, removes any pre-existing duplicate with `ID == entryID` in `toD := getOrCreateDossier(dossiers, toScope)`, appends the moved entry to `toD.Entries`, and updates `entryIndex[entryID] = toScope`.
  - In `applyMove`: resolve `toScope, _ := op.Fields["to_dossier"].(string)` and `fromScope := entryIndex[op.EntryID]` (falling back to `op.DossierScope` if `fromScope == ""`), and call `moveEntry(dossiers, entryIndex, op.EntryID, fromScope, toScope, op.Timestamp)`.
  - In `applyAdd`: when `existingScope, exists := entryIndex[op.EntryID]; exists`:
    - If `existingScope != op.DossierScope && op.DossierScope != ""`, call `moveEntry(dossiers, entryIndex, op.EntryID, existingScope, op.DossierScope, op.Timestamp)` (instead of `applyMove`, which failed because an `add` op has no `to_dossier` field).
    - Then call `applyAmend(dossiers, entryIndex, op)` and return.
    - Also, before appending a brand-new `entry` to `d.Entries` at the bottom of `applyAdd`, ensure no entry with `e.ID == entry.ID` already exists in `d.Entries`.

- [ ] `internal/core/index/v2_test.go` and `internal/core/dossiers/dossiers_test.go`:
  - In `internal/core/index/v2_test.go`, add `TestUpsertAndReplaceEntry_NoOrphanFTSOrDuplicateChildren` verifying:
    1. Inserting an entry with unique FTS token `"orphancheckalpha"`, anchors, and error pattern, then replacing/updating it with `"orphancheckbeta"` leaves **zero** FTS matches for `"orphancheckalpha"` (no orphan rowids in `entries_fts`), 1 match for `"orphancheckbeta"`, and no duplicate rows in `entry_anchors` or `error_patterns`.
    2. Calling `InsertEntry` twice with updated text/anchors/error patterns replaces child rows cleanly without duplicates or orphan FTS rows.
  - In `internal/core/dossiers/dossiers_test.go`, add `TestProjector_ReAddUnderDifferentScopeMovesEntry` verifying GitHub Issue **#11**:
    1. Adding entry `E` under scope `core/mod-a` places `E` in `core/mod-a`.
    2. Re-adding entry `E` (`OpAdd`) under scope `core/mod-b` with updated text moves `E` out of `core/mod-a` (`len(dossiers["core/mod-a"].Entries) == 0`) and into `core/mod-b` (`len(dossiers["core/mod-b"].Entries) == 1`, `DossierScope == "core/mod-b"`, and updated text applied), with zero duplicate copies of `E` across any scope.

## Verification Requirement
Run `go test -v ./internal/core/index/... ./internal/core/dossiers/...` and `go vet ./cmd/... ./internal/...` to confirm all tests pass. Once finished, **STOP and send a message back to the Principal Architect** confirming the implementation is validated. Once approved, append your Completion Report to this file, commit all changes on branch `pr-10b-delivery-and-storage-fixes/wo2-index-and-projector-fixes`, and terminate your session.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->
