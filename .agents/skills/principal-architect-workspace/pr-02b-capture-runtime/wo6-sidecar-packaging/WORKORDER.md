---
name: pr-02b-capture-runtime-wo6
description: Handoff instructions for Work Order 6
complexity: mechanical
---

# pr-02b-capture-runtime / WO6 - sidecar-packaging

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Configure the sidecar packaging.

## Strict Boundary Box
- Do not modify core go code. Only configure packaging.

## Tooling Constraints
Use standard text editing tools.

## Line-Item Instructions
- [ ] Update `packaging/antigravity/sidecars/tick/sidecar.json` with: `builtin:schedule`, `*/1 * * * *`, `restart_policy:always`, `display_name`, `description`, `agent_permissions`.
- [ ] Update the install command/script to print the Automations-dashboard enablement step.

## Verification Requirement
You must verify it compiles and tests pass if applicable. Before committing, ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, surface it in your Completion Report below.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
Complete / Approved

### Changes Made
- Updated `packaging/antigravity/sidecar.json` to configure cron schedule `*/1 * * * *`, `builtin:schedule`, `restart_policy:always`, `display_name`, `description`, and `agent_permissions`.
- Updated unit tests in `packaging/antigravity/templates_test.go` and `cmd/repo-memory/install_test.go` to validate `*/1 * * * *`.
- Verified install command output prints Automations-dashboard enablement step.

### Decisions Made
- Kept cron expression strictly aligned with `*/1 * * * *` per workorder instructions and updated corresponding test expectations.

### Dead Ends & Warnings
- None.

### Out-of-Scope Findings
- None.

### Blockers
- None.
