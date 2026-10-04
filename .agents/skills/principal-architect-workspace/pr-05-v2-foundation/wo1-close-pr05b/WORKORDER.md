---
name: pr-05-v2-foundation-wo1
description: Handoff instructions for Work Order 1 — close PR-05b onto main
complexity: mechanical
---

# WO1: Close PR-05b onto `main`

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Merge the PR-05b branch's commits onto `main` (via this WO branch) so that `main` contains the executor fixes, all v2 documents, and the dogfood evaluation. Archive v1 claims. No production code changes.

## Strict Boundary Box
- **DO NOT** modify any Go source code in `internal/`, `cmd/`, or `pkg/`.
- **DO NOT** create new branches or modify branch structure beyond what's specified.
- **DO NOT** touch any files outside the checklist below.
- **DO NOT** run the pipeline, executor, or any LLM calls.
- If you discover issues outside this boundary, surface them in your Completion Report.

## Tooling Constraints
- Use `git` commands for all merge operations.
- Use `find` / `ls` / `mv` for the claims archive.
- Verify with `go build ./...` after the merge to ensure nothing is broken.

## Line-Item Instructions

- [ ] **Cherry-pick the PR-05b commits onto this branch.** The commits to bring in (from `pr-05b-dogfood-pilot/wo4-b5-evaluation`, in order):
  - `6fde866` — PR-05b project initialization
  - `4163dd5` — WO1-WO3 pipeline implementation (executor stdin fix)
  - `960d852` — Robust JSON extraction and 429 handling
  - `38d1061` — Dogfood evaluation report
  - `7540680` — Recall efficacy assessment
  - `eb6fbe4` — Comprehensive solution evaluation with corrected PA costs
  - `127a2e1` — MCP tools approach and Stop hook lifecycle correction
  - `75a3d58` — Review of dogfood evaluation (Appendix C)
  - `a4424e9` — v2 design proposal
  - `fc50953` — v2 design revision 3
  Use `git cherry-pick <commit>` for each, in order. If any conflicts arise (likely in `.agents/` planning files), resolve by taking the version from this branch for planning files, and taking the PR-05b version for all other files. The planning files were already committed on this branch.

- [ ] **Archive v1 claims.** If `.memory/claims/` exists on disk (it may be gitignored), move it to `.memory/archive/v1-claims/`. Create the archive directory if needed. If `.memory/claims/` doesn't exist in git, note that in the completion report.

- [ ] **Verify the build.** Run `go build ./...` and `go vet ./...` from the repo root. All must pass.

- [ ] **Verify the key files are present on this branch.** Confirm these files exist:
  - `docs/dogfood-evaluation-01.md`
  - `docs/design-v2-distillation-retrieval.md`
  - `docs/experiments/SPEC-CHANGES.md`
  - `internal/harness/antigravity/executor.go` (with the stdin fix)

## Verification Requirement
Run `go build ./...` and confirm it passes. List the commit log (`git log --oneline -15`) showing the merged history. **CRITICAL:** Before committing any resolution, verify the working tree is clean with `git status`.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report (written by the worker at task completion)

<!-- Written by the worker at task completion -->

### Status
Complete (Approved by Principal Architect)

### Changes Made
- Cherry-picked all 10 commits from `pr-05b-dogfood-pilot/wo4-b5-evaluation` in order onto `pr-05-v2-foundation/wo1-close-pr05b`.
- Resolved merge conflicts in `.agents/` planning files by preserving the branch's version (`--ours`).
- Archived v1 claims by moving `.memory/claims/` to `.memory/archive/v1-claims/` and committing the updates.
- Verified build and vet compliance across the entire codebase (`go build ./...` and `go vet ./...` pass cleanly).
- Confirmed presence of all key v2 foundation and evaluation files (`docs/dogfood-evaluation-01.md`, `docs/design-v2-distillation-retrieval.md`, `docs/experiments/SPEC-CHANGES.md`, and `internal/harness/antigravity/executor.go`).

### Decisions Made
- Followed cherry-pick order exactly as specified in the work order.
- Staged and committed archive movement of `.memory/claims/` to maintain clean git status.

### Dead Ends & Warnings
None.

### Out-of-Scope Findings
None.

### Blockers
None.

