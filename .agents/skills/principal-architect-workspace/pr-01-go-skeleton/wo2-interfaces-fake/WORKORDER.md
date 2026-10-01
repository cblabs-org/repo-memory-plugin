---
name: pr-01-go-skeleton-wo2
description: Handoff instructions for Work Order 2 - Interfaces & Fake
complexity: standard
---

# WORKORDER: WO2 - Interfaces & Fake

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Define the core harness interfaces (`Harness`, `Capabilities`, `TranscriptSource`, `HookCodec`, `Reporter`, `Housekeeper`), normalized types (`Event`, `Injection`, `Step`), and the `Stage` enum. Implement the `fake` test adapter with basic in-memory state and verify it with a simple test suite shell in `internal/core`.

## Strict Boundary Box
- DO NOT implement the Antigravity or Claude Code adapters. ONLY implement the `fake` adapter.
- DO NOT implement pipeline logic or actual index/database systems.
- DO NOT modify the `internal/core/boundary_test.go` added in WO1. *(Authorized exception: PA approved allowing `*_test.go` to import `internal/harness/fake`)*

## Tooling Constraints
- Standard Go tooling: `make build`, `go test`.

## Line-Item Instructions
- [x] In `internal/harness`, define normalized types `Event`, `Injection`, `Step`, and the `Stage` enum.
- [x] In `internal/harness`, define the core interfaces:
  - `Harness` (methods: `Name() string`, `Capabilities() Capabilities`, `Transcripts() TranscriptSource`, `Hooks() HookCodec`, `Reporter() Reporter`)
  - `Capabilities` (a struct describing harness features like `InjectBeforeModelCall bool`, etc.)
  - `TranscriptSource` (discover/parse transcripts -> `[]Step`)
  - `HookCodec` (native hook JSON <-> normalized `Event`/`Injection`)
  - `Reporter` (start and poll IDE-visible report)
  - `Housekeeper` (run a pipeline stage `(ctx context.Context, stage Stage) (Result, error)`)
- [x] In `internal/harness/fake`, implement `FakeHarness` (and the related fake implementations for the other interfaces) using basic in-memory state (e.g. structs with slices or maps to mock transcripts and hook codec returns).
- [x] In `internal/core`, create a core test-suite shell (e.g., `suite_test.go`) that initializes the `fake` adapter and runs a trivial assertion (e.g., verifying `fake.Name()` returns the expected mock string).

## Verification Requirement
- Verify `make build` compiles correctly.
- Verify `go test ./...` passes.
- **CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report

### Status
complete

### Changes Made
- `internal/harness/types.go` — Defined normalized types `Event`, `EventType`, `Injection`, `Decision`, `Step`, `StepKind`, `Stage` enum with stringer, `Capabilities`, `Usage`, `Result`, and `ReportStatus`.
- `internal/harness/interfaces.go` — Defined core interfaces `Harness`, `TranscriptSource`, `HookCodec`, `Reporter`, and `Housekeeper`.
- `internal/harness/fake/fake.go` — Implemented in-memory `FakeHarness`, `FakeTranscriptSource`, `FakeHookCodec`, `FakeReporter`, and `FakeHousekeeper`.
- `internal/harness/fake/fake_test.go` — Unit tests exercising all fake implementations and assertions.
- `internal/core/suite_test.go` — Core test suite shell initializing the fake harness and asserting adapter behavior.
- `internal/core/boundary_test.go` — Updated with PA authorization to permit `*_test.go` files to import `internal/harness/fake` while strictly maintaining adapter prohibitions for production code and other harnesses.

### Decisions Made
- Allowed test files (`*_test.go`) under `internal/core` to import `internal/harness/fake` following approval from the Principal Architect to comply with SPEC §18.6 Rule 6 ("Core tests run against the fake adapter").

### Dead Ends & Warnings
- Running `go test ./...` with `suite_test.go` initially triggered a boundary failure in `TestCoreImportBoundaries` because `boundary_test.go` strictly forbade `internal/harness/fake` in all `.go` files under `internal/core`. Surfaced to PA, authorized adjustment, and resolved cleanly.

### Out-of-Scope Findings
(none)

### Blockers
(none)
