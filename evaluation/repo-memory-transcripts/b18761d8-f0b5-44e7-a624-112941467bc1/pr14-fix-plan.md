# PR-14 Review Fix Plan — All 10 Comments

## Fix 1: Gate acknowledgments never persist (pretooluse.go:160)
**Problem:** `HandlePreToolUse` creates a fresh `NewLedger` every call; `AcknowledgeGate` is never called.
**Fix:** Load persisted ledger via `LoadLedger`, acknowledge gate after first `force_ask`, save ledger.

## Fix 2: FTS5 syntax errors on prompt terms (engine.go:70)
**Problem:** Raw prompt words with punctuation (`.`, `'`, `,`) go into FTS5 MATCH and cause syntax errors. Search error also discards diff-path results.
**Fix:** Quote each term, join with OR, strip non-alphanumeric chars. Separate search from diff-path so one doesn't kill the other.

## Fix 3: Diff paths are absolute but anchors are relative (preinvocation.go:208)
**Problem:** `filepath.Join(workspaceRoot, line)` makes absolute paths, but anchors are repo-relative. Also `HEAD~1` fails on single-commit repos.
**Fix:** Keep `line` as-is (git already returns repo-relative paths). Guard `HEAD~1` failure.

## Fix 4: ErrorEntries ignores output (engine.go:146)
**Problem:** `output` parameter never used; every failure signature injected regardless. No dedup.
**Fix:** Filter results by matching pattern against output. Add ledger dedup in PostToolUse.

## Fix 5: invocationNum > 0 blocks mid-turn moments (preinvocation.go:52)
**Problem:** Early return skips first-touch, first-entry, about-to-edit for mid-turn invocations.
**Fix:** Only gate task-start on invocationNum == 0; let other moments run with ledger dedup.

## Fix 6: Ledger file safety (ledger_store.go:14)
**Problem:** Empty/malicious conversationID; non-atomic write; no file lock; SaveLedger error ignored.
**Fix:** Sanitise ID (hash or reject unsafe), atomic write via temp+rename, check SaveLedger error.

## Fix 7: Pull tools not connected (server.go:63)
**Problem:** Nothing calls `NewServer` outside tests; pull exposures lost since no persisted ledger.
**Fix:** Wire up MCP tools in cmd_hook.go or cmd_mcp.go. For now, document clearly + add `RegisterTools` method.

## Fix 8: Compaction re-detected every turn (compaction.go:39)
**Problem:** No record of handled checkpoints; about-to-edit records empty EntryID.
**Fix:** Store `LastHandledCheckpointStep` in ledger; update after re-injection. Fix empty EntryID in about-to-edit.

## Fix 9: Gate detection by substring (engine.go:179)
**Problem:** `strings.Contains(ent.Text, "gate: edit")` — case-sensitive, accidental matches possible. LIKE prefix with `_`/`%` wildcards.
**Fix:** Add structured `Gate` field to Entry (or at minimum, use case-insensitive matching with a dedicated tag format). Use `LookupByAnchorExact` for gates.

## Fix 10: Read inside WriteTx in memory.history (tools.go:304)
**Problem:** SELECT inside WriteTx contends with writers; error discarded with `_ =`.
**Fix:** Use plain db.QueryContext instead of WriteTx; return error properly.
