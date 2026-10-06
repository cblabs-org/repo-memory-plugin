---
name: pr-07-v2-storage-wo6
description: Handoff instructions for Work Order 6 — Seeding
complexity: architect
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Build the seeding subsystem that produces initial dossiers **without** the full turn-note distillation pipeline (which comes in PR-10). Seeding uses two sources:

1. **PA completion reports** — structured Markdown written by Principal Architect sessions into `WORKORDER.md` files. Parse "Decisions Made", "Changes Made", "Dead Ends & Warnings", and "Out-of-Scope Findings" into ops.
2. **Lean-agent evidence pass** — a one-off batch process that reads existing evidence files (episodes, notes, artifacts) and generates ops via the `repo-memory-distiller` agent (or a local equivalent).

The seeded dossiers will be reviewed at the X6-lite kill gate (PR-09).

# Context

## v2 Design References
- **§5.5 (PA reports):** PA completion reports are a free distillation source — they contain structured decisions, dead ends, and findings that map directly to entry types.
- **§7.4 (Op log):** Seeded ops use `source: seed` provenance.

## Dependencies (WO1-WO5, already built)
- `internal/core/ops/` — Op, OpBatch, Writer, NewEntryID
- `internal/core/dossiers/` — Entry types (decision, ruled-out, failure-signature, gotcha, procedure, invariant), Projector, Renderer, Writer
- `internal/core/config/` — ScopeResolver, DeriveModuleScopes
- `internal/core/index/` — RebuildFromOpLog

## PA Completion Report Format
Completion reports are appended to `WORKORDER.md` files in:
```
.agents/skills/principal-architect-workspace/<project>/wo<N>-<topic>/WORKORDER.md
```

The relevant sections in a completion report:
```markdown
## Completion Report

### Status
complete

### Changes Made
- `path/to/file` — what changed and why

### Decisions Made
- Decision description and rationale

### Dead Ends & Warnings
- What was attempted, why it failed, and what to avoid

### Out-of-Scope Findings
- Finding description and suggested action

### Blockers
(none)
```

## Evidence Files (in `<data-dir>/<repo-id>/`)
- `evidence/episodes/<session-id>/` — captured episodes
- `evidence/notes/` — extracted notes
- `evidence/artifacts/` — captured artifacts

## Source Tagging
All seeded ops MUST have `Source: "seed"` so they can be distinguished from runtime-produced knowledge.

# Strict Boundary Box

**DO NOT touch:**
- `internal/core/ops/` — use it, don't modify it
- `internal/core/dossiers/` — use it, don't modify it
- `internal/core/config/` — use it, don't modify it
- `internal/core/index/` — use it, don't modify it
- `internal/core/delivery/` — PR-08
- `internal/harness/` — harness code
- `.agents/` — PA workspace files (read the WORKORDERs but don't modify them)

**DO create:**
- `internal/core/seed/` — new package (the deliverable)
- `cmd/repo-memory/cmd_seed.go` — CLI subcommand

# Tooling Constraints

- **Build:** `go build ./...` must pass
- **Test:** `go test ./internal/core/seed/...` must pass
- **Vet:** `go vet ./internal/core/seed/...` must pass

# Line-Item Instructions

## 1. PA Report Parser
- [ ] Create `internal/core/seed/parser.go`:
  - `func ParseCompletionReport(content string) (*CompletionReport, error)`:
    - Parse a WORKORDER.md file, extract the `## Completion Report` section
    - Return structured data: Status, ChangesMade (list of file→description), DecisionsMade, DeadEnds, OutOfScopeFindings
  - `type CompletionReport struct`:
    - `Status string`
    - `Changes []Change` (Path, Description)
    - `Decisions []string`
    - `DeadEnds []string`
    - `Findings []string`
    - `ProjectName string` — derived from the WORKORDER path
    - `WorkOrderName string`

## 2. Report-to-Ops Converter
- [ ] Create `internal/core/seed/convert.go`:
  - `func ReportToOps(report *CompletionReport, resolver *config.ScopeResolver) []ops.Op`:
    - Map each Decision → `add` op with type `decision`, entry ID prefix `dec-`
    - Map each Dead End → `add` op with type `ruled-out` or `failure-signature`, entry ID prefix `rul-` or `fail-`
    - Map each Finding → `add` op with type `open-issue`, entry ID prefix `issue-`
    - Map each Change → anchors for the entry (use the file path)
    - Use `resolver.ResolveScope()` to determine the dossier scope from the change paths
    - All ops get `Source: ops.SourceSeed`

## 3. Workorder Scanner
- [ ] Create `internal/core/seed/scanner.go`:
  - `func ScanWorkorders(paDir string) ([]string, error)`:
    - Walk `.agents/skills/principal-architect-workspace/` recursively
    - Find all `WORKORDER.md` files that contain a `## Completion Report` section
    - Return the list of file paths
  - `func SeedFromWorkorders(paDir string, opsWriter *ops.Writer, resolver *config.ScopeResolver) error`:
    - Scan all workorders, parse each, convert to ops, write batches
    - Skip workorders with no completion report
    - Log progress: "Seeded N entries from M workorders"

## 4. Time-Cut Support
- [ ] Add to `internal/core/seed/scanner.go` or a new file:
  - `func SeedWithTimeCut(paDir string, opsWriter *ops.Writer, resolver *config.ScopeResolver, beforeCommit string) error`:
    - Same as SeedFromWorkorders but only includes workorders from projects whose branch was merged before the given commit
    - This supports X6-lite: seed only knowledge that existed at a specific point in history
    - For now, a simple implementation: use git to check if the workorder file existed at the given commit (`git show <commit>:<path>`)

## 5. CLI Subcommand
- [ ] Create `cmd/repo-memory/cmd_seed.go`:
  - Add a `seed` subcommand to the CLI
  - Flags:
    - `--pa-dir` — path to the PA workspace (default: `.agents/skills/principal-architect-workspace/`)
    - `--ops-dir` — path to the ops directory (default: `.memory/ops/`)
    - `--config` — path to config.yaml (default: `.memory/config.yaml`)
    - `--before` — optional git commit for time-cut seeding
    - `--dry-run` — print what would be seeded without writing
  - Register the subcommand in the CLI root (check how other subcommands like `reindex` are registered)

## 6. Tests
- [ ] Create `internal/core/seed/seed_test.go`:
  - **Parser test:** Parse a sample WORKORDER.md with a completion report. Verify all sections extracted correctly.
  - **Parser edge cases:** No completion report section → nil result. Empty sections → empty slices.
  - **Convert test:** Convert a CompletionReport to ops. Verify correct entry types, source tagging, scope resolution.
  - **Scanner test:** Create temp dir with mock WORKORDER.md files, scan, verify correct files found.
  - **Round-trip test:** Scan → parse → convert → write ops → project dossiers → verify entries appear with correct types and scopes.

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./internal/core/seed/...` — all tests must pass
3. Run `go vet ./internal/core/seed/...` — must pass
4. Manually verify: run `go run ./cmd/repo-memory seed --dry-run --pa-dir .agents/skills/principal-architect-workspace/` and confirm it finds the existing completion reports in this repo

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

<!-- Template — append this section when done -->
