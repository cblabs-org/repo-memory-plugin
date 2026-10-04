# PR-06-v2-spike: Prototype Spike

> **Programme:** repo-memory
> **PR:** pr-06-v2-spike
> **Base branch:** `pr-05-v2-foundation`
> **Status:** ☐ Planned
> **SPEC:** [SPEC.md](../../../../SPEC.md) v0.10 + [v2 Design](../../../../docs/design-v2-distillation-retrieval.md) rev 3
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Validate the platform mechanics v2 relies on.
- **Prototype code** (the `Stop` gate and harvest, `systemMessage` injection, the lean agent) lives on a spike branch behind a feature flag (`REPO_MEMORY_V2_SPIKE=1`), installed locally, with a documented rollback.
- **What merges:** only the experiment reports in `docs/experiments/` and the proposed SPEC edits. Production versions are built in PR-07 onward.

## Design & SPEC Sections

- **v2 Design rev 3:** §2.3 (Mechanics), §5.1 (`Stop` gate and harvest), §5.4 (Prompt design and cost), §8.2–8.4 (Delivery channels and moments), §9 (Cost, quota and gen_metadata), §12 (Validation experiments X1, X3, X5, X8, X9)
- **SPEC:** Proposed edits leading toward SPEC v1.0

## Dependencies & Gate Preconditions

- **Base branch:** `pr-05-v2-foundation` (PR-05-v2-foundation must be completed and merged before PR-06 begins)
- **Dependencies:** PR-05-v2-foundation
- **Gate Preconditions:** PR-06 establishes the **X-gate**. All exit criteria in this PR form the X-gate and must pass before PR-07 (Storage) can begin.

### Spike Environment, Feature Flag & Operational Procedures

- **Spike Branch Naming & Merge Policy:** Prototype code lives on dedicated spike branch(es) named `pr-06-v2-spike/prototype` (or `pr-06-v2-spike/woX-*`). Prototype code is **never merged** to the PR branch or `main`. Only experiment reports in `docs/experiments/` and proposed SPEC v1.0 edits merge to the PR branch.
- **Feature Flag:** All prototype hook behaviours (`Stop` gate/harvest, `systemMessage` injection, lean-agent invocation) are strictly gated behind the environment variable:
  ```bash
  REPO_MEMORY_V2_SPIKE=1
  ```
  When the flag is unset or set to `0`, prototype hook handlers must remain completely inactive and fail open.
- **Local Install Procedure:**
  1. Check out the prototype spike branch (`pr-06-v2-spike/prototype`).
  2. Build the prototype binary locally:
     ```bash
     go build -o repo-memory ./cmd/repo-memory
     ```
  3. Install the plugin into the local Antigravity environment:
     ```bash
     ./repo-memory install --harness antigravity
     ```
  4. Enable the prototype feature flag in the test or shell session:
     ```bash
     export REPO_MEMORY_V2_SPIKE=1
     ```
  5. Verify hook registration in `~/.gemini/config/plugins/repo-memory/hooks.json`.
- **Rollback Procedure:**
  1. Deactivate the feature flag:
     ```bash
     unset REPO_MEMORY_V2_SPIKE
     ```
  2. Check out the clean base branch:
     ```bash
     git checkout pr-05-v2-foundation
     ```
  3. Rebuild the clean binary:
     ```bash
     go build -o repo-memory ./cmd/repo-memory
     ```
  4. Reinstall the clean plugin:
     ```bash
     ./repo-memory install --harness antigravity
     ```
  5. Verify that `~/.gemini/config/plugins/repo-memory/hooks.json` reflects the clean PR-05 state and all hooks fail open.

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | `wo1-stop-continue` | **X1.** A prototype `Stop` gate: `terminationReason` equals `NO_TOOL_CALL` (case-insensitive), `fullyIdle`, `executionNum == 0`; continue with the draft request; harvest at `executionNum >= 1`. Measure per note from `gen_metadata`: uncached, cached and output tokens. Also measure user visibility; whether a user message can arrive mid-continuation; and `max_stop_hook_continuations`. **Include PA and worker sessions:** check that a continuation at a PA milestone or worker completion doesn't disrupt the PA loop or completion reports. Sample: ≥ 20 requests. | interactive | ☐ Planned |
| 2 | `wo2-hook-contracts` | **X9.** For every hook type, test exit 1, exit 2, empty output and `{}`. Document fail-open versus fail-closed per type, and update the hook handlers' contract tests to match. | standard | ☐ Planned |
| 3 | `wo3-channels` | **X5.** Do `systemMessage` and `userMessage` persist across invocations and turns? Do they keep the cache? Do they survive a `CHECKPOINT`? Produce a checkpoint deliberately: a long synthetic session, or a scripted large-context run, described in the report. Compare a first-touch push plus an edit pointer against deny-once on edit: what the agent does next, and how many extra steps. Cross-reference issue #2 Q44–Q49. | interactive | ☐ Planned |
| 4 | `wo4-lean-agent` | **X8.** Build the `repo-memory-distiller` Markdown agent (`excludeDefaultComponents: true`, `tools: []`, `inheritCustomizations: false`, `inheritMcp: false`, `hidden: true`, `mainAgent: true`). Find where it must be installed to resolve from the **runtime working directory `<data-dir>/runs/<repo-id>/`** (user-level `~/.gemini/config/agents/`, or inside the plugin if plugins can ship agents). Measure input tokens, single-shot and over `stream-json`. Check whether our plugin hooks fire inside it, and whether `--json-schema` applies. | interactive | ☐ Planned |
| 5 | `wo5-quota-and-reconciliation` | **X3.** `/usage` reports `remaining_fraction` (about 0.1% resolution) only for the cap nearest to being reached (`window` is null). Run on an otherwise idle pool, in batches large enough to move the reading, at least twice. Compare turn notes (cached context) with lean-agent calls. Then summarise X1, X3, X5, X8 and X9. List every v2 design section confirmed or changed, and propose SPEC v1.0 edits for user approval before PR-07. | standard | ☐ Planned |

## Exit Criteria

*(Exit Criteria define the **X-gate** that must pass before PR-07)*

- **X1:** harvest succeeds on ≥ 95% of ≥ 20 requests; median uncached input ≤ 10k per note; no interleaving; no disruption to PA or worker sessions, or an opt-out by role defined.
- **X9:** behaviour documented per hook type; contract tests updated.
- **X5:** persistence across invocations, turns and checkpoints confirmed, or the delivery design revised.
- **X8:** overhead measured; agent discoverable from the runtime working directory; no hook recursion; output-format strategy chosen.
- **X3:** the cost of cached tokens against quota understood well enough to set the gate and budgets.
- SPEC v1.0 edits proposed and approved.

## Human Tasks

- Observe the interactive experiments (X1, X3, X5, X8).
- Judge turn-note visibility.
- Approve the SPEC v1.0 edits (required for X-gate sign-off before PR-07).

## Risks

| Risk | Mitigation | Owner |
|---|---|---|
| **`Stop` continue doesn't behave as documented** | X1 with a prototype before production; fallback-only mode as backstop | PR-06 |
| **Lean agent not discoverable from the runtime working directory** | X8 tests install location (`~/.gemini/config/agents/` vs inside plugin) | PR-06 |
| **Lean-agent overhead too high** | X8 measures before reliance | PR-06 |
| **Turn notes disrupt PA or worker sessions** | X1 includes PA sessions; opt-out by role | PR-06, PR-10 |
| **Quota counts cached tokens at full rate** | Milestone-only gate; the fallback carries more load; X3 measures | PR-06 |

## Session Log

*(Empty, ready for use)*
