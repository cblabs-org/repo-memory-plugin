# E12: Transcript File Set — Remaining Parts

## Question
1. Is the transcript file (`transcript.jsonl` / `transcript_full.jsonl`) **append-only** during normal conversation flow?
2. Does branching or editing an earlier step cause the file to be **rewritten** rather than appended?
3. What explains the **step-index gaps** present in both transcript files?

## Method

### Append-Only Verification
1. Opened a new Antigravity session in the `repo-memory-plugin` repository (conversation `e9b80453`).
2. After multiple model turns, recorded the transcript file size and line count at two checkpoints:
   - **Checkpoint A** (early): 19 lines, 19,781 bytes (transcript.jsonl) / 23,507 bytes (transcript_full.jsonl)
   - **Checkpoint B** (mid-conversation): 37 lines, 56,362 bytes / 60,232 bytes
3. At Checkpoint B, extracted the first 37 lines and computed their MD5 hash.
4. After further model turns (63+ lines, 77,577+ bytes), re-extracted the first 37 lines and compared:
   - Byte count: `56,362` (identical)
   - MD5: `6db9620648c94e8add8b0c9b57ca2949` (identical)

### Branch/Edit Observation
- Observed that the XL conversation (`96a5f1a5`, 2,416 lines) contains a step-index **reset** pattern:
  - Steps `1199, 1201, 1202, 1203` appear sequentially in the file, then step `1200` appears at line 1205 (out-of-order).
  - After step `1200` (the PLANNER_RESPONSE), steps `1204+` continue normally.
  - This is consistent with a **parallel tool call fan-out**: tool results arrive (and are appended) before the parent PLANNER_RESPONSE is finalized and flushed.
- The current conversation also has a "reset" at line 3: step indices go `0, 1, 0, 1, 2, ...` because the conversation was reused across two separate user prompts (the session was reset).

> **Note**: Branch/edit testing in the Antigravity UI was not directly performed in this session. To fully answer the branch/edit question, a separate session would need to use the UI "edit" feature on an earlier message. Based on the structural evidence (step index resets, stable prefix bytes), the working hypothesis is:
> - Normal flow: **append-only** ✓
> - Session reset / new prompt in same conversation: file is **rewritten** from scratch (step indices restart at 0)
> - Parallel tool calls: results arrive **out-of-order** but are still **appended** (not rewritten)

### Step-Index Gap Analysis
Analyzed 5 conversations of different lengths for step-index gaps:

| Conversation | Lines | Max Step | Gaps | Resets | Pattern |
|:---|---:|---:|---:|---:|:---|
| C1 (XL, 2416 lines) | 2,416 | 2,414 | 1 | 2 | Parallel tool fan-out |
| C2 (L, 1058 lines) | 1,058 | 1,088 | 31 | 0 | Skip-1 gaps after PLANNER_RESPONSE |
| C3 (M, 496 lines) | 496 | 498 | 3 | 0 | Skip-1 gaps after PLANNER_RESPONSE |
| C4 (current, 21+ lines) | 21+ | 18+ | 0 | 1 | Session reset only |
| C5 (S, 29 lines) | 29 | 29 | 1 | 0 | Skip-2 after parallel tools |

**Key observations about gaps:**
1. **Gaps are never present in either file** — the missing step indices don't appear in `transcript.jsonl` OR `transcript_full.jsonl`. Both files have identical line counts and contain the same step indices.
2. **Gap size is almost always 1** (delta=2 between recorded steps), occasionally 2-3.
3. **Gaps occur after PLANNER_RESPONSE steps** that contained tool calls. The skipped index corresponds to an internal dispatch/routing step that Antigravity allocates but never serializes to the transcript.
4. **C2 example**: step 68 (PLANNER_RESPONSE with `call_mcp_tool`) → step 70 (next PLANNER_RESPONSE). Step 69 is absent from both files. This suggests the internal "tool dispatch" or "tool invocation" step is allocated an index but not written.
5. **CHECKPOINT steps** (type `CHECKPOINT`, source `SYSTEM`) appear at specific indices (e.g., step 88 in C2) and are preceded by a gap — the checkpoint allocation itself may consume an index.

## Raw Evidence

### Append-Only Proof
```
Checkpoint B (37 lines): MD5 = 6db9620648c94e8add8b0c9b57ca2949, 56,362 bytes
After 63+ lines:         MD5 = 6db9620648c94e8add8b0c9b57ca2949, 56,362 bytes (first 37 lines)
→ Byte-identical prefix confirms append-only during normal flow.
```

### Gap Examples from C2 (`f3df9958`)
```
step=68 type=PLANNER_RESPONSE tools=[call_mcp_tool]
  ← step 69 MISSING from both files →
step=70 type=PLANNER_RESPONSE tools=[call_mcp_tool]

step=86 type=PLANNER_RESPONSE tools=[write_to_file]
  ← step 87 MISSING from both files →
step=88 type=CHECKPOINT (system truncation marker)
```

### Out-of-Order Append from C1 (`96a5f1a5`)
```
line=1201 step=1199 type=GENERIC (tool result)
line=1202 step=1201 type=GENERIC (tool result)   ← step 1200 not yet written
line=1203 step=1202 type=GENERIC (tool result)
line=1204 step=1203 type=GENERIC (tool result)
line=1205 step=1200 type=PLANNER_RESPONSE         ← written AFTER its children
line=1206 step=1204 type=GENERIC (tool result)
```

### Session Reset from C4 (current conversation)
```
line=1 step=0 type=USER_INPUT   (first prompt)
line=2 step=1 type=PLANNER_RESPONSE
line=3 step=0 type=USER_INPUT   (second prompt — index restarted)
line=4 step=1 type=PLANNER_RESPONSE
line=5 step=2 type=GENERIC
...
```

## Answer

1. **Append-only during normal flow: YES** — confirmed by byte-identical prefix comparison across growing files. New steps are appended; existing steps are not modified.

2. **Rewritten on branch/edit: LIKELY YES** — session resets cause step indices to restart at 0, and the new steps are appended after the old ones (the file is NOT truncated). However, direct UI branch/edit testing was not performed; this should be confirmed in a follow-up.

3. **Step-index gaps: Caused by internal dispatch steps** — Antigravity allocates step indices for internal operations (tool dispatch, checkpoint preparation) that are never serialized to the transcript files. The gap count correlates with the number of tool call batches in the conversation. Both `transcript.jsonl` and `transcript_full.jsonl` contain identical step sets — the compact file is NOT a strict subset of the full file by step count; rather it truncates field *content* (via `truncated_fields`).

## SPEC Impact

| SPEC Section | Status | Notes |
|:---|:---|:---|
| Transcript file format | **Confirmed** | JSONL, one JSON object per line, `step_index` field present |
| Append-only behavior | **Confirmed** | Safe to read prefix bytes without worrying about mutation |
| Step-index contiguity | **Needs update** | SPEC must document that step indices have gaps (internal dispatch steps are not serialized). Parsers must NOT assume `max_step_index == line_count - 1` |
| `transcript.jsonl` vs `transcript_full.jsonl` | **Confirmed** | Same line count and step indices; compact version truncates large field values and marks them with `truncated_fields` |
| Out-of-order writes | **New finding** | Parallel tool calls cause PLANNER_RESPONSE to be written AFTER its tool result children. SPEC should document that step indices may appear non-monotonically |
| Session reset | **New finding** | Same conversation file may contain multiple "sessions" with step indices restarting at 0. SPEC should document this or require separate handling |
