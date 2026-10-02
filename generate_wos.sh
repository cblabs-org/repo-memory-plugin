#!/bin/bash
set -e

PROJ_DIR=".agents/skills/principal-architect-workspace/pr-04-entities-claims"
mkdir -p "$PROJ_DIR"

echo "- **2026-10-02**: Human approved the Work Order decomposition for PR-04." >> "$PROJ_DIR/PROJECT.md"

# WO1
mkdir -p "$PROJ_DIR/wo1-entity-types"
cat << 'WOEOF' > "$PROJ_DIR/wo1-entity-types/WORKORDER.md"
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
### Changes Made
### Decisions Made
### Dead Ends & Warnings
### Out-of-Scope Findings
### Blockers
WOEOF

# WO2
mkdir -p "$PROJ_DIR/wo2-entity-extractor"
cat << 'WOEOF' > "$PROJ_DIR/wo2-entity-extractor/WORKORDER.md"
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
WOEOF

# WO3
mkdir -p "$PROJ_DIR/wo3-claim-schema"
cat << 'WOEOF' > "$PROJ_DIR/wo3-claim-schema/WORKORDER.md"
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
WOEOF

# WO4
mkdir -p "$PROJ_DIR/wo4-claim-store"
cat << 'WOEOF' > "$PROJ_DIR/wo4-claim-store/WORKORDER.md"
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
WOEOF

# WO5
mkdir -p "$PROJ_DIR/wo5-sqlite-index"
cat << 'WOEOF' > "$PROJ_DIR/wo5-sqlite-index/WORKORDER.md"
---
name: pr-04-wo5
description: Handoff instructions for Work Order 5 - SQLite Index
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement a rebuildable SQLite index in `.memory/.cache/` mapping entities, claims, anchors, relations, and conditions.

### Strict Boundary Box
Only modify `internal/core/index/` and add the `reindex` command to `cmd/repo-memory/`.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] Create the SQLite schema for entities, claims, anchors, relations, and conditions.
- [ ] Implement WAL + busy_timeout for concurrency; ensure hooks open read-only.
- [ ] Implement short write transactions for the index.
- [ ] Implement a full rebuild mechanism (writes new versioned file, then switches pointer).
- [ ] Add `repo-memory reindex` CLI command.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints. **CRITICAL:** Before committing, the worker MUST ask the user if they are happy with the changes.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---
## Completion Report
<!-- Written by the worker at task completion -->
WOEOF

# WO6
mkdir -p "$PROJ_DIR/wo6-subsystem-config"
cat << 'WOEOF' > "$PROJ_DIR/wo6-subsystem-config/WORKORDER.md"
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
WOEOF

# WO7
mkdir -p "$PROJ_DIR/wo7-events-store"
cat << 'WOEOF' > "$PROJ_DIR/wo7-events-store/WORKORDER.md"
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
WOEOF

chmod +x generate_wos.sh
./generate_wos.sh
