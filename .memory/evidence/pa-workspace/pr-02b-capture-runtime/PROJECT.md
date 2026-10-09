# PR-02b: Capture Runtime

> **Programme:** repo-memory
> **PR:** pr-02b-capture-runtime
> **Base branch:** `main`
> **Status:** In Progress
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Every Antigravity session in an enrolled repository produces an episode within minutes of going idle. The `Stop` hook copies transcripts incrementally; the tick sidecar ingests the spool, runs the job table, sweeps, and builds episodes with PR-02a. This repository is enrolled as the dogfood repository.

## Work Order Ledger

| # | WO | Branch | Status | Complexity | Worker |
|---|---|---|---|---|---|
| 1 | [`wo1-spool-incremental-copy`](wo1-spool-incremental-copy/WORKORDER.md) | `pr-02b-capture-runtime/wo1-spool-incremental-copy` | ✅ Complete | standard | bc516ac3 |
| 2 | [`wo2-stop-hook`](wo2-stop-hook/WORKORDER.md) | `pr-02b-capture-runtime/wo2-stop-hook` | ✅ Complete | standard | 870043d |
| 3 | [`wo3-job-table`](wo3-job-table/WORKORDER.md) | `pr-02b-capture-runtime/wo3-job-table` | ✅ Complete | standard | 9df91b8 |
| 4 | [`wo4-tick-core`](wo4-tick-core/WORKORDER.md) | `pr-02b-capture-runtime/wo4-tick-core` | ✅ Complete | architect | e5c26427 |
| 5 | [`wo5-session-attribution`](wo5-session-attribution/WORKORDER.md) | `pr-02b-capture-runtime/wo5-session-attribution` | ✅ Complete | standard | 17d10203 |
| 6 | [`wo6-sidecar-packaging`](wo6-sidecar-packaging/WORKORDER.md) | `pr-02b-capture-runtime/wo6-sidecar-packaging` | ✅ Complete | mechanical | 66f07588 |
| 7 | [`wo7-claude-raw-copy`](wo7-claude-raw-copy/WORKORDER.md) | `pr-02b-capture-runtime/wo7-claude-raw-copy` | ✅ Complete | mechanical | f8ee0bc6 |

## Exit Criteria
- Every new Antigravity session in an enrolled repository produces an episode within minutes of going idle
- Sessions from non-enrolled workspaces are never read or copied
- Long sessions don't re-copy whole transcripts on each turn (incremental copy verified)
- Subagents linked to parents; app/CLI duplicates removed; commits linked
- `tick.lock` verified on macOS and Windows
- **repo-memory-plugin enrolled as the dogfood repository**

## Session Log

- **WO1**: `wo1-spool-incremental-copy` completed. The spool schema, writer (with Windows atomic rename support), reader, and incremental transcript copier were verified and committed.
- **WO2**: `wo2-stop-hook` completed. Stop hook added with kill-switch, enrolment, and housekeeping guards. Bounded to 200ms budget. Added `repo-memory` to `.gitignore` after encountering a global exit-127 hook lockup due to missing binary.
- **WO3**: `wo3-job-table` completed. SQLite job table with WAL mode implemented using pure-Go modernc.org/sqlite driver. Verified correct handling of idempotency, scheduling, and leases.
- **WO4**: `wo4-tick-core` completed. Implemented portable tick lock, the core tick loop, and sweep logic for enrolled repos including idle promotion and incremental copy.
- **WO5**: `wo5-session-attribution` completed. Implemented mapping of conversations to enrolled repositories using `conversation_summaries.db`, duplicated-conversation resolution between IDE and CLI, and hierarchical subagent linking. Wired attribution into the tick's sweep loop.
- **WO6**: `wo6-sidecar-packaging` completed. Updated the cron schedule to `*/1 * * * *` and ensured the Automations-dashboard enablement step prints on install.
- **WO7**: `wo7-claude-raw-copy` completed. `TranscriptCopier` registry added to the tick, scanning and mapping Claude Code project directories to enrolled repo IDs. Copies `raw/claude-code/` transcripts securely without executing arbitrary plugins.

### Project Wrap-Up
All 7 Work Orders have successfully completed.
- Spool, Stop hook, job table, tick core loop, session attribution, packaging, and claude raw copy logic are all implemented.
- Met all exit criteria: the capture pipeline is live and tracking idle sessions.
- An issue arose in WO2 where a missing binary broke the `PreToolUse` hook and blocked further tool execution, highlighting the risk of global plugin dependencies. Handled by disabling the global hooks temporarily and ignoring the built binary.
- This PR successfully establishes the reliable foundation necessary for the `repo-memory` program's event-path and time-path processes.
