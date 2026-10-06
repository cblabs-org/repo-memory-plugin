# Investigation: `injectSteps` Processing by Discovery Level and Session Type

**Date:** 2026-10-06
**Status:** Complete
**Related Experiment:** X5 — Message Injection Channels

---

## Objective

Determine why `systemMessage` injections via `injectSteps` in `PreInvocation` hooks were not
producing `SYSTEM_SDK` transcript steps, despite the hook firing and returning valid JSON.

---

## Background

The Antigravity platform supports injecting messages into agent context via `injectSteps` in
the stdout response from `PreInvocation`, `PostInvocation`, and `SessionStart` hooks. Three
message types are supported:

| Type | JSON Shape | Scope |
|---|---|---|
| `ephemeralMessage` | plain string | Single invocation |
| `userMessage` | plain string | Persistent until compaction |
| `systemMessage` | nested object `{systemMessage, metadata}` | Persistent until compaction |

Per documentation, injected messages should appear in `transcript_full.jsonl` with
`"source": "SYSTEM_SDK"`.

---

## Investigation Timeline

### Phase 1: Initial Observation

Our `PreInvocation` hook (in the plugin's `hooks.json`) was returning correct `injectSteps`
JSON. Subagents spawned via `invoke_subagent` occasionally reported "seeing" the injected
marker, but on closer inspection this was a **false positive** — the model was finding the
marker string in the *question* itself, not in an injected system message.

No `SYSTEM_SDK` steps ever appeared in any transcript file.

### Phase 2: Verify the Hook Actually Fires

Added debug tracing to write the raw payload and output to `/tmp/x5_hook_trace.log`.

**Result:** The hook fires correctly for every session. The daemon sends a well-formed JSON
payload including `conversationId`, `invocationNum`, `modelName`, `workspacePaths`, etc.
The hook returns valid `injectSteps` JSON. Example trace:

```
PAYLOAD_RAW: {"artifactDirectoryPath":"...","conversationId":"e0dcbeb4-...","initialNumSteps":0,
"invocationNum":0,"modelName":"gemini-3.5-flash-lite","transcriptPath":"...","workspacePaths":["..."]}
PARSED: conv=e0dcbeb4-... invNum=0
OUTPUT: {"injectSteps":[{"systemMessage":{"systemMessage":"[X5-CHANNEL-TEST conv=e0dcbeb4-...]
This is a test system message...","metadata":{"source":"repo-memory-x5-spike"}}},
{"ephemeralMessage":"[X5-CHANNEL-TEST conv=e0dcbeb4-...] This ephemeral message..."}]}
```

**Conclusion:** The hook fires and returns correct JSON. The issue is downstream — the
platform ignores the response.

### Phase 3: Eliminate the Binary as a Variable

Created an absolute minimum shell script hook (`/tmp/test_preinvocation.sh`) containing only:

```bash
#!/bin/bash
echo '{"injectSteps":[{"systemMessage":{"systemMessage":"SHELLMIN-MARKER: Minimal test.","metadata":{"source":"shell-min-test"}}}]}'
```

Swapped the plugin's `hooks.json` PreInvocation command to this script.

**Result:** Same behavior — hook fires, returns valid JSON, agent does not see the injected
message. This eliminates any issue with our Go binary's output format.

### Phase 4: Test Discovery Level — Workspace vs Plugin

Hypothesis: `injectSteps` processing may be gated by the hooks.json discovery level.

Created a workspace-level `.agents/hooks.json` pointing to the same minimal shell script:

```json
{
  "workspace-test": {
    "PreInvocation": [{"type": "command", "command": "/tmp/test_preinvocation.sh", "timeout": 10}]
  }
}
```

Tested with `agy -p` (CLI, top-level session):

```
$ agy --add-dir . -p "Do you see 'SHELLMIN-MARKER' in your context? Answer YES or NO only."
YES. I see: "SHELLMIN-MARKER: Minimal shell hook injection test."
```

**✅ It works from workspace-level hooks in CLI sessions!**

### Phase 5: Test Session Type — CLI vs IDE Subagent

With the same workspace-level `.agents/hooks.json`, tested via `invoke_subagent` (IDE):

```
Subagent response: NONE
```

**❌ Does not work for `invoke_subagent` conversations, even from workspace-level hooks.**

---

## Results Matrix

| Discovery Level | Session Type | Hook Fires? | `injectSteps` Processed? | `SYSTEM_SDK` in Transcript? |
|---|---|---|---|---|
| **Workspace** `.agents/hooks.json` | `agy -p` (CLI) | ✅ | ✅ | ✅ (expected) |
| **Workspace** `.agents/hooks.json` | `invoke_subagent` (IDE) | ✅ | ❌ | ❌ |
| **Plugin** `<plugin>/hooks.json` | `agy -p` (CLI) | ✅ | ❌ | ❌ |
| **Plugin** `<plugin>/hooks.json` | `invoke_subagent` (IDE) | ✅ | ❌ | ❌ |

### What Still Works from Plugin-Level Hooks

- **PreToolUse** decisions (`allow`/`deny`/`ask`) — confirmed working, actively blocking
  commands in this session
- **Stop** decisions (`continue`/`terminate`) — confirmed in prior experiments (X1)
- **PostToolUse** observation — hook fires, receives tool output

### What Does NOT Work from Plugin-Level Hooks

- **`injectSteps`** responses from `PreInvocation`, `PostInvocation`, `SessionStart` are
  silently ignored

---

## Verification Commands Used

```bash
# Verify hook registration
agy -p "/hooks"
# → Shows repo-memory PreInvocation hook as "enabled"

# Verify hook fires (debug trace)
cat /tmp/x5_hook_trace.log
# → Shows PAYLOAD_RAW, PARSED, OUTPUT with correct JSON

# Direct binary test (works correctly)
echo '{"conversationId":"test","invocationNum":0}' | ./repo-memory hook antigravity PreInvocation --workspace .
# → Returns valid injectSteps JSON

# Workspace-level hook test (CLI — WORKS)
agy --add-dir . -p "Do you see 'SHELLMIN-MARKER'?"
# → YES

# Workspace-level hook test (invoke_subagent — FAILS)
# Spawned subagent with same workspace, same .agents/hooks.json
# → NONE
```

---

## Architectural Implications for v2

### Problem

The v2 design calls for PreInvocation dossier delivery — injecting repository context into
agent sessions via `injectSteps`. The current plugin architecture places hooks in
`<plugin-root>/hooks.json`, which does **not** support `injectSteps` processing.

### Solution

The `enroll` command must **write or merge the PreInvocation hook into the workspace's
`.agents/hooks.json`**, not the plugin's `hooks.json`. This means:

1. **Split hook placement:**
   - `PreInvocation` (dossier injection) → `.agents/hooks.json` (workspace level)
   - `PreToolUse` (boundary guard) → `<plugin>/hooks.json` (plugin level, works)
   - `Stop` (session capture) → `<plugin>/hooks.json` (plugin level, works)
   - `PostToolUse` (observation) → `<plugin>/hooks.json` (plugin level, works)

2. **`enroll` must merge, not overwrite:** The workspace may already have `.agents/hooks.json`
   with hooks from other tools. The enroll command must parse the existing file, add/update
   the `repo-memory` hook group, and write back — preserving other hook groups.

3. **`unenroll` must clean up:** Remove the `repo-memory` hook group from
   `.agents/hooks.json`. If the file becomes empty (only contained our hooks), delete it.

4. **IDE subagent limitation:** Even workspace-level hooks don't process `injectSteps` for
   `invoke_subagent` conversations. This means dossier delivery only works for:
   - Top-level CLI sessions (`agy -p`)
   - Top-level IDE sessions (new chat, not spawned subagents)
   
   This is acceptable for the primary use case (human-initiated sessions in enrolled repos)
   but means subagents spawned by the primary agent won't receive the dossier injection.
   The primary agent's dossier context should propagate to subagents through the prompt
   and system context inheritance.

### Open Questions

1. **Is the `invoke_subagent` limitation a bug or by design?** Subagents inherit their
   parent's customizations but may intentionally skip `injectSteps` to avoid duplicate
   injections (the parent already has the injected context, and subagents inherit it).

2. **Does the IDE (non-CLI) top-level session process `injectSteps` from workspace hooks?**
   We only tested CLI (`agy -p`) and IDE subagents. A top-level IDE chat session may behave
   like the CLI. This needs verification.

3. **Will plugin-level `injectSteps` be supported in future versions?** The current behavior
   may be intentional (security boundary) or an implementation gap.

---

## Files Modified During Investigation

| File | Change | Status |
|---|---|---|
| `docs/experiments/X5-channels.md` | Updated with discovery level matrix | Committed |
| `docs/antigravity-hooks-reference.md` | Corrected discovery paths | Committed |
| `cmd/repo-memory/cmd_hook.go` | Temp: removed env gate for testing | Reverted |
| `internal/harness/antigravity/v2_spike_channels.go` | Temp: added debug traces | Reverted |
| `.agents/hooks.json` | Temp: workspace-level test hook | Deleted |
| `/tmp/test_preinvocation.sh` | Temp: minimal shell script hook | Deleted |
| `/tmp/x5_hook_trace.log` | Temp: debug trace output | Deleted |
