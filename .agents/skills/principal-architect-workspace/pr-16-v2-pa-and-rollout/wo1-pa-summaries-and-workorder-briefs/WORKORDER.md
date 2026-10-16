# WORKORDER: WO1 — PA Summaries and Workorder Briefs (`repo-memory summary` & `repo-memory brief`)

## 1. Metadata & Branch Topology
- **Parent Project:** `pr-16-v2-pa-and-rollout`
- **Work Order Slug:** `wo1-pa-summaries-and-workorder-briefs`
- **Target Branch:** `pr-16-v2-pa-and-rollout/wo1-pa-summaries-and-workorder-briefs`
- **Base Branch:** `pr-15-v2-learning/wo5-verification-and-skill-lint`
- **Status:** `COMPLETED`

---

## 2. Context & Architectural Intent
`repo-memory-plugin` serves as the memory spine for Principal Architect (PA) workflows (`docs/design-v2-distillation-retrieval.md` §10). Two explicit PA CLI entrypoints are required:
1. **`repo-memory summary <session-id-or-path> [--json]`**: Produces a deterministic, `<= 400`-token structured summary of what a worker session accomplished (`files_touched`, `notes_captured`, `failures_encountered`, `commands_run`, `verification_outcome`, `summary_markdown`), resolving either an existing episode JSON in `<dataDir>/repos/<repoID>/evidence/episodes/<id>.json` (or `<dataDir>/<repoID>/evidence/episodes/<id>.json`) or a raw session transcript file/directory path (`transcript.jsonl` / `transcript_full.jsonl` or Claude Code JSONL).
2. **`repo-memory brief [--workorder <path>] [--scope <scope>] [--max-tokens <n>] [--json]`**: Produces a deterministic, `<= 600`-token workorder brief for a PA drafting or spawning a worker on a `WORKORDER.md`, extracting target file paths and scopes from the workorder markdown (`### Permitted Mutable Files`, backticked repository paths, and file links), combining the 1-line A6 freshness status line, scope coverage gap markers, ranked active dossier entries (`invariant`, `gotcha`, `verification`, `architecture`, `recent_change`), and unconsolidated notes matching those scopes.

 Additionally, `dossiers.EstimateTokens` in `internal/core/dossiers/lint.go` should be exported alongside `estimateTokens` so all packages (`episode`, `delivery`, `dossiers`) share one canonical token estimation helper (`(len(text) + 3) / 4`, or `0` for empty strings).

---

## 3. Strict Scope & File Boundaries

### Permitted Mutable Files
- `internal/core/dossiers/lint.go` *(export `EstimateTokens(text string) int` and delegate `estimateTokens` to it)*
- `internal/core/episode/summary.go` *(NEW)*
- `internal/core/episode/summary_test.go` *(NEW)*
- `internal/core/delivery/workorder_brief.go` *(NEW)*
- `internal/core/delivery/workorder_brief_test.go` *(NEW)*
- `cmd/repo-memory/cmd_summary.go` *(NEW)*
- `cmd/repo-memory/cmd_brief.go` *(NEW)*
- `cmd/repo-memory/main.go` *(register `summary` and `brief` in `dispatch` and `usageText`)*
- `.agents/skills/principal-architect-workspace/pr-16-v2-pa-and-rollout/wo1-pa-summaries-and-workorder-briefs/WORKORDER.md` *(append Worker Completion Report only)*

### Forbidden Files (Read-Only Context)
- `internal/core/evidence/builder.go`
- `internal/core/evidence/parser.go`
- `internal/core/coverage/coverage.go`
- `internal/core/inbox/note.go`
- `internal/core/delivery/engine.go`
- `internal/core/delivery/formatter.go`
- `internal/core/delivery/ranker.go`

---

## 4. Detailed Implementation Instructions

### A. Export `dossiers.EstimateTokens` (`internal/core/dossiers/lint.go`)
- Export `func EstimateTokens(text string) int` returning `0` if `text == ""` and `(len(text) + 3) / 4` otherwise. Keep `func estimateTokens(text string) int { return EstimateTokens(text) }` so existing package-internal callers continue compiling untouched.

### B. Worker Session Summary (`internal/core/episode/summary.go` & `summary_test.go`)
- **CRITICAL IMPORT BOUNDARY:** `internal/core/evidence` imports `internal/core/episode`. Therefore, `internal/core/episode` **MUST NOT** import `internal/core/evidence` (doing so would create an import cycle). `internal/core/episode/summary.go` may import `internal/core/dossiers`, `internal/core/inbox`, `internal/harness`, and Go standard library packages.
- Define `SessionSummary`:
  ```go
  type SessionSummary struct {
      SessionID            string   `json:"session_id"`
      Harness              string   `json:"harness,omitempty"`
      Timestamp            string   `json:"timestamp,omitempty"`
      FilesTouched         []string `json:"files_touched"`
      NotesCaptured        []string `json:"notes_captured"`
      FailuresEncountered  []string `json:"failures_encountered"`
      CommandsRun          []string `json:"commands_run"`
      VerificationOutcome  string   `json:"verification_outcome"`
      TokenCount           int      `json:"token_count"`
      SummaryMarkdown      string   `json:"summary_markdown"`
  }
  ```
- Implement `BuildSessionSummary(ep *Episode, notes []*inbox.Note, maxTokens int) *SessionSummary`:
  - Default `maxTokens` to `400` when `<= 0`.
  - Populate `FilesTouched` (deduplicated sorted or order-preserved list of `ep.TouchedFiles`), `CommandsRun` (from `ep.ToolSequence` / `ep.VerificationCommands`), `FailuresEncountered` (each formatted concisely from `ep.ToolErrors`, e.g. `fmt.Sprintf("%s (exit %d): %s", err.Command, err.ExitCode, err.StderrDigest)` or `err.Tool`), and `NotesCaptured`:
    - Combine `ep.NoteIDs` with any `notes` whose `SessionID == ep.SessionID` or whose `ID` is in `ep.NoteIDs`, formatting each matched note as `fmt.Sprintf("%s [%s] (%s): %s", n.ID, n.Kind, n.Scope, firstLine(n.Statement))` (or bare note ID if the note struct isn't provided).
  - Compute `VerificationOutcome`:
    - `"verified"` if `ep.TerminalSuccess && len(ep.VerificationCommands) > 0`
    - `"completed (unverified)"` if `ep.TerminalSuccess && len(ep.VerificationCommands) == 0`
    - `"failed"` if `!ep.TerminalSuccess && len(ep.ToolErrors) > 0`
    - `"incomplete"` otherwise.
  - Render `SummaryMarkdown` with compact deterministic markdown headers (`## Session Summary: <session_id>`, `- **Outcome:** ...`, `### Files Touched`, `### Notes Captured`, `### Failures Encountered`, `### Commands Run`) and enforce `dossiers.EstimateTokens(SummaryMarkdown) <= maxTokens` by progressively truncating long lists (`CommandsRun`, `FilesTouched`, `FailuresEncountered`, `NotesCaptured`) and appending `- ... (+N more)` or hard-capping to `maxTokens * 4` bytes if necessary. Set `TokenCount = dossiers.EstimateTokens(SummaryMarkdown)`.
- Also provide `SummarizeSessionInput(dataDir, repoID, repoRoot, target string, maxTokens int) (*SessionSummary, error)` in `internal/core/episode/summary.go`:
  - First check if `target` resolves to an existing episode in `episode.NewStore(filepath.Join(dataDir, "repos", repoID)).Get(target)` or `episode.NewStore(filepath.Join(dataDir, repoID)).Get(target)`, or if `target` is a `.json` file on disk that unmarshals cleanly into `Episode` (with `SchemaVersion == "v1"` or non-empty `SessionID` + `TouchedFiles`/`ToolSequence`).
  - Otherwise, if `target` is an existing file or directory path on disk (such as a directory containing `transcript.jsonl` / `.system_generated/logs/transcript.jsonl`, or a direct `.jsonl` transcript file), parse the JSONL lines directly inside `episode/summary.go` (supporting both Antigravity `steps` / `tool_calls` JSONL and Claude Code `tool_use` / `tool_result` JSONL) to construct an `*Episode` in-memory without importing `internal/core/evidence`.
  - Load any notes from `<dataDir>/repos/<repoID>/evidence/notes/*.json` and `<dataDir>/<repoID>/evidence/notes/*.json` (and via `inbox.ReadAll(repoRoot)` if available) that match `ep.SessionID` or `ep.NoteIDs`, and return `BuildSessionSummary(ep, matchedNotes, maxTokens)`.
- Write unit tests in `internal/core/episode/summary_test.go` covering:
  1. Stored episode lookup + matching notes + `TokenCount <= 400`.
  2. Raw transcript file / conversation directory path resolution without pre-existing episode file.
  3. Token budget enforcement (`maxTokens = 400` and tight custom budget) when an episode has 100+ touched files and errors.

### C. Workorder Brief (`internal/core/delivery/workorder_brief.go` & `workorder_brief_test.go`)
- **CRITICAL IMPORT BOUNDARY:** `internal/core/inbox` imports `internal/core/delivery`, and `internal/core/coverage` imports `internal/core/tick` (which imports `internal/core/delivery`). Therefore, `internal/core/delivery` **MUST NOT** import `internal/core/inbox`, `internal/core/coverage`, or `internal/core/tick`!
- In `internal/core/delivery/workorder_brief.go`, you may import `internal/core/dossiers`, `internal/core/index`, `internal/core/ops`, `internal/core/gitops`, and Go standard library packages.
- Define `WorkorderBrief`:
  ```go
  type WorkorderBrief struct {
      WorkorderPath       string             `json:"workorder_path,omitempty"`
      ExtractedFiles      []string           `json:"extracted_files"`
      ResolvedScopes      []string           `json:"resolved_scopes"`
      StatusLine          string             `json:"status_line"`
      CoverageGaps        []string           `json:"coverage_gaps"`
      Entries             []RankedEntry      `json:"entries"`
      UnconsolidatedNotes []BriefNoteSummary `json:"unconsolidated_notes"`
      TokenCount          int                `json:"token_count"`
      BriefMarkdown       string             `json:"brief_markdown"`
  }

  type BriefNoteSummary struct {
      ID        string `json:"id"`
      Kind      string `json:"kind"`
      Scope     string `json:"scope"`
      Statement string `json:"statement"`
      CreatedAt string `json:"created_at,omitempty"`
  }
  ```
- Implement `ExtractWorkorderPaths(markdown string) []string`:
  - Deterministically extract repository-relative file and directory paths from `WORKORDER.md` content:
    - Prioritize paths under `### Permitted Mutable Files` and `### Forbidden Files` sections, as well as backticked relative paths (e.g. `` `internal/core/tick/backfill.go` ``, `` `cmd/repo-memory/cmd_brief.go` ``) and markdown link targets (`file:///...` or relative paths).
    - Normalize paths with `filepath.ToSlash`, strip leading `./`, filter out flags (`--foo`), URLs (`https://...`), commands (`go test ./...`), and deduplicate while preserving discovery order.
- Implement `BuildWorkorderBrief(ctx context.Context, idx *index.Index, repoRoot, dataDir, repoID, workorderPath, explicitScope string, maxTokens int) (*WorkorderBrief, error)`:
  - Default `maxTokens` to `600` when `<= 0`.
  - If `workorderPath != ""`, read the file and call `ExtractWorkorderPaths(content)`.
  - Resolve `ResolvedScopes` from `explicitScope` (comma-separated if multiple) and `ExtractedFiles` (using each file path and its parent directories up to repo root, plus matching dossier scopes from `.memory/dossiers/`).
  - Load unconsolidated notes directly from JSON files in `<dataDir>/repos/<repoID>/evidence/notes/*.json`, `<dataDir>/<repoID>/evidence/notes/*.json`, and `<repoRoot>/.memory/inbox/*.json` (unmarshaling `id`, `kind`, `scope`, `statement`, `created_at`, `consolidated_at`, `superseded_by`) without importing `internal/core/inbox`. Filter to notes with empty `consolidated_at` and empty `superseded_by` whose `scope` overlaps `ResolvedScopes` or `ExtractedFiles`.
  - Build `StatusLine` using the A6 single-line format (`"memory: index current (HEAD %s) · %d notes not yet consolidated · %d entries suspect"` or `"memory: index status unknown · ..."`), checking `<dataDir>/repos/<repoID>/state/freshness.json` or `<dataDir>/<repoID>/state/freshness.json` and suspect count from `.memory/dossiers/`.
  - Compute `CoverageGaps` for the resolved scopes/directories by inspecting `.memory/dossiers/` and active notes for those scopes (emitting `coverage: <scope> has no dossier yet (N unconsolidated notes pending)` or `coverage: <scope> dossier last updated <date>, N files changed since`).
  - Query and rank active dossier entries using `RankEntries` (or `idx.LoadActiveEntries` + `idx.SearchBM25` + `RankEntries`) across `ExtractedFiles` and `ResolvedScopes`, prioritizing `invariant`, `gotcha`, `verification`, `architecture`, and `recent_change` entries.
  - Format `BriefMarkdown` deterministically within `maxTokens` (`<= 600` tokens by default):
    - Line 1: `StatusLine`
    - Coverage gap lines (if any)
    - `## Workorder Brief` (with `Scopes: ...`)
    - `### Relevant Dossier Entries` (each formatted with `[entry_id] (kind, scope) statement — provenance`)
    - `### Unconsolidated Notes` (if any matching notes exist, each `- [id] (kind, scope) statement`)
    - Enforce `dossiers.EstimateTokens(BriefMarkdown) <= maxTokens` by budgeting entries and notes so `TokenCount <= maxTokens` is strictly guaranteed.
- Write unit tests in `internal/core/delivery/workorder_brief_test.go` covering:
  1. `ExtractWorkorderPaths` extracting paths from a realistic `WORKORDER.md`.
  2. `BuildWorkorderBrief` combining status line, coverage gap markers, ranked dossier entries, and unconsolidated notes within `<= 600` tokens.
  3. `--scope` only mode (without `--workorder`) and strict token budget enforcement.

### D. CLI Subcommands (`cmd/repo-memory/cmd_summary.go`, `cmd_brief.go`, `main.go`)
- In `cmd/repo-memory/cmd_summary.go`:
  - Implement `runSummary(ctx context.Context, cfg *config.Config, args []string) error`:
    - Flags: `--json`, `--max-tokens` (default `400`), `--session` (optional flag alternative to positional `<session-id-or-path>`).
    - Accept either positional `<session-id-or-path>` or `--session <id-or-path>`. Return usage error (`exitUsage`) if neither is given.
    - Call `episode.SummarizeSessionInput(cfg.DataDir, repoID, cfg.RepoRoot, target, maxTokens)` (where `repoID` is resolved via `gitops.RepoID(cfg.RepoRoot)`).
    - When `--json` is set, output indented JSON of `SessionSummary`; otherwise print `SummaryMarkdown`.
- In `cmd/repo-memory/cmd_brief.go`:
  - Implement `runBrief(ctx context.Context, cfg *config.Config, args []string) error`:
    - Flags: `--workorder <path>`, `--scope <scope>`, `--max-tokens <int>` (default `600`), `--json`. Also allow a positional `<workorder-path>` if `--workorder` is omitted and `--scope` is omitted.
    - Return usage error (`exitUsage`) if neither workorder path nor `--scope` is provided.
    - Open index via `openIndexForQuery(cfg)` (if index doesn't exist yet, still work gracefully by reading `.memory/dossiers/` directly inside `BuildWorkorderBrief`).
    - When `--json` is set, output indented JSON of `WorkorderBrief`; otherwise print `BriefMarkdown`.
- Register `case "summary": return runSummary(ctx, cfg, subArgs)` and `case "brief": return runBrief(ctx, cfg, subArgs)` in `cmd/repo-memory/main.go`, and add both commands to `usageText`.

---

## 5. Verification & Acceptance Criteria
Run and verify all of the following commands:
1. `go build ./cmd/repo-memory`
2. `go vet ./...`
3. `GOOS=windows GOARCH=amd64 go build ./cmd/repo-memory`
4. `go test -race ./internal/core/episode/... ./internal/core/delivery/... ./cmd/repo-memory/...`
5. `go test ./...`
6. Check `git status --short` to ensure only permitted files are modified/added and no untracked build artifacts remain.

---

## 6. Worker Completion Report

### Summary of Changes
1. **`internal/core/dossiers/lint.go`**:
   - Exported `EstimateTokens(text string) int` returning `0` when `text == ""` and `(len(text) + 3) / 4` otherwise, and kept `estimateTokens(text string) int` delegating to `EstimateTokens`.
2. **`internal/core/episode/summary.go` & `internal/core/episode/summary_test.go`**:
   - Implemented `SessionSummary`, `SummaryNote`, `SummaryNoteItem`, `BuildSessionSummary(ep *Episode, notes []*SummaryNote, maxTokens int) *SessionSummary`, and `SummarizeSessionInput(dataDir, repoID, repoRoot, target string, maxTokens int) (*SessionSummary, error)` without importing `internal/core/evidence` or `internal/core/inbox` (avoiding cyclic dependencies across `episode -> inbox -> delivery -> index -> episode`).
   - Added resolution for stored `.jsonl.zst` episodes, flat `.json` episodes under `evidence/episodes/`, and raw Antigravity / Claude Code `.jsonl` transcript files and conversation directories, with strict token budget enforcement (`<= 400` tokens by default).
3. **`internal/core/delivery/workorder_brief.go` & `internal/core/delivery/workorder_brief_test.go`**:
   - Implemented `WorkorderBrief`, `BriefNoteSummary`, `ExtractWorkorderPaths(markdown string) []string`, and `BuildWorkorderBrief(ctx context.Context, idx *index.Index, repoRoot, dataDir, repoID, workorderPath, explicitScope string, maxTokens int) (*WorkorderBrief, error)` without importing `internal/core/inbox`, `internal/core/coverage`, or `internal/core/tick`.
   - Extracts paths deterministically from `WORKORDER.md` (`### Permitted Mutable Files`, `### Forbidden Files`, backticked relative paths, and `file://` links), resolves target scopes, builds the 1-line A6 freshness status line, computes scope coverage gaps, ranks active dossier entries (across SQLite index, projected op-logs, and `.memory/dossiers/`), pairs unconsolidated notes, and enforces `TokenCount <= 600` (or custom `maxTokens`).
4. **`cmd/repo-memory/cmd_summary.go`, `cmd/repo-memory/cmd_brief.go`, `cmd/repo-memory/main.go`**:
   - Implemented `summaryCmd` (`repo-memory summary <session-id-or-path> [--session <id>] [--max-tokens <n>] [--json]`) and `briefCmd` (`repo-memory brief [--workorder <path>] [--scope <scope>] [--max-tokens <n>] [--json]`) and registered both commands on `rootCmd` in `cmd/repo-memory/main.go`.

### Verification Results
- `go build ./cmd/repo-memory`: **PASS**
- `GOOS=windows GOARCH=amd64 go build ./cmd/repo-memory`: **PASS**
- `go vet ./cmd/... ./internal/...`: **PASS**
- `go test -race ./internal/core/episode/... ./internal/core/delivery/... ./cmd/repo-memory/...`: **PASS**
- `go test ./cmd/... ./internal/...`: **PASS**

