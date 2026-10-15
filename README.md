# repo-memory

**Repository-scoped, evidence-linked engineering memory for coding agents (Antigravity & Claude Code).**

> **Specification:** [`SPEC.md` (v1.0)](SPEC.md) · **Design (v2.2):** [`docs/design-v2-distillation-retrieval.md`](docs/design-v2-distillation-retrieval.md) · **Provenance & Hardening:** [`docs/addendum-a-provenance-and-hardening.md`](docs/addendum-a-provenance-and-hardening.md) · **Hooks Reference:** [`docs/antigravity-hooks-reference.md`](docs/antigravity-hooks-reference.md)

---

## Overview

`repo-memory` gives coding agents durable, repository-specific engineering knowledge that compounds across sessions without requiring the user or agent to manually maintain notes.

- **Zero manual cooperation (`G1` / `P1`):** Capture and delivery are 100% automatic via harness lifecycle hooks (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`) and a 1-minute scheduled background tick.
- **Push-first, persistent session delivery (`G3` / `P2` / `P14`):** Task-start briefs, first-entry module invariants, first-touch file constraints, and post-compaction re-injections are pushed automatically on `PreInvocation` via persistent `userMessage` (`injectSteps`), surviving across all tool calls within a turn until context compaction (`CHECKPOINT`) or transcript rewind (`DetectRewind`), while 1-step hints (`about-to-edit`, `error-match`) use `ephemeralMessage`.
- **Two-Stage 5-Minute Warm-Cache Distillation:**
  - **Stage 1 (100% Automatic End-of-Turn Warm-Cache Harvest & Free Artifact Capture):** When an agent finishes a milestone turn (`Stop` with `terminationReason == "NO_TOOL_CALL"`, `fullyIdle == true`, `executionNum == 0`) that passes the deterministic **Milestone Gate** (`git commit`, PR opened, work order completed, error/test resolved after $\ge 15$ steps, or edits to $\ge 2$ files followed by a build/test), the hook returns `{"decision": "continue", "reason": "<request>"}` to harvest a structured `repo-memory-note` block while 94–99% of the session transcript sits in the 5-minute warm KV cache (~90% input token discount)—supplemented by zero-LLM harvesting of `CHECKPOINT` summaries and `<appDataDir>/brain/<conversation-id>/*.md` artifacts. Calling `memory_note` mid-turn is an optional override that advances the conversation mark so `Stop` skips duplicate continuation.
  - **Stage 2 (Warm-Cache Multi-Turn Consolidation):** The background tick runs a single multi-turn `--input-format stream-json --output-format stream-json` `agy -p --agent repo-memory-distiller` session (`X8`), passing candidates through the **Echo Filter (P13)** and **Pre-Consolidation Corroboration Gate** (`PostToolUse` build/test pass, verified `HEAD` anchor, or `USER_INPUT` confirmation) before committing operations to `.memory/ops/<yyyy-mm>/<ulid>.json`.
- **Eight Entry Types Across Three Formal Cognitive Lifecycles:**
  - **`semantic`** (`invariant`, `contract` [alias `convention`], `gotcha` [alias `external-behaviour`], `open-issue`): Module truths, interface contracts, platform/SDK behaviors, and machine-closable (`closes_when`) issues, verified against AST/file fingerprints at collection and before every push.
  - **`procedural`** (`recipe` [alias `procedure`], `failure-signature`): Validated by `PostToolUse` command exit codes (`success_count` / `failure_count`), flagged `stale_suspect` when build manifests change, and deterministically graduated into `.agents/skills/rm-<slug>/SKILL.md` once `success_count >= 2` across $\ge 2$ conversations (`Addendum A §10`).
  - **`episodic`** (`decision`, `ruled-out`): Append-only architectural choices (with rejected alternatives) and dead-end approaches that failed, preserved across time and superseded only by newer `decision` ops.
- **Day-1 Value via Doc Seeding & Async Freshness ("Dreaming"):** `repo-memory seed-docs` bootstraps dossiers from curated repository docs (`docs/developer/**/*.md`, `.agents/skills/**/SKILL.md`, or paths configured under `doc_seeds` in `.memory/config.yaml`) (`source: doc-seed`) and builds `<data-dir>/repos/<repo-id>/index.db` on Day 1. During background ticks, the daemon compares `source_doc_blob_sha` against git ("dreaming") to re-distill entries when source documentation changes.

---

## Architecture & Storage Layout

### Committed Repository State (`.memory/` — `repo:` and `branch:` tiers)

```text
.memory/
├── config.yaml                  # Scopes, aliases, scrub rules, and doc_seeds settings
├── scrub.yaml                   # Shared secret/PII scrubbing rules (fail-closed)
├── ATLAS.md                     # Deterministically rendered 1-line-per-dossier index
├── dossiers/                    # Deterministically rendered from .memory/ops/ (~3k token cap)
│   ├── <scope>.md               # Active entries ({section?} omits empty headings)
│   └── <scope>.history.md       # Superseded and retired entries
├── ops/
│   └── <yyyy-mm>/<ulid>.json    # Append-only operation batches (conflict-free in git)
└── local/                       # Gitignored branch-scoped ops & dossiers (`branch:` tier)
    ├── ops/<branch-slug>/<ulid>.json
    └── dossiers/<branch-slug>/...
```

### Private Machine State (`<data-dir>/repos/<repo-id>/` — `user:` and `session:` tiers)

```text
<data-dir>/repos/<repo-id>/
├── index.db                     # Rebuildable SQLite FTS5 + BM25 + anchor index (index.IndexPath)
├── jobs.sqlite                  # Private tick job queue (harvest, fallback, consolidate, seed-docs)
├── user-dossiers/               # Personal developer dossiers (`user:` tier, never committed)
├── state/
│   ├── marks/<conv-id>.json     # Per-conversation mark, ExposureList, InjectionLedger (`session:` tier)
│   ├── freshness.json           # Op-log cursor, HEAD SHA, inbox size, suspect count
│   └── subagents.sqlite         # Child subagent conversation -> parent mapping
├── evidence/                    # Scrubbed episodes, artifacts, checkpoints, inbox notes, PA reports
├── quarantine/                  # Notes held if a scrub layer errors
└── spool/                       # Lock-free hook event spool files ingested by tick
```

---

## Two-Plugin Packaging Split

Antigravity spawns `sidecar.json` scheduled background daemons (`builtin: schedule`) from global plugins under `~/.gemini/config/plugins/<name>/`, while workspace hooks and MCP tools can be scoped per repository via `.agents/plugins.json` ([`docs/experiments/phase0-plugin-split.md`](docs/experiments/phase0-plugin-split.md)):

1. **Global Daemon Plugin (`~/.gemini/config/plugins/repo-memory-daemon/`):** Runs the 1-minute `repo-memory tick` sidecar with **zero global hooks** (zero overhead in non-enrolled workspaces) and bundles `workspace-plugin/repo-memory/`.
2. **Per-Workspace Plugin (`.agents/plugins.json`):** `repo-memory enroll` (`init`) writes the resolved **absolute path** to `workspace-plugin/repo-memory` into `.agents/plugins.json`, activating `hooks/hooks.json` (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`), `repo-memory mcp`, and `repo-memory-distiller` only for enrolled repositories.

---

## CLI Reference

```bash
# Build and test
go build ./cmd/repo-memory
go test ./...

# Enrol (or leave) a repository and bootstrap Day-1 dossiers + index.db
repo-memory enroll          # alias: repo-memory init
repo-memory unenroll        # alias: repo-memory leave

# Seed from curated repository markdown docs & skills, or historical PA work orders
repo-memory seed-docs [--repo <path>] [--dry-run]
repo-memory seed [--repo <path>] [--before <rfc3339>]

# Deterministically re-render dossiers/ATLAS.md and rebuild index.db from .memory/ops/
repo-memory reindex [--repo <path>]

# Run a single background tick pass manually (or dry-run spend estimates)
repo-memory tick            # alias: repo-memory run

# Inspect status, search, usage, diagnostics, and reports
repo-memory status
repo-memory search <query>
repo-memory usage
repo-memory report
repo-memory snapshot
repo-memory doctor

# Emergency kill switch
repo-memory disable [--all]
repo-memory enable
```

---

## MCP Pull Tools (`repo-memory mcp`)

Spawned per session over `stdio` and backed by `<data-dir>/repos/<repo-id>/index.db`:

| Tool | Purpose |
|---|---|
| `memory_brief` | Return the full dossier for a module, topic, or path (with freshness header and gap lines) |
| `memory_why` | Trace an entry ID, file, symbol, or `path:line` (`Addendum A §13`) back to its rationale, commits, and evidence |
| `memory_history` | Chronological op-log evolution (`add`, `amend`, `supersede`, `retire`) for an entry, file, or dossier |
| `memory_search` | FTS5 (`ftsScore * 100`) search across active entries, unconsolidated inbox notes, and evidence |
| `memory_note` | Optional mid-turn override to record a structured discovery immediately (advances the conversation mark so `Stop` deduplicates) |
| `memory_status` | Repository memory coverage, dossier token utilization, freshness state, and procedural validation counts (alias `memory_stats`) |

---

## Documentation Index

- [`SPEC.md`](SPEC.md) — Canonical `v1.0` System Specification (`v2.2` Architecture)
- [`docs/design-v2-distillation-retrieval.md`](docs/design-v2-distillation-retrieval.md) — v2 Distillation, Storage & Retrieval Architecture (Revision 4, `v2.2`)
- [`docs/addendum-a-provenance-and-hardening.md`](docs/addendum-a-provenance-and-hardening.md) — Provenance, Seeding, Validation & Hardening (Revision 3)
- [`docs/antigravity-hooks-reference.md`](docs/antigravity-hooks-reference.md) — Empirical Antigravity Hooks Reference (`injectSteps`, `userMessage` vs `ephemeralMessage`, subagent ordering, two-plugin split)
- [`docs/experiments/SPEC-CHANGES.md`](docs/experiments/SPEC-CHANGES.md) — Experiment Spike (`E01`–`E12`) Specification Reconciliation Ledger
- [`docs/evaluations/pa-memory-retrieval-effectiveness.md`](docs/evaluations/pa-memory-retrieval-effectiveness.md) — 231-Session Empirical Study of Pull vs Push Memory Retrieval
