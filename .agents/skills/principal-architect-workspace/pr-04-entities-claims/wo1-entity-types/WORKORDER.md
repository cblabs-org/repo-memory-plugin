---
name: pr-04-wo1
description: Handoff instructions for Work Order 1 - Entity Types
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement the data model for entity types, entity relations, and the entity registry in the index for the `internal/core/entities` package.

### Strict Boundary Box
Only modify `internal/core/entities/`. Do not touch `internal/core/claims/`, `internal/core/index/` or any other subsystems yet.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] Create `internal/core/entities/entity.go` defining the entity types: `subsystem`, `file`, `symbol`, `config`, `env`, `topic`, `tool`, `error`, `external`.
- [ ] Define Entity relations in `internal/core/entities/relation.go`: `contains`, `depends_on`, `implements`, `configures`.
- [ ] Define the interface/schema for the Entity registry that will live in the index.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
Complete. Entities and Registry interfaces implemented successfully and passing tests.

### Changes Made
- Created `entity.go` with `Entity` struct and kinds.
- Created `relation.go` with `Relation` struct and relation kinds.
- Created `registry.go` with `Registry` interface.

### Decisions Made
- Implemented minimalistic structs matching the exact requirements from §5.2.
- Used `context.Context` in the `Registry` interface for standard cancellation and timeout handling.

### Dead Ends & Warnings
None.

### Out-of-Scope Findings
`generate_wos.sh` contained an infinite loop (it called itself recursively at the end) which was aborted and bypassed to generate this WO.

### Blockers
None.
