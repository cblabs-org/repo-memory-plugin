/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package seed_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
)

func TestParseCompletionReport(t *testing.T) {
	content := `
## Completion Report

### Status
complete

### Changes Made
- ` + "`internal/core/seed/parser.go`" + ` — added new feature
- ` + "`another/file.go`" + ` - fixed bug

### Decisions Made
- Used strategy X

### Dead Ends & Warnings
- Tried Y, didn't work

### Out-of-Scope Findings
- File Z needs refactoring

### Blockers
(none)
`
	report, err := seed.ParseCompletionReport(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatalf("expected report, got nil")
	}

	if report.Status != "complete" {
		t.Errorf("got status %q, want 'complete'", report.Status)
	}

	if len(report.Changes) != 2 {
		t.Fatalf("got %d changes, want 2", len(report.Changes))
	}
	if report.Changes[0].Path != "internal/core/seed/parser.go" || report.Changes[0].Description != "`internal/core/seed/parser.go` — added new feature" {
		t.Errorf("got change 0: %+v", report.Changes[0])
	}
	if report.Changes[1].Path != "another/file.go" || report.Changes[1].Description != "`another/file.go` - fixed bug" {
		t.Errorf("got change 1: %+v", report.Changes[1])
	}

	if len(report.Decisions) != 1 || report.Decisions[0] != "Used strategy X" {
		t.Errorf("got decisions: %v", report.Decisions)
	}
	if len(report.DeadEnds) != 1 || report.DeadEnds[0] != "Tried Y, didn't work" {
		t.Errorf("got dead ends: %v", report.DeadEnds)
	}
	if len(report.Findings) != 1 || report.Findings[0] != "File Z needs refactoring" {
		t.Errorf("got findings: %v", report.Findings)
	}

	// Empty content case
	report2, _ := seed.ParseCompletionReport("")
	if report2 != nil {
		t.Errorf("expected nil for empty content, got %+v", report2)
	}
}

func TestReportToOps(t *testing.T) {
	report := &seed.CompletionReport{
		Status: "complete",
		Changes: []seed.Change{
			{Path: "foo/bar.go", Description: "added feature"},
		},
		Decisions: []string{"Use singleton"},
		DeadEnds:  []string{"Global state failed"},
		Findings:  []string{"Refactor soon"},
	}

	resolver := config.NewScopeResolver(nil, nil, nil)
	opsList := seed.ReportToOps(report, resolver, "", time.Time{}, nil)

	if len(opsList) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(opsList))
	}

	var foundDec, foundRul, foundIssue bool

	for _, o := range opsList {
		if o.Source != ops.SourceSeed {
			t.Errorf("expected source seed, got %q", o.Source)
		}
		if len(o.Fields["anchors"].([]string)) != 1 || o.Fields["anchors"].([]string)[0] != "foo/bar.go" {
			t.Errorf("bad anchors in op: %v", o.Fields["anchors"])
		}

		switch o.Fields["entry_type"] {
		case "decision":
			foundDec = true
			if !strings.HasPrefix(o.EntryID, "seed-") {
				t.Errorf("bad dec prefix: %s", o.EntryID)
			}
		case "ruled-out":
			foundRul = true
			if !strings.HasPrefix(o.EntryID, "seed-") {
				t.Errorf("bad rul prefix: %s", o.EntryID)
			}
		case "open-issue":
			foundIssue = true
			if !strings.HasPrefix(o.EntryID, "seed-") {
				t.Errorf("bad issue prefix: %s", o.EntryID)
			}
		}
	}

	if !foundDec || !foundRul || !foundIssue {
		t.Errorf("missing expected op types: dec=%v rul=%v issue=%v", foundDec, foundRul, foundIssue)
	}
}

func TestScanner(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create mock files
	err := os.MkdirAll(filepath.Join(tmpDir, "p1", "w1"), 0755)
	if err != nil { t.Fatal(err) }
	
	validWO := filepath.Join(tmpDir, "p1", "w1", "WORKORDER.md")
	os.WriteFile(validWO, []byte("## Completion Report\n"), 0644)
	
	invalidWO := filepath.Join(tmpDir, "p1", "w2_WORKORDER.md") // not named exactly WORKORDER.md
	os.WriteFile(invalidWO, []byte("## Completion Report\n"), 0644)
	
	err = os.MkdirAll(filepath.Join(tmpDir, "p2", "w1"), 0755)
	if err != nil { t.Fatal(err) }
	noReportWO := filepath.Join(tmpDir, "p2", "w1", "WORKORDER.md")
	os.WriteFile(noReportWO, []byte("Just some text"), 0644)

	files, err := seed.ScanWorkorders(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if filepath.Clean(files[0]) != filepath.Clean(validWO) {
		t.Errorf("expected %s, got %s", validWO, files[0])
	}
}

func TestReportToOps_JunkFiltered(t *testing.T) {
	report := &seed.CompletionReport{
		Changes: []seed.Change{
			{Path: "pkg/core.go", Description: "core change"},
		},
		Decisions: []string{
			"None.",
			"(none)",
			"(none).",
			"Valid decision to keep",
			"n/a",
			"(n/a)",
			"TBD",
		},
		DeadEnds: []string{
			"(pending)",
			"-",
			"None encountered during implementation",
			"Real dead end that failed",
		},
		Findings: []string{
			"Pending",
			"(none)",
			"No issues found",
			"Nothing to report",
			"Important finding about concurrency",
		},
	}

	resolver := config.NewScopeResolver(nil, nil, nil)
	opsList := seed.ReportToOps(report, resolver, "wo1/WORKORDER.md", time.Time{}, nil)

	if len(opsList) != 3 {
		t.Fatalf("expected exactly 3 ops after filtering junk, got %d: %+v", len(opsList), opsList)
	}

	for _, o := range opsList {
		text := o.Fields["text"].(string)
		if text != "Valid decision to keep" &&
			text != "Real dead end that failed" &&
			text != "Important finding about concurrency" {
			t.Errorf("unexpected op text survived junk filter: %q", text)
		}
	}
}

func TestParseCompletionReport_FiltersNonPathTokens(t *testing.T) {
	content := `
## Completion Report

### Status
✅ Complete

### Changes Made
- ` + "`internal/core/seed/parser.go`" + ` — tightened ` + "`*claims.Claim`" + `, ` + "`cand.EpisodeID`" + `, ` + "`Reader.IterateBefore`" + `, and ` + "`!config.IsEnrolledFast(cwd)`" + `
- ` + "`.memory/config.yaml`" + ` — ignored ` + "`*_test.go`" + `, ` + "`<data-dir>/<repo-id>/metrics/*.jsonl`" + `, ` + "`~/.gemini/antigravity/hooks/hooks.json`" + `, and ` + "`modernc.org/sqlite`" + `
- ` + "`SPEC.md`" + ` — documented specification
- ` + "`internal/real/file.go#MyFunc`" + ` and ` + "`SPEC.md#Section`" + ` — accepted symbol anchors while rejecting ` + "`http://example.com/foo`" + ` and ` + "`foo:bar:baz`" + `
`
	report, err := seed.ParseCompletionReport(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatalf("expected report, got nil")
	}

	wantPaths := []string{
		"internal/core/seed/parser.go",
		".memory/config.yaml",
		"SPEC.md",
		"internal/real/file.go#MyFunc",
		"SPEC.md#Section",
	}
	if len(report.Changes) != len(wantPaths) {
		var got []string
		for _, c := range report.Changes {
			got = append(got, c.Path)
		}
		t.Fatalf("got changes %v, want %v", got, wantPaths)
	}
	for i, want := range wantPaths {
		if report.Changes[i].Path != want {
			t.Errorf("change[%d].Path = %q, want %q", i, report.Changes[i].Path, want)
		}
	}
}

func runGitWithDate(t *testing.T, dir string, date string, args ...string) string {
	t.Helper()
	fullArgs := append([]string{"-c", "user.name=test", "-c", "user.email=test@example.com"}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = dir
	if date != "" {
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_DATE="+date,
			"GIT_COMMITTER_DATE="+date,
		)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestScanAndParse_ValidatesRepoPathsAndBeforeCommitTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	runGitWithDate(t, tmpDir, "", "init")

	realFile := filepath.Join(tmpDir, "internal", "real", "file.go")
	if err := os.MkdirAll(filepath.Dir(realFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(realFile, []byte("package real\n"), 0644); err != nil {
		t.Fatal(err)
	}

	woPath := filepath.Join(tmpDir, ".agents", "p1", "wo1", "WORKORDER.md")
	if err := os.MkdirAll(filepath.Dir(woPath), 0755); err != nil {
		t.Fatal(err)
	}
	woContent := `# Work Order 1

## Completion Report

### Status
✅ Complete

### Changes Made
- ` + "`internal/real/file.go`" + ` — real tracked file
- ` + "`internal/real/file.go#MyFunc`" + ` — real tracked file with symbol anchor
- ` + "`internal/nonexistent/ghost.go`" + ` — nonexistent path that looks syntactic
- ` + "`http://example.com/foo`" + ` — URL rejected
- ` + "`foo:bar:baz`" + ` — multi-colon token rejected

### Decisions Made
- Validated paths against repository tree
`
	if err := os.WriteFile(woPath, []byte(woContent), 0644); err != nil {
		t.Fatal(err)
	}

	commit1TimeStr := "2026-01-10T10:00:00Z"
	runGitWithDate(t, tmpDir, commit1TimeStr, "add", ".")
	runGitWithDate(t, tmpDir, commit1TimeStr, "commit", "-m", "commit 1")
	commit1SHA := runGitWithDate(t, tmpDir, "", "rev-parse", "HEAD")

	// Commit 2 modifies WORKORDER.md at a later date so HEAD has a newer commit timestamp
	commit2TimeStr := "2026-06-15T12:00:00Z"
	if err := os.WriteFile(woPath, []byte(woContent+"\n<!-- post-cutoff edit -->\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitWithDate(t, tmpDir, commit2TimeStr, "add", ".")
	runGitWithDate(t, tmpDir, commit2TimeStr, "commit", "-m", "commit 2")

	parsed, err := seed.ScanAndParse(filepath.Join(tmpDir, ".agents"), seed.SeedOptions{
		RepoRoot:     tmpDir,
		BeforeCommit: commit1SHA,
	})
	if err != nil {
		t.Fatalf("ScanAndParse failed: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("got %d parsed workorders, want 1", len(parsed))
	}

	changes := parsed[0].Report.Changes
	if len(changes) != 2 || changes[0].Path != "internal/real/file.go" || changes[1].Path != "internal/real/file.go#MyFunc" {
		t.Fatalf("got changes %+v, want [internal/real/file.go, internal/real/file.go#MyFunc]", changes)
	}

	wantTime, err := time.Parse(time.RFC3339, commit1TimeStr)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed[0].CommitTime.UTC().Equal(wantTime) {
		t.Errorf("got CommitTime %v, want %v (from BeforeCommit, not HEAD)", parsed[0].CommitTime.UTC(), wantTime)
	}
}

func TestParseCompletionReport_SplitAuthorityYAMLFrontmatter(t *testing.T) {
	content := `---
schema_version: 1
wo_id: WO-01.3
pr_id: PR-01
title: "Split-Authority models and document parser"
complexity: architect
status: COMPLETED
allowed_files:
  - daemon/workspace-plugin/principal-architect/scripts/schemas.py
  - tests/test_schemas.py
completion:
  status: COMPLETED
  attempt_num: 1
  receipt_id: "rcpt_4b3b6d967b22"
  commit_sha: "555a90732214"
  files_modified:
    - daemon/workspace-plugin/principal-architect/scripts/schemas.py
    - tests/test_schemas.py
  verification_summary: "29 passed in pytest; ruff check clean."
  out_of_scope_findings:
    - "pa status creates state.db when invoked on unenrolled workspace"
  decisions_made:
    - "Preserved pre-H2 preamble as Section(heading='', body=...) so parse_document/dump_document round-trips losslessly."
  dead_ends:
    - "Splitting on raw ## without multiline regex broke code blocks containing comments."
---

# WO-01.3 — Split-Authority Models

## 1. Role & Objective
Implement schemas.py.

## Completion Report

### 1. Summary of Changes
- Implemented schemas.py and test_schemas.py.
`
	report, err := seed.ParseCompletionReport(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatalf("expected non-nil report for v0.1.0+ Split-Authority WORKORDER.md")
	}
	if report.Status != "COMPLETED" {
		t.Errorf("got status %q, want COMPLETED", report.Status)
	}
	if len(report.Changes) != 2 {
		t.Fatalf("got %d changes, want 2: %+v", len(report.Changes), report.Changes)
	}
	if len(report.Decisions) != 1 || !strings.Contains(report.Decisions[0], "Preserved pre-H2 preamble") {
		t.Errorf("unexpected decisions: %v", report.Decisions)
	}
	if len(report.DeadEnds) != 1 || !strings.Contains(report.DeadEnds[0], "Splitting on raw ##") {
		t.Errorf("unexpected dead ends: %v", report.DeadEnds)
	}
	if len(report.Findings) != 1 || !strings.Contains(report.Findings[0], "pa status creates state.db") {
		t.Errorf("unexpected findings: %v", report.Findings)
	}
}

func TestParseProjectReport_ArchitecturalDistillation(t *testing.T) {
	content := `---
schema_version: 1
charter:
  pr_id: PR-01
  slug: pr-01-core-schemas-and-enrollment
  allowed_modules:
    - daemon/workspace-plugin/principal-architect/scripts/schemas.py
    - daemon/workspace-plugin/principal-architect/scripts/pa_cli.py
status: GRADUATED
---

# PR-01 — Core Schemas

## 4. Outcome & Session Log

### 4.1 Architectural Distillation (` + "`memory_mode: builtin`" + `)

- **Key Architectural Decisions & Invariants Preserved:**
  - Pre-H2 preambles in ` + "`daemon/workspace-plugin/principal-architect/scripts/schemas.py`" + ` are preserved as a leading Section.
  - ` + "`pa status`" + ` is strictly read-only on unenrolled repositories.
- **Verification & Audit Summary:**
  - 51 passed.
`
	report, err := seed.ParseProjectReport(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatalf("expected non-nil project report")
	}
	if report.ProjectName != "pr-01-core-schemas-and-enrollment" {
		t.Errorf("got ProjectName %q, want pr-01-core-schemas-and-enrollment", report.ProjectName)
	}
	if len(report.Decisions) != 2 {
		t.Fatalf("got %d decisions, want 2: %v", len(report.Decisions), report.Decisions)
	}
	if len(report.Changes) != 2 {
		t.Fatalf("got %d changes, want 2: %+v", len(report.Changes), report.Changes)
	}
}

