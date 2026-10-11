# Memory Value Evaluation: Proposal (v2)

**Date:** 2026-10-11
**Author:** Claude Code (Opus 5.5), reviewer
**Audience:** Chief Architect
**Status:** Proposal for review. Nothing here starts before the Stage 0 prerequisites (§8) are met.
**Background:** [`x6-lite-verdict.md`](x6-lite-verdict.md) · [`x6-memory-retrieval-and-scope-report.md`](x6-memory-retrieval-and-scope-report.md) · [`pa-memory-retrieval-effectiveness.md`](pa-memory-retrieval-effectiveness.md)

**Changes from v1:**
- The primary design is now **forward tasks at HEAD in a 2×2 factorial** (PA memory on/off × repo-memory on/off). Historical replay becomes an optional objective cross-check.
- Workspaces are **build-ready template copies with a squashed single-commit git history**, not clones.
- **Simple local remote/GitHub protection** is a precondition (§10).
- c2 (self-built memory) is **deferred**: retained transcripts cover only Sep–Oct.
- c3 (advertised pull only) is added.

---

## 1. Questions

1. **Is repo-memory better than nothing?**
2. **Is repo-memory better than the Principal Architect (PA) skill's memory approach,** especially at the scale and complexity of esp32-app-foundation?

Both are judged on **output quality** and **total token cost = read side + write side**:
- **Read side:** tokens spent doing the task.
- **Write side:** tokens spent maintaining the memory, spread across the tasks it serves.

Results are reported as a **cost/quality frontier**, not as a single score.

---

## 2. What we've learned so far

| Source | Lesson | Design response |
|---|---|---|
| X6-lite | Small repo, plus a 150-line spec-style workorder, plus an API contract: 84% no-memory baseline, no headroom. The spec does memory's job | Large repo; **short task prompts**; tasks chosen where history matters |
| X6-lite | Full agent runs are expensive per data point (~300K+ tokens) | Offline retrieval check first, pilot to measure cost, hard caps |
| X6-lite | Push delivery works after the fixes, but the ranker ignores relevance (ordered by type, ~50% duplicate injections) | Fix the ranker in Stage 0, measured offline (Stage 1) |
| PA retrieval analysis | The PA skill is advertised in every session (one of ~50 skills, described as orchestration). Its knowledge was almost never pulled. repo-memory's MCP pull tools also got 0 calls in X6-lite | Test push vs **advertised pull** (c3) explicitly |
| Transcript retention | Retained esp32 transcripts cover only Sep–Oct | c2 deferred; evaluate distillation prospectively |

---

## 3. Testbed: esp32-app-foundation

| Property | Value (measured 2026-10-11) |
|---|---|
| Source | ~85K lines of C/C++ in 387 files (excluding `managed_components/`, `lib/`) |
| History | 1,270 commits (2026-05-21 → 2026-10-06); 174 closed issues; 101 issue-linked commits, 15 of them touching native tests |
| Build | PlatformIO; `env:native` googletest; 187 files under `test/` |
| Build needs gitignored generated files | Root `CMakeLists.txt` and `dependencies.lock`; `include/publicKey.hpp`, `include/portal_assets/`; `modules/LVGLDisplay/ui/`; `lib/Redactor/CMakeLists.txt`; `modules/ButtonModule/idf_component.yml`; `managed_components/` (566MB). **A fresh clone does not build without regeneration; a copy of the working tree does** |
| PA workspace | 105 project dirs, 435 WORKORDERs; ~4.8M characters (~1.2M tokens); `memory/` indexes ~45KB |
| Instruction layer | `AGENTS.md` / `CLAUDE.md` / `GEMINI.md` (~2.7KB) + `.agents/rules/` (7 files, 44KB) |
| Plugins in the working tree | `.agents/plugins.json` enables the **BugFixer daemon** workspace plugin |
| Remote | `https://github.com/matthewmcneill/esp32-app-foundation.git` (private) |

---

## 4. Design

### 4.1 Primary: forward tasks at HEAD, 2×2 factorial

Every condition starts from the same build-ready copy of esp32 at HEAD and receives the same short task prompt. They differ only in their knowledge sources:

| Condition | PA workspace + PA skill | repo-memory | Answers |
|---|:---:|:---:|---|
| **(a) Nothing** | off (workspace deleted, skill disabled) | off | Baseline |
| **(b) PA** | on (as the workflow normally has it; not directed) | off | Is PA better than nothing? |
| **(c) repo-memory, push** | off | on: hooks push entries from an index seeded from the PA workspace at HEAD | Is repo-memory better than nothing (Q1), and than PA (Q2, with write cost)? |
| **(d) Both** | on | on | Are they additive? |
| **(c3) repo-memory, advertised pull** *(variant of c)* | off | on, but **no push**: reachable only through retrieval-described MCP tools or a skill ("before changing module X, check what was tried and decided") | Is a well-advertised pull enough, and cheaper or more precise? Also records **uptake rate** |

**Notes:**
- **"PA skill disabled"** in (a) and (c): even with the workspace removed, a globally available PA skill can trigger and send the agent looking for memory, potentially in the real checkout. Disable it with a local override. Verify each condition's skill list with a one-off "list your available skills" check.
- **The instruction layer** (`AGENTS.md`, rules) is identical in all conditions. Verify it holds no project knowledge that should count as memory.
- **(c2), memory self-built from captured sessions,** is deferred. Run it prospectively: capture from now on, and evaluate on tasks raised after a few weeks of capture.

### 4.2 Tasks

- **Selection:**
  - 2–3 forward tasks, each plausible in one session.
  - Each touches areas with **rich history in the PA workspace and memory**, e.g. "add a display module for \<panel\>" (LVGL, CrowPanel, display driver history), or an OTA or portal feature.
  - Each must be buildable, and partly checkable with native tests.
- **Prompt:** short and issue-style, a few lines. No design detail, no API contract, so headroom is left for memory to matter. Identical across conditions. **Authored by the project owner before any run,** and not revised after seeing results.
- **Instruction appended in every condition:** "Work locally. Do not push, open PRs, or comment on GitHub." (§10).

### 4.3 Optional cross-check: historical replay

A few natively testable bug fixes (from the 15), replayed at their parent commits with the original issue text, scored objectively by the fix's tests. Use it only if budget remains after §4.1. It needs per-issue templates (§9) rather than the single HEAD template.

---

## 5. Metrics

| Dimension | Metric | How |
|---|---|---|
| **Quality: objective** | Builds | `pio run -e <target env>` succeeds on the agent's final tree |
| | Regression | Existing `env:native` suite still passes |
| | New tests | Any agent-added tests pass (reported, not trusted alone) |
| **Quality: rubric** | Conventions and pitfalls checklist | Per task, **written before any run and blind to condition**, from project rules and reviewer knowledge: module registration, lifecycle, house style, portal design rules, known display/LVGL/TWDT gotchas, etc. Graded by an LLM judge (blind to condition) with a fixed rubric; ≥25% hand-checked by the project owner |
| **Efficiency** | Read tokens | Billable / quota-weighted, **including cache reads** (§6) |
| | Steps | Tool calls; wall time |
| | Revision cycles | Number of build- or test-fail → edit iterations within the run (from the transcript) |
| **Precision** | Push precision (c, d) | Injected entries rated relevant/irrelevant to the task (ledger + labels); harm rate (stale or false entries) |
| | Pull behaviour (b, c3) | Uptake (did the agent consult memory?) and relevance of what it read |
| **Write cost** | Amortised maintenance tokens | §6.2 |

---

## 6. Token accounting

### 6.1 Where tokens go

| | Write side (maintenance) | Read side (per task) |
|---|---|---|
| **PA** | LLM: distilling into `PROJECT.md`, `MEMORY.md` and the indexes | LLM: `SKILL.md` + the mandatory `MEMORY.md` + `PROJECT.md` + the model's own reasoning about what to page in + the docs read. **Retrieval is the LLM** |
| **repo-memory (seeded)** | PA's write cost + ~0 (deterministic Go seeder) | Injected entry text only. A deterministic ranker selects them, at no token cost |
| **repo-memory (c2, later)** | LLM: the distiller over captured transcripts | As above |

The trade-off: PA spends tokens to retrieve intelligently; repo-memory retrieves for free, but only as well as its ranker.

**Count carried context.** Anything read or injected early is re-sent each turn as cache reads. X6-lite's `total_tokens` excluded these (~2M per run), so measure billable/quota-weighted tokens including cache reads.

### 6.2 Write-side measurement (no agent runs)
- **PA:** estimate from retained PA-workflow transcripts (tokens in steps writing under `.agents/skills/principal-architect-workspace/`), normalised per workorder. Optionally log it going forward for 2–4 weeks.
- **repo-memory (seeded):** ~0 on top of PA.
- **c2:** measured when it's evaluated prospectively.

---

## 7. Analysis plan (pre-register before each stage)

- **Unit:** task × replicate, with conditions compared within a task.
- **Estimates:**
  - mean paired differences for rubric score, build/regression success, read tokens and revision cycles;
  - 95% bootstrap CIs over tasks × replicates, reported as estimates, given the small n.
- **Decision rules:**
  - Q1: (c) vs (a).
  - Q2: (c) vs (b) on the cost/quality frontier, including amortised write cost.
  - Thresholds are written and committed **before** Stage 2.
- **Honesty clause:** with 2–3 tasks, results are directional. The report must say so, and must not extrapolate beyond the task types tested.

---

## 8. Stages, budget and gates

| Stage | What | Agent tokens | Gate |
|---|---|---:|---|
| **0. Product readiness** | Ship X6-lite fixes; index wiring; reinstall a current binary; ranker (relevance in final score, per-session dedupe, error logging, relevance threshold) | 0 | Unit tests + smoke run with injections reaching the model |
| **0b. Workspace + protection** | Build and verify the template (§9); remote/GitHub protection (§10) and its one-time check; build the esp32 index from the PA workspace at HEAD | 0 | Template builds; **the §10 check fails both the push and the `gh` call**; index non-trivial |
| **1. Offline retrieval** | Queries = the task prompts + a sample of historical issue texts. Rate the top-k injected entries for relevance (hand labels; for historical issues, entries anchored to fix-touched files count as relevant). Compare against plain search over the PA docs and over the code | ≈0 | Pre-set precision@k / recall@k thresholds met. Otherwise fix the ranker and repeat |
| **2a. Pilot** | 1 task × (a), (c) × 1 run | measured | Harness clean (contamination, protection, archival); **measured cost per run** sets 2b's size |
| **2b. Main** | 2–3 tasks × (a), (b), (c), [(d)], [(c3)] × 2 replicates, interleaved and pre-registered | ≤ cap | Final Q1/Q2 verdicts |
| **3. Optional** | Historical replay cross-check (§4.3) | remaining cap | — |

**Total agent-token cap for Stages 2–3: ~7.5M** (X6-lite spent ~22M).

**If the pilot shows runs cost more than ~450K tokens each,** shrink 2b in this priority order: keep (a) and (c); then (b); then (c3); then (d). Cut replicates before cutting tasks.

**Track orchestration and review tokens alongside, and hold one review per gate,** not per run.

---

## 9. Workspace procedure (build-ready, isolated, reproducible)

1. **Template:** copy the esp32 working tree to `$EVAL_ROOT/template/` (`rsync -a`), **excluding**:
   - `private_key.pem` (only `scripts/deploy_release.py` uses it; the build doesn't);
   - `.pio/build/` (its `CMakeCache.txt` files embed the original absolute path); keep `.pio/libdeps` and packages;
   - `.agents/plugins.json` (it enables the BugFixer daemon plugin);
   - `.memory/.enrolled`, `.vscode/`, `.agents/tmp/` (venvs), `release/`, `releases/`, `.DS_Store`.
2. **Verify the build once:** `pio run -e <target env>` and `pio test -e native` must pass. Use an agent once to resolve any surprises, then capture the steps as `x7_template_verify.sh`.
3. **Squash the history:** `rm -rf .git && git init && git add -A && git commit -m baseline`.
   - `.gitignore` keeps the generated build files on disk, untracked.
   - There is no history to mine, and no remote (§10).
   - Git stays available, because agents and the PA workflow expect it.
4. **Condition templates** (`tpl-a`, `tpl-b`, `tpl-c`, `tpl-d`), named neutrally on disk:
   - PA off: delete `.agents/skills/principal-architect-workspace/` and install the PA-skill disable override. Re-commit so `git status` is clean.
   - repo-memory on: marked for per-run enrolment.
5. **Each run:**
   - `RUN=$(mktemp -d)` and `cp -c -R tpl-X "$RUN/repo"`. APFS clonefile is instant and uses no extra disk.
   - For repo-memory-on conditions: enrol with a per-run data dir and plugin dir, and build the index from the HEAD PA workspace outside the repo.
   - Run the agent (env and protections from §10); score; archive the `git diff baseline` + untracked files, the transcript and the ledger.
   - Delete `$RUN`.

**Reused from X6-lite v3:**
- per-run plugin and data dirs;
- the pinned binary;
- the allowlist transcript scan (any access to the real esp32 checkout or another run's directory → invalid);
- interleaved scheduling;
- `analysis_set` tagging;
- conversation backup/restore.

---

## 10. Remote and GitHub protection (simple, local)

**Why it's needed:** `gh` is logged in via the keychain with `repo`, `workflow` and `delete_repo` scopes, so the agent effectively acts as you. Git pushes over HTTPS through the only credential helper on this machine (Xcode's system gitconfig, `osxkeychain`). esp32's rules and skills (BugFixer, `todo`) encourage GitHub activity.

**Why not GitHub branch protection:** branch protection and rulesets **aren't available** for these private repos on the current plan; the API returns "Upgrade to GitHub Pro". It also wouldn't cover issues, comments, or a PR the agent opens and merges itself.

**Protection (four steps, all local, covering every repo on disk):**
1. **No remote:** a fresh `git init` in the template (§9.3), so there's nothing to push to from the copy.
2. **No credentials in the agent's environment:**
   - `GH_TOKEN=invalid`: all `gh` writes (issues, comments, PRs, merges) fail.
   - `GIT_CONFIG_NOSYSTEM=1`: drops the `osxkeychain` helper, so HTTPS pushes fail from *any* repo, including the real checkouts.
   - `GIT_TERMINAL_PROMPT=0`: fail immediately rather than hang on a prompt.
   - *(Optional)* unset `SSH_AUTH_SOCK`. The remotes are HTTPS, so SSH isn't the default route.
3. **Prompt:** "Work locally. Do not push, open PRs, or comment on GitHub." in every condition.
4. **One-time check:** in that environment, `gh api user` and a test `git push` to GitHub must both fail.

**Already in place:** X6-lite's transcript scan flags any `git push`, `gh` write or access to the real checkout, and marks the run invalid.

**Also, for the batch:** disable the global BugFixer daemon plugin and the repo-memory tick sidecar, and restore them afterwards.

**Scope:** this stops accidents and casual misuse. It isn't designed to stop a deliberately adversarial agent reading key files from disk. A separate macOS user would be needed for that, and isn't proposed.

---

## 11. Risks

| Risk | Mitigation |
|---|---|
| Too few tasks to generalise | Results reported as directional; tasks chosen where memory should matter; historical replay as a cross-check |
| Rubric bias toward one memory source | Rubric written blind, from rules and reviewer knowledge, before runs; judge blind to condition; hand checks |
| The PA-skill override changes behaviour beyond "no memory" | Verify skill lists per condition; the rest of the instruction layer is identical |
| Build drift (template becomes stale) | One template per batch, verified by script; archived with its sha256 manifest |
| Runs cost more than planned | Pilot first; the §8 shrink order; hard cap |
| Agent reaches GitHub or the real checkout | §10 (no remote, no credentials in the agent env), plus the transcript scan |

---

## 12. Decisions requested from the Chief Architect

1. Approve the forward-task 2×2 as the primary design, with (c3) as a variant and historical replay optional.
2. Approve the §9 workspace procedure and the §10 protection.
3. Approve the token cap (~7.5M for Stages 2–3), the pilot-first sizing and the shrink order.
4. Confirm Stage 0 (product fixes, ranker) comes first and sits outside the evaluation budget.
5. Nominate the task author (the project owner) and the rubric author, and set the date both are frozen before Stage 2.
6. Approve deferring c2 to a prospective evaluation.
