package seed_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
- ` + "`path/to/file`" + ` — added new feature
- ` + "`another/file`" + ` - fixed bug

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
	if report.Changes[0].Path != "path/to/file" || report.Changes[0].Description != "added new feature" {
		t.Errorf("got change 0: %+v", report.Changes[0])
	}
	if report.Changes[1].Path != "another/file" || report.Changes[1].Description != "fixed bug" {
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
	opsList := seed.ReportToOps(report, resolver)

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

		switch o.Fields["type"] {
		case "decision":
			foundDec = true
			if !strings.HasPrefix(o.EntryID, "dec-") {
				t.Errorf("bad dec prefix: %s", o.EntryID)
			}
		case "ruled-out":
			foundRul = true
			if !strings.HasPrefix(o.EntryID, "rul-") {
				t.Errorf("bad rul prefix: %s", o.EntryID)
			}
		case "open-issue":
			foundIssue = true
			if !strings.HasPrefix(o.EntryID, "issue-") {
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
