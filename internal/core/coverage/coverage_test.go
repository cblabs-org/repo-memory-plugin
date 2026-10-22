/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package coverage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
)

func TestComputeCoverageAndGapLines(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()

	// Create source directories:
	// - internal/auth (covered by active entries)
	// - internal/payments (covered only by suspect entry -> low coverage)
	// - internal/unmapped (0 entries -> unmapped dir)
	for _, rel := range []string{
		"internal/auth/token.go",
		"internal/payments/charge.go",
		"internal/unmapped/worker.go",
	} {
		full := filepath.Join(repoRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("package main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entries := []dossiers.Entry{
		{
			ID:           "inv-0001",
			Type:         dossiers.EntryTypeInvariant,
			DossierScope: "auth",
			Text:         "**Token verification**: Always validate audience claims.",
			Anchors:      []string{"internal/auth/token.go#Verify"},
		},
		{
			ID:           "rec-0001",
			Type:         dossiers.EntryTypeRecipe,
			DossierScope: "auth",
			Text:         "**Rotate keys**: Run key rotation command.",
			Anchors:      []string{"internal/auth/token.go"},
		},
		{
			ID:           "got-0002",
			Type:         dossiers.EntryTypeGotcha,
			DossierScope: "payments",
			Text:         "**Idempotency key**: Stale anchor on charge handler.",
			Anchors:      []string{"internal/payments/charge.go"},
			StaleSuspect: true,
		},
		{
			ID:           "iss-0003",
			Type:         dossiers.EntryTypeOpenIssue,
			DossierScope: "billing",
			Text:         "**Webhook retries**: Missing dead-letter queue.",
			ClosesWhen:   "file_exists:internal/billing/dlq.go",
			Anchors:      []string{"internal/payments/charge.go"},
		},
	}

	report, err := ComputeCoverage(ctx, repoRoot, entries, 0.5)
	if err != nil {
		t.Fatalf("ComputeCoverage failed: %v", err)
	}

	scopeMap := make(map[string]ScopeCoverage)
	for _, sc := range report.Scopes {
		scopeMap[sc.Scope] = sc
	}

	authCov, ok := scopeMap["auth"]
	if !ok {
		t.Fatal("expected 'auth' scope in report")
	}
	if authCov.Score != 1.0 || authCov.BelowThreshold {
		t.Errorf("expected auth score 1.0 (BelowThreshold=false), got %+v", authCov)
	}
	if authCov.SemanticEntries != 1 || authCov.ProceduralEntries != 1 {
		t.Errorf("expected 1 semantic + 1 procedural in auth, got %+v", authCov)
	}

	payCov, ok := scopeMap["payments"]
	if !ok {
		t.Fatal("expected 'payments' scope in report")
	}
	if payCov.Score != 0.0 || !payCov.BelowThreshold || payCov.SuspectEntries != 1 {
		t.Errorf("expected payments score 0.0 (BelowThreshold=true, SuspectEntries=1), got %+v", payCov)
	}

	billCov, ok := scopeMap["billing"]
	if !ok {
		t.Fatal("expected 'billing' scope in report")
	}
	if !billCov.BelowThreshold || billCov.OpenIssues != 1 {
		t.Errorf("expected billing with only unverified open-issue to be BelowThreshold=true, got %+v", billCov)
	}

	foundUnmapped := false
	for _, d := range report.UnmappedDirs {
		if d == "internal/unmapped" {
			foundUnmapped = true
		}
		if d == "internal/auth" {
			t.Errorf("did not expect covered directory internal/auth in UnmappedDirs: %v", report.UnmappedDirs)
		}
	}
	if !foundUnmapped {
		t.Errorf("expected internal/unmapped in UnmappedDirs, got %v", report.UnmappedDirs)
	}

	// Test FormatGapLinesForScope:
	// 1. Zero active entries & zero unconsolidated notes -> "No memory covers <scope>..."
	noMemLines := FormatGapLinesForScope("unmapped", nil, 0, 0.5)
	if len(noMemLines) != 1 || noMemLines[0] != "No memory covers unmapped. Absence of memory is not evidence; read the code." {
		t.Errorf("unexpected gap lines for uncovered scope: %v", noMemLines)
	}

	// 2. Zero active entries but > 0 unconsolidated notes -> no "No memory covers" false alarm
	withNotesLines := FormatGapLinesForScope("unmapped", nil, 2, 0.5)
	if len(withNotesLines) != 0 {
		t.Errorf("expected 0 gap lines when unconsolidated notes exist, got %v", withNotesLines)
	}

	// 3. Low coverage scope -> "Low memory coverage for <scope>..."
	lowCovLines := FormatGapLinesForScope("payments", entries[2:3], 0, 0.5)
	if len(lowCovLines) != 1 || !strings.HasPrefix(lowCovLines[0], "Low memory coverage for payments (score 0.00). Absence of memory is not evidence; read the code.") {
		t.Errorf("unexpected gap lines for low-coverage scope: %v", lowCovLines)
	}

	// 4. Healthy scope -> 0 gap lines
	healthyLines := FormatGapLinesForScope("auth", entries[:2], 0, 0.5)
	if len(healthyLines) != 0 {
		t.Errorf("expected 0 gap lines for healthy scope, got %v", healthyLines)
	}
}

func TestWriteQuestionsFile(t *testing.T) {
	ctx := context.Background()
	repoRoot := t.TempDir()

	unmappedFile := filepath.Join(repoRoot, "pkg", " mystery", " mystery.go")
	unmappedDir := filepath.Join(repoRoot, "pkg", "mystery")
	if err := os.MkdirAll(unmappedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = unmappedFile
	if err := os.WriteFile(filepath.Join(unmappedDir, "mystery.go"), []byte("package mystery\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []dossiers.Entry{
		{
			ID:           "iss-0100",
			Type:         dossiers.EntryTypeOpenIssue,
			DossierScope: "sync",
			Text:         "**Race on shutdown**: Worker channel closes before drain.",
			ClosesWhen:   "symbol_absent:pkg/sync/worker.go#unsafeClose",
			Anchors:      []string{"pkg/sync/worker.go"},
		},
		{
			ID:           "inv-0200",
			Type:         dossiers.EntryTypeInvariant,
			DossierScope: "sync",
			Text:         "**Mutex order**: Acquire lockA before lockB.",
			StaleSuspect: true,
			Anchors:      []string{"pkg/sync/locks.go"},
		},
	}

	report, err := ComputeCoverage(ctx, repoRoot, entries, 0.5)
	if err != nil {
		t.Fatalf("ComputeCoverage failed: %v", err)
	}

	if err := WriteQuestionsFile(repoRoot, report, entries); err != nil {
		t.Fatalf("WriteQuestionsFile failed: %v", err)
	}

	qPath := filepath.Join(repoRoot, ".memory", "QUESTIONS.md")
	data, err := os.ReadFile(qPath)
	if err != nil {
		t.Fatalf("reading QUESTIONS.md failed: %v", err)
	}
	md := string(data)

	for _, want := range []string{
		"## 1. Unresolved Open Issues",
		"[iss-0100]",
		"symbol_absent:pkg/sync/worker.go#unsafeClose",
		"## 2. Suspect / Stale Entries Needing Verification",
		"[inv-0200]",
		"## 3. Uncovered & Low-Coverage Scopes",
		"Scope `sync`",
		"Unmapped directory `pkg/mystery`",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("expected QUESTIONS.md to contain %q, got:\n%s", want, md)
		}
	}
}
