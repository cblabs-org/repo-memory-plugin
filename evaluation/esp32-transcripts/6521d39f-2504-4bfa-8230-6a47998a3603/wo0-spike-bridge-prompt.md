# WO0 — Phase 0 Spike: Bridge Prompt

> **Type:** Interactive — you drive this in a live Antigravity session.
> **Duration:** ~30 minutes.
> **Outcome:** Pass/fail table that determines the WO1–WO6 implementation path.

## Setup

Create a throwaway global plugin to test against. Nothing touches bug-fixer-loop-plugin.

```bash
# 1. Create the test plugin structure
mkdir -p ~/.gemini/config/plugins/spike-test-daemon/scripts
mkdir -p ~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe/scripts
mkdir -p ~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe/skills/spike-skill
mkdir -p ~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe/rules
mkdir -p ~/.gemini/config/plugins/spike-test-daemon/sidecars/test-repo

# 2. Create the daemon plugin.json
cat > ~/.gemini/config/plugins/spike-test-daemon/plugin.json << 'EOF'
{
  "name": "spike-test-daemon",
  "description": "Throwaway spike for plugin architecture validation"
}
EOF

# 3. Create the workspace plugin.json
cat > ~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe/plugin.json << 'EOF'
{
  "name": "spike-probe",
  "description": "Workspace plugin for spike testing"
}
EOF

# 4. Create a test hook
cat > ~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe/hooks.json << 'EOF'
{
  "spike-test-hook": {
    "PreToolUse": [
      {
        "matcher": "write_to_file",
        "hooks": [
          {
            "type": "command",
            "command": "python3 scripts/spike_hook.py",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
EOF

# 5. Create the hook script
cat > ~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe/scripts/spike_hook.py << 'PYEOF'
#!/usr/bin/env python3
import json, os, sys
stdin_data = {}
try:
    stdin_data = json.load(sys.stdin)
except Exception:
    pass

# Log everything we receive
log_path = os.path.expanduser("~/.gemini/config/plugins/spike-test-daemon/spike_log.json")
with open(log_path, "a") as f:
    json.dump({
        "cwd": os.getcwd(),
        "PLUGIN_ROOT": os.environ.get("PLUGIN_ROOT", "<not set>"),
        "PLUGIN_DATA": os.environ.get("PLUGIN_DATA", "<not set>"),
        "ANTIGRAVITY_EXECUTABLE_DATA_DIR": os.environ.get("ANTIGRAVITY_EXECUTABLE_DATA_DIR", "<not set>"),
        "stdin_keys": list(stdin_data.keys()),
        "workspacePaths": stdin_data.get("workspacePaths", "<not present>"),
    }, f)
    f.write("\n")

# Allow the write
print(json.dumps({"decision": "allow"}))
PYEOF
chmod +x ~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe/scripts/spike_hook.py

# 6. Create a test skill
cat > ~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe/skills/spike-skill/SKILL.md << 'EOF'
---
name: spike-test-skill
description: "Test whether ${PLUGIN_ROOT} is substituted in skill text"
---

# Spike Test Skill

Plugin root is: ${PLUGIN_ROOT}
Plugin data is: ${PLUGIN_DATA}
EOF

# 7. Create a test rule
cat > ~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe/rules/spike-rule.md << 'EOF'
Run this command to prove the rule loaded: echo "SPIKE_RULE_LOADED"
Plugin root: ${PLUGIN_ROOT}
EOF

# 8. Create a sidecar
cat > ~/.gemini/config/plugins/spike-test-daemon/sidecars/test-repo/sidecar.json << 'EOF'
{
  "builtin": "schedule",
  "args": ["*/5 * * * *", "echo spike-tick"],
  "restart_policy": "always",
  "display_name": "Spike Test Sidecar",
  "agent_permissions": {
    "workspace_uris": ["file:///tmp/spike-test-repo"]
  },
  "env": {
    "SPIKE_ENV": "present"
  }
}
EOF

# 9. Create the consumer repo's plugins.json
mkdir -p /tmp/spike-test-repo/.agents
cat > /tmp/spike-test-repo/.agents/plugins.json << 'EOF'
{
  "plugins": [
    {"path": "~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe"}
  ]
}
EOF

# 10. Init a git repo (some hooks need this)
cd /tmp/spike-test-repo && git init && echo "test" > README.md && git add . && git commit -m "init"

echo "Setup complete. Clear spike_log.json before each test."
echo "Open /tmp/spike-test-repo in Antigravity to begin checks."
```

---

## Checks

Open `/tmp/spike-test-repo` as a workspace in Antigravity. Run each check and record the result.

### Check 1: Does a second sidecar appear without restart?

```bash
# Add a second sidecar while Antigravity is running
mkdir -p ~/.gemini/config/plugins/spike-test-daemon/sidecars/test-repo-2
cat > ~/.gemini/config/plugins/spike-test-daemon/sidecars/test-repo-2/sidecar.json << 'EOF'
{
  "builtin": "schedule",
  "args": ["*/5 * * * *", "echo spike-tick-2"],
  "restart_policy": "always",
  "display_name": "Spike Test Sidecar 2"
}
EOF
```
**Check:** Does it appear in the Automations dashboard without restarting Antigravity?
**Result:** ____

### Check 2: Does the sidecar with `agent_permissions` get its own approval switch?

**Check:** In the dashboard, does `Spike Test Sidecar` (the one with `workspace_uris`) have a per-sidecar approval toggle?
**Result:** ____

### Check 3: Are extra files in the sidecar dir ignored?

```bash
echo '{"extra": true}' > ~/.gemini/config/plugins/spike-test-daemon/sidecars/test-repo/config.json
```
**Check:** Does the sidecar still start normally? No errors in logs?
**Result:** ____

### Check 4–5: Does `agentapi new-conversation` inherit `agent_permissions`?

If the sidecar is approved and runs, does the conversation it starts:
- **Check 4:** Have write access to `/tmp/spike-test-repo`? → ____
- **Check 5:** Have access denied to a second repo (e.g. `/tmp/spike-test-repo-2`)? → ____

### Check 6: `${PLUGIN_ROOT}` and `PLUGIN_DATA` availability

**Check:** Look at `spike_log.json` — are `PLUGIN_ROOT` or `PLUGIN_DATA` set in the hook's environment?
**Result:** ____

### Check 7: Workspace plugin scoping

**Check:** Is `spike-probe` listed in the global plugin list? Or only visible when `/tmp/spike-test-repo` is open (via `plugins.json`)?
**Result:** ____

### Check 8: `workspacePaths` in hook stdin

**Check:** Look at `spike_log.json` — does the hook receive `workspacePaths` in its stdin JSON?
**Result:** ____

### Check 9: Do workspace-plugin hooks fire in sidecar-started conversations?

**This is the highest-priority check.**

If the sidecar starts a conversation via `agentapi`, does the `spike-test-hook` fire when that conversation uses `write_to_file`?

**Check:** Approve the sidecar, let it fire. Does `spike_log.json` show an entry from the sidecar's conversation?
**Result:** ____

### Check 10: Is hook cwd the directory containing hooks.json?

**Check:** Look at `spike_log.json` — what is `cwd`? Is it:
- (a) `~/.gemini/config/plugins/spike-test-daemon/workspace-plugin/spike-probe/` (the hooks.json dir)
- (b) `/tmp/spike-test-repo` (the workspace)
- (c) Something else

**Result:** ____

### Check 11: Is `${PLUGIN_ROOT}` substituted in skill and rule text?

Open `/tmp/spike-test-repo` in Antigravity. Ask: _"What does the spike-test-skill say?"_

**Check:** Does the agent see literal `${PLUGIN_ROOT}` or a substituted absolute path?
**Result:** ____

### Check 12: Does `plugins.json` expand `~`?

The entry uses `"path": "~/.gemini/..."`.

**Check:** Does the workspace plugin load correctly? If not, try with an absolute path.
Also check: does `plugins.json` point at `workspace-plugin/spike-probe/` (the plugin dir) or `workspace-plugin/` (the parent)?

**Result:** ____

### Check 13: Does `agy plugin enable` exist?

```bash
agy plugin enable spike-test-daemon 2>&1
agy plugin --help 2>&1
```

**Check:** Does the command exist? What does `--help` show?
**Result:** ____

### Check 14: Schedule builtin overlap and child survival

```bash
# Replace the sidecar with one that sleeps
cat > ~/.gemini/config/plugins/spike-test-daemon/sidecars/test-repo/sidecar.json << 'EOF'
{
  "builtin": "schedule",
  "args": ["* * * * *", "sleep 90 && echo done > /tmp/spike-child-survived"],
  "restart_policy": "always",
  "display_name": "Spike Overlap Test"
}
EOF
```

**Check 14a:** Does the schedule builtin start a second `sleep 90` while the first is still running, or wait/kill it?
**Check 14b:** If `tick.py` were to start `sleep 90` via `Popen(start_new_session=True)`, would the child survive after `tick.py` exits? Check if `/tmp/spike-child-survived` appears after 90s.

**Result 14a:** ____
**Result 14b:** ____

---

## Cleanup

```bash
rm -rf ~/.gemini/config/plugins/spike-test-daemon
rm -rf /tmp/spike-test-repo
rm -rf /tmp/spike-test-repo-2
```

---

## Reporting

Fill in the results table and send it back. The architecture choice depends on:

| Outcome | Impact |
|---------|--------|
| Check 9 = NO | **Blocker.** Dispatcher conversations run with no gates. Need a different hook delivery mechanism. |
| Check 10 ≠ (a) | Hook commands need absolute paths, not relative `scripts/...` |
| Check 11 = literal | Skills/rules must use absolute paths or a runtime resolver |
| Check 12 = no tilde | `plugins.json` entries must use absolute paths |
| Check 14a = kills | `tick_receiver` must `wait()` for all children before exiting |
