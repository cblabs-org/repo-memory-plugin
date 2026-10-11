# Impact Assessment: Split Plugin Architecture Pivot

> **Date:** 2026-10-05
> **Context:** X1 and X9 experiments surfaced that the current single global plugin architecture is fundamentally wrong — hooks fire in every workspace, MCP schemas load into every context window, and the enablement model is fragile (`agent_permissions` in sidecar.json silently disabled the whole plugin).

## What Changes

| Component | Current | After Split |
|---|---|---|
| **Plugin directory** | `~/.gemini/config/plugins/repo-memory/` | `~/.gemini/config/plugins/repo-memory-daemon/` |
| **Global scope** | Everything (hooks, MCP, rules, skills, sidecar) | Sidecar only (tick shim) |
| **Per-workspace** | Nothing — hooks fast-path out | Everything (hooks, MCP, rules, skills via `.agents/plugins.json`) |
| **Non-enrolled overhead** | Subprocess per hook event + MCP/rules in context | Zero |
| **Enrollment** | Registry only (machine-local SQLite) | Registry + `.agents/plugins.json` (gitignored) |
| **Install command** | Writes single plugin dir | Writes daemon dir + nested workspace-plugin |
| **Init command** | Registry entry + `.memory/` layout | + writes `.agents/plugins.json` |
| **Leave command** | Registry removal | + removes `.agents/plugins.json` entry |

## Files Touched

### Must change
| File | Change |
|---|---|
| `cmd/repo-memory/cmd_install.go` | New directory layout: daemon (plugin.json + sidecars/) and workspace-plugin/ (hooks, MCP, rules, skills) |
| `cmd/repo-memory/cmd_init.go` | Write `.agents/plugins.json` pointing to workspace bundle |
| `cmd/repo-memory/cmd_leave.go` | Remove `.agents/plugins.json` entry |
| `packaging/antigravity/templates.go` | New templates for daemon plugin.json and workspace plugin.json |
| `packaging/antigravity/hooks.json` | Moves to workspace-plugin — no content change |
| `internal/core/config/layout.go` | `.agents/plugins.json` path constant |
| `internal/core/config/marker.go` | `IsEnrolledFast` — may simplify (hooks only fire in enrolled repos anyway) |

### May change
| File | Change |
|---|---|
| `cmd/repo-memory/cmd_hook.go` | Fast-path becomes defense-in-depth, not primary gate |
| `cmd/repo-memory/cmd_doctor.go` | Check workspace plugin reference is valid |
| `.gitignore` or `.agents/.gitignore` | Add `plugins.json` |
| All test fixtures referencing plugin paths | Update paths |

### Unchanged
| Component | Why |
|---|---|
| `internal/core/tick/` | Tick binary is the sidecar — still runs globally, iterates registry |
| `internal/harness/antigravity/hooks.go` | Hook handlers don't change — only where they're installed |
| `internal/core/evidence/` | Unrelated |
| `internal/core/telemetry/` | Unrelated |
| Pipeline, jobs, episodes, entities, claims | All unrelated |

## Impact on Programme Plan

### PR-06-v2-spike (current — 2/5 WOs complete)

| WO | Impact | Action |
|---|---|---|
| WO1 (X1) ☑ | None — complete, findings drove this pivot | No change |
| WO2 (X9) ☑ | None — complete, contract tests still valid | No change |
| **WO3 (X5 channels)** | **Low** — tests `systemMessage`/`userMessage` persistence, pure platform mechanics. Independent of plugin architecture. | Can proceed NOW, no changes needed |
| **WO4 (X8 lean agent)** | **HIGH** — tests agent install location. Split architecture adds a third option: agent inside workspace-plugin bundle. WO4 scope needs updating. | Must include split architecture validation (Phase 0 checks #7, #8). Agent discovery tests should cover all three locations. |
| **WO5 (X3 quota + reconciliation)** | **Medium** — summarises all experiments and proposes SPEC edits. The split architecture is a SPEC-level change. | WO5 scope expands: must include the architecture in SPEC v1.0 edits |

### PR-07-v2-storage

| WO | Impact |
|---|---|
| wo1-op-log | None |
| wo2-dossier-schema-renderer | None |
| wo3-scopes-aliases | None |
| wo4-index-migration | None |
| **wo5-v1-removal** | **Low** — removes v1 stages. The install/enroll mechanism must be stable before this, but the split doesn't change v1 removal scope. |
| wo6-seeding | None |

### PR-08-v2-delivery

| WO | Impact |
|---|---|
| wo1-ledger-engine | None |
| **wo2-persistent-pushes** | **Medium** — `PreInvocation` as `systemMessage`. Must work with workspace-plugin hooks. The mechanism is the same, but the install location changes. |
| wo3-ephemeral-and-errors | Same as wo2 |
| wo4-gates | Same as wo2 |
| wo5-compaction-reinjection | Same as wo2 |
| wo6-pull-tools | **Low** — MCP tools move to workspace-plugin. No code change, just location. |

### PR-09 through PR-15

Minimal impact. By PR-09, the architecture is stable. Only PR-12 (pilot on esp32) needs the enrollment flow for a second repo — that's covered.

## Options

### Option A: Fold into PR-06 (recommended)

Add the split implementation as **WO3b** (between WO3 and WO4) in PR-06. This is the spike PR — it validates platform mechanics and produces SPEC edits. The architecture change IS a platform mechanic.

**Sequence:**
1. WO3 (X5 channels) — proceed now, independent
2. **WO3b (split architecture)** — implement the split, validate Phase 0 checks, migrate live install
3. WO4 (X8 lean agent) — runs on the new architecture, tests agent discovery in workspace-plugin
4. WO5 (X3 quota + reconciliation) — includes architecture in SPEC edits

**Pros:** Natural fit. The spike is supposed to produce architectural findings. No new PR in the plan. WO4 benefits from running on the real architecture.
**Cons:** PR-06 grows from 5 to 6 WOs. The WO3b implementation is `standard` complexity (~half a day).

### Option B: New PR-06b between PR-06 and PR-07

Dedicated PR for the architecture pivot. PR-06 completes with WO3-5 using the current architecture. PR-06b implements the split.

**Pros:** Clean separation. PR-06's X-gate criteria don't change.
**Cons:** Adds a 10th PR to the v2 plan. WO4 runs on the wrong architecture and may produce invalid findings. Extra merge and review overhead.

### Option C: Absorb into PR-07

PR-07 already removes v1 stages and restructures storage. Add the split as WO0 before wo1-op-log.

**Pros:** Groups restructuring work together.
**Cons:** PR-07 is already 6 WOs and `architect`-complexity. The split should be validated before the X-gate, not after. PR-08's delivery hooks depend on the architecture being settled.

## Recommendation

**Option A** — fold into PR-06 as WO3b. The spike PR is exactly where architectural findings belong. The sequence is clean: WO3 (channels, independent) → WO3b (split) → WO4 (lean agent on new architecture) → WO5 (reconciliation + SPEC edits including architecture).

Estimated scope of WO3b:
- `cmd_install.go` restructure (~100 lines)
- `cmd_init.go` plugins.json write (~30 lines)
- `cmd_leave.go` plugins.json removal (~20 lines)
- Template updates (~50 lines)
- Phase 0 validation checks #7, #8 (~manual test, documented in report)
- Migration of live install (~scripted, documented)
- Test fixture updates (~50 lines)
- `.agents/plugins.json` gitignore + doctor check

**Complexity:** `standard`. **Model:** `flash`. **Estimated:** 3-4 WOs of effort compressed into 1 since the architecture doc is already written.
