---
name: pr-02a-transcript-episode-wo2
description: Handoff instructions for Work Order 2 - Compaction
complexity: architect
---

# WORKORDER Template

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement deterministic compaction per SPEC 5.1 to reduce transcript footprint by ≥4x.

## Strict Boundary Box
You must only work within `internal/core/pipeline/compact/` and related tests. 

## Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

## Line-Item Instructions
- [ ] Implement `internal/core/pipeline/compact/` to perform deterministic compaction per SPEC §5.1.
- [ ] Implement author classification (`human` vs `automation`).
- [ ] Implement exposure markers (dropping actual memory content but keeping references).
- [ ] Implement secret scrubber (pattern + entropy) replacing matches with `«secret:type»`.
- [ ] Write tests ensuring target compaction (≥ 4× reduction) is measured and verified on full transcripts.

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
