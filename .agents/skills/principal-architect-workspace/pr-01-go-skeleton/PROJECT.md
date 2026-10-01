# PR-01: Go Skeleton, Interfaces, Enrolment, CI & Probe Hook

> **Programme:** repo-memory
> **PR:** pr-01-go-skeleton
> **Base branch:** `main`
> **Status:** In Progress
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.9
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Establish the Go module, binary entry point, subcommand wiring, package layout with the enforced core/adapter boundary, the harness interface types, and the `fake` test adapter. Add repository enrolment and the kill switch, so that a global plugin acts only on enrolled repositories. Add CI, the install lifecycle, and the probe hook the experiment spike needs.

## Approvals
- Work order decomposition approved by human on 2026-10-01.

## Work Order Ledger

| # | WO | Branch | Status | Worker |
|---|---|---|---|---|
| 1 | `wo1-module-layout` | `pr-01-go-skeleton/wo1-module-layout` | ☐ Pending | — |
| 2 | `wo2-interfaces-fake` | `pr-01-go-skeleton/wo2-interfaces-fake` | ☐ Pending | — |
| 3 | `wo3-config-datadir` | `pr-01-go-skeleton/wo3-config-datadir` | ☐ Pending | — |
| 4 | `wo4-subcommands-install` | `pr-01-go-skeleton/wo4-subcommands-install` | ☐ Pending | — |
| 5 | `wo5-enrolment-killswitch` | `pr-01-go-skeleton/wo5-enrolment-killswitch` | ☐ Pending | — |
| 6 | `wo6-ci-release` | `pr-01-go-skeleton/wo6-ci-release` | ☐ Pending | — |
| 7 | `wo7-probe-hook` | `pr-01-go-skeleton/wo7-probe-hook` | ☐ Pending | — |

## Exit Criteria
- `go build ./cmd/repo-memory` succeeds for all 5 targets
- `repo-memory install --harness antigravity` generates correct `hooks.json`, `mcp_config.json` and `sidecar.json` (with `restart_policy` and `agent_permissions`); re-running upgrades in place; `uninstall` removes them; nothing enables the sidecar automatically
- `repo-memory init` enrols a repository with a template-safe repo ID; `leave` unenrols it
- A hook in a non-enrolled workspace returns no-op output within 5 ms of process start; with the kill switch set, hooks no-op and `tick` exits
- The probe hook records payloads for every Antigravity hook event
- Import-graph boundary test passes; core tests run against the `fake` adapter
- CI green on macOS, Linux and Windows runners

## Session Log
