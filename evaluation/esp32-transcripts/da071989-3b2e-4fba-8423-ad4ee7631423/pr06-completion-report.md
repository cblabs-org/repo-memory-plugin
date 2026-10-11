# PR-06-v2-spike: Session Completion Report

**Date:** 2026-10-05
**Branch:** `pr-06-v2-spike/wo2-hook-contracts`
**Commits:** `920e915` → `7b46fe6` (10 commits on spike branches)

## Work Order Status

| WO | Experiment | Status | Key Result |
|---|---|---|---|
| WO1 | **X1: Stop Continue** | ✅ Complete | 21 conversations, 100% success, 5 real notes harvested |
| WO2 | **X9: Hook Contracts** | ✅ Complete | All 4 hook types documented, 35 contract tests |
| WO3 | X5: Channels | ⏳ Not started | |
| WO4 | X8: Lean Agent | ⏳ Not started | |
| WO5 | X3: Quota | ⏳ Not started | |

## X1 Results Summary

- **21/21 conversations** passed the stop-continue cycle (100%)
- **5 notes with content**, 16 NONE responses (trivial tasks self-filter)
- **0 failures** — no errors, no user interleaving
- **Overhead:** 1-2s (flash_lite), 3-4s (flash), ~30s (pro with 7-item note)
- **Report:** [`docs/experiments/X1-stop-continue.md`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/docs/experiments/X1-stop-continue.md)

### X1 Exit Criteria Assessment

| Criterion | Target | Result | Pass? |
|---|---|---|---|
| Success rate | ≥ 95% of ≥ 20 requests | 100% (21/21) | ✅ |
| Token cost | Median uncached ≤ 10k | Not measured (no telemetry in hook) | ⚠️ |
| No interleaving | User messages don't arrive during continue | None observed | ✅ |
| PA/worker impact | No disruption | Not tested | ⚠️ |

## X9 Results Summary

- **4 hook types** fully documented: PreInvocation, PreToolUse, PostToolUse, Stop
- **PreToolUse is fail-CLOSED by the platform** — critical finding
- **35 contract tests** added to hooks_test.go
- **Report:** [`docs/experiments/X9-hook-contracts.md`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/docs/experiments/X9-hook-contracts.md)

## Platform Findings (Unplanned but Significant)

### 1. Plugin Enablement Bug — FIXED

**Root cause:** `sidecars/tick/sidecar.json` had `agent_permissions`, which
caused Antigravity to default the entire plugin to disabled. All hooks
(PreInvocation, PreToolUse, PostToolUse, Stop) were silent.

**Fixes applied:**
- Removed `agent_permissions` from sidecar.json (tick uses OS subprocess, not agent tools)
- Added `ensurePluginEnabled()` to `cmd_install.go` — writes `enabled: true` to config.json
- Fixed hooks.json key casing: `"stop"` → `"Stop"` (PascalCase required)

### 2. `parentConversationId` Absent from Stop Payload

Antigravity does not populate `parentConversationId` in the Stop hook payload.
Gate 4 (subagent skip) never fires in production. Workaround options documented
(SQLite lookup, message file inspection). Not a blocker — NONE self-filtering
handles trivial subagents naturally.

### 3. Plugin Architecture Redesign — DOCUMENTED

Designed and documented a split architecture to eliminate overhead in non-enrolled repos:

- **`repo-memory-daemon`** (global): sidecar only, zero session overhead
- **`repo-memory`** (workspace): hooks/MCP/rules/skills, loaded per-repo via `.agents/plugins.json`
- Single directory tree under `~/.gemini/config/plugins/repo-memory-daemon/`
- **Report:** [`docs/antigravity-plugin-architecture.md`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/docs/antigravity-plugin-architecture.md)

## Documentation Produced

| Document | Purpose |
|---|---|
| [`docs/experiments/X1-stop-continue.md`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/docs/experiments/X1-stop-continue.md) | Full X1 experiment report with 21-conversation data |
| [`docs/experiments/X9-hook-contracts.md`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/docs/experiments/X9-hook-contracts.md) | Full X9 hook contracts report |
| [`docs/antigravity-plugin-hooks.md`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/docs/antigravity-plugin-hooks.md) | Plugin hooks discovery, enablement, sidecars reference |
| [`docs/antigravity-plugin-architecture.md`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/docs/antigravity-plugin-architecture.md) | Split daemon + workspace bundle design |

## Open Items for Remaining WOs

### WO3 (X5: Channels) — Dependencies: None
- Test `systemMessage` and `userMessage` persistence across invocations/turns/checkpoints

### WO4 (X8: Lean Agent) — Dependencies: None
- Build `repo-memory-distiller` Markdown agent
- Test agent discovery from runtime working directory

### WO5 (X3: Quota) — Dependencies: WO1 (done), WO3, WO4
- Measure cached token costs against quota
- Summarise all experiments and propose SPEC v1.0 edits

## Design Changes Recommended for v2

Based on X1 and X9 findings:

1. **Remove Gate 4 (subagent skip)** from spike — `parentConversationId` absent, NONE self-filters
2. **Use `payload.transcriptPath`** as primary — `findBrainTranscript` only as fallback
3. **Implement split plugin architecture** — daemon + workspace bundle
4. **Measure token cost** via transcript analysis (not available in hook payload)
5. **For production:** resolve subagent detection via `conversation_summaries.db`

## Commits on Spike Branch

```
7b46fe6 docs: split daemon + workspace bundle architecture
540ce8a docs: sidecars require global plugin installation
1342c3a fix: PascalCase Stop in hooks.json, add sidecars docs
f75d5ca chore: remove experiment debug artifacts
ea91e0a fix: auto-enable plugin on install, remove agent_permissions from sidecar
76f642a docs: Antigravity plugin hooks discovery & enablement reference
df8896d docs: X1 experiment complete — 21 conversations, 100% success rate
15a7653 test(antigravity): ensure deterministic mtime in TestSessionAttributor_Integration
b3a4b6e docs: X1 experiment report (awaiting live test data)
920e915 feat(v2-spike): X1 Stop continue prototype gate and harvest
```
