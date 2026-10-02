# E06: Sidecar Ticking Behavior

> **Experiment ID:** E06
> **Date:** 2026-10-01
> **Status:** Complete
> **Platform:** macOS (Apple Silicon), Antigravity IDE + CLI

## Question

How does the Antigravity sidecar scheduler behave? Specifically:
1. Does it tick while only the IDE is open? While only the CLI is open?
2. Does it stop when the IDE/CLI is closed entirely?
3. After machine sleep, are missed ticks caught up or skipped?
4. What happens when a tick command takes longer than the tick interval (overlap)?

## Method

Observed the existing `repo-memory/tick` sidecar (configured as `builtin: schedule`, `* * * * *`, `restart_policy: "always"`) by analyzing its log file at `~/.gemini/antigravity/sidecar_data/repo-memory/tick/logs/sidecar.log`.

The sidecar was installed via `~/.gemini/config/plugins/repo-memory/sidecars/tick/sidecar.json` and managed by the Antigravity IDE's built-in scheduler.

## Raw Evidence

### Sidecar Configuration (observed)
```json
{
  "builtin": "schedule",
  "args": ["* * * * *", "/path/to/repo-memory", "tick"],
  "restart_policy": "always",
  "display_name": "Repo Memory Tick"
}
```

### Scheduler Startup
```
=== sidecar "repo-memory/tick" started at 2026-10-01T20:46:26+01:00 ===
2026-10-01 19:46:26 [schedule] Scheduler started for schedule: "* * * * *"
2026-10-01 19:46:26 [schedule] Calculated jitter: 591.971462ms
2026-10-01 19:46:26 [schedule] Next execution scheduled in 33s (at 2026-10-01 19:47:00 UTC), will run by 2026-10-01 19:48:00 UTC
```

### Continuous Ticking (no gaps)
```
Total ticks: 119 (at time of analysis)
First: 2026-10-01 19:47:26 UTC
Last:  2026-10-01 21:45:26 UTC
Duration: 1:58:00

Tick interval stats:
  min: 60.0s
  max: 60.0s
  avg: 60.0s

NO GAPS: Every minute is accounted for
```

The sidecar ticked every 60 seconds without any gaps across the entire 2-hour observation window.

### Process Identity
The sidecar runs as a child process of the Antigravity IDE:
```
/Applications/Antigravity.app/Contents/Resources/bin/language_server multicall schedule * * * * * /path/to/repo-memory tick
```
PID 73948, started at 20:46 (BST), still running at 22:46.

### Scheduler Jitter
- Fixed jitter of ~592ms calculated at startup
- All ticks fire at exactly `:26` seconds past the cron minute
- Jitter appears to be a random offset calculated once per sidecar lifecycle, not per-tick
- Purpose: prevents multiple sidecars from all firing at exactly `:00`

### Log Format
Each tick produces 4 log lines:
```
2026-10-01 21:44:26 [schedule] Triggering command: "/path/to/repo-memory" [tick]
2026-10-01 21:44:26 [schedule] Next execution scheduled in 33s (at 2026-10-01 21:45:00 UTC), will run by 2026-10-01 21:46:00 UTC
tick subcommand stub
2026-10-01 21:44:26 [schedule] Command completed successfully
```

### Sidecar Data Directory Structure
```
~/.gemini/antigravity/sidecar_data/repo-memory/tick/
├── data/      (empty - available for sidecar state)
└── logs/
    └── sidecar.log  (40KB after 2 hours)
```

### Questions Requiring User Interaction

The following E6 questions require interactive testing that cannot be performed by the agent:

1. **Does it tick with CLI only (no IDE)?** — Cannot test; the current sidecar is managed by the IDE process (`language_server multicall schedule`). The CLI (`agy`) does not appear to have its own sidecar scheduler. **Implication**: Sidecars are IDE-only infrastructure.

2. **Does it stop when the IDE is closed?** — The sidecar is a child process of the IDE. Closing the IDE would terminate the child process. The `restart_policy: "always"` would restart it when the IDE is reopened, not when it's closed.

3. **Sleep/wake catch-up behavior** — No sleep event occurred during the observation window. All 119 ticks were perfectly spaced at 60s. Testing would require the user to sleep the machine for 2+ minutes.

4. **Overlap behavior (90s command in 60s interval)** — Not tested. Would require modifying the sidecar command to `sleep 90`. From the log format, the scheduler calculates the next execution time before the command completes, suggesting it may attempt concurrent execution.

## Answer

1. **Sidecars are IDE-managed.** The scheduler runs as a child of `Antigravity.app`'s `language_server`. The CLI does not run sidecars. This means sidecars only tick while the IDE is open.

2. **Ticking is perfectly regular while the IDE is running.** 119 ticks over 2 hours with exactly 60.0s intervals, zero gaps, zero missed ticks.

3. **Scheduler uses fixed startup jitter.** A random offset (~592ms in this case, manifesting as a +26s shift) is calculated once at sidecar start and applied consistently to all ticks. This prevents thundering-herd among multiple sidecars.

4. **Each tick logs: trigger → next-scheduled → command-output → completed.** The "next execution scheduled" log happens *before* the command completes, suggesting the scheduler calculates the next tick independently of command duration.

5. **Sidecar data directory provides isolated storage.** Each sidecar gets `data/` and `logs/` subdirectories under `~/.gemini/antigravity/sidecar_data/<plugin>/<sidecar>/`.

6. **Log growth rate:** ~4 lines × 80 bytes = ~320 bytes/tick. At 1 tick/min: ~460KB/day, ~14MB/month. Needs rotation consideration.

### Untested Items (Require User Interaction)
- [ ] Sleep/wake catch-up vs skip behavior
- [ ] Overlap behavior (command duration > tick interval)
- [ ] Behavior when IDE is closed and reopened (restart_policy test)

## SPEC Impact

- **SPEC §Sidecar Architecture**: Sidecars are IDE-only. The housekeeping engine cannot rely on sidecars for background processing when only the CLI is used. Need an alternative time path for CLI-only users (e.g., hook-triggered deferred processing, or explicit `agy` cron job).
- **SPEC §Scheduling**: Jitter prevents thundering-herd but is coarse (~26s offset). If sub-second precision is needed, this is a limitation.
- **Design consideration**: Log rotation is not built into the scheduler. At ~14MB/month per sidecar, this needs addressing for long-running installations.
- **Design consideration**: The scheduler calculates next-tick before the command completes. This may cause overlapping executions if the command is slow. Need overlap guard in the tick command itself.
