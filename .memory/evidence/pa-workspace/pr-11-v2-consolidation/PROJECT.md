# PR-11: Consolidation — Echo Filter, LLM Ops & Publish

> **Programme:** repo-memory
> **PR name:** pr-11-v2-consolidation
> **Base branch:** `PR-10-v2-distillation`
> **Status:** ☐ Planned
> **SPEC ref:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan ref:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Consolidate turn notes into dossier updates: filter echoes using exposure lists (P13), route by anchors and scopes, merge exact duplicates, decide dossier changes via the lean agent without page rewrites, validate every op in Go and append to the op log, re-render dossiers and lint caps, and publish via the two-lane model (local branch-scoped notes vs shared memory PR). Wire into the tick orchestration and validate consolidation quality (X4) and instruction-following robustness (X10).

## Design & SPEC Sections

- **v2 Design Document:**
  - [§6 (Consolidation)](../../../docs/design-v2-distillation-retrieval.md#6-consolidation): Echo filtering via exposure lists (step 1), anchor/scope routing (step 2), exact duplicate merging (step 3), stale anchor checking (step 4), lean-agent op decisions (step 5), Go op validation and recording to op log (step 6), two-lane publish and branch scoping (step 7).
  - [§5.6 (Keeping processing models on task)](../../../docs/design-v2-distillation-retrieval.md#56-keeping-processing-models-on-task): Defensive JSON container framing, step citations, and output validation preventing instruction drift during consolidation.
- **SPEC v0.10:**
  - §6.8 (Two-lane publish: local lane immediately active, shared lane via memory PR).

## Dependencies & Gate Preconditions

- **Base branch:** `PR-10-v2-distillation`
- **Dependencies:** PR-10 (turn notes, marks, subagent registry, preparation layer, lean agent) and PR-08 (exposure lists from delivery channels).
- **Gate Preconditions:** Early kill gate (X6-lite from PR-09) must have passed prior to PR-10; PR-10 exit criteria met.

## Work Order Ledger

| # | WO name | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | `wo1-echo-filter` | P13 echo filter (v2 §6 step 1), using exposure lists from PR-08. A repeat of an exposed entry is a no-op, unless the session changed an anchored file and then passed a build, test or the entry's check. | standard | ☐ Planned |
| 2 | `wo2-routing-dedup-anchors` | Route by anchors and scopes (derived scopes proposed through the memory PR); merge exact duplicates; flag existing entries whose anchors no longer resolve, for the LLM step. | standard | ☐ Planned |
| 3 | `wo3-llm-decide` | The lean agent decides the ops; the input is JSON records (§5.6); several dossiers per prompt, or `stream-json`. The LLM never rewrites a page. | architect | ☐ Planned |
| 4 | `wo4-validate-record` | Validate every op in Go; append to the op log; re-render; lint caps. | standard | ☐ Planned |
| 5 | `wo5-publish` | Two-lane publish (SPEC §6.8), reusing the PR-05a local-publish worktree. Branch scoping: notes from unmerged branches stay local and move to shared on merge, using the anchor checks recorded at collection. | architect | ☐ Planned |
| 6 | `wo6-orchestration` | Wire into the tick: run on ≥ 10 notes or daily; skip on the daily cap, or when the quota guard reports the pool below threshold. **X4** over one week of notes, every op reviewed. | standard | ☐ Planned |

## Exit Criteria

- **X4:** ≥ 80% of ops accepted; no echo counted as confirmation; caps respected.
- **X10:** holds for consolidation inputs.
- Two-lane publish works; branch-scoped notes stay local until merge.

## Human Tasks

- Review memory PRs; answer weekly review reports (PR-11 onward, ongoing).
- In wo6: Review every op over one week of notes for the X4 evaluation.

## Risks

- **Echoes come back as confirmation:** Pushed entries repeated in agent notes could be mistakenly treated as corroboration. *Mitigation:* Exposure lists from PR-08; P13 echo filter (a repeat of an exposed entry is a no-op unless the anchored file was modified and passed a build/test). (Owner: PR-08, PR-11)
- **Processing models follow instructions in their input:** Models processing instruction-heavy inputs drift into executing them instead of consolidating. *Mitigation:* §5.6 layered defences; lean agent with no tools; X10 test suite; JSON-delimited containers and strict Go output validation. (Owner: PR-10, PR-11)
- **Agents write wrong or self-serving notes:** Notes contain plausible but invented reasons. *Mitigation:* Evidence field; `verified` only from tests, commands, merges or a human; memory-PR review; anchor checks at collection and delivery. (Owner: PR-10, PR-11)
- **Notes from a branch that never merges:** Branch-specific notes pollute shared knowledge. *Mitigation:* Kept local and scoped to that branch until merged. (Owner: PR-11)

## Session Log

<!-- Empty, ready for use -->
