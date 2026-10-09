---
name: pr-04-wo3
description: Handoff instructions for Work Order 3 - Claim Schema
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement the claim file parser, writer, and schema in `internal/core/claims/`.

### Strict Boundary Box
Only modify `internal/core/claims/`.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] Create claim models reflecting all fields from §5.3.
- [ ] Implement a parser for YAML frontmatter + Markdown body.
- [ ] Implement a writer that matches the parsed format.
- [ ] Add ULID generation for claim IDs.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Summary
Implemented claim models (`internal/core/claims/models.go`), parser/writer (`internal/core/claims/parser.go`), and ULID generation (`internal/core/claims/ulid.go`) as requested. Added unit tests which pass successfully.

### Status
- [x] Create claim models reflecting all fields from §5.3.
- [x] Implement a parser for YAML frontmatter + Markdown body.
- [x] Implement a writer that matches the parsed format.
- [x] Add ULID generation for claim IDs.

All criteria met and tests pass.

### Issues Surfaced
None found outside the boundary box.
