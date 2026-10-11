# Chief Architect Bridge Prompt — repo-memory v2.1

> **Handover from:** Session `052e1c3d` (2026-10-06)
> **Assume:** PR #9 (PR-07-v2-storage) has been merged to `main`.

---

## Your Role

You are the **Chief Architect** for the repo-memory programme. Read the PA skill before proceeding:
```
view_file: /Users/mcneillm/.gemini/config/skills/principal-architect/SKILL.md
```

Then read these critical reference files:
```
view_file: /Users/mcneillm/.gemini/config/skills/principal-architect/references/CHIEF_ARCHITECT.md
view_file: /Users/mcneillm/.gemini/config/skills/principal-architect/references/harnesses/antigravity.md
view_file: /Users/mcneillm/.gemini/config/skills/principal-architect/references/workorder-template.md
```

---

## Programme Status

**Programme:** repo-memory v2.1 — **11/19 PRs complete.**
**Specification:** `SPEC.md` v0.10 + `docs/design-v2-distillation-retrieval.md` rev 3
**Plan:** `.agents/skills/principal-architect-workspace/programme-repo-memory/PROGRAM-PLAN.md`

### Dependency Graph
```
Phase 1: Infrastructure            ✅ Complete (PR-01 through PR-04)
Phase 2: v1 Pipeline               ✅ Complete (PR-05a, PR-05b)
Phase 3: v2 Distillation & Retrieval
  PR-05-v2  Foundation             ✅ Complete
  PR-06-v2  Prototype Spike        ✅ Complete (X-gate PASSED)
  PR-06b-v2 Plugin Split           ✅ Complete
  PR-07-v2  Storage                ✅ Complete (assume merged)
  ────────────────────────────────────────────
  PR-08-v2  Delivery               ☐ NEXT ← you are here
  PR-09-v2  X6-lite Kill Gate      ☐ Critical path
  ═══════ Early kill gate ═══════
  PR-10-v2  Distillation           ☐ After kill gate passes
  PR-11-v2  Consolidation          ☐
  PR-12-v2  Pilot & X6             ☐
  ═══════ Confirmatory gate ═══════
  PR-13-v2  Learning Loop          ☐
  PR-14-v2  PA Integration         ☐
  PR-15-v2  Backfill               ☐
```

### What Exists After PR-07

The v2 storage layer is fully built. The codebase now has:

**New packages (created in PR-07):**
- `internal/core/ops/` — Append-only op log. 6 op types (`add`, `amend`, `supersede`, `retire`, `move`, `split`). Month-sharded JSON under `.memory/ops/<yyyy-mm>/<ulid>.json`. Writer, Reader, Iterator with `IterateBefore` for time-cut.
- `internal/core/dossiers/` — Entry schema (8 types: `decision`, `invariant`, `ruled-out`, `failure-signature`, `gotcha`, `procedure`, `open-issue`, `external-behaviour`). Projector from op log (with `ProjectBefore` for time-cut). Deterministic Markdown renderer. Atlas. History. Cap linting. Hand-edit detection.
- `internal/core/config/scopes.go` — Declared scopes from `config.yaml`, derived module scopes from file anchors, alias resolution from split ops. `ScopeResolver` combining all three.
- `internal/core/seed/` — PA completion-report parser (backtick-token extraction), report→ops converter (deterministic content-addressed IDs `seed-<hash>`), workorder scanner. Git commit timestamps on ops. Idempotent. CLI subcommand `repo-memory seed`.
- `internal/core/index/` — v2 SQLite tables (`entries`, `entries_fts` FTS5, `entry_anchors`, `error_patterns`, `aliases`, `entry_history`). CRUD. `RebuildFromOpLog` (single-transaction, projection outside lock). Kept: `episodes_fts`, `entities`.

**Removed in PR-07:**
- `internal/core/pipeline/{triage,extract,reconcile,verify}` — v1 pipeline stages (~2500 lines)
- `internal/core/claims/` — v1 claims + store
- `internal/core/events/` — v1 events store
- `internal/core/views/` — v1 views generator
- `cmd/repo-memory/cmd_views.go`

**Retained (still in codebase):**
- `internal/core/pipeline/publish/` — local-publish worktree (reused by v2)
- `internal/core/pipeline/compact/` — compact stage
- `internal/core/tick/` — executor shell, job chaining, 429 handling (gutted: v1 stage calls replaced with log message)
- `internal/harness/` — Antigravity harness
- `internal/mcpserver/` — MCP server

**Storage layout after PR-07:**
```
.memory/ (in git, shared)
├── ops/<yyyy-mm>/<ulid>.json     ← Source of truth
├── dossiers/<scope>.md            ← Rendered from ops
├── dossiers/<scope>.history.md    ← Retired entries
├── ATLAS.md                       ← Directory of dossiers
└── config.yaml                    ← Declared scopes

<data-dir>/<repo-id>/ (private)
├── index.sqlite                   ← Rebuilt from ops
└── evidence/                      ← Episodes, notes, artifacts
```

**Seeding result:** 174 entries from 49 PA workorders, with module-based scopes and git commit timestamps.

---

## What PR-08 Must Build

**PR-08-v2-delivery: Channels, Moments & Injection Ledger**
Read the full plan: `PROGRAM-PLAN.md` lines 233-261.

6 work orders:

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-ledger-engine` | Ledger (entry IDs, channel, step, content hash, tags), ranking engine (anchor match → relevance → verified → evidence), budget enforcement, exposure list | architect |
| 2 | `wo2-persistent-pushes` | `PreInvocation` as `systemMessage`/`userMessage`. Task start (≤500 tokens), first entry into dossier (≤150), first touch of anchor (≤150). Branch diff precomputed. | architect |
| 3 | `wo3-ephemeral-and-errors` | About-to-edit pointer (≤50 tokens). Error matching on `PostToolUse` → `ephemeralMessage` (≤200 tokens). | standard |
| 4 | `wo4-gates` | `PreToolUse` deny-once for `gate: edit` entries. `force_ask` for invariants. X9 contract. | standard |
| 5 | `wo5-compaction-reinjection` | Detect compaction/rewind from transcript. Re-inject consolidated `systemMessage` (≤1k). Re-arm dedup. **X11 experiment.** | architect |
| 6 | `wo6-pull-tools` | MCP tools: `memory.brief`, `memory.why`, `memory.history`, `memory.search`, `memory.note`, `memory.stats`. Pulls recorded in exposure list. | standard |

**Exit criteria:**
- X5 confirmed in production code
- X11: compaction detection ≥95%, re-injection works
- All moments deliver correctly, budgets respected, ranking correct
- Gates deny once, then allow retry
- Hook latency: p95 ≤60ms when nothing injected, ≤150ms when injecting
- All MCP tools work; exposure lists populated

---

## Critical User Knowledge (MUST preserve)

### Hook injection format
- `ephemeralMessage`: plain string, single-invocation scope
- `userMessage`: plain string, persistent until compaction
- `systemMessage`: **MUST be a nested object** `{"systemMessage": {"systemMessage": "...", "metadata": {}}}` — bare string fails silently
- All wrapped in `injectSteps` array: `{"injectSteps": [...]}`
- Works in `SessionStart`, `PreInvocation`, AND `PostInvocation`
- `toolCall` in `injectSteps` is documented but empirically a no-op

### Hook discovery paths
- **Workspace:** `<workspace>/.agents/hooks.json` (or `.agent/hooks.json`)
- **Global:** `~/.gemini/config/hooks.json`
- **Plugin:** `<plugin-root>/hooks.json` — NOT `<plugin-root>/hooks/hooks.json`

### Agent frontmatter
- `name` and `description` REQUIRED — silent drop without them
- `model:` only accepts tier names: `inherit`, `flash_lite`, `flash`, `pro`
- `mainAgent` (default true) / `subagent` (default true) control visibility

### Architecture
- "fundamentally disagree - I think we need to enroll workspaces into memory, not assume it applies everywhere"
- Split plugin: daemon (`repo-memory-daemon`) + workspace bundle loaded per-repo via `.agents/plugins.json`

### Compaction
- Assume injected messages do NOT survive compaction — re-injection required
- Scan backwards to compaction event and reinject if they have gone missing
- `PreInvocation` fires BEFORE subagent prompt delivery (FEATURE, not bug)

### PostInvocation
- Has `terminationBehavior`: `"force_continue"` or `"terminate"`
- **PreToolUse** has `overwrite`: shallow-merges top-level keys into `toolCall.args`
- Stop continuation cap: 10 (introduced v1.1.9)

### Pre-existing test failures (NOT caused by our changes)
- `TestFastPath_NonEnrolledLatency` and `TestProbe_Disabled_FastPath` — doubled output buffer issue
- `TestSessionAttributor_Integration` — hardcoded date

---

## Worker Spawn Pattern

From the harness adapter (`/Users/mcneillm/.gemini/config/skills/principal-architect/references/harnesses/antigravity.md`):
- `TypeName: "self"`, `Workspace: "inherit"` (NOT branch — causes permission issues)
- Model mapping: `mechanical→flash_lite`, `standard→flash`, `architect→pro`, `interactive→bridge prompt`
- Workers commit on the current branch directly
- After worker commits: push branch, create next branch from tip, commit WO setup, push, spawn next worker

---

## PA Workspace Structure

```
.agents/skills/principal-architect-workspace/
├── memory/MEMORY.md                    ← Programme state
├── programme-repo-memory/
│   ├── PROGRAM-PLAN.md                 ← Master plan (read this)
│   └── PROGRAM.md                      ← Programme status
├── pr-07-v2-storage/
│   ├── PROJECT.md                      ← PR-07 project (now complete)
│   └── wo{1..6}-*/WORKORDER.md         ← All 6 WOs with completion reports
├── pr-06-v2-spike/PROJECT.md           ← PR-06 (complete)
├── pr-06b-v2-plugin-split/PROJECT.md   ← PR-06b (complete)
└── ... (earlier PRs)
```

---

## Immediate Actions for New Session

1. **Update MEMORY.md**: Graduate PR-07 to "Recently Completed"
2. **Update PROGRAM.md**: Mark PR-07 complete, 11/19
3. **Create PR-08 PROJECT.md**: 6 WOs from the programme plan
4. **Create WO1 branch and WORKORDER.md**: `pr-08-v2-delivery/wo1-ledger-engine`
5. **Spawn WO1 worker**: `architect` complexity → `pro` model

### Key Design Files to Read for PR-08
- `docs/design-v2-distillation-retrieval.md` §8.1-8.6 (delivery design)
- `docs/experiments/spike-summary.md` (X5 channel findings, X9 hook contracts)
- `internal/harness/antigravity/` (where hooks are handled — this is where delivery integrates)
- `internal/core/ops/` and `internal/core/dossiers/` (what delivery reads from)
- `internal/core/index/` (anchor lookups, FTS for pull tools)

### Risks for PR-08
- **Hook latency budget is tight** (p95 ≤60ms no-inject, ≤150ms inject) — precompute in tick, not in hook
- **Compaction re-injection (X11)** is the least-validated mechanism — needs careful experiment
- **MCP server integration** — existing `internal/mcpserver/` needs new tool handlers
- **Branch diff precomputation** — tick must prepare this before `PreInvocation` runs
