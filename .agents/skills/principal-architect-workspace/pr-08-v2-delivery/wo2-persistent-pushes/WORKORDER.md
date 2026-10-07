---
name: pr-08-v2-delivery-wo2
description: Handoff instructions for Work Order 2 — Persistent Pushes (PreInvocation)
complexity: architect
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Build the production `PreInvocation` handler that replaces the V2 spike handler. This is the primary delivery mechanism: it reads from the delivery engine (WO1) and outputs `injectSteps` with `systemMessage` entries for three moments:

1. **Task start** — relevant dossiers from prompt terms and branch diff paths (≤500 tokens)
2. **First entry into dossier** — module-wide invariants when a new scope is touched (≤150 tokens)
3. **First touch of anchor** — file/symbol-specific entries when a file is first read/edited (≤150 tokens)

All persistent pushes use `systemMessage` (confirmed by X5 spike). Gated on `invocationNum == 0` to prevent duplicate pushes across invocations within a turn.

# Context

## Injection Contract (from X5 spike — CRITICAL)
- `systemMessage` MUST be a nested object: `{"systemMessage": {"systemMessage": "...", "metadata": {...}}}`
- Wrapped in `{"injectSteps": [...]}`
- Gate on `invocationNum == 0` — `systemMessage` persists across invocations, so re-injecting accumulates duplicates
- In `invoke_subagent` sessions, `PreInvocation` fires with `initialNumSteps=0`, `userPrompt=""` — hooks must not require `userPrompt != ""`
- Fail-open: `PreInvocation` is NOT fail-closed. Return `{}` on any error.

## Existing Code (read these files)
- `internal/harness/antigravity/v2_spike_channels.go` — The spike handler being replaced. Contains `InjectStep`, `SystemMessageEntry`, `InjectStepsResponse` types that must be REUSED (not duplicated).
- `cmd/repo-memory/cmd_hook.go` lines 159-181 — The spike dispatch code. Replace the spike call with the production handler.
- `internal/core/delivery/` — The engine built in WO1. Use `Engine.TaskStartEntries()`, `Engine.DossierEntries()`, `Engine.AnchorEntries()`, `FormatEntries()`, `ContentHash()`.
- `internal/core/delivery/ledger.go` — `NewLedger()`, `RecordInjection()`, `HasSeenDossier()`, `MarkDossierSeen()`, `HasSeenAnchor()`, `MarkAnchorSeen()`.
- `internal/core/index/` — `NewIndex()` for opening the SQLite index.
- `internal/core/config/` — `DataDir()`, `RepoID()`, `IsEnrolledFast()`.

## PreInvocation Payload
The payload JSON contains:
```json
{
  "conversationId": "...",
  "invocationNum": 0,
  "initialNumSteps": 0,
  "userPrompt": "...",
  "transcriptPath": "...",
  "workspacePaths": ["..."],
  "workspace_paths": ["..."]
}
```

# Strict Boundary Box

**DO NOT:**
- Modify `internal/core/delivery/` (WO1 output — engine is frozen)
- Modify `internal/core/ops/`, `internal/core/dossiers/`, `internal/core/index/`, `internal/core/config/`
- Modify the `Stop` handler in `hooks.go`
- Remove or modify `InjectStep`, `SystemMessageEntry`, `InjectStepsResponse` types — they are correct
- Create any new CLI subcommands
- Add `PostToolUse` or `PreToolUse` handlers (those are WO3 and WO4)

**DO:**
- Create `internal/harness/antigravity/preinvocation.go` — the production PreInvocation handler
- Modify `cmd/repo-memory/cmd_hook.go` — replace the spike dispatch with production handler
- Delete or gut `V2SpikePreInvocation` in `v2_spike_channels.go` (but keep the types)
- Create tests

# Tooling Constraints

- Use `go build ./...` to verify compilation
- Use `go test ./internal/harness/antigravity/... ./cmd/repo-memory/...` to run tests
- Use `go vet ./...` for static analysis

# Line-Item Instructions

## 1. Production PreInvocation Handler (`internal/harness/antigravity/preinvocation.go`)

- [ ] Create the file with a function:
  ```go
  func HandlePreInvocation(ctx context.Context, payload []byte, opts ...HookOption) ([]byte, error)
  ```
- [ ] Parse the payload into a struct with these fields:
  ```go
  type PreInvocationPayload struct {
      ConversationID      string   `json:"conversationId"`
      ConversationIDSnake string   `json:"conversation_id"`
      InvocationNum       int      `json:"invocationNum"`
      InitialNumSteps     int      `json:"initialNumSteps"`
      UserPrompt          string   `json:"userPrompt"`
      TranscriptPath      string   `json:"transcriptPath"`
      TranscriptPathSnake string   `json:"transcript_path"`
      WorkspacePaths      []string `json:"workspacePaths"`
      WorkspacePathsSnake []string `json:"workspace_paths"`
  }
  ```
  Normalize camelCase/snake_case fields (same pattern as `StopPayload.Normalize()`).

- [ ] **Fast exit conditions** (return `{}` immediately):
  - `invocationNum > 0` — persistent messages already injected
  - `REPO_MEMORY_DISABLED=1` environment variable set
  - `REPO_MEMORY_HOUSEKEEPING=1` — skip delivery in housekeeping runs
  - No workspace path resolved
  - Workspace not enrolled (`config.IsEnrolledFast`)

- [ ] **Resolve workspace and index:**
  - Get workspace root from payload (first of WorkspacePaths, or cwd)
  - Get repoID via `config.RepoID(workspaceRoot)` or derive from gitops
  - Open the index: `index.NewIndex(filepath.Join(config.DataDir(), repoID, "index.db"))`
  - Create a delivery engine: `delivery.NewEngine(idx, workspaceRoot)`

- [ ] **Manage the ledger:**
  - The ledger is per-conversation. For now, create a new `delivery.NewLedger(conversationID)` on each PreInvocation call. (Ledger persistence across invocations comes in WO5 with compaction.)
  - NOTE: Since `invocationNum > 0` returns `{}`, and `invocationNum == 0` creates a fresh ledger, deduplication within a single conversation turn is handled. Cross-turn deduplication relies on the ledger being rebuilt — this is acceptable for now.

- [ ] **Task start moment** (when `initialNumSteps <= 1`):
  - Extract prompt terms from `userPrompt` (split on whitespace, filter short words < 3 chars)
  - Compute branch diff paths: run `git diff --name-only HEAD~1` in the workspace (or empty if it fails)
  - Call `engine.TaskStartEntries(ctx, promptTerms, diffPaths)`
  - If entries returned, format with `delivery.FormatEntries()` and add as a `systemMessage` InjectStep

- [ ] **First-entry and first-touch moments:**
  - Read the transcript (if `transcriptPath` is available) to find recently touched files/scopes
  - For each unique scope found, if `!ledger.HasSeenDossier(scope)`, call `engine.DossierEntries()` and add as a `systemMessage` InjectStep. Mark seen.
  - For each unique anchor (file path) found, if `!ledger.HasSeenAnchor(anchor)`, call `engine.AnchorEntries()` and add as a `systemMessage` InjectStep. Mark seen.
  - NOTE: On `invocationNum == 0` with a fresh ledger, ALL transcript-referenced scopes/anchors will be "first seen" — this is correct for the first invocation.

- [ ] **Build the response:**
  - Collect all InjectSteps into an `InjectStepsResponse`
  - If no steps, return `{}`
  - Record each injection in the ledger (for future compaction WO5)
  - Marshal and return

- [ ] **Error handling:**
  - Any error → return `{}` (fail-open)
  - Wrap all index/engine calls in error checks
  - Log errors if a logging mechanism exists, but never fail

## 2. Update Hook Dispatch (`cmd/repo-memory/cmd_hook.go`)

- [ ] Replace lines 159-181 (the V2 spike PreInvocation dispatch):
  - Remove the `REPO_MEMORY_V2_SPIKE` gate
  - Call `antigravity.HandlePreInvocation(cmd.Context(), payload, antigravity.WithWorkspace(hookWorkspaceFlag))` instead
  - Keep the fail-open pattern: if error, print `{}` and return nil

## 3. Clean Up Spike Code (`internal/harness/antigravity/v2_spike_channels.go`)

- [ ] Remove the `V2SpikePreInvocation` function body — replace with a comment: `// Removed: replaced by HandlePreInvocation in preinvocation.go`
- [ ] KEEP the type definitions (`InjectStep`, `SystemMessageEntry`, `InjectStepsResponse`) — they are the canonical injection types used by the production handler

## 4. Tests (`internal/harness/antigravity/preinvocation_test.go`)

- [ ] **Fast-exit tests:**
  - `invocationNum > 0` → returns `{}`
  - `REPO_MEMORY_DISABLED=1` → returns `{}`
  - `REPO_MEMORY_HOUSEKEEPING=1` → returns `{}`
- [ ] **Payload normalization test:**
  - snake_case fields correctly mapped to camelCase
- [ ] **Task start test:**
  - With a valid enrolled workspace, seeded index, and `initialNumSteps=0`:
  - Verify the response contains `injectSteps` with `systemMessage` entries
  - Verify the `systemMessage` is a nested object (not a plain string)
  - Verify entries contain `[repo-memory ...]` tags
- [ ] **Empty index test:**
  - Enrolled workspace with empty index → returns `{}` (no entries to inject)

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./internal/harness/antigravity/... -v` — all tests must pass
3. Run `go vet ./...` — must pass
4. Manually verify: `echo '{"conversationId":"test","invocationNum":0,"initialNumSteps":0}' | go run ./cmd/repo-memory hook antigravity PreInvocation` — should output `{}` (no enrolled workspace) or `{"injectSteps":[...]}` (if enrolled with seeded data)

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

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
