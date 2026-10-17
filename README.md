# repo-memory

**Repository-scoped, evidence-linked engineering memory for coding agents (Antigravity & Claude Code).**

> **Specification:** [`SPEC.md` (v1.0)](SPEC.md) · **Design (v2.2):** [`docs/design-v2-distillation-retrieval.md`](docs/design-v2-distillation-retrieval.md) · **Provenance & Hardening:** [`docs/addendum-a-provenance-and-hardening.md`](docs/addendum-a-provenance-and-hardening.md) · **Packaging & Distribution:** [`docs/packaging-and-distribution.md`](docs/packaging-and-distribution.md) · **Hooks Reference:** [`docs/antigravity-hooks-reference.md`](docs/antigravity-hooks-reference.md)

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

## Installation, Enrolment & Upgrading

`repo-memory` installs once globally on your machine (`~/.gemini/config/plugins/repo-memory-daemon/`) with **zero global hook overhead**, and is then enabled selectively per repository via `/repo-memory-admin` or `repo-memory enroll`.

### Step 1 — Install the Global Plugin (`repo-memory-daemon`)

Choose any of the three installation methods below:

#### Option A: One-Line Release Installer (Recommended — No Go Toolchain Required)

Downloads the pre-compiled binary for your OS and CPU architecture (`darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`) from [GitHub Releases](https://github.com/cblabs-org/repo-memory-plugin/releases), links `repo-memory` into `~/.local/bin/repo-memory`, and installs the `repo-memory-daemon` plugin bundle:

```bash
curl -fsSL https://raw.githubusercontent.com/cblabs-org/repo-memory-plugin/main/scripts/install-release.sh | sh
```

#### Option B: Download & Unzip Plugin Archive (Zero CLI)

1. Download the archive for your platform from [GitHub Releases](https://github.com/cblabs-org/repo-memory-plugin/releases):
   - **macOS (Apple Silicon):** `repo-memory-daemon-darwin-arm64.zip`
   - **macOS (Intel):** `repo-memory-daemon-darwin-amd64.zip`
   - **Linux (x86_64):** `repo-memory-daemon-linux-amd64.zip`
   - **Linux (ARM64):** `repo-memory-daemon-linux-arm64.zip`
   - **Windows (x86_64):** `repo-memory-daemon-windows-amd64.zip`
2. Extract the `.zip` directly into `~/.gemini/config/plugins/` so it creates `~/.gemini/config/plugins/repo-memory-daemon/`:
   ```bash
   mkdir -p ~/.gemini/config/plugins
   unzip -q repo-memory-daemon-<os>-<arch>.zip -d ~/.gemini/config/plugins/
   ```
   *(Optional: symlink `~/.gemini/config/plugins/repo-memory-daemon/bin/repo-memory` into `~/.local/bin/repo-memory` if you also want the `repo-memory` CLI on your terminal `$PATH`. On macOS, the bundled `bin/run-repo-memory.sh` wrapper automatically clears Browser/Finder `com.apple.quarantine` attributes on first run.)*
3. Restart Antigravity so it discovers the newly added plugin directory.

#### Option C: Build & Install from Source (Requires Go 1.24+)

```bash
git clone https://github.com/cblabs-org/repo-memory-plugin.git
cd repo-memory-plugin
./scripts/install.sh   # or: make install
```

*(To build all cross-platform binaries and portable `.zip` bundles locally into `dist/`, run `make dist-zip`.)*

---

### Step 2 — Enable the Background Tick Sidecar

After installing the plugin, enable the 1-minute background worker (`Repo Memory Tick`) so `repo-memory` automatically consolidates inbox notes, validates procedural recipes, and keeps doc seeds fresh:

1. Open the Antigravity Automations Dashboard (`sidecar://dashboard` or **Settings → Automations**).
2. Toggle **"Repo Memory Tick"** (`repo-memory-daemon/tick`) to **Enabled**.

---

### Step 3 — Enroll a Repository

Open any Git repository in Antigravity and either:

- **From Chat (Interactive):**
  ```text
  /repo-memory-admin enroll this repo please
  ```
  The `repo-memory-admin` skill inspects the repository, confirms your Day-1 seeding options (repository docs/skills and any Principal Architect history), runs `enroll`, and verifies health with `doctor` and `status`.

- **From the Terminal (CLI):**
  ```bash
  repo-memory enroll .
  repo-memory status
  repo-memory doctor .
  ```

What `enroll` sets up in the repository:
- **Committed in Git (`.memory/`):** `config.yaml`, `scrub.yaml`, `ATLAS.md`, `QUESTIONS.md`, `dossiers/`, and `ops/` (plus the managed Atlas block in `GEMINI.md`) so engineering memory is shared across the team.
- **Git-Excluded Local Pointers:** `.memory/.enrolled` and `.agents/plugins.json` (added automatically to `.git/info/exclude`), activating `hooks.json` and the `repo-memory` MCP server for that workspace only.

---

### Upgrading & Uninstalling

- **Upgrading:** Because `~/.gemini/config/plugins/repo-memory-daemon/` is 100% stateless (all repository memory lives in `<repo>/.memory/` and local index state lives in your OS user data directory), upgrading via any method (`git pull && ./scripts/install.sh`, re-running `install-release.sh`, or unzipping a newer `repo-memory-daemon-<os>-<arch>.zip` over `~/.gemini/config/plugins/repo-memory-daemon/`) **immediately upgrades all enrolled repositories on your machine** without re-enrolling.
- **Checking Version Sync:** Run `repo-memory status` or `repo-memory doctor .` at any time to verify that your CLI binary and installed plugin bundle versions match.
- **Unenrolling a Repository:** Run `repo-memory unenroll .` (or `/repo-memory-admin unenroll this repo`) to remove hooks and `.agents/plugins.json` while keeping committed `.memory/` files intact (pass `--purge` to also delete local SQLite index state).
- **Uninstalling Globally:** Run `repo-memory uninstall --harness antigravity` (pass `--purge` to also remove all local user data).

---

## Architecture & Storage Layout

### Committed Repository State (`.memory/` — `repo:` and `branch:` tiers)

```text
.memory/
├── config.yaml                  # Scopes, aliases, scrub rules, and doc_seeds settings
├── scrub.yaml                   # Shared secret/PII scrubbing rules (fail-closed)
├── ATLAS.md                     # Deterministically rendered 1-line-per-dossier index
├── QUESTIONS.md                 # Unresolved open issues, suspect entries, and coverage gaps
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

1. **Global Daemon Plugin (`~/.gemini/config/plugins/repo-memory-daemon/`):** Runs the 1-minute `repo-memory tick` sidecar with **zero global hooks** (zero overhead in non-enrolled workspaces), exposes the global `repo-memory-admin` management skill (`skills/repo-memory-admin/SKILL.md`, invokable via `/repo-memory-admin` in any workspace to enroll, seed, inspect, or unenroll repositories), and bundles `workspace-plugin/repo-memory/`.
2. **Per-Workspace Plugin (`.agents/plugins.json`):** `repo-memory enroll` (`init`) writes the resolved **absolute path** to `workspace-plugin/repo-memory` into `.agents/plugins.json`, activating `hooks.json` (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`), `repo-memory mcp`, and `repo-memory-distiller` only for enrolled repositories.

---

## CLI Reference

```bash
# Build and test
go build ./cmd/repo-memory
go test ./cmd/... ./internal/... ./packaging/...

# Installation & Repository Enrolment
repo-memory install --harness <antigravity|claude>
repo-memory uninstall --harness <antigravity|claude>
repo-memory enroll                                      # alias: repo-memory init
repo-memory unenroll                                    # alias: repo-memory leave

# Seeding & Op-Log Maintenance
repo-memory seed-docs [--repo <path>] [--dry-run] [--json]
repo-memory seed [--repo <path>] [--before <commit>] [--dry-run]
repo-memory ops gc [--repo <path>] [--dossiers-dir <path>] [--dry-run]

# Consolidation, Skill Graduation & Gardening
repo-memory consolidate [--repo <path>] [--dry-run] [--json]
repo-memory promote-skills [--repo <path>] [--dry-run] [--json]
repo-memory garden [--repo <path>] [--dry-run] [--json]

# Principal Architect (PA) Integration
repo-memory summary <session-id-or-path> [--max-tokens <n>] [--json]
repo-memory brief [--workorder <path>] [--scope <scope>] [--max-tokens <n>] [--json]

# Backfill, Reprojection & Indexing
repo-memory backfill [--repo <path>] [--transcripts] [--docs] [--git] [--max-items <n>] [--force] [--dry-run] [--json] # alias: repo-memory backfill-captures
repo-memory reproject [--repo <path>] [--include-local] [--allow-loss] [--dry-run] [--json]
repo-memory reindex [--repo <path>]

# Background Tick, Observability & Diagnostics
repo-memory tick                                        # alias: repo-memory run
repo-memory status [--json]
repo-memory search <query>
repo-memory usage
repo-memory report [--days <n>] [--json]
repo-memory snapshot
repo-memory doctor [path]
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
- [`docs/packaging-and-distribution.md`](docs/packaging-and-distribution.md) — Dual-Channel Plugin Packaging, Portable `.zip` Bundles & Versioning Guide
- [`docs/antigravity-hooks-reference.md`](docs/antigravity-hooks-reference.md) — Empirical Antigravity Hooks Reference (`injectSteps`, `userMessage` vs `ephemeralMessage`, subagent ordering, two-plugin split)
- [`docs/experiments/SPEC-CHANGES.md`](docs/experiments/SPEC-CHANGES.md) — Experiment Spike (`E01`–`E12`) Specification Reconciliation Ledger
- [`docs/evaluations/pa-memory-retrieval-effectiveness.md`](docs/evaluations/pa-memory-retrieval-effectiveness.md) — 231-Session Empirical Study of Pull vs Push Memory Retrieval
