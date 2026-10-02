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
- [x] Create `internal/core/spool/` package.
- [x] Define spool entry schema (JSON).
- [x] Implement a spool writer that writes to a unique temporary name in the spool directory and then atomically renames it (ensure Windows semantics are respected).
- [x] Implement a spool reader for the tick to ingest these entries.
- [x] Implement incremental transcript copy logic: store byte offset and prefix hash in `state/`. If the prefix changed, perform a full re-copy.

## Verification Requirement
You must verify it compiles and tests pass. `go build ./cmd/repo-memory` and `go test ./...`. Before committing, ask the user if they are happy with the changes (since I am the PA, you will ask me).

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `internal/core/spool/doc.go` — package documentation for harness-neutral spool management and incremental transcript copying.
- `internal/core/spool/entry.go` — defined spool entry schema (`Entry`, `EntryKind`) covering `capture`, `brief`, `dejavu`, and `recheck` events, along with constructor helpers and JSON normalization.
- `internal/core/spool/entry_test.go` — unit tests for JSON serialization, unmarshaling defaults, and constructor initialization.
- `internal/core/spool/writer.go` — implemented `Writer` with Windows-safe file rename semantics: writes to a unique temporary file (`.spool_tmp_*`) in the spool directory, syncs and closes file handles before rename, and performs atomic rename with retry backoff to sortable filenames without colons.
- `internal/core/spool/writer_test.go` — unit tests for atomic spool writing, temporary file cleanup, chronologically sortable unique naming, and directory auto-creation.
- `internal/core/spool/reader.go` — implemented `Reader` for tick ingestion: scans spool directory, ignores hidden and temporary files, parses entries in chronological order, provides `Acknowledge` removal, and transactional `Ingest` callback processing.
- `internal/core/spool/reader_test.go` — unit tests for directory scanning, noise filtering, chronological ordering, acknowledgements, and error-halting ingestion.
- `internal/core/spool/copy.go` — implemented `CopyFilesIncremental`, `CopyTranscriptIncremental`, and `ResolvePrimaryTranscript`: tracks byte offsets and prefix MD5 hashes in `state/cursor_<id>.json`, verifies append-only prefix integrity via streaming hash check, appends only new bytes, and triggers atomic full re-copy on prefix mismatch, truncation, or missing destination. Supports monolithic files, chunks, and compact fallbacks.
- `internal/core/spool/copy_test.go` — comprehensive unit and integration tests covering initial copy, incremental append, idempotent unchanged copies, prefix mismatch re-copy, truncation re-copy, missing destination re-copy, and chunked transcripts.

### Decisions Made
- Filename timestamps format: Formatted filenames as `20060102T150405.000000000Z_<kind>_<id>.json` to ensure chronological lexicographical ordering while avoiding colons (`:`) which are illegal in Windows filenames.
- Single-pass hashing: When performing prefix verification during incremental copying, bytes `0` to `state.ByteOffset` are hashed; if the prefix matches, subsequent appended bytes are piped through the same hasher via `io.TeeReader`, computing the complete new prefix hash in a single pass without re-reading the entire file.
- Crash-safe state writes: State cursors in `state/` are written using temporary files and atomic rename to prevent corrupted state during unexpected process termination.

### Dead Ends & Warnings
- When writing files on Windows, file handles must be closed before calling `os.Rename`. Attempting to rename an open handle results in `ERROR_ACCESS_DENIED`. The writer ensures handles are synced and closed before renaming, with a retry backoff loop for transient file indexer locks.

### Out-of-Scope Findings
- (none)

### Blockers
(none)
