---
complexity: standard
role: Data Engineer
objective: Snapshot candidates and build time-cut dossiers and PA documents for the 8 selected work orders per the X6 protocol.
boundary_box: Evidence store snapshotting, time-cut dossier generation. Do not execute the evaluation runs.
tooling: Use `go run ./cmd/repo-memory` for snapshotting and seeding.
---

# WO2: Build Time-Cut Dossiers & Snapshot

You are the Data Engineer. Your task is to prepare the evaluation data for the early kill gate.

## Context
We are executing PR-09-v2-x6-lite. The X6-lite protocol has been approved. The 8 selected work orders are from PR-07 (wo1-op-log, wo2-dossier-schema-renderer, wo3-scopes-aliases, wo4-index-migration, wo6-seeding) and PR-08 (wo1-ledger-engine, wo4-gates, wo6-pull-tools).

## Instructions

1. **Snapshot Requirement (PD17):** First, copy the approved protocol document (`x6-lite-protocol.md`) and the candidate WO transcripts and PA files into the evidence store to protect them from Antigravity pruning. Use the `snapshot` subcommand (`go run ./cmd/repo-memory snapshot ...`) or manual copying as appropriate.
2. **Build Time-Cut Dossiers:** For each of the 8 selected work orders, determine its exact parent commit. Build time-cut dossiers using `--before <parent commit>`. Ensure that no knowledge from the target WO or later is included. 
3. **Time-Cut PA Documents:** Capture the state of the PA documents (PROJECT.md, MEMORY.md, etc.) as they were at the parent commit for each of the 8 WOs. 
4. Store the built dossiers and PA documents in an organized manner (e.g., in an `evaluation/data/` or similar directory) so the replay harness (built in WO3) can easily load them.

## Verification
- Double-check for leakage. No transcript or PA doc from the target WO's commit or later should exist in its time-cut dossier.

## Completion Report Template

```markdown
## Completion Report: WO2 Dossier Build

### Decisions Made
- Snapshotting method: ...
- Storage layout for time-cut data: ...

### Dead Ends & Warnings
- [List any issues encountered]
```
