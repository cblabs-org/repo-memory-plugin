# Work Order 4: Spend Guards, Freshness Header, Coverage Gaps & Short-Term Recall (`wo4-coverage-gaps-short-term-recall-and-spend-guards`)

> **Project:** [PR-15-v2-learning](../PROJECT.md)
> **Branch:** `pr-15-v2-learning/wo4-coverage-gaps-short-term-recall-and-spend-guards`
> **Base Branch:** `pr-15-v2-learning/wo3-closes-when-and-gardening`
> **Complexity:** `standard`
> **Status:** 🔄 Active

---

## 1. Objective & Scope

Implement **Addendum A §6 A5 (`min_interval` + `max_items` spend guards & dry-run estimates)** and **Addendum A §7 A6 (1-line freshness state header, short-term recall of unconsolidated inbox notes in pull tools, coverage/gap lines, and `QUESTIONS.md` elicitation)**:

1. **A5 Spend Guards & Configuration (`internal/core/config/config.go` & `internal/core/tick/spend.go`):**
   - Extend `internal/core/config/config.go`:
     - Add `JobSpendConfig` (`MinInterval string`, `MaxItems int`) and `SpendConfig` (`QuotaFloor float64`, `Jobs map[string]JobSpendConfig`) to `MachineConfig` and `RepoConfig`, with `DefaultSpendConfig()` providing the Addendum A §6.2 defaults:
       - `quota_floor: 0.25`
       - `fallback`: `{min_interval: "1h", max_items: 10}`
       - `consolidation`: `{min_interval: "24h", max_items: 40}`
       - `backfill`: `{min_interval: "6h", max_items: 25}`
       - `gardening`: `{min_interval: "168h", max_items: 20}`
     - Add `GapsConfig` (`Threshold float64`, `Push bool`) to `RepoConfig` (default threshold `0.5`, `Push: false`).
   - Create `internal/core/tick/spend.go`:
     - Persist per-job-kind execution state (`LastRunAt map[string]string`, `CalibrationRatios []float64`) at `<dataDir>/<repoID>/state/spend_guards.json`.
     - Implement `(p *PipelineExecutor) CheckSpendGuard(jobKind string, force bool) (allowed bool, reason string, maxItems int)` and `RecordSpendRun(jobKind string, estimatedTokens, actualTokens int)`.
     - Wire `max_items` into `RunSidecarFallback` (capping conversations per run when `max_items > 0`) and `RunConsolidationWithOptions` (capping notes processed per run when `max_items > 0` and enforcing `min_interval` when `!opts.Force` and `len(unconsolidated) < DefaultConsolidationMinNotes`).

2. **A6 Freshness Record & 1-Line State Header (`internal/core/tick/consolidate.go` & `internal/mcpserver/tools.go`):**
   - Extend `FreshnessState` in `internal/core/tick/consolidate.go` with:
     - `IndexOpLogBatchID string` (`json:"index_op_log_batch_id,omitempty"`)
     - `IndexCurrentToOpLog bool` (`json:"index_current_to_op_log"`)
     - `IndexHeadSHA string` (`json:"index_head_sha,omitempty"`)
     - `HeadMovedSinceBuild bool` (`json:"head_moved_since_build,omitempty"`)
     - `SuspectEntries int` (`json:"suspect_entries"`)
   - Update `refreshFreshnessState` in `internal/core/tick/consolidate.go` (and call it at the end of `PipelineExecutor.Run`) so `freshness.json` records `IndexOpLogBatchID`, `IndexCurrentToOpLog`, `IndexHeadSHA`, `HeadMovedSinceBuild`, `UnconsolidatedNotes`, `ActiveEntries`, and `SuspectEntries`.
   - Implement `FormatFreshnessLine(ctx context.Context, dataDir, repoID, repoRoot string, idx *index.Index, inboxStore *inbox.Store) string` producing the 1-line A6 state header:
     `memory: <index status> · <HEAD status> · <N> notes not yet consolidated · <M> entries suspect`
     (e.g., `memory: index current to op log · HEAD current · 0 notes not yet consolidated · 0 entries suspect`, or `HEAD moved since build` when current `HEAD` differs from `IndexHeadSHA`).

3. **Coverage & Gap Detection + `QUESTIONS.md` Elicitation (`internal/core/coverage/coverage.go`):**
   - Create package `internal/core/coverage` (`internal/core/coverage/coverage.go`):
     - `ScopeCoverage`: tracks `Scope`, `ActiveEntries`, `SemanticEntries`, `ProceduralEntries`, `EpisodicEntries`, `SuspectEntries`, `VerifiedEntries`, `Score float64`, `BelowThreshold bool`.
     - `CoverageReport`: tracks `Scopes []ScopeCoverage`, `UncoveredPaths []string`, `Threshold float64`.
     - `ComputeCoverage(ctx context.Context, repoRoot string, entries []dossiers.Entry, threshold float64) (*CoverageReport, error)`: computes per-scope coverage and discovers uncovered module directories / configured subsystems.
     - `FormatGapLinesForScope(scope string, activeEntries []dossiers.Entry, unconsolidatedCount int, threshold float64) []string`: returns up to 3 gap lines for `memory_brief`:
       - When a requested scope has `0` active entries and `0` unconsolidated notes:
         `No memory covers <scope>. Absence of memory is not evidence; read the code.`
       - When a requested scope has active entries but its coverage score is below `threshold` (e.g. all entries are `stale_suspect` or unverified open issues):
         `Low memory coverage for <scope> (score <score>). Absence of memory is not evidence; read the code.`
     - `GenerateQuestionsMarkdown(report *CoverageReport, entries []dossiers.Entry) string` and `WriteQuestionsFile(repoRoot string, report *CoverageReport, entries []dossiers.Entry) error`: renders deterministic `.memory/QUESTIONS.md` surfacing uncovered scopes, `stale_suspect` procedures needing verification, and open issues without `closes_when`.

4. **Short-Term Recall of Unconsolidated Inbox Notes & Pull Tool Integration (`internal/mcpserver/tools.go`):**
   - **Pull-only short-term recall (Addendum A §7.4):**
     - `HandleMemoryBrief`:
       1. Prepend the 1-line A6 freshness state header (`memory: ...`).
       2. Query `s.inboxStore` (or `inbox.NewStore(s.dataDir, s.repoID)`) for unconsolidated notes (`!note.Consolidated`) whose item scope or anchor prefix matches `scope`, and render them under an `Unconsolidated Inbox Notes (unconsolidated):` section labelled `[unconsolidated <note.ID>] (<branch>) <type>: <title> — <why/do>`.
       3. Append up to 3 gap lines from `coverage.FormatGapLinesForScope` at the end of the response.
     - `HandleMemoryWhy`:
       1. Prepend the 1-line A6 freshness state header (`memory: ...`).
       2. If `s.idx.GetEntry(ctx, entryID)` returns `ErrNotFound`, check unconsolidated notes in `s.inboxStore` by note ID (`note.ID == entryID`) or anchor/title match, returning the unconsolidated note details labelled `Status: unconsolidated` if found.
     - `HandleMemorySearch`:
       1. Search both `s.idx.SearchEntries(ctx, query)` and unconsolidated notes in `s.inboxStore` matching the query terms (labelled `[unconsolidated] <note.ID> (<scope>, branch: <branch>): <title> — <why/do>`).
       2. When results are found (or when `s.dataDir != ""` / `s.repoRoot != ""` and `query != ""`), prepend the 1-line A6 freshness state header (`memory: ...`). Preserve exact `"No results found."` when `query == ""` or when a bare in-memory server (`s.dataDir == "" && s.repoRoot == ""`) finds zero matches so all existing unit tests in `internal/mcpserver/tools_test.go` continue to pass.
     - Ensure unconsolidated inbox notes are **never** pushed by `PreInvocation`, `PreToolUse`, or `PostToolUse`.

---

## 2. Checklist

- [ ] Extend `internal/core/config/config.go` with `SpendConfig`, `JobSpendConfig`, `DefaultSpendConfig()`, and `GapsConfig`.
- [ ] Create `internal/core/tick/spend.go` (`SpendGuardState`, `CheckSpendGuard`, `RecordSpendRun`) and wire `max_items` / `min_interval` into `internal/core/tick/fallback.go` and `internal/core/tick/consolidate.go`.
- [ ] Extend `FreshnessState` in `internal/core/tick/consolidate.go` and update `PipelineExecutor.Run` to refresh `freshness.json`.
- [ ] Create `internal/core/coverage/coverage.go` (`ComputeCoverage`, `FormatGapLinesForScope`, `GenerateQuestionsMarkdown`, `WriteQuestionsFile`).
- [ ] Update `internal/mcpserver/tools.go` (`HandleMemoryBrief`, `HandleMemoryWhy`, `HandleMemorySearch`) with the 1-line A6 freshness header, unconsolidated inbox note recall, and coverage gap lines.
- [ ] Write unit tests in `internal/core/coverage/coverage_test.go`, `internal/core/tick/spend_test.go`, and `internal/mcpserver/tools_test.go`.
- [ ] Run `go build ./...`, `go vet ./...`, and `go test -race ./internal/core/config/... ./internal/core/coverage/... ./internal/core/tick/... ./internal/mcpserver/...`.

---

## 3. Strict Hygiene & Git Constraints

- **Git Author Identity:** All commits MUST use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.
- **External Repo Hygiene:** Zero proprietary internal references, internal issue/CL prefixes, internal domain emails, or local home paths anywhere in code, comments, or tests.

---

## Completion Report Template

```markdown
## Completion Report (`wo4-coverage-gaps-short-term-recall-and-spend-guards`)
- **Commit(s):** `<sha>`
- **Files Created / Modified:**
  - `...`
- **Verification Output:**
  - `go test -race ...`
```
