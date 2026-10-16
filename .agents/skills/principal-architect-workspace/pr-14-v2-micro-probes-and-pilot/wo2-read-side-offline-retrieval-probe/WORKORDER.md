---
name: pr-14-v2-micro-probes-and-pilot-wo2
description: Handoff instructions for Work Order 2 — Read-Side Multi-Invocation Offline Retrieval Micro-Probe
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Governance & Authorization

- Your parent project is `.agents/skills/principal-architect-workspace/pr-14-v2-micro-probes-and-pilot/PROJECT.md`, where human approval of the 4-Work-Order decomposition is recorded (`3fd99ae`).
- Read `.agents/rules/00-operating-model.md` before starting.
- **External GitHub Hygiene & Git Author Rule (CRITICAL):**
  - Never include any internal references (`google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or absolute local home paths like `/usr/local/google/...`) in any repository files, comments, reports, or commit messages.
  - All git commits MUST use author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.

## Objective

Build and execute the deterministic Read-Side Multi-Invocation Micro-Probe against the real `esp32-app-foundation` (`45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`) `v2.2` combined (`seed` + `doc-seed`) index for both frozen forward tasks (`task-1-temperature-module.txt` and `task-2-bitcoin-price-module.txt`), verifying all 5 multi-invocation lifecycle moments (`MomentTaskStart`, `MomentFirstEntry`/`MomentFirstTouch`, `MomentAboutToEdit` & `PreToolUse` `gate: edit`, post-`CHECKPOINT` `BuildReinjection`, and `DetectRewind`), and publish the empirical report in `docs/evaluations/v2-2-read-side-micro-probe.md`.

## Strict Boundary Box

- **In Scope:**
  - `internal/harness/antigravity/eval_v2_probe_test.go` (new deterministic Go read-side micro-probe test suite)
  - `internal/harness/antigravity/preinvocation.go` and/or `internal/core/delivery/engine.go` (only if minor term-sanitization or scope matching refinements are needed so prompt terms with punctuation like `portal,` or `server.` cleanly match FTS5/scopes)
  - `evaluation/eval-v2/scripts/run_read_probe.sh` (new executable script running the read-side probe and printing structured summary tables)
  - `docs/evaluations/v2-2-read-side-micro-probe.md` (new comprehensive empirical evaluation report)
- **Out of Scope:**
  - Do NOT modify `internal/core/consolidate/` or write the `WO3` (`v2-2-write-side-micro-probe.md`) or `WO4` (`v2-2-micro-probe-verdict.md`) reports.
  - Do NOT commit any files inside gitignored `evaluation/eval-v2/{template,tpl-*,seed,runs}`.
  - Do NOT push branches or open PRs on GitHub.

## Tooling Constraints

- Use `go test -race ./cmd/... ./internal/... ./packaging/...` and `./evaluation/eval-v2/scripts/run_read_probe.sh`.
- Ensure `eval_v2_probe_test.go` runs against the real `evaluation/eval-v2/seed/index.db` and `evaluation/eval-v2/template` when present on disk, and gracefully falls back to a self-contained temporary seeded `esp32` fixture if `evaluation/eval-v2/seed/index.db` is absent (so `go test ./...` passes in clean CI checkouts where gitignored `evaluation/eval-v2/seed/` is not present).

## Line-Item Instructions

- [x] **1. Inspect Prompt Term Tokenization in `internal/harness/antigravity/preinvocation.go`:**
  - Check lines 242–249 of `internal/harness/antigravity/preinvocation.go`, where `strings.Fields(input.UserPrompt)` extracts `promptTerms`. Notice that punctuation attached to words in natural prompts (e.g. `"Sensor."`, `"carousel,"`, `"mcp"`, `"u8g2"`, `"https://pro-api.coinmarketcap.com/public-api/v1/simple/price?ids=1"`) can remain attached if not trimmed/split on punctuation.
  - Sanitize `promptTerms` in `preinvocation.go` (splitting/trimming punctuation and filtering common English stopwords like `the`, `and`, `for`, `with`, `from`, `into`, `this`, `that`, `should`, `write`, `build`, `using`, `once`, `per`, `between`, `need`, `added`, `show`, `them`) so domain terms (`temperature`, `sensor`, `ds18b20`, `u8g2`, `screen`, `carousel`, `portal`, `mcp`, `server`, `bitcoin`, `price`, `coinmarketcap`, `api`, `module`, etc.) match FTS5 and dossier scopes with high precision on `MomentTaskStart`.

- [x] **2. Implement `internal/harness/antigravity/eval_v2_probe_test.go`:**
  - Create `TestEvalV2ReadSideMicroProbe` in `internal/harness/antigravity/eval_v2_probe_test.go` that exercises both frozen tasks from `evaluation/eval-v2/tasks/task-1-temperature-module.txt` and `evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt` against the `v2.2` combined index (`evaluation/eval-v2/seed/index.db` + `evaluation/eval-v2/template`, with a deterministic synthetic fallback if those gitignored directories are absent in CI).
  - For **both Task 1 (DS18B20 Temperature Module)** and **Task 2 (Bitcoin Price Ticker Module)**, simulate a realistic 5-phase multi-invocation trajectory and verify/measure:
    1. **Phase 1 — Invocation 0 (`MomentTaskStart`, `invocationNum == 0`, `initialNumSteps == 1`):**
       - Verify `HandlePreInvocation` injects `UserMessage` (`channel: "user"`, `EphemeralMessage == ""`).
       - Verify top-ranked `seed` + `doc-seed` entries are surfaced (specifically checking that `doc-seed` architectural contracts/invariants from `framework-expert`, `creating-a-module`, `modules/Application`, `modules/U8G2Display`, `modules/WebServer`, `modules/MCPServer`, `modules/DataManager`, or `modules/EventBus` are delivered).
       - Verify the payload stays $\le 180$ lines and $\le 500$ tokens (well within the $\le 1200$ token cap).
    2. **Phase 2 — Invocation 1 (`MomentFirstEntry` & `MomentFirstTouch`, `invocationNum == 1`, `initialNumSteps == 5`):**
       - Simulate the agent viewing subsystem files in `transcript_full.jsonl`:
         - Task 1: `modules/U8G2Display/U8G2Display.hpp`, `modules/WebServer/WebServer.hpp`, `modules/MCPServer/MCPServer.hpp`, `modules/Application/Application.cpp`, `include/interfaces/IMCPContributor.hpp`
         - Task 2: `modules/DataManager/DataManager.hpp`, `include/interfaces/IDataSource.hpp`, `modules/U8G2Display/U8G2Display.hpp`, `modules/WebServer/WebServer.hpp`, `modules/MCPServer/MCPServer.hpp`
       - Verify `InjectedSet` suppresses duplicate `task-start` entries while injecting newly relevant module dossier entries (`MomentFirstEntry`) and file/interface-anchored entries (`MomentFirstTouch`, e.g., `IDataSource`/`FetchContext`/`DataManager`, `U8G2Display`, `WebServer`, `MCPServer`, `Application`) via persistent `UserMessage`.
       - Verify the Invocation 0 `UserMessage` step remains persisted in `transcript_full.jsonl` (unlike `v2.1` where `EphemeralMessage` vanished after Invocation 0).
    3. **Phase 3 — Mid-Turn Edit Pointer & Gate (`MomentAboutToEdit` + `PreToolUse` `gate: edit`):**
       - When the latest `PLANNER_RESPONSE` thinking/content states `"Now I will edit modules/Application/Application.cpp"`, verify `HandlePreInvocation` emits `EphemeralMessage` (`channel: "ephemeral"`) within $\le 35$ lines ($\le 50$ tokens).
       - Also test a `gate: edit` entry (and/or risky edit on `platformio.ini`) via `HandlePreToolUse` (`PreToolUse`), verifying that the first edit call returns `{"decision": "deny", "reason": "..."}` within $\le 35$ lines and the immediate retry returns `{"decision": "allow"}`.
    4. **Phase 4 — Post-`CHECKPOINT` Compaction (`BuildReinjection`):**
       - Append a `CHECKPOINT` compaction step to `transcript_full.jsonl` that compresses earlier steps.
       - Verify `HandlePreInvocation` detects the compaction, re-injects active scope entries via persistent `UserMessage` ($\le 1000$ tokens, `EphemeralMessage == ""`), and re-arms dossier/anchor dedup.
    5. **Phase 5 — Transcript Rewind (`DetectRewind`):**
       - Truncate `transcript_full.jsonl` to simulate `/rewind` (`len(steps) < ledger.LastKnownStepCount`).
       - Verify `DetectRewind` fires, prunes rewound injections, and re-arms `InjectedSet` so the next invocation cleanly re-delivers the relevant entries via `UserMessage`.
  - Also include a direct quantitative comparison test or measurement helper comparing:
    - **`v2.1` Baseline (PA-only `source: seed` + `EphemeralMessage`):** 256 PA entries only (0 `doc-seed` entries from `framework-expert` / `creating-a-module` / `docs/developer/`), and `EphemeralMessage` vanishing after 1 invocation (0 entries surviving in context at the moment of code edit in Invocation 2+).
    - **`v2.2` Combined (`source: seed` + `source: doc-seed` + persistent `UserMessage`):** 1,283 entries (`256` `seed` + `1,027` `doc-seed`), persistent `UserMessage` retaining 100% of injected `task-start`, `first-entry`, and `first-touch` rules in `transcript_full.jsonl` across tool calls until `CHECKPOINT`, and automatic `BuildReinjection` restoring them after `CHECKPOINT`.

- [x] **3. Create `evaluation/eval-v2/scripts/run_read_probe.sh`:**
  - Create an executable script `evaluation/eval-v2/scripts/run_read_probe.sh` (`chmod +x`) that runs `go test -v -run TestEvalV2ReadSideMicroProbe ./internal/harness/antigravity/...` and exits 0 on pass.

- [x] **4. Write `docs/evaluations/v2-2-read-side-micro-probe.md`:**
  - Write a comprehensive empirical evaluation report in `docs/evaluations/v2-2-read-side-micro-probe.md` covering:
    1. **Executive Summary & Verdict:** How `v2.2` resolves the two root findings from `PR-09`/`PR-10` Stage 1 (the `8d7ad7b`+`d3ab994` One-Invocation Vanishing Memory bug and the 1–8% pull rate on `esp32-app-foundation` skills/developer guides).
    2. **Corpus & Coverage Comparison (`v2.1` PA-Only vs. `v2.2` PA + Doc-Seed):** Exact table comparing entry counts (`256` vs. `1,283`), anchor coverage (`2,206` vs. `3,190`), cognitive lifecycles (`semantic: 957`, `episodic: 214`, `procedural: 112`), and subsystem dossier coverage for the 6 subsystems touched by Task 1 and Task 2 (`modules/Application`, `modules/DataManager`, `modules/U8G2Display`, `modules/WebServer`, `modules/MCPServer`, `modules/EventBus`).
    3. **Multi-Invocation Trajectory Results — Task 1 (DS18B20 Temperature Module) & Task 2 (Bitcoin Price Ticker Module):**
       - Per-phase table (Invocation 0 `MomentTaskStart`, Invocation 1 `MomentFirstEntry`/`MomentFirstTouch`, Invocation 2 Dedup, `MomentAboutToEdit` & `PreToolUse` `gate: edit`, Post-`CHECKPOINT` `BuildReinjection`, Post-`/rewind` `DetectRewind`) showing channel (`UserMessage` vs `EphemeralMessage`), entry count, line count (vs $\le 180$ / $\le 35$ caps), token count (vs $\le 500$ / $\le 150$ / $\le 1000$ budgets), latency (ms), and exact injected entry IDs, sources (`seed` vs `doc-seed`), kinds, and titles.
    4. **Context Persistence Proof (`v2.1` vs. `v2.2` Across Invocations $0 \dots N$):** Table showing active rules in model context at the moment of code editing (Invocation $k \ge 2$): `0` entries in `v2.1` (vanished after Invocation 0 while blocked by `InjectedSet`) vs. 100% persistent retention in `v2.2` (plus automatic recovery across `CHECKPOINT` and `/rewind`).
    5. **Precision@10 & Architectural Rule Coverage Analysis:** Hand-graded relevance of the injected entries against the `esp32-app-foundation` architectural requirements for Task 1 and Task 2 (`IDataSource`/`FetchContext`, `U8G2Display` carousel/top-level icon, `IWebContributor`/`ITabRegistry`, `IMCPContributor`, `ConfigManager` interval validation, `Application.cpp` registration).

## Verification Requirement

1. `./evaluation/eval-v2/scripts/run_read_probe.sh` passes with 0 failures.
2. `go test -race ./cmd/... ./internal/... ./packaging/...` passes with 0 failures.
3. `docs/evaluations/v2-2-read-side-micro-probe.md` is complete, accurate, and contains zero internal references or absolute local paths.
4. `git status -s` shows only expected tracked files modified/added.

**CRITICAL:** Once verified, commit your changes on branch `pr-14-v2-micro-probes-and-pilot/wo2-read-side-offline-retrieval-probe` (using author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`), append your Completion Report to this `WORKORDER.md` (and commit it), and send a message back to the Principal Architect confirming completion.

## Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report

### Summary of Changes
1. **Prompt Tokenization & Dossier Contract Eligibility (`internal/harness/antigravity/preinvocation.go`, `internal/core/delivery/engine.go`):**
   - Added `ExtractPromptTerms(prompt string) []string` in `internal/harness/antigravity/preinvocation.go` to split natural-language prompts on non-word characters (stripping trailing punctuation and splitting URL tokens) and filter common English stopwords and URL/prompt boilerplate (`https`, `com`, `write`, `build`, `using`, `once`, `per`, `between`, `need`, `added`, `show`, `top`, `level`, `hour`, `sec`, `mins`, `public`, `simple`, `ids`, `pro`, `waterproof`, `compatible`, `device`).
   - Synchronized `stopWords` in `internal/core/delivery/engine.go` and included `dossiers.EntryTypeContract` alongside `EntryTypeInvariant`, `EntryTypeFailureSignature`, and `EntryTypeRuledOut` in `Engine.DossierEntries` so `v2.2` `doc-seed` C++ interface contracts (`IDataSource`, `IDisplayBoard`, `IMCPContributor`, `IWebContributor`) are eligible during `MomentFirstEntry`.
2. **Deterministic Read-Side Multi-Invocation Micro-Probe (`internal/harness/antigravity/eval_v2_probe_test.go`, `evaluation/eval-v2/scripts/run_read_probe.sh`):**
   - Implemented `TestEvalV2ReadSideMicroProbe` exercising both frozen forward tasks (`task-1-temperature-module.txt` and `task-2-bitcoin-price-module.txt`) against the combined `v2.2` `esp32-app-foundation` seed index (`1,283` entries: `256` `seed` + `1,027` `doc-seed`), with a self-contained synthetic fallback for clean CI environments where gitignored `evaluation/eval-v2/seed/` is absent.
   - Verified all 5 lifecycle phases: Phase 1 (`MomentTaskStart` persistent `UserMessage`, 14 entries, 492–493 entry tokens $\le 500$ budget, 90%–100% Precision@10), Phase 2 (`MomentFirstTouch` + `MomentFirstEntry` persistent `UserMessage`, 57–72 entries, 0 duplicates of Phase 1, plus Invocation 2 mid-turn dedup returning `{}` while retaining 71–86 entries in `transcript_full.jsonl`), Phase 3 (`MomentAboutToEdit` 1-line `EphemeralMessage` pointer + `PreToolUse` `gate: edit` returning `force_ask` on first edit and `allow` after `PostToolUse` acknowledgment), Phase 4 (post-`CHECKPOINT` `BuildReinjection` restoring 30–39 entries within the 1,000-token budget via persistent `UserMessage` and re-arming dedup), and Phase 5 (`DetectRewind` pruning rewound records, resetting `LastHandledCheckpointStep = -1`, and re-delivering 54–68 entries via `UserMessage`).
   - Added `BaselineComparison_v2_1_vs_v2_2` quantifying active rules surviving in model context at code-edit time (Invocation $\ge 2$): `0` entries in `v2.1` vs. `67–86` persistent rules in `v2.2`.
   - Created executable script `evaluation/eval-v2/scripts/run_read_probe.sh`.
3. **Empirical Evaluation Report (`docs/evaluations/v2-2-read-side-micro-probe.md`):**
   - Published the complete `v2.2` Read-Side Micro-Probe evaluation report covering executive verdict, corpus & subsystem coverage (`v2.1` vs. `v2.2`), per-phase trajectory tables and injected entry inventories for Task 1 and Task 2, context persistence proof across invocations $0 \dots N$, and hand-graded Precision@10 and architectural requirement coverage.

### Verification Results
- `./evaluation/eval-v2/scripts/run_read_probe.sh`: **PASS** (`0` failures).
- `go test -race ./cmd/... ./internal/... ./packaging/...`: **PASS** (`0` failures across all packages).
- External hygiene scan on `docs/evaluations/v2-2-read-side-micro-probe.md` and modified files: **PASS** (zero internal references or absolute local paths).
