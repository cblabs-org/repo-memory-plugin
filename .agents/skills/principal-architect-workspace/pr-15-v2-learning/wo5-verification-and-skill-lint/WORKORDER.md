# Work Order 5: Skill Lint, Doctor Integration & Full Verification

> **Project:** [pr-15-v2-learning](../PROJECT.md)
> **Branch:** `pr-15-v2-learning/wo5-verification-and-skill-lint`
> **Base:** `pr-15-v2-learning/wo4-coverage-gaps-short-term-recall-and-spend-guards`
> **Complexity:** `standard`
> **Status:** ✅ Complete

---

## 1. Objective

Implement deterministic skill regeneration lint (`CheckGraduatedSkillsLint`) in `internal/core/skills/graduate.go` and wire it into `repo-memory doctor` (`cmd/repo-memory/cmd_doctor.go`), upgrade `repo-memory status` (`cmd/repo-memory/cmd_status.go`) and `repo-memory report` (`cmd/repo-memory/cmd_report.go`) from stubs to real `v2.2` repository status/metrics commands, run full cross-platform and race-detector verification across all packages, and rebuild `~/.local/bin/repo-memory`.

---

## 2. Context & Architectural Constraints

1. **Skill Lint (`SPEC.md` §11.4 & Addendum A §10 A7):**
   - `repo-memory doctor` verifies that every `.agents/skills/rm-*/SKILL.md` carrying `generated_by: repo-memory` matches deterministic regeneration from `.memory/ops/` (and local ops / index) and still qualifies for skill graduation.
   - Hand-authored skills in `.agents/skills/` without `generated_by: repo-memory` are never flagged by `CheckGraduatedSkillsLint`.
   - Repositories without any `.agents/skills/rm-*/SKILL.md` files on disk produce zero lint warnings so `RunDoctor` remains clean by default.
2. **Real `repo-memory status` & `repo-memory report` Commands:**
   - Replace the `"status subcommand stub"` and `"report subcommand stub"` placeholders in `cmd/repo-memory/cmd_status.go` and `cmd/repo-memory/cmd_report.go` with real `v2.2` implementations (`--repo`, `--json`) that report enrollment state, the A6 freshness line (`coverage.FormatFreshnessHeader`), projected dossier/entry counts, graduated skill count (`skills.SyncGraduatedSkills(..., true)`), and coverage gaps (`coverage.IdentifyCoverageGaps`).
   - Both commands must write to `cmd.OutOrStdout()` and succeed (`return nil`) even when invoked in a non-enrolled directory (so `TestSubcommandsExecution` in `cmd/repo-memory/install_test.go` passes).
3. **External GitHub Hygiene:**
   - Never include any proprietary internal identifiers, internal issue/changelist prefixes, internal domain emails, or absolute local machine home paths in any repository files.

---

## 3. Atomic Implementation Checklist

### Task 1: Implement `CheckGraduatedSkillsLint` (`internal/core/skills/graduate.go`)
- [x] In `internal/core/skills/graduate.go` (after line `614`), add:
  ```go
  // CheckGraduatedSkillsLint verifies that every .agents/skills/rm-*/SKILL.md carrying
  // `generated_by: repo-memory` still qualifies for graduation and matches deterministic
  // regeneration from .memory/ops/. Hand-authored skills without `generated_by: repo-memory`
  // are ignored.
  func CheckGraduatedSkillsLint(ctx context.Context, repoRoot, dataDir, repoID string) ([]string, error)
  ```
- [x] Implementation details for `CheckGraduatedSkillsLint`:
  1. If `strings.TrimSpace(repoRoot) == ""` or `.agents/skills` directory does not exist on disk (`os.ReadDir` returns error), return `nil, nil`.
  2. Scan `.agents/skills/` for subdirectories starting with `"rm-"` that contain a `SKILL.md` file with `IsGeneratedByRepoMemory(content) == true`. If no such generated `rm-*/SKILL.md` exists on disk, return `nil, nil`.
  3. Load active procedural entries via `loadAllProceduralEntriesForSync(ctx, repoRoot, dataDir, repoID)` and `outcomes, _ := LoadOutcomeState(dataDir, repoID)`.
  4. Build `qualifyingBySlug := make(map[string]string)` mapping `slug -> expectedContent` for every entry where `QualifiesForSkill(entry, outcomes)` returns `true` (`slug, content := RenderSkillMarkdown(entry)`).
  5. For each on-disk `.agents/skills/rm-*/SKILL.md` with `IsGeneratedByRepoMemory(existingContent) == true`:
     - `relSkillPath := filepath.ToSlash(filepath.Join(".agents", "skills", dirName, "SKILL.md"))`
     - If `expectedContent, ok := qualifyingBySlug[dirName]; !ok`:
       append warning: `fmt.Sprintf("Graduated skill %s is stale or its source entry no longer qualifies; run 'repo-memory promote-skills'.", relSkillPath)`
     - Else if `existingContent != expectedContent`:
       append warning: `fmt.Sprintf("Graduated skill %s is out of sync with .memory/ops/; run 'repo-memory promote-skills'.", relSkillPath)`
  6. Sort warnings lexicographically (`sort.Strings(warnings)`) and return `warnings, nil`.

### Task 2: Wire `CheckGraduatedSkillsLint` into `RunDoctor` (`cmd/repo-memory/cmd_doctor.go`)
- [x] In `cmd/repo-memory/cmd_doctor.go` (`RunDoctor`, lines `88`–`100`), import `"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"` and after the `GEMINI.md` check (when `hasMemoryDir` is true):
  - Resolve `dataDir := config.DataDir()` and `repoID` (via `config.FindRegistryEntry(dataDir, repoRoot)` or `gitops.GenerateRepoID(repoRoot)`).
  - Call `skillWarnings, err := skills.CheckGraduatedSkillsLint(cmd.Context(), repoRoot, dataDir, repoID)`.
  - If `err == nil`:
    - For each `w` in `skillWarnings`:
      `fmt.Fprintf(cmd.OutOrStdout(), "⚠ %s\n", w)`

### Task 3: Upgrade `cmd/repo-memory/cmd_status.go` and `cmd/repo-memory/cmd_report.go`
- [x] Update `cmd/repo-memory/cmd_status.go` (`status [repo-dir]`, flags `--repo`, `--json`) to inspect `repoRoot` and `dataDir`:
  - Determine `enrolled := config.IsEnrolledFast(repoRoot)` and `freshness := coverage.FormatFreshnessHeader(repoRoot, dataDir, repoID)`.
  - Count active dossiers and active entries from `.memory/ops` (if present) and qualified graduated skills via `skills.SyncGraduatedSkills(cmd.Context(), repoRoot, dataDir, repoID, true)`.
  - Write human-readable status (or JSON if `--json`) to `cmd.OutOrStdout()`, never returning an error if `.memory/` is absent.
- [x] Update `cmd/repo-memory/cmd_report.go` (`report [repo-dir]`, flags `--repo`, `--json`) to output a `v2.2` repository memory report including:
  - Enrollment & freshness status (`coverage.FormatFreshnessHeader`),
  - Dossier count, active entry count by cognitive kind (`semantic`, `procedural`, `episodic`),
  - Graduated skills (`skills.SyncGraduatedSkills(..., true)`),
  - Coverage scores and gap lines (`coverage.ComputeCoverage`, `coverage.IdentifyCoverageGaps`, `coverage.FormatGapLines`).
  - Write to `cmd.OutOrStdout()` and succeed cleanly even in an unenrolled temp directory.

### Task 4: Add Unit & Integration Tests (`internal/core/skills/graduate_test.go` & `cmd/repo-memory/enrolment_test.go`)
- [x] In `internal/core/skills/graduate_test.go`, add `TestCheckGraduatedSkillsLint`:
  - Verify clean return (`len(warnings) == 0`) when `.agents/skills/rm-<slug>/SKILL.md` matches `SyncGraduatedSkills`.
  - Verify out-of-sync warning when `.agents/skills/rm-<slug>/SKILL.md` body is manually modified while keeping `generated_by: repo-memory`.
  - Verify stale warning when `.agents/skills/rm-stale/SKILL.md` has `generated_by: repo-memory` for a retired/non-existent entry.
  - Verify hand-authored `.agents/skills/rm-custom/SKILL.md` without `generated_by: repo-memory` produces zero warnings.
- [x] In `cmd/repo-memory/enrolment_test.go`, extend `TestDoctor_Checks` (or add `TestDoctor_GraduatedSkillsLint`) to verify `RunDoctor` prints `⚠ Graduated skill .agents/skills/rm-...` when a generated skill file is out of sync, and prints nothing once `SyncGraduatedSkills` restores it.

### Task 5: Full Build, Race Test & Binary Rebuild Verification
- [x] Run `go build ./...`
- [x] Run `go vet ./...`
- [x] Run `GOOS=windows go build ./...`
- [x] Run `go test -race ./cmd/... ./internal/... ./packaging/...`
- [x] Rebuild the local binary: `go build -o ~/.local/bin/repo-memory ./cmd/repo-memory`

---

## 4. Verification Criteria

1. `go build ./...`, `go vet ./...`, and `GOOS=windows go build ./...` succeed with zero warnings/errors.
2. `go test -race ./cmd/... ./internal/... ./packaging/...` passes 100%.
3. `~/.local/bin/repo-memory doctor .` and `~/.local/bin/repo-memory status .` execute cleanly.
4. `git status -s` is clean after committing.

---

## Completion Report

- **Status:** ✅ Complete
- **Implementation Commit:** `4e941ce`
- **Summary of Changes:**
  - Implemented `CheckGraduatedSkillsLint` in `internal/core/skills/graduate.go` to deterministically verify `.agents/skills/rm-*/SKILL.md` files carrying `generated_by: repo-memory` against projected procedural recipes and outcomes while ignoring hand-authored skills.
  - Wired `CheckGraduatedSkillsLint` into `RunDoctor` (`cmd/repo-memory/cmd_doctor.go`) when `.memory` is present.
  - Added `FormatFreshnessHeader`, `IdentifyCoverageGaps`, and `FormatGapLines` in `internal/core/coverage/coverage.go`, and upgraded `cmd/repo-memory/cmd_status.go` and `cmd/repo-memory/cmd_report.go` from stubs to full `v2.2` repository status and coverage/metrics commands (`--repo`, `--json`).
  - Added unit and integration tests (`TestCheckGraduatedSkillsLint` in `internal/core/skills/graduate_test.go`, `TestDoctor_GraduatedSkillsLint` in `cmd/repo-memory/enrolment_test.go`).
  - Verified `go build ./...`, `go vet ./cmd/... ./internal/... ./packaging/...`, `GOOS=windows go build ./...`, `go test -race ./cmd/... ./internal/... ./packaging/...`, and rebuilt `~/.local/bin/repo-memory`.
