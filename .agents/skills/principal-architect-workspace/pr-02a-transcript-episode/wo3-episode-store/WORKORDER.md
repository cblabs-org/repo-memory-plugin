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
- [x] Implement `internal/core/episode/store.go` to handle storage at `<data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst`.
- [x] Implement Episode Header structures with `harness`, `harness_version`, `started_by`, `pipeline_version`, and gap recording.
- [x] Ensure episodes are read/written correctly (zstd compressed).

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
- Implemented `Store` in `internal/core/episode/store.go` to handle persisting and loading compacted episodes at `<data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst` using pure-Go zstd compression.
- Implemented `Header` structure with `harness`, `harness_version`, `started_by`, `pipeline_version`, `compactor_version`, `episode_id` (`ep-<ulid>`), `session_id`, `repo_id`, `parent_episode`, `branch`, `started`, `ended`, `head_start`, `head_end`, `commits`, `model`, and `stats`.
- Implemented flexible RFC3339/RFC3339Nano/empty/null timestamp parsing on `Header`.
- Implemented `StepGap [2]int` and gap detection/recording (`RecordGap`, `RecordGaps`, `DetectGaps`, `TotalMissingSteps`, `HasGaps`, `Gaps`).
- Implemented atomic file writing (temporary file in destination directory followed by atomic rename) to guarantee write integrity.
- Implemented `Read`, `ReadHeader`, `ReadFromPath`, `ReadHeaderFromPath`, `FindPath`, `Exists`, `List`, `ListPaths`, and `ListHeaders`.
- Added unit test suite in `internal/core/episode/store_test.go` covering serialization, zstd magic bytes (`0x28 0xb5 0x2f 0xfd`), read/write roundtrip, gap recording, multi-year path layout, searching across years, listing, and error validation.
- Added pure-Go `github.com/klauspost/compress/zstd` and `github.com/oklog/ulid/v2` dependencies to `go.mod`.

### Decisions Made
- Selected pure-Go `github.com/klauspost/compress/zstd` per SPEC §18.4 (Episode compression: Pure-Go zstd, zero cgo).
- Selected `github.com/oklog/ulid/v2` for generating standardized `ep-<ulid>` episode IDs.
- Implemented atomic rename on write within the target directory to prevent partial files or reader corruption.
- Implemented streaming chunk-based line reader `readLineBytes` with `bufio.Reader` to ensure arbitrarily long JSON lines do not fail with buffer overflow.
- Automated gap detection on write if gaps have not been explicitly populated.

### Dead Ends & Warnings
(none)

### Out-of-Scope Findings
(none)

### Blockers
(none)
