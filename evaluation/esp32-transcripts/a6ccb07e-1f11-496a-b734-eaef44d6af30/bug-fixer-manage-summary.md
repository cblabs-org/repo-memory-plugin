# Bug-Fixer Management Skill — Implementation Summary

## What was built

### 1. `manage.py` — Central management CLI
**Source**: [`daemon/scripts/manage.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/daemon/scripts/manage.py)
**Deployed**: `~/.gemini/config/plugins/bug-fixer-daemon/scripts/manage.py`

Subcommands:
| Command | Type | Description |
|---------|------|-------------|
| `status` | Read-only | Health check — loop status, errors, claims, cooldowns |
| `logs <loop>` | Read-only | Tail loop log files |
| `config [--show]` | Read-only | Display configuration |
| `enroll` | Mutable | Enroll a repo (takes explicit `--project-id`) |
| `unenroll` | Mutable | Remove a repo |
| `config --set` | Mutable | Update config values |
| `reset` | Mutable | Clear cooldowns, locks, dispatches |
| `labels --sync` | Mutable | Create required GitHub labels |
| `refresh-sidecars` | Internal | Regenerate sidecar.json (used by install.sh) |

### 2. `SKILL.md` — Thin management skill
**Source**: [`daemon/skills/bug-fixer-manage/SKILL.md`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/daemon/skills/bug-fixer-manage/SKILL.md)

Maps user intent to `manage.py` subcommands. Enforces **ask_question HITL gate** for all mutable operations.

### 3. `install.sh` — Auto-refresh on upgrade
**Source**: [`scripts/install.sh`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/install.sh)

Now calls `manage.py refresh-sidecars` after rsync, so upgrades preserve per-repo values (project ID, PATH) without requiring re-enrollment.

## Security model

- **HITL via `ask_question`**: All mutable operations require human confirmation. Autonomous agents (spawned by dispatchers) have no human to respond → naturally blocked.
- **Skill isolation**: The management skill lives in the daemon plugin (global), not the workspace plugin. Autonomous bug-fixer sessions only get workspace-plugin skills.
- **Explicit CLI args**: `manage.py enroll` takes `--project-id` as an argument — no environment variable dependencies. The agent reads env vars and passes them as explicit values after human approval.

## Remaining manual step

> [!IMPORTANT]
> The pre-tool gate (`pre_tool_gate.py`) blocks `run_command` calls that reference paths under the daemon directory. This means `python3 ~/.../manage.py status` gets blocked even though it's read-only.
>
> **You need to add `manage.py` to the trusted scripts list from a terminal** (the gate protects itself from agent modification — by design).

### Fix: Add to `daemon/workspace-plugin/bug-fixer/scripts/pre_tool_gate.py`

After the `HOST_SECURITY_SCRIPTS` definition (line 163-167), add:

```python
# Scripts in the daemon's scripts/ dir that agents are allowed to execute.
DAEMON_TRUSTED_SCRIPTS = frozenset({
    "manage.py", "tick.py",
})
```

Then in the `_ProtectedPaths.build()` method (around line 939-944), add after the host_scripts loop:

```python
        # Also trust daemon-level scripts (manage.py, tick.py)
        daemon_dir = _find_daemon_dir(_SCRIPTS_DIR)
        if daemon_dir:
            daemon_scripts_dir = daemon_dir / "scripts"
            for script in DAEMON_TRUSTED_SCRIPTS:
                real = os.path.normpath(str(daemon_scripts_dir / script))
                host_scripts.add(real)
```

Then re-run `./scripts/install.sh` to deploy the change.

### After that, re-enroll both repos

From within each target workspace's agent session, say **"enroll this repo in bug-fixer"** — the skill will handle the rest with the HITL confirmation flow.
