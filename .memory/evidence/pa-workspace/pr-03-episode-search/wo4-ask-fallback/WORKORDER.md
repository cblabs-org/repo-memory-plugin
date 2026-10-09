---
name: pr-03-episode-search-wo4
description: Handoff instructions for Work Order 4
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement the `memory.ask` MCP tool fallback logic. When no knowledge claims exist yet (because the pipeline isn't built yet), it should fall back to a full-text search over the episode index and return the results marked as "unverified (episode search)".

### Strict Boundary Box
Only modify MCP server logic, specifically the handler for `memory.ask`. Do not modify the underlying FTS5 index logic.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [x] Locate the `memory.ask` tool handler in the MCP server implementation (likely in `internal/mcpserver/` or similar). If the MCP server doesn't fully exist yet, create the skeleton for the `memory.ask` tool.
- [x] In the handler, check if there are claims to return (currently there are none). If none, invoke the FTS search query over episodes using the index from WO1/WO2.
- [x] Format the fallback results, explicitly marking them with the phrase "unverified (episode search)" to make it clear this is raw episode text, not verified pipeline claims.
- [x] Write/update unit tests to verify the fallback logic triggers and formats the text correctly.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints (`go build ./cmd/repo-memory` and `go test ./...`). **CRITICAL:** Commit your changes to the branch after verifying tests pass, as requested by the PA SYSTEM_MESSAGE.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `internal/mcpserver/server.go` — created the MCP server skeleton and `HandleMemoryAsk` fallback logic that performs full-text search against the episode index when no claims are found.
- `internal/mcpserver/server_test.go` — added a test to ensure `memory.ask` gracefully falls back to the episode index and formats results with "unverified (episode search)".

### Blockers
(none)
