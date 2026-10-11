# PR #291 Review — chore: migrate bug-fixer from submodule to daemon plugin

**Author:** matthewmcneill · **State:** OPEN · **Commits:** 2
**Companion:** matthewmcneill/bug-fixer-loop-plugin#56

---

## Summary

This PR migrates the bug-fixer from a **git submodule checked into the repo** to the **daemon-installed workspace plugin** at `~/.gemini/config/plugins/bug-fixer-daemon/workspace-plugin/bug-fixer`. The key changes:

| Change | File(s) | Lines |
|--------|---------|-------|
| Remove submodule entry | `.gitmodules` | −3 |
| Delete submodule pointer | `.agents/plugins/bug-fixer-loop-plugin` | −1 (160000 mode) |
| Delete workspace config | `.agents/plugins/bug-fixer-loop-workspace/config.json` | −51 |
| Delete workspace plugin.json | `.agents/plugins/bug-fixer-loop-workspace/plugin.json` | −6 |
| Delete workspace .gitignore | `.agents/plugins/bug-fixer-loop-workspace/.gitignore` | −6 |
| Remove old hooks from hooks.json | `.agents/hooks.json` | −42 |
| Add new plugins.json | `.agents/plugins.json` | +7 |

**Net:** −109 / +7 — a clean removal.

---

## Verdict: ✅ Looks good — with caveats below

The migration is structurally sound. The old submodule and its workspace artefacts are cleanly removed, and the new `plugins.json` correctly points at the daemon's workspace plugin. The second commit rightly fixes the path to include the `/bug-fixer` suffix so it resolves to the directory containing `plugin.json`.

The daemon plugin at the target path already provides equivalent hooks (`bug-fixer-file-gate`, `bug-fixer-command-gate`, `bug-fixer-url-gate`) via its own [hooks.json](file:///Users/mcneillm/.gemini/config/plugins/bug-fixer-daemon/hooks.json), plus additional integrity monitoring (`PostToolUse` integrity monitor, `PreToolUse` integrity snapshot). So removing the old absolute-path hooks from the repo's `hooks.json` is correct — they'll be loaded from the daemon plugin instead.

---

## Issues & Observations

### 1. ⚠️ Stale `github-reads.md` rule (low risk, worth fixing)

The repo's [`.agents/rules/github-reads.md`](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/.agents/rules/github-reads.md) still says:

```bash
python3 scripts/safe_ingest.py issue-view <number>
```

But `safe_ingest.py` doesn't exist at `scripts/` — it never did in this repo. It lives in the daemon plugin at `~/.gemini/config/plugins/bug-fixer-daemon/workspace-plugin/bug-fixer/scripts/safe_ingest.py`. The daemon plugin ships its [own updated `github-reads.md`](file:///Users/mcneillm/.gemini/config/plugins/bug-fixer-daemon/workspace-plugin/bug-fixer/rules/github-reads.md) which tells agents to use the path from the gate's deny message.

**Impact:** In practice this is **low risk** because:
- The `bug-fixer-command-gate` hook blocks raw `gh` commands and the deny message includes the correct full path
- Interactive sessions (like this one) can recover by following the deny message

**Recommendation:** Either update the repo's `github-reads.md` to match the daemon's version, or delete it entirely (since the daemon plugin now ships the rule). This could be a follow-up.

### 2. ℹ️ `config.json` migration — where did it go?

The old workspace `config.json` (51 lines with `target_repo`, `maintainer`, cron schedules, model choices, etc.) is deleted. The daemon presumably reads this config from `~/.gemini/config/plugins/bug-fixer-daemon/sidecars/matthewmcneill--esp32-app-foundation/config.json` instead. This is fine — just noting the config is no longer version-controlled in the repo.

### 3. ℹ️ `bug-fixer-AGENTS.md` rule — no changes needed

The bug-fixer safety rules reference `safe_ingest.py` generically (no path) and reference "this plugin" abstractly. These still work correctly with the daemon plugin providing the same contract. No changes needed.

### 4. ✅ Submodule removal is complete

- `.gitmodules` entry removed ✓
- Submodule directory (160000 mode) removed ✓
- No orphaned `.git/modules/` entry concerns at the repo level (that's a local git state issue, not committed)

### 5. ✅ Hooks.json — correctly preserves project hooks

The three project-specific hooks (`flash-tag-gate`, `pio-cli-gate`, `pio-reset-gate`) are preserved. The four bug-fixer hooks with absolute paths are removed. The daemon's own `hooks.json` provides equivalent coverage via relative paths within the plugin directory.

---

## TL;DR

Clean migration. The structural changes are correct and complete. The one actionable gap is the stale `github-reads.md` rule in `.agents/rules/` — it references a non-existent local `scripts/safe_ingest.py`. The daemon plugin ships its own updated version of this rule, so agents enrolled via the plugin get the right instructions. Consider updating or removing the repo copy as a fast follow.
