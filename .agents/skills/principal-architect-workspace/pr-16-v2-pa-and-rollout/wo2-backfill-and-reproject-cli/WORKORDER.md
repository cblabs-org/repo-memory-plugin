# WORKORDER: WO2 — Historical Backfill & Op-Log Reprojection (`repo-memory backfill` & `repo-memory reproject`)

## 1. Metadata & Branch Topology
- **Parent Project:** `pr-16-v2-pa-and-rollout`
- **Work Order Slug:** `wo2-backfill-and-reproject-cli`
- **Target Branch:** `pr-16-v2-pa-and-rollout/wo2-backfill-and-reproject-cli`
- **Base Branch:** `pr-16-v2-pa-and-rollout/wo1-pa-summaries-and-workorder-briefs`
- **Status:** `COMPLETED`

---

## 2. Context & Architectural Intent
Per `SPEC.md` §14, `PROGRAM-PLAN.md` (PR-15 / PR-16), and Addendum A §6 (A5 Spend Guards):
1. **`repo-memory backfill` (retaining `backfill-captures` as a backward-compatible alias)**:
   - Backfills repository memory from existing historical artifacts:
     - `--transcripts`: imports PA workspace artifacts (`.agents/skills/principal-architect-workspace/**/PROJECT.md`, `WORKORDER.md`, `context_bridge.md`) and queues uncovered historical episodes/captures for processing.
     - `--docs`: runs incremental doc seeding (`seed.SeedDocs`) across configured/default repository docs.
     - `--git`: scans git log for revert commits (`Revert "..."` / `revert: ...`) and emits deterministic refutation/gotcha notes (`kind: "gotcha"`, `source: "backfill-git"`) anchored to the files touched by the reverted change.
   - Throttled by A5 spend guards (`SpendConfig.Jobs["backfill"]`: default `min_interval: "6h"`, `max_items: 25`, `quota_floor: 0.25`) with `--max-items`, `--force`, `--dry-run`, and `--json`.
2. **`repo-memory reproject`**:
   - Replays the `.memory/ops/` op-log (plus local branch `<dataDir>/repos/<repoID>/ops-local` and user tier `<dataDir>/user/ops` when present) under the current projector and renderer to deterministically regenerate `.memory/dossiers/*.md`, `.memory/ATLAS.md`, `.memory/QUESTIONS.md`, and graduated `.agents/skills/rm-*/SKILL.md`.
   - Verifies before writing that every pre-existing entry ID, `verified` stamp, and human decision (`decision`, `invariant`, `ruled_out`, or `source: human`/`hand-edit`) in existing `.memory/dossiers/` survives in the reprojected output (either active in `Entries` or superseded/retired in `History`), failing safely if any would be lost.
   - Supports `--dry-run` and `--json`.

---

## 3. Strict Scope & File Boundaries

### Permitted Mutable Files
- `internal/core/tick/backfill.go` *(NEW)*
- `internal/core/tick/backfill_test.go` *(NEW)*
- `internal/core/dossiers/reproject.go` *(NEW)*
- `internal/core/dossiers/reproject_test.go` *(NEW)*
- `cmd/repo-memory/cmd_backfill.go` *(UPGRADE)*
- `cmd/repo-memory/cmd_reproject.go` *(NEW)*
- `cmd/repo-memory/main.go` *(register `reprojectCmd` if not auto-registered in `init()`)*
- `.agents/skills/principal-architect-workspace/pr-16-v2-pa-and-rollout/wo2-backfill-and-reproject-cli/WORKORDER.md` *(append Worker Completion Report only)*

### Forbidden Files (Read-Only Context)
- `internal/core/tick/spend.go`
- `internal/core/dossiers/projector.go`
- `internal/core/dossiers/renderer.go`
- `internal/core/dossiers/writer.go`
- `internal/core/coverage/coverage.go`
- `internal/core/skills/graduator.go`

---

## 4. Detailed Implementation Instructions

### A. Historical Backfill (`internal/core/tick/backfill.go` & `backfill_test.go`)
- Define `BackfillOptions` and `BackfillResult` in `package tick`:
  ```go
  type BackfillOptions struct {
      RepoRoot           string
      DataDir            string
      RepoID             string
      DryRun             bool
      Force              bool
      IncludeTranscripts bool
      IncludeDocs        bool
      IncludeGit         bool
      MaxItems           int
      QuotaChecker       func(context.Context) (*telemetry.QuotaSnapshot, error)
  }

  type BackfillResult struct {
      RepoID             string   `json:"repo_id"`
      DryRun             bool     `json:"dry_run"`
      SpendGuardAllowed  bool     `json:"spend_guard_allowed"`
      SpendGuardReason   string   `json:"spend_guard_reason,omitempty"`
      EffectiveMaxItems  int      `json:"effective_max_items"`
      ItemsProcessed     int      `json:"items_processed"`
      CapturesRequeued   int      `json:"captures_requeued"`
      PAFilesImported    int      `json:"pa_files_imported"`
      EpisodesQueued     int      `json:"episodes_queued"`
      DocsSeeded         int      `json:"docs_seeded"`
      DocEntriesMutated  int      `json:"doc_entries_mutated"`
      GitRevertsScanned  int      `json:"git_reverts_scanned"`
      GitNotesCreated    int      `json:"git_notes_created"`
      CreatedNoteIDs     []string `json:"created_note_ids,omitempty"`
      ImportedPAFiles    []string `json:"imported_pa_files,omitempty"`
  }
  ```
- Implement `RunBackfill(ctx context.Context, opts BackfillOptions) (*BackfillResult, error)` (and method `(p *PipelineExecutor) RunBackfill(ctx context.Context, opts BackfillOptions) (*BackfillResult, error)`):
  - If none of `IncludeTranscripts`, `IncludeDocs`, `IncludeGit` are true, default all three to `true`.
  - Evaluate A5 spend guards via `PipelineExecutor.CheckSpendGuard("backfill", opts.Force)`:
    - Determine `effectiveMaxItems` from `opts.MaxItems` (if `> 0`, cap or override `jobCfg.MaxItems`; if `<= 0`, use `jobCfg.MaxItems`, defaulting to `25`).
    - Set `res.SpendGuardAllowed = allowed` and `res.SpendGuardReason = reason`.
    - If `!allowed && !opts.Force && !opts.DryRun`, return `res, nil` immediately without mutating state (throttled by A5 spend guard).
  - Track remaining budget `remaining = effectiveMaxItems`.
  - **1. Transcripts & Captures (`IncludeTranscripts`)**:
    - If `<dataDir>/<repoID>/jobs.sqlite` or `<dataDir>/repos/<repoID>/jobs.sqlite` exists, count (or in `!DryRun`, call `store.RequeueKind(ctx, jobs.KindCapture)`) completed/dead capture jobs and record in `CapturesRequeued`.
    - Walk `<repoRoot>/.agents/skills/principal-architect-workspace` (if it exists) for `PROJECT.md`, `WORKORDER.md`, and `context_bridge.md`.
      - Use a deterministic marker/note or episode ID based on relative path (e.g., check if an episode or note for that relative path already exists in `<dataDir>/<repoID>/state/backfill_pa.json` or `<dataDir>/<repoID>/evidence/notes/` so repeated runs are idempotent).
      - Up to `remaining` items, import new PA files (creating an episode in `episode.NewStore(dataDir, repoID)` when `!DryRun`) and increment `PAFilesImported`, `ItemsProcessed`, and append relative path to `ImportedPAFiles`.
    - Also scan flat episode JSONs in `<dataDir>/repos/<repoID>/evidence/episodes/*.json` and `<dataDir>/<repoID>/evidence/episodes/*.json` that have `len(ep.NoteIDs) == 0` and are not yet recorded as backfilled; count them in `EpisodesQueued` (up to `remaining`).
  - **2. Docs (`IncludeDocs` and `remaining > 0`)**:
    - Run `seed.SeedDocs` against `<repoRoot>/.memory/ops` with `DryRun: opts.DryRun`.
    - Record `res.DocsSeeded = docRes.FilesSeeded` and `res.DocEntriesMutated = docRes.EntriesAdded + docRes.EntriesAmend + docRes.EntriesRetire`. If `!opts.DryRun && res.DocEntriesMutated > 0`, reproject dossiers to `.memory/dossiers/` and `.memory/ATLAS.md`.
    - Deduct `docRes.FilesSeeded` from `remaining` (clamped at 0) and add to `ItemsProcessed`.
  - **3. Git Revert Commits (`IncludeGit` and `remaining > 0`)**:
    - Run `git -C <repoRoot> log --grep=Revert --grep=revert -i --format=%H%x09%s -n 50` (ignoring error if repo has no commits).
    - For each commit line where subject starts with `Revert "` or `revert:` / `revert(` (case-insensitive) and `remaining > 0`:
      - Increment `GitRevertsScanned`.
      - Compute deterministic note ID: `fmt.Sprintf("note-git-revert-%s", sha[:min(12, len(sha))])`.
      - Check if `<dataDir>/repos/<repoID>/evidence/notes/<noteID>.json` or `<dataDir>/<repoID>/evidence/notes/<noteID>.json` already exists; if so, skip (idempotent!).
      - Determine touched files of that commit via `git -C <repoRoot> diff-tree --no-commit-id --name-only -r <sha>`.
      - Infer `scope` from the first touched file's directory (or `"repo"` if top-level/empty).
      - Build an `inbox.Note` with `ID: noteID`, `Kind: "gotcha"`, `Scope: scope`, `Statement: fmt.Sprintf("Reverted commit %s: %s", sha[:min(7, len(sha))], subject)`, `Anchors: touchedFiles`, `Source: "backfill-git"`, `CreatedAt: time.Now().UTC().Format(time.RFC3339)`.
      - When `!opts.DryRun`, write the note JSON to `<dataDir>/<repoID>/evidence/notes/<noteID>.json` (and `<dataDir>/repos/<repoID>/evidence/notes/<noteID>.json` if `<dataDir>/repos/<repoID>` is used).
      - Increment `GitNotesCreated`, `ItemsProcessed`, decrement `remaining`, and append `noteID` to `CreatedNoteIDs`.
  - When `!opts.DryRun && res.ItemsProcessed > 0`, call `p.RecordSpendRun("backfill", res.ItemsProcessed*100, res.ItemsProcessed*100)` so `spend_guards.json` records `last_run_at["backfill"]`.
- Write unit tests in `internal/core/tick/backfill_test.go` covering:
  1. `--dry-run` preview across PA files, doc seeds, and a git revert commit in a temp git repo without mutating disk.
  2. Real execution creating the git revert note, importing PA files, seeding docs, and recording `spend_guards.json`, followed by an immediate second run verifying A5 spend-guard throttling (`SpendGuardAllowed == false`, `SpendGuardReason == "min_interval not elapsed"`), and a `--force` run verifying idempotency (0 duplicate notes/imports).

### B. Op-Log Reprojection (`internal/core/dossiers/reproject.go` & `reproject_test.go`)
- **CRITICAL IMPORT BOUNDARY:** `internal/core/dossiers` **MUST NOT** import `internal/core/coverage`, `internal/core/skills`, `internal/core/index`, or `internal/core/tick`. Only import `internal/core/ops` and Go standard library packages.
- Define in `package dossiers`:
  ```go
  var ErrHumanDecisionLost = errors.New("reproject would lose existing entry ID, verified stamp, or human decision")

  type LostEntryViolation struct {
      EntryID string `json:"entry_id"`
      Scope   string `json:"scope"`
      Reason  string `json:"reason"`
  }

  type ReprojectOptions struct {
      MemoryDir string   // e.g. filepath.Join(repoRoot, ".memory")
      OpsDirs   []string // e.g. [<repoRoot>/.memory/ops, <dataDir>/repos/<repoID>/ops-local, <dataDir>/user/ops]
      DryRun    bool
      AllowLoss bool
  }

  type ReprojectResult struct {
      DryRun                  bool                 `json:"dry_run"`
      DossiersProjected       int                  `json:"dossiers_projected"`
      ActiveEntries           int                  `json:"active_entries"`
      HistoryEntries          int                  `json:"history_entries"`
      PreExistingEntries      int                  `json:"pre_existing_entries"`
      VerifiedPreserved       int                  `json:"verified_preserved"`
      HumanDecisionsPreserved int                  `json:"human_decisions_preserved"`
      FilesAdded              []string             `json:"files_added,omitempty"`
      FilesUpdated            []string             `json:"files_updated,omitempty"`
      FilesUnchanged          []string             `json:"files_unchanged,omitempty"`
      FilesRemoved            []string             `json:"files_removed,omitempty"`
      LostEntries             []LostEntryViolation `json:"lost_entries,omitempty"`
      QuestionsUpdated        bool                 `json:"questions_updated,omitempty"`
      SkillsGraduated         int                  `json:"skills_graduated,omitempty"`
  }
  ```
- Implement `Reproject(ctx context.Context, opts ReprojectOptions) (*ReprojectResult, map[string]*Dossier, error)`:
  - Default `opts.MemoryDir` to `".memory"` if empty, and `opts.OpsDirs` to `[]string{filepath.Join(opts.MemoryDir, "ops")}` if empty.
  - Filter `opts.OpsDirs` to directories that exist on disk; create an `ops.NewMultiReader(existingDirs...)` (or `ops.NewReader` if single/empty) and run `NewProjector(reader).Project(ctx)`.
  - Scan existing `.memory/dossiers/**/*.md` (both active `.md` and `.history.md`) using `ParseRenderedEntries` (from `handedit.go`) and also parse metadata lines (` `...` `) for `verified <sha>` and `decision` / `invariant` / `ruled_out`.
  - Verify that every pre-existing entry ID parsed from `.memory/dossiers/` exists in the newly projected `dossiersMap` (either in `d.Entries` or `d.History` across any scope, accounting for `move`/`split`/`supersede`/`retire`):
    - If an entry ID is completely missing from the projected `Entries` + `History`, append a `LostEntryViolation{EntryID: id, Scope: oldScope, Reason: "entry_id missing from projected ops"}` (or `"human decision missing from projected ops"` if its type was `decision`/`invariant`/`ruled_out`).
    - If an entry had a non-empty `verified` stamp on disk and the projected entry exists in `Entries` with an empty `Verified` field, preserve/carry over the `Verified` stamp or flag a violation if `Verified` was lost.
    - Count `PreExistingEntries`, `VerifiedPreserved`, and `HumanDecisionsPreserved`.
  - If `len(res.LostEntries) > 0 && !opts.AllowLoss`, return `res, dossiersMap, fmt.Errorf("%w: %d entries/decisions missing", ErrHumanDecisionLost, len(res.LostEntries))`.
  - Compare rendered markdown (`RenderDossier`, `RenderHistory`, `RenderAtlas`) against existing files on disk under `opts.MemoryDir` to populate `FilesAdded`, `FilesUpdated`, `FilesUnchanged`, and `FilesRemoved` (sorted relative paths under `.memory/`).
  - When `!opts.DryRun`, call `NewWriter(opts.MemoryDir).WriteAll(dossiersMap)`.
- Write unit tests in `internal/core/dossiers/reproject_test.go` covering:
  1. Deterministic replay of `.memory/ops` producing identical `.memory/dossiers/*.md` and `.memory/ATLAS.md`, with second run reporting all files in `FilesUnchanged` and 0 in `FilesUpdated`.
  2. Preservation check catching an unrecorded hand-added decision/entry in `.memory/dossiers/` (`ErrHumanDecisionLost`) and refusing to overwrite unless reconciled or `AllowLoss: true`.
  3. `--dry-run` computing exact diff (`FilesAdded`, `FilesUpdated`) without writing to disk.

### C. CLI Subcommands (`cmd/repo-memory/cmd_backfill.go`, `cmd_reproject.go`, `main.go`)
- Upgrade `cmd/repo-memory/cmd_backfill.go`:
  - `Use: "backfill"`, `Aliases: []string{"backfill-captures"}`.
  - Flags: `--repo`, `--data-dir`, `--dry-run`, `--json`, `--force`, `--transcripts`, `--docs`, `--git`, `--max-items`.
  - If `--repo` is provided (or current working directory is inside a git repo), run `tick.RunBackfill` for that repo; if not in a git repo and `backfill-captures` was invoked with registry entries, also support iterating registered repos as fallback.
  - When `--json` is set, output indented JSON of `BackfillResult`; otherwise print a concise human-readable summary (including `Requeued N captures` when invoked via `backfill-captures` or when captures were requeued).
- Create `cmd/repo-memory/cmd_reproject.go`:
  - `Use: "reproject [repo-dir]"`.
  - Flags: `--repo`, `--data-dir`, `--ops-dir` (default `".memory/ops"`), `--dry-run`, `--json`, `--allow-loss`.
  - Resolve `repoRoot`, `dataDir`, `repoID`. Build `opsDirs` starting with `<repoRoot>/.memory/ops` (or `--ops-dir`), plus `<dataDir>/repos/<repoID>/ops-local`, `<dataDir>/<repoID>/ops-local`, and `<dataDir>/user/ops` if they exist.
  - Call `dossiers.Reproject(ctx, opts)`.
  - Also run `skills.SyncGraduatedSkills(ctx, repoRoot, dataDir, repoID, reprojectDryRun)` and record `res.SkillsGraduated = len(skillRes.Graduated)`.
  - When `!reprojectDryRun`, call `coverage.WriteQuestionsFile(ctx, repoRoot, dataDir, repoID)` and set `res.QuestionsUpdated = true` (or in `--dry-run`, check if `coverage.GenerateQuestionsMarkdown` differs from `.memory/QUESTIONS.md`).
  - When `--json` is set, output indented JSON of `ReprojectResult`; otherwise print a concise human-readable summary.
- Ensure both `backfillCmd` and `reprojectCmd` are registered on `rootCmd`.

---

## 5. Verification & Acceptance Criteria
Run and verify all of the following commands:
1. `go build ./cmd/repo-memory`
2. `go vet ./cmd/... ./internal/...`
3. `GOOS=windows GOARCH=amd64 go build ./cmd/repo-memory`
4. `go test -race ./internal/core/tick/... ./internal/core/dossiers/... ./cmd/repo-memory/...`
5. `go test ./cmd/... ./internal/...`
6. Check `git status --short` to ensure only permitted files are modified/added.

---

## 6. Worker Completion Report

### Summary of Changes
1. **Historical Backfill (`internal/core/tick/backfill.go` & `internal/core/tick/backfill_test.go`)**:
   - Implemented `BackfillOptions`, `BackfillResult`, `RunBackfill`, and `(p *PipelineExecutor) RunBackfill` enforcing A5 spend guards (`SpendConfig.Jobs["backfill"]`: `min_interval: "6h"`, `max_items: 25`, `quota_floor: 0.25`) with `--force`, `--dry-run`, and `--max-items` budgeting.
   - Implemented `--transcripts` backfill: requeues completed/dead/failed capture jobs in `jobs.sqlite`, idempotently imports PA workspace artifacts (`.agents/skills/principal-architect-workspace/**/PROJECT.md`, `WORKORDER.md`, `context_bridge.md`) into the episode store with state persisted in `<dataDir>/<repoID>/state/backfill_pa.json`, and queues uncovered historical flat episodes.
   - Implemented `--docs` backfill: invokes `seed.SeedDocs` against `.memory/ops` and reprojects dossiers when entries are mutated.
   - Implemented `--git` backfill: scans git revert commits (`Revert "..."` / `revert: ...`) and emits deterministic `gotcha` notes (`note-git-revert-<sha[:12]>`, `source: "backfill-git"`) anchored to the touched files of the reverted commit.
   - Added unit tests in `internal/core/tick/backfill_test.go` verifying `--dry-run` non-mutation, full execution across PA artifacts/docs/git reverts, A5 spend-guard throttling (`min_interval not elapsed`), and `--force` idempotency.

2. **Op-Log Reprojection (`internal/core/dossiers/reproject.go` & `internal/core/dossiers/reproject_test.go`)**:
   - Implemented `ReprojectOptions`, `ReprojectResult`, `LostEntryViolation`, `ErrHumanDecisionLost`, and `Reproject(ctx, opts)` while strictly adhering to the `internal/core/dossiers` import boundary (importing only `internal/core/ops` and Go standard library packages).
   - Replays op batches across multiple op directories (`.memory/ops`, `ops-local`, `user/ops`), verifies that all pre-existing dossier entry IDs, `verified` stamps, and human decisions (`decision`, `invariant`, `ruled-out`, or `source: human`/`hand-edit`) survive in the projected `Entries` or `History`, and aborts safely with `ErrHumanDecisionLost` unless `AllowLoss: true` is set.
   - Computes exact file diff classifications (`FilesAdded`, `FilesUpdated`, `FilesUnchanged`, `FilesRemoved`) for `.memory/dossiers/**/*.md` and `.memory/ATLAS.md`.
   - Added unit tests in `internal/core/dossiers/reproject_test.go` covering `--dry-run`, deterministic idempotent replay (`FilesUnchanged`), and `ErrHumanDecisionLost` protection against unrecorded hand-added decisions.

3. **CLI Subcommands (`cmd/repo-memory/cmd_backfill.go`, `cmd/repo-memory/cmd_reproject.go`, `cmd/repo-memory/main.go`)**:
   - Upgraded `cmd_backfill.go` to `Use: "backfill [repo-dir]"` with `Aliases: []string{"backfill-captures"}` and flags `--repo`, `--data-dir`, `--dry-run`, `--json`, `--force`, `--transcripts`, `--docs`, `--git`, and `--max-items`.
   - Created `cmd_reproject.go` (`Use: "reproject [repo-dir]"`) with flags `--repo`, `--data-dir`, `--ops-dir`, `--dry-run`, `--json`, and `--allow-loss`, coordinating `dossiers.Reproject`, `skills.SyncGraduatedSkills`, and `coverage.WriteQuestionsFile` (`.memory/QUESTIONS.md`).
   - Registered both `backfillCmd` and `reprojectCmd` on `rootCmd` in `cmd/repo-memory/main.go`.

### Verification Results
- `go build ./cmd/repo-memory`: **PASS**
- `go vet ./cmd/... ./internal/...`: **PASS**
- `GOOS=windows GOARCH=amd64 go build ./cmd/repo-memory`: **PASS**
- `go test -race ./internal/core/tick/... ./internal/core/dossiers/... ./cmd/repo-memory/...`: **PASS**
- `go test ./cmd/... ./internal/...`: **PASS**
- `git status --short`: Verified only permitted files were created/modified.
