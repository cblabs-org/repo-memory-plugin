---
name: pr-03-episode-search-wo3
description: Handoff instructions for Work Order 3
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement functionality to import Principal Architect files (PROJECT.md, WORKORDER.md, context_bridge.md) as episodes into the episode store, so they become searchable.

### Strict Boundary Box
Do not change the FTS search CLI or indexing logic. Only focus on parsing PA files and creating `EpisodeRecord` entries in the episode store. Ensure `harness` is set to `"import"`.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] Add an import tool or CLI subcommand (e.g., `repo-memory import-pa`) to scan a given directory (or the default PA workspace) for `PROJECT.md`, `WORKORDER.md`, and `context_bridge.md` files.
- [ ] For each file found, generate an episode. The episode should have `harness: "import"`.
- [ ] Extract the date from the file's content (e.g., frontmatter, Session Log dates) or fall back to git history/file modification time.
- [ ] Store one episode per file into the episode store using existing store logic.
- [ ] Add tests verifying the parsing and import logic for PA files.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints (`go build ./cmd/repo-memory` and `go test ./...`). **CRITICAL:** Commit your changes to the branch after verifying tests pass, as requested by the PA SYSTEM_MESSAGE.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `cmd/repo-memory/cmd_import_pa.go` — Added the `import-pa` command to parse `PROJECT.md`, `WORKORDER.md`, and `context_bridge.md` files and store them as episodes in the JSONL store.
- `cmd/repo-memory/cmd_import_pa_test.go` — Added unit tests verifying the directory scanning and episode creation logic.
- `cmd/repo-memory/main.go` — Registered the `importPACmd` into the CLI router.

### Out-of-Scope Findings
<!-- Defects, bugs, or concerns outside the boundary box. The PA will triage these. -->
- Work order originally mentioned creating `EpisodeRecord` entries directly, but `harness: "import"` refers to the JSONL store `episode.Episode` header schema. I implemented the latter (saving to JSONL store via `episode.NewStore`), which behaves seamlessly with downstream components like FTS index population.

### Blockers
<!-- Empty if status is 'complete'. If partial/blocked, explain what prevents completion. -->
(none)
