---
name: pr-08-v2-delivery-wo4
description: Handoff instructions for Work Order 4 — PreToolUse Gates
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Implement the `PreToolUse` hook handler that gates dangerous edits. This is the fail-closed hook — any mistake (empty output, panic, bad JSON, non-zero exit) will block ALL tool calls across the entire machine. The handler must:

1. Always return `{"decision": "allow"}` by default
2. Check if the edit target has gated entries (`gate: edit` in entry text)
3. If gated and not yet acknowledged: return `{"decision": "deny", "reason": "..."}` with the gate message
4. After denial, the gate is acknowledged — subsequent edits to the same file are allowed

# Context

## X9 PreToolUse Contract (CRITICAL — read carefully)
- **`PreToolUse` is FAIL-CLOSED:** Returning `{}`, invalid JSON, panic, or non-zero exit → **implicit deny** blocking ALL tool calls machine-wide
- Must ALWAYS return `{"decision": "allow"}` on success, error recovery, or when no gate applies
- Supports: `"decision": "deny"`, `"decision": "ask"`, `"decision": "force_ask"`
- Supports: `"overwrite": {...}` for argument rewriting (not used in this WO)
- Zero lag: `PLANNER_RESPONSE` and model thinking are already flushed to disk before `PreToolUse` fires

## PreToolUse Payload
```json
{
  "conversationId": "...",
  "invocationNum": 0,
  "toolCallId": "...",
  "toolName": "replace_file_content",
  "toolCall": {
    "name": "replace_file_content",
    "args": { "TargetFile": "/path/to/file.go", ... }
  },
  "transcriptPath": "...",
  "workspacePaths": ["..."]
}
```

Tools that edit files include: `replace_file_content`, `write_to_file`, `run_command` (potentially).

## Existing Code
- `cmd/repo-memory/cmd_hook.go` — Lines 39-51 already have `PreToolUse` panic recovery and fail-open handling at the top level. Lines 86-92 have the fast path. Line ~195 is the stub.
- `internal/core/delivery/engine.go` — `Engine.GateEntries(ctx, targetFile, ledger)` returns entries with `gate: edit` for a target file.
- `internal/core/delivery/formatter.go` — `FormatGateDeny(entry)` formats the deny message.
- `internal/core/delivery/ledger.go` — `Ledger.IsGateAcknowledged(entryID)`, `Ledger.AcknowledgeGate(entryID)`.

# Strict Boundary Box

**DO NOT:**
- Modify `internal/core/delivery/` (frozen)
- Modify `internal/core/ops/`, `internal/core/dossiers/`, `internal/core/index/`
- Modify the `PreInvocation` or `PostToolUse` handlers
- Add compaction detection (WO5)
- Return `{}` from the PreToolUse handler — this is an implicit deny!

**DO:**
- Create `internal/harness/antigravity/pretooluse.go` — the PreToolUse handler
- Modify `cmd/repo-memory/cmd_hook.go` — wire PreToolUse dispatch
- Create tests

# Tooling Constraints

- Use `go build ./...` to verify compilation
- Use `go test ./internal/harness/antigravity/... -v` to run tests
- Use `go vet ./...` for static analysis

# Line-Item Instructions

## 1. PreToolUse Handler (`internal/harness/antigravity/pretooluse.go`)

- [ ] Create the file with:
  ```go
  func HandlePreToolUse(ctx context.Context, payload []byte, opts ...HookOption) ([]byte, error)
  ```

- [ ] Define `PreToolUsePayload` struct:
  ```go
  type PreToolUsePayload struct {
      ConversationID      string   `json:"conversationId"`
      ConversationIDSnake string   `json:"conversation_id"`
      InvocationNum       int      `json:"invocationNum"`
      ToolName            string   `json:"toolName"`
      ToolCall            struct {
          Name string          `json:"name"`
          Args json.RawMessage `json:"args"`
      } `json:"toolCall"`
      TranscriptPath      string   `json:"transcriptPath"`
      TranscriptPathSnake string   `json:"transcript_path"`
      WorkspacePaths      []string `json:"workspacePaths"`
      WorkspacePathsSnake []string `json:"workspace_paths"`
  }
  ```
  Add `Normalize()` method.

- [ ] Define the allow response constant:
  ```go
  var allowResponse = []byte(`{"decision":"allow"}`)
  ```

- [ ] **CRITICAL: Every exit path MUST return `allowResponse` or a valid deny/ask JSON. NEVER return `{}` or empty bytes.**

- [ ] **Fast exit conditions** (return `allowResponse`):
  - `REPO_MEMORY_DISABLED=1` or `REPO_MEMORY_HOUSEKEEPING=1`
  - Not an edit tool: if `toolName` is not `replace_file_content` or `write_to_file`, return allow
  - No workspace resolved or not enrolled
  - JSON unmarshal error → return `allowResponse`

- [ ] **Extract target file:**
  - Parse `toolCall.Args` to get `TargetFile` (or `AbsolutePath` for view_file, but we only gate edits)
  - If no target file found → return `allowResponse`

- [ ] **Gate check:**
  - Open the index and engine (same pattern as other handlers)
  - Create a per-conversation ledger: `delivery.NewLedger(conversationID)`
  - Call `engine.GateEntries(ctx, targetFile, ledger)`
  - If no gated entries → return `allowResponse`
  - If gated entries found:
    - Format with `delivery.FormatGateDeny(entries[0].Entry)` (take the first/highest ranked)
    - Return deny response:
      ```json
      {"decision": "deny", "reason": "<formatted gate message>"}
      ```
    - NOTE: The ledger is not persisted across calls. On the first call, the gate fires. On retry, a new ledger is created with no acknowledgements — BUT the engine skips acknowledged gates. Since the ledger is fresh, the gate will fire again. This means: for now, gates deny on every call to the same file. This is acceptable for v1 of gates. True per-conversation gate acknowledgement requires ledger persistence (WO5 territory).
    - DECISION: For this WO, implement `force_ask` instead of `deny` for gated entries — this asks the user for confirmation rather than blocking. This is safer because it doesn't repeatedly block the agent.

- [ ] **Response types:**
  ```go
  type PreToolUseResponse struct {
      Decision string `json:"decision"`           // "allow", "deny", "ask", "force_ask"
      Reason   string `json:"reason,omitempty"`
  }
  ```

## 2. Wire PreToolUse Dispatch (`cmd/repo-memory/cmd_hook.go`)

- [ ] In `hookRunE`, add a `PreToolUse` case BEFORE the stub at line ~195:
  ```go
  if (strings.EqualFold(harnessName, "antigravity") || harnessName == "") && strings.EqualFold(eventName, "PreToolUse") {
      payload := preReadPayload
      if len(payload) == 0 {
          payload, _ = io.ReadAll(cmd.InOrStdin())
      }
      out, err := antigravity.HandlePreToolUse(cmd.Context(), payload, antigravity.WithWorkspace(hookWorkspaceFlag))
      if err != nil {
          // FAIL-OPEN: error → allow
          fmt.Fprintln(cmd.OutOrStdout(), `{"decision":"allow","reason":"fail-open: error"}`)
          return nil
      }
      fmt.Fprintln(cmd.OutOrStdout(), string(out))
      return nil
  }
  ```

## 3. Tests (`internal/harness/antigravity/pretooluse_test.go`)

- [ ] **Allow by default:** Empty payload → `{"decision":"allow"}`
- [ ] **Fast exits:** disabled/housekeeping → `{"decision":"allow"}`
- [ ] **Non-edit tools:** `toolName: "view_file"` → `{"decision":"allow"}`
- [ ] **Edit with no gate:** Enrolled workspace, empty index, `toolName: "replace_file_content"` → `{"decision":"allow"}`
- [ ] **Edit with gate:** Enrolled workspace, seeded entry with `gate: edit` anchored to target file → `{"decision":"force_ask", "reason":"..."}`
- [ ] **Verify:** response reason contains `[REPO-MEMORY` tag and the gate text

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./internal/harness/antigravity/... -v` — all tests pass
3. Run `go vet ./...` — must pass
4. **Manual verification:** `echo '{}' | go run ./cmd/repo-memory hook antigravity PreToolUse` should output `{"decision":"allow"}` (NOT `{}`)

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below.

---

## Completion Report (written by the worker at task completion)

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
