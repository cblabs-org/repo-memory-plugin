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
- [ ] Extend the sweep logic to copy Claude Code transcripts from `~/.claude/projects/` to `<data-dir>/<repo-id>/raw/claude-code/`.
- [ ] Respect permissions (read only).

## Verification Requirement
You must verify it compiles and tests pass. `go build ./cmd/repo-memory` and `go test ./...`. Before committing, ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, surface it in your Completion Report below.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
### Changes Made
### Decisions Made
### Dead Ends & Warnings
### Out-of-Scope Findings
### Blockers
