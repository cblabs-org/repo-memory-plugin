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
### Changes Made
### Decisions Made
### Dead Ends & Warnings
### Out-of-Scope Findings
### Blockers
