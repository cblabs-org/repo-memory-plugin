---
name: pr-02a-transcript-episode-wo5
description: Handoff instructions for Work Order 5 - Metrics Skeleton
complexity: standard
---

# WORKORDER Template

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement metrics writing to record pipeline telemetry and costs.

## Strict Boundary Box
You must only work within `internal/core/metrics/` and related tests. 

## Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

## Line-Item Instructions
- [ ] Implement `internal/core/metrics/` package with a JSONL metrics writer.
- [ ] Capture health metrics: sessions seen, captured, skipped, missed.
- [ ] Capture hook execution time, tick duration, budget exhaustion.
- [ ] Implement a tool/test to re-measure §4.4.1 transcript sizes from merged full transcripts.

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
