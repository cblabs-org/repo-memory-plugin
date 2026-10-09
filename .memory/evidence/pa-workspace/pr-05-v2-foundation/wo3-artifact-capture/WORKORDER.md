---
name: pr-05-v2-foundation-wo3
description: Handoff instructions for Work Order 3 — artifact capture and snapshot command
complexity: standard
---

# WO3: Artifact Capture & Snapshot Command

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Antigravity prunes conversations to ~500 (~19 days). Artifacts (markdown files in `brain/<id>/`) contain structured summaries, analysis, and plans that are valuable evidence for v2. Add:
1. **Artifact capture** at `Stop` and during the sweep — copy conversation artifacts into the evidence store
2. A **`snapshot` CLI command** that copies named conversations' transcripts and artifacts immediately, for preserving X6 candidates before pruning

## Strict Boundary Box
- **Modify:** `internal/harness/antigravity/hooks.go` (add artifact copy at `Stop`)
- **Modify:** `internal/core/tick/sweep.go` (add artifact copy during sweep)
- **Modify:** `internal/core/config/layout.go` (add `evidence/artifacts` subdirectory)
- **Create:** `internal/core/evidence/artifacts.go` (artifact copy logic)
- **Create:** `cmd/repo-memory/cmd_snapshot.go` (new CLI command)
- **Modify:** `cmd/repo-memory/main.go` (register the snapshot command)
- **DO NOT** modify the pipeline, episode store, spool, entities, claims, index, delivery, or MCP server
- **DO NOT** run any LLM calls

## Tooling Constraints
- Use `go build ./...` and `go vet ./...` to verify
- Run `go test ./internal/core/evidence/... ./internal/core/tick/...`

## Context

### Where artifacts live
Antigravity stores conversation artifacts at:
`~/.gemini/antigravity/brain/<conversation-id>/`

Files in that directory include `*.md` artifacts and `.metadata.json`. The `.system_generated/` subdirectory contains logs (transcripts) — we already capture transcripts via the spool; this WO is about the other files.

### The Stop payload
The `StopPayload` in `hooks.go` has `ArtifactDirectoryPath string` which points to the brain directory for the current conversation. This is the source for artifact capture at hook time.

### Where to store artifacts
Copy artifacts to: `<data-dir>/<repo-id>/evidence/artifacts/<conversation-id>/`
- Copy all `.md` files and `.metadata.json`
- Skip `.system_generated/` (already handled by transcript capture)
- Skip files > 1 MB (safety limit)
- Idempotent: if the destination file exists with the same size, skip it

### The snapshot command
`repo-memory snapshot <conversation-id> [<conversation-id>...]`
- For each conversation ID, copy both the transcript and artifacts into the evidence store
- Transcript: use the existing `spool.CopyTranscript` or reuse the logic from `hooks.go` that resolves transcript paths
- Artifacts: use the new artifact copy function
- Print what was copied
- Exit 0 if all succeeded, exit 1 if any failed

## Line-Item Instructions

- [x] **Add `filepath.Join("evidence", "artifacts")` to `DataDirSubdirectories`** in `internal/core/config/layout.go`.

- [x] **Create `internal/core/evidence/artifacts.go`** with:
  - `CopyArtifacts(srcDir, dstDir string) (int, error)` — copies `.md` files and `.metadata.json` from srcDir to dstDir, skipping `.system_generated/` and files > 1 MB. Returns count of files copied. Idempotent (skip if dest exists with same size).
  - `ArtifactDir(dataDir, repoID, conversationID string) string` — returns the destination directory path.

- [x] **Add artifact capture to `HandleStopEvent`** in `internal/harness/antigravity/hooks.go`:
  - After the existing transcript capture logic, if `payload.ArtifactDirectoryPath` is non-empty and the repo is enrolled, call `evidence.CopyArtifacts` to copy artifacts to the evidence store.
  - Log errors but don't fail the hook (artifact capture is best-effort).

- [x] **Add artifact capture to the sweep** in `internal/core/tick/sweep.go`:
  - During the sweep of idle conversations, also copy artifacts from the brain directory.
  - The sweep already knows the conversation ID and can derive the brain directory path.

- [x] **Create `cmd/repo-memory/cmd_snapshot.go`**:
  - Register `snapshot` subcommand
  - Accept one or more conversation IDs as arguments
  - For each: resolve the brain directory, copy transcript (using existing spool logic or direct file copy), copy artifacts (using `evidence.CopyArtifacts`)
  - Print summary of what was copied
  - The command needs the repo to be enrolled (needs `repoID` from the registry)

- [x] **Register the command** in `cmd/repo-memory/main.go`.

- [x] **Add a test** for `CopyArtifacts` in `internal/core/evidence/artifacts_test.go`: create a temp dir with sample `.md` files, `.metadata.json`, and a `.system_generated/` subdir, verify only the right files are copied.

- [x] **Verify:** `go build ./...`, `go vet ./...`, and relevant tests pass.

## Verification Requirement
Run `go build ./...`, `go vet ./...`, and `go test ./internal/core/evidence/...`. Commit when clean.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

### Status
Complete. Implementation verified via unit tests, integration tests, and build/vet checks.

### Changes Made
- `internal/core/config/layout.go`: Added `filepath.Join("evidence", "artifacts")` to `DataDirSubdirectories`.
- `internal/core/evidence/artifacts.go`: Implemented `CopyArtifacts` and `ArtifactDir` with 1 MB limit, idempotency check, atomic writes, and skipping `.system_generated/`.
- `internal/core/evidence/artifacts_test.go`: Added tests for `ArtifactDir` and `CopyArtifacts` verifying size constraints, idempotency, updates, and `.system_generated` exclusion.
- `internal/harness/antigravity/hooks.go`: Added artifact capture step to `HandleStopEvent` when `payload.ArtifactDirectoryPath` is present; added snake_case normalization and tilde expansion.
- `internal/harness/antigravity/hooks_test.go`: Added `TestStopHook_ArtifactCapture`.
- `internal/core/tick/sweep.go`: Added artifact capture during sweep of idle sessions with `deriveBrainDir`.
- `internal/core/tick/sweep_test.go`: Added `TestSweep_ArtifactCapture`.
- `cmd/repo-memory/cmd_snapshot.go`: Created `snapshot` subcommand to snapshot transcripts and artifacts for one or more conversation IDs.
- `cmd/repo-memory/cmd_snapshot_test.go`: Added CLI test suite for `snapshot` subcommand.
- `cmd/repo-memory/main.go`: Registered `snapshotCmd` with `rootCmd`.

### Decisions Made
- Used atomic temp-file write and rename in `CopyArtifacts` to avoid partial reads or corrupt writes.
- In `snapshot` command, `resolveBrainDir` accepts both conversation IDs and direct directory paths (extracting the basename as conversation ID), making local testing and explicit path inputs seamless.
- Handled both camelCase (`artifactDirectoryPath`) and snake_case (`artifact_directory_path`) in `StopPayload.Normalize()`.

### Dead Ends & Warnings
- None encountered within the boundary box.

### Out-of-Scope Findings
1. `.gitignore` line 6 contains `repo-memory` without a leading slash (`/repo-memory`), causing git to ignore any directory or file with `repo-memory` in the path (including `cmd/repo-memory/cmd_snapshot.go`). Changed files were staged via `git add -f`. Recommend updating `.gitignore` line 6 to `/repo-memory`.
2. `attribution.go:572` in `IsConversationIdle`: `now.Sub(fi.ModTime()) < idleCutoff` returns true whenever `now` is older than `fi.ModTime()` because negative durations are `< 15m`. If `now` is mocked in the past (e.g., in `TestSessionAttributor_Integration`), freshly touched files appear non-idle.

### Blockers
None.
