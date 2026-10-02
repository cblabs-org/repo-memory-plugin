---
name: pr-04-wo6
description: Handoff instructions for Work Order 6 - Subsystem Config
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement parser and logic for `.memory/config.yaml` subsystem configuration.

### Strict Boundary Box
Modify subsystem config parsing logic in `internal/core/config/` or `internal/core/entities/`.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] Parse `.memory/config.yaml` for subsystem definitions.
- [ ] Implement glob matching logic for file → subsystem assignment.
- [ ] Implement unassigned file detection logic.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---
## Completion Report
<!-- Written by the worker at task completion -->

### Implementation Details
- `RepoConfig.Subsystems` parsing already handled by standard YAML mapping.
- Added `AssignSubsystem(filePath)` using standard `filepath.Match` in `internal/core/config/subsystem.go`.
- Added `UnassignedFiles(files)` to detect unmatched files.
- Verified parsing and assignments with unit tests (`go test ./internal/core/config`).

### Out-of-Boundary Issues Surfaced
- Standard `filepath.Match` does not cross directory boundaries with `*`. A `**` standard is common for this type of path globbing but not natively supported without bringing in a third-party module like `github.com/bmatcuk/doublestar/v4`. For now, `filepath.Match` is used as standard library dependency.
