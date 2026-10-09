---
name: pr-03-episode-search-wo6
description: Handoff instructions for Work Order 6
complexity: standard
---

# WORKORDER Template

## Required Sections

### Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

### Objective
Create the initial evaluation datasets (B1-B5) and any necessary tooling to support them, specifically focusing on generating the data artifacts for B1, B2, B3, and B5 as described in SPEC §10.3.

### Strict Boundary Box
Only modify evaluation datasets (JSON/YAML) and auxiliary dataset generation scripts/tools within the `eval` package or a dedicated scripts directory. Do not alter core application logic, production data stores, or the FTS index.

### Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

### Line-Item Instructions
- [ ] **B5 (Precision):** Create a sampling tool or script that selects ≥ 20 random/diverse episodes from the store and generates a CSV/JSON "labelling sheet" template with columns for correct/wrong/not-durable and missed knowledge.
- [ ] **B2 (Question Answering):** Create the B2 dataset file (e.g. `eval/datasets/b2_questions.json`) containing a question bank of ≥ 50 questions derived from standard project contexts (you can stub some if necessary, but aim to provide realistic mock data or extract from available PA logs if possible).
- [ ] **B1 (Briefing Parity):** Create the B1 dataset configuration specifying a repository cut-off at commit `69d9c813` and outline the expected facts.
- [ ] **B3 (Task Replay):** Create the B3 dataset configuration containing 3-5 historical tasks, each specifying its starting commit.
- [ ] Verify the dataset formats can be loaded by the evaluation runner built in WO5.

### Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints (`go build ./cmd/repo-memory` and `go test ./...`). **CRITICAL:** Commit your changes to the branch after verifying tests pass, as requested by the PA SYSTEM_MESSAGE.

### Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete | partial | blocked

### Changes Made
- `path/to/file` — what changed and why

### Decisions Made
<!-- Only non-obvious decisions. Omit this section entirely for mechanical WOs. -->
- Decision description and rationale

### Dead Ends & Warnings
<!-- Things that didn't work. Critical for preventing future agents from repeating mistakes. -->
- What was attempted, why it failed, and what to avoid

### Out-of-Scope Findings
<!-- Defects, bugs, or concerns outside the boundary box. The PA will triage these. -->
- Finding description and suggested action

### Blockers
<!-- Empty if status is 'complete'. If partial/blocked, explain what prevents completion. -->
(none)


### Status
complete

### Changes Made
- `internal/eval/datasets/b1_briefing.json` — created B1 dataset specifying cut-off commit.
- `internal/eval/datasets/b2_questions.json` — created B2 dataset with 50 generated questions.
- `internal/eval/datasets/b3_replay.json` — created B3 dataset with 3 historical tasks and commits.
- `internal/eval/scripts/sample_b5.go` — created B5 sampling tool to generate CSV labeling sheet from store.

### Decisions Made
- Chose to write the datasets in JSON to comply with `eval.LoadDataset` format shown in WO5.
- Wrote `sample_b5.go` as a standalone script in `internal/eval/scripts` that directly hits the `episode.Store` and writes out a CSV file. Note: The `episode.Header` struct did not have a `Summary` field as originally expected, so I omitted it from the CSV output.

### Dead Ends & Warnings
(none)

### Out-of-Scope Findings
(none)

### Blockers
(none)
