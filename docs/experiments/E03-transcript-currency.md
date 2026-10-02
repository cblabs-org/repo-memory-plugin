# E03: Transcript Currency at Hooks

## Question
1. At the moment `PreInvocation` fires, how current is the transcript file? Does `initialNumSteps` in the hook payload match the actual line count of the transcript?
2. At the moment `PreToolUse` fires, does the transcript include the current turn's reasoning (`thinking` and tool call plan)?
3. What is the lag (if any) between transcript writes and hook execution?

## Method

### Data Source
Used existing probe logs from a recent Antigravity session on a sibling project (`bug-fixer-loop-plugin`, conversation `00720941`, repo ID `01d9c267...`). The probe hook was installed with `--probe` flag, which:
1. Records the raw hook payload to a timestamped JSON file
2. Snapshots the transcript file at the path given in the payload

This gives us the transcript state **at the exact moment** the hook fires.

### PreInvocation Analysis
Two PreInvocation hook fires were captured:

| Invocation | `initialNumSteps` | Transcript Snapshot Lines | Snapshot Max Step | Match? |
|:---|---:|---:|---:|:---|
| #0 (`invocationNum=0`) | 67 | 67 | 66 | ✅ Perfect |
| #1 (`invocationNum=1`) | 70 | 70 | 69 | ✅ Perfect |

The transcript snapshot at hook fire time contained **exactly** `initialNumSteps` lines, with step indices `0` through `initialNumSteps - 1`.

### Stop Hook Analysis
The Stop hook fired after invocation #1 completed:
- Transcript snapshot: 71 lines, max step index 70
- The final line was a `PLANNER_RESPONSE` step — the model's final response was already written before Stop fired.
- `terminationReason`: `NO_TOOL_CALL` (the model stopped voluntarily)

### PreToolUse Analysis
PreToolUse probes were subsequently captured from two conversations in the `repo-memory-plugin` workspace:

**Conversation `6a41a111` (test session), PreToolUse at `stepIdx=2`:**
- Transcript snapshot: **2 lines** (steps 0-1)
- Step 0: `USER_INPUT` (the user's prompt)
- Step 1: `PLANNER_RESPONSE` with `thinking=True` and `tools=['run_command']`
- **The current turn's reasoning (thinking + tool call plan) IS in the transcript** at PreToolUse time

**Conversation `e9b80453` (main session), PreToolUse at `stepIdx=99`:**
- Transcript snapshot: **99 lines** (steps 0-96, with gaps)
- Last step (96): `PLANNER_RESPONSE` with `thinking=True` and `tools=['write_to_file']`
- Confirms the pattern: the PLANNER_RESPONSE that produced the tool call is already flushed to disk before PreToolUse fires

**PreToolUse payload includes:**
- `stepIdx`: the step index of the tool call being evaluated
- `toolCall`: full tool call details including `name` and `args`
- `conversationId`, `modelName`, `transcriptPath`, `workspacePaths`

### Timeline Reconstruction
```
T+0.000s  PreInvocation #0 fires (initialNumSteps=67, transcript=67 lines)
          → Model processes, produces steps 67-69
T+13.06s  PreInvocation #1 fires (initialNumSteps=70, transcript=70 lines)  
          → Model processes, produces step 70
T+17.49s  Stop fires (transcript=71 lines, step 70 = PLANNER_RESPONSE)
```

The gap between events confirms real-time processing: ~13s for invocation #0, ~4.5s for invocation #1.

## Raw Evidence

### PreInvocation #0 Payload
```json
{
    "artifactDirectoryPath": "/Users/mcneillm/.gemini/antigravity/brain/00720941-a1f8-4b01-a3e3-4594859e844a",
    "conversationId": "00720941-a1f8-4b01-a3e3-4594859e844a",
    "initialNumSteps": 67,
    "invocationNum": 0,
    "modelName": "gemini-3.8-flash-tiered",
    "transcriptPath": "/Users/mcneillm/.gemini/antigravity/brain/00720941-a1f8-4b01-a3e3-4594859e844a/.system_generated/logs/transcript_full.jsonl",
    "workspacePaths": [
        "/Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin"
    ]
}
```

### PreInvocation #1 Payload
```json
{
    "artifactDirectoryPath": "/Users/mcneillm/.gemini/antigravity/brain/00720941-a1f8-4b01-a3e3-4594859e844a",
    "conversationId": "00720941-a1f8-4b01-a3e3-4594859e844a",
    "initialNumSteps": 70,
    "invocationNum": 1,
    "modelName": "gemini-3.8-flash-tiered",
    "transcriptPath": "/Users/mcneillm/.gemini/antigravity/brain/00720941-a1f8-4b01-a3e3-4594859e844a/.system_generated/logs/transcript_full.jsonl",
    "workspacePaths": [
        "/Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin"
    ]
}
```

### Stop Payload
```json
{
    "artifactDirectoryPath": "/Users/mcneillm/.gemini/antigravity/brain/00720941-a1f8-4b01-a3e3-4594859e844a",
    "conversationId": "00720941-a1f8-4b01-a3e3-4594859e844a",
    "error": "",
    "executionNum": 0,
    "fullyIdle": false,
    "modelName": "gemini-3.8-flash-tiered",
    "terminationReason": "NO_TOOL_CALL",
    "transcriptPath": "/Users/mcneillm/.gemini/antigravity/brain/00720941-a1f8-4b01-a3e3-4594859e844a/.system_generated/logs/transcript_full.jsonl",
    "workspacePaths": [
        "/Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin"
    ]
}
```

### File Sizes
```
probe_PreInvocation_..._transcript.jsonl  251,252 bytes (67 lines)
probe_PreInvocation_..._transcript.jsonl  261,936 bytes (70 lines)
probe_Stop_..._transcript.jsonl           262,973 bytes (71 lines)
```

## Answer

1. **PreInvocation transcript currency: ZERO LAG** — `initialNumSteps` exactly matches the transcript file's line count at the moment the hook fires. The transcript is fully current — all previous turns' steps are flushed to disk before PreInvocation executes. The transcript path provided points to `transcript_full.jsonl` (not the compact version).

2. **PreToolUse transcript currency: ZERO LAG, INCLUDES CURRENT TURN'S THINKING** — The transcript snapshot at PreToolUse time contains the PLANNER_RESPONSE step that produced the tool call, including the model's `thinking` and the tool call plan. The hook can read the model's reasoning before deciding whether to allow or deny the tool call.

3. **Transcript lag: NONE DETECTED** — The transcript file on disk matches the expected line count at every observed hook event. Writes appear to be synchronous: Antigravity flushes the transcript before invoking hooks.

### Additional Findings
- The `transcriptPath` in hook payloads points to `transcript_full.jsonl` (not `transcript.jsonl`). Hooks receive the **full** (untruncated) transcript path.
- `invocationNum` increments across hook fires within a session, starting at 0.
- The Stop hook payload includes `terminationReason`, `fullyIdle`, and `error` fields not present in PreInvocation.
- `executionNum` (in Stop) and `invocationNum` (in PreInvocation) appear to track different counters.

## SPEC Impact

| SPEC Section | Status | Notes |
|:---|:---|:---|
| Hook transcript currency | **Confirmed (PreInvocation)** | Zero lag — transcript is fully current at PreInvocation time |
| Hook transcript currency | **Confirmed (PreToolUse)** | Zero lag; PLANNER_RESPONSE with thinking is flushed before hook fires |
| `initialNumSteps` semantics | **Confirmed** | Equals transcript line count, NOT max step_index + 1 (these may differ due to gaps) |
| `transcriptPath` target | **New finding** | Always points to `transcript_full.jsonl`, not compact `transcript.jsonl` |
| Hook payload schema | **Documented** | PreInvocation includes `initialNumSteps`, `invocationNum`, `modelName`, `conversationId`, `artifactDirectoryPath`, `workspacePaths` |
| Stop payload schema | **Documented** | Includes `terminationReason`, `fullyIdle`, `error`, `executionNum` — additional fields vs PreInvocation |
