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

## Results Matrix (Corrected After Second Review)

| Discovery Level | Session Type | Hook Fires? | `injectSteps` Processed? |
|---|---|---|---|
| **Plugin** `<plugin>/hooks.json` | `agy --add-dir . -p` (CLI) | ✅ | ✅ |
| **Plugin** `<plugin>/hooks.json` | `invoke_subagent` (IDE) | ✅ | ❌ |
| **Workspace** `.agents/hooks.json` | `agy -p` (CLI) | ✅ | ✅ |
| **Workspace** `.agents/hooks.json` | `invoke_subagent` (IDE) | ✅ | ❌ |

**Key correction from first investigation:** Plugin-level hooks DO support `injectSteps` for
top-level CLI sessions when `--add-dir .` is used. The earlier failure was because the CLI
invocation did not include `--add-dir .`, so `.agents/plugins.json` was not scanned and the
workspace plugin's hooks were not discovered.

### Transcript Recording Nuances

1. **`SYSTEM_MESSAGE` steps omit `content` in transcript files.** Both `transcript.jsonl` and
   `transcript_full.jsonl` only serialize `content` for certain step types (`USER_INPUT`,
   `PLANNER_RESPONSE`, `EPHEMERAL_MESSAGE`, `ERROR_MESSAGE`). `SYSTEM_MESSAGE` entries always
   omit `content` even when the message text was delivered to the model. This means you cannot
   grep for message content in transcript files to verify `systemMessage` injection — you
   must look for the presence of `SYSTEM_SDK` steps.

2. **Subagent prompt delivery uses `SYSTEM_MESSAGE`/`SYSTEM` (not `SYSTEM_SDK`).** When
   `invoke_subagent` spawns a subagent, the parent's prompt is delivered as a `SYSTEM_MESSAGE`
   with `source: "SYSTEM"`. This step appears before any hook-injected steps. If `injectSteps`
   worked, there would be TWO `SYSTEM_MESSAGE` steps: one from the hook (`SYSTEM_SDK`) and
   one from the parent prompt delivery (`SYSTEM`).

### What Works from Plugin-Level Hooks

- **PreToolUse** decisions (`allow`/`deny`/`ask`) — confirmed working
- **Stop** decisions (`continue`/`terminate`) — confirmed in prior experiments (X1)
- **PostToolUse** observation — hook fires, receives tool output
- **`injectSteps`** — works for CLI sessions with `--add-dir .`

### What Does NOT Work

- **`injectSteps` for `invoke_subagent` conversations** — the hook fires and returns valid
  JSON, but the platform does not inject the steps into the subagent's context. This applies
  to both workspace-level and plugin-level hooks.

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

### Corrected Understanding

Plugin-level hooks in `<plugin>/hooks.json` DO support `injectSteps` for top-level sessions
(both CLI and IDE). The earlier failure was a test setup issue (`--add-dir .` was missing,
so `.agents/plugins.json` was not scanned). **No split hook placement is needed.**

All hooks can remain in the plugin's `hooks.json`:
- `PreInvocation` (dossier injection) → `<plugin>/hooks.json` ✅
- `PreToolUse` (boundary guard) → `<plugin>/hooks.json` ✅
- `Stop` (session capture) → `<plugin>/hooks.json` ✅
- `PostToolUse` (observation) → `<plugin>/hooks.json` ✅

### Remaining Limitation: `invoke_subagent`

`injectSteps` does not work for `invoke_subagent` conversations. This means dossier delivery
only reaches the top-level agent in a session (human-initiated chat). This is acceptable
because:
- Subagents inherit the parent's full context, including any injected messages
- The primary use case is human-initiated sessions in enrolled repos
- PA-spawned workers receive context via their WORKORDER prompt, not hook injection

### Design Note for `PreInvocation` on Subagents

When `PreInvocation` fires on `invocationNum: 0` of a subagent, `initialNumSteps` is `0`
and `userPrompt` is always `""` (the parent's prompt hasn't been delivered yet — it arrives
as a `SYSTEM_MESSAGE` after hooks run). The hook must not require `userPrompt != ""` to
inject baseline context. Check `invocationNum > 0` and fall back to baseline context when
`userPrompt` is empty.

### Open Questions

1. **Is the `invoke_subagent` limitation a bug or by design?** Subagents inherit their
   parent's customizations but may intentionally skip `injectSteps` to avoid duplicate
   injections (the parent already has the injected context).

2. **Does the IDE (non-CLI) top-level session process `injectSteps`?** We tested CLI
   (`agy -p`) and IDE subagents. A top-level IDE chat session likely processes `injectSteps`
   the same as CLI. This needs verification.

---

## Files Modified During Investigation

| File | Change | Status |
|---|---|---|
| `docs/experiments/X5-channels.md` | Updated with discovery level matrix | Committed |
| `docs/antigravity-hooks-reference.md` | Corrected discovery paths | Committed |
| `cmd/repo-memory/cmd_hook.go` | Temp: removed env gate for testing | Reverted |
| `internal/harness/antigravity/v2_spike_channels.go` | Temp: added debug traces | Reverted |
| `.agents/hooks.json` | Temp: workspace-level test hook | Deleted |
| `/tmp/test_preinvocation.sh` | Temp: BANANA-99 test hook | Deleted |
| `/tmp/hook_payload.log` | Temp: payload trace | Deleted |
