---
name: pr-04-wo4
description: Handoff instructions for Work Order 4 - Claim Store
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement the claim store for CRUD operations with month-sharded writes.

### Strict Boundary Box
Only modify `internal/core/claims/store/` or related store logic.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] Implement CRUD operations for claims following `.memory/claims/<yyyy-mm>/` layout.
- [ ] Implement logic to perform month-sharded writes.
- [ ] Implement an in-memory or abstracted relation index for inverse lookups.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
Completed.

### Work Summary
- Implemented `Store` in `internal/core/claims/store/store.go` for CRUD operations.
- Implemented month-sharded writes by saving claims to `<baseDir>/<yyyy-mm>/<id>.md`.
- Implemented an in-memory inverse relation index with `GetInverseRelations` that dynamically updates on store mutations.
- Added test coverage in `internal/core/claims/store/store_test.go`.
- Code compiles and passes all tests successfully.

### Notes & Issues Surfaced
No boundary-spanning issues or bugs discovered during implementation.
