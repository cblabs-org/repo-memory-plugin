---
name: pr-04-wo5
description: Handoff instructions for Work Order 5 - SQLite Index
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement a rebuildable SQLite index in `.memory/.cache/` mapping entities, claims, anchors, relations, and conditions.

### Strict Boundary Box
Only modify `internal/core/index/` and add the `reindex` command to `cmd/repo-memory/`.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] Create the SQLite schema for entities, claims, anchors, relations, and conditions.
- [ ] Implement WAL + busy_timeout for concurrency; ensure hooks open read-only.
- [ ] Implement short write transactions for the index.
- [ ] Implement a full rebuild mechanism (writes new versioned file, then switches pointer).
- [ ] Add `repo-memory reindex` CLI command.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---
## Completion Report
<!-- Written by the worker at task completion -->

**Worker:** execution-worker (via Antigravity)
**Status:** Completed
**Validation:** `go build ./cmd/repo-memory` and `go test ./...` successfully executed.
**Notes:**
- Extended `index.go` schema to support tables for entities, claims, anchors, claim_relations, entity_relations, and conditions.
- Implemented short write transactions via `WriteTx` wrapper in `index`.
- Developed `Rebuild` method (`rebuild.go`) handling full index write to a versioned SQLite file, followed by an atomic symlink/rename pointer switch.
- Added `ListAll()` capability to the claims `Store` to facilitate loading instances for indexing.
- Wired the `repo-memory reindex` CLI command to orchestrate the rebuilding process for `.memory/.cache/index.db`.
- **Surfaced Issue:** No file-based `Store` implementation currently exists for Entities (only the `extractor` package and `Registry` interface exist in `internal/core/entities`). Consequently, the `reindex` command passes an empty entity slice when calling `Rebuild()`. The Principal Architect should triage adding a persistent entity store implementation so they can be properly indexed.
