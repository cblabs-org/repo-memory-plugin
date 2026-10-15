# docs/experiments

## Invariants & contracts

### [docseed-088fe7bf] **Harvest, `CHECKPOINT`/artifact capture, ledger, re-injection checks** — none (deterministic) — none — Per turn — none
`invariant · kind: semantic · anchors: docs/experiments/spike-summary.md, docs/experiments/X8-lean-agent.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-1e650408] X8 — **Lean agent.** Measure `agy -p --agent repo-memory-distiller` input tokens, single-shot and over `stream-json`; verify hook isolation and frontmatter keys. — 38.4% fixed token reduction (~6.6k tokens saved); no hook recursion with plugin split — **PASS** (`docs/experiments/X8-lean-agent.md`, `phase0-plugin-split.md`)
`invariant · kind: semantic · anchors: docs/experiments/X8-lean-agent.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-393e2fc3] **Turn note (`Stop` continue, Stage 1)** — ~1–5k uncached, plus warm 5-min cached context (median 75k on Gemini, 130k on Opus; 94–99% cache hit rate, ~90% discount) — 300–1,000 — Milestones only: about 2–6 a day at esp32-app-foundation's session rates — The session's model
`invariant · kind: semantic · anchors: docs/experiments/spike-summary.md, docs/experiments/X8-lean-agent.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-5f6e0bf1] **Lean agent.** `agy -p --agent repo-memory-distiller`. In experiment X8 (`docs/experiments/X8-lean-agent.md`), we verified the exact frontmatter keys supported by the custom agent loader (`name`, `description`, `mainAgent`, `subagent`, `hidden`, `model`, `enable_write_tools`, `enable_mcp_tools`, `enable_subagent_tools`), which cut fixed input token overhead by **38.4%** (~6.6k tokens saved per...
`invariant · kind: semantic · anchors: docs/experiments/X8-lean-agent.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-77661109] v1 dogfood pipeline: ~112k per episode.
`invariant · kind: semantic · anchors: docs/experiments/spike-summary.md, docs/experiments/X8-lean-agent.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-7e05295b] **Global Daemon Plugin (`~/.gemini/config/plugins/repo-memory-daemon/`):** Declares `plugin.json` and `sidecar.json` (`builtin: schedule`) so the scheduled tick runs while the IDE is open, and bundles the workspace plugin inside `workspace-plugin/repo-memory/`. It declares **no global hooks**, preventing hooks from firing in non-enrolled workspaces.
`invariant · kind: semantic · anchors: docs/experiments/phase0-plugin-split.md · source_doc: docs/antigravity-hooks-reference.md · evidence: doc:docs/antigravity-hooks-reference.md`

### [docseed-a319591f] X5 — **Delivery channels** (Q2–Q4). Verify `systemMessage`, `userMessage`, and `ephemeralMessage` persistence across invocations, turns, and `CHECKPOINT`. — Channels confirmed as in §8.2 (`userMessage` persistent on `agy 1.3.3`; `ephemeralMessage` 1-step) — **PASS** (`docs/experiments/X5-channels.md`, `X5-injectSteps-investigation.md`)
`invariant · kind: semantic · anchors: docs/experiments/X5-channels.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-c27dbb35] **Delivery** — none (deterministic) — none — Injects ≤ ~1–2.5k per session, including re-injection — n/a
`invariant · kind: semantic · anchors: docs/experiments/spike-summary.md, docs/experiments/X8-lean-agent.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-c9ca41b5] Splitting packaging into a global daemon plugin (`~/.gemini/config/plugins/repo-memory-daemon/`, sidecar only) and a workspace-scoped plugin (`workspace-plugin/repo-memory/`, hooks + MCP + skills enabled via `.agents/plugins.json` with resolved absolute paths) guarantees zero hook or MCP overhead in unenrolled repositories. — Phase 0 spike (`docs/experiments/phase0-plugin-split.md`)
`invariant · kind: semantic · anchors: docs/experiments/phase0-plugin-split.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-e98c6ff1] Disabling write tools, MCP tools, and subagent tools strips all mutation, MCP server, and subagent schemas from the prompt. In addition, the workspace/daemon plugin split (`docs/experiments/phase0-plugin-split.md`) and the `REPO_MEMORY_INTERNAL=1` environment variable guard prevent hook recursion inside the distiller.
`contract · kind: semantic · anchors: docs/experiments/phase0-plugin-split.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-f1998a5d] X9 — **Hook exit codes** (Q27). Run `PreToolUse` with exit 1, exit 2, empty output and `{}`, and do the same for the other hook types. — `exit 1` fails open; `exit 2` and `{}` fail closed (deny) — **PASS** (`docs/experiments/spike-summary.md`)
`invariant · kind: semantic · anchors: docs/experiments/spike-summary.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-f4c6de9d] PA: ~5k per work order to produce, plus ~9k per session to load (and 94–99% unread when left to passive pull).
`invariant · kind: semantic · anchors: docs/experiments/spike-summary.md, docs/experiments/X8-lean-agent.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-f66eb00e] **Per-Workspace Plugin (`<workspace>/.agents/plugins.json`):** Enrolled repositories reference the nested `workspace-plugin/repo-memory` directory via a resolved **absolute path** (because `.agents/plugins.json` does not expand `~` or `$HOME`). This activates `hooks.json`, `mcp.json`, agents, and skills exclusively for that workspace.
`invariant · kind: semantic · anchors: docs/experiments/phase0-plugin-split.md · source_doc: docs/antigravity-hooks-reference.md · evidence: doc:docs/antigravity-hooks-reference.md`

## Ruled out, failure signatures and gotchas

### [seed-8675082c] **Warning on `.agents/plugins.json`:** Never write `~` or `$HOME` in `.agents/plugins.json`. Antigravity's Go customization loader does not evaluate shell expansions, resulting in silent load failures.
`ruled-out · kind: episodic · anchors: docs/experiments/phase0-plugin-split.md`

### [seed-a45619cf] **Warning:** Token measurement is impossible by parsing transcripts right now.
`ruled-out · kind: episodic · anchors: docs/experiments/X3-quota.md, docs/experiments/spike-summary.md`

### [seed-d472d172] **Dead End:** `PreInvocation` context delivery via `systemMessage` and `userMessage`. We must pivot to an alternative for PR-08.
`ruled-out · kind: episodic · anchors: docs/experiments/X3-quota.md, docs/experiments/spike-summary.md`

## Decisions

### [seed-26fc4334] **Target layout confirmed:** `workspace-plugin/repo-memory` can safely be nested under `repo-memory-daemon/` without global discovery collision.
`decision · kind: episodic · anchors: docs/experiments/phase0-plugin-split.md`

### [seed-6ed4ed4d] **Absolute paths required:** `repo-memory enroll` (WO4) must resolve and write absolute paths to `.agents/plugins.json` because Antigravity's loader does not expand `~` or `$HOME`.
`decision · kind: episodic · anchors: docs/experiments/phase0-plugin-split.md`

### [seed-86292314] Chose to omit token tracking directly from the X3-quota document's "results" other than noting its absence, since the platform simply doesn't expose it there.
`decision · kind: episodic · anchors: docs/experiments/X3-quota.md, docs/experiments/spike-summary.md`

### [seed-bcdfacd4] **Sidecar directory configuration supported:** Storing `config.json` in `sidecars/tick/config.json` causes zero supervisor or validation issues.
`decision · kind: episodic · anchors: docs/experiments/phase0-plugin-split.md`

## Procedures / Recipes

### [docseed-3247f175] `agy -p "/usage" --output-format json` reports quota and spends zero tokens. Gemini Pro, Gemini Flash and Claude/GPT have **separate quota pools, each with a per-week and per-5-hour cap**. — Live call, X3 (`docs/experiments/spike-summary.md`, Q7, Q41)
`recipe · kind: procedural · anchors: docs/experiments/spike-summary.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-6326e025] **Warm Batch Consolidation (Stage 2, `stream-json`)** — ~10.6k fixed overhead once on Turn 1, plus ~0.5k per note and 1–3k per dossier across Turns $2..N$ (warm prefix cache) — ~1–2k — Daily, 1 multi-turn `stream-json` session — Chosen by the `/usage` quota check
`recipe · kind: procedural · anchors: docs/experiments/spike-summary.md, docs/experiments/X8-lean-agent.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-9226563d] X3 — **Quota accounting** (Q6, Q41). Read `/usage` before and after turn notes and lean-agent calls. — Confirms the gate and fallback budgets — **PASS** (`docs/experiments/spike-summary.md`)
`recipe · kind: procedural · anchors: docs/experiments/spike-summary.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-f4e23c70] **Sidecar fallback (`repo-memory-distiller`)** — ~10.6k fixed overhead (down 38.4% from 17.2k in X8), plus 5–15k per conversation (checkpoint plus tail), shared across `stream-json` turns — ~500 per conversation — Missed conversations only — Chosen by the `/usage` quota check
`recipe · kind: procedural · anchors: docs/experiments/spike-summary.md, docs/experiments/X8-lean-agent.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

## Open issues

### [seed-8389c92c] In `internal/harness/antigravity/attribution_test.go`, `TestSessionAttributor_Integration` fails when executed beyond the hardcoded test timestamp (`now := time.Date(2026, 10, 2, ...)`), because dynamically created transcript files on disk have modification times newer than `now` (`now.Sub(fi.ModTime())` is negative, triggering the idle cutoff check).
`open-issue · kind: semantic · anchors: docs/experiments/phase0-plugin-split.md`

### [seed-c7598b0b] `agy -p /usage` acts as an undocumented, zero-token CLI subcommand (though the interactive TUI `/usage` behaves differently). We must invoke it via `-p` and JSON output to reliably parse the model buckets.
`open-issue · kind: semantic · anchors: docs/experiments/X3-quota.md, docs/experiments/spike-summary.md`
