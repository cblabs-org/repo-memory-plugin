---
name: pr-02a-transcript-episode-wo1
description: Handoff instructions for Work Order 1 - Transcript Parser
complexity: architect
---

# WORKORDER Template

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Build the transcript parser to read `transcript_full.jsonl` or fallback chunks, handle step index gaps and CHECKPOINTs, and classify step sources/types.

## Strict Boundary Box
You must only work within `internal/harness/antigravity/` for the parsing logic, and `testdata/fixtures/` for test fixtures. Do not modify `internal/core/` beyond adding necessary struct types if absolutely required, but try to keep parser implementation isolated. `import` boundaries: `internal/core` must NEVER import `internal/harness`. 

## Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes. 

## Line-Item Instructions
- [ ] Implement `internal/harness/antigravity/transcript.go`: read one source in priority order: `transcript_full.jsonl`; else `chunks/transcript_full/*.jsonl` concatenated in lexical order as a byte stream (io.MultiReader) before line splitting; else `transcript.jsonl` with `truncated_fields` flagged.
- [ ] Implement handling for step gaps and `CHECKPOINT` truncation markers.
- [ ] Implement source/type classification.
- [ ] Create a scrubbed fixture corpus under `testdata/fixtures/antigravity/<version>/`, including a chunked conversation with records straddling boundaries, and a compact-only conversation with truncated lines. Use mock data devoid of real personal content.
- [ ] Write a canary test that flags drops in parsed-step counts when the Antigravity version changes.

## Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes (since you run autonomously, ensure all tests pass cleanly).

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
Completed

### Changes Made
- Implemented `ParseTranscript` in `internal/harness/antigravity/transcript.go` that prioritizes reading `transcript_full.jsonl`, chunked `transcript_full/*.jsonl`, and fallback `transcript.jsonl`.
- Added `Step` struct that supports identifying `CHECKPOINT` rows and logging gap locations via `IsGap` / `GapSize`.
- Added source/type classification method `Classification() (actor string, action string)` to `Step`.
- Wrote robust unit tests and a suite of fixtures representing standard, chunked, and truncated transcript structures under `testdata/fixtures/antigravity/v1/`.
- Added a canary test to flag when parsed step counts change due to an unexpected format bump (e.g., `v2`).

### Decisions Made
- Used `bufio.Scanner` with a generous 50MB buffer on top of an `io.MultiReader` to ensure lexical chunks that straddle a single JSON line can seamlessly parse into unified lines before being evaluated.

### Dead Ends & Warnings
- `cat`'s EOF behavior on newlines broke the chunked test initially. Fixed using `printf` to ensure strict byte continuity over chunks without injection of newlines.

### Out-of-Scope Findings
None.

### Blockers
(none)
