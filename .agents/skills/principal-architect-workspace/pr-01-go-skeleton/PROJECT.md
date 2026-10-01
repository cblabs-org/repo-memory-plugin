# PR-01: Go Skeleton, Interfaces, Enrolment, CI & Probe Hook

> **Programme:** repo-memory
> **PR:** pr-01-go-skeleton
> **Base branch:** `main`
> **Status:** ✅ Complete
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.9
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Establish the Go module, binary entry point, subcommand wiring, package layout with the enforced core/adapter boundary, the harness interface types, and the `fake` test adapter. Add repository enrolment and the kill switch, so that a global plugin acts only on enrolled repositories. Add CI, the install lifecycle, and the probe hook the experiment spike needs.

## Approvals
- Work order decomposition approved by human on 2026-10-01.

## Work Order Ledger

| # | WO | Branch | Status | Worker |
|---|---|---|---|---|
| 1 | `wo1-module-layout` | `pr-01-go-skeleton/wo1-module-layout` | ☑ Complete | flash |
| 2 | `wo2-interfaces-fake` | `pr-01-go-skeleton/wo2-interfaces-fake` | ☑ Complete | flash |
| 3 | `wo3-config-datadir` | `pr-01-go-skeleton/wo3-config-datadir` | ☑ Complete | flash |
| 4 | `wo4-subcommands-install` | `pr-01-go-skeleton/wo4-subcommands-install` | ☑ Complete | flash |
| 5 | `wo5-enrolment-killswitch` | `pr-01-go-skeleton/wo5-enrolment-killswitch` | ☑ Complete | flash |
| 6 | `wo6-ci-release` | `pr-01-go-skeleton/wo6-ci-release` | ☑ Complete | flash |
| 7 | `wo7-probe-hook` | `pr-01-go-skeleton/wo7-probe-hook` | ☑ Complete | flash |

## Exit Criteria
- [x] `go build ./cmd/repo-memory` succeeds for all 5 targets
- [x] `repo-memory install --harness antigravity` generates correct `hooks.json`, `mcp_config.json` and `sidecar.json` (with `restart_policy` and `agent_permissions`); re-running upgrades in place; `uninstall` removes them; nothing enables the sidecar automatically
- [x] `repo-memory init` enrols a repository with a template-safe repo ID; `leave` unenrols it
- [x] A hook in a non-enrolled workspace returns no-op output within 5 ms of process start; with the kill switch set, hooks no-op and `tick` exits
- [x] The probe hook records payloads for every Antigravity hook event
- [x] Import-graph boundary test passes; core tests run against the `fake` adapter
- [ ] CI green on macOS, Linux and Windows runners (CI workflows written; need to be verified on push)

## Session Log

- **WO1 (module-layout):** Initialised `github.com/cblabs-org/repo-memory-plugin`, wired up 10 cobra subcommand stubs (`hook`, `tick`, `mcp`, `install`, `uninstall`, `search`, `report`, `status`, `reindex`, `doctor`), created cross-compile Makefile for 5 targets, scaffolded 23 core/harness package stubs with `doc.go`, and implemented an AST-based boundary test in `internal/core/boundary_test.go` ensuring no concrete adapters leak into core.
- **WO2 (interfaces-fake):** Defined normalized types (`Event`, `Injection`, `Step`), `Stage` enum, and core harness interfaces (`Harness`, `Capabilities`, `TranscriptSource`, `HookCodec`, `Reporter`, `Housekeeper`) in `internal/harness`. Implemented in-memory fake test adapter with unit test coverage in `internal/harness/fake`. Added core test suite shell in `internal/core/suite_test.go` asserting adapter behavior against fake, and updated `internal/core/boundary_test.go` to permit test files to import the fake adapter while preserving strict adapter prohibitions for production code.
- **WO3 (config-datadir):** Implemented configuration loading for `MachineConfig` and `RepoConfig`, platform-specific `DataDir` resolution, and idempotent `EnsureStorageLayout` for `.memory/` and `<data-dir>/<repo-id>/`.
- **WO4 (subcommands-install):** Split and wired all 10 Cobra subcommands in `cmd/repo-memory/`, implemented Antigravity plugin installation and uninstallation with `--harness` and `--purge` flags, packaged JSON templates for `hooks.json`, `mcp_config.json`, `sidecar.json`, and `plugin.json` in `packaging/antigravity/` with Windows path escaping, enforced that sidecars are never auto-enabled, and provided unit test coverage across template generation and installation lifecycles.
- **WO5 (enrolment-killswitch):** Implemented `init`, `leave --purge`, `enable`/`disable --repo` subcommands. Registry at `<data-dir>/registry.json` with repo ID = `sha256(root commit : canonical origin URL)`. Kill switch via `REPO_MEMORY_DISABLED=1` and `enabled: false` in machine config. Hook framework checks kill switch → enrolment → fast no-op for non-enrolled workspaces.
- **WO6 (ci-release):** GitHub Actions CI workflow (`ci.yml`) with matrix build across macOS, Linux, and Windows. Release workflow (`release.yml`) triggered by tags, building all 5 targets with version stamp injection via ldflags. Makefile `release` target.
- **WO7 (probe-hook):** `repo-memory hook antigravity <event> --probe` records full hook payload plus transcript file-set snapshot to `<data-dir>/<repo-id>/probes/`. Configurable `PreToolUse` probe decision (allow/deny/ask with reason) via `--probe-decision` and `--probe-reason`.

