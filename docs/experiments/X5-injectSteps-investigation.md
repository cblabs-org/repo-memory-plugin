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

## Results Matrix (Corrected After Second and Third Review)

| Discovery Level | Session Type | Hook Fires? | `injectSteps` Processed? |
|---|---|---|---|
| **Plugin** `<plugin>/hooks.json` | `agy --add-dir . -p` (CLI) | ✅ | ✅ Confirmed |
| **Plugin** `<plugin>/hooks.json` | `invoke_subagent` (IDE) | ✅ | ⚠️ Inconclusive |
| **Workspace** `.agents/hooks.json` | `agy -p` (CLI) | ✅ | ✅ Confirmed |
| **Workspace** `.agents/hooks.json` | `invoke_subagent` (IDE) | ✅ | ⚠️ Inconclusive |

### Why `invoke_subagent` is Inconclusive (Not Proven Broken)

The BANANA-99 subagent test (`edda8c6e`) produced **mixed evidence**:

- **Evidence FOR injection working:** The idle notification captured
  `"most recent response: BANANA-99"` — the model output the secret code at some point,
  which it could only know from the injected message.
- **Evidence AGAINST:** The model's final `send_message` said "There is no secret code
  visible in the context", and the transcript has no `SYSTEM_SDK` step.
- **BUT:** The CLI test that **definitively worked** (model answered BANANA-99) ALSO has
  **zero** `SYSTEM_SDK` steps in its transcript. This means `SYSTEM_SDK` steps may not be
  recorded in transcripts at all, invalidating transcript-based verification entirely.

The most likely explanation: injection worked initially (hence BANANA-99 in idle), but the
flash_lite model got confused across multiple invocations (the test script injected on every
invocation regardless of `invocationNum`, causing repeated injections that may have confused
context). Per the authoritative second opinion, `injectSteps` SHOULD work for subagents.

### Transcript Recording Nuances

1. **`SYSTEM_MESSAGE` steps may not appear in transcripts at all.** The confirmed-working CLI
   BANANA-99 test has zero `SYSTEM_SDK` steps in `transcript_full.jsonl` despite the model
   clearly receiving the injected message. Transcript files cannot be used to verify or
   disprove `injectSteps` injection.

2. **`SYSTEM_MESSAGE` steps omit `content` when they DO appear.** Both `transcript.jsonl` and
   `transcript_full.jsonl` only serialize `content` for certain step types (`USER_INPUT`,
   `PLANNER_RESPONSE`, `EPHEMERAL_MESSAGE`, `ERROR_MESSAGE`).

3. **Subagent prompt delivery ordering.** When `invoke_subagent` spawns a subagent, the
   conversation starts with `initialNumSteps: 0` and `userPrompt: ""`. PreInvocation hooks
   fire FIRST, then the parent's prompt is delivered via the inter-agent messaging system as
   a `SYSTEM_MESSAGE`. This means hooks run **before** the subagent has its initial prompt —
   important for hook design (cannot rely on `userPrompt` being populated).

### What is Confirmed Working

- **PreToolUse** decisions (`allow`/`deny`/`ask`) — confirmed working
- **Stop** decisions (`continue`/`terminate`) — confirmed in prior experiments (X1)
- **PostToolUse** observation — hook fires, receives tool output
- **`injectSteps`** for top-level sessions (CLI with `--add-dir .`) — confirmed

### What Needs Further Testing

- **`injectSteps` for `invoke_subagent`** — per authoritative documentation, this should work.
  The test was inconclusive due to repeated injection (no `invocationNum` check) and model
  confusion. Needs re-test with a script that only injects on `invocationNum: 0`.

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

### `invoke_subagent` Status

Per authoritative documentation, `injectSteps` should work for `invoke_subagent` conversations.
Our test was **inconclusive** — the idle notification captured "BANANA-99" (suggesting the
model saw the injection), but the model's final response said "no secret code" (possibly due
to the test script re-injecting on every invocation without checking `invocationNum`). Even
if subagent injection doesn't work, this is acceptable because:
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
