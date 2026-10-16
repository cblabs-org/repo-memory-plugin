# Project: `pr-14-v2-micro-probes-and-pilot` — Layer 2 Read-Side & Write-Side Micro-Probes and `esp32` 1-Task Pilot

> **Programme:** [PROGRAM.md](../programme-repo-memory/PROGRAM.md) · [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)
> **Status:** Complete (`WO1`–`WO4` complete; Step 6A Cold-Firewall PR #22 Audit `APPROVED (0 blocking, 0 major)`; Step 6B Graduated)
> **Base Branch:** `pr-13-v2-consolidation/wo6-tick-orchestration-and-verification` (`696a1b8`)
> **Head Branch / PR:** `pr-14-v2-micro-probes-and-pilot/wo4-live-esp32-pilot-and-synthesis` ([PR #22](https://github.com/cblabs-org/repo-memory-plugin/pull/22))
> **Specifications:** [SPEC.md](../../../../SPEC.md) (§6–§8, §10) · [v2.2 Design Rev 4](../../../../docs/design-v2-distillation-retrieval.md) (§5–§10) · [Evaluation v2 Proposal](../../../../docs/evaluations/memory-value-evaluation-proposal.md)

---

## 1. Objective & Testing Strategy

Validate the complete `v2.2` Read-Side (`PR-10b`, `PR-11`) and Write-Side (`PR-12`, `PR-13`) architecture through **Layer 2 Component Micro-Probes & a 1-Task `esp32` Pilot** before scaling to full multi-replicate benchmarks:

1. **`WO1` (`wo1-esp32-doc-seeding-and-linux-harness`):**
   - Make `evaluation/eval-v2/scripts/eval_run.sh` portable across Linux and macOS (replacing macOS-only `cp -c -R` with a portable clone helper using `cp --reflink=auto -a` on Linux and `cp -c -R` on macOS, and archiving `state/marks/*.json`, `evidence/notes/*.json`, and `state/freshness.json` alongside `ledgers/*.json`).
   - Stage `matthewmcneill/esp32-app-foundation` (`45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`) into `evaluation/eval-v2/template/` and `tpl-{a,b,c,d}` (gitignored), upgrade `evaluation/eval-v2/scripts/build_index.sh` to run **both** `repo-memory seed` (PA workorders, `source: pa`) **and** `repo-memory seed-docs` (curated developer guides in `docs/developer/**/*.md` and `.agents/skills/**/SKILL.md` including `framework-expert` and `creating-a-module`, `source: doc-seed`) + `repo-memory ops gc` + `repo-memory reindex`, and update `evaluation/eval-v2/SEED_REPORT.md`.
2. **`WO2` (`wo2-read-side-offline-retrieval-probe`):**
   - Build a repeatable Read-Side Micro-Probe (`evaluation/eval-v2/scripts/read_probe.go` / test runner) and execute it against the combined `esp32` (`pa` + `doc-seed`) index for both frozen forward tasks (`task-1-temperature-module.txt` and `task-2-bitcoin-price-module.txt`):
     - Verify Turn 1 `invocationNum=0` (`task-start` persistent `UserMessage`, Precision@10, token budget $\le 500$);
     - Verify Turn 1 `invocationNum=1` after simulated `PostToolUse` file touches (`DataManager`, `U8G2Display`, `WebServer`, `MCPServer`, `Application.cpp`) — confirming `InjectedSet` suppresses duplicate `task-start` while `first-entry` and `first-touch` inject the anchored PA + `doc-seed` rules via persistent `UserMessage`;
     - Verify Turn 2 post-`CHECKPOINT` compaction re-injection (`BuildReinjection` via `UserMessage`) and `DetectRewind` re-arming.
   - Record full metrics and exact injected payloads in `docs/evaluations/v2-2-read-side-micro-probe.md`.
3. **`WO3` (`wo3-write-side-distillation-consolidation-probe`):**
   - Run the complete `v2.2` Write-Side pipeline (`Stage 0 Capture/Compact` $\to$ `Stage 1a/1b/1c Distillation` $\to$ `Stage 2 Consolidation`) over real transcripts in `evaluation/repo-memory-transcripts/` (and current repo sessions):
     - Measure Stage 1 milestone detection accuracy, zero-token `CHECKPOINT`/`brain/` artifact harvest yield, and `<session-log-TOKEN>` preparation layer instruction-stripping (`X10`);
     - Measure Stage 2 `P13` Echo Filter suppressions, `VD18` Corroboration Gate downgrades vs. promotions, exact-duplicate `OpAmend` coalescing, and Five-Tier routing (`user:`, `branch:`, `repo:`);
     - Record full metrics, op precision (`X4`), and sample dossier diffs in `docs/evaluations/v2-2-write-side-micro-probe.md`.
4. **`WO4` (`wo4-live-esp32-pilot-and-synthesis`):**
   - Run the `PR-10` `WO5` 1-Task Pilot (`Task 1: DS18B20 Temperature Module`, Condition A [No-Memory] vs. Condition C [`repo-memory v2.2` Push]) under the §10 protection harness (`verify_protection.sh`, `eval_run.sh`, `check_contamination.sh`), comparing hook delivery persistence, convention adherence, build/test outcomes, steps, and token cost.
   - Synthesize the combined Read-Side, Write-Side, and Live Pilot findings in `docs/evaluations/v2-2-micro-probe-verdict.md`.

---

## 2. Work Order Decomposition (Approved)

| # | Work Order | Branch | Scope & Deliverables |
|---|---|---|---|
| **WO1** | ✅ [wo1-esp32-doc-seeding-and-linux-harness](wo1-esp32-doc-seeding-and-linux-harness/WORKORDER.md) | `pr-14-v2-micro-probes-and-pilot/wo1-esp32-doc-seeding-and-linux-harness` | Fix Linux compatibility & `state/marks/` archival in `evaluation/eval-v2/scripts/eval_run.sh`, stage `esp32-app-foundation` (`45d4b5d`) templates, upgrade `build_index.sh` to combine `seed` (`source: seed`) + `seed-docs` (`source: doc-seed`) + `ops gc`, rebuild `evaluation/eval-v2/seed/index.db`, and update `evaluation/eval-v2/SEED_REPORT.md`. |
| **WO2** | ✅ [wo2-read-side-offline-retrieval-probe](wo2-read-side-offline-retrieval-probe/WORKORDER.md) | `pr-14-v2-micro-probes-and-pilot/wo2-read-side-offline-retrieval-probe` | Implement and run the Read-Side Multi-Invocation Micro-Probe across `task-1-temperature-module.txt` and `task-2-bitcoin-price-module.txt` (`task-start`, `first-entry`, `first-touch`, `CHECKPOINT` re-injection, `DetectRewind`) against the combined `pa` + `doc-seed` `esp32` index; publish `docs/evaluations/v2-2-read-side-micro-probe.md`. |
| **WO3** | ✅ [wo3-write-side-distillation-consolidation-probe](wo3-write-side-distillation-consolidation-probe/WORKORDER.md) | `pr-14-v2-micro-probes-and-pilot/wo3-write-side-distillation-consolidation-probe` | Implement and run the Write-Side Distillation & Consolidation Micro-Probe (`X2`/`X4`/`X10`) over real `evaluation/repo-memory-transcripts/` sessions, measuring Stage 1 harvest, `P13` echo suppression, `VD18` corroboration gating, and dossier projection quality; publish `docs/evaluations/v2-2-write-side-micro-probe.md`. |
| **WO4** | ✅ [wo4-live-esp32-pilot-and-synthesis](wo4-live-esp32-pilot-and-synthesis/WORKORDER.md) | `pr-14-v2-micro-probes-and-pilot/wo4-live-esp32-pilot-and-synthesis` | Execute the protected 1-Task Pilot (`Task 1: DS18B20 Temperature Module`, Condition A vs. Condition C) via `eval_run.sh`, verify zero contamination, grade against the `esp32` module rubric, and publish the consolidated verdict in `docs/evaluations/v2-2-micro-probe-verdict.md`. |

---

## 3. Approval & Execution Ledger

- **Plan Approved:** 2026-10-16 (User approved Layer 2 Read-Side & Write-Side Micro-Probes strategy)
- **Execution Mode:** Strict 3-Tier Delegation (PA spawns 1 dedicated Tier 3 Execution Worker per Work Order; Cold-Firewall PR Audit Sub-Agent reviews the stacked GitHub PR at Step 6A before Step 6B graduation).

## 4. Session Log
- `2026-10-16`: Initialized `pr-14-v2-micro-probes-and-pilot` on base `pr-13-v2-consolidation/wo6-tick-orchestration-and-verification` (`696a1b8`).
- `2026-10-16` (`WO1` complete, `f623e65`): Added `--repo` flag to `seed`, `seed-docs`, and `ops gc`; made `eval_run.sh` and `verify_template.sh` Linux-portable (`clone_tree_portable`, `--dry-run`, `v2.2` `state/marks/`, `evidence/notes/`, and `state/freshness.json` archival); created `prepare_templates.sh` and staged `esp32-app-foundation` (`template/` and `tpl-{a,b,c,d}`); enhanced `doc_seed.go` for C++/PlatformIO interfaces, 2-file submodule scopes, and recipes; upgraded `build_index.sh` and built the combined `v2.2` corpus (`1,283` entries: `256` `seed` + `1,027` `doc-seed`, `3,203` anchors, `54` dossiers).
- `2026-10-16` (`WO2` complete, `3a8e75e`): Added `ExtractPromptTerms` in `internal/harness/antigravity/preinvocation.go` and updated `stopWords` + `EntryTypeContract` `MomentTaskStart` eligibility in `internal/core/delivery/engine.go`; implemented `TestEvalV2ReadSideMicroProbe` (`internal/harness/antigravity/eval_v2_probe_test.go`) and `evaluation/eval-v2/scripts/run_read_probe.sh` verifying all 5 read-side lifecycle phases across `task-1-temperature-module.txt` (`90.0%` Precision@10, `71` active persistent rules at edit time vs `0` in `v2.1`) and `task-2-bitcoin-price-module.txt` (`100.0%` Precision@10, `86` active persistent rules at edit time vs `0` in `v2.1`); published `docs/evaluations/v2-2-read-side-micro-probe.md`.
- `2026-10-16` (`WO3` complete, `cc13948`): Implemented `TestEvalV2WriteSideMicroProbe` (`internal/core/tick/eval_v2_write_probe_test.go`) and `evaluation/eval-v2/scripts/run_write_probe.sh` across all 25 real transcripts in `evaluation/repo-memory-transcripts/` (`83` turn boundaries, `21.69%` milestone selectivity, `62.30%` overall / `74.20%` top-session / `93.96%` PA-session `PrepareSession` payload reduction, `100%` `[repo-memory ...]` stripping and instruction-read stubbing, `100%` `FilterEchoes` suppression and `EnforceCorroborationGate` downgrade accuracy, Five-Tier routing isolation, and `memory/pending` worktree commit); published `docs/evaluations/v2-2-write-side-micro-probe.md`.
- `2026-10-16` (`WO4` complete, `ea82a0b`): Added `evaluation/eval-v2/runs/` to `.gitignore`; implemented and ran `./evaluation/eval-v2/scripts/run_pilot_verification.sh` verifying §10 protection (`verify_protection.sh`), template integrity (`verify_template.sh`), Condition A vs. Condition C staging (`eval_run.sh --dry-run`), compiled `repo-memory` binary hook execution (`PreInvocation` Inv 0 [`14` entries] -> Inv 1 [`57` new / `71` total] -> `Stop` `executionNum=0` [`continue`] -> `Stop` `executionNum=1` [note + mark harvested] -> `repo-memory consolidate` [`unconsolidated_notes: 0`]), zero transcript contamination (`check_contamination.sh`), and `eval_run.sh --archive`; published `docs/evaluations/v2-2-micro-probe-verdict.md` (`VERDICT: GO` for Layer 3 and `PR-15`).
- `2026-10-16` (`Step 6A Cold-Firewall PR Audit & Remediation`, `1c8e7a0`): Opened [PR #22](https://github.com/cblabs-org/repo-memory-plugin/pull/22); remediated all 3 Round 1 audit findings (`[BLOCKING-1]` workorder hygiene wording, `[MAJOR-1]` dynamic repo-agnostic symbol/method/fallback index `repoSymbolIndex` in `internal/core/seed/doc_seed.go`, `[MINOR-1]` consolidated `delivery.IsStopWord` and generalized `ExtractPromptTerms`); verified `APPROVED (0 blocking, 0 major)` in Round 2 Cold-Firewall PR Audit.
- `2026-10-16` (`Step 6B Graduation`): Graduated `pr-14-v2-micro-probes-and-pilot` (PR #22) into `MEMORY.md`, `PROGRAM.md`, and `PROGRAM-PLAN.md` as `Audited & Approved (Ready for Merge)`.
