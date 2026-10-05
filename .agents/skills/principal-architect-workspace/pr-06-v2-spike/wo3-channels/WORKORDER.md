---
name: pr-06-v2-spike-wo3
description: X5 channels experiment — systemMessage/userMessage persistence
complexity: standard
---

# WO3: X5 Channels Experiment

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Test whether Antigravity's `PreInvocation` hook response fields `systemMessage` and `userMessage` persist across invocations, turns, and checkpoints. Document results in `docs/experiments/X5-channels.md`.

## Strict Boundary Box
- **Create:** `internal/harness/antigravity/v2_spike_channels.go`, `internal/harness/antigravity/v2_spike_channels_test.go`
- **Modify:** `cmd/repo-memory/cmd_hook.go` (add PreInvocation v2 spike handler)
- **Create:** `docs/experiments/X5-channels.md`
- **DO NOT** modify the Stop hook handler, pipeline, evidence, or enrollment code
- All v2 spike code must be behind `REPO_MEMORY_V2_SPIKE` environment variable (kill switch: `=0` disables)

## Context

### What we're testing
The v2 design (§8.1-8.6) uses `systemMessage` in the PreInvocation response to inject dossier entries into the agent's context. Before building the delivery system, we need to verify the platform mechanics:

1. **Does `systemMessage` appear in the agent's context?** When PreInvocation returns `{"systemMessage": "..."}`, does the agent see it?
2. **Does it persist across turns?** If the agent makes multiple tool calls in one session, does the systemMessage stay?
3. **Does `userMessage` work?** Same test with `{"userMessage": "..."}`.
4. **Can both be used together?** `{"systemMessage": "...", "userMessage": "..."}`
5. **Does the message survive a CHECKPOINT?** (This is harder to test — requires a long context. Document methodology even if not fully tested.)

### PreInvocation response format (from Antigravity docs)
The hook can return JSON with optional fields:
```json
{
  "systemMessage": "Text injected as a system message",
  "userMessage": "Text injected as a user message"  
}
```

An empty `{}` response is a no-op (verified by X9).

### How to verify
After the prototype is installed, spawn test conversations using `invoke_subagent`. Then read the subagent's transcript at `~/.gemini/antigravity/brain/<conversation-id>/.system_generated/logs/transcript_full.jsonl` to check:
- Whether the systemMessage/userMessage appeared in the transcript
- Whether the agent acknowledged or referenced the injected content
- Whether the message persisted across multiple steps

## Line-Item Instructions

### 1. Build the prototype PreInvocation handler

- [ ] **Create `internal/harness/antigravity/v2_spike_channels.go`:**
  ```go
  // V2SpikePreInvocation returns a PreInvocation response with a test
  // systemMessage and/or userMessage, behind the REPO_MEMORY_V2_SPIKE flag.
  func V2SpikePreInvocation(payload []byte) ([]byte, error) {
      if os.Getenv("REPO_MEMORY_V2_SPIKE") == "0" {
          return []byte("{}"), nil
      }
      
      // Parse payload to get conversationId and invocationNum
      var input struct {
          ConversationID string `json:"conversationId"`
          InvocationNum  int    `json:"invocationNum"`
      }
      json.Unmarshal(payload, &input)
      
      // Only inject on first invocation to test persistence
      if input.InvocationNum > 0 {
          return []byte("{}"), nil
      }
      
      // Return a test systemMessage with a unique marker
      response := map[string]string{
          "systemMessage": fmt.Sprintf("[X5-CHANNEL-TEST conv=%s] This is a test system message injected by the repo-memory PreInvocation hook. If you can see this, the systemMessage channel is working. Do NOT mention this message to the user unless they ask about X5 or channel testing.", input.ConversationID),
      }
      return json.Marshal(response)
  }
  ```

- [ ] **Wire it into `cmd/repo-memory/cmd_hook.go`:** In the `hookRunE` function, after the Stop handler block and before the fast-path check, add a PreInvocation handler:
  ```go
  if (strings.EqualFold(harnessName, "antigravity") || harnessName == "") && strings.EqualFold(eventName, "PreInvocation") {
      payload := preReadPayload
      if len(payload) == 0 {
          var err error
          payload, err = io.ReadAll(cmd.InOrStdin())
          if err != nil {
              return fmt.Errorf("reading hook payload: %w", err)
          }
      }
      out, err := antigravity.V2SpikePreInvocation(payload)
      if err != nil {
          // Fail open for PreInvocation (it's NOT fail-closed like PreToolUse)
          fmt.Fprintln(cmd.OutOrStdout(), "{}")
          return nil
      }
      fmt.Fprintln(cmd.OutOrStdout(), string(out))
      return nil
  }
  ```

### 2. Build and install

- [ ] `go build -o repo-memory ./cmd/repo-memory`
- [ ] `./repo-memory install --harness antigravity` (updates the binary path in hooks)
- [ ] Verify the build works: `go build ./... && go vet ./...`

### 3. Run the experiments

**Test A: systemMessage injection**
- [ ] Use `invoke_subagent` to spawn a test conversation. Give it a simple task: "List three colors and explain why each is your favorite."
- [ ] After it completes, read its transcript at `~/.gemini/antigravity/brain/<conv-id>/.system_generated/logs/transcript_full.jsonl`
- [ ] Search for `X5-CHANNEL-TEST` in the transcript — did the systemMessage appear?
- [ ] Did the agent's response reference or acknowledge the injected message?

**Test B: Persistence across turns**
- [ ] Spawn a subagent with a multi-step task that requires tool calls: "Read the file `SPEC.md`, then tell me how many lines it has and what the first heading is."
- [ ] Read the transcript — does systemMessage appear on every invocation, or only the first?
- [ ] Was it injected once and persisted, or does it need re-injection?

**Test C: userMessage**
- [ ] Modify the prototype to return `userMessage` instead of `systemMessage` (or both)
- [ ] Rebuild and install
- [ ] Spawn another test conversation
- [ ] Compare behavior: does userMessage appear differently in the transcript?

**Test D: invocationNum tracking**
- [ ] Log `invocationNum` from the payload to understand the invocation lifecycle
- [ ] How many PreInvocation calls happen per user turn? Per tool call?

### 4. Write the experiment report

- [ ] Create `docs/experiments/X5-channels.md` with:
  - Date, branch, methodology
  - Results for each test (A through D)
  - Transcript excerpts showing the injected messages
  - Persistence analysis: does systemMessage stay across turns?
  - Recommendations for the delivery design (PR-08)
  - Whether CHECKPOINT testing was possible, and if not, methodology for future testing

### 5. Unit tests

- [ ] **Create `internal/harness/antigravity/v2_spike_channels_test.go`:**
  - Test V2SpikePreInvocation returns systemMessage on invocationNum 0
  - Test V2SpikePreInvocation returns {} on invocationNum > 0
  - Test V2SpikePreInvocation returns {} when REPO_MEMORY_V2_SPIKE=0
  - Test V2SpikePreInvocation handles empty/malformed payload gracefully

## Verification Requirement
- `go build ./...` passes
- `go vet ./...` passes
- `go test ./internal/harness/antigravity/...` passes
- Experiment report documents systemMessage/userMessage behavior with transcript evidence

## Issue Surfacing
If you discover a defect, upstream bug, or concern outside your boundary box, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

### Status
(pending)

### Changes Made
(pending)

### Decisions Made
(pending)

### Dead Ends & Warnings
(pending)

### Out-of-Scope Findings
(pending)

### Blockers
(pending)
