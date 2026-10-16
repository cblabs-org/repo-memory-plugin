# Local Project Scratchpad — `pr-16-v2-pa-and-rollout`

## Architectural Findings & Package DAG Rules
- **Package Import Boundaries:**
  - `internal/core/episode` MUST NOT import `internal/core/evidence` or `internal/core/inbox` (`inbox -> delivery -> index -> episode`).
  - `internal/core/delivery` MUST NOT import `internal/core/inbox`, `internal/core/coverage`, or `internal/core/tick`.
  - `internal/core/dossiers` exports `EstimateTokens(text string) int` in `lint.go` for deterministic token estimation (`(len(text)+3)/4`, `0` for `""`).
- **WO1 (`wo1-pa-summaries-and-workorder-briefs`) — Completed (`c52caed`):**
  - `repo-memory summary` (`cmd/repo-memory/cmd_summary.go`, `internal/core/episode/summary.go`) resolves stored `.jsonl.zst` episodes, flat `.json` episodes, and raw Antigravity/Claude Code `.jsonl` transcript files/directories.
  - `repo-memory brief` (`cmd/repo-memory/cmd_brief.go`, `internal/core/delivery/workorder_brief.go`) extracts target files/scopes from `WORKORDER.md`, prepends the A6 freshness line, computes coverage gap markers, and ranks active dossier entries + unconsolidated notes.
