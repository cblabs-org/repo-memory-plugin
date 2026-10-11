# PR-14: PA Integration (Cross-Repo)

> **Programme:** repo-memory
> **PR:** pr-14-v2-pa-integration
> **Base branch:** PR-13-v2-learning
> **Status:** ☐ Planned
> **SPEC:** [SPEC.md](../../../SPEC.md) §13
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Integrate `repo-memory` with the Principal Architect (PA) workflow across repositories. Provide generated episode summaries (`repo-memory summary <session>`) and workorder briefs (`repo-memory brief --workorder <path>`). Raise a change request through `mm-skills/principal-architect` for a compatibility mode where the PA embeds generated briefs and reads generated summaries when a repo is enrolled, with turn-note interaction following the PR-10 role settings. Migrate `esp32-app-foundation` so that `PROJECT.md` holds intent, plan, ledger, and approvals only, while knowledge comes from dossiers and hand-written memory files are retired.

## Design & SPEC Sections

- **v2 Design:** §5.5 (PA reports)
- **SPEC:** §13 (Principal-architect integration), §15.1 (P4 PA integration)

## Dependencies & Gate Preconditions

- **Base branch:** `PR-13-v2-learning`
- **Dependencies & Preconditions:**
  - PR-13-v2-learning complete (close conditions, gardening, coverage, elicitation, session metrics, replay tuning).
  - Confirmatory X6 gate passed at PR-12.
  - Role-based turn-note settings defined in PR-10 (ensuring turn-note interaction does not disrupt PA or worker loops).
- **Repositories in scope:**
  - `repo-memory-plugin`
  - `mm-skills/principal-architect`
  - `esp32-app-foundation`

## Work Order Ledger

| # | WO | Repository | Scope | Complexity | Status |
|---|---|---|---|---|---|
| 1 | `wo1-episode-summaries` | repo-memory-plugin | `repo-memory summary <session>` (≤ 1.5k tokens) | standard | ☐ Planned |
| 2 | `wo2-workorder-briefs` | repo-memory-plugin | `repo-memory brief --workorder <path>` | standard | ☐ Planned |
| 3 | `wo3-pa-change-request` | mm-skills/principal-architect | Compatibility mode through that repo's process: when the repo is enrolled, the PA embeds generated briefs and reads generated summaries; turn-note interaction per the PR-10 role settings. | architect | ☐ Planned |
| 4 | `wo4-esp32-migration` | esp32-app-foundation | After the PA change ships: PROJECT.md holds intent, plan, ledger and approvals only; knowledge comes from dossiers. | standard | ☐ Planned |

## Exit Criteria

- Compatibility mode merged and verified on a non-enrolled project.
- **No hand-written memory files** for a full project cycle in esp32-app-foundation.
- B4 shows lower total cost than PA hand-written memory.

## Human Tasks

- Approve the PA compatibility-mode change (needed for PR-14 exit).

## Risks

| Risk | Mitigation | Owner |
|---|---|---|
| PA change disrupts other projects | Compatibility mode; through the PA repo's process | PR-14 |

## Session Log

*(Empty, ready for use)*
