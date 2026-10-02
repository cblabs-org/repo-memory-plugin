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
- [x] Implement `internal/core/pipeline/compact/` to perform deterministic compaction per SPEC §5.1.
- [x] Implement author classification (`human` vs `automation`).
- [x] Implement exposure markers (dropping actual memory content but keeping references).
- [x] Implement secret scrubber (pattern + entropy) replacing matches with `«secret:type»`.
- [x] Write tests ensuring target compaction (≥ 4× reduction) is measured and verified on full transcripts.

## Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
Complete

### Changes Made
- Implemented `Compactor.Compact()` in `internal/core/pipeline/compact/compact.go` per SPEC 5.1 rules (truncating command output, collapsing identical repeating tasks).
- Added `classifyAuthor()` to determine if a prompt is from a `human` or `automation` (e.g., scheduled ticks, system messages, agentapi).
- Stripped actual content text from exposure markers and reads/edits, keeping only reference IDs.
- Implemented `scrubSecrets()` with regex + basic heuristic to replace AWS keys, UUIDs, Tokens, and high-entropy hex with `«secret:type»`.
- Included `compact_test.go` checking for proper sequence handling, assertions for correct parsing, and confirming a raw footprint size reduction of >= 4x.
- Marked all checklists complete.

### Decisions Made
- Chose simple heuristic checks for `classifyAuthor` based on keyword analysis of transcript text.
- Collapsing duplicate identical log lines for device/test outputs ensures major compaction without losing structure.
- Bounded command excerpt truncation retains failing markers or the last 5-10 lines of output for contexts yielding a non-zero exit code.

### Dead Ends & Warnings
(none)

### Out-of-Scope Findings
(none)

### Blockers
(none)
