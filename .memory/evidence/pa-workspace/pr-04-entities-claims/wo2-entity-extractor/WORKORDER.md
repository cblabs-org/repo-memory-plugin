---
name: pr-04-wo2
description: Handoff instructions for Work Order 2 - Entity Extractor
complexity: architect
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement a No-LLM entity extractor in `internal/core/entities/extractor/`.

### Strict Boundary Box
Only modify `internal/core/entities/extractor/`. Do not touch other core pipelines.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] Implement a tree walk for files (`.gitignore`-aware).
- [ ] Parse `sdkconfig`, `Kconfig`, `platformio.ini`, and lockfiles for `config` entities.
- [ ] Implement brace-aware C/C++ symbol scanner (with `ctags` fallback) for `symbol` entities.
- [ ] Implement extractor rules from `config.yaml` for `topic`, `tool`, and `error` entities.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---
## Completion Report
<!-- Written by the worker at task completion -->


### Status
- **Result:** SUCCESS
- **Commit:** (Pending)

### Implementation Notes
- Created `internal/core/entities/extractor` package.
- Implemented `IgnoreMatcher` for `.gitignore` aware file walking.
- Handled config parsing for `sdkconfig`, `Kconfig`, `platformio.ini`, `go.mod` and `.lock` files.
- Implemented C/C++ symbol extraction utilizing `ctags` (if available), with a custom brace-aware fallback scanner.
- Added extraction of `topic`, `tool`, and `error` entities from `config.yaml`.
- Validated functionality with local testing suite and verified successful `go build`.

### Issues Surfaced
None outside the boundary box.
