# repo-memory

**Repository-scoped, evidence-linked engineering memory for coding agents (Antigravity & Claude Code).**

> **Specification:** [`SPEC.md` (v1.0)](SPEC.md) · **Design (v2.2):** [`docs/design-v2-distillation-retrieval.md`](docs/design-v2-distillation-retrieval.md) · **Provenance & Hardening:** [`docs/addendum-a-provenance-and-hardening.md`](docs/addendum-a-provenance-and-hardening.md) · **Hooks Reference:** [`docs/antigravity-hooks-reference.md`](docs/antigravity-hooks-reference.md)

---

## Overview

`repo-memory` gives coding agents durable, repository-specific engineering knowledge that compounds across sessions without requiring the user or agent to manually maintain notes.

- **Zero manual cooperation (`G1` / `P1`):** Capture and delivery are 100% automatic via harness lifecycle hooks (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`) and a 1-minute scheduled background tick.
- **Push-first, persistent session delivery (`G3` / `P2`):** Startup and scope dossiers are pushed automatically on `PreInvocation` via persistent `userMessage` (`injectSteps`), surviving across all tool calls within a turn until context compaction (`CHECKPOINT`) or transcript rewind (`DetectRewind`), while 1-step transient alerts use `ephemeralMessage`.
- **Two-Stage 5-Minute Warm-Cache Distillation:**
  - **Stage 1 (In-Session Reflection & Free Artifact Harvest):** When an agent finishes a turn (`Stop` with `stopReason == "NO_TOOL_CALL"` and `executionNum == 0`) that passes the deterministic **Milestone Gate** ($\ge 1$ file edit, $\ge 1$ non-zero command fixed, or $\ge 5$ steps), the hook returns `{"decision": "continue"}` to harvest a structured reflection while 95%+ of the session transcript is hot in the agent's prompt cache—supplemented by zero-LLM harvesting of `CHECKPOINT` summaries and `<appDataDir>/brain/<session-id>/*.md` artifacts.
  - **Stage 2 (Warm-Cache Multi-Turn Consolidation):** Within 5 minutes, the background tick runs a multi-turn `--input-format stream-json` `agy -p --agent repo-memory-distiller` session (`X5`), passing candidates through the **Pre-Consolidation Corroboration Gate** (`git diff`, exit codes, anchors) before committing operations to `.memory/ops/YYYY-MM.jsonl`.
- **Three Cognitive Lifecycles & Deterministic Materialisation:**
  - **`semantic`** (`invariant`, `constraint`, `decision`, `convention`): Long-lived architectural truths with structural anchor verification.
  - **`procedural`** (`procedure`, `gotcha`, `ruled_out`): Validated by `PostToolUse` command exit codes (`success_count` / `failure_count`) and automatically graduated into native `.agents/skills/rm-<slug>/SKILL.md` skills (`A7`).
  - **`episodic`** (`recent_work`): Rolling window (max 5 per scope, 14-day TTL) summarizing recent session changes.
- **Day-1 Value via Doc Seeding & Async Freshness ("Dreaming"):** `repo-memory seed-docs` bootstraps dossiers from `README.md`, `CLAUDE.md`, `AGENTS.md`, `docs/**/*.md`, and `.agents/skills/**/SKILL.md` (`source: doc-seed`) and builds `<data-dir>/<repo-id>/index.db` on Day 1. During idle ticks, the daemon compares `source_sha` against `HEAD` to keep doc-seeded entries fresh.

---

## Architecture & Storage Layout

### Committed Repository State (`.memory/`)

```text
.memory/
├── config.yaml              # Five-tier scope definitions, caps, and seeding settings
├── ATLAS.md                 # Deterministically generated ~400-token index of all scopes
├── scopes/
│   ├── global.md            # Repository-wide invariants, build/test commands, conventions
│   └── <scope>.md           # Per-scope dossier (<= 1,000 tokens; {section?} omit-when-empty)
└── ops/
    └── YYYY-MM.jsonl        # Append-only monthly operation log (git merge=union)
```

### Private Machine State (`<data-dir>/<repo-id>/`)

```text
<data-dir>/<repo-id>/
├── index.db                 # SQLite FTS5 + BM25 index (via index.IndexPath), anchors, doc_seeds
├── jobs.sqlite              # Private tick job queue (harvest, consolidate, seed-docs, materialise)
├── sessions/
│   └── <session-id>.json    # Per-session delivery ledger (InjectedSet, GateAttempts, step watermarks)
├── episodes/
│   └── <yyyy>/<id>.jsonl.zst
└── spool/                   # Lock-free hook event spool files ingested by tick
```

---

## Two-Plugin Packaging Split

Antigravity spawns `sidecar.json` scheduled background daemons (`builtin: schedule`) from global plugins under `~/.gemini/config/plugins/<name>/`, while workspace hooks and MCP tools can be scoped per repository via `.agents/plugins.json` ([`docs/experiments/phase0-plugin-split.md`](docs/experiments/phase0-plugin-split.md)):

1. **Global Daemon Plugin (`~/.gemini/config/plugins/repo-memory-daemon/`):** Runs the 1-minute `repo-memory tick` sidecar with **zero global hooks** (zero overhead in non-enrolled workspaces) and bundles `workspace-plugin/repo-memory/`.
2. **Per-Workspace Plugin (`.agents/plugins.json`):** `repo-memory init` writes the resolved **absolute path** to `workspace-plugin/repo-memory` into `.agents/plugins.json`, activating `hooks.json` (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`), `repo-memory mcp`, and `repo-memory-distiller` only for enrolled repositories.

---

## CLI Reference

```bash
# Build and test
go build ./cmd/repo-memory
go test ./...

# Enrol a repository and bootstrap Day-1 dossiers + index.db
repo-memory init

# Seed from repository markdown docs & skills, or historical PA work orders / git commits
repo-memory seed-docs --repo <path>
repo-memory seed --repo <path> [--source pa-workorders|git-commits|doc-seed]

# Deterministically render .memory/scopes/*.md, .memory/ATLAS.md, and update index.db
repo-memory materialise --repo <path>
repo-memory lint --repo <path>
repo-memory reindex --repo <path>

# Inspect status, scopes, and coverage
repo-memory status
repo-memory scopes --repo <path>

# Emergency kill switch
repo-memory disable [--repo <path>]
repo-memory enable [--repo <path>]
```

---

## MCP Pull Tools (`repo-memory mcp`)

Spawned per session over `stdio` and backed by `<data-dir>/<repo-id>/index.db`:

| Tool | Purpose |
|---|---|
| `memory_brief` | Return the full dossier for a scope (resolved through the 5-tier scope hierarchy and aliases) |
| `memory_search` | FTS5 + BM25 search across all entries, body bullets, anchors, and error patterns |
| `memory_history` | Chronological op-log history (`add`, `update`, `supersede`, `retire`, `verify`) for a scope or entry ID |
| `memory_note` | Optional mid-turn override to record a candidate observation immediately (sets `MemoryNoteCalledInTurn = true`) |
| `memory_feedback` | Flag an injected entry as helpful, stale, or inaccurate |
| `memory_stats` | Repository memory coverage, dossier token utilization, and procedural validation counts |

---

## Documentation Index

- [`SPEC.md`](SPEC.md) — Canonical `v1.0` System Specification (`v2.2` Architecture)
- [`docs/design-v2-distillation-retrieval.md`](docs/design-v2-distillation-retrieval.md) — v2 Distillation, Storage & Retrieval Architecture (Revision 4, `v2.2`)
- [`docs/addendum-a-provenance-and-hardening.md`](docs/addendum-a-provenance-and-hardening.md) — Provenance, Seeding, Validation & Hardening (Revision 3)
- [`docs/antigravity-hooks-reference.md`](docs/antigravity-hooks-reference.md) — Empirical Antigravity Hooks Reference (`injectSteps`, `userMessage` vs `ephemeralMessage`, subagent ordering, two-plugin split)
- [`docs/experiments/SPEC-CHANGES.md`](docs/experiments/SPEC-CHANGES.md) — Experiment Spike (`E01`–`E12`) Specification Reconciliation Ledger
- [`docs/evaluations/pa-memory-retrieval-effectiveness.md`](docs/evaluations/pa-memory-retrieval-effectiveness.md) — 231-Session Empirical Study of Pull vs Push Memory Retrieval
