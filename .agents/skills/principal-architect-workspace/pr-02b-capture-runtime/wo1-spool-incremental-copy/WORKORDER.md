---
name: pr-02b-capture-runtime-wo1
description: Handoff instructions for Work Order 1
complexity: standard
---

# pr-02b-capture-runtime / WO1 - spool-incremental-copy

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement spool entry schema, a writer with Windows-safe rename semantics, a reader for the tick, and incremental transcript copying.

## Strict Boundary Box
- Do not modify existing capture schemas outside of the spool and incremental copy logic.
- Do not implement the `Stop` hook itself or the job table (those are future WOs).
- Do not import concrete adapters in `internal/core`.

## Tooling Constraints
Use `go build` and `go test` to verify changes. 

## Line-Item Instructions
- [ ] Create `internal/core/spool/` package.
- [ ] Define spool entry schema (JSON).
- [ ] Implement a spool writer that writes to a unique temporary name in the spool directory and then atomically renames it (ensure Windows semantics are respected).
- [ ] Implement a spool reader for the tick to ingest these entries.
- [ ] Implement incremental transcript copy logic: store byte offset and prefix hash in `state/`. If the prefix changed, perform a full re-copy.

## Verification Requirement
You must verify it compiles and tests pass. `go build ./cmd/repo-memory` and `go test ./...`. Before committing, ask the user if they are happy with the changes (since I am the PA, you will ask me).

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete | partial | blocked

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
