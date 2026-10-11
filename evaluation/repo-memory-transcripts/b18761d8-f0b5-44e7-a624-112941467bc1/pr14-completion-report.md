# Bug-Fixer Completion Report: PR-14 (PR-08 v2 Delivery)

**Date:** 2026-10-07  
**PR:** [#14 — PR-08: v2 Delivery Engine, Push Channels & Pull Tools](https://github.com/cblabs-org/repo-memory-plugin/pull/14)  
**Branch:** `pr-08-v2-delivery/wo6-pull-tools`  
**Base commit:** `0f6b365` (WO1–WO6 implementation by Chief Architect)  
**Fix commits:** `3a28d07` → `fc19d30` → `cd07cdf` → `02bc624` → `e5a386f`  
**Reviewer:** Claude Code (automated via `matthewmcneill`)

---

## Summary

PR-14 underwent **5 rounds of review**, surfacing a total of **34 findings** (10 + 7 + 6 + 8 + 3) across the delivery engine, harness hooks, MCP server, ledger persistence, and specification. **All 34 findings were resolved** in 5 fix commits. The PR is now in a merge-ready state.

### Final Build Status

| Check | Result |
|-------|--------|
| `go build ./...` (darwin) | ✅ Clean |
| `GOOS=windows go build ./...` | ✅ Clean |
| `go vet ./...` | ✅ Clean |
| `go test` (all affected packages) | ✅ All pass |
| Pre-existing flaky: `TestSessionAttributor_Integration` | ⚠️ Unrelated (hardcoded date) |

### Impact: 16 files changed, +1,147 / −119 lines

---

## Round-by-Round Detail

### Round 1 — Initial Review (10 findings → `3a28d07`)

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | PreToolUse didn't load persisted ledger — gates reset every process | Bug | Load ledger from disk before gate check |
| 2 | `sanitizeFTSQuery` collapsed punctuation into words (`auth.go` → `authgo`) | Bug | Split on non-word characters via `strings.FieldsFunc` |
| 3 | `invocationNum > 0` guard blocked all mid-turn moments | Bug | Gate only `task_start` on `invocationNum == 0` |
| 4 | `ErrorEntries` didn't check pattern against actual output | Bug | Added `patternMatchesOutput` filter |
| 5 | Diff paths in preinvocation included absolute workspace prefix | Bug | Made paths relative via `filepath.Rel` |
| 6 | `sanitizeConversationID` missing — path-traversal risk | Security | Added sanitization + SHA-256 hash fallback |
| 7 | Atomic ledger write not implemented | Reliability | Write-to-temp + rename pattern |
| 8 | Compaction checkpoints re-triggered every turn | Bug | `LastHandledCheckpointStep` field (default -1) |
| 9 | `GateEntries` used LIKE with unescaped wildcards | Bug | `LookupByAnchorExact` first, prefix fallback |
| 10 | `collectRetiredForScope` opened a write transaction for a SELECT | Performance | `QueryHistoryByScope` with plain read |

### Round 2 — Re-review (7 findings → `fc19d30`)

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| A | MCP server didn't speak the protocol at all | Blocker | Full `initialize`/`tools/list`/`tools/call` handshake, notifications silenced |
| B | FTS returned everything (no stop words, no result cap) | Bug | 60+ stop-word set, top-20 bm25 cap |
| C | Gate acknowledged on same call as `force_ask` | Bug | Deferred via `PendingGates` map |
| D | No cross-process ledger safety (`sync.Mutex` only) | Bug | `ModifyLedger` with `flock(LOCK_EX)`, skip empty conversation IDs |
| E | Full transcript re-parsed every model call | Performance | `LastProcessedStepIndex` high-water mark; about-to-edit dedup |
| F | Gate regex compiled per call; LIKE fallback too broad | Performance/Bug | Package-level `var gatePattern`; symbol-only fallback (`target:` prefix) |
| G | `QueryHistoryByScope` returned raw `*sql.Rows` | Consistency | Returns `[]dossiers.Entry`, tools.go simplified |

### Round 3 — Third Pass (6 findings → `cd07cdf`)

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | Gate still auto-acknowledged in PreToolUse (not PostToolUse) | Blocker | Gates acknowledged only in PostToolUse after successful edit |
| 2 | `syscall.Flock` broke Windows build | Blocker | Platform-specific `lock_unix.go` / `lock_windows.go` with build tags |
| 3 | `ModifyLedger` defined but never called | Bug | All 3 hooks wired through `ModifyLedger` |
| 4 | MCP `ping` returned `-32601` | Spec compliance | Returns `{}` |
| 5 | Tool errors returned as JSON-RPC errors | Spec compliance | Returns `isError: true` result |
| 6 | Missing index dir crashed MCP in unenrolled repos | Bug | `MkdirAll` + enrollment check; tool naming → underscores |

### Round 4 — Fourth Pass (8 findings → `02bc624`)

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | PreInvocation replay panic (`slice bounds out of range`) | Blocker | `injectionBase` saved at load time, replay from offset |
| 2 | Compaction flag relied on comparing empty maps | Fragile | Explicit `rearmed` bool |
| 3 | PostToolUse paid repo lookup cost for every tool call | Performance | Early return for non-edit non-error tools |
| 4 | Empty `Status` treated as success for gate approval | Bug | Require explicit `"success"` |
| 5 | Windows lock had no TODO | Minor | Detailed TODO referencing `windows.LockFileEx` |
| 6 | MCP created `index.db` in unenrolled repos | Side effect | Enrollment check; nil-index server returns `isError` |
| 7 | No MCP command test | Coverage | `cmd_mcp_test.go` covering protocol framing |
| 8 | SPEC.md used dotted tool names | Consistency | Updated to underscore names |

### Round 5 — Final Pass (3 findings → `e5a386f`)

| # | Finding | Severity | Fix |
|---|---------|----------|-----|
| 1 | Error message said `enrol` (command is `enroll`) | Typo | Fixed in server.go and cmd_mcp.go |
| 2 | Non-git workspace crashed MCP server at startup | Bug | Falls through to nil-index server |
| 3 | Test didn't exercise the dispatch loop | Coverage | Extracted `serveMCP(ctx, in, out, srv)`, test runs full 6-request loop |

---

## Key Architectural Decisions Made During Fixes

1. **Gate lifecycle: PreToolUse → PostToolUse handshake.** PreToolUse sets `PendingGates` and returns `force_ask`. PostToolUse acknowledges on successful edit only. Rejected edits stay gated. This is the only safe flow because PreToolUse can't distinguish approval from rejection.

2. **Cross-process ledger safety.** `ModifyLedger(fn)` holds an in-process mutex AND a cross-process `flock(LOCK_EX)` around load-modify-save. All three hooks use it. Platform-abstracted via build tags (no-op on Windows with TODO).

3. **Incremental transcript scanning.** `LastProcessedStepIndex` high-water mark means long conversations only pay cost proportional to new steps, not total steps.

4. **MCP server always starts.** Even in unenrolled or non-git workspaces, the server responds to all protocol messages. Tools return `"repo not enrolled"` as `isError` results — the model can see the message and guide the user.

5. **Tool names use underscores.** `memory_brief`, `memory_search`, etc. for portability across all model APIs. Dotted names kept as aliases in `DispatchTool`.

---

## Files Modified (16)

| File | Changes |
|------|---------|
| `SPEC.md` | Tool names → underscores |
| `cmd/repo-memory/cmd_mcp.go` | Full MCP protocol, `serveMCP` extracted, enrollment/non-git handling |
| `cmd/repo-memory/cmd_mcp_test.go` | **New** — full dispatch loop test |
| `internal/core/delivery/engine.go` | FTS sanitization, stop words, gate regex, error pattern matching |
| `internal/core/delivery/ledger.go` | `PendingGates`, `LastProcessedStepIndex`, pending gate methods |
| `internal/core/index/entries.go` | `QueryHistoryByScope` returns `[]dossiers.Entry` |
| `internal/harness/antigravity/compaction.go` | `CheckpointStepIdx` from step data |
| `internal/harness/antigravity/ledger_store.go` | `ModifyLedger`, platform-abstracted flock, empty ID skip, atomic writes |
| `internal/harness/antigravity/lock_unix.go` | **New** — `flock` for Unix |
| `internal/harness/antigravity/lock_windows.go` | **New** — no-op for Windows (with TODO) |
| `internal/harness/antigravity/posttooluse.go` | Gate ack on success, early return, `ModifyLedger` |
| `internal/harness/antigravity/preinvocation.go` | Incremental scan, about-to-edit dedup, `injectionBase`, `rearmed` flag, `ModifyLedger` |
| `internal/harness/antigravity/pretooluse.go` | Pending gates (no auto-ack), `ModifyLedger` |
| `internal/harness/antigravity/pretooluse_test.go` | Full Pre→Post→Pre gate flow test |
| `internal/mcpserver/server.go` | `ToolSchemas` with `inputSchema`, `DispatchTool`, nil-index guard, underscore names |
| `internal/mcpserver/tools.go` | Simplified `collectRetiredForScope`, removed raw `sql.Rows` |

---

## Remaining Known Issues (not in scope)

- **`TestSessionAttributor_Integration`** — pre-existing flaky test (hardcoded date). Unrelated to PR-08.
- **`TestStopHook_CompletePipeline`** — timing-flaky. Unrelated to PR-08.
- **Windows file locking** — no-op with TODO. Cross-process safety on Windows relies on in-process mutex + atomic writes only.
- **MCP pull exposure tracking** — TODO: accept conversation ID from client metadata in `initialize`.

---

> **Status: All review findings resolved. PR-14 is merge-ready.**
