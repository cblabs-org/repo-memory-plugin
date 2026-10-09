---
name: pr-03-episode-search-wo5
description: Handoff instructions for Work Order 5
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Implement the `repo-memory eval <benchmark>` CLI command to serve as the evaluation runner and result store for benchmarks B1-B5 (as described in SPEC §10.3). Establish the dataset formats and implement the logic to run the B2 baseline (question-answering) against the episode search (FTS) built in WO1/WO2.

### Strict Boundary Box
Only modify evaluation runner code, CLI commands, and test datasets formats in the `eval` package/command. Do not touch production data stores, core episode FTS indexing, or MCP tool logic.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] Create the `eval` package/command (e.g., `cmd/repo-memory/cmd_eval.go`) and the underlying evaluation runner logic.
- [ ] Define the dataset formats for B1-B5 (JSON or YAML files representing the benchmark tasks/questions).
- [ ] Implement a result store mechanism (e.g., SQLite or JSONL in `.memory/eval/` or similar) to record the outcomes of evaluation runs.
- [ ] Implement the specific runner logic for the **B2 baseline**: given the B2 dataset of questions, query the episode search FTS index, and record the top retrieved results.
- [ ] Write/update unit tests to verify the eval runner and result storage behavior.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints (`go build ./cmd/repo-memory` and `go test ./...`). **CRITICAL:** Commit your changes to the branch after verifying tests pass, as requested by the PA SYSTEM_MESSAGE.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `internal/eval/dataset.go` — defined dataset formats for benchmarks (B1-B5).
- `internal/eval/store.go` — implemented a JSONL result store mechanism for eval outcomes.
- `internal/eval/runner.go` — implemented runner logic for B2 benchmark using the episode search FTS index.
- `cmd/repo-memory/cmd_eval.go` — created the eval CLI command.
- `cmd/repo-memory/main.go` — registered the eval command with cobra.
- `internal/eval/eval_test.go` — added unit tests for dataset loading, result storage, and B2 runner execution.

### Decisions Made
- Used JSONL for result storage because it easily supports appending individual runs and meshes well with `encoding/json`.
- Used `-f` to `git add` the CLI command `cmd/repo-memory/cmd_eval.go` because `.gitignore` globally ignores anything matching `repo-memory` unless specified otherwise.

### Dead Ends & Warnings
- When adding packages to index and runner, `index.EpisodeIndex` was incorrect; the correct type was `index.Index` and `InsertEpisode`.
- `git add` behavior: `.gitignore` has an entry for `repo-memory` which causes git to ignore the `cmd/repo-memory` directory entirely. Use `git add -f` to stage files inside this directory.

### Out-of-Scope Findings
- (none)

### Blockers
(none)
