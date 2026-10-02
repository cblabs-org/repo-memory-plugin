---
name: pr-03-episode-search-wo1
description: Handoff instructions for Work Order 1
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement a SQLite FTS5 index over episodes (WAL, read-only readers) with specific columns, and populate it from the episode store.

### Strict Boundary Box
Do not modify pipeline execution logic or entity extraction. Only create/modify index creation and population logic for episodes. Do not implement the search CLI here.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes. 

### Line-Item Instructions
- [ ] Create/update `internal/core/index/` (or appropriate package for episode FTS) to implement SQLite FTS5 indexing over episodes.
- [ ] Configure WAL mode and read-only readers for SQLite connections.
- [ ] Ensure the FTS table schema includes: `episode_id`, `session_id`, `content` (compacted text), `files_touched`, `outcomes`, `date`.
- [ ] Implement logic to populate this FTS index from the episode store (`internal/core/episode/store.go` or similar).

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints (`go build ./cmd/repo-memory` and `go test ./...`). **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes. Note: in this programmatic loop, just commit after verifying tests pass, as requested by the PA SYSTEM_MESSAGE which overrides this. Wait, the system message said "Workers must verify their changes compile (`go build ./cmd/repo-memory` and `go test ./...`). Commit after each WO." so you should just commit your changes.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `internal/core/index/index.go` — Added `Index` struct with methods `NewIndex`, `NewReadOnlyIndex`, `InsertEpisode`, and `SearchEpisodes`. Configured FTS5 schema and WAL pragmas.
- `internal/core/index/populate.go` — Implemented logic to populate the FTS5 index from the episode store, compacting steps into `content`, `files_touched`, and `outcomes`.
- `internal/core/index/index_test.go` — Added unit tests verifying search and store population logic.

### Decisions Made
- Date logic: For indexing, if an episode's start date is zero, we fall back to the end date. Also used `time.RFC3339` for string serialization in the `date` unindexed column since `time.Time` support in Go's `sql` package may behave unexpectedly without proper mapping, whereas text is highly deterministic in SQLite.

### Dead Ends & Warnings
- Ensure that the episode store has data written using `Write()` for population testing. Initially used a stub that wasn't writing completely before `PopulateFromStore()` was called. 

### Out-of-Scope Findings
- None

### Blockers
(none)
