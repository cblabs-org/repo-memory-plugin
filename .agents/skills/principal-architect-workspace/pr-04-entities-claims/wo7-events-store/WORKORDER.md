---
name: pr-04-wo7
description: Handoff instructions for Work Order 7 - Events Store
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement the events store per §5.4.

### Strict Boundary Box
Only modify `internal/core/events/`.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] Implement schema and store for `.memory/events/reviews/*.jsonl` and `.memory/events/answers/*.jsonl`.
- [ ] Ensure events are keyed by claim ID and content hash.
- [ ] Add writers and readers that PR-06 will need.
- [ ] Document the replay order for PR-10.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---
## Completion Report
<!-- Written by the worker at task completion -->
