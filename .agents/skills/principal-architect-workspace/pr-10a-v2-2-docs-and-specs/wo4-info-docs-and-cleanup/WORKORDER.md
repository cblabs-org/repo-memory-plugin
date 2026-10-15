# Work Order 4: `wo4-info-docs-and-cleanup`

> **Project:** `pr-10a-v2-2-docs-and-specs`
> **Branch:** `pr-10a-v2-2-docs-and-specs/wo4-info-docs-and-cleanup`
> **Base:** `pr-10a-v2-2-docs-and-specs/wo3-spec-v1-0`
> **Complexity:** `standard`
> **Status:** ✅ Complete

---

## 1. Objective

Update `docs/antigravity-hooks-reference.md` with verified `agy 1.3.3` `userMessage` vs `ephemeralMessage` lifetime, `Stop` `NO_TOOL_CALL`, `invoke_subagent` hook ordering, and the Two-Plugin Packaging Split; create `README.md` at the repository root aligned with `SPEC.md v1.0` (`v2.2` Architecture); and remove the stale v1 `E02` root `GEMINI.md` (`@[memory map](.memory/views/MAP.md)`) and `.memory/views/` artifacts.

---

## 2. Scope & Deliverables

1. **`docs/antigravity-hooks-reference.md`:**
   - Document the Two-Plugin Packaging Split (`~/.gemini/config/plugins/repo-memory-daemon/` global scheduled sidecar + per-workspace `.agents/plugins.json` referencing the nested `workspace-plugin/repo-memory` via a resolved **absolute path**, since `~` and `$HOME` are not expanded).
   - Document verified `agy 1.3.3` `userMessage` vs `ephemeralMessage` vs `systemMessage` behavior: why `userMessage` is the canonical persistent session-scoped plain-string channel (`USER_INPUT` with `source: SYSTEM_SDK`), why combining `ephemeralMessage` with `InjectedSet` deduplication causes 1-invocation vanishing memory, and how `DetectCompaction` and `DetectRewind` interact with persistent `userMessage` injections.
   - Document `invoke_subagent` hook ordering (`PreToolUse` in parent $\to$ `PreInvocation` in child with `initialNumSteps == 0` before message delivery $\to$ `PostToolUse` in parent).
   - Update Practical Patterns to show `UserMessage` for persistent dossier delivery (`DetectCompaction` + `DetectRewind`), `EphemeralMessage` for 1-step alerts, and `Stop` `NO_TOOL_CALL` milestone-gated continuation.
2. **`README.md`:**
   - Create `README.md` at the repository root aligned with `SPEC.md v1.0` and `v2.2` Architecture (Overview, Core Principles, Architecture Overview, Two-Stage 5-Minute Warm-Cache Distillation, Op Log & 3 Cognitive Lifecycles, Push-First Delivery & MCP Pull Tools, Two-Plugin Packaging & CLI Reference, Documentation Index).
3. **Retire Stale v1 Artifacts (`VD19`):**
   - Remove the stale v1 `E02` root `GEMINI.md` (`@[memory map](.memory/views/MAP.md)`) and `.memory/views/` (`DECISIONS.md`, `MAP.md`, `SIGNATURES.md`, `index/`).

---

## 3. Acceptance Criteria

- [x] `docs/antigravity-hooks-reference.md` accurately documents `userMessage` vs `ephemeralMessage` lifetime, the `ephemeralMessage` + `InjectedSet` anti-pattern, `invoke_subagent` hook ordering, and the Two-Plugin Packaging Split.
- [x] `README.md` is created at the repository root and matches `SPEC.md v1.0` (`v2.2` Architecture).
- [x] Root `GEMINI.md` (`@[memory map](.memory/views/MAP.md)`) and `.memory/views/` are deleted via `git rm`.
- [x] Zero internal or non-public references exist in any modified or created file.

---

## 4. Completion Report

### Summary of Changes
- Updated `docs/antigravity-hooks-reference.md` with the Two-Plugin Packaging Split (`sidecar.json` global vs `.agents/plugins.json` absolute path workspace plugin), verified `agy 1.3.3` `userMessage` vs `ephemeralMessage` persistence semantics and the `InjectedSet` anti-pattern warning, `invoke_subagent` lifecycle hook ordering (`X8`), and updated Go code patterns (`UserMessage` persistent dossier injection with `DetectCompaction` & `DetectRewind`, 1-step `EphemeralMessage` nudges, and milestone-gated `Stop` `NO_TOOL_CALL` continuation).
- Created `README.md` at the repository root documenting the `v2.2` (`SPEC.md v1.0`) architecture, CLI commands, storage layout, and documentation links.
- Removed stale v1 `GEMINI.md` and `.memory/views/` (`DECISIONS.md`, `MAP.md`, `SIGNATURES.md`, `index/`).
