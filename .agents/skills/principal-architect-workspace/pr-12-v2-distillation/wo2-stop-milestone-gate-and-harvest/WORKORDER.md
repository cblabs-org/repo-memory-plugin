---
name: pr-12-v2-distillation-wo2
description: Handoff instructions for Work Order 2 (Automatic Milestone-Gated Stop Warm-Cache Continuation & Harvest)
complexity: architect
---

# pr-12-v2-distillation / WO2 — Automatic Milestone-Gated `Stop` (`NO_TOOL_CALL`) Warm-Cache Continuation & Harvest

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not create branches other than the active branch (`pr-12-v2-distillation/wo2-stop-milestone-gate-and-harvest`), and do not plan. Your task is strictly limited to executing the checklist below.
>
> **Authorization & Governance:** Read `.agents/rules/00-operating-model.md` and `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/PROJECT.md` (which records human approval of this 6-WO decomposition). You are authorized to implement and commit this Work Order autonomously within the Strict Boundary Box below.

## Objective
Implement the `v2.2` Stage 1 Primary Warm-Cache Distillation mechanism in the Antigravity `Stop` hook (`SPEC.md` §6.3.1, `docs/design-v2-distillation-retrieval.md` §5.1): add `turn_notes` (`on | fallback-only | off`) and `turn_notes_role_opt_out` to `config.RepoConfig`, implement the deterministic `executionNum == 0` Milestone Gate returning `{"decision": "continue", "reason": "<request>"}`, and implement the `executionNum >= 1` `repo-memory-note` YAML harvester with loop protection, anchor verification, `ExposureList` attachment, and mark advancement.

## Strict Boundary Box
- **Allowed to create/modify:**
  - `internal/core/config/config.go` and `internal/core/config/config_test.go`
  - `internal/harness/antigravity/hooks.go` and `internal/harness/antigravity/hooks_test.go`
  - `internal/harness/antigravity/stop_distill.go` and `internal/harness/antigravity/stop_distill_test.go`
  - `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/wo2-stop-milestone-gate-and-harvest/WORKORDER.md` (Completion Report)
- **Forbidden:**
  - Do NOT modify `internal/core/tick/` (belongs to WO3 and WO5).
  - Do NOT import `internal/harness/...` from any package under `internal/core/...`.
  - NEVER include any internal references (`google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or absolute `/usr/local/google/...` paths) in any code, comment, or commit message.
  - All git commits must use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.

## Tooling Constraints
- Use standard `read_file`, `write_file`, `edit_file`, and `run_command` tools.
- Run `go test ./...` and `go vet ./...` to verify.

## Line-Item Instructions

- [ ] **1. `internal/core/config/config.go` — Add `TurnNotes` and `TurnNotesRoleOptOut` to `RepoConfig`**
  - Add fields to `RepoConfig`:
    ```go
    TurnNotes           string   `json:"turn_notes,omitempty" yaml:"turn_notes,omitempty"`
    TurnNotesRoleOptOut []string `json:"turn_notes_role_opt_out,omitempty" yaml:"turn_notes_role_opt_out,omitempty"`
    ```
  - Add helper method `(c RepoConfig) TurnNotesMode() string`:
    - Normalize `strings.ToLower(strings.TrimSpace(c.TurnNotes))`.
    - If `"off"` or `"false"` or `"disabled"`, return `"off"`.
    - If `"fallback-only"` or `"fallback_only"`, return `"fallback-only"`.
    - Otherwise return `"on"` (the default).
  - Add unit tests in `internal/core/config/config_test.go` verifying YAML round-trip and `TurnNotesMode()` defaults and overrides.

- [ ] **2. `internal/harness/antigravity/hooks.go` — Wire `REPO_MEMORY_INTERNAL`, `Decision: "continue"`, and Stop Distillation**
  - Add fields to `StopResult`:
    ```go
    Decision       string `json:"decision,omitempty"`
    ContinueReason string `json:"continue_reason,omitempty"`
    NoteID         string `json:"note_id,omitempty"`
    ```
  - In `HandleStopEvent`:
    - Alongside the `REPO_MEMORY_HOUSEKEEPING` check, also check `os.Getenv("REPO_MEMORY_INTERNAL") == "1" || strings.EqualFold(os.Getenv("REPO_MEMORY_INTERNAL"), "true")`. If set, return `&StopResult{Skipped: true, Reason: "internal run skipped (REPO_MEMORY_INTERNAL)", Duration: time.Since(start)}, nil`.
    - After artifact capture (step 7, before returning `StopResult`), invoke `evaluateStopDistillation(ctx, payload, result, dataDir, repoID, repoRoot, repoCfg, transcriptPath)`.
  - In `HandleStop`:
    - Capture `res, err := HandleStopEvent(ctx, &payload, opts...)`.
    - If `err != nil`, return `nil, err`.
    - If `res != nil && res.Decision == "continue" && res.ContinueReason != ""`, return `json.Marshal(map[string]string{"decision": "continue", "reason": res.ContinueReason})`.
    - Otherwise return `[]byte("{}"), nil`.

- [ ] **3. `internal/harness/antigravity/stop_distill.go` — Stage 1 Milestone Gate (`executionNum == 0`) & Harvest (`executionNum >= 1`)**
  - Create `internal/harness/antigravity/stop_distill.go`.
  - Define constants:
    - `TurnNoteCooldown = 30 * time.Minute`
    - `TurnNoteDailyCap = 20`
    - `MaxNoteItems = 6`
  - **Milestone Gate (`payload.ExecutionNum == 0`):**
    1. Check `repoCfg.TurnNotesMode() == "on"`. If not `"on"`, return without continuing.
    2. Check subagent exclusion:
       - If `res.ParentConversationID != ""` or `payload.ParentConversationID != ""`, skip.
       - Call `isSub, _, err := state.IsSubagent(ctx, dataDir, repoID, convID)`. If `err == nil && isSub`, skip.
    3. Check normal turn completion:
       - Require `strings.EqualFold(payload.TerminationReason, "NO_TOOL_CALL") && payload.FullyIdle`. (Note: existing tests in `hooks_test.go` that omit `terminationReason` or `fullyIdle` will naturally skip continuation and return `"{}"`, preserving backward compatibility with all existing capture tests!)
    4. Require `transcriptPath != ""` and parse steps via `steps, err := ParseTranscript(filepath.Dir(transcriptPath))`. If `err != nil || len(steps) == 0`, skip.
    5. Load `mark, err := state.LoadMark(dataDir, repoID, convID)`.
       - Find `stepAtMarkTimestamp`: look for a step in `steps` with `!step.IsCheckpoint && !step.IsGap && step.StepIndex == mark.StepIndex`.
       - Call `if mark.ClampIfRewound(len(steps), stepAtMarkTimestamp) { _ = state.SaveMark(dataDir, repoID, mark) }`.
    6. Check role opt-out and autonomous `/goal` or workflow exclusion:
       - Inspect steps for active `/goal` command or role opt-out (`repoCfg.TurnNotesRoleOptOut`, case-insensitive match against initial user/system prompt text). If matched, skip.
    7. **Detect Milestones in steps strictly after the mark** (i.e., if `mark.StepIndex == 0 && mark.NoteCount == 0 && mark.LastNoteAt.IsZero()`, inspect all steps `>= 0`; otherwise inspect steps with `step.StepIndex > mark.StepIndex`):
       - Parse tool calls from each step (`step.ToolCalls`, supporting both native `[{"name": "...", "args": {...}}]` and OpenAI `[{"function": {"name": "...", "arguments": "{...}"}}]` formats, as well as subsequent `GENERIC` / tool result steps or step `Status`).
       - Detect the 5 milestone signals (`DetectMilestones(steps []Step, afterStepIdx int, isInitialMark bool) MilestoneResult` with `Matched bool`, `IsCommitOrPR bool`, `Reason string`):
         1. **`git commit`:** a `run_command` / `bash` tool call whose command contains `git commit` (not followed by a failed status/error output in the step or immediately following `GENERIC`/tool-result step). Sets `IsCommitOrPR = true`.
         2. **`gh pr create`:** a `run_command` / `bash` tool call whose command contains `gh pr create` (and succeeded). Sets `IsCommitOrPR = true`.
         3. **Work order completed:** an edit/write tool call (`write_to_file`, `replace_file_content`, `edit_file`, `multi_replace_file_content`) targeting a path ending in `WORKORDER.md` whose content/replacement contains `## Completion Report`.
         4. **Error or failing test resolved after $\ge 15$ steps:** an earlier command/test step at `StepIndex` $s_1$ (may be before or after `afterStepIdx`) that failed (`Status == "ERROR"` or tool output contains `FAIL` / `exit code 1` / `exit status` / `error:`), and a later command/test step with the same command prefix (e.g. `go test`, `pio test`, `pytest`, `npm test`, `cargo test`, `make`) at `StepIndex` $s_2 > \text{afterStepIdx}$ that succeeded (`Status != "ERROR"` and no failure marker) with $s_2 - s_1 \ge 15$.
         5. **Edits to $\ge 2$ distinct files followed by a build or test:** at least 2 distinct target file paths modified via edit/write tools in the uncovered window, and a build/test command (`go test`, `go build`, `go vet`, `pytest`, `npm test`, `npm run build`, `cargo test`, `cargo build`, `pio test`, `pio run`, `make`, `ctest`) executed at a step index $\ge$ the step where the 2nd distinct file was edited.
       - If `!milestone.Matched`, skip.
    8. **Cooldown & Daily Cap:**
       - If `!milestone.IsCommitOrPR && !mark.LastNoteAt.IsZero() && time.Since(mark.LastNoteAt) < TurnNoteCooldown`, skip.
       - Call `dailyCount, err := state.CountTurnNotesToday(dataDir, repoID, time.Now().UTC())`. If `err == nil && dailyCount >= TurnNoteDailyCap`, skip.
    9. **Issue Continuation Request:**
       - Determine `latestStepIdx` (highest `step.StepIndex` in `steps`).
       - Set `mark.PendingRequestStep = latestStepIdx` and save `mark` via `state.SaveMark(dataDir, repoID, mark)`.
       - Check if any `step.IsCheckpoint` occurred after `mark.StepIndex`.
       - Build prompt via `BuildTurnNoteRequestPrompt(mark.NoteTitles, hasCheckpointSinceMark)`:
         Include the exact canonical prompt from `docs/design-v2-distillation-retrieval.md` §5.1 (including the fenced ```` ```repo-memory-note ```` example block and allowed types/fields). If `hasCheckpointSinceMark && len(mark.NoteTitles) > 0`, append the list of earlier note titles in this conversation so they are not repeated.
       - Set `res.Decision = "continue"` and `res.ContinueReason = prompt`.

  - **Harvest Pass (`payload.ExecutionNum >= 1`):**
    1. Load `mark, _ := state.LoadMark(dataDir, repoID, convID)`.
    2. Parse `steps, err := ParseTranscript(filepath.Dir(transcriptPath))`.
    3. Find the latest step index (`latestStepIdx`), latest step timestamp (`latestTimestamp`), and current transcript file size (`byteOffset`).
    4. Find the model response step(s) after `mark.PendingRequestStep` (or the final `PLANNER_RESPONSE` step in `steps`).
    5. **Always advance mark before returning (loop protection):**
       - Helper `advanceMarkAndClearPending(mark, latestStepIdx, byteOffset, latestTimestamp)` sets `mark.StepIndex = latestStepIdx`, `mark.ByteOffset = byteOffset`, `mark.StepTimestamp = latestTimestamp`, `mark.PendingRequestStep = 0`.
       - `res.Decision` is ALWAYS `""` on `executionNum >= 1` so `HandleStop` returns `"{}"` and never loops!
    6. Check if the reply step contains tool calls (`hasNonEmptyToolCalls(step.ToolCalls)`). If it does, save the advanced mark and return immediately (failure recorded by skipping inbox write).
    7. Parse the reply text via `ParseRepoMemoryNoteBlock(text string) (items []inbox.NoteItem, isNone bool, err error)`:
       - If trimmed text is `"NONE"` (or inside the fenced block is `"NONE"`), save advanced mark and return.
       - Extract the content inside ```` ```repo-memory-note ... ``` ```` (also tolerate ```` ```yaml ```` if it parses as a list of note items with `type:` and `title:`, though `repo-memory-note` is primary).
       - Unmarshal YAML into `[]inbox.NoteItem`.
    8. Lint items via `LintNoteItems(items []inbox.NoteItem) []inbox.NoteItem`:
       - Allowed types: `decision`, `ruled-out`, `failure-signature`, `invariant`, `contract`, `convention`, `gotcha`, `external-behaviour`, `procedure`, `recipe`, `open-issue`.
       - Require non-empty `Title`.
       - Require non-empty `Why` for all types except `procedure` and `recipe`.
       - Require non-empty `Do` for `ruled-out`, `failure-signature`, `invariant`, `procedure`, and `recipe`.
       - Cap at `MaxNoteItems` (6).
    9. If `len(validItems) > 0`:
       - For each item, scrub `Title`, `Why`, `Do`, `Evidence` (redact AWS keys `AKIA...`, `Bearer ...` / `token ...`, `ghp_...`, private keys) and populate `item.AnchorChecks = inbox.CheckAnchors(repoRoot, item.Anchors)`.
       - Load `ledger, _ := LoadLedger(dataDir, repoID, convID)` and get `var exposures []delivery.ExposureRecord` (`if ledger != nil { exposures = ledger.GetExposureList() }`).
       - Write `&inbox.Note{Source: inbox.SourceTurnNote, ConversationID: convID, StepsCovered: [2]int{startStep, latestStepIdx}, Model: payload.ModelName, Branch: res.Branch, HeadCommit: res.Head, ExposureList: exposures, Items: validItems}` to `inbox.NewStore(dataDir, repoID)`.
       - Update `mark.LastNoteAt = time.Now().UTC()`, `mark.NoteCount++`, and append each item's `Title` to `mark.NoteTitles`.
       - Save `mark` and set `res.NoteID = noteID`.

- [ ] **4. `internal/harness/antigravity/stop_distill_test.go` — Comprehensive Unit Tests**
  - Test all 5 milestone signals on `executionNum == 0` returning `{"decision": "continue", "reason": "..."}`:
    1. `git commit` milestone;
    2. `gh pr create` milestone;
    3. `WORKORDER.md` `## Completion Report` milestone;
    4. Command/test failure resolved after $\ge 15$ steps;
    5. Edits to $\ge 2$ distinct files followed by `go test`.
  - Test gate rejections (returns `"{}"`):
    - No milestone in steps after mark;
    - `terminationReason != "NO_TOOL_CALL"` or `fullyIdle == false`;
    - Subagent conversation (via `ParentConversationID` or `state.RecordSubagent`);
    - `turn_notes: off` and `turn_notes: fallback-only`;
    - Role opt-out (`TurnNotesRoleOptOut`) and `/goal` active;
    - 30-minute cooldown blocks non-commit milestone, but is waived for `git commit` and `gh pr create`;
    - Daily cap of 20 `turn-note` notes blocks 21st request;
    - Mid-turn `memory_note` MCP call that advanced `mark.StepIndex` past the milestone deduplicates the `Stop` gate so `Stop` returns `"{}"`.
  - Test `executionNum >= 1` harvest pass:
    - Valid `repo-memory-note` block writes `source: "turn-note"` note to `<dataDir>/<repoID>/evidence/notes/`, populates `AnchorChecks` against `repoDir`, attaches `ExposureList` from the conversation's `delivery.Ledger`, updates `mark.StepIndex`, `mark.LastNoteAt`, `mark.NoteTitles`, `mark.NoteCount`, clears `mark.PendingRequestStep`, and returns `"{}"`;
    - `NONE` reply advances `mark.StepIndex`, writes no inbox note, and returns `"{}"`;
    - Malformed reply or reply containing tool calls advances `mark.StepIndex` (loop protection), writes no inbox note, and returns `"{}"`.
  - Test rewind clamping during `Stop`: if transcript shrinks below `mark.StepIndex`, `mark` clamps back and detects new milestones.

## Verification Requirement
- Run `go test ./...` and `go vet ./...` and verify 100% pass.
- Ensure `git status` shows only expected files in the Strict Boundary Box.
- Commit your changes on `pr-12-v2-distillation/wo2-stop-milestone-gate-and-harvest` using `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.
- Append your Completion Report to this `WORKORDER.md`, commit it, and send a message back to the Principal Architect confirming completion.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->
