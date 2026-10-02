---
name: pr-02b-capture-runtime-wo7
description: Handoff instructions for Work Order 7
complexity: mechanical
---

# pr-02b-capture-runtime / WO7 - claude-raw-copy

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Extend the sweep to copy Claude Code transcripts.

## Strict Boundary Box
- Only implement the raw copy. Do not attempt to parse the transcripts yet.

## Tooling Constraints
Use `go build` and `go test` to verify changes.

## Line-Item Instructions
- [x] Extend the sweep logic to copy Claude Code transcripts from `~/.claude/projects/` to `<data-dir>/<repo-id>/raw/claude-code/`.
- [x] Respect permissions (read only).

## Verification Requirement
You must verify it compiles and tests pass. `go build ./cmd/repo-memory` and `go test ./...`. Before committing, ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, surface it in your Completion Report below.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
Complete & Approved

### Changes Made
- Added `TranscriptCopier` interface/type and registration support in `internal/core/tick/sweep.go` and `internal/core/tick/tick.go` while maintaining strict core/adapter boundary separation.
- Implemented Claude Code raw transcript copying in `internal/harness/claudecode/copier.go`, scanning `~/.claude/projects/`, matching project directories to enrolled repositories by root path, and copying top-level and subagent (`subagents/*.jsonl`) transcripts into `<data-dir>/<repo-id>/raw/claude-code/` with read-only source permissions and skipping `memory/` directories.
- Wired `claudecode.CopyTranscripts` in `cmd/repo-memory/cmd_tick.go`.
- Added unit tests in `internal/harness/claudecode/copier_test.go`.

### Decisions Made
- Used a decoupled registration pattern (`tick.RegisterCopier`) to keep `internal/core` free of adapter dependencies.

### Dead Ends & Warnings
- None.

### Out-of-Scope Findings
- Transcript parsing and compaction are deferred to Release 2 (R2.1) per SPEC §18.6 rule 11.

### Blockers
- None.
