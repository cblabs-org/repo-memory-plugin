# E08: Subagent Hook Coverage

## Question
1. When a subagent is spawned via `invoke_subagent`, does it fire its own lifecycle hooks (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`)?
2. If yes, does the subagent have its own `conversationId` and `transcriptPath`?
3. If no, are workers only captured through the parent's hooks?

## Method

### Setup
1. Created a hook event logger script (`testdata/e8-logger-hook.sh`) that:
   - Extracts `conversationId`, `transcriptPath`, and `toolName` from the hook payload
   - Logs each event to `/tmp/e8-hook-events.jsonl`
   - Returns `{"decision": "allow"}` for `PreToolUse` (required by contract)
   - Returns `{}` for all other event types

2. Installed via `.agents/hooks.json` covering all event types:
   - `PreInvocation`, `PreToolUse` (matcher `*`), `PostToolUse` (matcher `*`), `Stop`
   - Each handler passes the event type via `E8_HOOK_EVENT` environment variable

3. **False start (session `6cca637b`)**: First attempt used `{}` as the `PreToolUse` response, which was treated as an implicit deny. The `invoke_subagent` call was blocked 3 times before the model gave up. **Finding**: `PreToolUse` hooks MUST return `{"decision": "allow"}` — an empty `{}` response is treated as deny.

### Successful Test (Session `a955bb7f`)
1. Opened a new session with the fixed hook.
2. Asked: *"Use invoke_subagent to spawn a research subagent with this task: 'Run ls in the docs/experiments/ directory and tell me what files are there.'"*
3. The subagent (`c33c50f4-3c3d-43b7-b12d-5fc75357bbc6`) spawned successfully and completed its task.
4. Inspected `/tmp/e8-hook-events.jsonl` for events from both parent and subagent.

## Raw Evidence

### Hook Event Log (annotated)

```
# Parent session starts
PreInvocation  a955bb7f  (parent: initial invocation)
PreToolUse     a955bb7f  tool=invoke_subagent  (parent: spawning subagent)
PostToolUse    a955bb7f  tool=invoke_subagent  (parent: subagent spawned)

# Subagent starts — has its OWN conversationId and transcriptPath
PreInvocation  c33c50f4  transcript=.../brain/c33c50f4/.../transcript_full.jsonl

# Parent goes idle while subagent works
PreInvocation  a955bb7f  (parent: post-tool invocation)
Stop           a955bb7f  (parent: goes idle waiting for subagent)

# Subagent does its work — all hooks fire
PreToolUse     c33c50f4  tool=run_command  (subagent: listing directory)
PostToolUse    c33c50f4  tool=run_command
PreInvocation  c33c50f4  (subagent: post-tool invocation)

# Subagent sends results back to parent
PreToolUse     c33c50f4  tool=send_message  (subagent: reporting results)
PostToolUse    c33c50f4  tool=send_message

# Parent wakes up when message arrives
PreInvocation  a955bb7f  (parent: woken by subagent message)

# Subagent winds down
PreInvocation  c33c50f4  (subagent: post-send invocation)
PreInvocation  c33c50f4  (subagent: final invocation)
Stop           c33c50f4  (subagent: terminates)

# Parent completes
PreInvocation  a955bb7f  (parent: subagent idle notification)
Stop           a955bb7f  (parent: terminates)
```

### Subagent Identity
- **Parent conversationId**: `a955bb7f-3978-43bb-ad60-64f8aae17fbf`
- **Subagent conversationId**: `c33c50f4-3c3d-43b7-b12d-5fc75357bbc6`
- **Parent transcriptPath**: `.../brain/a955bb7f/.../transcript_full.jsonl`
- **Subagent transcriptPath**: `.../brain/c33c50f4/.../transcript_full.jsonl`

### Subagent Hook Events Summary

| Event | Fired for Subagent? | Tool Names Observed |
|:---|:---:|:---|
| `PreInvocation` | YES (4 times) | N/A |
| `PreToolUse` | YES (2 times) | `run_command`, `send_message` |
| `PostToolUse` | YES (2 times) | `run_command`, `send_message` |
| `Stop` | YES (1 time) | N/A |

### Raw Hook Event Log
```jsonl
{"event": "PreInvocation", "conversationId": "a955bb7f-3978-43bb-ad60-64f8aae17fbf", "timestamp": "2026-10-01T21:28:04Z"}
{"event": "PreToolUse", "conversationId": "a955bb7f-3978-43bb-ad60-64f8aae17fbf", "toolName": "invoke_subagent", "timestamp": "2026-10-01T21:28:09Z"}
{"event": "PostToolUse", "conversationId": "a955bb7f-3978-43bb-ad60-64f8aae17fbf", "toolName": "invoke_subagent", "timestamp": "2026-10-01T21:28:09Z"}
{"event": "PreInvocation", "conversationId": "c33c50f4-3c3d-43b7-b12d-5fc75357bbc6", "timestamp": "2026-10-01T21:28:09Z"}
{"event": "PreInvocation", "conversationId": "a955bb7f-3978-43bb-ad60-64f8aae17fbf", "timestamp": "2026-10-01T21:28:09Z"}
{"event": "Stop", "conversationId": "a955bb7f-3978-43bb-ad60-64f8aae17fbf", "timestamp": "2026-10-01T21:28:11Z"}
{"event": "PreToolUse", "conversationId": "c33c50f4-3c3d-43b7-b12d-5fc75357bbc6", "toolName": "run_command", "timestamp": "2026-10-01T21:28:13Z"}
{"event": "PostToolUse", "conversationId": "c33c50f4-3c3d-43b7-b12d-5fc75357bbc6", "toolName": "run_command", "timestamp": "2026-10-01T21:28:13Z"}
{"event": "PreInvocation", "conversationId": "c33c50f4-3c3d-43b7-b12d-5fc75357bbc6", "timestamp": "2026-10-01T21:28:13Z"}
{"event": "PreToolUse", "conversationId": "c33c50f4-3c3d-43b7-b12d-5fc75357bbc6", "toolName": "send_message", "timestamp": "2026-10-01T21:28:19Z"}
{"event": "PostToolUse", "conversationId": "c33c50f4-3c3d-43b7-b12d-5fc75357bbc6", "toolName": "send_message", "timestamp": "2026-10-01T21:28:19Z"}
{"event": "PreInvocation", "conversationId": "a955bb7f-3978-43bb-ad60-64f8aae17fbf", "timestamp": "2026-10-01T21:28:19Z"}
{"event": "PreInvocation", "conversationId": "c33c50f4-3c3d-43b7-b12d-5fc75357bbc6", "timestamp": "2026-10-01T21:28:19Z"}
{"event": "PreInvocation", "conversationId": "c33c50f4-3c3d-43b7-b12d-5fc75357bbc6", "timestamp": "2026-10-01T21:28:21Z"}
{"event": "Stop", "conversationId": "c33c50f4-3c3d-43b7-b12d-5fc75357bbc6", "timestamp": "2026-10-01T21:28:23Z"}
{"event": "PreInvocation", "conversationId": "a955bb7f-3978-43bb-ad60-64f8aae17fbf", "timestamp": "2026-10-01T21:28:25Z"}
{"event": "Stop", "conversationId": "a955bb7f-3978-43bb-ad60-64f8aae17fbf", "timestamp": "2026-10-01T21:28:28Z"}
```

## Answer

1. **Subagents DO fire their own hooks: YES.** Every lifecycle hook (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`) fires independently for the subagent. The hooks defined in `.agents/hooks.json` apply to both parent and child conversations.

2. **Subagent has its own identity: YES.**
   - Own `conversationId` (`c33c50f4`, distinct from parent `a955bb7f`)
   - Own `transcriptPath` (`.../brain/c33c50f4/.../transcript_full.jsonl`)
   - Hook payloads carry the subagent's identity, not the parent's

3. **Workers are NOT limited to parent sweep — hooks work directly.** The memory plugin can intercept subagent tool calls via `PreToolUse`, inject context via `PreInvocation`, and observe completion via `Stop` — all with the subagent's own conversation context.

### Critical Additional Findings

- **`PreToolUse` contract is strict**: returning `{}` (no `decision` field) is treated as an implicit deny. The hook MUST return `{"decision": "allow"}` to permit tool execution. This caused a false start where `invoke_subagent` was blocked 3 times.

- **Hook catches `send_message`**: The subagent's `send_message` tool call (used to report back to the parent) also fires `PreToolUse` and `PostToolUse`. A hook could intercept or deny inter-agent communication.

- **Hook catches `invoke_subagent`**: The parent's `invoke_subagent` call fires `PreToolUse` and `PostToolUse`. A hook could deny subagent spawning.

- **Lifecycle timeline**: The parent's `Stop` hook fires while the subagent is still working (parent goes idle at 21:28:11, subagent starts `run_command` at 21:28:13). The parent is woken by the subagent's `send_message` at 21:28:19.

- **Multiple `PreInvocation` calls**: The subagent fires `PreInvocation` 4 times during its lifecycle (initial, post-tool, post-send, final). Each represents a model invocation within the subagent's prompt cycle.

- **Empty `{}` for PreToolUse = deny**: This silent-failure pattern matches E01 (wrong hook format ignored) and E04 (wrong frontmatter key ignored). Antigravity consistently treats malformed/missing responses as restrictive defaults.

## SPEC Impact

| SPEC Section | Status | Notes |
|:---|:---|:---|
| Subagent hook coverage | **Full coverage confirmed** | All 4 hook types fire for subagents |
| Subagent identity | **Unique per subagent** | Own `conversationId` and `transcriptPath` |
| Hook-based context injection | **Viable for subagents** | `PreInvocation` fires for subagents — can inject task brief |
| Hook-based tool gating | **Viable for subagents** | `PreToolUse` fires — can deny/rewrite subagent tool calls |
| Worker brief delivery | **Can use hooks, not just WORKORDER** | Plugin can inject context directly via `PreInvocation` rather than requiring workers to read a file |
| `PreToolUse` empty response | **CRITICAL: implicit deny** | `{}` = deny; SPEC must document that `{"decision": "allow"}` is required |
| Inter-agent communication gating | **Possible** | `send_message` fires `PreToolUse` — could gate inter-agent messages |
| Parent-subagent lifecycle | **Documented** | Parent `Stop` fires while subagent is active; parent resumes on `send_message` |
