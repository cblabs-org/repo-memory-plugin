---
name: pr-06b-v2-plugin-split-wo3
description: Rename init→enroll, leave→unenroll, write .agents/plugins.json, update doctor
complexity: standard
---

# WO3: Enrollment Flow

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Rename the enrollment commands (`init`→`enroll`, `leave`→`unenroll`), make `enroll` write `.agents/plugins.json` referencing the workspace bundle, and update `doctor` to detect missing plugins for new clones.

## Strict Boundary Box
- **Modify:** `cmd/repo-memory/cmd_init.go`, `cmd/repo-memory/cmd_leave.go`, `cmd/repo-memory/cmd_enable.go`, `cmd/repo-memory/cmd_disable.go`, `cmd/repo-memory/cmd_doctor.go`, `cmd/repo-memory/main.go` (command registration)
- **DO NOT** modify `cmd_install.go` (done in WO2)
- **DO NOT** modify hook handlers, pipeline, evidence, telemetry
- **DO NOT** modify the v2 spike prototype code

## Context

### Phase 0 results
Read `docs/experiments/phase0-plugin-split.md` — it documents whether `~` expansion works in `.agents/plugins.json`. If `~` doesn't work, use absolute paths generated at enrollment time.

### Shared convention (§3 from pattern-refinements)
Commands should be: `install`/`uninstall` (machine scope), `enroll`/`unenroll` (repo scope), `enable`/`disable` (toggle). This aligns with the bug-fixer-loop-plugin conventions.

### .agents/plugins.json format
```json
{
  "entries": [
    { "path": "~/.gemini/config/plugins/repo-memory-daemon/workspace-plugin" }
  ]
}
```
If `~` expansion doesn't work (per Phase 0 results), use the absolute path from the install location.

## Line-Item Instructions

### Rename init → enroll

- [ ] **Rename `cmd_init.go` → `cmd_enroll.go`** (git mv)
- [ ] **Update the cobra command:**
  - `Use: "enroll [path]"` (was `"init [path]"`)
  - `Short: "Enroll a repository for memory tracking"`
  - Add `Aliases: []string{"init"}` so the old command still works
- [ ] **Rename `RunInit` → `RunEnroll`** — update all callers

### Rename leave → unenroll

- [ ] **Rename `cmd_leave.go` → `cmd_unenroll.go`** (git mv)
- [ ] **Update the cobra command:**
  - `Use: "unenroll [path]"` (was `"leave [path]"`)
  - `Short: "Remove a repository from memory tracking"`
  - Add `Aliases: []string{"leave"}` so the old command still works
- [ ] **Rename `RunLeave` → `RunUnenroll`** — update all callers

### Write .agents/plugins.json on enroll

- [ ] **In `RunEnroll`**, after creating `.memory/` layout and `.enrolled` marker:
  - Resolve the workspace plugin path (use `ResolveWorkspacePluginDir` from WO2, or compute from `ResolveAntigravityPluginDir`)
  - Determine whether to use `~` or absolute path (based on Phase 0 results)
  - Write `.agents/plugins.json` with the entry
  - If `.agents/plugins.json` already exists, read it, append the entry if not already present, and write back
  - **Add `plugins.json` to `.agents/.gitignore`** (create `.agents/.gitignore` if it doesn't exist). The file is machine-specific and should NOT be committed.

- [ ] **Print:** `"Workspace plugin enabled for this repository via .agents/plugins.json"`

### Remove .agents/plugins.json on unenroll

- [ ] **In `RunUnenroll`**, before removing `.enrolled` marker:
  - Read `.agents/plugins.json`
  - Remove the repo-memory workspace-plugin entry
  - If no entries remain, delete the file
  - If other entries remain, write back without the repo-memory entry

### Update enable/disable

- [ ] **`cmd_enable.go`:** Add the `.agents/plugins.json` entry back (same as enroll's write logic)
- [ ] **`cmd_disable.go`:** Remove the `.agents/plugins.json` entry (same as unenroll's removal logic), but keep the registry entry and `.enrolled` marker

### Update doctor

- [ ] **In `cmd_doctor.go`**, add a check:
  - If `.memory/` exists and `.memory/.enrolled` exists, but `.agents/plugins.json` doesn't reference the workspace plugin → warn:
    ```
    ⚠ repo-memory is enrolled but workspace plugin is not configured.
      Run: repo-memory install --harness antigravity && repo-memory enroll .
      (Or if already installed: repo-memory enroll .)
    ```
  - If the workspace plugin path in `.agents/plugins.json` doesn't exist on disk → warn:
    ```
    ⚠ Workspace plugin not found at <path>.
      Run: repo-memory install --harness antigravity
    ```

### Update main.go command registration

- [ ] **In `main.go`:** Update the root command's subcommand registration if the variable names changed (e.g., `initCmd` → `enrollCmd`, `leaveCmd` → `unenrollCmd`)

## Verification Requirement
- `go build ./...` passes
- `go vet ./...` passes
- `go test ./cmd/repo-memory/...` passes
- `repo-memory enroll .` writes `.agents/plugins.json` correctly
- `repo-memory init .` still works (alias)
- `repo-memory unenroll .` removes the entry
- `repo-memory leave .` still works (alias)
- `.agents/plugins.json` is in `.agents/.gitignore`

## Issue Surfacing
If you discover a defect, upstream bug, or concern outside your boundary box, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

### Status
(pending)

### Changes Made
(pending)

### Decisions Made
(pending)

### Dead Ends & Warnings
(pending)

### Out-of-Scope Findings
(pending)

### Blockers
(pending)
