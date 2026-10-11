---
name: pr-05a-pipeline-code-wo5
description: Handoff instructions for Work Order 5 (Verify)
complexity: standard
---

# WORKORDER: wo5-verify

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement Stage 5 of the knowledge pipeline: Verify (`internal/core/pipeline/verify/`). Perform anchor resolution, compute fingerprints, perform behavioral checks using a small model on code snippets, and implement branch scoping, including merge promotion and abandoned-branch promotion of `ruled-out` claims.

## Strict Boundary Box
- Do not implement any other pipeline stages (extract, reconcile, publish, etc.).
- Do not modify git history or branch structures.
- You may only modify `internal/core/pipeline/verify/`, `prompts/verify/`, and related test files.
- Do not import `internal/harness/antigravity` or any concrete adapter into `internal/core`.

## Tooling Constraints
- Ensure your changes compile and tests pass (`go build ./cmd/repo-memory` and `go test ./...`).
- Use recorded executor cassettes for tests (e.g., `fake.NewFakeHousekeeper`).

## Line-Item Instructions
- [ ] Implement `internal/core/pipeline/verify/verifier.go` to handle Stage 5 of the pipeline.
- [ ] Implement anchor resolution and fingerprinting for the extracted claims.
- [ ] Implement behavioral checks utilizing a small model (via the Housekeeper executor) on code snippets. Create `prompts/verify/prompt.md` to guide the model.
- [ ] Implement branch scoping logic, including merge promotion for validated claims and abandoned-branch promotion for `ruled-out` claims as defined in the SPEC.
- [ ] Write unit tests verifying anchor resolution, behavioral checks, and branch scoping behavior.

## Verification Requirement
The worker must verify it compiles and runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ensure all tests pass. After changes are tested, commit them and append the Completion Report.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete | partial | blocked

### Changes Made
- `path/to/file` — what changed and why

### Decisions Made
<!-- Only non-obvious decisions. Omit this section entirely for mechanical WOs. -->
- Decision description and rationale

### Dead Ends & Warnings
<!-- Things that didn't work. Critical for preventing future agents from repeating mistakes. -->
- What was attempted, why it failed, and what to avoid

### Out-of-Scope Findings
<!-- Defects, bugs, or concerns outside the boundary box. The PA will triage these. -->
- Finding description and suggested action

### Blockers
<!-- Empty if status is 'complete'. If partial/blocked, explain what prevents completion. -->
(none)

### Status
complete

### Changes Made
- `internal/core/pipeline/verify/verifier.go` — Implemented the `Verifier` struct and `VerifyClaims` method to resolve anchors, compute fingerprints, verify behavior using the Housekeeper, and apply branch scoping rules.
- `internal/core/pipeline/verify/verifier_test.go` — Wrote unit tests using `fake.NewFakeHousekeeper()` and a localized Git repository to test branch promotion, behavioral check calls, and updates.
- `prompts/verify/prompt.md` — Created prompt to guide the Housekeeper model on determining claim behavioral consistency from code snippets.

### Decisions Made
- Used local shell `exec.Command` calls for `git branch --contains` and `git rev-parse` directly inside the Verifier since the `gitops` package was strictly out-of-scope for editing.

### Dead Ends & Warnings
- N/A

### Out-of-Scope Findings
- The git querying for merged vs abandoned branches had to be duplicated within `verifier.go` because we lacked suitable methods in `gitops/branch.go` and modifying `gitops` was restricted. The Principal Architect might want to consider adding these utility methods into `gitops` for future reuse.

### Blockers
(none)
