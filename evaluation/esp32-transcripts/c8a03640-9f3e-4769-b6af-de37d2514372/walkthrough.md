# Walkthrough: PIO Rule Consolidation & Two-Mode Enforcement

## Changes Made

### 1. Consolidated rules 04+05 → single `04-mcp-tooling.md`
- **Deleted** `05-mcp-locks.md`
- **Rewrote** `04-mcp-tooling.md` with three clear sections:
  - §1 MCP-First (no direct CLI)
  - §2 Lock Discipline (wait your turn, release locks)
  - §3 Never Reset Autonomously (HITL required)
- Mechanical Enforcement table documents both hooks

### 2. Updated `pio-cli-gate.py` — two-mode enforcement
- **Worktree CWD** (`/worktrees/` in path) → `deny` (autonomous bug-fixer, no human present)
- **Main workspace CWD** → `force_ask` (interactive, user approves per-invocation)
- **Enhanced messages** now cover both CLI prohibition AND lock-bypass context, telling agents to use `get_lock_status` and wait

### 3. New `pio-reset-gate.py` hook
- Intercepts `call_mcp_tool` with `ToolName == "reset_server_state"`
- Issues `force_ask` for all callers (can't detect autonomous mode for MCP calls — no `Cwd` field)
- Message includes pre-approval checklist and explicit instruction for autonomous agents to exit cleanly

### 4. Updated `hooks.json`
- Added `pio-reset-gate` entry matching `call_mcp_tool`

### 5. Filed upstream issue
- [bug-fixer-loop-plugin#32](https://github.com/matthewmcneill/bug-fixer-loop-plugin/issues/32) — request for conversation-scoped autonomous-mode signal
- Added follow-up comment noting env var propagation concern and alternative approaches

### 6. Synced worktrees
- issue-280, issue-283, issue-284 — all files synced, old rule 05 deleted

## What Was Tested

All 7 hook test cases pass:

| Test | Input | Expected | Result |
|------|-------|----------|--------|
| `pio run` interactive | Cwd: `/workspace` | `force_ask` | ✅ |
| `pio run` worktree | Cwd: `/workspace/worktrees/issue-99` | `deny` | ✅ |
| `ls -la` (normal cmd) | Cwd: `/workspace` | `allow` | ✅ |
| `esptool.py` worktree | Cwd: `.../worktrees/issue-280/src` | `deny` | ✅ |
| `reset_server_state` | MCP call | `force_ask` | ✅ |
| `build_project` | MCP call | `allow` | ✅ |
| `run_command` (non-MCP) | through reset gate | `allow` | ✅ |

## Validation
- `hooks.json` validates as valid JSON
- All three worktrees confirmed identical to main workspace
- Rule 05 confirmed deleted from all locations
