---
name: pr-02b-capture-runtime-wo2
description: Handoff instructions for Work Order 2
complexity: standard
---

# pr-02b-capture-runtime / WO2 - stop-hook

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement the `Stop` hook in `internal/harness/antigravity/hooks.go`.

## Strict Boundary Box
- Do not implement the job table or tick logic.
- Do not import concrete adapters in `internal/core`.

## Tooling Constraints
Use `go build` and `go test` to verify changes.

## Line-Item Instructions
- [ ] In `internal/harness/antigravity/hooks.go`, handle the `Stop` event.
- [ ] Add kill-switch and enrolment checks first.
- [ ] Add `REPO_MEMORY_HOUSEKEEPING` guard to ensure the hook skips housekeeping runs.
- [ ] Execute spool entry creation and incremental copy (using the packages from WO1).
- [ ] Ensure execution budget is ≤ 200ms.

## Verification Requirement
You must verify it compiles and tests pass. `go build ./cmd/repo-memory` and `go test ./...`. Before committing, ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
### Changes Made
### Decisions Made
### Dead Ends & Warnings
### Out-of-Scope Findings
### Blockers
