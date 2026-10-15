---
name: pr-10a-v2-2-docs-and-specs-wo1
description: Handoff instructions for Work Order 1 (Upgrade design-v2-distillation-retrieval.md to Revision 4 / v2.2)
complexity: architect
---

# Work Order 1: Upgrade `docs/design-v2-distillation-retrieval.md` to Revision 4 (`v2.2`)

**Parent Project:** [PR-10a: v2.2 Specifications, Design & Reference Docs](../PROJECT.md)

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Upgrade `docs/design-v2-distillation-retrieval.md` from Revision 3 to **Revision 4 (`v2.2`)**, incorporating empirical findings from `PR-06` through `PR-10` Stage 1 and the six `v2.2` architectural pillars (`VD13`–`VD19`).

## Strict Boundary Box
- **In scope:** `docs/design-v2-distillation-retrieval.md` and this `WORKORDER.md` completion report.
- **Out of scope:** Do NOT modify `docs/addendum-a-provenance-and-hardening.md` (WO2), `SPEC.md` (WO3), `docs/antigravity-hooks-reference.md` or `README.md` (WO4), or any Go source code (`PR-10b`).
- **External GitHub Hygiene:** Never include any non-public internal paths, internal issue numbers, or internal codenames. Never use absolute OS paths (`file:///...` or `/Users/...`) in markdown links—always use relative links.

## Tooling Constraints
Use standard file editing tools (`read_file`, `edit_file`, `write_file`) and `run_command` for `git status` / `git diff` verification.

## Line-Item Instructions
- [x] `docs/design-v2-distillation-retrieval.md` — Bump header to **Revision 4 (`v2.2`)** (`2026-10-15`) and add Revision 4 change summary table & bullet list.
- [x] `docs/design-v2-distillation-retrieval.md` (§2) — Add **§2.4 Empirical findings from PR-06 through PR-10 Stage 1** covering: (a) X6-lite STOP verdict on 150-line spec-style work orders (84% no-memory baseline, 0 headroom), (b) 231-session PA retrieval study + X6-lite 0 MCP pull calls proving unprompted pull memory is unused (`P14`), (c) `PR-10` Stage 1 77-session audit showing curated repo skills/guides are read only 1–8% of the time, (d) `PR-06`/`PR-06b` spike validations (plugin split, `repo-memory-distiller` frontmatter requirements, `8d7ad7b` + `d3ab994` `EphemeralMessage` vs `UserMessage` persistence interaction).
- [x] `docs/design-v2-distillation-retrieval.md` (§4 & §5.2) — Group the 8 entry types (`invariant`, `contract`/`convention`, `gotcha`/`external-behaviour`, `open-issue` | `recipe`/`procedure`, `failure-signature` | `decision`, `ruled-out`) under **3 Formal Cognitive Memory Lifecycles (`kind: semantic | procedural | episodic`)** with distinct invalidation, delivery, and skill graduation rules (`VD17`).
- [x] `docs/design-v2-distillation-retrieval.md` (§5.1, §5.3, §5.4) — Specify the **Two-Stage 5-Minute Warm-Cache Distillation Pipeline (`VD14`, `VD15`)**: Stage 1 Primary is 100% automatic end-of-turn `Stop` (`terminationReason == "NO_TOOL_CALL"`, `fullyIdle == true`, `executionNum == 0`) continuation on milestone turns + automatic zero-token `CHECKPOINT` and `brain/<id>/` harvest (never waiting for user or agent cooperation); `memory_note` MCP tool is an optional mid-turn override that deduplicates against `Stop`; Stage 2 runs a single multi-turn `agy -p --agent repo-memory-distiller --input-format stream-json --output-format stream-json` process per tick to share the warm prefix cache across dossiers.
- [x] `docs/design-v2-distillation-retrieval.md` (§6 & §7.1) — Formalize the **Five-Tier State Scope Hierarchy** (`temp:`, `session:`, `user:` in gitignored `<data-dir>/repos/<repo-id>/user-dossiers/`, `branch:` in `.memory/local/`, `repo:` in `.memory/ops/` & `.memory/dossiers/`) and the **Pre-Consolidation Corroboration Gate** (`VD18`) requiring a passing `PostToolUse` test/build step, verified `HEAD` anchor, or `USER_INPUT` confirmation before promoting `verified: null` observations to active `do:` imperatives.
- [x] `docs/design-v2-distillation-retrieval.md` (§7.3 & §7.6) — Specify **Optional Section Resolution (`{section?}`)** so empty headings are omitted, and add **§7.6 Doc-Seeded Dossiers (`repo-memory seed-docs` / `source: doc-seed`) & Async Doc Freshness ("Dreaming")** (`VD16`).
- [x] `docs/design-v2-distillation-retrieval.md` (§8.2–§8.4) — Update the **Delivery Channel Contract (`VD13`)** (`userMessage` plain string persistent until `CHECKPOINT` for `task-start`, `first-entry`, `first-touch`, and `re-injection` whenever `systemMessage` is rejected; `ephemeralMessage` strictly for 1-step hints), document `PR-09b` bm25 ranking (`ftsScore * 100`, `DefaultMinRelevanceScore = 50`, `InjectedSet` dedup), and require `DetectRewind` wiring in `HandlePreInvocation`.

## Verification Requirement
Verify `git diff docs/design-v2-distillation-retrieval.md` is clean, complete, and contains zero internal references or absolute OS paths.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report

### Summary of Changes
- Upgraded `docs/design-v2-distillation-retrieval.md` from Revision 3 to **Revision 4 (`v2.2`)** (`2026-10-15`).
- Added **§2.4 Empirical findings from PR-06 through PR-10 Stage 1** documenting: (1) X6-lite B1 replay findings (84.0% no-memory baseline pass rate on spec-style tasks, 31.6% step reduction on `T2`, 0% task-start precision on `T1`, and 0% dossier coverage on `T5`), (2) the 231-transcript PA retrieval study showing 94–99% of passive pull memory goes unread (`1.3%` `MEMORY.md` reads, `0` MCP pull calls), (3) the 77-session `esp32` audit showing 1.3–7.8% read rates for static repo documentation, and (4) the `8d7ad7b` + `d3ab994` "One-Invocation Vanishing Memory" regression (`ephemeralMessage` + `InjectedSet`).
- Updated **§5.2** to group all 8 entry types under **Three Formal Cognitive Memory Lifecycles (`kind: semantic | procedural | episodic`)** with explicit verification, decay/invalidation, and skill graduation rules (`VD17`).
- Updated **§5.1, §5.3, §5.4, and §6** for the **Two-Stage 5-Minute Warm-Cache Distillation Pipeline (`VD14`, `VD15`)**: Stage 1 Primary is 100% automatic end-of-turn `Stop` (`NO_TOOL_CALL`, `fullyIdle == true`, `executionNum == 0`) continuation reusing the warm 5-minute KV cache (~90% input discount) + free zero-token `CHECKPOINT` and `brain/<id>/` harvest; `memory_note` MCP tool is an optional mid-turn override that advances the mark and deduplicates against `Stop`; Stage 2 runs a single multi-turn `agy -p --agent repo-memory-distiller --input-format stream-json --output-format stream-json` process per tick; added the **Pre-Consolidation Corroboration Gate (`VD18`)** and **Five-Tier State Scope Hierarchy** (`temp:`, `session:`, `user:`, `branch:`, `repo:`).
- Updated **§7.3 and §7.6** with **Optional Section Resolution (`{section?}`)** and **Doc-Seeded Dossiers (`repo-memory seed-docs`) & Async Doc Freshness ("Dreaming")** (`VD16`).
- Updated **§8.2–§8.4** with the persistent `userMessage` (on `agy 1.3.3`) vs. 1-step `ephemeralMessage` delivery contract (`VD13`), `PR-09b` scaled FTS5 ranking (`ftsScore * 100`, `DefaultMinRelevanceScore = 50`, `InjectedSet` dedup), and `DetectRewind` wiring in `HandlePreInvocation`.

### Decisions Made
- Preserved `G1`/`P1` ("zero user or agent cooperation required") throughout §1, §4, and §5.1 by keeping the milestone-gated `Stop` continuation (`NO_TOOL_CALL`, `executionNum == 0`) as the primary automatic Stage 1 mechanism while framing `memory_note` strictly as an optional mid-turn override that advances the conversation mark.
- Retained `contract` as an alias for `convention`, `gotcha` as an alias for `external-behaviour`, and `recipe` as an alias for `procedure` so existing seeded ops and new `v2.2` entries remain schema-compatible.

### Dead Ends & Warnings
- None encountered in WO1.

### Out-of-Scope Issues Surfaced
- Pre-existing hygiene items noticed during read-only inspection of older docs (`docs/experiments/phase0-plugin-split.md` line 82 internal path and `/Users/mcneillm` references; `docs/evaluations/pa-memory-retrieval-effectiveness.md` `file:///Users/mcneillm/...` links) are queued for cleanup in WO5 (`wo5-verification-and-handover`).
