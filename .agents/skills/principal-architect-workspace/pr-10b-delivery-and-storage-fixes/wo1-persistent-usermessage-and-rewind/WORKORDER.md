---
name: pr-10b-delivery-and-storage-fixes-wo1
description: Handoff instructions for Work Order 1 (Persistent UserMessage Delivery & Rewind Wiring)
complexity: standard
---

# WORKORDER 1: Persistent `UserMessage` Delivery & `DetectRewind` Wiring

> **Parent Project:** [pr-10b-delivery-and-storage-fixes](../PROJECT.md) (`PROGRAM-PLAN.md` v2.2 human approval recorded 2026-10-15)
> **Branch:** `pr-10b-delivery-and-storage-fixes/wo1-persistent-usermessage-and-rewind`
> **Base Branch:** `pr-10a-v2-2-docs-and-specs/wo5-verification-and-handover`

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Fix the `8d7ad7b` + `d3ab994` "One-Invocation Vanishing Memory" regression by switching persistent delivery moments (`MomentTaskStart`, `MomentFirstTouch`, `MomentFirstEntry`, and `BuildReinjection`) from `EphemeralMessage` to `UserMessage` while keeping 1-step transient moments (`about-to-edit`, `error-match`, and FTS search warnings) on `EphemeralMessage`, wire `DetectRewind` into `HandlePreInvocation` before moment evaluation, and make `hooks_test.go` hermetic with respect to git user configuration.

## Strict Boundary Box
- **ONLY touch:**
  - `internal/core/delivery/types.go`
  - `internal/core/delivery/ledger.go`
  - `internal/core/delivery/delivery_test.go`
  - `internal/harness/antigravity/preinvocation.go`
  - `internal/harness/antigravity/compaction.go`
  - `internal/harness/antigravity/preinvocation_test.go`
  - `internal/harness/antigravity/compaction_test.go`
  - `internal/harness/antigravity/hooks_test.go`
  - `.agents/skills/principal-architect-workspace/pr-10b-delivery-and-storage-fixes/wo1-persistent-usermessage-and-rewind/WORKORDER.md`
- **DO NOT touch:**
  - `internal/core/index/` (reserved for WO2)
  - `internal/core/dossiers/` (reserved for WO2)
  - `internal/core/seed/` (reserved for WO3)
  - `.memory/` (reserved for WO4)
- **External GitHub Hygiene:** Never include any internal references or absolute OS paths in code, comments, or commit messages. Use git author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.

## Tooling Constraints
- Run tests and builds via `run_command` using standard `go test ./internal/core/delivery/... ./internal/harness/antigravity/...` and `go vet ./...`.

## Line-Item Instructions

- [ ] `internal/core/delivery/types.go`:
  - Add `ChannelUser Channel = "userMessage"` alongside `ChannelSystem`, `ChannelEphemeral`, and `ChannelDeny`.

- [ ] `internal/core/delivery/ledger.go`:
  - Add `LastKnownStepCount int `json:"last_known_step_count,omitempty"`` to `Ledger`.
  - In `RearmDedup()`, clear existing keys from `l.SeenDossiers`, `l.SeenAnchors`, and `l.InjectedSet` (or reinitialize them cleanly, and ensure any `Engine` referencing `l.InjectedSet` sees the cleared map—for example by deleting keys in-place `clear(l.InjectedSet)` before/instead of replacing the map header, so an `Engine` holding the map reference stays synchronized).

- [ ] `internal/harness/antigravity/compaction.go`:
  - In `BuildReinjection`, return `&InjectStep{UserMessage: formatted}, nil` instead of `&InjectStep{EphemeralMessage: formatted}, nil` so post-compaction re-injected entries persist across turns until the next `CHECKPOINT`.

- [ ] `internal/harness/antigravity/preinvocation.go`:
  - After parsing `steps` from `input.TranscriptPath` (around line 125) and **before** scanning `steps` against `hwm := ledger.LastProcessedStepIndex`:
    - Check `if DetectRewind(steps, ledger.LastKnownStepCount)`:
      - Call `ledger.RearmDedup()`
      - Reset `ledger.LastProcessedStepIndex = 0`
      - Prune any `ledger.Injections` whose `StepIndex` is greater than the highest remaining `StepIndex` in `steps` (or `0` if `steps` is empty), updating `injectionBase = len(ledger.Injections)` and setting `rearmed = true` (and `rewound := true`).
    - Set `ledger.LastKnownStepCount = len(steps)`.
  - Ensure `engine := delivery.NewEngine(idx, workspaceRoot, delivery.WithInjectedSet(ledger.InjectedSet))` uses the post-rewind / post-compaction `ledger.InjectedSet` (either by constructing `engine` after the rewind and compaction blocks or by clearing `ledger.InjectedSet` in-place in `RearmDedup()`).
  - Switch `MomentTaskStart` (when `err == nil && len(entries) > 0`, around lines 241–254) from `EphemeralMessage: formatted` / `Channel: delivery.ChannelEphemeral` to `UserMessage: formatted` / `Channel: delivery.ChannelUser`. **Keep** the FTS search error branch (`if err != nil`) on `EphemeralMessage`.
  - Switch `MomentFirstTouch` (around lines 273–283) from `EphemeralMessage: formatted` / `Channel: delivery.ChannelEphemeral` to `UserMessage: formatted` / `Channel: delivery.ChannelUser`.
  - Switch `MomentFirstEntry` (around lines 299–309) from `EphemeralMessage: formatted` / `Channel: delivery.ChannelEphemeral` to `UserMessage: formatted` / `Channel: delivery.ChannelUser`.
  - **Keep** `MomentAboutToEdit` (around lines 352–360) on `EphemeralMessage` and `Channel: delivery.ChannelEphemeral`.
  - In the `ModifyLedger` callback (around lines 370–397):
    - When `rearmed` (or `rewound`) is true, if `rewound`, also prune `l.Injections` down to the surviving prefix before replaying `ledger.Injections[injectionBase:]`, and set `l.LastProcessedStepIndex = ledger.LastProcessedStepIndex` unconditionally (since on a rewind the new step index is smaller than the old high-water mark).
    - Persist `l.LastKnownStepCount = ledger.LastKnownStepCount`.

- [ ] `internal/harness/antigravity/hooks_test.go`:
  - In `runGitInDir` (line 17), prepend `-c`, `user.name=test`, `-c`, `user.email=test@example.com` to the `git` command arguments so `git commit` inside `setupEnrolledRepo` (and `TestStopHook_CompletePipeline`) succeeds hermetically even in environments without global git user config.

- [ ] `internal/harness/antigravity/compaction_test.go`, `internal/harness/antigravity/preinvocation_test.go`, and `internal/core/delivery/delivery_test.go`:
  - Update `TestBuildReinjection` in `compaction_test.go` to assert `step.UserMessage` contains `"Some test data"` and `step.EphemeralMessage == ""`.
  - Update `TestPreInvocation_TaskStart_And_Empty`, `TestPreInvocation_AboutToEdit`, `TestPreInvocation_CompactionAndReinjection`, and `TestPreInvocation_AntigravityNativeToolCalls` in `preinvocation_test.go` so that:
    - `task-start`, `first-touch`, `first-entry`, and compaction `re-injection` assert `step.UserMessage` (and `step.EphemeralMessage == ""`).
    - `about-to-edit` asserts `step.EphemeralMessage == "[repo-memory] Check injected entries above before editing ..."` (and `step.UserMessage == ""`).
  - Add a new test `TestPreInvocation_RewindRearmsDedup` in `preinvocation_test.go` verifying that:
    1. On an initial invocation with a 4-step transcript touching `service.go`, `HandlePreInvocation` injects the entry as `UserMessage` and records `LastKnownStepCount == 4` and `InjectedSet["native-entry-1"] == true`.
    2. On a subsequent invocation with the same 4-step transcript, `HandlePreInvocation` returns `{}` (deduplicated by `InjectedSet` / `SeenAnchors`).
    3. When the transcript rewinds to 2 steps (`len(steps) < ledger.LastKnownStepCount`), `HandlePreInvocation` detects the rewind via `DetectRewind`, resets `InjectedSet` / `SeenAnchors` / `LastProcessedStepIndex`, updates `LastKnownStepCount == 2`, and re-injects the entry via `UserMessage` when `service.go` is touched.

## Verification Requirement
Run `go test -v ./internal/core/delivery/... ./internal/harness/antigravity/...` and `go vet ./...` to confirm all tests pass. Once finished, **STOP and send a message back to the Principal Architect** confirming the implementation is validated. Once approved, append your Completion Report to this file, commit all changes on branch `pr-10b-delivery-and-storage-fixes/wo1-persistent-usermessage-and-rewind`, and terminate your session.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->
