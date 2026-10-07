---
name: pr-08-v2-delivery-wo3
description: Handoff instructions for Work Order 3 — Ephemeral Messages & Error Matching
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Add two ephemeral delivery moments to the hook system:

1. **About-to-edit pointer** — when the model's reasoning mentions editing a file, inject an ephemeral reminder of relevant entries already in context (≤50 tokens).
2. **Error matching** — when a tool call fails (`PostToolUse` with an error), match the error output against known failure-signature patterns in the index and deliver the fix as an `ephemeralMessage` (≤200 tokens).

Both moments use `ephemeralMessage` (plain string, single-invocation scope) rather than persistent `systemMessage`.

# Context

## Injection Contract
- `ephemeralMessage` is a plain string in an `InjectStep`: `{"injectSteps": [{"ephemeralMessage": "..."}]}`
- Unlike `systemMessage`, it does NOT need a nested object
- It is visible for one invocation only — no persistence across turns
- Returned from `PreInvocation` (for about-to-edit) or `PostToolUse` (for error matching)

## Existing Code (read these files)
- `internal/harness/antigravity/preinvocation.go` — The WO2 production handler. About-to-edit should be added here as an additional InjectStep.
- `internal/harness/antigravity/v2_spike_channels.go` — Contains `InjectStep`, `InjectStepsResponse` types.
- `internal/core/delivery/engine.go` — `Engine.ErrorEntries()` for error pattern matching.
- `internal/core/delivery/formatter.go` — `FormatEntry()`, `FormatEntries()` for formatting.
- `internal/core/delivery/types.go` — `MomentAboutToEdit`, `MomentError`, `ChannelEphemeral`.
- `internal/core/index/entries.go` — `LookupErrorPatterns(ctx, commandPrefix)` returns `ErrorPatternResult`.
- `internal/harness/antigravity/transcript.go` — `ParseTranscript()`, `Step` struct with `Content`, `ToolCalls`, `Type`.
- `cmd/repo-memory/cmd_hook.go` — Hook dispatch. Currently no `PostToolUse` handler exists (line ~195 returns a stub).

## PostToolUse Payload
```json
{
  "conversationId": "...",
  "invocationNum": 0,
  "toolCallId": "...",
  "toolName": "run_command",
  "toolCall": { "name": "run_command", "args": { "CommandLine": "...", "Cwd": "..." } },
  "toolResult": { "status": "error", "output": "..." },
  "stepIdx": 42,
  "transcriptPath": "...",
  "workspacePaths": ["..."]
}
```

# Strict Boundary Box

**DO NOT:**
- Modify `internal/core/delivery/` (WO1 output — frozen)
- Modify `internal/core/ops/`, `internal/core/dossiers/`, `internal/core/index/`
- Add `PreToolUse` gates (that is WO4)
- Add compaction detection (that is WO5)
- Create CLI subcommands

**DO:**
- Create `internal/harness/antigravity/posttooluse.go` — PostToolUse handler for error matching
- Modify `internal/harness/antigravity/preinvocation.go` — add about-to-edit ephemeral pointer
- Modify `cmd/repo-memory/cmd_hook.go` — wire PostToolUse dispatch
- Create tests

# Tooling Constraints

- Use `go build ./...` to verify compilation
- Use `go test ./internal/harness/antigravity/... -v` to run tests
- Use `go vet ./...` for static analysis

# Line-Item Instructions

## 1. PostToolUse Error Handler (`internal/harness/antigravity/posttooluse.go`)

- [ ] Create the file with:
  ```go
  func HandlePostToolUse(ctx context.Context, payload []byte, opts ...HookOption) ([]byte, error)
  ```
- [ ] Define `PostToolUsePayload` struct:
  ```go
  type PostToolUsePayload struct {
      ConversationID      string   `json:"conversationId"`
      ConversationIDSnake string   `json:"conversation_id"`
      InvocationNum       int      `json:"invocationNum"`
      ToolName            string   `json:"toolName"`
      ToolCall            struct {
          Name string         `json:"name"`
          Args json.RawMessage `json:"args"`
      } `json:"toolCall"`
      ToolResult struct {
          Status string `json:"status"`
          Output string `json:"output"`
      } `json:"toolResult"`
      StepIdx             int      `json:"stepIdx"`
      TranscriptPath      string   `json:"transcriptPath"`
      TranscriptPathSnake string   `json:"transcript_path"`
      WorkspacePaths      []string `json:"workspacePaths"`
      WorkspacePathsSnake []string `json:"workspace_paths"`
  }
  ```
  Add a `Normalize()` method following the same pattern as `PreInvocationPayload`.

- [ ] **Fast exit conditions** (return `{}` immediately):
  - `REPO_MEMORY_DISABLED=1` or `REPO_MEMORY_HOUSEKEEPING=1`
  - No workspace resolved or not enrolled
  - `toolResult.Status != "error"` — only process failed tool calls

- [ ] **Error matching logic:**
  - Extract the command prefix from `toolCall.Args`. If `toolName` is `run_command`, parse `CommandLine` from args and take the first word as the command prefix. For other tools, use `toolName` as the prefix.
  - Open the index and create a delivery engine (same pattern as `HandlePreInvocation`)
  - Call `engine.ErrorEntries(ctx, commandPrefix, toolResult.Output)`
  - If entries found, format with `delivery.FormatEntries()` and return as `ephemeralMessage`:
    ```go
    InjectStepsResponse{InjectSteps: []InjectStep{{EphemeralMessage: formatted}}}
    ```
  - If no entries found, return `{}`

## 2. About-to-Edit Pointer (modify `internal/harness/antigravity/preinvocation.go`)

- [ ] After the first-touch/first-entry section (around line 235), add the about-to-edit moment:
  - Check the transcript's latest `PLANNER_RESPONSE` step's `Thinking` or `Content` for edit intent
  - Look for patterns like: `"edit "`, `"modify "`, `"change "`, `"update "`, `"write to "`, `"replace "` followed by a file path
  - If a target file is identified AND there are already entries in context for that file (check ledger):
    - Add an `ephemeralMessage` InjectStep: `"[repo-memory] Check injected entries above before editing <filename>"`
    - Budget: ≤50 tokens (this is a short pointer, not content)
  - If no edit intent detected or no relevant entries, skip (do nothing)

## 3. Wire PostToolUse Dispatch (`cmd/repo-memory/cmd_hook.go`)

- [ ] Add a `PostToolUse` case in `hookRunE` (after the PreInvocation block, before the stub at line ~195):
  ```go
  if (strings.EqualFold(harnessName, "antigravity") || harnessName == "") && strings.EqualFold(eventName, "PostToolUse") {
      // Read payload if not already read
      payload := preReadPayload
      if len(payload) == 0 {
          payload, _ = io.ReadAll(cmd.InOrStdin())
      }
      out, err := antigravity.HandlePostToolUse(cmd.Context(), payload, antigravity.WithWorkspace(hookWorkspaceFlag))
      if err != nil {
          fmt.Fprintln(cmd.OutOrStdout(), "{}")
          return nil
      }
      fmt.Fprintln(cmd.OutOrStdout(), string(out))
      return nil
  }
  ```

## 4. Tests

- [ ] Create `internal/harness/antigravity/posttooluse_test.go`:
  - **Fast exit:** disabled/housekeeping/not-enrolled → `{}`
  - **Non-error result:** `status: "success"` → `{}`
  - **Error with matching pattern:** Set up enrolled workspace with seeded error pattern in index. Send error payload with matching command prefix. Verify `ephemeralMessage` in response.
  - **Error with no match:** Send error with unknown command prefix → `{}`

- [ ] Add about-to-edit test to `preinvocation_test.go`:
  - Verify that when transcript thinking mentions "edit file.go", and file.go has entries in context, an ephemeral pointer is added

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./internal/harness/antigravity/... -v` — all tests must pass
3. Run `go vet ./...` — must pass

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

<!-- Template — append this section when done -->

### Status
(pending)

### Changes Made
- (pending)

### Decisions Made
- (pending)

### Dead Ends & Warnings
- (pending)

### Out-of-Scope Findings
- (pending)

### Blockers
- (pending)
