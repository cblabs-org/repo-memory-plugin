# Bug-Fixer Plugin — Revised Architecture

> Consolidated from the evaluation and second opinion review, 4 Oct 2026.
> All 9 corrections verified and incorporated.

---

## Design Summary

Split the plugin into two layers:

1. **Global daemon plugin** (`~/.gemini/config/plugins/bug-fixer-daemon/`) — sidecars only, zero workspace overhead
2. **Shared workspace plugin** (nested inside the daemon at `workspace-plugin/bug-fixer-hooks/`) — hooks, skills, rules, agents. Loaded per-repo via `.agents/plugins.json`

One sidecar per enrolled repo (not 4 × N). Each sidecar runs `tick.py` once per minute, which dispatches whichever loops are due. Existing dispatcher logic stays single-repo — no multi-repo refactor.

---

## Corrections Accepted

All 9 corrections from the second opinion are incorporated:

| # | My error | Correction | Status |
|---|----------|------------|:---:|
| 1 | Remove `agent_permissions` | **Keep** — dispatchers start agentapi conversations that need `workspace_uris`. Dashboard approval is one-time per sidecar. | ✅ |
| 2 | Sidecar grants cause disabled-by-default | **Wrong** — `plugin.json` ships `"disabled": true` deliberately ([commit d7bf546](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/plugin.json)). That's the cause, not sidecar grants. | ✅ |
| 3 | Replace `workspace_uris` with `ANTIGRAVITY_PROJECT_ID` | **Keep both** — nobody has tested with only one. Project grants carry `fileAccessPolicy: ALLOW` and sandbox-off. | ✅ |
| 4 | Remove old submodule in Phase 3 | **Phase 1** — old submodule name `bug-fixer-loop` collides with workspace plugin. One gets dropped silently. | ✅ |
| 5 | Remove injected hooks in Phase 3 | **Phase 1** — legacy hooks + new plugin hooks would double-fire, blocking the agent. | ✅ |
| 6 | Use `${PLUGIN_ROOT}` / `${PLUGIN_DATA}` | **Unverified locally** — only in pasted notes citing v1.2.7. Locally documented: `ANTIGRAVITY_EXECUTABLE_DATA_DIR` (per-sidecar, agents can't write). Use absolute paths + external registry. | ✅ |
| 7 | Hook workspace check is defense-in-depth | **Mandatory** — `pre_tool_gate.py:117-118` resolves config via `plugin_dir.parent / "bug-fixer-loop-workspace"` or `cwd`. Both break when global. Must use `workspacePaths` → registry lookup. | ✅ |
| 8 | Single-point upgrades are free | **Need tagged copies** — symlinked install makes checked-out branch live everywhere. Record version in registry. | ✅ |
| 9 | Multi-repo dispatchers are ~30–50 lines | **Avoided entirely** — per-repo sidecar design keeps dispatchers single-repo. `tick.py` replaces 4 cron entries per repo. | ✅ |

---

## Directory Layout

```
~/.gemini/config/plugins/bug-fixer-daemon/
├── plugin.json                          # { "name": "bug-fixer-daemon" }
│
├── scripts/
│   ├── tick.py                          # NEW: unified entry point per repo
│   ├── engineering_dispatcher.py        # unchanged single-repo logic
│   ├── triage_dispatcher.py
│   ├── architect_dispatcher.py
│   ├── trajectory_dispatcher.py
│   ├── liveness.py
│   ├── bugfixer_api.py
│   └── safe_ingest.py
│
├── sidecars/                            # one sidecar PER enrolled repo
│   ├── esp32-app-foundation/
│   │   └── sidecar.json                 # rendered from registry by configure.sh
│   └── bug-fixer-loop-plugin/
│       └── sidecar.json
│
└── workspace-plugin/                    # NOT loaded by global plugin reader
    └── bug-fixer-hooks/                 # name ≠ "bug-fixer-loop" to avoid collision
        ├── plugin.json                  # { "name": "bug-fixer-hooks" }
        ├── hooks.json                   # PreToolUse gates
        ├── rules/
        │   └── AGENTS.md
        ├── skills/
        │   ├── bug-fixer/SKILL.md
        │   ├── bug-fixer-engineering/SKILL.md
        │   ├── bug-fixer-triage/SKILL.md
        │   ├── bug-fixer-trajectory-review/SKILL.md
        │   ├── architect-review/SKILL.md
        │   └── test-quality-audit/SKILL.md
        ├── agents/
        │   └── leak-auditor.md
        └── scripts/                     # hook scripts only
            ├── pre_tool_gate.py
            └── github_url_gate.py
```

### External State (Not Inside Plugin)

```
~/.local/share/bug-fixer/
├── registry.json                        # [{repo, repo_dir, project_id, version}, ...]
└── repos/
    ├── esp32-app-foundation/
    │   └── config.json                  # maintainer, test_command, labels, intervals
    └── bug-fixer-loop-plugin/
        └── config.json
```

### Per Enrolled Repo

```
<repo>/.agents/
└── plugins.json
```

```json
{
  "entries": [
    { "path": "~/.gemini/config/plugins/bug-fixer-daemon/workspace-plugin" }
  ]
}
```

---

## Per-Repo Sidecar Design

Each sidecar runs `tick.py` every minute. `tick.py` checks which of the 4 loops are due (based on configured intervals) and runs them as subprocesses. The existing dispatchers keep their single-repo logic — they read the repo from env vars.

```json
{
  "builtin": "schedule",
  "args": [
    "* * * * *",
    "<abs_path>/bug-fixer-daemon/scripts/tick.py"
  ],
  "restart_policy": "always",
  "display_name": "Bug Fixer (matthewmcneill/esp32-app-foundation)",
  "description": "Runs triage, engineering, review, and trajectory loops for this repo.",
  "agent_permissions": {
    "workspace_uris": [
      "file:///Users/mcneillm/Documents/Projects/esp32-app-foundation"
    ]
  },
  "env": {
    "BUG_FIXER_REPO": "matthewmcneill/esp32-app-foundation",
    "ANTIGRAVITY_PROJECT_ID": "fee1fa3f-77c2-48e9-bf1f-649ba5599473",
    "PATH": "<captured at enrol>"
  }
}
```

### `tick.py` Sketch

```python
#!/usr/bin/env python3
"""Unified entry point: runs whichever loops are due for one repo."""
import json, os, subprocess, sys, time
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
REPO_SLUG = os.environ["BUG_FIXER_REPO"]
REGISTRY = Path.home() / ".local/share/bug-fixer/registry.json"

# Load per-repo config
repo_safe = REPO_SLUG.replace("/", "--")
config_path = REGISTRY.parent / "repos" / repo_safe / "config.json"
config = json.loads(config_path.read_text())

LOOPS = {
    "triage":       {"script": "triage_dispatcher.py",      "interval": config.get("triage_interval_min", 60)},
    "engineering":  {"script": "engineering_dispatcher.py",  "interval": config.get("engineering_interval_min", 10)},
    "pr_review":    {"script": "architect_dispatcher.py",    "interval": config.get("review_interval_min", 15)},
    "trajectory":   {"script": "trajectory_dispatcher.py",   "interval": config.get("trajectory_interval_min", 1440)},
}

state_dir = REGISTRY.parent / "repos" / repo_safe / "state"
state_dir.mkdir(parents=True, exist_ok=True)

for name, loop in LOOPS.items():
    marker = state_dir / f"{name}.last"
    if marker.exists():
        elapsed = time.time() - marker.stat().st_mtime
        if elapsed < loop["interval"] * 60:
            continue
    # Run the dispatcher as a subprocess (inherits env: BUG_FIXER_REPO, PROJECT_ID, etc.)
    subprocess.Popen(
        [sys.executable, str(SCRIPT_DIR / loop["script"])],
        env={**os.environ, "BUG_FIXER_CONFIG": str(config_path)},
    )
    marker.touch()
```

### Result

| Metric | Before | After |
|--------|--------|-------|
| Sidecars for 2 repos | 8 (4 × 2) | 2 (1 × 2) |
| Sidecars for 5 repos | 20 (4 × 5) | 5 (1 × 5) |
| Dashboard switches | 8 | 2 |
| Per-repo scoping | via env vars | via env vars + `agent_permissions` |
| Plugin-level stop | ❌ No | ✅ Disable `bug-fixer-daemon` stops all |

---

## Hook Config Resolution Fix

Current [`pre_tool_gate.py:115-120`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/pre_tool_gate.py#L115-L120) resolves config via relative paths from `__file__` or `cwd`. Both break in the new layout.

New resolution order:

```python
def _load_config(explicit_path=None):
    # 1. Explicit path (CLI --config)
    if explicit_path:
        return json.loads(Path(explicit_path).read_text())

    # 2. BUG_FIXER_CONFIG env var (set by sidecar stubs)
    if os.environ.get("BUG_FIXER_CONFIG"):
        return json.loads(Path(os.environ["BUG_FIXER_CONFIG"]).read_text())

    # 3. Registry lookup via workspacePaths (hooks receive this on stdin)
    # The hook's stdin includes workspacePaths — match against registry
    registry_path = Path.home() / ".local/share/bug-fixer/registry.json"
    if registry_path.exists():
        registry = json.loads(registry_path.read_text())
        for entry in registry:
            if workspace_path and workspace_path.startswith(entry["repo_dir"]):
                repo_safe = entry["repo"].replace("/", "--")
                config_file = registry_path.parent / "repos" / repo_safe / "config.json"
                if config_file.exists():
                    return json.loads(config_file.read_text())

    # 4. Fail-open defaults (no config found)
    return {"spec_file_path": "docs/design_specification.md", ...}
```

---

## Phase 0 Spike Checks

> ~Half a day with a throwaway `bug-fixer-spike` plugin.

| # | Check | Pass | If fail |
|---|-------|------|---------|
| 1 | Add a second `sidecars/<name>/sidecar.json` to loaded plugin | Appears in Automations dashboard without restart | Use standalone sidecars, or accept restart per enrolment |
| 2 | Give that sidecar `agent_permissions.workspace_uris` | Has its own switch, recorded in `config.json` under `sidecars` keyed as `bug-fixer-spike/<name>` | Standalone sidecars: plugin-level-only switching would bypass per-repo approval |
| 3 | Edit an approved `sidecar.json` (change cron) | Keeps running or asks for approval again | Only re-render on upgrade, never on every tick |
| 4 | Spawn `agentapi new-conversation` from sidecar with `workspace_uris` + `PROJECT_ID` | Conversation writes file, runs `git status` and `gh issue list` with no prompt | Revisit grants |
| 5 | Same conversation tries to write in a second repo | **Denied** | Scoping is not per-sidecar; design needs rethinking |
| 6 | Put `${PLUGIN_ROOT}` in `args` and print env from script | Note substitution; check for `PLUGIN_DATA` vs `ANTIGRAVITY_EXECUTABLE_DATA_DIR` | Use absolute paths rendered at install time |
| 7 | Add `workspace-plugin/x/plugin.json`; run `GetAllPlugins` | Not listed globally; listed in repo that names it in `.agents/plugins.json` | Move workspace plugin to `~/.local/share/bug-fixer/` |
| 8 | In that repo, have the hook print its stdin `workspacePaths` | Contains the repo path, including in multi-root window | Fall back to hook cwd + registry |

**If checks 1–2 pass → daemon-plugin layout. If either fails → standalone sidecars in `~/.gemini/config/sidecars/`. All other checks apply to both.**

---

## Phased Implementation

### Phase 0 — Spike (no production changes)
Run the 8 checks above. Pick daemon-plugin or fallback layout.

### Phase 1 — Workspace Plugin (PR1)
In a single PR, for each enrolled repo:
- [ ] Create `workspace-plugin/bug-fixer-hooks/` with hooks, skills, rules, agents, hook scripts
- [ ] Name the workspace plugin `bug-fixer-hooks` (NOT `bug-fixer-loop` — collision with old submodule)
- [ ] Make `pre_tool_gate.py` and `github_url_gate.py` resolve config via `workspacePaths` → registry
- [ ] Create `~/.local/share/bug-fixer/registry.json` and migrate each repo's config
- [ ] In each consumer repo: add `.agents/plugins.json`, remove injected hooks from `.agents/hooks.json`, remove the git submodule
- [ ] Leave existing 8 standalone sidecars running (pointed at new script locations)

### Phase 2 — Per-Repo Sidecars (PR2)
- [ ] Write `tick.py` (interval-based loop dispatcher)
- [ ] Have dispatchers read repo from `BUG_FIXER_REPO` env var + registry config path
- [ ] `configure.sh` renders one sidecar per repo with `agent_permissions`, prints dashboard link
- [ ] Enable new sidecar for one repo, confirm full triage→fix→review cycle
- [ ] Delete that repo's 4 old standalone sidecars, repeat for second repo

### Phase 3 — Cleanup & Install Model (PR3)
- [ ] `install.sh` installs a tagged copy and re-renders sidecars from registry
- [ ] Delete `hooks.json.template` and hook-injection code
- [ ] Remove stale `sidecars` entries from `config.json` via language server RPC (not by hand)
- [ ] Write uninstall steps: plugin, per-repo `plugins.json`, registry, enable entries
- [ ] Update README

---

## Fallback Path

If Phase 0 checks 1–2 fail (plugin sidecars can't be added live or lack per-sidecar switches):

Write the same `sidecar.json` files to `~/.gemini/config/sidecars/bug-fixer--<repo>/` instead of `sidecars/<repo>/` inside the daemon plugin. Everything else stays identical. The only loss is the plugin-level stop switch (disable `bug-fixer-daemon` wouldn't stop standalone sidecars).
