---
name: pr-02b-capture-runtime-wo5
description: Handoff instructions for Work Order 5
complexity: standard
---

# pr-02b-capture-runtime / WO5 - session-attribution

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Map conversations to enrolled repos and handle duplicate deduplication.

## Strict Boundary Box
- Do not change tick core logic beyond session attribution.

## Tooling Constraints
Use `go build` and `go test` to verify changes.

## Line-Item Instructions
- [x] Map conversations to enrolled repos via `workspace_uris` in `conversation_summaries.db` (app and CLI).
- [x] De-duplicate conversations present in both stores.
- [x] Link subagents to parents via `parent_conversation_id`.

## Verification Requirement
You must verify it compiles and tests pass. `go build ./cmd/repo-memory` and `go test ./...`. Before committing, ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, surface it in your Completion Report below.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
Complete. Approved by parent agent (13c34b9d-f833-4a86-9a97-58f7cc15a5d9).

### Changes Made
1. **Antigravity Session Attribution Component** (`internal/harness/antigravity/attribution.go`):
   - Implemented `ParseWorkspaceURI` normalizing URI schemas (`file:///`, percent-encoding, Windows drive paths).
   - Implemented `LoadSummariesFromDB` reading SQLite database (`modernc.org/sqlite`) for both IDE app and CLI `conversation_summaries.db` instances. Gracefully handles non-existent databases.
   - Implemented `DeduplicateSummaries` resolving records present in both stores (latest modified time, maximum step count, non-empty workspace URIs and parent IDs merged).
   - Implemented `LinkSubagents` building parent-to-child links (`SubagentIDs`).
   - Implemented `AttributeConversations` matching workspace paths against enrolled repos and performing multi-pass tree inheritance for subagents with empty workspace paths.
   - Implemented `SessionAttributor` implementing `tick.SessionAttributor`.
   - Implemented `IsConversationIdle` adhering to the 15-minute idle rule in SPEC §6.2.
   - Implemented `LookupConversationSummary` for fast single-conversation metadata queries.

2. **Core Tick Sweep & Attribution Interfaces** (`internal/core/tick/`):
   - Defined `AttributedSession` and `SessionAttributor` interface in `internal/core/tick/sweep.go`.
   - Replaced old stub `extractWorkspaceFromTranscript` and directory walk in `internal/core/tick/sweep.go` with delegation to `SessionAttributor`.
   - Enriched capture job payload with `parent_conversation_id`.
   - Added `TickerOption` support (`WithAttributor`) to `internal/core/tick/tick.go`.

3. **Spool Entry & Hooks Integration** (`internal/core/spool/entry.go`, `internal/harness/antigravity/hooks.go`):
   - Added `ParentConversationID` to `spool.Entry` with backward-compatible JSON marshaling.
   - Enhanced `StopPayload` and `HookOptions` in `hooks.go` to capture and preserve `ParentConversationID` during Stop hook processing.

4. **CLI Wiring** (`cmd/repo-memory/cmd_tick.go`):
   - Registered and wired `antigravity.NewSessionAttributor()` to `tick.NewTicker` in `cmd/repo-memory/cmd_tick.go`.

5. **Tests** (`internal/harness/antigravity/attribution_test.go`, `internal/core/tick/sweep_test.go`):
   - Added comprehensive unit and integration tests covering URI parsing, DB queries, de-duplication, subagent linking, repository attribution, idle checks, and tick sweep.

### Decisions Made
- Maintained strict core/adapter boundary boundaries per SPEC §18.2: `internal/core/tick` defines the `SessionAttributor` interface and does not import concrete adapters (`boundary_test.go` passed 100% with 0 violations across 61 files).
- Subagents with empty or missing `workspace_uris` recursively inherit their enrolled repository attribution from their parent conversation up the hierarchy tree.
- When de-duplicating between app and CLI databases, the record with the latest timestamp is preferred for status/activity, while preserving non-empty workspace paths and parent IDs from either store.

### Dead Ends & Warnings
- Direct imports of concrete harness adapters from `internal/core` are forbidden by `boundary_test.go`. Interface injection via `SessionAttributor` avoided boundary violations while allowing full concrete functionality.

### Out-of-Scope Findings
- None.

### Blockers
- None.
