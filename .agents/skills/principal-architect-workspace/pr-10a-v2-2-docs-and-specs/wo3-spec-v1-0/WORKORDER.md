---
name: pr-10a-v2-2-docs-and-specs-wo3
description: Handoff instructions for Work Order 3 (Upgrade SPEC.md to v1.0 and mark SPEC-CHANGES.md applied)
complexity: architect
---

# Work Order 3: Upgrade `SPEC.md` from `Draft v0.10` to `v1.0`

**Parent Project:** [PR-10a: v2.2 Specifications, Design & Reference Docs](../PROJECT.md)

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Upgrade `SPEC.md` from `Draft v0.10` (`2026-09-30`) to **`v1.0` (`2026-10-15`)**, retiring the v1 4-stage pipeline (`triage`, `extract`, `reconcile`, `verify`) and `.memory/claims/` / `.memory/views/` storage sections, unifying with [docs/design-v2-distillation-retrieval.md](../../../../../docs/design-v2-distillation-retrieval.md) Revision 4 (`v2.2`) and [docs/addendum-a-provenance-and-hardening.md](../../../../../docs/addendum-a-provenance-and-hardening.md) Revision 3, and applying all 16 items from [docs/experiments/SPEC-CHANGES.md](../../../../../docs/experiments/SPEC-CHANGES.md).

## Strict Boundary Box
- **In scope:** `SPEC.md`, `docs/experiments/SPEC-CHANGES.md`, and this `WORKORDER.md` completion report.
- **Out of scope:** Do NOT modify `docs/antigravity-hooks-reference.md` or `README.md` (WO4), or any Go source code (`PR-10b`).
- **External GitHub Hygiene:** Never include any non-public internal paths, internal issue numbers, or internal codenames. Never use absolute OS paths in markdown links.

## Tooling Constraints
Use standard file editing tools (`read_file`, `edit_file`, `write_file`) and `run_command` for `git status` / `git diff` verification.

## Line-Item Instructions
- [x] `SPEC.md` (Header, §1–§3) — Update Status to **`v1.0`** (`2026-10-15`), link [v2.2 Design (Rev 4)](../../../../../docs/design-v2-distillation-retrieval.md) and [Addendum A (Rev 3)](../../../../../docs/addendum-a-provenance-and-hardening.md); preserve `G1`/`P1` ("Zero agent or user cooperation required": both `PreInvocation` push delivery and milestone-gated `Stop` `NO_TOOL_CALL` end-of-turn capture + free `CHECKPOINT`/`brain/` harvest operate 100% automatically without waiting for user or agent tool calls, while `memory_note` is an optional mid-turn override); update `P4` to **Op-Log Event Sourcing & Cohesive Subsystem Dossiers** across 3 cognitive lifecycles (`semantic`, `procedural`, `episodic`); add **`P14` (Push-First Delivery, Pull for Depth)**; update §3 Terminology (Turn Note, Mark, Exposure List, Injection Ledger, Op, Op Log, Dossier, Entry, Atlas).
- [x] `SPEC.md` (§4 Architecture Overview) — Replace the retired v1 4-stage pipeline diagram and tables with the **v2.2 Two-Stage Warm-Cache Distillation, Op-Log/Dossier Projection, and Push-First Delivery Architecture**; document the two-plugin split (`repo-memory-daemon` global sidecar vs. `repo-memory` workspace bundle enabled via `.agents/plugins.json` with resolved absolute paths), `repo-memory enroll`/`unenroll` (with `init`/`leave` aliases), `repo-memory-distiller` custom agent (~38.4% fixed overhead reduction in X8), `/usage` quota check (X3), and `SPEC-CHANGES.md` items for §4.
- [x] `SPEC.md` (§5 Storage Architecture) — Replace `.memory/claims/` and `.memory/views/` with the **v2.2 Storage Architecture**: `.memory/ops/<yyyy-mm>/<ulid>.json` (`add`, `amend`, `supersede`, `retire`, `move`, `split`), `.memory/dossiers/<scope>.md` & `.history.md` (with optional `{section?}` resolution, ≤3k token cap) and `.memory/ATLAS.md`, the **Five-Tier State Scope Hierarchy** (`temp:`, `session:`, `user:`, `branch:`, `repo:`), the 8 entry types grouped under **3 Formal Cognitive Lifecycles (`semantic`, `procedural`, `episodic`)** + `closes_when` conditions, and the local rebuildable SQLite index (`<data-dir>/repos/<repo-id>/index.db` via canonical `index.IndexPath`).
- [x] `SPEC.md` (§6 Capture, Distillation & Consolidation Pipeline) — Replace v1 Stages 2–5 with the v2.2 pipeline: Stage 0/1 Capture & Compaction (`Stop` spool + incremental copy, `CHECKPOINT` summaries, `brain/<id>/` artifacts, E12 append-only & step-gap rules); Stage 1 Distillation (100% automatic milestone-gated `Stop` `NO_TOOL_CALL` continuation + free `CHECKPOINT`/`brain/` harvest + optional `memory_note` mid-turn dedup + PA completion report parser `source: pa` + `repo-memory seed-docs` `source: doc-seed` + sidecar fallback `repo-memory-distiller`); Preparation Layer (A4 layered scrubber, instruction stripping, JSON data framing, Go validation) + Pre-Consolidation Corroboration Gate (`VD18`); Stage 2 Warm Batch Consolidation (`repo-memory-distiller` over multi-turn `stream-json`: P13 echo filter, routing, duplicate merge, anchor check, LLM decide, Go validation, op-log write, dossier render) + Async Doc Freshness ("Dreaming") + Five-Tier Publish.
- [x] `SPEC.md` (§7 Delivery Engine) — Specify the 5 push moments (`task-start`, `first-entry`, `first-touch`, `about-to-edit`, `error-match`) plus `PreToolUse` `gate-once` (`gate: edit` and risky actions), the persistent (`userMessage` on `agy 1.3.3` / `systemMessage` where supported) vs. transient (`ephemeralMessage`) channel contract (`VD13`), `PR-09b` scaled FTS5 ranking (`ftsScore * 100`, `DefaultMinRelevanceScore = 50`, `InjectedSet` per-session deduplication), `CHECKPOINT` compaction and `DetectRewind` recovery, and the 6 MCP pull tools (`memory_brief`, `memory_why`, `memory_history`, `memory_search`, `memory_note`, `memory_status`/`memory_stats`).
- [x] `SPEC.md` (§8–§18 & Revision History) — Align §8 (Validity, `closes_when`, `procedural` execution-outcome validation & A7 skill graduation into `.agents/skills/rm-<slug>/SKILL.md`), §10 (Measurement & Evaluation: X6-lite findings, 231-session PA retrieval study, Eval v2 2×2 forward-task design, A10 paired evaluation discipline), §11 (Security: A4 layered scrubber), §12 (Antigravity harness integration: split plugin layout, `userMessage`/`ephemeralMessage` contracts, resolved E1–E12 and X1–X9 tables), §14 (Backfill: `seed` and `seed-docs`), §15 (Build phases updated to v2.2), §16 (Resolved questions), §18 (Go CLI subcommands including `enroll`/`unenroll`, `seed`, `seed-docs`, `reindex`, `usage`, `snapshot`, and v2 package layout `ops/`, `dossiers/`, `seed/`, `delivery/`), and add Revision `1.0` (`2026-10-15`) and `0.10` to the Revision history table.
- [x] `docs/experiments/SPEC-CHANGES.md` — Update Status from `Pending Review / Awaiting User Approval` to **`Applied in SPEC.md v1.0 (2026-10-15)`**.

## Verification Requirement
Verify `git diff SPEC.md docs/experiments/SPEC-CHANGES.md` is clean, complete, and contains zero internal references or absolute OS paths.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report

### Summary of Changes
- Upgraded `SPEC.md` from `Draft v0.10` (`2026-09-30`) to **`v1.0` (`2026-10-15`)**, unifying it with `docs/design-v2-distillation-retrieval.md` Revision 4 (`v2.2`), `docs/addendum-a-provenance-and-hardening.md` Revision 3, and all 16 empirical corrections from `docs/experiments/SPEC-CHANGES.md`.
- Updated **§1–§3 (Purpose, Goals, Design Principles, Terminology):** preserved `G1`/`P1` ("zero agent or user cooperation required") across both `PreInvocation` push delivery and automatic milestone-gated `Stop` (`NO_TOOL_CALL`, `executionNum == 0`) warm-cache capture + free `CHECKPOINT`/`brain/` harvest; updated `P4` to Op-Log Event Sourcing & Cohesive Subsystem Dossiers across 3 cognitive lifecycles (`semantic`, `procedural`, `episodic`); added `P14` ("Push-First Delivery, Pull for Depth"); updated §3 Terminology.
- Updated **§4 (Architecture Overview):** replaced the retired v1 4-stage pipeline with the v2.2 Two-Stage 5-Minute Warm-Cache Distillation, Op-Log/Dossier Projection, and Push-First Delivery architecture; documented the two-plugin split (`repo-memory-daemon` global sidecar vs. `repo-memory` workspace bundle enabled via `.agents/plugins.json`), `repo-memory-distiller` custom agent (38.4% fixed overhead reduction in X8), and `/usage` quota check (X3).
- Updated **§5 (Data Model & Storage Architecture):** replaced `.memory/claims/` and `.memory/views/` with `.memory/ops/<yyyy-mm>/<ulid>.json` (`add`, `amend`, `supersede`, `retire`, `move`, `split`), `.memory/dossiers/<scope>.md` & `.history.md` (with optional `{section?}` resolution) and `.memory/ATLAS.md`, the Five-Tier State Scope Hierarchy (`temp:`, `session:`, `user:`, `branch:`, `repo:`), the 8 entry types grouped under 3 cognitive lifecycles (`semantic`, `procedural`, `episodic`), `closes_when` conditions, and `<data-dir>/repos/<repo-id>/index.db` (`index.IndexPath`).
- Updated **§6 (Capture, Distillation & Consolidation Pipeline):** replaced v1 Stages 2–5 with Stage 0/1 Capture & Compaction, Stage 1 Distillation (automatic `Stop` warm-cache turn notes + free `CHECKPOINT`/`brain/` harvest + optional `memory_note` mid-turn dedup + `source: pa` + `repo-memory seed-docs` `source: doc-seed` + sidecar fallback), §6.4 Preparation Layer & Pre-Consolidation Corroboration Gate (`VD18`), and Stage 2 Warm Batch Consolidation (`repo-memory-distiller` over multi-turn `stream-json` + Async Doc Freshness "Dreaming" + Five-Tier Publish).
- Updated **§7 (Delivery Engine):** documented the 5 push moments + `PreToolUse` `gate-once`, the persistent (`userMessage` on `agy 1.3.3` / `systemMessage` where supported) vs. transient (`ephemeralMessage`) channel contract (`VD13`), `PR-09b` scaled FTS5 ranking (`ftsScore * 100`, `DefaultMinRelevanceScore = 50`, `InjectedSet` dedup), `DetectCompaction` and `DetectRewind` recovery, and the 6 MCP pull tools (`memory_brief`, `memory_why`, `memory_history`, `memory_search`, `memory_note`, `memory_status`).
- Updated **§8–§18 & Revision History:** aligned validity/re-checking (`procedural` execution-outcome validation & A7 skill graduation into `.agents/skills/rm-<slug>/SKILL.md`), measurement (`X6-lite`, 231-session PA study, Eval v2 2×2 forward-task design), security (A4 layered scrubber), Antigravity harness integration (resolved E1–E12 and X1–X9 tables, two-plugin layout), migration (`seed` and `seed-docs`), build phases, Go CLI subcommands (`enroll`/`unenroll`, `seed`, `seed-docs`, `reindex`, `usage`, `snapshot`), and added `0.10` and `1.0` to the Revision history table.
- Updated `docs/experiments/SPEC-CHANGES.md` status to **`Applied in SPEC.md v1.0 (2026-10-15)`**.

### Decisions Made
- Kept both `memory_status` and `memory_stats` documented as aliases in §7.7 / §18.1 so existing tests and documentation references resolve seamlessly.

### Dead Ends & Warnings
- None encountered in WO3.
