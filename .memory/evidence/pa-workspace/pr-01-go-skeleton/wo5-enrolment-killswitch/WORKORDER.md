---
name: pr-01-go-skeleton-wo5
description: Handoff instructions for Work Order 5 - Enrolment & Kill Switch
complexity: standard
---

# WORKORDER: WO5 - Enrolment & Kill Switch

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement repository enrolment (`init`, `leave`), the global and repo-specific kill switch (`enable`, `disable`), and the sub-5ms fast-path hook validation that bypasses execution if disabled or not enrolled.

## Strict Boundary Box
- DO NOT implement the logic of specific hook events (e.g. `PreToolUse`, `PreInvocation`). Only the validation layer wrapping them that immediately returns the harness's no-op output (`{}`) if validation fails.
- DO NOT implement the SQLite databases or pipeline stages.

## Tooling Constraints
- Standard Go tooling: `make build`, `go test`.

## Line-Item Instructions
- [x] Implement `init` subcommand in `cmd/repo-memory/cmd_init.go`:
  - Create `.memory/` directory and `.memory/.gitignore` (ignoring `.cache/`).
  - Append the MAP include line (`@[memory map](.memory/views/MAP.md)`) to `GEMINI.md` if it doesn't already exist. Create `GEMINI.md` if missing.
  - Generate the Repo ID: `sha256(root commit + ":" + canonical origin URL)` falling back to main worktree real path if no remote URL exists. Warn the user if `.memory/` was inherited from a template (i.e. directory exists before `init`).
  - Add to `<data-dir>/registry.json` (format: map of workspace paths to `{RepoID, RootPath}`).
- [x] Implement `leave [--purge]` subcommand in `cmd/repo-memory/cmd_leave.go`:
  - Removes the current repository's entry from `<data-dir>/registry.json`.
  - If `--purge` is passed, deletes `<data-dir>/<repo-id>/`.
- [x] Implement kill switch toggles in `enable`/`disable [--repo]` subcommands:
  - Modifies `<data-dir>/config.yaml` to set `enabled: true/false`. If `--repo` is passed, only enable/disable it in `.memory/config.yaml`.
- [x] Implement the `hook` entry point fast-path validation (e.g. in `cmd_hook.go` or `internal/harness`):
  - Check `REPO_MEMORY_DISABLED=1` environment variable. If set, return no-op.
  - Check `enabled` from machine and repo config.
  - Check if the current workspace path is in `registry.json`.
  - If any check fails (not enabled, not enrolled), return empty JSON `{}` immediately to stdout (budget < 5ms process start-to-exit).
- [x] Implement unit tests validating Repo ID generation, registry operations, kill switch config changes, and the fast-path hook validation.

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
- `internal/core/gitops/repoid.go`: Implemented `ComputeRepoID`, `CanonicalURL`, `GetRootCommit`, `GetCanonicalOriginURL`, `GetMainWorktreePath`, `GetWorktreePaths`, `FindRepoRoot`, and `GenerateRepoID`.
- `internal/core/gitops/repoid_test.go`: Added unit tests for canonical URL normalization, root commit extraction, repo ID stability across commits, and worktree real path fallback.
- `internal/core/config/registry.go`: Implemented `RegistryEntry`, `Registry` map type, `LoadRegistry`, `SaveRegistry`, `AddToRegistry`, `RemoveFromRegistry`, and `FindRegistryEntry`. Handles case insensitivity and symlinks/subdirectories.
- `internal/core/config/config.go`: Added `Enabled *bool` and `IsEnabled() bool` to `RepoConfig`, and implemented `SaveMachineConfig`, `SaveRepoConfig`, `SetMachineEnabled`, and `SetRepoEnabled`.
- `internal/core/config/registry_test.go`: Added unit tests for registry JSON serialization, add/find/remove operations, and machine/repo enable/disable functions.
- `internal/harness/fastpath.go`: Implemented `FastPathValidation` checking `REPO_MEMORY_DISABLED=1`, machine config `enabled`, `registry.json` workspace enrolment, and repo config `enabled`.
- `internal/harness/fastpath_test.go`: Added tests verifying all four validation layers and sub-5ms latency benchmark (~17 µs measured).
- `cmd/repo-memory/cmd_init.go`: Implemented `repo-memory init` with pre-existing template directory warning, layout provisioning, MAP include in `GEMINI.md`, Repo ID generation, data dir provisioning, and `registry.json` enrolment.
- `cmd/repo-memory/cmd_leave.go`: Implemented `repo-memory leave [--purge]` to unenroll from `registry.json` and optionally remove `<data-dir>/<repo-id>/` while leaving `.memory/` intact.
- `cmd/repo-memory/cmd_enable.go` & `cmd/repo-memory/cmd_disable.go`: Implemented global and repository-specific kill switch toggles (`--repo`).
- `cmd/repo-memory/cmd_hook.go`: Updated hook command to run fast-path validation and immediately emit `{}` upon failure.
- `cmd/repo-memory/main.go`: Registered `initCmd`, `leaveCmd`, `enableCmd`, and `disableCmd` in `rootCmd`.
- `cmd/repo-memory/enrolment_test.go`: Added end-to-end integration tests for `init`, `leave`, `leave --purge`, `enable`, `disable`, and hook fast-path behavior.

### Decisions Made
- Symlinks & Subdirectory Enrolment: Registered paths are indexed both as raw paths and evaluated symlink paths (e.g. `/var` vs `/private/var` on macOS). `FindRegistryEntry` also supports subdirectories within enrolled repositories so hooks invoked from subfolders match correctly.
- Origin URL Canonicalization: Normalized SSH (`git@host:owner/repo.git`), schemes (`ssh://`, `git://`), trailing slashes, and `.git` suffixes to `https://host/owner/repo` so all clone protocols produce matching Repo IDs.

### Dead Ends & Warnings
(none)

### Out-of-Scope Findings
(none)

### Blockers
(none)

