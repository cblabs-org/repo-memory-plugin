# Local Project Scratchpad — `pr-16-v2-pa-and-rollout`

## Architectural Findings & Package DAG Rules
- **Package Import Boundaries:**
  - `internal/core/episode` MUST NOT import `internal/core/evidence` or `internal/core/inbox` (`inbox -> delivery -> index -> episode`).
  - `internal/core/delivery` MUST NOT import `internal/core/inbox`, `internal/core/coverage`, or `internal/core/tick`.
  - `internal/core/dossiers` exports `EstimateTokens(text string) int` in `lint.go` for deterministic token estimation (`(len(text)+3)/4`, `0` for `""`).
- **WO1 (`wo1-pa-summaries-and-workorder-briefs`) — Completed (`c52caed`):**
  - `repo-memory summary` (`cmd/repo-memory/cmd_summary.go`, `internal/core/episode/summary.go`) resolves stored `.jsonl.zst` episodes, flat `.json` episodes, and raw Antigravity/Claude Code `.jsonl` transcript files/directories.
  - `repo-memory brief` (`cmd/repo-memory/cmd_brief.go`, `internal/core/delivery/workorder_brief.go`) extracts target files/scopes from `WORKORDER.md`, prepends the A6 freshness line, computes coverage gap markers, and ranks active dossier entries + unconsolidated notes.
- **WO2 (`wo2-backfill-and-reproject-cli`) — Completed (`6ec6794`):**
  - `repo-memory backfill` (alias `backfill-captures`, `cmd/repo-memory/cmd_backfill.go`, `internal/core/tick/backfill.go`) supports `--dry-run`, `--json`, `--force`, `--transcripts`, `--docs`, `--git`, and `--max-items`, guarded by A5 spend guards (`SpendConfig.Jobs["backfill"]`).
  - `repo-memory reproject` (`cmd/repo-memory/cmd_reproject.go`, `internal/core/dossiers/reproject.go`) deterministically replays `.memory/ops` (and optional `ops-local`/`user/ops`), checks that all pre-existing entry IDs, `verified` stamps, and human decisions survive (`ErrHumanDecisionLost`), and refreshes `.memory/dossiers/`, `.memory/ATLAS.md`, `.memory/QUESTIONS.md`, and `.agents/skills/rm-*/SKILL.md`.
- **WO3 (`wo3-dogfood-rollout-and-ci-packaging`) — Completed (`d89c041`):**
  - `repo-memory reproject` defaults to `<repoRoot>/.memory/ops` unless `--include-local` is set.
  - Dogfooded `.memory/` state (`30` dossiers, `259` active entries, `225` human decisions preserved) with zero second-run churn and upgraded `.github/workflows/ci.yml`.
- **WO4 (`wo4-programme-closeout-and-handover`) — Completed (`6e765e4` / Polish `005efd4`):**
  - Updated `README.md` with `.memory/QUESTIONS.md` storage layout and the complete `v2.2` CLI subcommand reference, rebuilt `~/.local/bin/repo-memory`, applied all 5 Cold-Firewall PR Audit polish items (`[POLISH-1]`..`[POLISH-5]`), and passed all cross-platform and race-detector verification gates.
