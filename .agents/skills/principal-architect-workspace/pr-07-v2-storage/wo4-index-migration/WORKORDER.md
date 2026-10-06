---
name: pr-07-v2-storage-wo4
description: Handoff instructions for Work Order 4 — Index Migration
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Migrate the SQLite index from the v1 claims/episodes schema to the v2 entries/anchors/aliases schema. The new index is rebuilt from the op log. Keep the existing `episodes_fts` table (it's still used). Add new tables for entries, anchors, error patterns, aliases, and history.

# Context

## v2 Design Reference (§7.5 Index)
The SQLite index holds:
- A map from anchors (paths, symbols, config keys) to entry IDs, with fingerprints for verification
- Full-text search (FTS5) over entries, session synopses and checkpoint summaries
- Error patterns for `failure-signature` entries, each with a command prefix and dossier scope
- Dossier aliases, and history entries by ID
- Rebuilt from the op log, using the pointer-file swap from SPEC §18.4

## Existing Index Schema (`internal/core/index/index.go`)
Current tables:
- `episodes_fts` (FTS5) — episode search. **KEEP this table.**
- `entities` — entity records. **KEEP for now** (entity extractor still uses it).
- `entity_relations` — entity relationships. **KEEP for now.**
- `claims` — v1 claim records. **REPLACE with `entries`.**
- `anchors` — v1 claim anchors (claim_id FK). **REPLACE with new `anchors` (entry_id FK).**
- `relations` — v1 claim relations. **REMOVE** (superseded by op log).
- `conditions` — v1 claim conditions. **REMOVE** (superseded by `closes_when` in entries).

## Existing Files
- `internal/core/index/index.go` (~243 lines) — schema init, CRUD, search
- `internal/core/index/index_test.go` (~91 lines) — basic tests
- `internal/core/index/populate.go` (~75 lines) — populate from episodes
- `internal/core/index/rebuild.go` (~154 lines) — rebuild from claims
- `internal/core/index/entities.go` — entity CRUD
- `internal/core/index/doc.go` — package doc

## Dependencies (WO1-WO3, already built)
- `internal/core/ops/` — op log (Reader, Iterate)
- `internal/core/dossiers/` — Entry, EntryType, Dossier, Projector
- `internal/core/config/` — ScopeResolver, AliasMap

# Strict Boundary Box

**DO NOT touch:**
- `internal/core/ops/` — op log (WO1)
- `internal/core/dossiers/` — dossiers (WO2) — but you WILL import from it
- `internal/core/config/scopes.go` — scopes (WO3) — but you WILL import from it
- `internal/core/pipeline/` — v1 pipeline (WO5)
- `internal/core/claims/` — v1 claims (WO5)
- `internal/core/delivery/` — delivery (PR-08)
- `cmd/repo-memory/` — CLI
- `internal/harness/`
- `.agents/`

**DO modify:**
- `internal/core/index/` — migrate the schema and add new functions

# Tooling Constraints

- **Build:** `go build ./...` must pass
- **Test:** `go test ./internal/core/index/...` must pass
- **Vet:** `go vet ./internal/core/index/...` must pass
- Use `modernc.org/sqlite` (already in the project)

# Line-Item Instructions

## 1. New Schema Tables
- [ ] Modify `internal/core/index/index.go` `initSchema()`:
  - **Keep:** `episodes_fts`, `entities`, `entity_relations`
  - **Add** new tables (alongside the existing ones):
    ```sql
    CREATE TABLE IF NOT EXISTS entries (
      id TEXT PRIMARY KEY,
      type TEXT NOT NULL,
      dossier_scope TEXT NOT NULL,
      text TEXT NOT NULL,
      source TEXT,
      verified TEXT,
      closes_when TEXT,
      created_at TEXT,
      updated_at TEXT
    );

    CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
      id,
      dossier_scope,
      text,
      content=entries,
      content_rowid=rowid
    );

    CREATE TABLE IF NOT EXISTS entry_anchors (
      entry_id TEXT NOT NULL,
      anchor TEXT NOT NULL,
      kind TEXT,
      fingerprint TEXT,
      FOREIGN KEY(entry_id) REFERENCES entries(id) ON DELETE CASCADE
    );
    CREATE INDEX IF NOT EXISTS idx_entry_anchors_anchor ON entry_anchors(anchor);

    CREATE TABLE IF NOT EXISTS error_patterns (
      entry_id TEXT NOT NULL,
      command_prefix TEXT,
      pattern TEXT NOT NULL,
      dossier_scope TEXT NOT NULL,
      FOREIGN KEY(entry_id) REFERENCES entries(id) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS aliases (
      old_scope TEXT NOT NULL,
      new_scope TEXT NOT NULL,
      created_at TEXT
    );

    CREATE TABLE IF NOT EXISTS entry_history (
      id TEXT NOT NULL,
      entry_id TEXT NOT NULL,
      type TEXT NOT NULL,
      dossier_scope TEXT NOT NULL,
      text TEXT,
      reason TEXT,
      superseded_by TEXT,
      retired_at TEXT
    );
    ```
  - **Do NOT drop** the old `claims`, `anchors`, `relations`, `conditions` tables yet — WO5 handles that. Just add the new tables alongside.

## 2. Entry CRUD
- [ ] Create `internal/core/index/entries.go`:
  - `func (idx *Index) UpsertEntry(ctx context.Context, entry dossiers.Entry) error`
  - `func (idx *Index) GetEntry(ctx context.Context, id string) (*dossiers.Entry, error)`
  - `func (idx *Index) DeleteEntry(ctx context.Context, id string) error`
  - `func (idx *Index) ListEntriesByScope(ctx context.Context, scope string) ([]dossiers.Entry, error)`
  - `func (idx *Index) SearchEntries(ctx context.Context, query string) ([]dossiers.Entry, error)` — FTS5 search over `entries_fts`

## 3. Anchor Lookups
- [ ] Create `internal/core/index/entry_anchors.go`:
  - `func (idx *Index) UpsertEntryAnchors(ctx context.Context, entryID string, anchors []string) error` — replace all anchors for an entry
  - `func (idx *Index) LookupByAnchor(ctx context.Context, anchor string) ([]string, error)` — returns entry IDs matching the anchor (prefix match using LIKE)
  - `func (idx *Index) LookupByAnchorExact(ctx context.Context, anchor string) ([]string, error)` — exact match

## 4. Error Patterns
- [ ] Add to `internal/core/index/entries.go` (or a new file):
  - `func (idx *Index) InsertErrorPattern(ctx context.Context, entryID, commandPrefix, pattern, scope string) error`
  - `func (idx *Index) LookupErrorPatterns(ctx context.Context, commandPrefix string) ([]ErrorPatternResult, error)` — returns matching failure-signature entries for a command

## 5. Aliases
- [ ] Add to `internal/core/index/entries.go` (or a new file):
  - `func (idx *Index) UpsertAlias(ctx context.Context, oldScope, newScope string) error`
  - `func (idx *Index) ResolveAlias(ctx context.Context, scope string) (string, error)` — follow alias chain

## 6. History
- [ ] Add:
  - `func (idx *Index) InsertHistory(ctx context.Context, entry dossiers.Entry, reason string) error`
  - `func (idx *Index) GetHistory(ctx context.Context, entryID string) ([]dossiers.Entry, error)`

## 7. Rebuild from Op Log
- [ ] Modify `internal/core/index/rebuild.go`:
  - Add `func (idx *Index) RebuildFromOpLog(ctx context.Context, opsReader *ops.Reader) error`:
    - Clear all v2 tables (entries, entry_anchors, error_patterns, aliases, entry_history, entries_fts)
    - Project dossiers from op log using `dossiers.NewProjector(opsReader).Project(ctx)`
    - For each dossier and its entries: UpsertEntry, UpsertEntryAnchors
    - For failure-signature entries: InsertErrorPattern
    - For retired/superseded entries in History: InsertHistory
    - Build aliases from `config.BuildAliasMap`
  - Keep the existing `RebuildIndex` function (renamed to `RebuildV1Index` if needed to avoid confusion) — it will be removed in WO5

## 8. Tests
- [ ] Update `internal/core/index/index_test.go` (or create new test files):
  - Test UpsertEntry + GetEntry round-trip
  - Test SearchEntries with FTS5 query
  - Test UpsertEntryAnchors + LookupByAnchor
  - Test LookupByAnchorExact
  - Test InsertErrorPattern + LookupErrorPatterns
  - Test UpsertAlias + ResolveAlias chain
  - Test InsertHistory + GetHistory
  - Test RebuildFromOpLog end-to-end (write ops → rebuild → verify entries/anchors populated)

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./internal/core/index/...` — all tests must pass
3. Run `go vet ./internal/core/index/...` — must pass
4. Verify backward compatibility: existing episode and entity functions still work

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

<!-- Template — append this section when done -->
