# Plugin Architecture Migration — Review Assessment

## Verdict: The reviewer is right on all 7 blockers. Revise the plan before WO0.

The review caught fundamental design flaws at exactly the right time — before any code was written. Every blocker is real. Here's my assessment of each:

---

## Blocker 1: Copy-not-move creates dual source of truth

**Agree — critical.**

The reviewer is correct: if WO1 *copies* `scripts/`, `skills/`, `rules/`, `agents/` into `daemon/workspace-plugin/bug-fixer/` and the originals survive, there are two copies. Tests only cover the originals, so WO2's changes to the daemon copy get zero test coverage.

**Resolution:**
- WO1 must use `git mv` to move files, not copy
- Test import paths must be updated in the *same* WO (not deferred to WO6)
- WO3's edit to `scripts/bugfixer_api.py` at the old location confirms the plan is confused about which copy is canonical

---

## Blocker 2: Config loader rewrite is a regression

**Agree — this is the most dangerous one.**

The reviewer identified **7 separate regressions** in the proposed `load_config` rewrite:

| Regression | Impact |
|-----------|--------|
| Wrong function targeted | Breaks `github_url_gate.py` import |
| Drops `normalise_config()` | Gate B stops seeing leak patterns |
| Drops `_loaded_from` | `SCANNER_PROTECTED_FILES` not rebuilt — config unprotected |
| Bad paths don't fail closed | Silent fallthrough instead of error |
| Restores `docs/design_specification.md` | Re-introduces bug #43 |
| `startswith` prefix match | `/x/repo-other` matches `/x/repo` |
| Hardcoded `~/.gemini/config/plugins/bug-fixer-daemon` | Breaks tests and symlinked installs |

**Resolution:**
- The rewrite must be an *extension* of the existing `load_config`, not a replacement
- Add the new candidate paths (workspace-plugin install dir, sidecar scan) to the existing candidate list
- All existing invariants must be preserved: `normalise_config()`, `_loaded_from`, fail-closed
- Use `Path.is_relative_to()` not `startswith()`
- Derive install path from `__file__`, never hardcode

---

## Blocker 3: Gate A doesn't protect new trust anchors

**Agree — this is a security gap.**

After migration, Gate A only knows about `.agents/plugins/bug-fixer-loop-workspace/` paths. An agent could:
- Edit `.agents/plugins.json` → disable all hooks
- Edit `sidecars/<repo>/config.json` → weaken patterns
- Edit `sidecars/<repo>/sidecar.json` → widen workspace URIs
- Poison `workspace-cache.json` → redirect config resolution
- Edit hook scripts in install directory

**Resolution:**
- `_derive_workspace_paths()` must be extended to include ALL trust anchors
- Drop `workspace-cache.json` entirely (the reviewer is right — scanning a few directories costs nothing)
- Add `.agents/plugins.json` to the static protected set
- Add `sidecars/*/config.json` and `sidecars/*/sidecar.json` via glob
- The integrity monitor must also cover these paths

---

## Blocker 4: Integrity monitor breaks post-install

**Agree — and the scope is wider than stated.**

The monitor resolves `__file__/../.agents/plugins/bug-fixer-loop-workspace`. Once installed to `~/.gemini/config/plugins/bug-fixer-daemon/`, that directory isn't a git repo. `git checkout` would fail silently.

Same issue in `safe_ingest.py`, `safe_post.py`, `injection_scanner.py`. The plan only covers `bugfixer_api.py`.

**Resolution:**
- The integrity monitor must detect whether it's in a git repo and use a different revert strategy (cached known-good copies, or checksums stored at install time)
- ALL scripts with the legacy `__file__` fallback must be updated in the same WO
- `safe_ingest.py` must not raise on missing config in interactive sessions

---

## Blocker 5: Runtime state has no migration path

**Agree — data loss.**

WO5 deletes `.agents/plugins/bug-fixer-loop-workspace/` which contains:
- `claims/` and `dispatches/` (in-flight work)
- Cooldown and failure counters
- `pr_completions.jsonl`
- `injection_cache.json`
- `issue_dispatch_failures.json`
- `.integrity_snapshot.json`

5 skills also hardcode `STATE_DIR` to this path.

**Resolution:**
- Define `sidecars/<repo>/state/` as the new state location
- WO must include a migration step: move state files before deleting the old directory
- Skills must be updated to use the new state path
- Enroll script must create the state directory

---

## Blocker 6: Old and new loops run simultaneously

**Agree — this causes duplicate claims and PRs.**

Nothing removes the existing standalone sidecars in `~/.gemini/config/sidecars/*--<repo>/`. After WO4, both old and new sidecars fire.

**Resolution:** The cutover must be sequenced:
1. Install and enrol (new sidecars created)
2. Confirm new hooks fire (smoke test)
3. **Then** remove old sidecars and old hooks
4. `enrol.sh` should check for and warn about existing old-style sidecars

---

## Blocker 7: Reinstall wipes per-repo config

**Agree — destructive upgrade.**

`install.sh` copies `daemon/` over the install directory. If `sidecars/` (per-repo config + state) lives inside it, a reinstall destroys everything.

**Resolution options (pick one):**
- **Option A:** Keep per-repo data outside the plugin directory (e.g. `~/.gemini/config/bug-fixer/sidecars/`)
- **Option B:** Define rsync-with-exclude upgrade behaviour (exclude `sidecars/`)
- `uninstall.sh` must warn before deleting per-repo data
- Add an `upgrade.sh` that preserves state

> [!IMPORTANT]
> **My recommendation: Option A.** Separating per-repo state from the plugin install directory is cleaner. The plugin directory becomes stateless and can be blown away and reinstalled at any time. Per-repo state is user data, not plugin code.

---

## Non-blocking but important

| Issue | Assessment |
|-------|-----------|
| **Dispatchers don't find config** | Real — `BUG_FIXER_CONFIG` must be set in sidecar env at enrol time |
| **Internal cron underspecified** | Real — needs persistent last-run times, cron matcher, per-loop locking, ~200+ lines not ~45 |
| **Skill/rule paths broken** | Real — `scripts/safe_ingest.py` references need `${PLUGIN_DIR}` substitution |
| **WO0 spike additions** | All valid — hooks firing in sidecar conversations is highest priority |
| **Factual errors** | All confirmed: hooks.json isn't empty, directory name is wrong, hook list incomplete |
| **WO6 cleanup** | Good call — clean up `pr_52.json`, `update_pr*.sh`, `test-repo/` etc. |

---

## Recommended Next Steps

1. **Revise the plan** to address all 7 blockers
2. **Add WO0 spike checks** from the review (especially: do workspace-plugin hooks fire in sidecar conversations?)
3. **Run WO0** before WO1 starts — the spike answers will influence the entire design
4. **Choose per-repo state location** (I recommend outside plugin dir)
5. **Define the cutover sequence** explicitly in the plan

The reviewer offered to make the edits to the plan file or post as a review comment. Given the scope of changes, **revising the plan file** is the right call — it keeps everything in one place.
