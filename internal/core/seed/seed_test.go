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
			"Valid decision to keep",
			"n/a",
			"TBD",
		},
		DeadEnds: []string{
			"(pending)",
			"None encountered during implementation",
			"Real dead end that failed",
		},
		Findings: []string{
			"Pending",
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
- ` + "`internal/nonexistent/ghost.go`" + ` — nonexistent path that looks syntactic

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
	if len(changes) != 1 || changes[0].Path != "internal/real/file.go" {
		t.Fatalf("got changes %+v, want only internal/real/file.go", changes)
	}

	wantTime, err := time.Parse(time.RFC3339, commit1TimeStr)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed[0].CommitTime.UTC().Equal(wantTime) {
		t.Errorf("got CommitTime %v, want %v (from BeforeCommit, not HEAD)", parsed[0].CommitTime.UTC(), wantTime)
	}
}
