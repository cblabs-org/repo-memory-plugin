---
name: pr-02b-capture-runtime-wo4
description: Handoff instructions for Work Order 4
complexity: architect
---

# pr-02b-capture-runtime / WO4 - tick-core

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement the tick loop that ingests the spool, runs jobs, and sweeps enrolled repositories.

## Strict Boundary Box
- Do not modify hook logic.

## Tooling Constraints
Use `go build` and `go test` to verify changes.

## Line-Item Instructions
- [ ] Create `internal/core/tick/` package.
- [ ] Implement the tick loop (Section 4.3).
- [ ] Implement portable `tick.lock` (`flock` on Unix, `LockFileEx` on Windows).
- [ ] Implement spool ingestion into jobs, and lease reclamation.
- [ ] Implement the sweep of enrolled repos only (Antigravity app & CLI stores). Include idle promotion (15 min) and incremental copying.
- [ ] Enforce the tick time budget. Implement overlap behavior (per E6).

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
