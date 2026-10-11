---
name: pr-08-v2-delivery-wo6
description: Handoff instructions for Work Order 6 — MCP Pull Tools
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Add 5 MCP server tools to the existing `internal/mcpserver/` package, replacing the single `memory.ask` stub. These tools enable the agent to pull deeper context on demand (the "pull for depth" channel from §8.2):

1. `memory.brief` — Dossier summary and invariants for a scope
2. `memory.why` — Rationale, evidence, and history for an entry
3. `memory.history` — Retired and superseded entries
4. `memory.search` — Full-text BM25 search over entries
5. `memory.note` — Agent-prompted memory capture (write to inbox)

Each pull is recorded in the exposure list via `ledger.RecordPull()`.

# Context

## Existing MCP Server (`internal/mcpserver/`)
- `server.go` — `Server` struct holding `*index.Index`. Has `HandleMemoryAsk` (stub). Uses stdio JSON-RPC.
- `server_test.go` — Tests for the stub handler.
- The server is registered in the plugin manifest and exposed via the hooks infrastructure.

## Dependencies
- `internal/core/index/` — `SearchEntries(ctx, query)`, `GetEntry(ctx, id)`, `ListEntriesByScope(ctx, scope)`, `GetHistory(ctx, entryID)`, `ResolveAlias(ctx, scope)`, `LookupByAnchor(ctx, anchor)`
- `internal/core/dossiers/` — `Entry` struct
- `internal/core/delivery/ledger.go` — `RecordPull(entryID)`

# Strict Boundary Box

**DO NOT:**
- Modify `internal/core/delivery/`, `internal/core/ops/`, `internal/core/dossiers/`, `internal/core/index/`
- Modify any harness hook handler
- Create CLI subcommands

**DO:**
- Modify `internal/mcpserver/server.go` — add tool handlers
- Create `internal/mcpserver/tools.go` — tool implementations
- Create/update tests

# Tooling Constraints

- Use `go build ./...` to verify compilation
- Use `go test ./internal/mcpserver/... -v` to run tests
- Use `go vet ./...` for static analysis

# Line-Item Instructions

## 1. Tool Definitions (`internal/mcpserver/tools.go`)

- [ ] `memory.brief` — `HandleMemoryBrief(ctx, req MemoryBriefRequest) (MemoryBriefResponse, error)`
  - Request: `{ "scope": "string" }` — dossier scope name
  - Resolve aliases via `index.ResolveAlias(ctx, scope)` if the scope isn't found directly
  - Fetch entries: `index.ListEntriesByScope(ctx, resolvedScope)`
  - Filter to active (non-retired) entries
  - Format: scope name, entry count, then each entry as `[<type>] <id>: <text>` (truncated to 200 chars if needed)
  - Response: `{ "result": "string" }` — formatted brief

- [ ] `memory.why` — `HandleMemoryWhy(ctx, req MemoryWhyRequest) (MemoryWhyResponse, error)`
  - Request: `{ "entryId": "string" }` — entry ID
  - Fetch entry: `index.GetEntry(ctx, entryId)`
  - Fetch history: `index.GetHistory(ctx, entryId)`
  - Format: entry type, text, anchors, evidence list, verification status, created/updated timestamps, history (superseded by, retired reason)
  - Response: `{ "result": "string" }`

- [ ] `memory.history` — `HandleMemoryHistory(ctx, req MemoryHistoryRequest) (MemoryHistoryResponse, error)`
  - Request: `{ "scope": "string" }` — dossier scope
  - Fetch all entries by scope (including retired)
  - Filter to retired/superseded entries only
  - Format: entry ID, type, text, retired reason or superseded-by
  - Response: `{ "result": "string" }`

- [ ] `memory.search` — `HandleMemorySearch(ctx, req MemorySearchRequest) (MemorySearchResponse, error)`
  - Request: `{ "query": "string" }` — search terms
  - Call `index.SearchEntries(ctx, query)` (FTS5 BM25)
  - Limit to top 10 results
  - Format: ranked list with entry ID, type, scope, and text snippet (first 100 chars)
  - Response: `{ "result": "string" }`

- [ ] `memory.note` — `HandleMemoryNote(ctx, req MemoryNoteRequest) (MemoryNoteResponse, error)`
  - Request: `{ "scope": "string", "text": "string", "type": "string" }` — type defaults to "decision" if not specified
  - Validate: scope and text are required; type must be a valid `dossiers.EntryType`
  - Write a new op to the op log: `ops.OpAdd` with `source: "agent"`, current timestamp
  - NOTE: This requires writing to the ops directory. Import `internal/core/ops` Writer.
  - Response: `{ "result": "Note recorded: <entryID>" }`

## 2. Update Server Registration (`internal/mcpserver/server.go`)

- [ ] Replace or deprecate `HandleMemoryAsk` with the new tool handlers
- [ ] Register tools with the MCP server framework (follow existing registration pattern in `server.go`)
- [ ] Each handler should accept a `context.Context` and a typed request struct

## 3. Request/Response Types (`internal/mcpserver/tools.go`)

- [ ] Define request/response structs for each tool:
  ```go
  type MemoryBriefRequest struct { Scope string `json:"scope"` }
  type MemoryBriefResponse struct { Result string `json:"result"` }
  // ... similar for each tool
  ```

## 4. Tests (`internal/mcpserver/tools_test.go`)

- [ ] **memory.brief:** Set up index with entries in a known scope → verify formatted output
- [ ] **memory.why:** Set up entry with anchors, evidence, history → verify complete output
- [ ] **memory.search:** Set up entries with searchable text → verify FTS results
- [ ] **memory.note:** Verify note creates an op (or returns the entry ID)
- [ ] **memory.history:** Set up retired entries → verify filtered output
- [ ] **Alias resolution:** Set up alias → verify brief resolves correctly

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./internal/mcpserver/... -v` — all tests pass
3. Run `go vet ./...` — must pass

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below.

---

## Completion Report (written by the worker at task completion)

### Status
Complete. All 5 MCP server tools (`memory.brief`, `memory.why`, `memory.history`, `memory.search`, `memory.note`) implemented, integrated with ledger pull tracking, and verified with 88.5% statement test coverage.

### Changes Made
- `internal/mcpserver/server.go`: Added `ledger *delivery.Ledger` and `opsWriter *ops.Writer` fields to `Server`, configured functional options (`WithLedger`, `WithOpsWriter`, `WithOpsDir`) and setters (`SetLedger`, `SetOpsWriter`), defined `SupportedTools`, and marked `HandleMemoryAsk` as deprecated.
- `internal/mcpserver/tools.go`: Implemented typed request/response structs and handlers:
  - `HandleMemoryBrief`: Active entry listing by scope, alias resolution via `idx.ResolveAlias`, 200-rune truncation, and ledger exposure tracking.
  - `HandleMemoryWhy`: Complete entry details including type, scope, text, anchors, evidence/source, verification status, timestamps, and history records; records pull to ledger.
  - `HandleMemoryHistory`: Fetches retired/superseded entries across `entries` and `entry_history` with alias resolution; formats type, text, retired reason, and superseded-by; records pulls to ledger.
  - `HandleMemorySearch`: FTS5 BM25 search via `idx.SearchEntries`, caps results to top 10, formats ranked list with 100-char snippet, and records pulls to ledger.
  - `HandleMemoryNote`: Validates required fields and `dossiers.EntryType` (defaults to "decision"), constructs `ops.OpAdd` with `source: "agent"`, writes batch to ops log via `ops.Writer`, and updates the in-memory index for immediate availability.
- `internal/mcpserver/tools_test.go`: Added test suite covering all 5 tools, alias resolution, text truncation, ledger pull exposure recording, server options/setters, and error validation edge cases.

### Decisions Made
- In `HandleMemoryHistory`, combined queries from `ListEntriesByScope` (for entries marked retired/superseded), `GetHistory` for each entry ID, and direct query of `entry_history` via `WriteTx` with deduplication to ensure retired entries are reliably discovered regardless of whether the index was populated via ops replay or direct insertion.
- In `HandleMemoryWhy`, supported both `entry.Evidence` slice and `entry.Source` fallback to guarantee evidence provenance is printed.
- In `HandleMemoryNote`, wrote the op batch to disk via `ops.Writer` and additionally upserted the entry into `idx` (when present) so subsequent queries in the same process can immediately access newly captured notes.

### Dead Ends & Warnings
- SQLite `entries` table schema does not store a `retired` column (retired entries reside in `entry_history`). Therefore, in `HandleMemoryBrief`, entries retrieved from `ListEntriesByScope` are cross-checked against `GetHistory(ctx, e.ID)` to ensure retired entries are filtered out.

### Out-of-Scope Findings
- `internal/harness/antigravity/attribution_test.go` (`TestSessionAttributor_Integration`) failed due to an idle time assumption (`expected c1 to be idle (modified 1h ago)`). This is an existing timing test failure outside the `internal/mcpserver` boundary box.

### Blockers
None.
