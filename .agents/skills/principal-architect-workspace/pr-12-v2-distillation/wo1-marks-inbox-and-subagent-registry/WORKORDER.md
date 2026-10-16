---
name: pr-12-v2-distillation-wo1
description: Handoff instructions for Work Order 1 (Per-Conversation Marks, Inbox Notes & Subagent Registry)
complexity: standard
---

# pr-12-v2-distillation / WO1 — Per-Conversation Marks, Inbox Notes & Subagent Registry

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not create branches other than the active branch (`pr-12-v2-distillation/wo1-marks-inbox-and-subagent-registry`), and do not plan. Your task is strictly limited to executing the checklist below.
>
> **Authorization & Governance:** Read `.agents/rules/00-operating-model.md` and `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/PROJECT.md` (which records human approval of this 6-WO decomposition). You are authorized to implement and commit this Work Order autonomously within the Strict Boundary Box below.

## Objective
Implement the `v2.2` Stage 1 foundational stores and hooks: per-conversation Marks (`internal/core/state/marks.go` with rewind clamping and daily note counting), the structured Inbox Note store (`internal/core/inbox/note.go` with anchor checking), the SQLite Subagent Registry (`internal/core/state/subagents.go` wired into `HandlePostToolUse` on `invoke_subagent`), and MCP updates for `memory_note` (writing `source: "mcp-note"` inbox notes and advancing the conversation's mark) and `memory_status` (aliases `memory_stats`, `memory.status`, `memory.stats`).

## Strict Boundary Box
- **Allowed to create/modify:**
  - `internal/core/config/layout.go` (and `internal/core/config/layout_test.go` if needed)
  - `internal/core/state/marks.go` and `internal/core/state/marks_test.go`
  - `internal/core/state/subagents.go` and `internal/core/state/subagents_test.go`
  - `internal/core/inbox/note.go` and `internal/core/inbox/note_test.go`
  - `internal/harness/antigravity/posttooluse.go` and `internal/harness/antigravity/posttooluse_test.go`
  - `internal/mcpserver/tools.go`, `internal/mcpserver/server.go`, `internal/mcpserver/server_test.go` (or `tools_test.go`)
  - `cmd/repo-memory/cmd_mcp.go`
  - `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/wo1-marks-inbox-and-subagent-registry/WORKORDER.md` (Completion Report)
- **Forbidden:**
  - Do NOT modify `Stop` hook logic (`hooks.go`) — that belongs to WO2.
  - Do NOT modify `tick/pipeline.go` — that belongs to WO3 and WO5.
  - Do NOT import `internal/harness/...` from any package under `internal/core/...` (`boundary_test.go` enforces this).
  - NEVER include any internal references (`google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or absolute `/usr/local/google/...` paths) in any code, comment, or commit message.
  - All git commits must use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.

## Tooling Constraints
- Use standard `read_file`, `write_file`, `edit_file`, and `run_command` tools.
- Run `go test ./...` and `go vet ./...` to verify.

## Line-Item Instructions

- [ ] **1. `internal/core/config/layout.go` — Add `state/marks`, `evidence/notes`, and `quarantine` subdirectories**
  - Add `filepath.Join("state", "marks")`, `filepath.Join("evidence", "notes")`, and `"quarantine"` to `DataDirSubdirectories` in `internal/core/config/layout.go`.
  - Verify existing tests in `internal/core/config/` pass (updating any exact subdirectory slice assertions if present).

- [ ] **2. `internal/core/state/marks.go` — Per-conversation Marks & Rewind Clamping**
  - Create package `state` in `internal/core/state/marks.go`.
  - Define `Mark`:
    ```go
    type Mark struct {
        ConversationID     string    `json:"conversation_id"`
        StepIndex          int       `json:"step_index"`
        ByteOffset         int64     `json:"byte_offset"`
        StepTimestamp      string    `json:"step_timestamp,omitempty"`
        LastNoteAt         time.Time `json:"last_note_at,omitempty"`
        NoteTitles         []string  `json:"note_titles,omitempty"`
        NoteCount          int       `json:"note_count"`
        PendingRequestStep int       `json:"pending_request_step,omitempty"`
        UpdatedAt          time.Time `json:"updated_at"`
    }
    ```
  - Sanitize `conversationID` for path safety (matching `^[a-zA-Z0-9_\-]+$` up to 255 chars, else SHA-256 hex prefix) and store at `<dataDir>/<repoID>/state/marks/<safeConvID>.json`.
  - Implement:
    - `MarkPath(dataDir, repoID, convID string) string`
    - `LoadMark(dataDir, repoID, convID string) (*Mark, error)`: if the file does not exist (or `convID == ""`), return `&Mark{ConversationID: convID, NoteTitles: []string{}}, nil`.
    - `SaveMark(dataDir, repoID string, m *Mark) error`: if `m == nil || m.ConversationID == ""`, return `nil`. Set `m.UpdatedAt = time.Now().UTC()`, create parent directory, and atomically write via temp file + `os.Rename`.
    - `(m *Mark) ClampIfRewound(stepCount int, stepAtMarkTimestamp string) bool`:
      - If `m.StepIndex <= 0 && m.ByteOffset <= 0 && m.PendingRequestStep <= 0`, return `false`.
      - Rewound if `stepCount <= m.StepIndex` OR (`m.StepTimestamp != ""` && `stepAtMarkTimestamp != ""` && `stepAtMarkTimestamp != m.StepTimestamp`).
      - When rewound: if `stepCount < 0`, `stepCount = 0`; set `m.StepIndex = 0` (or if `stepCount > 0` and only `stepCount <= m.StepIndex`, clamp `m.StepIndex = 0`), `m.ByteOffset = 0`, `m.StepTimestamp = ""`, `m.PendingRequestStep = 0`, and return `true`. (Note: checkpoints never move the mark.)
    - `CountTurnNotesToday(dataDir, repoID string, now time.Time) (int, error)` (and alias `CountNotesToday(dataDir, repoID string, now time.Time) (int, error)`): scans `<dataDir>/<repoID>/evidence/notes/*.json` and counts notes created on `now.UTC()`'s calendar date (`YYYY-MM-DD`) where `source == "turn-note"` (for `CountTurnNotesToday`) or any source (for `CountNotesToday`). If the directory doesn't exist, return `0, nil`.

- [ ] **3. `internal/core/inbox/note.go` — Structured Inbox Note Store & Anchor Verification**
  - Create package `inbox` in `internal/core/inbox/note.go`.
  - Define `NoteSource` string type and constants:
    - `SourceTurnNote NoteSource = "turn-note"`
    - `SourceMCPNote NoteSource = "mcp-note"`
    - `SourceCheckpoint NoteSource = "checkpoint"`
    - `SourceArtifact NoteSource = "artifact"`
    - `SourceSidecar NoteSource = "sidecar"`
    - `SourcePA NoteSource = "pa"`
    - `SourceDocSeed NoteSource = "doc-seed"`
  - Define `AnchorCheck`:
    ```go
    type AnchorCheck struct {
        Anchor     string    `json:"anchor"`
        Exists     bool      `json:"exists"`
        VerifiedAt time.Time `json:"verified_at"`
    }
    ```
  - Define `NoteItem`:
    ```go
    type NoteItem struct {
        Type          dossiers.EntryType `json:"type" yaml:"type"`
        Title         string             `json:"title" yaml:"title"`
        Why           string             `json:"why,omitempty" yaml:"why,omitempty"`
        Do            string             `json:"do,omitempty" yaml:"do,omitempty"`
        Anchors       []string           `json:"anchors,omitempty" yaml:"anchors,omitempty"`
        AnchorChecks  []AnchorCheck      `json:"anchor_checks,omitempty" yaml:"anchor_checks,omitempty"`
        Evidence      string             `json:"evidence,omitempty" yaml:"evidence,omitempty"`
        ClosesWhen    string             `json:"closes_when,omitempty" yaml:"closes_when,omitempty"`
        ErrorPattern  string             `json:"error_pattern,omitempty" yaml:"error_pattern,omitempty"`
        Commands      []string           `json:"commands,omitempty" yaml:"commands,omitempty"`
        StepCitations []int              `json:"step_citations,omitempty" yaml:"step_citations,omitempty"`
        Verified      string             `json:"verified,omitempty" yaml:"verified,omitempty"`
        Scope         string             `json:"scope,omitempty" yaml:"scope,omitempty"`
    }
    ```
  - Define `Note`:
    ```go
    type Note struct {
        ID             string                    `json:"id"`
        Source         NoteSource                `json:"source"`
        ConversationID string                    `json:"conversation_id,omitempty"`
        StepsCovered   [2]int                    `json:"steps_covered,omitempty"`
        Model          string                    `json:"model,omitempty"`
        HarnessVersion string                    `json:"harness_version,omitempty"`
        Branch         string                    `json:"branch,omitempty"`
        HeadCommit     string                    `json:"head_commit,omitempty"`
        ExposureList   []delivery.ExposureRecord `json:"exposure_list,omitempty"`
        Items          []NoteItem                `json:"items"`
        CreatedAt      time.Time                 `json:"created_at"`
        Consolidated   bool                      `json:"consolidated"`
        DedupKey       string                    `json:"dedup_key,omitempty"`
    }
    ```
  - Implement `NewNoteID() string` returning `"note-" + ulid.Make().String()`.
  - Implement `CheckAnchors(repoRoot string, anchors []string) []AnchorCheck`:
    - For each anchor in `anchors`, strip optional `#symbol` or `:symbol` suffix to get the relative file/directory path (normalize slashes, reject `..` traversal outside `repoRoot`).
    - Stat `filepath.Join(repoRoot, cleanPath)` and record `AnchorCheck{Anchor: rawAnchor, Exists: err == nil, VerifiedAt: time.Now().UTC()}`.
  - Implement `Store`:
    - `NewStore(dataDir, repoID string) *Store` (storing JSON files in `<dataDir>/<repoID>/evidence/notes/<id>.json`).
    - `(s *Store) Dir() string`
    - `(s *Store) Write(note *Note) (string, error)`: assigns `note.ID = NewNoteID()` if empty, `note.CreatedAt = time.Now().UTC()` if zero, writes atomically via temp file + rename, returns `note.ID, nil`.
    - `(s *Store) Read(id string) (*Note, error)`
    - `(s *Store) ListAll() ([]*Note, error)`: returns all notes sorted by `CreatedAt` ascending (then `ID`). Returns empty slice if directory doesn't exist.
    - `(s *Store) ListUnconsolidated() ([]*Note, error)`: filters `ListAll()` where `!n.Consolidated`.
    - `(s *Store) MarkConsolidated(id string) error`: loads note `id`, sets `Consolidated = true`, and re-writes atomically.
    - `(s *Store) HasDedupKey(dedupKey string) (bool, error)`: returns `true` if any existing note in the store has `n.DedupKey == dedupKey`.

- [ ] **4. `internal/core/state/subagents.go` & `internal/harness/antigravity/posttooluse.go` — Subagent Registry**
  - Create `internal/core/state/subagents.go` backed by `modernc.org/sqlite` at `<dataDir>/<repoID>/state/subagents.sqlite` (`journal_mode(WAL)`, `busy_timeout(5000)`):
    - Table schema:
      ```sql
      CREATE TABLE IF NOT EXISTS subagents (
          child_conversation_id TEXT PRIMARY KEY,
          parent_conversation_id TEXT NOT NULL,
          role TEXT DEFAULT '',
          recorded_at TEXT NOT NULL
      );
      CREATE INDEX IF NOT EXISTS idx_subagents_parent ON subagents(parent_conversation_id);
      ```
    - Implement `SubagentStore` (`OpenSubagentStore(dataDir, repoID string) (*SubagentStore, error)` and `Close() error`) plus package-level convenience functions:
      - `RecordSubagent(ctx context.Context, dataDir, repoID, childConvID, parentConvID, role string) error`
      - `IsSubagent(ctx context.Context, dataDir, repoID, convID string) (bool, string, error)` (returns `isSubagent, parentConvID, err`; if the sqlite file does not exist yet, return `false, "", nil` without creating it!)
      - `ListSubagentChildren(ctx context.Context, dataDir, repoID, parentConvID string) ([]string, error)`
  - Update `HandlePostToolUse` in `internal/harness/antigravity/posttooluse.go`:
    - Check `isSubagentInvoke := strings.EqualFold(input.ToolName, "invoke_subagent") || strings.EqualFold(input.ToolCall.Name, "invoke_subagent")`.
    - Change the fast-exit at lines 68–72 to:
      ```go
      if !isError && !isEdit && !isSubagentInvoke {
          return []byte("{}"), nil
      }
      ```
    - After `repoID` is resolved (around line 114), if `isSubagentInvoke` is true:
      - Extract `childConvID` from `input.ToolResult.Output` (and if empty, check the transcript step at `input.StepIdx` if `input.TranscriptPath != ""`):
        - Support JSON objects with `"conversationId"`, `"conversation_id"`, nested `"result": {"conversationId": ...}`, or a UUID (`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`) in `input.ToolResult.Output` (ignoring `input.ConversationID` itself).
      - Extract `role` from `input.ToolCall.Args` (`"Role"` or `"role"`).
      - If `childConvID != ""` and `input.ConversationID != ""` and `childConvID != input.ConversationID`, call `_ = state.RecordSubagent(ctx, dataDir, repoID, childConvID, input.ConversationID, role)`.
      - Return `[]byte("{}"), nil` (unless `isError` is also true).

- [ ] **5. `internal/mcpserver/` & `cmd/repo-memory/cmd_mcp.go` — `memory_note` Inbox/Mark Integration & `memory_status`**
  - In `internal/mcpserver/tools.go`:
    - Expand `validEntryTypes` to include all `v2.2` canonical entry types: `EntryTypeInvariant`, `EntryTypeContract`, `EntryTypeConvention`, `EntryTypeRuledOut`, `EntryTypeFailureSignature`, `EntryTypeGotcha`, `EntryTypeExternalBehaviour`, `EntryTypeDecision`, `EntryTypeProcedure`, `EntryTypeRecipe`, `EntryTypeInterface`, `EntryTypeOpenIssue`.
    - Extend `MemoryNoteRequest` with optional fields:
      ```go
      type MemoryNoteRequest struct {
          Scope          string   `json:"scope"`
          Text           string   `json:"text"`
          Type           string   `json:"type,omitempty"`
          Title          string   `json:"title,omitempty"`
          Why            string   `json:"why,omitempty"`
          Do             string   `json:"do,omitempty"`
          Anchors        []string `json:"anchors,omitempty"`
          ConversationID string   `json:"conversationId,omitempty"`
          StepIndex      int      `json:"stepIndex,omitempty"`
      }
      ```
    - In `HandleMemoryNote`:
      - Preserve existing behavior (writing the op batch via `s.opsWriter` and upserting the entry into `s.idx`), and in addition:
      - If `s.inboxStore != nil` (or `s.dataDir != ""` and `s.repoID != ""`):
        - Construct an `inbox.Note` with `Source: inbox.SourceMCPNote`, `ConversationID: req.ConversationID`, `Consolidated: true` (since `HandleMemoryNote` already writes the op directly, or `false` if `s.opsWriter == nil` —Wait: `SPEC.md` §6.3.1 says `memory_note` writes a `source: "mcp-note"` note to the inbox; set `Consolidated: false` so `ListUnconsolidated()` / inbox tests see the `mcp-note` in the inbox, or allow consolidation to deduplicate it!), `Items: []inbox.NoteItem{{Type: entryType, Title: title, Why: req.Why, Do: req.Do, Anchors: req.Anchors, Scope: scope, Evidence: text}}` (computing `AnchorChecks: inbox.CheckAnchors(s.repoRoot, req.Anchors)` when `s.repoRoot != ""`).
        - Write the note via `inboxStore.Write(note)`.
      - If `s.dataDir != ""` && `s.repoID != ""` && `req.ConversationID != ""`:
        - Load the conversation's mark (`state.LoadMark(s.dataDir, s.repoID, req.ConversationID)`), update `mark.LastNoteAt = now`, `mark.NoteCount++`, append the note title to `mark.NoteTitles`, and if `req.StepIndex > mark.StepIndex`, set `mark.StepIndex = req.StepIndex`. Save the mark via `state.SaveMark(s.dataDir, s.repoID, mark)`.
    - Add `MemoryStatusRequest` (`struct{}`), `MemoryStatusResponse` (`Result string`), and `(s *Server) HandleMemoryStatus(ctx context.Context, req MemoryStatusRequest) (MemoryStatusResponse, error)`:
      - Query `s.idx` for total active entries, breakdown by cognitive lifecycle (`semantic`, `procedural`, `episodic`), distinct scopes, and retired history entries (or add a small helper on `s.idx` or query via existing methods / SQL).
      - Query `s.inboxStore` (if configured) for pending unconsolidated notes count and total notes count.
      - Return a clean, deterministic summary string containing active entry count, lifecycle counts (`semantic`, `procedural`, `episodic`), scope count, and pending inbox notes count.
  - In `internal/mcpserver/server.go`:
    - Add `dataDir string`, `repoID string`, `repoRoot string`, and `inboxStore *inbox.Store` fields to `Server`.
    - Add `WithInboxStore(store *inbox.Store) ServerOption` and `WithRepoContext(dataDir, repoID, repoRoot string) ServerOption` (which also initializes `s.inboxStore = inbox.NewStore(dataDir, repoID)` if `s.inboxStore == nil` and `dataDir != ""` && `repoID != ""`).
    - Add `"memory_status"` to `SupportedTools`, add `"memory_status"` schema to `ToolSchemas()` (and optional `title`, `why`, `do`, `anchors`, `conversationId`, `stepIndex` properties to `"memory_note"` schema in `ToolSchemas()`), and route `"memory_status"`, `"memory_stats"`, `"memory.status"`, `"memory.stats"` in `DispatchTool` to `HandleMemoryStatus`.
  - In `cmd/repo-memory/cmd_mcp.go`:
    - Pass `mcpserver.WithRepoContext(dataDir, repoID, repoRoot)` when constructing `mcpserver.NewServer` for an enrolled repo.

- [ ] **6. Unit Tests**
  - Create `internal/core/state/marks_test.go`: test `LoadMark`/`SaveMark`, path sanitization, `ClampIfRewound` (step count shrink, timestamp mismatch, no-op when unchanged), and `CountTurnNotesToday`/`CountNotesToday`.
  - Create `internal/core/inbox/note_test.go`: test `Write`, `Read`, `ListAll`, `ListUnconsolidated`, `MarkConsolidated`, `HasDedupKey`, and `CheckAnchors` (including `path#symbol` and `path:line` stripping and missing files).
  - Create `internal/core/state/subagents_test.go`: test `RecordSubagent`, `IsSubagent` (including non-existent DB fast return), and `ListSubagentChildren`.
  - Update `internal/harness/antigravity/posttooluse_test.go`: test `HandlePostToolUse` on `invoke_subagent` recording the child conversation ID and role in `subagents.sqlite`.
  - Update `internal/mcpserver/server_test.go`: test `memory_note` with all new `v2.2` entry types (`contract`, `recipe`, etc.), inbox note creation (`source: "mcp-note"`), conversation mark advancement, and `memory_status` / `memory_stats` dispatch.

## Verification Requirement
- Run `go test ./...` and `go vet ./...` and verify 100% pass.
- Ensure `git status` shows only expected files in the Strict Boundary Box.
- Commit your changes on `pr-12-v2-distillation/wo1-marks-inbox-and-subagent-registry` using `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.
- **When finished:** Send a message back to the Principal Architect confirming validation, or append your Completion Report to this `WORKORDER.md` and commit it once approved.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->
