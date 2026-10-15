# Work Order: wo5-dry-run-verification

> **Project:** pr-10b-delivery-and-storage-fixes · **WO:** 5 of 5
> **Branch:** `pr-10b-delivery-and-storage-fixes/wo5-dry-run-verification`
> **Base:** `pr-10b-delivery-and-storage-fixes/wo4-ops-cleanup-and-rebuild`
> **Complexity:** standard

## Objective

Add an end-to-end multi-invocation integration test (`TestPreInvocation_MultiInvocationLifecycle` in `internal/harness/antigravity/preinvocation_test.go`) verifying the complete v2.2 delivery lifecycle across `MomentTaskStart`, `MomentFirstTouch`, `MomentFirstEntry`, `MomentAboutToEdit`, mid-turn deduplication, `CHECKPOINT` compaction re-injection, and transcript rewind re-arming, and verify the full Go build, vet, and test suites across the repository.

## Context

In `PR-09`/`PR-09b` (`8d7ad7b` + `d3ab994`), persistent moments (`task-start`, `first-touch`, `first-entry`, and compaction re-injection) were routed through `EphemeralMessage` while `InjectedSet` suppressed re-injection for the rest of the session — causing injected entries to vanish after 1 tool call ("One-Invocation Vanishing Memory").
With WO1–WO4 complete:
- Persistent moments (`MomentTaskStart`, `MomentFirstTouch`, `MomentFirstEntry`, `BuildReinjection`) emit `UserMessage` (`ChannelUser`), which persists in `transcript_full.jsonl` until `CHECKPOINT` compaction.
- Ephemeral 1-step moments (`MomentAboutToEdit`, `MomentError`, FTS search warnings) emit `EphemeralMessage` (`ChannelEphemeral`).
- `DetectRewind` re-arms `InjectedSet`, `SeenAnchors`, and `SeenDossiers` when the transcript step count decreases.

This final Work Order proves the full multi-invocation lifecycle in an end-to-end integration test and verifies that all repository packages build, pass `go vet`, and pass `go test`.

## Files to Modify

| File | Action | Why |
|---|---|---|
| `internal/harness/antigravity/preinvocation_test.go` | Modify | Add `TestPreInvocation_MultiInvocationLifecycle` verifying multi-invocation `UserMessage` persistence, `InjectedSet` deduplication across tool calls, `EphemeralMessage` for `about-to-edit`, `CHECKPOINT` re-injection via `UserMessage`, and rewind re-arming. |

## Atomic Implementation Checklist

- [ ] **Step 1: Add `TestPreInvocation_MultiInvocationLifecycle` in `internal/harness/antigravity/preinvocation_test.go`**
  - Set up an enrolled workspace with `internal/auth/token.go` on disk and seed the index with:
    - `task-entry-1` (`DossierScope: "core/auth"`, `Type: dossiers.EntryTypeInvariant`, `Text: "Use HMAC-SHA256 for session tokens in auth module"`)
    - `touch-entry-1` (`DossierScope: "core/auth"`, `Type: dossiers.EntryTypeGotcha`, `Text: "Never mutate claims map in place inside internal/auth/token.go"`, `Anchors: []string{"internal/auth/token.go"}`)
    - `dossier-entry-1` (`DossierScope: "core/auth"`, `Type: dossiers.EntryTypeDecision`, `Text: "Always validate token expiry before signature verification in core/auth"`)
  - **Phase 1 (Turn 1, `invocationNum: 0`, `initialNumSteps: 1` — `MomentTaskStart`):**
    - Write initial `step_index: 0` (`USER_INPUT`) to `transcript_full.jsonl`.
    - Call `HandlePreInvocation` with `userPrompt: "Update session tokens in auth module"`.
    - Assert `resp.InjectSteps` emits a `UserMessage` (`EphemeralMessage == ""`) containing `[repo-memory task-entry-1]`.
    - Append that injected `UserMessage` to `transcript_full.jsonl` as a persistent `USER_INPUT` step (`step_index: 1`), followed by a `PLANNER_RESPONSE` (`step_index: 2`) calling `view_file` on `internal/auth/token.go`, a `TOOL_RESPONSE` (`step_index: 3`), and a `PLANNER_RESPONSE` (`step_index: 4`) with `thinking: "Now I will edit internal/auth/token.go to update token signing."`.
  - **Phase 2 (Turn 1, `invocationNum: 1`, `initialNumSteps: 5` — `MomentFirstTouch` + `MomentFirstEntry` + `MomentAboutToEdit`):**
    - Call `HandlePreInvocation` with `invocationNum: 1, initialNumSteps: 5`.
    - Assert:
      1. `task-entry-1` is **not** re-injected in `resp.InjectSteps` (suppressed by `InjectedSet`), while its `USER_INPUT` step remains present in `transcript_full.jsonl`.
      2. `touch-entry-1` and `dossier-entry-1` are injected via `UserMessage` (`EphemeralMessage == ""`).
      3. `MomentAboutToEdit` (`[repo-memory] Check injected entries above before editing token.go`) is injected via `EphemeralMessage` (`UserMessage == ""`).
    - Append the newly injected `UserMessage` steps (`step_index: 5`) and another `PLANNER_RESPONSE` (`step_index: 6`, `thinking: "Finished reviewing."`) to `transcript_full.jsonl`.
  - **Phase 3 (Turn 1, `invocationNum: 2`, `initialNumSteps: 7` — Mid-turn Deduplication):**
    - Call `HandlePreInvocation` with `invocationNum: 2, initialNumSteps: 7`.
    - Assert `string(out) == "{}"` (no duplicate injections), while all previously injected `UserMessage` steps remain in `transcript_full.jsonl`.
  - **Phase 4 (Turn 2, `invocationNum: 0`, `initialNumSteps: 9` after `CHECKPOINT` — Compaction Re-injection):**
    - Append `CHECKPOINT` and a summary `SYSTEM` step (`step_index: 8`) to `transcript_full.jsonl`.
    - Call `HandlePreInvocation` with `invocationNum: 0, initialNumSteps: 9`.
    - Assert `BuildReinjection` emits `UserMessage` (`EphemeralMessage == ""`) containing `task-entry-1`, `touch-entry-1`, and `dossier-entry-1` (and does **not** re-inject `about-to-edit:internal/auth/token.go`).
  - **Phase 5 (Turn 3, Transcript Rewind to 3 steps — `DetectRewind` Re-arms Dedup):**
    - Rewrite `transcript_full.jsonl` with only the first 3 steps (`step_index: 0..2`, ending at the `view_file` call on `internal/auth/token.go`).
    - Call `HandlePreInvocation` with `invocationNum: 1, initialNumSteps: 3`.
    - Assert `DetectRewind` re-arms `InjectedSet` / `SeenAnchors` / `SeenDossiers` and re-injects `touch-entry-1` and `dossier-entry-1` via `UserMessage`.
- [ ] **Step 2: Run full repository build, vet, test suite, and binary install**
  - Run `go build ./cmd/... ./internal/... ./evaluation/cmd/build-index/...`.
  - Run `go vet ./cmd/... ./internal/... ./evaluation/cmd/build-index/...`.
  - Run `go test ./cmd/... ./internal/...`.
  - Rebuild `~/.local/bin/repo-memory`: `go build -o ~/.local/bin/repo-memory ./cmd/repo-memory`.

## Verification Criteria

- [ ] `go test -v -run TestPreInvocation_MultiInvocationLifecycle ./internal/harness/antigravity/...` passes.
- [ ] `go test ./cmd/... ./internal/...` passes with 0 failures across all packages.
- [ ] `go vet ./cmd/... ./internal/... ./evaluation/cmd/build-index/...` passes with 0 warnings.
- [ ] `~/.local/bin/repo-memory` is built from the latest code.

## Anti-Patterns — Do NOT

- **Do NOT** modify any files outside `internal/harness/antigravity/preinvocation_test.go` (and `WORKORDER.md` upon completion).
- **Do NOT** include any internal paths or external hygiene violations in code or comments.

---

## Completion Report

*(To be filled in by the Worker after PA approval)*

### Status
(pending)

### Changes Made
- `path/to/file` — what changed and why

### Decisions Made
- Decision description and rationale

### Dead Ends & Warnings
- What was attempted, why it failed, and what to avoid

### Out-of-Scope Findings
- Finding description and suggested action
