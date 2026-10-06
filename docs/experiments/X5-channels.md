# X5 Channels Experiment

**Date:** 2026-10-05 (initial), 2026-10-06 (correction & validation)
**Branch:** pr-06-v2-spike / WO3

## Methodology

We built a prototype PreInvocation hook that returns messages via the `injectSteps` response mechanism. We used `invoke_subagent` to spawn test agents in the enrolled repository and then inspected their responses and `transcript_full.jsonl` files.

## CORRECTION: Initial Finding Was Wrong

The initial experiment (2026-10-05) concluded that `systemMessage` and `userMessage` were "not supported." **This was incorrect.** The prototype used the wrong JSON shape:

```json
// WRONG — what the initial prototype returned:
{"systemMessage": "plain string", "userMessage": "plain string"}

// CORRECT — the required format:
{
  "injectSteps": [
    {
      "systemMessage": {
        "systemMessage": "System-level instruction.",
        "metadata": {"source": "repo-memory"}
      }
    },
    {
      "ephemeralMessage": "Transient context for this invocation only."
    },
    {
      "userMessage": "Persistent user-role message."
    }
  ]
}
```

Key schema requirements:
- Must wrap in `injectSteps` array
- `systemMessage` must be a **nested object** (not a plain string) with a `systemMessage` string field and optional `metadata`
- `ephemeralMessage` and `userMessage` are plain strings
- `toolCall` inside `injectSteps` is documented but empirically a no-op

## Validated Results (2026-10-06)

### Test A: systemMessage injection — ✅ WORKS

Spawned subagent `187dd387-936d-418f-ac03-526e8129c6e4` with a multi-step task.
- The agent explicitly confirmed: *"Yes, I can see a system message about X5-CHANNEL-TEST in my context"*
- It quoted the exact marker: `[X5-CHANNEL-TEST conv=187dd387-...]`
- The injected message does NOT appear as a visible step in `transcript_full.jsonl` but IS visible to the model

### Test B: invocationNum gating — ✅ WORKS

- `invocationNum` is accurately passed (0 for first invocation in a turn)
- Hook correctly returns `{}` for `invocationNum > 0` to avoid duplicate injection
- `systemMessage` persists across invocations within a turn (no need to re-inject)

### Test C: ephemeralMessage — ✅ WORKS (inferred)

- Hook returns ephemeralMessage alongside systemMessage
- ephemeralMessage is single-invocation scope (auto-excluded after PLANNER_RESPONSE)
- Ideal for per-invocation dynamic context that shouldn't accumulate

### `injectSteps` Discovery Level Findings (Corrected)

**Critical finding:** `injectSteps` works for top-level sessions but NOT for `invoke_subagent`:

| Discovery Level | Session Type | Hook Fires? | `injectSteps` Processed? |
|---|---|---|---|
| **Plugin** `<plugin>/hooks.json` | `agy --add-dir . -p` (CLI) | ✅ | ✅ |
| **Plugin** `<plugin>/hooks.json` | `invoke_subagent` (IDE) | ✅ | ❌ |
| **Workspace** `.agents/hooks.json` | `agy -p` (CLI) | ✅ | ✅ |
| **Workspace** `.agents/hooks.json` | `invoke_subagent` (IDE) | ✅ | ❌ |

**Key correction:** The earlier conclusion that plugin-level hooks don't support `injectSteps`
was wrong — the test setup omitted `--add-dir .`, preventing `.agents/plugins.json` from
being scanned and the workspace plugin hooks from being discovered.

**Transcript note:** `SYSTEM_MESSAGE` steps in `transcript_full.jsonl` omit the `content`
field. You cannot verify injection by grepping for message text — look for `SYSTEM_SDK` steps
instead. See [X5-injectSteps-investigation.md](X5-injectSteps-investigation.md) for details.

Expected transcript format per docs (when injection works):

| `injectSteps` Variant | Transcript `source` | Transcript `type` |
|---|---|---|
| `systemMessage` | `SYSTEM_SDK` | `SYSTEM_MESSAGE` |
| `ephemeralMessage` | `SYSTEM_SDK` | `EPHEMERAL_MESSAGE` |
| `userMessage` | `SYSTEM_SDK` | `USER_INPUT` |

### Test D: Direct hook output verification

```bash
$ echo '{"conversationId":"test","invocationNum":0}' | ./repo-memory hook antigravity PreInvocation --workspace .
{"injectSteps":[{"systemMessage":{"systemMessage":"[X5-CHANNEL-TEST conv=test] ...","metadata":{"source":"repo-memory-x5-spike"}}},{"ephemeralMessage":"[X5-CHANNEL-TEST conv=test] ..."}]}
```

## Message Type Reference

| Type | JSON Shape | Scope | Best For |
|---|---|---|---|
| `ephemeralMessage` | plain string | Single invocation | Per-invocation dynamic context, status reminders |
| `userMessage` | plain string | Persistent (until compaction) | Durable user-role instructions |
| `systemMessage` | nested object `{systemMessage, metadata}` | Persistent (until compaction) | Dossier delivery, system-level context |

## Supported Hook Events

`injectSteps` works in `SessionStart`, `PreInvocation`, and `PostInvocation`.

## Recommendations for PR-08 (Delivery)

1. **Use `systemMessage` via `PreInvocation` `injectSteps`** for dossier delivery. The mechanism works.
2. **Gate on `invocationNum == 0`** to avoid accumulating duplicate system messages.
3. **Use `ephemeralMessage`** for transient per-invocation context (e.g., "you are currently in a memory capture continuation").
4. **Consider `SessionStart`** for one-time session bootstrap context.
5. **CHECKPOINT survival** still needs testing — injected messages may or may not survive context compaction.
