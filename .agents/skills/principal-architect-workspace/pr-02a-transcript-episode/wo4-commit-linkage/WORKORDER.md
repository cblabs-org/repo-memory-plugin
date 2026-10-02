---
name: pr-02a-transcript-episode-wo4
description: Handoff instructions for Work Order 4 - Commit Linkage
complexity: standard
---

# WORKORDER Template

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement commit linkage from git log/reflog for the session's branch between start and end.

## Strict Boundary Box
You must only work on commit linkage logic and updates to the episode header parsing. 

## Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

## Line-Item Instructions
- [ ] Implement commit linkage logic per §6.2.
- [ ] Link commits on the session's branch between the session's start and end times from git log/reflog.
- [ ] Confirm those commits against commit commands in the transcript.
- [ ] Record the linked commits in the episode header.

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
