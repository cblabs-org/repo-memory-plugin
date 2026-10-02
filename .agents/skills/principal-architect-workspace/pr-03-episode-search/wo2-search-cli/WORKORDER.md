---
name: pr-03-episode-search-wo2
description: Handoff instructions for Work Order 2
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement the `repo-memory search <query>` CLI command to perform full-text search over episodes, using the FTS index implemented in WO1. Rank results by BM25.

### Strict Boundary Box
Do not modify the underlying FTS5 schema created in WO1. Limit changes to the CLI command package (`cmd/repo-memory/...` or similar) and the index querying logic to format the results.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes. 

### Line-Item Instructions
- [ ] Ensure the FTS search query ranks results using SQLite's built-in `bm25()` function.
- [ ] Add the `search` subcommand to the CLI.
- [ ] Connect the subcommand to the `Index.SearchEpisodes` (or similar) method implemented in WO1.
- [ ] Format the output of the CLI to display: episode ID, date, branch, a snippet (using SQLite's `snippet()` function on `content`), and files touched.
- [ ] Write/update unit tests for the search CLI logic.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints (`go build ./cmd/repo-memory` and `go test ./...`). **CRITICAL:** Commit your changes to the branch after verifying tests pass, as requested by the PA SYSTEM_MESSAGE.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete | partial | blocked

### Changes Made
- `path/to/file` — what changed and why

### Decisions Made
<!-- Only non-obvious decisions. Omit this section entirely for mechanical WOs. -->
- Decision description and rationale

### Dead Ends & Warnings
<!-- Things that didn't work. Critical for preventing future agents from repeating mistakes. -->
- What was attempted, why it failed, and what to avoid

### Out-of-Scope Findings
<!-- Defects, bugs, or concerns outside the boundary box. The PA will triage these. -->
- Finding description and suggested action

### Blockers
<!-- Empty if status is 'complete'. If partial/blocked, explain what prevents completion. -->
(none)
