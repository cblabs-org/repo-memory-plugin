---
name: pr-01-go-skeleton-wo3
description: Handoff instructions for Work Order 3 - Config & Data Directory
complexity: standard
---

# WORKORDER: WO3 - Config & Data Directory

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement configuration loading for the machine and the repository. Resolve the platform-appropriate data directory and implement an idempotent setup for the storage layout.

## Strict Boundary Box
- DO NOT implement parsing of all specific config structures yet beyond what is necessary (enabled, housekeeping.executor, reports.surface, tick.trigger, budgets).
- DO NOT implement the SQLite databases or FTS index logic. Just create the directory structures.

## Tooling Constraints
- Standard Go tooling: `make build`, `go test`.

## Line-Item Instructions
- [x] In `internal/core/config/datadir.go`, implement a function `DataDir() string` that returns the platform-appropriate data directory:
  - macOS: `~/Library/Application Support/repo-memory`
  - Linux: `~/.local/share/repo-memory`
  - Windows: `%LOCALAPPDATA%\repo-memory` (or `AppData\Local\repo-memory`)
- [x] In `internal/core/config/config.go`, define `MachineConfig` (fields: enabled, housekeeping_executor, reports_surface, tick_trigger, budgets) and `RepoConfig` (subsystems definition). Use appropriate struct tags for YAML or JSON.
- [x] In `internal/core/config/config.go`, implement `LoadMachineConfig(dataDir string)` and `LoadRepoConfig(repoRoot string)`. If the `config.yaml` file doesn't exist, they should return a default or empty configuration without failing.
- [x] In `internal/core/config/layout.go`, implement `EnsureStorageLayout(dataDir, repoID string)` that creates the following directories (it must be idempotent):
  - `.memory/` in the repo root (and `<repoRoot>/.memory/claims`, `<repoRoot>/.memory/events/reviews`, `<repoRoot>/.memory/events/answers`, `<repoRoot>/.memory/views/index`, `<repoRoot>/.memory/views/subsystems`, `<repoRoot>/.memory/.cache`)
  - `<data-dir>/<repo-id>/` subdirectories: `spool`, `queue`, `runs`, `episodes`, `pending`, `alerts`, `metrics`, `logs`, `state`.
- [x] Write unit tests for the `config` package (e.g. testing `DataDir()` path generation and `EnsureStorageLayout` idempotency using `t.TempDir()`).

## Verification Requirement
- Verify `make build` compiles correctly.
- Verify `go test ./...` passes.
- **CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report

### Status
complete

### Changes Made
- `internal/core/config/datadir.go` — implemented `DataDir()` returning platform-appropriate paths (`~/Library/Application Support/repo-memory` on macOS, `~/.local/share/repo-memory` on Linux, `%LOCALAPPDATA%\repo-memory` on Windows) with `REPO_MEMORY_DATA_DIR` environment override support.
- `internal/core/config/config.go` — defined `MachineConfig` (enabled, housekeeping_executor, reports_surface, tick_trigger, budgets), `RepoConfig` (subsystems definition, budgets), and loaders `LoadMachineConfig(dataDir)` and `LoadRepoConfig(repoRoot)` with default fallbacks when `config.yaml` is absent.
- `internal/core/config/layout.go` — implemented idempotent directory structure creators `EnsureStorageLayout(dataDir, repoID string, repoRoot ...string)`, `EnsureDataDirLayout(dataDir, repoID)`, and `EnsureRepoLayout(repoRoot)`.
- `internal/core/config/config_test.go` — unit test suite covering cross-platform data directory resolution, config defaults, YAML variations (flat, dotted, nested), JSON serialization, and storage layout idempotency.
- `go.mod`, `go.sum` — added `gopkg.in/yaml.v3`.

### Decisions Made
- `EnsureStorageLayout`: Implemented as `EnsureStorageLayout(dataDir, repoID string, repoRoot ...string) error`, plus standalone `EnsureDataDirLayout` and `EnsureRepoLayout` helpers. This preserves the 2-argument signature specified in WO3 line-item instructions while allowing caller to optionally pass `repoRoot` for the repository `.memory` structure.
- `MachineConfig` YAML Unmarshaling: Implemented custom unmarshaler supporting flat snake_case (`housekeeping_executor`), dotted keys (`housekeeping.executor`), and nested maps (`housekeeping: { executor: ... }`) to seamlessly accept all representations mentioned in SPEC.md.

### Dead Ends & Warnings
- When testing Windows and Linux data directory resolution on a Darwin host, path separators in unit tests must be handled with platform awareness or mock functions. Used `dataDirForOS` with injected env and home getters to allow cross-platform testing on any host OS.

### Out-of-Scope Findings
- None.

### Blockers
(none)
