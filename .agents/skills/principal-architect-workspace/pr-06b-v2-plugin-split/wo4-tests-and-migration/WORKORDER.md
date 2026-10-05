---
name: pr-06b-v2-plugin-split-wo4
description: Update tests, migrate live install, final verification
complexity: standard
---

# WO4: Tests and Migration

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Update all test fixtures referencing the old plugin path, add an integration test for the full install→enroll→unenroll flow, migrate the live installed plugin on the dev machine, and verify everything works end-to-end.

## Strict Boundary Box
- **Modify:** Test files referencing old plugin paths, `.gitignore` if needed
- **Create:** Integration test for install→enroll→unenroll flow
- **Run:** Live migration on the dev machine
- **DO NOT** modify production code (that's WO2 and WO3)
- **DO NOT** modify hook handlers, pipeline, evidence, telemetry

## Context

### What changed in WO2 and WO3
- Plugin directory: `~/.gemini/config/plugins/repo-memory/` → `~/.gemini/config/plugins/repo-memory-daemon/`
- Workspace plugin at: `repo-memory-daemon/workspace-plugin/repo-memory/`
- `init` → `enroll` (alias preserved), `leave` → `unenroll` (alias preserved)
- `.agents/plugins.json` written on enroll, removed on unenroll
- `agent_permissions` removed from sidecar.json

### Test files to check
```
cmd/repo-memory/cmd_search_test.go
cmd/repo-memory/cmd_snapshot_test.go
cmd/repo-memory/cmd_usage_test.go
cmd/repo-memory/fastpath_test.go
packaging/antigravity/templates_test.go
internal/harness/antigravity/hooks_test.go
```

## Line-Item Instructions

### Fix test fixtures

- [ ] **`grep -r 'plugins/repo-memory"' --include='*.go' .`** — find all hardcoded references to the old plugin path
- [ ] **`grep -r 'repo-memory' --include='*.go' . | grep -i 'plugin'`** — broader search
- [ ] **Update each reference** to use `repo-memory-daemon` where it refers to the global plugin directory
- [ ] **Update `templates_test.go`** for renamed functions (`GeneratePluginJSON` → `GenerateWorkspacePluginJSON`, etc.)

### Add integration test

- [ ] **Create `cmd/repo-memory/cmd_enroll_test.go`:**
  ```go
  func TestEnrollUnenrollFlow(t *testing.T) {
      // 1. Create temp dirs for plugin and repo
      // 2. Set REPO_MEMORY_PLUGIN_DIR to temp plugin dir
      // 3. Run install (creates daemon + workspace-plugin layout)
      // 4. Verify directory structure matches target
      // 5. Run enroll on temp repo
      // 6. Verify .agents/plugins.json exists and points to workspace-plugin
      // 7. Verify .agents/.gitignore contains plugins.json
      // 8. Run unenroll
      // 9. Verify .agents/plugins.json entry removed
      // 10. Run install again (idempotent)
      // 11. Verify layout unchanged
  }
  ```

- [ ] **Test migration path:** If old `repo-memory/` exists alongside new install target, verify it's backed up

### Verify full test suite

- [ ] **`go build ./...`** — must pass
- [ ] **`go vet ./...`** — must pass
- [ ] **`go test ./...`** — must pass (except pre-existing failures)
- [ ] **Document any test that needed fixing** and why

### Live migration

- [ ] **Check current live install:**
  ```bash
  ls ~/.gemini/config/plugins/ | grep repo-memory
  ls ~/.gemini/config/plugins/repo-memory/
  ```

- [ ] **Build and run the new install:**
  ```bash
  go build -o repo-memory ./cmd/repo-memory
  ./repo-memory install --harness antigravity
  ```

- [ ] **Verify daemon layout:**
  ```bash
  ls ~/.gemini/config/plugins/repo-memory-daemon/
  cat ~/.gemini/config/plugins/repo-memory-daemon/plugin.json
  cat ~/.gemini/config/plugins/repo-memory-daemon/sidecars/tick/sidecar.json
  # Verify NO agent_permissions
  ```

- [ ] **Verify workspace bundle:**
  ```bash
  ls ~/.gemini/config/plugins/repo-memory-daemon/workspace-plugin/repo-memory/
  cat ~/.gemini/config/plugins/repo-memory-daemon/workspace-plugin/repo-memory/plugin.json
  ```

- [ ] **Enroll this repo:**
  ```bash
  ./repo-memory enroll .
  cat .agents/plugins.json
  cat .agents/.gitignore  # should contain plugins.json
  ```

- [ ] **Verify old plugin backed up:**
  ```bash
  ls ~/.gemini/config/plugins/ | grep repo-memory
  # Should see: repo-memory-daemon/ and repo-memory.bak-YYYYMMDD/ (if old existed)
  ```

- [ ] **Clean up old backup** after verifying everything works

### Commit

- [ ] Commit all changes with: `fix: update tests and fixtures for split plugin architecture`

## Verification Requirement
- `go build ./...`, `go vet ./...`, `go test ./...` all pass
- Live plugin migrated and working
- Integration test passes
- Old plugin path references eliminated

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
