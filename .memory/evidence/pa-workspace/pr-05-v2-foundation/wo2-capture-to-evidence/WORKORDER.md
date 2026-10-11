---
name: pr-05-v2-foundation-wo2
description: Handoff instructions for Work Order 2 — fix capture→evidence pipeline
complexity: standard
---

# WO2: Capture to Evidence

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Fix the broken link between capture and the episode/evidence store. Currently `convertCapturesToTriage` in `internal/core/tick/pipeline.go` dequeues `KindCapture` jobs and enqueues `KindTriage` jobs with the conversation ID as payload — but `processTriage` tries to read an episode by that ID, and no episode exists because nothing ever parsed the raw transcript. The result: 124 captured conversations, 0 processed.

Replace this with an **evidence pipeline** that:
1. Reads the raw transcript from the spool (already copied by the capture stage)
2. Parses it into a structured evidence record (episode metadata + parsed checkpoint sections)
3. Stores the evidence record in the episode store
4. Removes the `KindCapture` → `KindTriage` hand-off (the v1 triage→extract→reconcile→verify pipeline is being removed in a later WO)

## Strict Boundary Box
- **Modify:** `internal/core/tick/pipeline.go` (the `convertCapturesToTriage` function and its call in `Run`)
- **Modify:** `internal/core/episode/store.go` (add a method to write an evidence record from a raw transcript path, if needed)
- **Modify:** `internal/core/config/layout.go` (add `evidence` to `DataDirSubdirectories`)
- **Create:** `internal/core/evidence/` package if beneficial for checkpoint parsing
- **DO NOT** modify any other packages (triage, extract, reconcile, verify, publish, entities, claims, index, events, delivery, mcpserver)
- **DO NOT** modify hooks, harness code, or the spool
- **DO NOT** run any LLM calls or use the executor
- If you discover issues outside this boundary, surface them in your Completion Report.

## Tooling Constraints
- Use `go build ./...` and `go vet ./...` to verify compilation
- Run `go test ./internal/core/tick/... ./internal/core/episode/...` to verify tests pass
- Use standard Go idioms; no new dependencies

## Context

### How capture currently works (correct — don't change this)
1. The `Stop` hook in `internal/harness/antigravity/hooks.go` calls `spool.CopyTranscript()` to copy the transcript files
2. The spool writer creates a `KindCapture` job in the job table with the spool entry (conversation ID, paths) as payload
3. The tick's sweep also creates capture jobs for idle conversations

### What's broken (fix this)
- `convertCapturesToTriage` (pipeline.go:122) dequeues `KindCapture` and enqueues `KindTriage` with just the conversation ID
- `processTriage` (pipeline.go:153) calls `p.epStore.Read(epID)` but no episode with that ID exists — nothing parsed the transcript
- The v1 flow assumed episodes existed before triage; they were created by the PA import path, not by capture

### What the evidence record should contain
An evidence record is a lightweight episode. At minimum:
- Session ID (conversation ID)
- Source (captured transcript path in the spool)
- Timestamp (from the transcript)
- Step count
- Parsed CHECKPOINT sections (if any): extract structured fields from CHECKPOINT steps' `content` field. The typical sections are "User Knowledge", "Model Knowledge", "Work Accomplished", "Current Work", "Outstanding User Requests", "Files and Code". Store these as a map of section name → text content.
- Status: `captured` (just stored) or `parsed` (checkpoints extracted)

### Spool entry structure
Look at `internal/core/spool/entry.go` for `Entry` and `internal/core/spool/copy.go` for `CopyTranscript`. The spool entry has `ConversationID`, `TranscriptPath` (the copied JSONL file), and `BrainDir` (the original brain directory).

### Episode store
Look at `internal/core/episode/store.go`. The `Episode` type has a `Header` with metadata and `Steps` with the parsed content. You can either:
- (a) Use the existing `Episode` type and `WriteEpisode` to store evidence as episodes (simpler), or
- (b) Create a new evidence record type alongside episodes

Option (a) is preferred for now — use the existing episode infrastructure. The key change is: read the raw transcript JSONL, parse the steps, extract metadata, and write as an episode using the existing store.

## Line-Item Instructions

- [x] **Add `evidence` to `DataDirSubdirectories`** in `internal/core/config/layout.go`. This gives us `<data-dir>/<repo-id>/evidence/` for future use.

- [x] **Replace `convertCapturesToTriage` with `convertCapturesToEvidence`** in `internal/core/tick/pipeline.go`:
  - Dequeue `KindCapture` jobs (same as now)
  - Unmarshal the spool entry from the payload
  - Read the transcript JSONL from the spool entry's `TranscriptPath`
  - Parse it into steps using the transcript parser (see `internal/harness/antigravity/transcript.go` — the `ParseTranscript` or equivalent function)
  - Extract CHECKPOINT steps and parse their structured sections
  - Create an `Episode` with the parsed steps and metadata
  - Write it via `p.epStore.WriteEpisode(ep)`
  - Complete the capture job (no triage job enqueued — the v1 pipeline stages will be removed later)

- [x] **Remove the `KindTriage` enqueue** — evidence records go straight to the episode store, not through triage. The v1 pipeline stages (triage, extract, reconcile, verify) still exist in code but won't receive new jobs from capture.

- [x] **Update `PipelineExecutor.Run`** — call `convertCapturesToEvidence` instead of `convertCapturesToTriage`. The rest of the pipeline (`processTriage`, `processExtract`, etc.) stays for now — they'll still process any pre-existing triage/extract/reconcile/verify jobs in the queue from before, but new captures won't feed into them.

- [x] **Add checkpoint section parsing.** When a step has type `CHECKPOINT` (or source `SYSTEM` with checkpoint-like content), parse the content to extract the structured sections. Store them in the episode's metadata or as a field on the episode. A simple approach: scan for `## Section Name` headers and capture the text between them.

- [x] **Test: write a test** in `internal/core/tick/pipeline_test.go` that:
  - Creates a mock spool entry pointing to a test transcript JSONL file
  - Enqueues a `KindCapture` job with that entry
  - Runs `convertCapturesToEvidence`
  - Verifies the episode was written to the store with correct metadata and parsed checkpoints

- [x] **Verify:** `go build ./...`, `go vet ./...`, and `go test ./internal/core/tick/... ./internal/core/episode/...` all pass.

## Verification Requirement
Run `go build ./...`, `go vet ./...`, and the relevant tests. Commit when clean. **CRITICAL:** Before committing, verify the working tree is clean with `git status`.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report (written by the worker at task completion)

### Status
Complete

### Changes Made
- `internal/core/config/layout.go`: Added `"evidence"` to `DataDirSubdirectories`.
- `internal/core/episode/store.go`: Added `Source`, `Status`, `StepCount`, `Checkpoints map[string]string`, and `Metadata map[string]any` to `episode.Header`, plus `SetCheckpoint` and `Checkpoint` helper methods.
- `internal/core/evidence/checkpoint.go`: Implemented `ParseCheckpointSections` to extract structured sections from checkpoint markdown headers (`## Section Name`, `### N. Section Name`) into canonical and custom sections.
- `internal/core/evidence/parser.go`: Implemented `ParseTranscript` and stream/dir reading capable of handling monolithic, chunked, or compact transcripts, timestamp extraction, step normalization to `harness.Step`, and checkpoint step parsing.
- `internal/core/evidence/builder.go`: Implemented `BuildEpisode` constructing valid `episode.Episode` instances with metadata, commit linkages, step gap detection, character stats, and parsed checkpoint sections.
- `internal/core/evidence/checkpoint_test.go` & `parser_test.go`: Added unit tests verifying checkpoint extraction across multiple markdown variations and transcript step/episode conversion.
- `internal/core/tick/pipeline.go`: Replaced `convertCapturesToTriage` with `convertCapturesToEvidence`. Capture jobs now parse transcripts and write zstd-compressed episodes directly into `episode.Store` and complete without queuing `KindTriage`. Updated `PipelineExecutor.Run` to call `convertCapturesToEvidence`. Added `NewPipelineExecutor` and `ConvertCapturesToEvidence`.
- `internal/core/tick/pipeline_test.go`: Added `TestConvertCapturesToEvidence` and `TestPipelineRun_ConvertCapturesToEvidence` validating capture processing, checkpoint extraction, and episode persistence.

### Decisions Made
- Implemented `internal/core/evidence/` within `internal/core` rather than importing `internal/harness/antigravity`, strictly complying with `internal/core/boundary_test.go` architectural constraints that prohibit core packages from importing harness adapters.
- Used `episode.Header` for storing extracted checkpoint sections (`map[string]string`) and evidence metadata (`Source`, `Status`, `StepCount`), enabling rapid header inspection without needing full decompression of stored episodes.
- Standardized canonical checkpoint titles (`User Knowledge`, `Model Knowledge`, `Work Accomplished`, `Current Work`, `Outstanding User Requests`, `Files and Code`) while mapping common synonyms (e.g. `Current Work and Next Steps` -> `Current Work`, `User Requests` -> `Outstanding User Requests`) and supporting custom markdown sections.

### Dead Ends & Warnings
- Pre-existing failure observed in `internal/core/jobs/store_test.go` (`TestJobLifecycleSuccess:308`) expecting `store.AcquireNext` to return `nil, nil` when no jobs are available rather than `ErrNoJobAvailable`. Left unmodified as it is outside this work order's boundary box.
- Pre-existing failures observed in `cmd/repo-memory` (`TestSearchCommand`) and `internal/harness/antigravity` (`TestSessionAttributor_Integration`).

### Out-of-Scope Findings
- `internal/core/jobs/store_test.go`: Line 308 checks `if err != nil` after `AcquireNext`, which returns `ErrNoJobAvailable` on empty queues. Should check `errors.Is(err, jobs.ErrNoJobAvailable)`.

### Blockers
None. Implementation validated and approved.
