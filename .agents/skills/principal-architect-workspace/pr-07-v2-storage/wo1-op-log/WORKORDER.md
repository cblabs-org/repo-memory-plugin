---
name: pr-07-v2-storage-wo1
description: Handoff instructions for Work Order 1 — Op Log
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Build the append-only op log that is v2's source of truth for all knowledge entries. The op log replaces the v1 events store. Each op targets a specific dossier entry by ID and records how it was created, changed, or retired.

# Context

## v2 Design References
- **v2 Design §7.1 (Layout):** Ops live at `.memory/ops/<yyyy-mm>/<ulid>.json`, one JSON file per batch. The op log is the source of truth; dossiers are rendered from it.
- **v2 Design §7.4 (Why pages, rendered from an op log):** Ops target entry IDs, not lines. Each batch is a new file, so ops never conflict in git. Hand edits to dossiers are converted to ops. Knowledge is a projection of events.

## Op Types (from v2 Design)
The six op types, each targeting an entry by ID:

| Op | Purpose |
|---|---|
| `add` | Create a new entry in a dossier |
| `amend` | Update an existing entry (keep the same ID) |
| `supersede` | Replace one entry with another (old → retired, new created) |
| `retire` | Remove an entry from the active dossier (goes to `.history.md`) |
| `move` | Change which dossier an entry belongs to |
| `split` | Split a dossier that has grown past the cap into two dossiers |

## Entry Schema (from v2 Design §7.3)
Each entry has:
- **ID:** type prefix + short ULID (e.g. `inv-01J5K3...`, `rul-7Q2M`)
- **Type:** one of 8: `invariant`, `ruled-out`, `failure-signature`, `gotcha`, `decision`, `procedure`, `interface`, `open-issue`
- **Required fields per type:** text (2-6 lines), anchors (paths/symbols), evidence links, verified stamp, optional `closes_when`
- **Dossier scope:** which dossier this entry belongs to

## Existing v1 Events Store
- Package: `internal/core/events/` — contains `models.go` (ReviewEvent, AnswerEvent with SHA256 hashes), `store.go`, `store_test.go`, `doc.go`
- Data dir: `.memory/events/` — may contain existing event files
- **Action:** Archive `.memory/events/` to `.memory/archive/v1-events/` (if any events exist). Remove the Go package.

## Existing Packages to Understand (DO NOT MODIFY)
- `internal/core/evidence/` — evidence records
- `internal/core/episode/` — episode store
- `internal/core/config/` — configuration
- `internal/core/jobs/` — job table

# Strict Boundary Box

**DO NOT touch:**
- `internal/core/pipeline/` — v1 pipeline stages (removed in WO5)
- `internal/core/claims/` — v1 claims store (removed in WO5)
- `internal/core/views/` — v1 views (removed in WO5)
- `internal/core/index/` — SQLite index (migrated in WO4)
- `internal/core/dossiers/` — dossiers package (built in WO2)
- `internal/core/delivery/` — already has spike code, leave it
- `cmd/repo-memory/` — CLI subcommands (later WOs)
- `internal/harness/` — harness layer
- `.agents/` — PA workspace files
- Any hook handler code

**DO create:**
- `internal/core/ops/` — new package (the deliverable)

# Tooling Constraints

- **Build verification:** `go build ./...` must pass
- **Test execution:** `go test ./internal/core/ops/...` must pass
- **Lint check:** `go vet ./internal/core/ops/...` must pass
- Use the `modernc.org/sqlite` driver if SQLite is needed (consistent with existing code)
- Use ULIDs from a standard Go library (e.g. `github.com/oklog/ulid/v2`)

# Line-Item Instructions

## 1. Op Types and Models
- [x] Create `internal/core/ops/op.go` with the op type definitions:
  - Define an `OpType` string enum: `add`, `amend`, `supersede`, `retire`, `move`, `split`
  - Define an `Op` struct containing:
    - `Type OpType` — the operation type
    - `EntryID string` — target entry ID (type prefix + ULID, e.g. `inv-01J5K3M2`)
    - `DossierScope string` — which dossier this entry belongs to (e.g. `core/transcript-parser`)
    - `Timestamp time.Time` — when the op was recorded
    - `Source string` — provenance: `turn-note`, `sidecar`, `seed`, `pa`, `human`, `consolidation`
    - `SessionID string` — conversation ID that produced this op (empty for seed/human)
    - `Fields map[string]any` — type-specific payload (the entry content, anchors, evidence, etc.)
  - Define an `OpBatch` struct containing:
    - `Version int` — schema version (start at 1)
    - `Ops []Op` — the operations in this batch
    - `CreatedAt time.Time` — batch creation timestamp
    - `BatchID string` — ULID for this batch file
  - Define validation rules per op type:
    - `add`: EntryID, DossierScope, and Fields (with at least `text`) are required
    - `amend`: EntryID and Fields are required; DossierScope optional (unchanged if empty)
    - `supersede`: EntryID (old) and Fields must include `new_entry_id` and `text`
    - `retire`: EntryID required; Fields must include `reason`
    - `move`: EntryID and Fields must include `to_dossier`
    - `split`: DossierScope required; Fields must include `new_dossiers` (list of new scope names) and `entry_moves` (map of entry ID → new scope)

## 2. Op File I/O
- [x] Create `internal/core/ops/writer.go`:
  - `type Writer struct` with a base directory (the `.memory/ops/` path)
  - `func (w *Writer) Write(batch OpBatch) error` — writes the batch as JSON to `.memory/ops/<yyyy-mm>/<batch-id>.json`, creating the month directory if needed. Use `os.MkdirAll`. Use indented JSON for git readability.
  - `func NewWriter(baseDir string) *Writer`

- [x] Create `internal/core/ops/reader.go`:
  - `type Reader struct` with a base directory
  - `func (r *Reader) ReadBatch(path string) (*OpBatch, error)` — reads and validates a single batch file
  - `func NewReader(baseDir string) *Reader`

## 3. Op Log Iterator
- [x] Create `internal/core/ops/iterator.go`:
  - `type Iterator struct` — iterates over all op batches in chronological order
  - `func (r *Reader) Iterate(ctx context.Context) iter.Seq2[*OpBatch, error]` — yields batches in ULID order (which is chronological). Walk month directories in order, then files within each month in ULID order.
  - `func (r *Reader) IterateAfter(ctx context.Context, afterBatchID string) iter.Seq2[*OpBatch, error]` — yields only batches after the given batch ID (for incremental processing)
  - `func (r *Reader) IterateBefore(ctx context.Context, beforeCommit string) iter.Seq2[*OpBatch, error]` — yields only batches whose `CreatedAt` is before the given commit's timestamp. This is the time-cut mechanism needed for X6. The `beforeCommit` is a git commit hash; resolve its timestamp via `git log -1 --format=%cI <commit>`.

## 4. Validation
- [x] Create `internal/core/ops/validate.go`:
  - `func ValidateOp(op Op) error` — validates a single op against the rules defined above
  - `func ValidateBatch(batch OpBatch) error` — validates all ops in a batch
  - Return clear error messages indicating which field is missing/invalid

## 5. v1 Events Store Retirement
- [x] Archive existing v1 events data:
  - In the **code**, add a migration function `internal/core/ops/migrate.go`:
    - `func MigrateV1Events(eventsDir, archiveDir string) error` — moves `.memory/events/` to `.memory/archive/v1-events/` if it exists. This is a simple directory rename.
  - Do NOT remove `internal/core/events/` Go package yet — that will be done in WO5 alongside the other v1 removals. Just add a `// Deprecated: use internal/core/ops instead. Will be removed in WO5.` comment to the package doc.

## 6. Tests
- [x] Create `internal/core/ops/ops_test.go` (or split across files):
  - Test `ValidateOp` for each op type: valid ops pass, missing required fields fail with clear errors
  - Test `Writer.Write` + `Reader.ReadBatch` round-trip: write a batch, read it back, verify identical
  - Test `Reader.Iterate`: write 3 batches across 2 months, iterate, verify chronological order
  - Test `Reader.IterateAfter`: write 3 batches, iterate after the 1st, verify only 2nd and 3rd returned
  - Test `MigrateV1Events`: create a mock events dir, migrate, verify it moved
  - Use `t.TempDir()` for all test file I/O

# Verification Requirement

1. Run `go build ./...` — must pass with no errors
2. Run `go test ./internal/core/ops/...` — all tests must pass
3. Run `go vet ./internal/core/ops/...` — must pass
4. Verify no imports of the new package from outside `internal/core/ops/` (it's self-contained for now)
5. Commit all changes on the current branch with a descriptive message

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report (written by the worker at task completion)

### Status
Completed

### Changes Made
- Created `internal/core/ops/op.go`: defines `OpType` (`add`, `amend`, `supersede`, `retire`, `move`, `split`), provenance constants, `Op`, `OpBatch`, and ULID helper constructors (`NewBatchID`, `NewBatchIDWithTime`, `NewEntryID`, `NewEntryIDWithTime`, `NewBatch`).
- Created `internal/core/ops/validate.go`: implements `ValidateOp` with validation rules per op type and `ValidateBatch`. Supports both Go typed structs and unmarshaled JSON maps/slices.
- Created `internal/core/ops/writer.go`: implements `Writer.Write(batch)` targeting `.memory/ops/<yyyy-mm>/<batch-id>.json`.
- Created `internal/core/ops/reader.go`: implements `Reader.ReadBatch(path)` with path resolution and batch validation.
- Created `internal/core/ops/iterator.go`: implements `Iterator` (supporting push `Seq()` and pull `Next()`), `Reader.Iterate`, `Reader.IterateAfter`, and `Reader.IterateBefore` with git commit timestamp resolution (`git log -1 --format=%cI`).
- Created `internal/core/ops/migrate.go`: implements `MigrateV1Events` directory migration from `.memory/events/` to `.memory/archive/v1-events/`.
- Created `internal/core/ops/doc.go`: package documentation for `ops`.
- Updated `internal/core/events/doc.go`: added deprecation notice for v1 events store.
- Created `internal/core/ops/ops_test.go`: comprehensive unit test suite covering validation rules, writer/reader round-trip, chronological iteration, `IterateAfter`, `IterateBefore` via git commit, context cancellation, v1 migration, and edge cases.

### Decisions Made
- Used standard `github.com/oklog/ulid/v2` matching existing codebase patterns for ULID generation.
- Designed `Iterator` to support both Go 1.23+ range sequence iteration (`iter.Seq2[*OpBatch, error]`) and pull iteration (`Next() (*OpBatch, error)` via `iter.Pull2`).
- Made `split` validation accept both native Go slices/maps and JSON unmarshaled `[]any`/`map[string]any` to ensure round-trip validation seamlessly passes.
- Supported `WithGitDir` on `Reader` to enable testing and flexible repository location resolution for `IterateBefore`.

### Verification Results
- `go build ./...`: passed cleanly.
- `go test -v ./internal/core/ops/...`: passed (10 test suites, all green).
- `go vet ./internal/core/ops/...`: passed with 0 warnings.
- External import verification: package is completely self-contained.

### Dead Ends & Warnings
None.

### Out-of-Scope Findings
None.

### Blockers
None.

