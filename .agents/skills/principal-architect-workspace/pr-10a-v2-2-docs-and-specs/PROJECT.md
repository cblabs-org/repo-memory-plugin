# PR-10a: v2.2 Specifications, Design & Reference Docs

> **Programme:** repo-memory
> **PR:** pr-10a-v2-2-docs-and-specs
> **Base branch:** `pr-10-eval-v2/wo3-index-build` (`ec96755`)
> **Status:** 🔄 Active (2/5 WOs complete)
> **Human approval:** Decomposition approved 2026-10-15 as part of the v2.2 Programme Transition Plan.

## Objective

Align all canonical specification, design, reference, and programme documents in `repo-memory-plugin` with the approved `v2.2` (Revision 4) architecture and empirical findings from `PR-09` (X6-lite + 231-transcript study) and `PR-10` Stage 1 (77-session doc/skill audit) before mutating Go code in `PR-10b`.

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | [wo1-design-v2-rev4](wo1-design-v2-rev4/WORKORDER.md) | Upgrade `docs/design-v2-distillation-retrieval.md` from Revision 3 to **Revision 4 (`v2.2`)**: empirical grounding from `PR-09` & `PR-10` Stage 1; 3 cognitive lifecycles (`semantic`, `procedural`, `episodic`) & `{section?}` optional section rendering; Two-Stage 5-minute warm-cache distillation (100% automatic milestone-gated `Stop` `NO_TOOL_CALL` continuation primary + free `CHECKPOINT`/`brain/` harvest + optional `memory_note` mid-turn dedup + multi-turn `stream-json` consolidation); 5-tier state scope hierarchy & Pre-Consolidation Corroboration Gate; Doc-Seeded Dossiers (`repo-memory seed-docs`) & Async Doc Freshness ("Dreaming"); persistent `userMessage` vs transient `ephemeralMessage` delivery contract & `DetectRewind` wiring. | architect | ✅ Complete |
| 2 | [wo2-addendum-a-rev3](wo2-addendum-a-rev3/WORKORDER.md) | Upgrade `docs/addendum-a-provenance-and-hardening.md` from Revision 2 to **Revision 3**: A7 `procedural` execution-outcome validation (`PostToolUse` exit-code tracking, `success_count`/`failure_count`, deterministic `.agents/skills/rm-<slug>/SKILL.md` graduation); A8 managed instruction block refinement (preserving automatic `Stop` end-of-turn capture while noting optional `memory_note` override); A9 `DocSeedSource` (`source: doc-seed` + git-diff freshness). | standard | ✅ Complete |
| 3 | [wo3-spec-v1-0](wo3-spec-v1-0/WORKORDER.md) | Upgrade `SPEC.md` from `Draft v0.10` to **`v1.0`**, retiring v1 4-stage pipeline & `.memory/claims/` sections, incorporating all items from `docs/experiments/SPEC-CHANGES.md` and `design-v2-distillation-retrieval.md` Rev 4 while preserving `G1`/`P1` zero-cooperation capture and delivery. | architect | ☐ Pending |
| 4 | [wo4-info-docs-and-cleanup](wo4-info-docs-and-cleanup/WORKORDER.md) | Update `docs/antigravity-hooks-reference.md` with verified `agy 1.3.3` `userMessage` vs `ephemeralMessage` lifetime, `Stop` `NO_TOOL_CALL`, `invoke_subagent` hook ordering, and plugin split; update `README.md` to match `SPEC.md v1.0`; remove the stale v1 `E02` root `GEMINI.md` (`@[memory map](.memory/views/MAP.md)`) and `.memory/views/` artifacts. | standard | ☐ Pending |
| 5 | [wo5-verification-and-handover](wo5-verification-and-handover/WORKORDER.md) | Cross-check all internal markdown links, section references, and external GitHub hygiene across `SPEC.md`, `docs/`, and `.agents/skills/principal-architect-workspace/`; finalize `PR-10a` Session Log. | mechanical | ☐ Pending |

## Dependencies

- **Base:** `pr-10-eval-v2/wo3-index-build` at `ec96755`
- **Preconditions:** Approved `v2.2` Programme Plan (`PROGRAM.md` and `PROGRAM-PLAN.md`)
- **Downstream:** `PR-10b-delivery-and-storage-fixes` and `PR-11-v2-doc-seeding-and-lifecycles` implement the contracts codified here

## Exit Criteria

- `docs/design-v2-distillation-retrieval.md` is at Revision 4 (`v2.2`), `docs/addendum-a-provenance-and-hardening.md` is at Revision 3, and `SPEC.md` is at `v1.0` with zero contradictions between them.
- `docs/experiments/SPEC-CHANGES.md` items are marked applied in `SPEC.md v1.0`.
- `docs/antigravity-hooks-reference.md` and `README.md` reflect `v2.2` (`SPEC.md v1.0`).
- Stale v1 `GEMINI.md` (`@[memory map](.memory/views/MAP.md)`) and `.memory/views/` are removed.
- Zero internal/non-public references in any repository file.

## Session Log

### 2026-10-15 — Project Creation & Human Approval Recorded
- User approved the `v2.2` Programme Transition Plan and the 5-WO decomposition for `pr-10a-v2-2-docs-and-specs` (`2026-10-15`).
- Confirmed key design clarification (`VD14`): Stage 1 end-of-turn capture is 100% automatic via the milestone-gated `Stop` hook (`stopReason == "NO_TOOL_CALL"`, `executionNum == 0`, returning `{"decision": "continue"}`) + free `CHECKPOINT`/`brain/` harvest, never relying on the user or agent remembering to call `memory_note` or `/remember` (`memory_note` is strictly an optional mid-turn override that deduplicates against `Stop`).
- Confirmed `VD19`: Remove the stale 1-line v1 `E02` root `GEMINI.md` (`@[memory map](.memory/views/MAP.md)`) and `.memory/views/` rather than hand-populating `GEMINI.md`.

### 2026-10-15 — WO1 Complete (`wo1-design-v2-rev4`)
- Upgraded `docs/design-v2-distillation-retrieval.md` from Revision 3 to **Revision 4 (`v2.2`)** (`VD13`–`VD19`).
- Added §2.4 empirical grounding (`X6-lite` B1 replay, 231-session PA retrieval study, 77-session `esp32` doc audit, `8d7ad7b` + `d3ab994` vanishing memory regression), 3 cognitive lifecycles (`semantic`, `procedural`, `episodic`), Two-Stage 5-minute warm-cache distillation (`Stop` continue primary + `CHECKPOINT`/`brain/` harvest + `memory_note` mid-turn dedup + multi-turn `stream-json` consolidation), Pre-Consolidation Corroboration Gate, 5-tier state scope hierarchy, `{section?}` optional rendering, `repo-memory seed-docs` + async "dreaming" freshness, persistent `userMessage` vs 1-step `ephemeralMessage` delivery contract, and `DetectRewind` wiring.

### 2026-10-15 — WO2 Complete (`wo2-addendum-a-rev3`)
- Upgraded `docs/addendum-a-provenance-and-hardening.md` from Revision 2 to **Revision 3** (`2026-10-15`).
- Updated §8 (`A8: Managed instruction blocks` `v2` block + `VD19` machine-managed `GEMINI.md`), §10 (`A7: Procedures become skills` with `PostToolUse` exit-code `success_count`/`failure_count` tracking, build-manifest `stale_suspect` flagging, and graduation into `.agents/skills/rm-<slug>/SKILL.md`), §11 (`A9: DocSeedSource` Priority 0 built-in source + async "Dreaming" freshness), §16 (`index.db` via `index.IndexPath`, `doc_seeds`), and §17 (`PR-10a`/`PR-10b`/`PR-11` phasing).
