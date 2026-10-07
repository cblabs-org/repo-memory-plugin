# PR-08-v2-delivery: Channels, Moments & Injection Ledger

> **Programme:** repo-memory
> **PR:** pr-08-v2-delivery
> **Base branch:** `main` (after PR-07 merged at `2405db9`)
> **Status:** 🔄 **Active** — WO1 in progress
> **SPEC:** [SPEC.md](../../../../SPEC.md) v0.10 + [v2 Design](../../../../docs/design-v2-distillation-retrieval.md) rev 3 §8.1–8.6
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md) lines 233–261
> **Human Approval:** Autonomous execution authority granted (2026-10-04, programme-level)

## Objective

Deliver dossier entries precisely and keep them in context:
- persistent `systemMessage` for knowledge that lasts;
- `ephemeralMessage` for hints at the moment of need;
- deny-once gates;
- the injection ledger, with compaction and rewind re-injection;
- the pull tools (MCP).

This is the last implementation PR before the early kill gate (PR-09 X6-lite).

## Design & SPEC Sections

- **v2 Design rev 3:** §8.1 (Principles), §8.2 (Channels & Budgets), §8.3 (Moments & Trigger Matrix), §8.4 (Compaction & Rewind), §8.5 (Scale), §8.6 (Native Knowledge Items)
- **Spike findings:** X5 (injectSteps contracts), X9 (hook fail-open/fail-closed), X11 (compaction detection)
- **SPEC:** §7.5 (Risky-action gates), §7.7 (Pull tools)

## Dependencies

- **Base:** `main` at `2405db9` (PR-07 merged)
- **Reads from:** `internal/core/ops/` (op log), `internal/core/dossiers/` (entries, projector), `internal/core/index/` (anchor lookups, FTS5, error patterns), `internal/core/config/` (scope resolver)
- **Integrates into:** `internal/harness/antigravity/` (hook handlers), `internal/mcpserver/` (pull tools)
- **Spike code reference:** `internal/harness/antigravity/v2_spike_channels.go` (InjectStep, SystemMessageEntry contracts)
- **Informs:** PR-09 X6-lite (delivered dossiers), PR-11 (exposure lists for echo filter)

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | [wo1-ledger-engine](wo1-ledger-engine/WORKORDER.md) | Ledger (entry IDs, channel, step, content hash, tags), ranking engine (anchor match → relevance → verified → evidence), budget enforcement, exposure list | architect | 🔄 Active |
| 2 | wo2-persistent-pushes | `PreInvocation` as `systemMessage`. Task start (≤500 tokens), first entry into dossier (≤150), first touch of anchor (≤150). Branch diff precomputed. | architect | ☐ Planned |
| 3 | wo3-ephemeral-and-errors | About-to-edit pointer (≤50 tokens). Error matching on `PostToolUse` → `ephemeralMessage` (≤200 tokens). | standard | ☐ Planned |
| 4 | wo4-gates | `PreToolUse` deny-once for `gate: edit` entries. `force_ask` for invariants. X9 contract. | standard | ☐ Planned |
| 5 | wo5-compaction-reinjection | Detect compaction/rewind from transcript. Re-inject consolidated `systemMessage` (≤1k). Re-arm dedup. **X11 experiment.** | architect | ☐ Planned |
| 6 | wo6-pull-tools | MCP tools: `memory.brief`, `memory.why`, `memory.history`, `memory.search`, `memory.note`, `memory.stats`. Pulls recorded in exposure list. | standard | ☐ Planned |

## Exit Criteria

- X5 confirmed in production code
- X11: compaction detection ≥95%, re-injection works
- All moments deliver correctly, budgets respected, ranking correct
- Gates deny once, then allow retry
- Hook latency: p95 ≤60ms when nothing injected, ≤150ms when injecting
- All MCP tools work; exposure lists populated

## Modules & Components

| Module | Package | Status |
|---|---|---|
| Delivery Engine | `internal/core/delivery/` | WO1 — new |
| PreInvocation Handler | `internal/harness/antigravity/` | WO2 — extend |
| PostToolUse Handler | `internal/harness/antigravity/` | WO3 — extend |
| PreToolUse Handler | `internal/harness/antigravity/` | WO4 — extend |
| Compaction Detection | `internal/core/delivery/` | WO5 — new |
| MCP Pull Tools | `internal/mcpserver/` | WO6 — extend |

## Risks

| Risk | Mitigation | Owner |
|---|---|---|
| Hook latency budget exceeded (p95 ≤60ms no-inject) | Precompute in tick, not in hook; SQLite index for fast lookups; budget enforcement | WO1, WO2 |
| Compaction re-injection (X11) least-validated mechanism | Careful experiment; detect CHECKPOINT in transcript | WO5 |
| MCP server tool registration complexity | Extend existing Server struct; reuse index queries | WO6 |
| `systemMessage` nested object contract brittle | Use proven InjectStep/SystemMessageEntry from spike code | WO2 |
| `PreToolUse` fail-closed can block agent | Always return `{"decision":"allow"}` on error/panic; per X9 | WO4 |

## Session Log

### 2026-10-07 — Project kickoff (Chief Architect)
- Project workspace activated. 6 WOs decomposed per PROGRAM-PLAN.md §PR-08.
- Base: `main` at `2405db9` after PR-07 merged.
- Codebase surveyed: delivery integrates into existing hook handlers (`hooks.go`, `v2_spike_channels.go`), reads from ops/dossiers/index/scopes, extends MCP server.
- Key contracts confirmed from X5/X9/X11 spike findings: nested `systemMessage` objects, fail-closed `PreToolUse`, CHECKPOINT-based compaction detection.
- WO1 (ledger-engine) spawned as first worker.
