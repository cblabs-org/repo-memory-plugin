# X5 Channels Experiment

**Date:** 2026-10-05
**Branch:** pr-06-v2-spike / WO3
**Methodology:** 
We built a prototype PreInvocation hook returning `systemMessage` and `userMessage` payloads to Antigravity. We used `invoke_subagent` to spawn several test agents in the enrolled repository and then inspected their `transcript_full.jsonl` files for the injected payloads (`[X5-CHANNEL-TEST ...]`).

## Results

### Test A: systemMessage injection
- **Action:** Spawend subagent to list three colors, returning `{"systemMessage": "..."}`.
- **Result:** The `systemMessage` **did not appear** in the subagent's transcript. The hook was successfully executed and logged the incoming payload (verified by patching the hook to write to a `/tmp/` file), but Antigravity ignored the `systemMessage` field.

### Test B: Persistence across turns
- **Action:** Spawend subagent with a multi-step task requiring tool calls.
- **Result:** Since `systemMessage` was ignored entirely, it did not persist. 

### Test C: userMessage
- **Action:** Modified the prototype to return both `systemMessage` and `userMessage`, and spawned a subagent.
- **Result:** Neither `systemMessage` nor `userMessage` appeared in the transcript. Antigravity appears to ignore both fields from the PreInvocation hook response.

### Test D: invocationNum tracking
- **Result:** `invocationNum` is accurately passed by Antigravity in the PreInvocation payload. For the initial turn, it is `0`.

## Analysis & Recommendations
The current platform mechanics for Antigravity do not support injecting messages via the `PreInvocation` hook using `systemMessage` or `userMessage` fields. The JSON response from the hook is silently ignored. 
For PR-08 (the delivery system), we cannot rely on `PreInvocation` returning a message. Alternative injection methods (such as modifying the prompt, or relying on `PreToolUse` fast-pathing) must be investigated.

## CHECKPOINT Testing
CHECKPOINT testing was not possible because the message injection itself failed on the initial invocation. If the platform is updated to support this feature in the future, we will need to re-run this spike to test CHECKPOINT survival.
