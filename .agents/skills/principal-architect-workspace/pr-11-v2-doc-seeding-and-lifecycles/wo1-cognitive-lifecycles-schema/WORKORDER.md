---
name: pr-11-v2-doc-seeding-and-lifecycles-wo1
description: Add CognitiveKind (semantic, procedural, episodic), v2.2 entry types/prefixes, doc-seed provenance fields, and procedural tracking fields to dossiers, ops, index, delivery ranker, and jobs
complexity: standard
---

# Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.
>
> **Authorization & Governance:** Read `.agents/rules/00-operating-model.md`. This `WORKORDER.md` belongs to parent project `pr-11-v2-doc-seeding-and-lifecycles`, whose 5-WO decomposition is explicitly approved by the human in `.agents/skills/principal-architect-workspace/pr-11-v2-doc-seeding-and-lifecycles/PROJECT.md`. You are authorized to implement and commit directly on the current branch (`pr-11-v2-doc-seeding-and-lifecycles/wo1-cognitive-lifecycles-schema`).
>
> **External GitHub Hygiene:** Never include any proprietary internal references (`cl/`, `b/`, `go/`, or local `/home/<user>/...` paths) in any code, comments, tests, or commit messages. Use git author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.

## Objective
Add the `v2.2` Three Cognitive Memory Lifecycles (`semantic`, `procedural`, `episodic`), canonical `EntryType` constants and prefixes (`contract`, `convention`, `external-behaviour`, `recipe`), `doc-seed` provenance fields (`SourceDocPath`, `SourceDocBlobSHA`), and `procedural` execution-tracking fields (`SuccessCount`, `FailureCount`, `LastSucceededAt`, `StaleSuspect`, `RenderAsSkill`) across `internal/core/dossiers`, `internal/core/ops`, `internal/core/index`, `internal/core/delivery`, and `internal/core/jobs`.

## Strict Boundary Box
- **Allowed to modify:**
  - `internal/core/dossiers/entry.go`
  - `internal/core/dossiers/projector.go`
  - `internal/core/dossiers/dossiers_test.go`
  - `internal/core/ops/op.go`
  - `internal/core/index/index.go`
  - `internal/core/index/entries.go`
  - `internal/core/index/rebuild.go`
  - `internal/core/index/v2_test.go`
  - `internal/core/delivery/ranker.go`
  - `internal/core/jobs/types.go`
  - `internal/core/jobs/state_test.go`
- **Do NOT touch:**
  - `internal/core/dossiers/renderer.go` or `internal/core/dossiers/handedit.go` (those belong to WO2)
  - `internal/core/seed/` (WO3)
  - `internal/core/tick/` (WO4)
  - `.memory/` (WO5)

## Tooling Constraints
- Run tests with `go test -race ./internal/core/dossiers/... ./internal/core/ops/... ./internal/core/index/... ./internal/core/delivery/... ./internal/core/jobs/...`.
- Commit with `git -c user.name="matthewmcneill" -c user.email="97467074+matthewmcneill@users.noreply.github.com" commit ...`.

## Line-Item Instructions

- [x] `internal/core/dossiers/entry.go` — Add `CognitiveKind`, new `EntryType` constants/prefixes, `DeriveCognitiveKind`, and `Entry` fields:
  1. Define `type CognitiveKind string` with constants:
     - `CognitiveKindSemantic CognitiveKind = "semantic"`
     - `CognitiveKindProcedural CognitiveKind = "procedural"`
     - `CognitiveKindEpisodic CognitiveKind = "episodic"`
  2. Add `Valid() bool` on `CognitiveKind` returning true for `CognitiveKindSemantic`, `CognitiveKindProcedural`, `CognitiveKindEpisodic`.
  3. Add the `v2.2` `EntryType` constants alongside the existing 8 constants:
     - `EntryTypeContract EntryType = "contract"`
     - `EntryTypeConvention EntryType = "convention"`
     - `EntryTypeExternalBehaviour EntryType = "external-behaviour"`
     - `EntryTypeRecipe EntryType = "recipe"`
  4. Update `EntryTypePrefixes` so all canonical types and aliases have prefixes:
     - `EntryTypeInvariant: "inv-"`
     - `EntryTypeContract: "con-"`
     - `EntryTypeConvention: "con-"`
     - `EntryTypeRuledOut: "rul-"`
     - `EntryTypeFailureSignature: "fail-"`
     - `EntryTypeGotcha: "got-"`
     - `EntryTypeExternalBehaviour: "got-"`
     - `EntryTypeDecision: "dec-"`
     - `EntryTypeProcedure: "proc-"`
     - `EntryTypeRecipe: "rec-"`
     - `EntryTypeInterface: "iface-"`
     - `EntryTypeOpenIssue: "issue-"`
  5. Add `DeriveCognitiveKind(t EntryType) CognitiveKind`:
     - Returns `CognitiveKindSemantic` for `EntryTypeInvariant`, `EntryTypeContract`, `EntryTypeConvention`, `EntryTypeInterface`, `EntryTypeGotcha`, `EntryTypeExternalBehaviour`, `EntryTypeOpenIssue` (and as default fallback if `t` is empty/unrecognized).
     - Returns `CognitiveKindProcedural` for `EntryTypeRecipe`, `EntryTypeProcedure`, `EntryTypeFailureSignature`.
     - Returns `CognitiveKindEpisodic` for `EntryTypeDecision`, `EntryTypeRuledOut`.
  6. Add fields to `Entry`:
     - `Kind CognitiveKind` (`json:"kind,omitempty"`)
     - `SourceDocPath string` (`json:"source_doc_path,omitempty"`)
     - `SourceDocBlobSHA string` (`json:"source_doc_blob_sha,omitempty"`)
     - `SuccessCount int` (`json:"success_count,omitempty"`)
     - `FailureCount int` (`json:"failure_count,omitempty"`)
     - `LastSucceededAt string` (`json:"last_succeeded_at,omitempty"`)
     - `StaleSuspect bool` (`json:"stale_suspect,omitempty"`)
     - `RenderAsSkill string` (`json:"render_as_skill,omitempty"`)
  7. Add method `(e Entry) EffectiveKind() CognitiveKind`: if `e.Kind.Valid()` return `e.Kind`; otherwise return `DeriveCognitiveKind(e.Type)`.

- [x] `internal/core/ops/op.go` — Add `SourceDocSeed = "doc-seed"` to the provenance source constants block.

- [x] `internal/core/dossiers/projector.go` — Populate and preserve the new `Entry` fields across `applyAdd`, `applyAmend`, `applySupersede`, and `typeOrder`:
  1. **Careful disambiguation of `op.Fields["kind"]`**: Existing ops (such as `TestDossiersRoundTrip`) pass `"kind": "module"` (or `"topic"`, `"external"`, `"procedure"`) in `op.Fields` to set the *dossier's* `d.Kind`, whereas entry cognitive kind is `"semantic"`, `"procedural"`, or `"episodic"`.
     - If `op.Fields["kind"]` is `"semantic"`, `"procedural"`, or `"episodic"`, set `entry.Kind = CognitiveKind(k)`. Otherwise, if it is `"module"`, `"topic"`, `"external"`, or `"procedure"`, set `d.Kind = DossierKind(k)`.
     - Also check `op.Fields["cognitive_kind"].(string)` (if valid `CognitiveKind`, set `entry.Kind`) and `op.Fields["dossier_kind"].(string)` (if non-empty, set `d.Kind`).
     - In `applyAdd`, if `entry.Kind == ""` after reading fields, set `entry.Kind = DeriveCognitiveKind(entry.Type)`.
     - In `applyAmend`, if `entry_type` is amended and `kind`/`cognitive_kind` was not explicitly set in `op.Fields`, update `d.Entries[i].Kind = DeriveCognitiveKind(d.Entries[i].Type)`.
  2. Read and apply the new fields in `applyAdd`, `applyAmend`, and `applySupersede` (inheriting from `entryToMove` in `applySupersede` when not overridden, except `SuccessCount`/`FailureCount`/`StaleSuspect` which reset unless explicitly provided in `op.Fields`, while `SourceDocPath`, `SourceDocBlobSHA`, `RenderAsSkill` inherit if not overridden):
     - `source_doc_path` (`string`) -> `SourceDocPath`
     - `source_doc_blob_sha` (`string`) -> `SourceDocBlobSHA`
     - `success_count` (`int`, `int64`, or `float64` from JSON) -> `SuccessCount`
     - `failure_count` (`int`, `int64`, or `float64` from JSON) -> `FailureCount`
     - `last_succeeded_at` (`string`) -> `LastSucceededAt`
     - `stale_suspect` (`bool`) -> `StaleSuspect`
     - `render_as_skill` (`string`) -> `RenderAsSkill`
  3. Update `typeOrder` in `projector.go` to include the new `EntryType` constants:
     - `EntryTypeInvariant: 1`, `EntryTypeContract: 1`, `EntryTypeConvention: 1`
     - `EntryTypeRuledOut: 2`
     - `EntryTypeFailureSignature: 3`
     - `EntryTypeGotcha: 4`, `EntryTypeExternalBehaviour: 4`
     - `EntryTypeDecision: 5`
     - `EntryTypeProcedure: 6`, `EntryTypeRecipe: 6`
     - `EntryTypeInterface: 7`
     - `EntryTypeOpenIssue: 8`

- [x] `internal/core/delivery/ranker.go` — Update `typeOrderRanking` to include the new `EntryType` constants:
  - `dossiers.EntryTypeContract: 80`
  - `dossiers.EntryTypeConvention: 80`
  - `dossiers.EntryTypeExternalBehaviour: 40`
  - `dossiers.EntryTypeRecipe: 20`

- [x] `internal/core/index/index.go`, `internal/core/index/entries.go`, and `internal/core/index/rebuild.go` — Add the new columns to `entries` with idempotent migration and wire them into all entry queries:
  1. In `index.go` `initSchema()`:
     - Include `kind TEXT DEFAULT ''`, `source_doc_path TEXT DEFAULT ''`, `source_doc_blob_sha TEXT DEFAULT ''`, `success_count INTEGER DEFAULT 0`, `failure_count INTEGER DEFAULT 0`, `last_succeeded_at TEXT DEFAULT ''`, `stale_suspect INTEGER DEFAULT 0`, `render_as_skill TEXT DEFAULT ''` in `CREATE TABLE IF NOT EXISTS entries (...)`.
     - Also run an idempotent migration step in `initSchema()` (e.g. querying `PRAGMA table_info(entries)` to check existing columns, and executing `ALTER TABLE entries ADD COLUMN <col> <type> DEFAULT <default>` for any missing column) so pre-existing `index.db` files on disk upgrade seamlessly when opened by `NewIndex`!
  2. In `entries.go`:
     - Update `UpsertEntry` and `InsertEntry` to write `kind` (`string(entry.EffectiveKind())`), `source_doc_path`, `source_doc_blob_sha`, `success_count`, `failure_count`, `last_succeeded_at`, `stale_suspect` (`0` or `1`), and `render_as_skill`.
     - Update `GetEntry`, `ListEntriesByScope`, and `SearchEntriesWithScore` to `SELECT` and scan those 8 columns into `dossiers.Entry` (and if scanned `kind` is empty, set `e.Kind = dossiers.DeriveCognitiveKind(e.Type)`).
     - Add `ListEntriesBySource(ctx context.Context, source string) ([]dossiers.Entry, error)` that queries `entries WHERE source = ? ORDER BY id`, populates `Anchors` for each returned entry, and returns `[]dossiers.Entry{}` when empty.
  3. In `rebuild.go`:
     - Update `insertEntryQuery` and `tx.ExecContext` in `RebuildFromOpLog` to insert and `ON CONFLICT(id) DO UPDATE` all 8 new columns (`kind`, `source_doc_path`, `source_doc_blob_sha`, `success_count`, `failure_count`, `last_succeeded_at`, `stale_suspect`, `render_as_skill`).

- [x] `internal/core/jobs/types.go` — Add `KindSeedDocs` and `KindDreamFreshness`:
  - Add `KindSeedDocs Kind = "seed-docs"` and `KindDreamFreshness Kind = "dream-freshness"` to the `Kind` constants and include both in `Kind.Valid()`.

- [x] Unit Tests (`internal/core/dossiers/dossiers_test.go`, `internal/core/index/v2_test.go`, `internal/core/jobs/state_test.go`):
  - In `dossiers_test.go`: add tests for `DeriveCognitiveKind`, `EffectiveKind`, and projecting `add`/`amend`/`supersede` ops with `contract`, `recipe`, `source_doc_path`, `source_doc_blob_sha`, `success_count`, `failure_count`, `last_succeeded_at`, `stale_suspect`, and `render_as_skill`.
  - In `v2_test.go`: add tests verifying (a) `UpsertEntry` / `InsertEntry` / `GetEntry` / `ListEntriesByScope` / `ListEntriesBySource` / `SearchEntriesWithScore` / `RebuildFromOpLog` preserve all new `v2.2` fields, and (b) opening an older `entries` table without the new columns via `NewIndex` auto-migrates the table via `ALTER TABLE` without error.
  - In `state_test.go`: verify `jobs.KindSeedDocs.Valid()` and `jobs.KindDreamFreshness.Valid()` return `true`.

## Verification Requirement
Run:
```bash
go test -race ./internal/core/dossiers/... ./internal/core/ops/... ./internal/core/index/... ./internal/core/delivery/... ./internal/core/jobs/...
```
All tests must pass with zero race warnings. Commit your changes to `pr-11-v2-doc-seeding-and-lifecycles/wo1-cognitive-lifecycles-schema`, then **STOP and send a message back to the Principal Architect** confirming validation. Once approved, append your Completion Report below and commit it.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---
## Completion Report
- **Status:** Complete
- **Implementation Commit:** `c9640fc`
- **Summary:**
  - Added `CognitiveKind` (`semantic`, `procedural`, `episodic`), `DeriveCognitiveKind`, `EffectiveKind`, `v2.2` `EntryType` constants (`contract`, `convention`, `external-behaviour`, `recipe`), and `v2.2` `Entry` provenance/procedural tracking fields (`Kind`, `SourceDocPath`, `SourceDocBlobSHA`, `SuccessCount`, `FailureCount`, `LastSucceededAt`, `StaleSuspect`, `RenderAsSkill`) in `internal/core/dossiers/entry.go`.
  - Added `SourceDocSeed = "doc-seed"` in `internal/core/ops/op.go`.
  - Updated `internal/core/dossiers/projector.go` to disambiguate `kind`/`cognitive_kind`/`dossier_kind`, populate/inherit/reset all new `v2.2` fields across `applyAdd`/`applyAmend`/`applySupersede`, and rank the new `EntryType` constants in `typeOrder`.
  - Updated `typeOrderRanking` in `internal/core/delivery/ranker.go`.
  - Added the 8 new `entries` table columns, idempotent `migrateEntriesColumns()` (`PRAGMA table_info(entries)` + `ALTER TABLE`), `ListEntriesBySource`, and full read/write/rebuild wiring across `internal/core/index/{index,entries,rebuild}.go`.
  - Added `KindSeedDocs` and `KindDreamFreshness` in `internal/core/jobs/types.go`.
  - Added unit tests in `dossiers_test.go`, `v2_test.go`, and `state_test.go` and verified `go test -race ./internal/core/dossiers/... ./internal/core/ops/... ./internal/core/index/... ./internal/core/delivery/... ./internal/core/jobs/...` passes with 0 race warnings.
- **Out-of-Boundary Issues Discovered:** None.

