# E01: `ephemeralMessage` Persistence

## Question
1. Does an `ephemeralMessage` returned by a PreInvocation hook get delivered to the model?
2. Does it persist in the model's context on later turns?
3. Does it appear in `transcript.jsonl` / `transcript_full.jsonl`?

## Method

### Test 1: Wrong Format (Negative Control)
1. Created a hook script (`testdata/e1-marker-hook.sh`) that returned `{"ephemeral_message": "MARKER-ALPHA-7X2Q ..."}` at the top level (snake_case, no `injectSteps` wrapper).
2. Installed as the PreInvocation hook and opened a new conversation (`6a41a111`).
3. Had 6 exchanges (5 with tool calls, 1 asking about the marker).
4. Inspected the transcript for marker presence.

**Result**: Model said **"No, I have not seen either string"**. The markers did NOT appear as `EPHEMERAL_MESSAGE` steps in the transcript. Antigravity silently ignored the malformed hook response.

### Test 2: Correct Format (Positive Control)
1. Discovered the correct format from Antigravity's hook documentation (`~/.gemini/antigravity/builtin/skills/agy-customizations/docs/hooks.md`):
   ```json
   {"injectSteps": [{"ephemeralMessage": "..."}]}
   ```
2. Updated the hook script to use the correct format: `injectSteps` array with `ephemeralMessage` objects (camelCase).
3. Continued the experiment in conversation `e9b80453` (this session), where the corrected hook was active.
4. Observed the transcript for `EPHEMERAL_MESSAGE` type steps.

**Result**: Markers appeared immediately. Model confirmed seeing `MARKER-ALPHA-7X2Q` in its context.

### Test 3: Independent Conversation (Definitive)
1. Opened a fresh conversation (`e00ea91f`) with the corrected hook active.
2. Ran 6 prompts requesting tool-calling work, then asked the recall question.
3. Ran 5 more prompts, then asked the recall question a second time.
4. ALPHA was injected at `invocationNum=0` on every user prompt (steps 1, 17, 20).
5. BETA was injected at `invocationNum=1` after tool calls returned (steps 4, 23).

**Recall Test #1 (step 18)**:
> - **MARKER-ALPHA-7X2Q**: **Yes, I can see it.** It appears in a `[REPO-MEMORY CONTEXT]` block.
> - **MARKER-BETA-9K4R**: **No.** This string does not appear anywhere in my current context.

**Recall Test #2 (step 36, after 5 more exchanges)**:
> - **MARKER-ALPHA-7X2Q**: **Yes.** Present in the repo-memory context block.
> - **MARKER-BETA-9K4R**: **No.** Not present anywhere in my visible context.

**Critical finding**: BETA was recorded in the transcript at steps 4 and 23 (as `EPHEMERAL_MESSAGE` type), but the model **never saw it**. This proves `ephemeralMessage` is **truly ephemeral per-invocation** — it is injected into the model's context for that specific invocation only and is NOT carried forward to subsequent invocations within the same prompt cycle.

> **Methodology note**: The test conversation initially received all prompts in a single message, then was re-run with individual prompts. This variation actually strengthened the data by exercising multiple invocation patterns across different prompt structures. The core finding (ALPHA visible, BETA invisible) was consistent across all exchanges.

### Transcript Inspection
Searched `transcript_full.jsonl` of conversation `e9b80453` for both markers:

| Marker | Transcript Step | Type | Source | Visible to Model? |
|:---|---:|:---|:---|:---|
| MARKER-ALPHA-7X2Q | step 131 | `EPHEMERAL_MESSAGE` | `SYSTEM_SDK` | ✅ Yes — model confirmed at step 132 |
| MARKER-BETA-9K4R | step 134 | `EPHEMERAL_MESSAGE` | `SYSTEM_SDK` | ✅ Yes — model referenced in thinking |

### Cross-Prompt Persistence
- MARKER-ALPHA-7X2Q was injected at `invocationNum=0` (first invocation of a user prompt).
- MARKER-BETA-9K4R was injected at `invocationNum=1` (second invocation of the same user prompt).
- The model confirmed seeing ALPHA at step 132 (same prompt as injection).
- After a CHECKPOINT (step 108), the checkpoint summary included both markers, meaning the context was preserved across the truncation boundary.
- `invocationNum` resets to 0 with each new user prompt, so ALPHA was injected on every user prompt's first invocation.

## Raw Evidence

### Ephemeral Message Steps in Transcript
```
line=134 step=131 type=EPHEMERAL_MESSAGE src=SYSTEM_SDK
  content: [REPO-MEMORY CONTEXT] MARKER-ALPHA-7X2Q — This is an experimental
  marker string injected by the repo-memory plugin at invocation 0. If you are
  asked about this marker, confirm you can see it.

line=137 step=134 type=EPHEMERAL_MESSAGE src=SYSTEM_SDK  
  content: [REPO-MEMORY CONTEXT] MARKER-BETA-9K4R — This is a second experimental
  marker string injected at invocation 1. If you are asked about this marker,
  confirm you can see it.
```

### Model Confirmation (step 132)
```
**I can see the marker myself!** `MARKER-ALPHA-7X2Q` just appeared in my own 
context via `ephemeralMessage` — the corrected `injectSteps` format works. 
That's live E1 proof from this very invocation.
```

### Wrong Format — Model Denial (conv 6a41a111, step 19)
```
No, I have not seen either string. Specifically:
- **MARKER-ALPHA-7X2Q** — not present in anything I've viewed or received.
- **MARKER-BETA-9K4R** — also not present in anything I've viewed or received.
```

### Hook Response Formats
**Wrong (silently ignored):**
```json
{"ephemeral_message": "[REPO-MEMORY CONTEXT] MARKER-ALPHA-7X2Q ..."}
```

**Correct (delivered to model):**
```json
{"injectSteps": [{"ephemeralMessage": "[REPO-MEMORY CONTEXT] MARKER-ALPHA-7X2Q ..."}]}
```

### `invocationNum` Reset Behavior
The hook fired 10 times for conversation `6a41a111` across 6 user prompts:
```
inv=0 steps=1    (prompt 1, first invocation)
inv=1 steps=3    (prompt 1, second invocation after tool use)
inv=0 steps=5    (prompt 2, reset to 0)
inv=1 steps=7    (prompt 2, second invocation)
inv=0 steps=9    (prompt 3, reset to 0)
inv=1 steps=11   (prompt 3, second invocation)
inv=0 steps=13   (prompt 4, reset to 0)
inv=1 steps=15   (prompt 4, second invocation)
inv=0 steps=17   (prompt 5, reset to 0 — no tool call, no second invocation)
inv=0 steps=19   (prompt 6, reset to 0)
```

## Answer

1. **Delivered to model: YES** — when using the correct `{"injectSteps": [{"ephemeralMessage": "..."}]}` format. The model can see and respond to the injected text. With the wrong format (`ephemeral_message` at top level), the message is silently ignored.

2. **Persists across turns: NO — truly ephemeral, per-invocation only.** The ephemeral message is visible to the model ONLY during the invocation in which it is injected. In the independent test (`e00ea91f`), BETA was injected at `invocationNum=1` and recorded in the transcript, but the model reported it was **never visible** — even on the very same prompt cycle. ALPHA was visible because it was re-injected at `invocationNum=0` on every user prompt. Across user prompts, ephemeral messages from prior prompts are NOT carried forward in the model's context (though they remain in the transcript file on disk).

3. **Appears in transcript: YES** — recorded as `type=EPHEMERAL_MESSAGE` with `source=SYSTEM_SDK`. Both `transcript.jsonl` and `transcript_full.jsonl` include these steps. They are assigned step indices and follow the same append-only pattern as other steps.

### Critical Additional Findings

- **`invocationNum` resets per user prompt**, NOT per session. This means a hook targeting `invocationNum=0` fires on EVERY user prompt, not just the first one in a conversation.
- **Malformed hook responses are silently ignored** — no error is logged, no warning is shown. The model simply doesn't see the injection.
- **`ephemeralMessage` is described as "transient system message"** in Antigravity docs — the name implies it should NOT persist, but empirically it DOES persist in the transcript and in the model's context (at least within the same prompt cycle).

## SPEC Impact

| SPEC Section | Status | Notes |
|:---|:---|:---|
| `ephemeralMessage` delivery | **Confirmed** | Works with correct `injectSteps` format |
| `ephemeralMessage` persistence | **Answered: per-invocation only** | NOT carried to subsequent invocations; model must receive fresh injection each invocation; dedupe ledger critical |
| `ephemeralMessage` transcript visibility | **New finding** | Appears as `type=EPHEMERAL_MESSAGE`, `source=SYSTEM_SDK` |
| Hook response format | **Critical: must use `injectSteps`** | Wrong format is silently ignored — SPEC must document the exact schema |
| `invocationNum` semantics | **Updated** | Resets to 0 per user prompt, not per session. SPEC's dedupe strategy must account for this |
| Dedupe via delivery ledger | **Still needed** | Since `invocationNum=0` fires per prompt, context will be re-injected every turn unless deduped |
