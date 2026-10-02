---
name: pr-02a-transcript-episode-wo3
description: Handoff instructions for Work Order 3 - Episode Store
complexity: standard
---

# WORKORDER Template

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement the episode store for storing compacted episodes.

## Strict Boundary Box
You must only work within `internal/core/episode/` and related tests. 

## Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

## Line-Item Instructions
- [ ] Implement `internal/core/episode/store.go` to handle storage at `<data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst`.
- [ ] Implement Episode Header structures with `harness`, `harness_version`, `started_by`, `pipeline_version`, and gap recording.
- [ ] Ensure episodes are read/written correctly (zstd compressed).

## Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
(pending)

### Changes Made
(pending)

### Decisions Made
(pending)

### Dead Ends & Warnings
(pending)

### Out-of-Scope Findings
(pending)

### Blockers
(none)
