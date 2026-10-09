---
name: pr-07-v2-storage-wo2
description: Handoff instructions for Work Order 2 — Dossier Schema & Renderer
complexity: architect
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Build the dossier schema, renderer, atlas, and history subsystem. Dossiers are Markdown pages rendered from the op log (built in WO1). Each dossier is a module/topic/external/procedure page that agents read as working context. The renderer must produce identical output when run twice against the same op log.

# Context

## v2 Design References
- **§7.2 (Dossier kinds):** module (path globs + symbols), architectural topic, external system, procedure area
- **§7.3 (Dossier format):** Required sections: "Purpose and boundaries", "Invariants", "Ruled out, failure signatures and gotchas". Optional sections (omitted when empty): Decisions, Procedures, Interfaces and dependencies, Open issues, Evidence index.
- **§7.4 (Why pages, rendered from an op log):** Pages read as working context. The op log is the source of truth. Hand edits to dossiers are detected and converted to ops.
- **§7.5 (Index):** Anchor → entry map, FTS5 (WO4 scope, not this WO).

## Entry Schema (from v2 Design)
Each entry has:
- **ID:** type prefix + short ULID (e.g. `inv-01J5K3M2`, `rul-7Q2M`)
- **Type:** one of 8: `invariant`, `ruled-out`, `failure-signature`, `gotcha`, `decision`, `procedure`, `interface`, `open-issue`
- **Content:** 2–6 lines of text
- **Anchors:** paths, symbols, config keys this entry is about
- **Evidence:** links to sessions/notes that support this entry
- **Verified stamp:** optional, date + experiment/session reference
- **`closes_when`:** optional condition for auto-retirement (PR-13 implements evaluation, but the schema needs the field now)

## Example Dossier Format (from v2 Design §7.3)
```markdown
# core/transcript-parser

## Purpose and boundaries
Parses Antigravity `transcript_full.jsonl` into structured steps...

## Invariants

### [inv-01J5K3] Transcript step ordering must be preserved
`invariant · anchors: internal/harness/antigravity/transcript.go · verified 2026-10-02 (PR-02a) · evidence: note-01M4…`
Steps are appended in order; the parser must yield them in index order...

## Ruled out, failure signatures and gotchas

### [rul-7Q2M] Relative hook command paths in hooks.json
`ruled-out · anchors: packaging/antigravity/hooks.json, cmd/repo-memory/cmd_install.go · verified 2026-10-01 (E09) · evidence: note-01M4…, E09`
Why: Antigravity runs hook commands from the directory containing hooks.json, not the repo root.
Do: generate absolute paths at install time.
```

## Op Log (WO1 — already built)
- Package: `internal/core/ops/`
- Types: `Op`, `OpBatch`, `OpType` (add, amend, supersede, retire, move, split)
- Reader: `ops.NewReader(baseDir)`, `reader.Iterate(ctx)` yields `*OpBatch` in ULID order
- Writer: `ops.NewWriter(baseDir)`, `writer.Write(batch)` writes to `.memory/ops/<yyyy-mm>/<ulid>.json`
- Validation: `ops.ValidateOp(op)`, `ops.ValidateBatch(batch)`
- Entry IDs: `ops.NewEntryID("inv")` → `inv-01J5K3M2…`

## Storage Layout
```
.memory/
  ops/<yyyy-mm>/<ulid>.json    (op log — source of truth, WO1)
  dossiers/                     (rendered from op log — THIS WO)
    core/transcript-parser.md
    core/transcript-parser.history.md
    harness/antigravity-hooks.md
    external/antigravity-cli.md
    procedures/release.md
  ATLAS.md                      (one line per dossier, rendered — THIS WO)
  config.yaml                   (scopes — WO3, already exists)
```

# Strict Boundary Box

**DO NOT touch:**
- `internal/core/ops/` — op log (already built in WO1)
- `internal/core/pipeline/` — v1 pipeline (removed in WO5)
- `internal/core/claims/` — v1 claims (removed in WO5)
- `internal/core/index/` — index (migrated in WO4)
- `internal/core/delivery/` — delivery (PR-08)
- `cmd/repo-memory/` — CLI (later WOs add subcommands)
- `internal/harness/` — harness code
- `.agents/` — PA workspace

**DO create:**
- `internal/core/dossiers/` — new package (the deliverable)

# Tooling Constraints

- **Build verification:** `go build ./...` must pass
- **Test execution:** `go test ./internal/core/dossiers/...` must pass
- **Lint check:** `go vet ./internal/core/dossiers/...` must pass

# Line-Item Instructions

## 1. Entry Types and Schema
- [ ] Create `internal/core/dossiers/entry.go`:
  - Define `EntryType` string enum: `invariant`, `ruled-out`, `failure-signature`, `gotcha`, `decision`, `procedure`, `interface`, `open-issue`
  - Define a type-prefix map: `inv-`, `rul-`, `fail-`, `got-`, `dec-`, `proc-`, `iface-`, `issue-`
  - Define `Entry` struct:
    - `ID string` — the entry ID (e.g. `inv-01J5K3M2`)
    - `Type EntryType` — one of the 8 types
    - `DossierScope string` — which dossier this belongs to
    - `Text string` — the 2-6 line content
    - `Anchors []string` — paths, symbols, config keys
    - `Evidence []string` — links to sessions/notes
    - `Verified string` — optional stamp: "2026-10-02 (PR-02a)"
    - `ClosesWhen string` — optional auto-retirement condition
    - `Source string` — provenance: seed, turn-note, sidecar, pa, human, consolidation
    - `CreatedAt time.Time`
    - `UpdatedAt time.Time`
    - `Retired bool` — true if retired (in history)
    - `RetiredReason string`
    - `SupersededBy string` — ID of replacement entry, if superseded

## 2. Dossier Kinds and Schema
- [ ] Create `internal/core/dossiers/dossier.go`:
  - Define `DossierKind` string enum: `module`, `topic`, `external`, `procedure`
  - Define `Dossier` struct:
    - `Scope string` — e.g. `core/transcript-parser`
    - `Kind DossierKind`
    - `Title string` — human-readable title
    - `Purpose string` — "Purpose and boundaries" section text
    - `Entries []Entry` — active entries, sorted by type then ID
    - `History []Entry` — retired/superseded entries
  - Define `Atlas` struct:
    - `Dossiers []AtlasEntry`
  - Define `AtlasEntry` struct:
    - `Scope string`
    - `Kind DossierKind`
    - `Title string`
    - `EntryCount int`
    - `LastUpdated time.Time`

## 3. Op Log Projector
- [ ] Create `internal/core/dossiers/projector.go`:
  - `type Projector struct` — projects the op log into a set of dossiers
  - `func NewProjector(opsReader *ops.Reader) *Projector`
  - `func (p *Projector) Project(ctx context.Context) (map[string]*Dossier, error)`:
    - Iterate all ops via `opsReader.Iterate(ctx)`
    - For each op:
      - `add`: create entry, add to dossier (create dossier if new)
      - `amend`: update entry fields
      - `supersede`: retire old entry (move to History), create new entry
      - `retire`: move entry to History
      - `move`: remove from old dossier, add to new dossier
      - `split`: create new dossiers, move specified entries
    - Return map of scope → Dossier
  - `func (p *Projector) ProjectBefore(ctx context.Context, beforeCommit string) (map[string]*Dossier, error)`:
    - Same but using `opsReader.IterateBefore(ctx, beforeCommit)` for time-cut X6 support

## 4. Dossier Renderer
- [ ] Create `internal/core/dossiers/renderer.go`:
  - `func RenderDossier(d *Dossier) string` — renders a dossier to Markdown:
    - Title as `# <scope>`
    - `## Purpose and boundaries` + `d.Purpose` text
    - Then entries grouped by section, in this order:
      1. `## Invariants` (types: `invariant`)
      2. `## Ruled out, failure signatures and gotchas` (types: `ruled-out`, `failure-signature`, `gotcha`)
      3. `## Decisions` (types: `decision`)
      4. `## Procedures` (types: `procedure`)
      5. `## Interfaces and dependencies` (types: `interface`)
      6. `## Open issues` (types: `open-issue`)
    - Omit empty sections entirely
    - Each entry rendered as:
      ```
      ### [<id>] <first line of text as title>
      `<type> · anchors: <comma-separated> · verified <stamp> · evidence: <comma-separated>`
      <remaining lines of text>
      ```
    - Must be deterministic: same input → same output
  - `func RenderHistory(d *Dossier) string` — renders the `.history.md` with retired/superseded entries
  - `func RenderAtlas(atlas *Atlas) string` — renders `ATLAS.md`:
    - One line per dossier: `| <scope> | <kind> | <entry count> | <last updated> |`
    - Sorted by scope

## 5. Dossier Writer
- [ ] Create `internal/core/dossiers/writer.go`:
  - `type Writer struct` with base directory (the `.memory/` path)
  - `func NewWriter(baseDir string) *Writer`
  - `func (w *Writer) WriteDossier(d *Dossier) error`:
    - Write rendered dossier to `.memory/dossiers/<scope>.md`
    - Write rendered history to `.memory/dossiers/<scope>.history.md` (only if history non-empty)
    - Create directories as needed
  - `func (w *Writer) WriteAtlas(atlas *Atlas) error`:
    - Write rendered atlas to `.memory/ATLAS.md`
  - `func (w *Writer) WriteAll(dossiers map[string]*Dossier) error`:
    - Write all dossiers + history + atlas

## 6. Cap Linting
- [ ] Create `internal/core/dossiers/lint.go`:
  - `func LintDossier(d *Dossier) []string` — returns warnings:
    - "dossier <scope> exceeds ~3k token cap (estimated <n> tokens)" if rendered size > ~12k bytes (rough 3:1 ratio)
    - "entry <id> exceeds 6 lines" if entry text > 6 lines
    - "dossier has no purpose section" if Purpose is empty

## 7. Hand-Edit Detection
- [ ] Create `internal/core/dossiers/handedit.go`:
  - `func DetectHandEdits(rendered string, onDisk string) ([]ops.Op, error)`:
    - Compare the rendered version of a dossier against what's on disk
    - If they differ, return `amend` ops for changed entries and `add` ops for new entries
    - This is a best-effort heuristic: parse the on-disk Markdown looking for entry headers (`### [<id>]`), compare text content
    - Return empty slice if no differences

## 8. Tests
- [ ] Create `internal/core/dossiers/dossiers_test.go`:
  - **Round-trip test:** Write ops → project → render → re-render → verify identical output
  - **Projector test:** Add entries via add ops, then amend, supersede, retire, move. Verify final dossier state.
  - **Renderer test:** Render a dossier with entries of each type. Verify output matches expected Markdown.
  - **History test:** Retire and supersede entries, verify they appear in history
  - **Atlas test:** Build atlas from multiple dossiers, verify sorted output
  - **Cap lint test:** Create an oversized dossier, verify warning produced
  - **Empty sections test:** Verify empty sections are omitted from rendered output
  - Use `t.TempDir()` for all file I/O

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./internal/core/dossiers/...` — all tests must pass
3. Run `go vet ./internal/core/dossiers/...` — must pass
4. Verify the round-trip property: write → project → render → re-render gives identical output
5. Commit all changes on the current branch with a descriptive message

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report (written by the worker at task completion)

<!-- Template — append this section when done -->
### Summary of Work
- Created `entry.go` defining the `EntryType` enums and `Entry` schema.
- Created `dossier.go` defining the `DossierKind` and `Atlas` structs.
- Implemented `projector.go` to iterate op batches and compute final active and historical entries for the dossiers.
- Implemented `renderer.go` deterministically rendering active entries, retired entries to `.history.md`, and the root `ATLAS.md` table.
- Added `writer.go` logic to iterate dossiers and write rendering to standard disk files using nested folders per scope.
- Wrote `lint.go` providing max token capacities warnings and row size hints.
- Added `handedit.go` detection translating direct markdown modifications (headers + content diffing) into mapped standard OPs.
- Implemented robust `dossiers_test.go` confirming behavior of sorting rules, handedit diffing, missing section omission, and a complete pipeline round-trip assertion.

### Verification Results
1. `go build ./...` - Passed.
2. `go test ./internal/core/dossiers/...` - All tests passed.
3. `go vet ./internal/core/dossiers/...` - Passed.
4. Changes were explicitly approved by user and committed.

### Upstream Issues / Defects Found
- None found during execution.
