---
name: pr-10a-v2-2-docs-and-specs-wo2
description: Handoff instructions for Work Order 2 (Upgrade addendum-a-provenance-and-hardening.md to Revision 3)
complexity: standard
---

# Work Order 2: Upgrade `docs/addendum-a-provenance-and-hardening.md` to Revision 3

**Parent Project:** [PR-10a: v2.2 Specifications, Design & Reference Docs](../PROJECT.md)

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Upgrade `docs/addendum-a-provenance-and-hardening.md` from Revision 2 to **Revision 3** (`2026-10-15`), aligning it with `SPEC.md v1.0` and `docs/design-v2-distillation-retrieval.md` Revision 4 (`v2.2`).

## Strict Boundary Box
- **In scope:** `docs/addendum-a-provenance-and-hardening.md` and this `WORKORDER.md` completion report.
- **Out of scope:** Do NOT modify `SPEC.md` (WO3), `docs/antigravity-hooks-reference.md` or `README.md` (WO4), or any Go source code (`PR-10b`).
- **External GitHub Hygiene:** Never include any non-public internal paths, internal issue numbers, or internal codenames. Never use absolute OS paths in markdown links.

## Tooling Constraints
Use standard file editing tools (`read_file`, `edit_file`, `write_file`) and `run_command` for `git status` / `git diff` verification.

## Line-Item Instructions
- [x] `docs/addendum-a-provenance-and-hardening.md` — Bump header to **Revision 3** (`2026-10-15`, applies to `SPEC v1.0` together with [v2 design](../../../../../docs/design-v2-distillation-retrieval.md) Revision 4) and add Revision 3 row in the revisions table and summary table.
- [x] `docs/addendum-a-provenance-and-hardening.md` (§8 `A8: Managed instruction blocks`) — Upgrade the delimited block to `<!-- repo-memory:begin v2 -->` including `@[memory atlas](.memory/ATLAS.md)`, guidance to trust `[repo-memory]` pushed blocks without echoing them verbatim, pull tool pointers (`memory_brief`, `memory_why`, `memory_history`, `memory_search`), and a note that `memory_note` is available for explicit mid-turn notes while automatic end-of-turn capture is handled by the `Stop` hook without requiring agent cooperation. Clarify (`VD19`) that root `GEMINI.md` is strictly machine-managed by `repo-memory enroll` / `unenroll` (`init`/`leave`).
- [x] `docs/addendum-a-provenance-and-hardening.md` (§10 `A7: Procedures become skills`) — Integrate `procedural` lifecycle execution-outcome validation: `PostToolUse` matches executed bash commands against active `recipe` / `procedure` commands, incrementing `success_count` on exit code `0` and `failure_count` on non-zero exit (and flagging `stale_suspect` when build manifests like `Makefile`, `CMakeLists.txt`, `platformio.ini`, `go.mod`, or `package.json` change). When a `recipe` / `procedure` reaches `success_count >= 2` across distinct sessions (or carries `render_as_skill: rm-<slug>`), the renderer deterministically writes `.agents/skills/rm-<slug>/SKILL.md` with `generated_by: repo-memory`.
- [x] `docs/addendum-a-provenance-and-hardening.md` (§11 `A9: External and repository knowledge sources`) — Add **`DocSeedSource` (`source: doc-seed`)** as Priority 0 built-in source ahead of external trackers: ingests curated in-repo markdown guides (`docs/developer/`, `.agents/skills/`, architecture docs) via `repo-memory seed-docs`, records `source_doc_path` and `source_doc_blob_sha` in entry metadata, and triggers async re-distillation ("Dreaming") in `tick` when `git diff` detects changes to the source markdown file.
- [x] `docs/addendum-a-provenance-and-hardening.md` (§16 & §17) — Update section cross-references (`index.db` via `index.IndexPath`, `doc_seeds` in `.memory/config.yaml`, `PR-10a`/`PR-10b`/`PR-11` phasing).

## Verification Requirement
Verify `git diff docs/addendum-a-provenance-and-hardening.md` is clean, complete, and contains zero internal references or absolute OS paths.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report

### Summary of Changes
- Upgraded `docs/addendum-a-provenance-and-hardening.md` from Revision 2 to **Revision 3** (`2026-10-15`), aligned with `SPEC v1.0` and `docs/design-v2-distillation-retrieval.md` Revision 4 (`v2.2`).
- Updated **§8 (`A8: Managed instruction blocks`)** to define the `<!-- repo-memory:begin v2 -->` block in `GEMINI.md` / `CLAUDE.md`, including `@[memory atlas](.memory/ATLAS.md)`, anti-echo instruction framing, MCP pull tool pointers (`memory_brief`, `memory_why`, `memory_history`, `memory_search`), and the `Stop`-primary + optional `memory_note` mid-turn capture rule, while enforcing `VD19` (machine-managed only via `enroll`/`unenroll`).
- Updated **§10 (`A7: Procedures become skills`)** with `procedural` lifecycle execution-outcome validation (`PostToolUse` exit-code tracking, `success_count` / `failure_count`, build-manifest `stale_suspect` flagging, and deterministic graduation at `success_count >= 2` across sessions into `.agents/skills/rm-<slug>/SKILL.md` with `generated_by: repo-memory`).
- Updated **§11 (`A9: External and repository knowledge sources`)** to add **`DocSeedSource` (`source: doc-seed`)** as the Priority 0 built-in repository source (`repo-memory seed-docs`, `source_doc_path`, `source_doc_blob_sha`, and async "Dreaming" re-distillation on `git diff`).
- Updated **§16 (`Changes to existing sections`)** and **§17 (`Phasing and validation`)** to reflect `index.db` (`index.IndexPath`), `doc_seeds`, and the `PR-10a` / `PR-10b` / `PR-11` schedule.

### Decisions Made
- Kept `AGENTS.md` untouched by default unless configured, while making `GEMINI.md` and `CLAUDE.md` strictly machine-managed via the `<!-- repo-memory:begin v2 -->` delimited block during `repo-memory enroll` / `unenroll` (`init`/`leave`).

### Dead Ends & Warnings
- None encountered in WO2.
