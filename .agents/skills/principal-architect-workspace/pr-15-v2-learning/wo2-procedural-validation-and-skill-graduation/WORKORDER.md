# Work Order 2: Procedural Execution Validation & Skill Graduation (`wo2-procedural-validation-and-skill-graduation`)

> **Project:** [PR-15-v2-learning](../PROJECT.md)
> **Branch:** `pr-15-v2-learning/wo2-procedural-validation-and-skill-graduation`
> **Base Branch:** `pr-15-v2-learning/wo1-layer3-eval-bridge-and-handover`
> **Complexity:** `architect`
> **Status:** 🔄 Active

---

## 1. Objective & Scope

Implement **Addendum A §10 A7 (Procedures become skills — `procedural` cognitive lifecycle)**:

1. **Index Support (`internal/core/index/entries.go`):**
   - Add `ListProceduralEntries(ctx context.Context) ([]dossiers.Entry, error)` returning all active entries where `type IN ('recipe', 'procedure')` or `kind = 'procedural'`, ordered by `id`, with anchors populated.
   - Add `ListAllEntries(ctx context.Context) ([]dossiers.Entry, error)` returning all active entries ordered by `id`, with anchors populated.

2. **Procedural Execution-Outcome Tracking & Build-Manifest Freshness (`internal/core/skills/tracker.go`):**
   - Implement command extraction and matching for `recipe` / `procedure` entries:
     - `ExtractRecipeCommands(text string) []string`: extracts shell commands from inline code spans (`` `...` ``), fenced code blocks (` ```...``` `), or explicit command lines (`$ ...` or lines starting with common CLI verbs like `go `, `git `, `pio `, `make `, `cmake `, `npm `, `pnpm `, `yarn `, `cargo `, `pytest`, `python`, `bash `, `sh `, `task `, `./`).
     - `NormalizeCommand(cmd string) string`: trims whitespace, strips leading `$ `, and collapses internal whitespace.
     - `CommandMatchesRecipe(executedCmd string, recipeCmds []string) bool`: returns `true` when `NormalizeCommand(executedCmd)` equals a normalized recipe command, starts with `<recipeCmd> `, or invokes `<recipeCmd>` as a segment of a compound shell command (`&&`, `;`, `||`).
   - Persist per-entry procedural execution state at `<dataDir>/<repoID>/state/procedural_outcomes.json` (`OutcomeState` / `EntryOutcome`):
     - Track `SucceededConversations []string`, `ConsecutiveFailedConversations []string`, `LastFailedConversation string`, `ManifestBlobSHAs map[string]string`.
   - Implement `RecordCommandOutcome(ctx context.Context, dataDir, repoID, repoRoot, convID, command string, succeeded bool) ([]string, error)`:
     - Loads active procedural entries (`recipe` / `procedure`) from `.memory/ops/` (projected via `dossiers.NewProjector`) or `index.db`.
     - For every active `recipe` / `procedure` entry whose extracted commands match `command`:
       - **On success (`succeeded == true`, exit code `0`):**
         - Increments `entry.SuccessCount`, resets `entry.FailureCount = 0`, sets `entry.StaleSuspect = false`, sets `entry.LastSucceededAt = now.UTC().Format(time.RFC3339)`.
         - Records `convID` in `SucceededConversations` (deduplicated) and clears `ConsecutiveFailedConversations`.
         - Snapshots current git blob SHAs for any build-manifest anchors on the entry into `ManifestBlobSHAs`.
         - Appends `"conv:" + convID` to `entry.Evidence` (if `convID != ""` and not already present).
       - **On failure (`succeeded == false`, non-zero exit code):**
         - Increments `entry.FailureCount`.
         - Records `convID` in `ConsecutiveFailedConversations` (if `convID == ""` use a synthetic session token so anonymous invocations still count, or deduplicate consecutive failures by session ID). If `len(ConsecutiveFailedConversations) >= 2` (two consecutive distinct sessions fail on the recipe's command), sets `entry.StaleSuspect = true`.
       - Persists an `ops.OpAmend` batch to `.memory/ops/` updating `success_count`, `failure_count`, `last_succeeded_at`, `stale_suspect`, and `evidence`, re-projects `.memory/dossiers/`, updates `index.db` (`idx.UpsertEntry`), saves `procedural_outcomes.json`, and runs `SyncGraduatedSkills` so newly qualified skills are written immediately and `stale_suspect` skills are removed immediately.
   - Implement **Build-Manifest Freshness (`CheckBuildManifestFreshness`)**:
     - `IsBuildManifestPath(relPath string) bool`: returns `true` for `Makefile`, `CMakeLists.txt`, `platformio.ini`, `sdkconfig*`, `go.mod`, `package.json`, `Taskfile.yml`, `Taskfile.yaml`, and `.github/workflows/*.yml` / `.github/workflows/*.yaml`.
     - `CheckBuildManifestFreshness(ctx context.Context, repoRoot, dataDir, repoID string, idx *index.Index) ([]string, error)`:
       - Inspects active `recipe` / `procedure` entries with build-manifest anchors.
       - A build-manifest anchor is considered modified since last verification if:
         1. It has uncommitted changes in the working tree or index (`git status --porcelain -- <path>` is non-empty and not untracked `??`), OR
         2. Its current `git hash-object <path>` differs from a previously recorded `ManifestBlobSHAs[path]` in `procedural_outcomes.json` (when `ManifestBlobSHAs[path]` was recorded), OR
         3. No `ManifestBlobSHAs[path]` was recorded yet and the file's latest git commit timestamp (`git log -1 --format=%cI -- <path>`) is strictly after the entry's `LastSucceededAt` (or `UpdatedAt`/`CreatedAt` if `LastSucceededAt == ""`).
       - Flags affected entries `stale_suspect: true` via `ops.OpAmend`, re-projects `.memory/dossiers/`, updates `idx`, and syncs graduated skills.

3. **Deterministic Skill Graduation (`internal/core/skills/graduate.go`):**
   - Implement `CountRecipeSteps(text string) int`: counts numbered steps (`1.`, `2.`), bullet steps (`- `, `* `), command lines, or semicolon/`&&`/`->`/sentence-separated imperative steps in a `recipe`/`procedure` entry.
   - Implement `QualifiesForSkill(entry dossiers.Entry, outcomes *OutcomeState) (bool, string)`:
     - Returns `(false, "")` if `entry.Retired`, `entry.StaleSuspect`, or `entry.Type != dossiers.EntryTypeRecipe && entry.Type != dossiers.EntryTypeProcedure`.
     - Qualifies if **either**:
       1. `strings.TrimSpace(entry.RenderAsSkill) != ""` (normalized to `rm-<slug>` prefix), **or**
       2. `CountRecipeSteps(entry.Text) >= 2` AND `entry.SuccessCount >= 2` AND `entry.FailureCount == 0` AND distinct succeeded conversations `>= 2` (checked via `outcomes` + `conv:` tags in `entry.Evidence`; if `outcomes` has tracked conversations for `entry.ID` and `len(SucceededConversations) < 2`, return `false`).
   - Implement `SkillSlug(entry dossiers.Entry) string`:
     - Uses `entry.RenderAsSkill` (ensuring `rm-` prefix) when set; otherwise derives a deterministic kebab-case slug `rm-<slug>` from the entry's title/first clause.
   - Implement `RenderSkillMarkdown(entry dossiers.Entry) (slug string, content string)`:
     - Pure deterministic projection of `entry` (no wall-clock timestamps):
       - YAML frontmatter with `name: rm-<slug>`, `description: <single-line title + scope/when>`, `generated_by: repo-memory`, and `source_entry_id: <entry.ID>`.
       - Markdown body with `# <Title>`, `## Procedure`, `## Anchors`, `## Provenance`, and footer:
         `Generated from [<entry.ID>] by repo-memory. Edit the dossier entry or op log, not this file.`
   - Implement `SyncGraduatedSkills(ctx context.Context, repoRoot, dataDir, repoID string, dryRun bool) (*GraduationResult, error)`:
     - Projects active entries from `.memory/ops/` (and `.memory/local/ops/` if present), loads `OutcomeState`, determines all qualifying `rm-<slug>` skills, writes/updates `.agents/skills/rm-<slug>/SKILL.md`, and removes any stale `.agents/skills/rm-*/SKILL.md` that contains `generated_by: repo-memory` and is no longer qualifying (never touching hand-authored skills without `generated_by: repo-memory`).

4. **Hook, Delivery & CLI Wiring:**
   - **`internal/harness/antigravity/posttooluse.go`:**
     - Extend fast-exit gate so `run_command` and `bash` tool executions proceed on enrolled repos.
     - Determine command string and whether execution succeeded (`status == "success"` without non-zero exit code in output vs `status == "error"` or non-zero exit code in output), and invoke `skills.RecordCommandOutcome`.
   - **`internal/core/delivery/formatter.go` & `internal/harness/antigravity/preinvocation.go`:**
     - Add `FormatEntryForMoment(entry dossiers.Entry, moment Moment, workspaceRoot string) string` and `FormatEntriesForMoment(entries []RankedEntry, moment Moment, workspaceRoot string) string`.
     - When `moment == MomentFirstTouch` and `entry` is an active, non-`stale_suspect` `recipe`/`procedure` whose graduated skill `.agents/skills/rm-<slug>/SKILL.md` exists on disk (or qualifies via `RenderAsSkill`/`SuccessCount`), render the compact 1-line pointer:
       `[repo-memory <entry.ID>] skill rm-<slug> covers this procedure: <firstLineTitle>`
     - Wire `FormatEntriesForMoment(entries, delivery.MomentFirstTouch, workspaceRoot)` into `HandlePreInvocation` in `internal/harness/antigravity/preinvocation.go`.
   - **`cmd/repo-memory/cmd_promote_skills.go` & `cmd/repo-memory/main.go` & `internal/core/tick/pipeline.go`:**
     - Add `repo-memory promote-skills [repo-dir] [--dry-run] [--json]` CLI subcommand and register it in `cmd/repo-memory/main.go`.
     - Wire `CheckBuildManifestFreshness` and `SyncGraduatedSkills` into `PipelineExecutor.Run(ctx)` in `internal/core/tick/pipeline.go`.

---

## 2. Checklist

- [ ] Add `ListProceduralEntries` and `ListAllEntries` in `internal/core/index/entries.go`.
- [ ] Create `internal/core/skills/tracker.go` (`ExtractRecipeCommands`, `NormalizeCommand`, `CommandMatchesRecipe`, `OutcomeState`, `RecordCommandOutcome`, `IsBuildManifestPath`, `CheckBuildManifestFreshness`).
- [ ] Create `internal/core/skills/graduate.go` (`CountRecipeSteps`, `SkillSlug`, `QualifiesForSkill`, `RenderSkillMarkdown`, `SyncGraduatedSkills`, `GraduatedSkillSlug`).
- [ ] Wire `PostToolUse` command outcome tracking in `internal/harness/antigravity/posttooluse.go`.
- [ ] Wire `MomentFirstTouch` 1-line skill pointer in `internal/core/delivery/formatter.go` and `internal/harness/antigravity/preinvocation.go`.
- [ ] Create `cmd/repo-memory/cmd_promote_skills.go`, register `promoteSkillsCmd` in `cmd/repo-memory/main.go`, and wire `CheckBuildManifestFreshness` + `SyncGraduatedSkills` into `internal/core/tick/pipeline.go`.
- [ ] Write comprehensive unit tests in `internal/core/skills/skills_test.go`, `internal/harness/antigravity/posttooluse_test.go`, and `internal/core/delivery/formatter_test.go`.
- [ ] Run `go build ./...`, `go vet ./...`, and `go test -race ./internal/core/skills/... ./internal/core/delivery/... ./internal/core/tick/... ./internal/harness/antigravity/... ./cmd/repo-memory/...`.

---

## 3. Strict Hygiene & Git Constraints

- **Git Author Identity:** All commits MUST use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.
- **External Repo Hygiene:** Zero proprietary internal references, internal issue/CL prefixes, internal domain emails, or local home paths anywhere in code, comments, or tests.

---

## Completion Report Template

```markdown
## Completion Report (`wo2-procedural-validation-and-skill-graduation`)
- **Commit(s):** `<sha>`
- **Files Created / Modified:**
  - `...`
- **Verification Output:**
  - `go test -race ...`
```
