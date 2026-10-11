# Split Daemon + Workspace Plugin — Proposed Pattern Refinements

> Developed during the bug-fixer-loop-plugin architecture review (4–5 Oct 2026).
> These refinements build on the pattern documented in
> `repo-memory-plugin/docs/antigravity-plugin-architecture.md` and are proposed
> as shared conventions for both plugins.

---

## 1. Sidecar Folders as Definitive Source (No Separate Registry)

**Current repo-memory pattern:** External `registry.json` tracks enrolled repos.

**Proposed:** Eliminate the separate registry. The `sidecars/` directory structure
IS the registry:

```
sidecars/
├── esp32-app-foundation/
│   ├── sidecar.json     # sidecar config
│   └── config.json      # per-repo config (enrolment data, intervals, etc.)
└── another-repo/
    ├── sidecar.json
    └── config.json
```

- **Discovery:** `ls sidecars/` — each directory with a `sidecar.json` is an enrolled repo
- **No sync bugs:** Registry can't drift from actual sidecars
- **Extra files in sidecar dirs are ignored** by Antigravity's sidecar loader (verify in spike)
- **One less file to manage** on `enroll`/`unenroll`

> [!NOTE]
> For plugins with a single shared sidecar (like repo-memory's `tick/`), the
> registry is still needed because the sidecar doesn't map 1:1 to repos. This
> refinement applies when there's one sidecar per enrolled repo.

---

## 2. Workspace Cache for Hook Performance

**Problem:** Hook scripts need to check "is this workspace enrolled?" on every
tool call. Scanning `sidecars/*/config.json` and matching `workspacePaths` is
cheap (1–5 dirs, sub-ms), but adds up for heavy hook sets.

**Proposed:** Treat the sidecar folders as **definitive** and add an optional
**cache** for hook lookups:

```
bug-fixer-daemon/
└── workspace-cache.json    # { "/abs/path/to/repo": "sidecar-dir-name", ... }
```

- Hook scripts read cache first (one `json.load`, one dict lookup)
- Cache miss → scan `sidecars/*/config.json`, rebuild cache
- `enroll`/`unenroll`/`enable`/`disable` invalidate the cache (delete or rewrite)

This matters more for repo-memory (4 hook events: PreInvocation, PreToolUse,
PostToolUse, Stop) than for bug-fixer (PreToolUse only), but it's the right
pattern for both.

---

## 3. Command Naming

**Current repo-memory pattern:** `init` / `leave`

**Proposed:** `enroll` / `unenroll` — clearer about what's happening (the repo
is being enrolled into a system, not initialised).

| Command | Scope | What it does |
|---------|-------|-------------|
| `install` | Machine | Creates `~/.gemini/config/plugins/<name>-daemon/` with sidecar + workspace bundle |
| `uninstall` | Machine | Removes the entire daemon directory |
| `enroll` | Repo | Renders per-repo sidecar + config, writes `.agents/plugins.json` |
| `unenroll` | Repo | Removes sidecar dir + `.agents/plugins.json` entry |
| `enable` | Toggle | Re-adds `.agents/plugins.json` entry (keeps config) |
| `disable` | Toggle | Removes `.agents/plugins.json` entry (keeps config) |

---

## 4. Dispatcher Scripts Inside Workspace Plugin

**Applies to:** Plugins where the sidecar runs scripts (not a compiled binary).

**Proposed:** Put ALL executable logic in the workspace plugin, not the daemon:

```
bug-fixer-daemon/
├── scripts/
│   └── tick.py                          # MINIMAL: read config, invoke tick_receiver
│
└── workspace-plugin/
    └── bug-fixer/
        └── scripts/
            ├── tick_receiver.py          # Internal cron logic
            ├── engineering_dispatcher.py  # All dispatch logic
            ├── triage_dispatcher.py
            ├── pre_tool_gate.py          # Hook scripts
            └── ...
```

The global daemon's `tick.py` is a ~12-line router. It reads the repo from env
vars, finds the config in its sidecar directory, and invokes `tick_receiver.py`
in the workspace plugin.

**Benefits:**
- Single location for all logic — hooks, skills, rules, AND dispatchers
- Changes to dispatchers propagate to all enrolled repos immediately
- Daemon is truly just a heartbeat

> [!NOTE]
> This doesn't apply to repo-memory, where the tick binary IS the logic.
> It applies to plugins where sidecars invoke script-based dispatchers.

---

## 5. Per-Repo Sidecars with `agent_permissions`

**Applies to:** Plugins whose sidecars spawn `agentapi new-conversation`.

**repo-memory pattern:** Single `tick/` sidecar, no `agent_permissions` (correct
— `repo-memory tick` is a pure OS binary that never starts agent conversations).

**Bug-fixer pattern:** One sidecar per enrolled repo, each with
`agent_permissions.workspace_uris` scoped to that repo:

```json
{
  "builtin": "schedule",
  "args": ["* * * * *", "<daemon>/scripts/tick.py"],
  "agent_permissions": {
    "workspace_uris": ["file:///path/to/repo"]
  },
  "env": {
    "BUG_FIXER_REPO": "owner/repo",
    "ANTIGRAVITY_PROJECT_ID": "..."
  }
}
```

- Conversations spawned by the dispatcher inherit the sidecar's `agent_permissions`
- Each repo is scoped independently (conversation can't reach another repo)
- One dashboard approval switch per repo
- `agent_permissions` triggers per-sidecar approval, not plugin-level disable

**Decision rule:** If your sidecar spawns `agentapi` conversations → needs
`agent_permissions`. If it's a pure OS subprocess → omit them.

---

## 6. Internal Cron via `tick_receiver.py`

**Problem:** With per-repo sidecars, you can't have different cron expressions
per loop type (triage hourly, engineering every 10 min). The sidecar has one cron.

**Solution:** Single sidecar fires every minute. `tick_receiver.py` manages
internal intervals:

```python
LOOPS = {
    "triage":      {"script": "triage_dispatcher.py",     "interval_min": 60},
    "engineering": {"script": "engineering_dispatcher.py", "interval_min": 10},
}

for name, loop in LOOPS.items():
    marker = state_dir / f"{name}.last"
    if marker.exists() and minutes_since(marker) < loop["interval_min"]:
        continue
    subprocess.Popen([sys.executable, str(SCRIPT_DIR / loop["script"])], ...)
    marker.touch()
```

State files (`state/triage.last`, etc.) live in the sidecar's directory alongside
`config.json`.

**Result:** N sidecars total (1 per repo) instead of 4×N, with configurable
per-loop intervals in `config.json`.

---

## 7. Phase 0 Spike Checks (Shared)

These validate undocumented behaviours both plugins depend on:

| # | Check | Needed by |
|---|-------|-----------|
| 1 | Adding a new `sidecars/<name>/sidecar.json` to a loaded plugin appears in dashboard without restart | Both |
| 2 | Sidecar with `agent_permissions` gets its own approval switch (not plugin-level) | Bug-fixer |
| 3 | Extra files (`config.json`) in sidecar directory are ignored by loader | Both |
| 4 | `agentapi new-conversation` from sidecar subprocess inherits `agent_permissions` | Bug-fixer |
| 5 | Conversation scoped to one repo is denied access to another | Bug-fixer |
| 6 | `${PLUGIN_ROOT}` substitution in sidecar args; `PLUGIN_DATA` vs `ANTIGRAVITY_EXECUTABLE_DATA_DIR` | Both |
| 7 | `workspace-plugin/x/plugin.json` not loaded globally; loaded via `.agents/plugins.json` | Both |
| 8 | Hook stdin `workspacePaths` contains correct repo path in multi-root windows | Both |

---

## Summary of Refinements

| # | Refinement | Applies to |
|---|-----------|------------|
| 1 | Sidecar folders as definitive source (no separate registry) | Plugins with per-repo sidecars |
| 2 | Workspace cache for hook performance | Both |
| 3 | `enroll`/`unenroll` naming | Both |
| 4 | Dispatcher scripts inside workspace plugin | Script-based plugins |
| 5 | Per-repo sidecars with `agent_permissions` | Plugins spawning agent conversations |
| 6 | Internal cron via `tick_receiver.py` | Multi-loop plugins |
| 7 | Shared Phase 0 spike checks | Both |
