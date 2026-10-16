# PR-15-v2-learning: Procedural Execution Validation, Skill Graduation, Close Conditions, Gardening & Layer 3 Eval Bridge

> **Programme:** [repo-memory](../programme-repo-memory/PROGRAM.md) ([PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md))
> **Project:** `pr-15-v2-learning`
> **Base branch:** `pr-14-v2-micro-probes-and-pilot/wo4-live-esp32-pilot-and-synthesis` (`1c6a54b`)
> **Status:** 🔄 Active — Approved for execution (`WO1`–`WO5`)
> **Specification:** [SPEC.md](../../../../SPEC.md) v1.0 (§5.3, §6.6, §7.9, §9, §11.4) + [v2.2 Design (Rev 4)](../../../../docs/design-v2-distillation-retrieval.md) (§5.2, §6, §7.3, §8) + [Addendum A (Rev 3)](../../../../docs/addendum-a-provenance-and-hardening.md) (§6 A5, §7 A6, §10 A7)

---

## Objective

Complete the `v2.2` learning, procedural graduation, and honesty/cost-guard loop (`PR-15-v2-learning`), and commit portable workstation bridge-prompt instructions + `WO4`–`WO7` workorders for `PR-10-eval-v2` (Layer 3 live multi-replicate evaluation) so Layer 3 can be pulled and executed seamlessly on a macOS workstation with `pio` and `agy` CLI installed:

1. **Layer 3 (`PR-10-eval-v2`) Portable Bridge Prompt & `WO4`–`WO7` Work Orders (`WO1`):** Commit `wo4-offline-retrieval/WORKORDER.md`, `wo5-pilot/WORKORDER.md`, `wo6-main-evaluation/WORKORDER.md`, `wo7-report/WORKORDER.md`, `.agents/skills/principal-architect-workspace/pr-10-eval-v2/chief-architect-handover.md`, blind grading rubrics (`evaluation/eval-v2/rubrics/`), and a self-contained workstation bridge prompt (`evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md`) so the project owner can `git pull` on another machine and run Stage 2a/2b live evaluation immediately.
2. **Procedural Execution Validation & Skill Graduation (`WO2` — Addendum A §10 A7):** Track command execution outcomes (`PostToolUse` exit code `0` vs non-zero) against `recipe`/`procedure` entries (`success_count`, `failure_count`, `last_succeeded_at`, `stale_suspect`), flag recipes `stale_suspect` when anchored build manifests change in git, deterministically render qualifying recipes (`render_as_skill: rm-<slug>` or `>= 2` steps with `success_count >= 2`, `failure_count == 0`, `!stale_suspect`) to `.agents/skills/rm-<slug>/SKILL.md` (`generated_by: repo-memory`), and shrink `first-touch` delivery for graduated skills to a 1-line pointer (`skill rm-<slug> covers this procedure`).
3. **`closes_when` Auto-Retirement & Dossier Gardening (`WO3` — `design-v2` §5.2 & §6):** Evaluate deterministic `closes_when` conditions on `open-issue` entries in `tick` (`path_exists`, `path_absent`, `file_contains`, `file_not_contains`, `branch_merged`, `workorder_complete`, `command_passes`), enforce pull-only delivery for `open-issue` entries without `closes_when`, and implement `repo-memory garden` (`--dry-run`, `--json`) to retire broken-anchor entries, merge near-duplicates, and propose splits when dossiers exceed the 3,000-token cap.
4. **A5 Spend Guards & A6 Freshness, Coverage Gaps & Short-Term Recall (`WO4` — Addendum A §6 & §7):** Prepend the 1-line freshness state header to `memory_brief`, `memory_why`, and `memory_search`; include unconsolidated inbox notes (`status: unconsolidated`) in pull responses (never pushed); compute per-dossier coverage/gap scores (`No memory covers <scope>...`) and `QUESTIONS.md` elicitation; and enforce A5 `min_interval` + `max_items` spend guards.
5. **Skill Lint, Doctor Integration & Full Verification (`WO5`):** Add `CheckGraduatedSkillsLint` to `repo-memory doctor` verifying that every `.agents/skills/rm-*/SKILL.md` with `generated_by: repo-memory` matches deterministic regeneration, run full `go test -race ./...`, and rebuild `~/.local/bin/repo-memory`.

---

## Architecture & Module Map

| Area | Files / Packages | Responsibility |
|---|---|---|
| Layer 3 Eval Bridge & Rubrics | `evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md`, `evaluation/eval-v2/rubrics/`, `.agents/skills/principal-architect-workspace/pr-10-eval-v2/` | Portable workstation handover, blind task rubrics, and `WO4`–`WO7` workorders |
| Procedural Validation & Skill Graduation | `internal/core/skills/graduate.go`, `internal/core/skills/tracker.go`, `internal/harness/antigravity/posttooluse.go`, `internal/core/delivery/`, `cmd/repo-memory/cmd_promote_skills.go` | `PostToolUse` exit-code tracking, build-manifest `stale_suspect` flagging, `.agents/skills/rm-<slug>/SKILL.md` deterministic rendering, and 1-line `first-touch` pointer |
| Close Conditions & Gardening | `internal/core/tick/closes_when.go`, `internal/core/tick/gardening.go`, `cmd/repo-memory/cmd_garden.go`, `internal/core/delivery/engine.go` | Deterministic `closes_when` evaluation (`OpRetire`), pull-only `open-issue` without `closes_when`, and dossier gardening (`retire`, `amend`, `split`) |
| Freshness, Gaps, Short-Term Recall & Spend Guards | `internal/core/coverage/coverage.go`, `internal/mcpserver/tools.go`, `internal/core/config/config.go`, `internal/core/tick/pipeline.go` | A6 freshness line, unconsolidated inbox note recall in pull tools, coverage/gap lines, `QUESTIONS.md`, and A5 `min_interval`/`max_items` spend guards |
| Doctor Lint & Verification | `cmd/repo-memory/cmd_doctor.go` | Deterministic skill regeneration lint check and end-to-end verification |

---

## Work Order Ledger

| # | Work Order | Branch | Scope | Complexity | Status |
|---|---|---|---|---|---|
| 1 | [wo1-layer3-eval-bridge-and-handover](wo1-layer3-eval-bridge-and-handover/WORKORDER.md) | `pr-15-v2-learning/wo1-layer3-eval-bridge-and-handover` | Commit `PR-10-eval-v2` `WO4`–`WO7` `WORKORDER.md` files, `chief-architect-handover.md`, blind grading rubrics (`evaluation/eval-v2/rubrics/`), and copy-pasteable macOS workstation bridge prompt (`evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md`). | standard | ☐ Ready |
| 2 | [wo2-procedural-validation-and-skill-graduation](wo2-procedural-validation-and-skill-graduation/WORKORDER.md) | `pr-15-v2-learning/wo2-procedural-validation-and-skill-graduation` | Implement Addendum A §10 A7: `PostToolUse` command outcome tracking (`success_count`/`failure_count`/`stale_suspect`), build-manifest `stale_suspect` check in `tick`, deterministic `.agents/skills/rm-<slug>/SKILL.md` graduation (`repo-memory promote-skills`), and 1-line `first-touch` pointer for graduated skills. | architect | ☐ Planned |
| 3 | [wo3-closes-when-and-gardening](wo3-closes-when-and-gardening/WORKORDER.md) | `pr-15-v2-learning/wo3-closes-when-and-gardening` | Implement `closes_when` evaluator in `tick` (`OpRetire` on satisfied conditions), enforce pull-only for `open-issue` entries without `closes_when`, and add `repo-memory garden --dry-run --json` (`internal/core/tick/gardening.go`). | standard | ☐ Planned |
| 4 | [wo4-coverage-gaps-short-term-recall-and-spend-guards](wo4-coverage-gaps-short-term-recall-and-spend-guards/WORKORDER.md) | `pr-15-v2-learning/wo4-coverage-gaps-short-term-recall-and-spend-guards` | Implement Addendum A §6 A5 (`min_interval` + `max_items` spend guards) and §7 A6 (1-line freshness header on MCP pull tools, short-term recall of unconsolidated inbox notes in `memory_search`/`memory_brief`/`memory_why`, coverage/gap lines, and `QUESTIONS.md` elicitation). | standard | ☐ Planned |
| 5 | [wo5-verification-and-skill-lint](wo5-verification-and-skill-lint/WORKORDER.md) | `pr-15-v2-learning/wo5-verification-and-skill-lint` | Wire `CheckGraduatedSkillsLint` into `repo-memory doctor`, add end-to-end integration tests, run `go build ./...`, `go vet ./...`, `GOOS=windows go build ./...`, `go test -race ./...`, and rebuild `~/.local/bin/repo-memory`. | standard | ☐ Planned |

---

## Exit Criteria

- `evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md`, `evaluation/eval-v2/rubrics/`, and `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo4..wo7/WORKORDER.md` are committed and self-contained for pulling on another machine.
- `PostToolUse` tracks `recipe`/`procedure` command executions (`success_count`, `failure_count`, `stale_suspect`), and build-manifest edits flag anchored recipes `stale_suspect`.
- Qualifying `recipe`/`procedure` entries graduate deterministically to `.agents/skills/rm-<slug>/SKILL.md` (`generated_by: repo-memory`), are cleaned up when retired/superseded/`stale_suspect`, and shrink `first-touch` pushes to a 1-line pointer.
- `closes_when` conditions (`path_exists`, `path_absent`, `file_contains`, `file_not_contains`, `branch_merged`, `workorder_complete`, `command_passes`) retire satisfied `open-issue` entries in `tick`; `open-issue` entries without `closes_when` are pull-only.
- `repo-memory garden --dry-run --json` and `repo-memory promote-skills --dry-run --json` work deterministically.
- `memory_brief`, `memory_why`, and `memory_search` prepend the 1-line A6 freshness state header, surface unconsolidated inbox notes (`status: unconsolidated`) on pull, and append up to 3 gap lines on `memory_brief`.
- `repo-memory doctor` verifies deterministic regeneration of `.agents/skills/rm-*/SKILL.md`.
- All unit and race tests pass (`go test -race ./cmd/... ./internal/... ./packaging/...`).

## Human Approval
- [x] 5-WO decomposition approved by project owner (2026-10-16).

## Session Log

### 2026-10-16 — Project Initialized
- Initialized `pr-15-v2-learning` stacked on `pr-14-v2-micro-probes-and-pilot/wo4-live-esp32-pilot-and-synthesis` (`1c6a54b`).
- Approved 5-WO decomposition (`WO1` Layer 3 portable bridge prompt & rubrics + `WO2`–`WO5` `v2.2` procedural learning, skill graduation, `closes_when`, gardening, and A5/A6 hardening).
