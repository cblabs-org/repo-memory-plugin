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
- [x] Implement commit linkage logic per §6.2.
- [x] Link commits on the session's branch between the session's start and end times from git log/reflog.
- [x] Confirm those commits against commit commands in the transcript.
- [x] Record the linked commits in the episode header.

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
- Implemented `CommitLinker` and linkage engine in `internal/core/episode/commit_linkage.go` per SPEC §6.2.
- Created `GitRunner` interface and `ExecGitRunner` default implementation for testable and pluggable git execution.
- Added candidate commit discovery querying both `git log` and `git reflog` for commits on the session's branch within the session's `[Started, Ended]` timeframe (with configurable clock skew margin `timeMargin`).
- Implemented `ExtractTranscriptCommits` and commit command confirmation logic:
  - Supports direct commit hash matching against command excerpts, output brackets `[<branch> <sha>]`, text, and explicit `step.Commit`.
  - Supports commit subject/message matching against `-m`, `-am`, and `--message` flags.
  - Supports sequence correlation of successful commit commands with commits on the branch.
  - Rejects unconfirmed commits (e.g. commits on branch created outside session) and failed commands (exit != 0 or 'nothing to commit').
- Added chronological commit ordering and deduplication, recording confirmed commits into `header.Commits` and updating `header.HeadEnd`.
- Implemented edit step resolution (`resolveEditSteps`), populating `step.Commit` for edit steps whose modified file was touched in one of the confirmed commits per SPEC §5.1 / §6.2.
- Updated `Header.UnmarshalJSON` in `internal/core/episode/store.go` to flexibly handle various representations of `commits` (slice of strings, comma/space-delimited string, array of map objects with sha/commit/hash keys, or null).
- Added `ParseHeader` and `ParseHeaderLine` functions in `internal/core/episode/store.go`.
- Added helper methods `RecordCommit`, `RecordCommits`, `HasCommits`, and `HasCommit` to both `Header` and `Episode`.
- Added unit and integration tests in `internal/core/episode/commit_linkage_test.go` and `internal/core/episode/store_test.go` verifying mock git log/reflog matching, error handling, time filtering, and real Git repository execution.

### Decisions Made
- Reversing raw git log and reflog output (which lists newest commits first) ensures that commits occurring within the same second preserve their chronological parent/creation ordering.
- A configurable `timeMargin` (default 60 seconds) handles timestamp granularity rounding and minor clock skew between transcript logs and git committer timestamps without false positives, because commits still require transcript command confirmation.
- Edit steps (`StepKindEdit`) in the episode are automatically matched to commits via `git diff-tree --name-only -r --root <hash>` where possible.

### Dead Ends & Warnings
- `git log -g` (reflog walker) does not permit `--reverse` flag in git (`fatal: options '--reverse' and '--walk-reflogs' cannot be used together`). Reversed the candidate slice in Go instead of passing `--reverse` to git reflog.

### Out-of-Scope Findings
(none)

### Blockers
(none)
