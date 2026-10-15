---
name: pr-10a-v2-2-docs-and-specs-wo5
description: Handoff instructions for Work Order 5 (Cross-document verification, external hygiene scrub, and PR-10a handover)
complexity: mechanical
---

# Work Order 5: Verification, External Hygiene Scrub & Project Handover

**Parent Project:** [PR-10a: v2.2 Specifications, Design & Reference Docs](../PROJECT.md)

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Cross-check all internal markdown links, section references, and external GitHub hygiene across `SPEC.md`, `README.md`, `docs/`, and `.agents/skills/principal-architect-workspace/`; scrub pre-existing local machine paths in historical experiment/evaluation docs; verify the Go test suite; and finalize the `PR-10a` Session Log and `MEMORY.md` state.

## Strict Boundary Box
- **In scope:** Hygiene and link fixes across `docs/` and `.agents/skills/principal-architect-workspace/`, `PROJECT.md`, `PROGRAM.md`, `PROGRAM-PLAN.md`, `MEMORY.md`, and this `WORKORDER.md`.
- **Out of scope:** Do NOT modify any Go source code (`PR-10b`).

## Line-Item Instructions
- [x] Scrub pre-existing internal/local path strings in `docs/experiments/phase0-plugin-split.md`, `docs/evaluations/pa-memory-retrieval-effectiveness.md`, `docs/dogfood-evaluation-01.md`, `docs/evaluations/run1-invalid/x6-lite-run1-postmortem.md`, `docs/experiments/E02-include-resolution.md` through `E05-hook-latency.md`, and `docs/experiments/x6-lite/*.md`.
- [x] Verify relative markdown links across `SPEC.md`, `README.md`, `docs/design-v2-distillation-retrieval.md`, `docs/addendum-a-provenance-and-hardening.md`, `docs/antigravity-hooks-reference.md`, `PROGRAM.md`, `PROGRAM-PLAN.md`, and `pr-10a-v2-2-docs-and-specs/`.
- [x] Run `go test ./cmd/... ./internal/...` to confirm all Go packages pass.
- [x] Update `PROJECT.md` (`5/5 WOs` complete + Project Wrap-Up in Session Log), `PROGRAM.md`, `PROGRAM-PLAN.md`, and `MEMORY.md` (move `pr-10a-v2-2-docs-and-specs` to Recently Completed).

---

## Completion Report

### Summary of Changes
- Scrubbed pre-existing local machine path strings (`<home-dir>`, `<repo-root>`) and internal binary disassembly references across `docs/experiments/phase0-plugin-split.md`, `docs/evaluations/pa-memory-retrieval-effectiveness.md`, `docs/dogfood-evaluation-01.md`, `docs/evaluations/run1-invalid/x6-lite-run1-postmortem.md`, `docs/experiments/E02`–`E05`, and `docs/experiments/x6-lite/*.md`.
- Verified relative markdown links across `README.md`, `SPEC.md`, `docs/design-v2-distillation-retrieval.md`, `docs/addendum-a-provenance-and-hardening.md`, `docs/antigravity-hooks-reference.md`, `PROGRAM.md`, `PROGRAM-PLAN.md`, and `pr-10a-v2-2-docs-and-specs`.
- Verified all Go unit tests pass (`go test ./cmd/... ./internal/...` — 25 packages green).
- Finalized `PROJECT.md` (`5/5 WOs` complete), `PROGRAM.md`, `PROGRAM-PLAN.md`, and `MEMORY.md`.

### Out-of-Scope Issues Surfaced
- `TestStopHook_CompletePipeline` in `internal/harness/antigravity/hooks_test.go` invokes `git commit` inside `t.TempDir()` without setting repo-local `user.name` / `user.email` in the temporary repository, relying on global git config or `GIT_AUTHOR_*` / `GIT_COMMITTER_*` environment variables. Candidate minor test-isolation cleanup for `PR-10b`.
