# X6-lite: Closing Verdict

**Date:** 2026-10-11
**Author:** Claude Code (Opus 5.5), independent reviewer of the X6-lite programme
**Audience:** Chief Architect / Principal Architect
**Branch of record:** `pr-09-v2-x6-lite/wo4-runs`
**Evidence:** `evaluation/results/x6-lite-results-v3.jsonl` (`analysis_set` = `w6-pilot`, `w6-baseline`), `evaluation/results/archive/`, `evaluation/oracle/`

---

## Verdict

**STOP, per the pre-registered rule, because the no-memory baseline leaves no headroom.**

- On spec-style, new-package work orders with an API contract, the agent (`gemini-3.8-flash-high`) scores **84% with no memory** (42/50 hidden tests).
- The remaining failures are almost all **details the original implementer chose that the WO never specified**: exact error-message wording, output formats, tie-breaking. No memory system could have supplied them.
- So **memory's effect cannot be measured on this task type**, and the gate threshold (dossiers − no_memory ≥ 15 points) can't be reached even by a perfect memory system.

**This is not evidence that memory doesn't help.** It is evidence that well-specified new-package tasks are the wrong instrument: the specification already does memory's job.

**Validated along the way:** the evaluation harness, the delivery pipeline, and three product fixes (below).

---

## 1. Baseline (no_memory, W6 harness: API contracts + test-set replacement)

| WO | Hidden tests | Score |
|---|---:|---:|
| wo2-dossier-schema-renderer | 3/3 | 100% |
| wo4-index-migration | 9/9 | 100% |
| wo4-gates | 5/5 | 100% |
| wo3-scopes-aliases | 6/7 | 86% |
| wo1-ledger-engine | 4/5 | 80% |
| wo1-op-log | 10/13 | 77% |
| wo6-pull-tools | 5/8 | 63% |
| **Pooled** | **42/50** | **84%** |
| **Mean of per-WO fractions** | | **86.4%** |

Note: wo4-gates is from Block 1 (pre-W6). It needed no API contract fix and scored 5/5 in both conditions.

## 2. Why the gate can't be passed

The pre-registered rule: stop if mean(dossiers) − mean(no_memory) < 15 points.
- **Mean of per-WO fractions:** baseline 86.4%, so the most memory could add is **13.6 points**, even fixing everything.
- **Pooled:** baseline 84%, so 15 points needs **50/50 with zero regressions.**

Either way the stop outcome is effectively decided, and further dossier runs on this task set cannot change it.

## 3. What the remaining failures are

| WO | Failing test | Failure |
|---|---|---|
| wo1-op-log | `TestValidateOp_AllTypes` | Wants error text `"entry_id is required"`, got `"add op requires entry_id"`. The dossier run failed the same test on different wording (`"fields.text is required"` vs `"add: fields must include text"`) |
| wo1-op-log | `TestValidateBatch`, `TestMigrateV1Events` (+ `_NotDir` in (c)) | Edge cases: `version < 1`, migrating into an existing archive dir |
| wo6-pull-tools | `TestHandleMemoryBrief` / `Why` / `History` | Exact output format: header text, scope line, "superseded-by" phrasing |
| wo3-scopes-aliases | `TestDeriveModuleScopes_RollupAndNaming` | Rollup choice: expected 4 derived scopes, got 5 |
| wo1-ledger-engine | `TestRanker` | Selection detail: expected 2 entries, got 1 |
| wo4-index-migration (c) | `TestRebuildFromOpLog_EndToEnd` | Alias resolution order |

These are mostly the oracle's own wording and tie-breaking choices: the same "guess the original implementer" problem the API contract fixed for names, one level down. A few are genuine edge cases, but not enough to matter.

## 4. Dossier condition (where measured)

| WO | no_memory | dossiers | Δ tests | Δ tokens |
|---|---:|---:|---:|---:|
| wo4-index-migration | 9/9 | 8/9 | −1 | −8.6% |
| wo1-op-log | 10/13 | 10/13 | 0 | −13.1% |

- **n = 1 per cell.** No evidence of a correctness benefit.
- **The token differences are noise.** In Block 1, the per-WO token deltas ranged from −37% to +83%, with a mean of +11% in the opposite direction.

## 5. What was validated

**Evaluation harness (v3, reusable for future evals):**
- standalone clones with `git filter-repo` history sanitisation (paths and home-dir rewrite);
- content-based isolation checks;
- an allowlist transcript scan;
- a pinned build;
- per-run plugin dir and data dir;
- the cutoff-filtering index builder;
- hidden oracle tests with API contracts and test-set replacement;
- agent diff archival;
- interleaved scheduling;
- `analysis_set` tagging.

**Product fixes found by the eval** (all on `pr-09-v2-x6-lite/wo4-runs`; need a product PR):

| Fix | Commit | Problem it fixed |
|---|---|---|
| Delivery channel `systemMessage` → `ephemeralMessage` | `8d7ad7b` | agy rejected every injection (`type mismatch: cannot convert string to message`); nothing reached the model |
| Antigravity native tool-call format | `d57f561` | `first_touch` moments never fired |
| Placeholder filter in seeder | `3036eb4` | ~10% of entries were `None.` / `(pending)` / `N/A` |

**Product issues found and still open:**
- **Index wiring:** `reindex`, the tick pipeline, and the hooks/MCP each use a different index file. The live hook index has 0 entries.
- **Stale installed binary:** the installed plugin runs a 2026-10-06 binary that predates the delivery engine.
- **Ranker:**
  - task-start re-ranking throws away the bm25 relevance score, so the order is effectively by entry type;
  - ~35–60% of injections are repeats;
  - search errors are swallowed, and two runs got no task-start injection at all.
- **`about_to_edit` never fires.**
- **Pull tools unused:** MCP pull tools received 0 calls across all runs.
- **Issue #16:** clean junk entries from existing ops logs.

## 6. Scope: what X6-lite did not measure

> X6-lite measured seeded-memory delivery on a ~18K-line Go repository, using new-package work orders with detailed specifications plus an API contract. It does not measure navigation benefit on large codebases, retrieval precision at large index sizes, the session-learning loop, or tasks dominated by historical pitfalls and undocumented decisions. The result should not be generalised to those settings.

## 7. Cost

| Phase | Agent runs (incl. invalid) | Agent tokens |
|---|---:|---:|
| Run 1 (invalidated: harness defects) | 52 | 11.37M |
| v2 pilot (invalidated) | 2 | 0.55M |
| v3 pilots + Block 1 | 29 | 7.00M |
| W6 pilot | 5 | 1.81M |
| W6 baseline sweep | 4 | 1.34M |
| **Total** | **92** | **~22.1M** (budget: 7M) |

This counts the evaluated agent's tokens only. It excludes orchestration and review sessions (PA, Chief Architect, reviewer), which were substantial.

**Cancelled as no longer informative:** the Block 1 re-run (~4.8M) and the n=3 replicates (~9.6M).

## 8. Recommendations

1. **Close WO4/WO5 of PR-09 with this verdict.** Cancel the remaining X6-lite runs.
2. **Ship the product fixes** (`8d7ad7b`, `d57f561`, `3036eb4`) in a product PR. Then fix the index wiring, reinstall a current binary, and fix the ranker issues in §5. None of these depend on further evaluation.
3. **Re-scope the evaluation programme before spending more agent tokens.** Lessons for any next evaluation:
   - **Run the no-memory baseline first,** and only keep tasks scoring roughly 30–70%. That one step (≈1.3M tokens) settled what the planned 14.4M would have spent on.
   - **Prefer tasks that change existing code** (bug fixes, refactors, review findings, replayed from history). Their APIs already exist, so oracle tests need no contract, and pitfalls and past decisions are what memory holds.
   - **Gate relative to headroom** ("share of remaining failures fixed"), not as an absolute point difference.
   - **Prefer oracle assertions on behaviour and error types** over exact strings and formats.
   - **Measure retrieval quality offline first** (precision of injected entries, duplicates, harmful entries) before end-to-end agent runs. That costs almost no agent tokens.
   - **Measure all three value dimensions explicitly:** accuracy (task success), efficiency (tokens and steps to solution), and precision (of injections, and of the agent's own file and edit targeting).
4. **The natural next target is a large repository with deep history** (e.g. esp32-app-foundation), evaluated with the methodology above. It's proposed separately; see [`x6-memory-retrieval-and-scope-report.md`](x6-memory-retrieval-and-scope-report.md) (in this folder) and the forthcoming evaluation-approach review.

---

*Supersedes the interim statuses in `x6-lite-results.md`, `x6-lite-pilot-report.md` and the remediation plan. Run 1 artifacts: `docs/evaluations/run1-invalid/`.*
