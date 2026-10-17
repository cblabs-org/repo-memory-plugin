/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/coverage/coverage.go
 * Description: Computes repository memory coverage scores across scopes, detects
 *              unmapped source directories, formats concise coverage gap lines,
 *              and renders .memory/QUESTIONS.md.
 *
 * Exported Types / Functions:
 * - DefaultGapThreshold:       Default coverage score threshold below which a scope is flagged.
 * - MaxGapLinesPerResponse:    Hard cap on coverage gap lines per tool response (3).
 * - ScopeCoverage:             Summarizes memory coverage and entry health for a single dossier scope.
 * - CoverageReport:            Summarizes repository-wide memory coverage, unmapped dirs, and contradictions.
 * - ComputeCoverage():         Analyzes active entries and repository structure to produce a CoverageReport.
 * - FormatGapLinesForScope():  Returns up to MaxGapLinesPerResponse one-line gap notices for a scope.
 * - FormatFreshnessHeader():   Returns the 1-line A6 freshness status header for repoRoot.
 * - IdentifyCoverageGaps():    Returns all scopes in a CoverageReport whose score is below threshold.
 * - FormatGapLines():          Returns one-line coverage gap notices across a CoverageReport.
 * - GenerateQuestionsMarkdown(): Renders .memory/QUESTIONS.md content from coverage and entries.
 * - WriteQuestionsFile():      Renders and writes <repoRoot>/.memory/QUESTIONS.md.
 */

// Package coverage computes per-scope memory health scores, identifies unmapped
// source directories, and formats coverage gap notices and elicitation questions.
package coverage

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
)

const (
	// DefaultGapThreshold is the default coverage score threshold below which a scope is flagged.
	DefaultGapThreshold = 0.5
	// MaxGapLinesPerResponse is the hard cap on coverage gap lines per tool response (SPEC.md §5.2).
	MaxGapLinesPerResponse = 3
)

// ScopeCoverage summarizes memory coverage and entry health for a single dossier scope.
type ScopeCoverage struct {
	Scope             string   `json:"scope"`                    // Canonical dossier scope identifier.
	Score             float64  `json:"score"`                    // Weighted health score in [0.0, 1.0] rounded to two decimal places.
	ActiveEntries     int      `json:"active_entries"`           // Count of non-retired, non-superseded entries in the scope.
	SemanticEntries   int      `json:"semantic_entries"`         // Count of active entries with semantic cognitive kind.
	ProceduralEntries int      `json:"procedural_entries"`       // Count of active entries with procedural cognitive kind.
	SuspectEntries    int      `json:"suspect_entries"`          // Count of active entries flagged as stale or suspect.
	OpenIssues        int      `json:"open_issues"`              // Count of active open-issue entries in the scope.
	BelowThreshold    bool     `json:"below_threshold"`          // True when Score is strictly below the evaluation threshold.
	UnmappedFiles     []string `json:"unmapped_files,omitempty"` // Optional list of unanchored source files within the scope.
}

// CoverageReport summarizes repository-wide memory coverage, unmapped source directories, and contradictions.
type CoverageReport struct {
	Scopes         []ScopeCoverage `json:"scopes"`                   // Sorted per-scope coverage summaries.
	UnmappedDirs   []string        `json:"unmapped_dirs,omitempty"`  // Source directories lacking any anchored memory entries.
	Contradictions []string        `json:"contradictions,omitempty"` // Detected cross-entry contradiction descriptions, if any.
	Threshold      float64         `json:"threshold"`                // Coverage score threshold applied to produce BelowThreshold flags.
}

// normalizeThreshold returns DefaultGapThreshold when threshold is <= 0, or threshold otherwise.
func normalizeThreshold(threshold float64) float64 {
	if threshold <= 0 {
		return DefaultGapThreshold
	}
	return threshold
}

// isActiveEntry reports whether e is neither retired nor superseded by another entry.
func isActiveEntry(e dossiers.Entry) bool {
	return !e.Retired && strings.TrimSpace(e.SupersededBy) == ""
}

// isSuspectEntry reports whether e is marked StaleSuspect or has Verified set to "suspect".
func isSuspectEntry(e dossiers.Entry) bool {
	return e.StaleSuspect || strings.EqualFold(strings.TrimSpace(e.Verified), "suspect")
}

// computeSingleScopeCoverage calculates the ScopeCoverage metrics and weighted score for a single scope.
func computeSingleScopeCoverage(scope string, entries []dossiers.Entry, threshold float64) ScopeCoverage {
	// --- Step 1: Filter to active entries and handle empty scopes ---
	threshold = normalizeThreshold(threshold)
	sc := ScopeCoverage{
		Scope: scope,
	}

	var active []dossiers.Entry
	for _, e := range entries {
		if isActiveEntry(e) {
			active = append(active, e)
		}
	}

	sc.ActiveEntries = len(active)
	if len(active) == 0 {
		sc.Score = 0.0
		sc.BelowThreshold = true
		return sc
	}

	// --- Step 2: Accumulate cognitive kind counts and weighted verification scores ---
	totalWeight := 0.0
	for _, e := range active {
		if e.EffectiveKind() == dossiers.CognitiveKindProcedural {
			sc.ProceduralEntries++
		} else {
			sc.SemanticEntries++
		}
		if e.Type == dossiers.EntryTypeOpenIssue {
			sc.OpenIssues++
		}

		if isSuspectEntry(e) {
			sc.SuspectEntries++
			continue
		}
		if e.Type == dossiers.EntryTypeOpenIssue && strings.TrimSpace(e.Verified) == "" {
			totalWeight += 0.25
			continue
		}
		totalWeight += 1.0
	}

	// --- Step 3: Clamp and round the normalized coverage score ---
	rawScore := totalWeight / float64(len(active))
	if rawScore < 0 {
		rawScore = 0
	}
	if rawScore > 1 {
		rawScore = 1
	}
	sc.Score = math.Round(rawScore*100) / 100
	sc.BelowThreshold = sc.Score < threshold
	return sc
}

// ComputeCoverage analyzes active entries and repository structure to produce a CoverageReport.
func ComputeCoverage(ctx context.Context, repoRoot string, entries []dossiers.Entry, threshold float64) (*CoverageReport, error) {
	// --- Step 1: Group active entries by scope and collect all anchored file paths ---
	if ctx == nil {
		ctx = context.Background()
	}
	threshold = normalizeThreshold(threshold)

	byScope := make(map[string][]dossiers.Entry)
	scopeNames := make(map[string]bool)
	anchoredPaths := make(map[string]bool)

	for _, e := range entries {
		if !isActiveEntry(e) {
			continue
		}
		scope := strings.TrimSpace(e.DossierScope)
		if scope == "" {
			scope = "general"
		}
		scopeNames[scope] = true
		byScope[scope] = append(byScope[scope], e)

		for _, a := range e.Anchors {
			p := stripAnchorPath(a)
			if p != "" {
				anchoredPaths[p] = true
			}
		}
	}

	// --- Step 2: Merge configured subsystems and declared scopes from repository config ---
	if strings.TrimSpace(repoRoot) != "" {
		if rCfg, err := config.LoadRepoConfig(repoRoot); err == nil {
			for sub := range rCfg.Subsystems {
				if trimmed := strings.TrimSpace(sub); trimmed != "" {
					scopeNames[trimmed] = true
				}
			}
		}
		if sCfg, err := config.LoadScopeConfig(filepath.Join(repoRoot, ".memory", "config.yaml")); err == nil && sCfg != nil {
			for _, list := range [][]config.DeclaredScope{sCfg.Topics, sCfg.Externals, sCfg.Procedures} {
				for _, ds := range list {
					if trimmed := strings.TrimSpace(ds.Name); trimmed != "" {
						scopeNames[trimmed] = true
					}
				}
			}
		}
	}

	// --- Step 3: Compute coverage for each sorted scope and scan for unmapped source directories ---
	sortedScopes := make([]string, 0, len(scopeNames))
	for s := range scopeNames {
		sortedScopes = append(sortedScopes, s)
	}
	sort.Strings(sortedScopes)

	report := &CoverageReport{
		Scopes:    make([]ScopeCoverage, 0, len(sortedScopes)),
		Threshold: threshold,
	}

	for _, s := range sortedScopes {
		sc := computeSingleScopeCoverage(s, byScope[s], threshold)
		report.Scopes = append(report.Scopes, sc)
	}

	if strings.TrimSpace(repoRoot) != "" {
		report.UnmappedDirs = findUnmappedSourceDirs(repoRoot, anchoredPaths, scopeNames, byScope)
	}

	return report, nil
}

// FormatGapLinesForScope returns up to MaxGapLinesPerResponse (3) one-line coverage gap notices
// for a requested scope (SPEC.md §5.2).
func FormatGapLinesForScope(scope string, activeEntries []dossiers.Entry, unconsolidatedCount int, threshold float64) []string {
	// --- Step 1: Validate scope and filter to active entries ---
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return nil
	}
	threshold = normalizeThreshold(threshold)

	var active []dossiers.Entry
	for _, e := range activeEntries {
		if isActiveEntry(e) {
			active = append(active, e)
		}
	}

	// --- Step 2: Format zero-coverage or low-coverage notice and enforce MaxGapLinesPerResponse ---
	var lines []string
	if len(active) == 0 && unconsolidatedCount == 0 {
		lines = append(lines, fmt.Sprintf("No memory covers %s. Absence of memory is not evidence; read the code.", scope))
	} else if len(active) > 0 {
		sc := computeSingleScopeCoverage(scope, active, threshold)
		if sc.BelowThreshold {
			lines = append(lines, fmt.Sprintf("Low memory coverage for %s (score %.2f). Absence of memory is not evidence; read the code.", scope, sc.Score))
		}
	}

	if len(lines) > MaxGapLinesPerResponse {
		lines = lines[:MaxGapLinesPerResponse]
	}
	return lines
}

// FormatFreshnessHeader returns the 1-line A6 freshness status header for repoRoot.
func FormatFreshnessHeader(repoRoot, dataDir, repoID string) string {
	return tick.FormatFreshnessLine(context.Background(), dataDir, repoID, repoRoot, nil, nil)
}

// IdentifyCoverageGaps returns all scopes in report whose coverage score is below threshold.
func IdentifyCoverageGaps(report *CoverageReport) []ScopeCoverage {
	if report == nil {
		return nil
	}
	var gaps []ScopeCoverage
	for _, sc := range report.Scopes {
		if sc.BelowThreshold {
			gaps = append(gaps, sc)
		}
	}
	return gaps
}

// FormatGapLines returns one-line coverage gap notices across a CoverageReport.
func FormatGapLines(report *CoverageReport) []string {
	// --- Step 1: Format notices for scopes below the coverage threshold ---
	if report == nil {
		return nil
	}
	var lines []string
	for _, sc := range report.Scopes {
		if !sc.BelowThreshold {
			continue
		}
		if sc.ActiveEntries == 0 {
			lines = append(lines, fmt.Sprintf("No memory covers %s. Absence of memory is not evidence; read the code.", sc.Scope))
		} else {
			lines = append(lines, fmt.Sprintf("Low memory coverage for %s (score %.2f). Absence of memory is not evidence; read the code.", sc.Scope, sc.Score))
		}
	}

	// --- Step 2: Append notices for unmapped source directories ---
	for _, dir := range report.UnmappedDirs {
		lines = append(lines, fmt.Sprintf("Unmapped directory %s has 0 anchored memory entries.", dir))
	}
	return lines
}

// GenerateQuestionsMarkdown renders .memory/QUESTIONS.md listing unresolved open issues,
// suspect/stale entries needing verification, and uncovered/low-coverage scopes (SPEC.md §9.2).
func GenerateQuestionsMarkdown(report *CoverageReport, entries []dossiers.Entry) string {
	// --- Step 1: Partition active entries into open issues and suspect entries ---
	var b strings.Builder
	b.WriteString("# Memory Elicitation & Open Questions\n\n")
	b.WriteString("Generated automatically from repository memory coverage, open issues, and staleness checks.\n\n")

	var openIssues []dossiers.Entry
	var suspectEntries []dossiers.Entry
	for _, e := range entries {
		if !isActiveEntry(e) {
			continue
		}
		if e.Type == dossiers.EntryTypeOpenIssue {
			openIssues = append(openIssues, e)
		}
		if isSuspectEntry(e) {
			suspectEntries = append(suspectEntries, e)
		}
	}

	sort.Slice(openIssues, func(i, j int) bool { return openIssues[i].ID < openIssues[j].ID })
	sort.Slice(suspectEntries, func(i, j int) bool { return suspectEntries[i].ID < suspectEntries[j].ID })

	// --- Step 2: Render Section 1 (Unresolved Open Issues) ---
	b.WriteString("## 1. Unresolved Open Issues\n\n")
	if len(openIssues) == 0 {
		b.WriteString("_No unresolved open issues._\n\n")
	} else {
		for _, e := range openIssues {
			scope := strings.TrimSpace(e.DossierScope)
			if scope == "" {
				scope = "general"
			}
			summary := firstLineSummary(e.Text)
			b.WriteString(fmt.Sprintf("- **[%s]** (`%s`): %s\n", e.ID, scope, summary))
			if cw := strings.TrimSpace(e.ClosesWhen); cw != "" {
				b.WriteString(fmt.Sprintf("  - Closes when: `%s`\n", cw))
			}
			if len(e.Anchors) > 0 {
				b.WriteString(fmt.Sprintf("  - Anchors: `%s`\n", strings.Join(e.Anchors, "`, `")))
			}
		}
		b.WriteString("\n")
	}

	// --- Step 3: Render Section 2 (Suspect / Stale Entries Needing Verification) ---
	b.WriteString("## 2. Suspect / Stale Entries Needing Verification\n\n")
	if len(suspectEntries) == 0 {
		b.WriteString("_No suspect or stale entries._\n\n")
	} else {
		for _, e := range suspectEntries {
			scope := strings.TrimSpace(e.DossierScope)
			if scope == "" {
				scope = "general"
			}
			summary := firstLineSummary(e.Text)
			b.WriteString(fmt.Sprintf("- **[%s]** (`%s`, `%s`): %s\n", e.ID, scope, e.Type, summary))
			if len(e.Anchors) > 0 {
				b.WriteString(fmt.Sprintf("  - Anchors: `%s`\n", strings.Join(e.Anchors, "`, `")))
			}
		}
		b.WriteString("\n")
	}

	// --- Step 4: Render Section 3 (Uncovered & Low-Coverage Scopes) ---
	b.WriteString("## 3. Uncovered & Low-Coverage Scopes\n\n")
	hasGaps := false
	if report != nil {
		for _, sc := range report.Scopes {
			if sc.BelowThreshold {
				hasGaps = true
				b.WriteString(fmt.Sprintf("- **Scope `%s`**: coverage score `%.2f` (%d active, %d suspect, %d open issues)\n",
					sc.Scope, sc.Score, sc.ActiveEntries, sc.SuspectEntries, sc.OpenIssues))
			}
		}
		for _, dir := range report.UnmappedDirs {
			hasGaps = true
			b.WriteString(fmt.Sprintf("- **Unmapped directory `%s`**: 0 anchored memory entries\n", dir))
		}
	}
	if !hasGaps {
		b.WriteString("_All discovered scopes and source directories meet the coverage threshold._\n\n")
	} else {
		b.WriteString("\n")
	}

	return b.String()
}

// WriteQuestionsFile renders and writes <repoRoot>/.memory/QUESTIONS.md.
func WriteQuestionsFile(repoRoot string, report *CoverageReport, entries []dossiers.Entry) error {
	// --- Step 1: Validate repoRoot and ensure .memory directory exists ---
	if strings.TrimSpace(repoRoot) == "" {
		return fmt.Errorf("repoRoot is required")
	}
	memDir := filepath.Join(repoRoot, ".memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return fmt.Errorf("creating .memory directory: %w", err)
	}

	// --- Step 2: Generate markdown and write QUESTIONS.md ---
	content := GenerateQuestionsMarkdown(report, entries)
	target := filepath.Join(memDir, "QUESTIONS.md")
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing QUESTIONS.md: %w", err)
	}
	return nil
}

// firstLineSummary returns the first non-empty trimmed line of text.
func firstLineSummary(text string) string {
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// stripAnchorPath removes #symbol and :Lline suffixes and normalizes slashes on an anchor string.
func stripAnchorPath(anchor string) string {
	// --- Step 1: Trim whitespace and strip symbol or line-range suffixes ---
	a := strings.TrimSpace(anchor)
	if a == "" {
		return ""
	}
	if idx := strings.Index(a, "#"); idx >= 0 {
		a = a[:idx]
	}
	if idx := strings.Index(a, ":L"); idx >= 0 {
		a = a[:idx]
	}

	// --- Step 2: Convert path separators to forward slashes and strip leading relative prefixes ---
	a = filepath.ToSlash(strings.TrimSpace(a))
	a = strings.TrimPrefix(a, "./")
	a = strings.TrimPrefix(a, "/")
	return a
}

// excludedTopDirs lists top-level and nested directory names ignored when scanning for unmapped source directories.
var excludedTopDirs = map[string]bool{
	".git":         true,
	".memory":      true,
	".agents":      true,
	".github":      true,
	"vendor":       true,
	"node_modules": true,
	"testdata":     true,
	"dist":         true,
	"build":        true,
	"docs":         true,
}

// sourceFileExts lists file extensions recognized as source code when discovering candidate module directories.
var sourceFileExts = map[string]bool{
	".go":   true,
	".py":   true,
	".ts":   true,
	".tsx":  true,
	".js":   true,
	".jsx":  true,
	".rs":   true,
	".c":    true,
	".cc":   true,
	".cpp":  true,
	".h":    true,
	".hpp":  true,
	".java": true,
	".rb":   true,
	".sh":   true,
}

// findUnmappedSourceDirs scans up to two directory levels under repoRoot and returns sorted source
// directories that have no anchored memory entries or matching populated scopes.
func findUnmappedSourceDirs(
	repoRoot string,
	anchoredPaths map[string]bool,
	scopeNames map[string]bool,
	byScope map[string][]dossiers.Entry,
) []string {
	// --- Step 1: Read top-level directories under repoRoot ---
	topEntries, err := os.ReadDir(repoRoot)
	if err != nil {
		return nil
	}

	// --- Step 2: Collect candidate first- or second-level directories containing source files ---
	var candidateDirs []string
	for _, te := range topEntries {
		if !te.IsDir() {
			continue
		}
		name := te.Name()
		if strings.HasPrefix(name, ".") || excludedTopDirs[name] {
			continue
		}
		topPath := filepath.Join(repoRoot, name)
		subEntries, subErr := os.ReadDir(topPath)
		if subErr != nil {
			continue
		}

		hasSubDirs := false
		for _, se := range subEntries {
			if se.IsDir() && !strings.HasPrefix(se.Name(), ".") && !excludedTopDirs[se.Name()] {
				subRel := filepath.ToSlash(filepath.Join(name, se.Name()))
				if dirHasSourceFiles(filepath.Join(repoRoot, filepath.FromSlash(subRel))) {
					candidateDirs = append(candidateDirs, subRel)
					hasSubDirs = true
				}
			}
		}
		if !hasSubDirs && dirHasSourceFiles(topPath) {
			candidateDirs = append(candidateDirs, name)
		}
	}

	// --- Step 3: Filter out directories already covered by anchored paths or populated scopes ---
	sort.Strings(candidateDirs)
	var unmapped []string
	for _, dirRel := range candidateDirs {
		if isDirCovered(dirRel, anchoredPaths, scopeNames, byScope) {
			continue
		}
		unmapped = append(unmapped, dirRel)
	}
	return unmapped
}

// dirHasSourceFiles walks dirPath and reports whether it contains at least one recognized source file.
func dirHasSourceFiles(dirPath string) bool {
	// --- Step 1: Walk directory tree skipping hidden and excluded directories ---
	found := false
	_ = filepath.WalkDir(dirPath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || excludedTopDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// --- Step 2: Stop traversal immediately upon finding a recognized source extension ---
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if sourceFileExts[ext] {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// isDirCovered reports whether dirRel is covered by any anchored file path or populated dossier scope.
func isDirCovered(
	dirRel string,
	anchoredPaths map[string]bool,
	scopeNames map[string]bool,
	byScope map[string][]dossiers.Entry,
) bool {
	// --- Step 1: Check whether any anchored path equals or resides inside dirRel ---
	prefix := dirRel + "/"
	for p := range anchoredPaths {
		if p == dirRel || strings.HasPrefix(p, prefix) {
			return true
		}
	}

	// --- Step 2: Check whether dirRel or its base name matches a populated scope ---
	base := filepath.Base(dirRel)
	if scopeNames[dirRel] && len(byScope[dirRel]) > 0 {
		return true
	}
	if scopeNames[base] && len(byScope[base]) > 0 {
		return true
	}
	return false
}
