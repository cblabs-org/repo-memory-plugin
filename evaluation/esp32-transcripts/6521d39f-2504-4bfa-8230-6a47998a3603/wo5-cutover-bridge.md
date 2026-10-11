# WO5 — Ordered Cutover (Interactive)

> [!IMPORTANT]
> This work order requires your direct involvement. It touches live sidecars, external repos, and the Antigravity IDE.

## Prerequisites
- WO1-WO4 committed and verified ✅
- WO6 (cleanup) committed and verified ✅

## Cutover Sequence (per repo)

### Step 1: Remove old standalone sidecars
```bash
# List current sidecars for this repo
ls ~/.gemini/config/sidecars/bug-fixer-*--<repo-slug>/

# Remove them
rm -rf ~/.gemini/config/sidecars/bug-fixer-*--<repo-slug>/

# Remove their entries from ~/.gemini/config/config.json
# (edit manually or use jq to remove the sidecar keys)
```

### Step 2: Install the daemon plugin
```bash
cd /Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin
./scripts/install.sh
```

### Step 3: Enroll + migrate state
```bash
# For bug-fixer-loop-plugin itself:
./scripts/enroll.sh -r matthewmcneill/bug-fixer-loop-plugin \
  -d /Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin \
  --migrate-state

# For esp32-app-foundation:
./scripts/enroll.sh -r matthewmcneill/esp32-app-foundation \
  -d /Users/mcneillm/Documents/Projects/esp32-app-foundation \
  --migrate-state
```

### Step 4: Verify new hooks fire (**CRITICAL — Check 9**)
1. Open an enrolled workspace in Antigravity
2. Confirm hooks load from `plugins.json` → workspace plugin
3. Test: write to a protected file → should be denied
4. If hooks DON'T fire, this is an architecture blocker — see fallback plan below

### Step 5: Remove injected hooks from consumer repos

**esp32-app-foundation** `.agents/hooks.json`:
- Remove all 4 `bug-fixer-*` entries
- Keep `flash-tag-gate`, `pio-cli-gate`, `pio-reset-gate`

**bug-fixer-loop-plugin** `.agents/hooks.json.bak`:
- Delete entirely (it's gitignored)

### Step 6: Remove git submodule from esp32-app-foundation
```bash
cd /Users/mcneillm/Documents/Projects/esp32-app-foundation
git submodule deinit .agents/plugins/bug-fixer-loop-plugin
git rm .agents/plugins/bug-fixer-loop-plugin
```

### Step 7: Remove old workspace dirs
```bash
rm -rf <repo>/.agents/plugins/bug-fixer-loop-workspace/
```

### Step 8: Approve new sidecars in Automations dashboard

---

## Fallback Plan (if Check 9 fails)

If workspace-plugin hooks DON'T fire in sidecar conversations:
1. Keep the `enroll.sh` `.agents/hooks.json` injection as the primary mechanism
2. The workspace plugin still provides skills/rules/agents
3. Hooks fire from the injected `.agents/hooks.json` as before
4. This is a degraded but functional architecture — the hooks just aren't loaded via `plugins.json`

## After Cutover
- Restore `.agents/hooks.json` from `.agents/hooks.json.bak` (with updated paths if needed)
- Merge the PR branch
