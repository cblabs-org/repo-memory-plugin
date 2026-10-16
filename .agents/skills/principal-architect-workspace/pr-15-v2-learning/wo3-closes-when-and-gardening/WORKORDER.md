# Work Order 3: `closes_when` Auto-Retirement, Pull-Only Open Issues & Dossier Gardening (`wo3-closes-when-and-gardening`)

> **Project:** [PR-15-v2-learning](../PROJECT.md)
> **Branch:** `pr-15-v2-learning/wo3-closes-when-and-gardening`
> **Base Branch:** `pr-15-v2-learning/wo2-procedural-validation-and-skill-graduation`
> **Complexity:** `standard`
> **Status:** 🔄 Active

---

## 1. Objective & Scope

Implement **SPEC §5.3.2 & `design-v2` §5.2 (`closes_when` deterministic evaluation & pull-only `open-issue`)** and **`design-v2` §6 (Dossier Gardening & `repo-memory garden` CLI)**:

1. **Deterministic `closes_when` Evaluator (`internal/core/tick/closes_when.go` & `internal/core/tick/pipeline.go`):**
   - Implement `EvaluateClosesWhenCondition(ctx context.Context, repoRoot, dataDir, repoID string, epStore *episode.Store, entry dossiers.Entry) (bool, string)` supporting single conditions and compound conditions (`&&` / `;` for AND, `||` for OR):
     - `path_exists:<relPath>` (alias `file_exists:<relPath>`): satisfied when `<repoRoot>/<relPath>` exists on disk (if `<relPath>:<Symbol>` is specified, the file must exist and contain `<Symbol>`).
     - `path_absent:<relPath>` (aliases `file_absent:<relPath>`, `deleted:<relPath>`): satisfied when `<repoRoot>/<relPath>` does not exist on disk (or if `<relPath>:<Symbol>` is specified, either the file is absent or no longer contains `<Symbol>`).
     - `file_contains:<relPath>:<pattern>`: satisfied when `<repoRoot>/<relPath>` exists and contains `<pattern>` (literal substring match, or regex match if prefixed with `re:`).
     - `file_not_contains:<relPath>:<pattern>`: satisfied when `<repoRoot>/<relPath>` is absent or does not contain `<pattern>`.
     - `branch_merged:<branch>`: satisfied when `<branch>` is merged into `main`/`master`/`HEAD` (using `consolidate.DefaultBranchMergedChecker(repoRoot)`).
     - `workorder_complete:<path_or_slug>`: satisfied when the target `WORKORDER.md` (at `<repoRoot>/<path_or_slug>` or `.agents/skills/principal-architect-workspace/**/<slug>/WORKORDER.md`) contains `Status: ✅ Complete` or `## Completion Report (` (with a non-template commit SHA) or when the parent `PROJECT.md` marks `<slug>` as `✅ Complete`.
     - `command_passes:<cmd>` (alias `test_passes:<cmd>`): satisfied without executing arbitrary shell commands by checking whether `skills.LoadOutcomeState(dataDir, repoID)` or any episode in `epStore` recorded after `entry.CreatedAt` has a passing (`exit 0` / `status == "success"`) command matching `<cmd>` via `skills.CommandMatchesRecipe`.
   - Implement `(p *PipelineExecutor) EvaluateClosesWhen(ctx context.Context) (*ClosesWhenResult, error)`:
     - Inspects active `open-issue` entries (from `.memory/ops/` and `.memory/local/ops/`) with non-empty `ClosesWhen`.
     - For each satisfied entry, emits an `ops.OpRetire` (`source: "tick"`, `fields: {"reason": "closes_when satisfied: " + reason}`) to the corresponding ops directory, re-projects `.memory/dossiers/`, and updates `p.idx` (if non-nil).
   - Wire `EvaluateClosesWhen(ctx)` into `PipelineExecutor.Run(ctx)` in `internal/core/tick/pipeline.go`.

2. **Pull-Only Enforcement for `open-issue` Without `closes_when` (`internal/core/delivery/engine.go`):**
   - In `Engine.filterAndVerify(entries []dossiers.Entry)` in `internal/core/delivery/engine.go`:
     - Exclude any entry where `ent.Type == dossiers.EntryTypeOpenIssue && strings.TrimSpace(ent.ClosesWhen) == ""`.
     - This guarantees `open-issue` entries without `closes_when` are pull-only (never pushed at `task-start`, `first-touch`, `first-entry`, `error`, or `gate`), while `open-issue` entries with a non-empty `closes_when` continue to be pushed at `task-start` and `first-touch`, and MCP pull tools (`memory_brief`, `memory_search`, `memory_why`) continue to return all active `open-issue` entries.

3. **Dossier Gardening (`internal/core/tick/gardening.go`, `cmd/repo-memory/cmd_garden.go`, `cmd/repo-memory/main.go`):**
   - Implement `(p *PipelineExecutor) RunGardening(ctx context.Context, opts GardeningOptions) (*GardeningResult, error)` in `internal/core/tick/gardening.go`:
     - **Step 1 — Broken-anchor cleanup:** Uses `consolidate.CheckDossierAnchors(p.repoRoot, d)` across projected dossiers.
       - For non-episodic entries (`e.EffectiveKind() != dossiers.CognitiveKindEpisodic`) where `report.AllMissing` is true, emit `ops.OpRetire` (`reason: "gardening: all anchors missing (" + strings.Join(report.MissingAnchors, ", ") + ")"`).
       - For entries where `len(report.ValidAnchors) > 0 && len(report.MissingAnchors) > 0`, emit `ops.OpAmend` pruning `anchors` to `report.ValidAnchors`.
     - **Step 2 — Near-duplicate merging:** Within each dossier, identify active entries of the same `consolidate.CanonicalEntryTypeGroup(e.Type)` that have identical normalized titles (`consolidate.NormalizeTitle(consolidate.ExtractEntryTitle(e.Text))`) or token Jaccard similarity `>= 0.75` with overlapping anchors (or both unanchored).
       - Emit `ops.OpAmend` on the earlier/canonical entry combining `anchors`, `evidence`, `verified`, and `success_count`, and emit `ops.OpRetire` on the duplicate entry (`reason: fmt.Sprintf("gardening: merged near-duplicate into %s", survivor.ID)`).
     - **Step 3 — Dossier cap split (> 3,000 tokens):** Check each dossier's estimated token count (`len(rendered) / 4` or `opts.MaxDossierTokens`, defaulting to `cfg.Budgets.DossierCapTokens` or `3000`).
       - When a dossier exceeds the token cap and its entries can be partitioned into `>= 2` sub-scopes (by anchor sub-directory prefix or entry kind/type), emit an `ops.OpSplit` operation (`fields: {"new_dossiers": newScopes, "entry_moves": moves, "split_from": d.Scope}`) and record scope aliases (`idx.UpsertAlias` when `!opts.DryRun`).
     - **Step 4 — Write & Re-project (when `!opts.DryRun`):**
       - Write generated ops as a single batch (`source: "gardening"`) to `.memory/ops/`, re-project `.memory/dossiers/` and `.memory/ATLAS.md`, reconcile `CheckDocFreshness(ctx)`, and rebuild the multi-tier index (`consolidate.RebuildMultiTierIndex`).
   - Create `cmd/repo-memory/cmd_garden.go` (`repo-memory garden [repo-dir] [--dry-run] [--json] [--max-tokens 3000]`) and register `gardenCmd` in `cmd/repo-memory/main.go`.

---

## 2. Checklist

- [ ] Implement `internal/core/tick/closes_when.go` (`EvaluateClosesWhenCondition`, `EvaluateClosesWhen`) and wire into `PipelineExecutor.Run(ctx)` in `internal/core/tick/pipeline.go`.
- [ ] Enforce pull-only delivery for `open-issue` entries without `closes_when` in `internal/core/delivery/engine.go` (`filterAndVerify`).
- [ ] Implement `internal/core/tick/gardening.go` (`GardeningOptions`, `GardeningResult`, `RunGardening`).
- [ ] Create `cmd/repo-memory/cmd_garden.go` and register `gardenCmd` in `cmd/repo-memory/main.go`.
- [ ] Write unit tests in `internal/core/tick/closes_when_test.go`, `internal/core/tick/gardening_test.go`, and `internal/core/delivery/engine_test.go`.
- [ ] Run `go build ./...`, `go vet ./...`, and `go test -race ./internal/core/tick/... ./internal/core/delivery/... ./cmd/repo-memory/...`.

---

## 3. Strict Hygiene & Git Constraints

- **Git Author Identity:** All commits MUST use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.
- **External Repo Hygiene:** Zero proprietary internal references, internal issue/CL prefixes, internal domain emails, or local home paths anywhere in code, comments, or tests.

---

## Completion Report Template

```markdown
## Completion Report (`wo3-closes-when-and-gardening`)
- **Commit(s):** `<sha>`
- **Files Created / Modified:**
  - `...`
- **Verification Output:**
  - `go test -race ...`
```
