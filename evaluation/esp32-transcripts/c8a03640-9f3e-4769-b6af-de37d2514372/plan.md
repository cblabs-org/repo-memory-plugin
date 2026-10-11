# Implementation Plan: Consolidate PIO Rules & Two-Mode Enforcement

## Goal

Merge rules 04 (MCP Tooling Priority) and 05 (MCP Locks) into a single coherent
rule, add worktree-aware two-mode enforcement to `pio-cli-gate.py`, and create a
new `pio-reset-gate.py` hook to mechanically block autonomous `reset_server_state`
calls. Filed upstream feature request for cleaner mode detection:
[bug-fixer-loop-plugin#32](https://github.com/matthewmcneill/bug-fixer-loop-plugin/issues/32).

## User Review Required

> [!IMPORTANT]
> **Worktree heuristic as autonomous-mode signal.** The `pio-cli-gate` uses CWD
> path inspection: if `Cwd` contains a `/worktrees/` segment → `deny`
> (autonomous), otherwise → `force_ask` (interactive). This is simple and correct
> for your current layout but would need updating if you ever run interactive
> sessions from inside a worktree. A cleaner conversation-scoped signal is
> tracked in [bug-fixer-loop-plugin#32](https://github.com/matthewmcneill/bug-fixer-loop-plugin/issues/32)
> but needs design work — env vars don't propagate reliably through Antigravity's
> hook subprocess chain, so the mechanism may need to be payload-level metadata
> instead.

> [!WARNING]
> **`reset_server_state` can only be `force_ask`, not `deny`-for-autonomous.**
> The `call_mcp_tool` payload has no `Cwd` field, so the worktree heuristic
> doesn't apply. All callers get `force_ask`. For the bug-fixer this means it
> blocks (acceptable — better than resetting the server). The env var from #32
> would close this gap.

---

## Proposed Changes

### Rule Consolidation

#### [MODIFY] [04-mcp-tooling.md](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/.agents/rules/04-mcp-tooling.md)

Rewrite to absorb all content from rule 05. The consolidated rule covers three
concerns: MCP-first, lock discipline, and reset prohibition.

```markdown
# 04-MCP PlatformIO Discipline

This project uses the `platformio` MCP server for all PlatformIO-related tasks.
The rules below are non-negotiable — they protect multi-agent workflows from
deadlocks, orphaned locks, and corrupted hardware state.

## 1. MCP-First — No Direct CLI

When compiling, building, cleaning, uploading firmware/filesystem, or performing
other PlatformIO operations, you **MUST** use the `platformio` MCP tools
(`build_project`, `clean_project`, `upload_firmware`, `upload_filesystem`,
`run_tests`, etc.) — never direct `pio`, `platformio`, or `esptool` CLI commands.

**Why:** The MCP server manages project state, queue locks, and environment
across agents. Direct CLI use bypasses all of that.

## 2. Lock Discipline — Wait Your Turn

Commands that execute long-running operations — `build_project`, `run_tests`,
`upload_firmware`, `upload_filesystem` — **implicitly acquire a hardware queue
lock**. Even without an explicit `acquire_lock`, the MCP server locks the
resource on your behalf.

- **Release:** Once your task completes (success or failure), you **MUST**
  explicitly call `release_lock`.
- **Contention:** If you receive a "lock held" or "resource busy" error,
  another agent is executing. You **MUST wait your turn**:
  1. Call `get_lock_status` to inspect the queue.
  2. Wait and retry after a short delay.
  3. **DO NOT** fall back to raw CLI commands to bypass the lock.
  4. **DO NOT** call `reset_server_state` (see §3).

## 3. Never Reset Autonomously

`reset_server_state` is a **destructive action** that terminates active builds
and flashes mid-operation.

- You **MUST NEVER** execute `reset_server_state` autonomously.
- If you have explicitly verified a lock is orphaned (held by a crashed/dead
  session with no progress), you **MUST stop and ask the human user** for
  explicit permission (HITL intervention).

## Mechanical Enforcement

These rules are enforced by two `PreToolUse` lifecycle hooks
(`.agents/hooks.json`):

| Hook | Intercepts | Decision |
|------|-----------|----------|
| `pio-cli-gate` | `run_command` containing `pio`/`platformio`/`esptool` | `deny` (worktree) · `force_ask` (interactive) |
| `pio-reset-gate` | `call_mcp_tool` with `reset_server_state` | `force_ask` |

**For agents:** Do not attempt to use `pio`, `esptool`, or `reset_server_state`
directly. Use the MCP tools and respect the lock queue. If the hook blocks you,
it means you should be waiting your turn or asking the human for help.
```

#### [DELETE] [05-mcp-locks.md](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/.agents/rules/05-mcp-locks.md)

Content absorbed into consolidated `04-mcp-tooling.md`.

---

### Hook Scripts

#### [MODIFY] [pio-cli-gate.py](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/.agents/scripts/pio-cli-gate.py)

Two changes:
1. **Worktree detection** — if `Cwd` contains `/worktrees/`, emit `deny` instead
   of `force_ask` (autonomous agent, no human to approve).
2. **Enhanced message** — covers both CLI prohibition and lock-bypass context,
   tells the agent what to do instead (wait, `get_lock_status`).

Key diff (conceptual):

```python
# New: detect autonomous mode from worktree CWD
_WORKTREE_SEGMENT = "/worktrees/"

def _is_autonomous(cwd: str) -> bool:
    """Detect bug-fixer autonomous session via worktree CWD heuristic.

    This checks whether the command's working directory is inside a
    worktrees/ directory, which indicates an autonomous bug-fixer session.

    Future: bug-fixer-loop-plugin#32 may provide a cleaner signal
    (e.g. conversation-scoped metadata in the hook payload). The
    mechanism needs design work — env vars won't propagate reliably
    through Antigravity's hook subprocess chain.
    """
    return _WORKTREE_SEGMENT in cwd


def _force_ask(matched_binary):
    """Emit force_ask decision (interactive session)."""
    json.dump({
        "decision": "force_ask",
        "reason": (
            f"⚠️ Direct CLI use of '{matched_binary}' detected. "
            f"Rule 04 requires using the `platformio` MCP tools instead "
            f"(build_project, upload_firmware, etc.) for proper lock "
            f"management and multi-agent safety.\n\n"
            f"If the MCP tools returned a lock error, you must WAIT YOUR "
            f"TURN — use `get_lock_status` to check the queue. Do NOT "
            f"attempt to bypass by using CLI commands or resetting the "
            f"server.\n\n"
            f"If this is an authorised debugging operation, you may approve "
            f"this specific invocation. The approval will NOT be cached — "
            f"each direct CLI use requires fresh permission."
        ),
    }, sys.stdout)
    sys.exit(0)


def _deny_autonomous(matched_binary):
    """Emit deny decision (autonomous/worktree session)."""
    json.dump({
        "decision": "deny",
        "reason": (
            f"⛔ Direct CLI use of '{matched_binary}' is prohibited in "
            f"autonomous mode. Rule 04 requires using the `platformio` MCP "
            f"tools (build_project, upload_firmware, etc.).\n\n"
            f"If the MCP tools returned a lock error, wait your turn: call "
            f"`get_lock_status`, then retry after a delay. If the lock "
            f"appears orphaned, exit cleanly and let the next scheduled "
            f"run retry."
        ),
    }, sys.stdout)
    sys.exit(0)
```

In `main()`, after detecting a match:

```python
    matched = _check_command(command_line)
    if matched:
        cwd = args.get("Cwd", "")
        if _is_autonomous(cwd):
            _deny_autonomous(matched)
        else:
            _force_ask(matched)
```

#### [NEW] pio-reset-gate.py

New script at `.agents/scripts/pio-reset-gate.py`. Intercepts `call_mcp_tool`
where `ToolName == "reset_server_state"`.

```python
#!/usr/bin/env python3
"""ESP32 App Foundation — Antigravity PreToolUse Hook
.agents/scripts/pio-reset-gate.py

Enforces rule 04 §3 (Never Reset Autonomously): blocks autonomous
reset_server_state calls, requiring explicit per-invocation user approval.

Uses "force_ask" so the "Always Allow" cache is bypassed — every single
reset_server_state invocation needs fresh human approval.
"""

import json
import sys


def _allow():
    json.dump({"decision": "allow"}, sys.stdout)
    sys.exit(0)


def _force_ask():
    json.dump({
        "decision": "force_ask",
        "reason": (
            "⚠️ `reset_server_state` is a destructive action that terminates "
            "active builds and flashes mid-operation. Rule 04 §3 prohibits "
            "autonomous use.\n\n"
            "Before approving, verify:\n"
            "  1. The lock is genuinely orphaned (crashed/dead session)\n"
            "  2. No other agent is actively building or flashing\n"
            "  3. `get_lock_status` confirms no progress is being made\n\n"
            "If you are an autonomous agent (bug-fixer), do NOT approve — "
            "exit cleanly and let the next scheduled run retry.\n\n"
            "The approval will NOT be cached — each reset requires fresh "
            "human permission."
        ),
    }, sys.stdout)
    sys.exit(0)


def main():
    payload = json.load(sys.stdin)
    tool_call = payload.get("toolCall", {})
    tool_name = tool_call.get("name", "")

    # Only gate call_mcp_tool invocations
    if tool_name != "call_mcp_tool":
        _allow()

    args = tool_call.get("args", {})
    mcp_tool = args.get("ToolName", "")

    if mcp_tool == "reset_server_state":
        _force_ask()

    _allow()


if __name__ == "__main__":
    main()
```

---

### Hook Configuration

#### [MODIFY] [hooks.json](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/.agents/hooks.json)

Add the `pio-reset-gate` entry. The existing `pio-cli-gate` path is correct
(relative to `.agents/`).

```diff
   "pio-cli-gate": {
     ...
   },
+  "pio-reset-gate": {
+    "PreToolUse": [
+      {
+        "matcher": "call_mcp_tool",
+        "hooks": [
+          {
+            "type": "command",
+            "command": "python3 ./scripts/pio-reset-gate.py",
+            "timeout": 5
+          }
+        ]
+      }
+    ]
+  },
   "bug-fixer-file-gate": {
     ...
```

---

### Worktree Sync

#### [MODIFY] Worktree copies

Sync updated files to active worktrees:
- `worktrees/issue-278/`
- `worktrees/issue-279/`
- `worktrees/issue-280/`

Files to sync:
- `.agents/rules/04-mcp-tooling.md`
- `.agents/rules/05-mcp-locks.md` (delete)
- `.agents/scripts/pio-cli-gate.py`
- `.agents/scripts/pio-reset-gate.py` (new)
- `.agents/hooks.json`

---

## Verification Plan

### Automated Tests

```bash
# 1. Verify pio-cli-gate detects direct CLI and applies correct mode
echo '{"toolCall":{"name":"run_command","args":{"CommandLine":"pio run","Cwd":"/workspace"}}}' \
  | python3 .agents/scripts/pio-cli-gate.py
# Expected: {"decision": "force_ask", ...}

echo '{"toolCall":{"name":"run_command","args":{"CommandLine":"pio run","Cwd":"/workspace/worktrees/issue-99"}}}' \
  | python3 .agents/scripts/pio-cli-gate.py
# Expected: {"decision": "deny", ...}

echo '{"toolCall":{"name":"run_command","args":{"CommandLine":"ls -la","Cwd":"/workspace"}}}' \
  | python3 .agents/scripts/pio-cli-gate.py
# Expected: {"decision": "allow"}

# 2. Verify pio-reset-gate blocks reset_server_state
echo '{"toolCall":{"name":"call_mcp_tool","args":{"ToolName":"reset_server_state"}}}' \
  | python3 .agents/scripts/pio-reset-gate.py
# Expected: {"decision": "force_ask", ...}

echo '{"toolCall":{"name":"call_mcp_tool","args":{"ToolName":"build_project"}}}' \
  | python3 .agents/scripts/pio-reset-gate.py
# Expected: {"decision": "allow"}
```

### Manual Verification

1. **Interactive session**: Try `pio run` — should get `force_ask` prompt
2. **Worktree session**: Try `pio run` from a worktree — should get hard `deny`
3. **Reset gate**: Try `reset_server_state` via MCP — should get `force_ask`
4. **Non-PIO commands**: Normal `run_command` calls should pass through unblocked
5. **Flash-tag-gate coexistence**: `upload_firmware` should still be checked by
   the flash-tag-gate (both hooks fire independently for `call_mcp_tool`)
