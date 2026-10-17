/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/closes_when.go
 * Description: Deterministic evaluation of closes_when predicates on active open-issue
 *              dossier entries (SPEC §6.3.4). Evaluates file/path existence, file
 *              content patterns, git branch merges, Principal Architect workorder
 *              completions, and passing test/build outcomes, emitting OpRetire ops
 *              and updating projected dossiers and the SQLite search index.
 *
 * Exported Types / Functions:
 * - ClosesWhenResult:                    Summary of open-issue entries evaluated and retired.
 * - ClosedIssueRecord:                   Details of a single open-issue entry retired by closes_when.
 * - PipelineExecutor.EvaluateClosesWhen(): Evaluates closes_when predicates across repo and branch tiers.
 * - EvaluateClosesWhenCondition():       Evaluates a single or compound closes_when expression for an entry.
 */

package tick

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/consolidate"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// ClosesWhenResult summarizes the deterministic closes_when evaluation pass.
type ClosesWhenResult struct {
	Evaluated int                 `json:"evaluated"`        // Total active open-issue entries with non-empty ClosesWhen inspected.
	Closed    []ClosedIssueRecord `json:"closed,omitempty"` // Open-issue entries whose closes_when predicate was satisfied.
}

// ClosedIssueRecord describes an open-issue entry retired because its closes_when predicate was satisfied.
type ClosedIssueRecord struct {
	EntryID    string `json:"entry_id"`    // Unique identifier of the retired open-issue entry.
	Scope      string `json:"scope"`       // Dossier scope containing the entry.
	ClosesWhen string `json:"closes_when"` // Raw closes_when expression that was evaluated.
	Reason     string `json:"reason"`      // Human-readable explanation of why the predicate passed.
}

var (
	monthDirNameRe    = regexp.MustCompile(`^\d{4}-\d{2}$`)               // Matches YYYY-MM monthly op-log subdirectory names.
	conditionPrefixRe = regexp.MustCompile(`^\s*[a-zA-Z_][a-zA-Z0-9_]*:`) // Matches a leading closes_when predicate kind prefix (e.g. "file_exists:").
)

// EvaluateClosesWhen inspects active open-issue entries (from .memory/ops/ and .memory/local/ops/)
// with non-empty ClosesWhen, emits ops.OpRetire for each satisfied entry, re-projects dossiers,
// and updates p.idx if non-nil.
func (p *PipelineExecutor) EvaluateClosesWhen(ctx context.Context) (*ClosesWhenResult, error) {
	// --- Step 0: Initialize context, episode store, and evaluation clock ---
	if ctx == nil {
		ctx = context.Background()
	}
	res := &ClosesWhenResult{}
	if strings.TrimSpace(p.repoRoot) == "" {
		return res, nil
	}

	epStore := p.epStore
	if epStore == nil && strings.TrimSpace(p.dataDir) != "" && strings.TrimSpace(p.repoID) != "" {
		epStore = episode.NewStore(p.dataDir, p.repoID)
	}

	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}

	// --- Step 1: Evaluate shared repository ops (.memory/ops/) ---
	repoOpsDir := filepath.Join(p.repoRoot, ".memory", "ops")
	if info, err := os.Stat(repoOpsDir); err == nil && info.IsDir() {
		closedRecords, retireOps, err := p.evaluateOpsDirIssues(ctx, repoOpsDir, epStore, now)
		if err != nil {
			return nil, fmt.Errorf("evaluating repo closes_when in %s: %w", repoOpsDir, err)
		}
		res.Evaluated += closedRecords.Evaluated
		if len(retireOps) > 0 {
			if _, err := consolidate.RecordRepoTier(ctx, p.repoRoot, retireOps, nil, nil, now); err != nil {
				return nil, fmt.Errorf("recording repo closes_when retirements: %w", err)
			}
			res.Closed = append(res.Closed, closedRecords.Closed...)
		}
	}

	// --- Step 2: Evaluate branch-local ops (.memory/local/ops/) ---
	localOpsRoot := filepath.Join(p.repoRoot, ".memory", "local", "ops")
	if entries, err := os.ReadDir(localOpsRoot); err == nil {
		hasDirectBatches := false
		var branchSlugs []string
		for _, e := range entries {
			if e.IsDir() {
				if monthDirNameRe.MatchString(e.Name()) {
					hasDirectBatches = true
				} else {
					branchSlugs = append(branchSlugs, e.Name())
				}
			} else if strings.HasSuffix(e.Name(), ".json") {
				hasDirectBatches = true
			}
		}
		sort.Strings(branchSlugs)

		if hasDirectBatches {
			closedRecords, retireOps, err := p.evaluateOpsDirIssues(ctx, localOpsRoot, epStore, now)
			if err != nil {
				return nil, fmt.Errorf("evaluating local closes_when in %s: %w", localOpsRoot, err)
			}
			res.Evaluated += closedRecords.Evaluated
			if len(retireOps) > 0 {
				localBaseDir := filepath.Join(p.repoRoot, ".memory", "local")
				if _, err := consolidate.RecordAndRenderDirs(ctx, localOpsRoot, localBaseDir, retireOps, nil, nil, now); err != nil {
					return nil, fmt.Errorf("recording local closes_when retirements: %w", err)
				}
				res.Closed = append(res.Closed, closedRecords.Closed...)
			}
		}

		for _, branchSlug := range branchSlugs {
			branchOpsDir := filepath.Join(localOpsRoot, branchSlug)
			closedRecords, retireOps, err := p.evaluateOpsDirIssues(ctx, branchOpsDir, epStore, now)
			if err != nil {
				return nil, fmt.Errorf("evaluating branch closes_when in %s: %w", branchOpsDir, err)
			}
			res.Evaluated += closedRecords.Evaluated
			if len(retireOps) > 0 {
				if _, err := consolidate.RecordBranchTier(ctx, p.repoRoot, branchSlug, retireOps, nil, nil, now); err != nil {
					return nil, fmt.Errorf("recording branch closes_when retirements for %s: %w", branchSlug, err)
				}
				res.Closed = append(res.Closed, closedRecords.Closed...)
			}
		}
	}

	// --- Step 3: Update multi-tier SQLite index if any entries were retired ---
	if len(res.Closed) > 0 && p.idx != nil {
		for _, closed := range res.Closed {
			_ = p.idx.DeleteEntry(ctx, closed.EntryID)
		}
		currentBranch, _ := gitops.GetCurrentBranchWithContext(ctx, p.repoRoot)
		if err := consolidate.RebuildMultiTierIndex(ctx, p.idx, p.dataDir, p.repoID, p.repoRoot, currentBranch); err != nil {
			return nil, fmt.Errorf("updating index after closes_when evaluation: %w", err)
		}
	}

	return res, nil
}

// evaluateOpsDirIssues projects dossiers from opsDir, evaluates active open-issue closes_when predicates,
// and returns the evaluation summary alongside any generated OpRetire operations.
func (p *PipelineExecutor) evaluateOpsDirIssues(ctx context.Context, opsDir string, epStore *episode.Store, now time.Time) (*ClosesWhenResult, []ops.Op, error) {
	// --- Step 1: Project dossiers from opsDir and sort scopes deterministically ---
	out := &ClosesWhenResult{}
	projected, err := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx)
	if err != nil {
		return nil, nil, err
	}

	scopes := make([]string, 0, len(projected))
	for s := range projected {
		scopes = append(scopes, s)
	}
	sort.Strings(scopes)

	// --- Step 2: Evaluate each active open-issue entry with a non-empty ClosesWhen predicate ---
	var retireOps []ops.Op
	for _, scope := range scopes {
		d := projected[scope]
		if d == nil {
			continue
		}
		targetScope := strings.TrimSpace(d.Scope)
		if targetScope == "" {
			targetScope = scope
		}
		for _, entry := range d.Entries {
			if entry.Retired || entry.SupersededBy != "" {
				continue
			}
			if entry.Type != dossiers.EntryTypeOpenIssue {
				continue
			}
			if strings.TrimSpace(entry.ClosesWhen) == "" {
				continue
			}
			out.Evaluated++
			ok, reason := EvaluateClosesWhenCondition(ctx, p.repoRoot, p.dataDir, p.repoID, epStore, entry)
			if !ok {
				continue
			}
			retireOps = append(retireOps, ops.Op{
				Type:         ops.OpRetire,
				EntryID:      entry.ID,
				DossierScope: targetScope,
				Timestamp:    now,
				Source:       ops.SourceTick,
				Fields: map[string]any{
					"reason": "closes_when satisfied: " + reason,
				},
			})
			out.Closed = append(out.Closed, ClosedIssueRecord{
				EntryID:    entry.ID,
				Scope:      targetScope,
				ClosesWhen: entry.ClosesWhen,
				Reason:     reason,
			})
		}
	}

	return out, retireOps, nil
}

// EvaluateClosesWhenCondition evaluates deterministic closes_when expressions
// (supporting single conditions and compound conditions joined by && / ; for AND, or || for OR).
func EvaluateClosesWhenCondition(ctx context.Context, repoRoot, dataDir, repoID string, epStore *episode.Store, entry dossiers.Entry) (bool, string) {
	// --- Step 1: Validate context and non-empty ClosesWhen string ---
	if ctx == nil {
		ctx = context.Background()
	}
	raw := strings.TrimSpace(entry.ClosesWhen)
	if raw == "" {
		return false, ""
	}

	// --- Step 2: Split by OR (||) clauses and evaluate conjunctive AND terms ---
	orClauses := strings.Split(raw, "||")
	for _, orClause := range orClauses {
		orClause = strings.TrimSpace(orClause)
		if orClause == "" {
			continue
		}
		andTerms := splitAndConditions(orClause)
		if len(andTerms) == 0 {
			continue
		}
		allSatisfied := true
		var reasons []string
		for _, term := range andTerms {
			ok, reason := evaluateSingleClosesWhen(ctx, repoRoot, dataDir, repoID, epStore, entry, term)
			if !ok {
				allSatisfied = false
				break
			}
			reasons = append(reasons, reason)
		}
		if allSatisfied && len(reasons) > 0 {
			return true, strings.Join(reasons, "; ")
		}
	}

	return false, ""
}

// splitAndConditions splits a conjunctive clause on "&&" and top-level ";" boundaries
// where the following segment starts with a known condition prefix.
func splitAndConditions(clause string) []string {
	// --- Step 1: Split clause on explicit "&&" operators ---
	ampParts := strings.Split(clause, "&&")
	var out []string
	for _, part := range ampParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// --- Step 2: Split semicolon-delimited sub-clauses only when followed by a predicate prefix ---
		semiParts := strings.Split(part, ";")
		if len(semiParts) == 1 {
			out = append(out, part)
			continue
		}
		current := strings.TrimSpace(semiParts[0])
		for i := 1; i < len(semiParts); i++ {
			next := strings.TrimSpace(semiParts[i])
			if conditionPrefixRe.MatchString(next) {
				if current != "" {
					out = append(out, current)
				}
				current = next
			} else {
				current = current + ";" + semiParts[i]
			}
		}
		if strings.TrimSpace(current) != "" {
			out = append(out, strings.TrimSpace(current))
		}
	}
	return out
}

// evaluateSingleClosesWhen parses and evaluates a single "<kind>:<arg>" predicate expression.
func evaluateSingleClosesWhen(ctx context.Context, repoRoot, dataDir, repoID string, epStore *episode.Store, entry dossiers.Entry, expr string) (bool, string) {
	// --- Step 1: Split predicate kind and argument around the first colon ---
	expr = strings.TrimSpace(expr)
	colonIdx := strings.IndexByte(expr, ':')
	if colonIdx <= 0 {
		return false, ""
	}
	kind := strings.ToLower(strings.TrimSpace(expr[:colonIdx]))
	arg := strings.TrimSpace(expr[colonIdx+1:])
	if arg == "" {
		return false, ""
	}

	// --- Step 2: Dispatch evaluation by predicate kind ---
	switch kind {
	case "path_exists", "file_exists":
		if strings.TrimSpace(repoRoot) == "" {
			return false, ""
		}
		if consolidate.CheckAnchor(repoRoot, arg) {
			return true, fmt.Sprintf("path exists: %s", arg)
		}
		return false, ""

	case "path_absent", "file_absent", "deleted":
		if strings.TrimSpace(repoRoot) == "" {
			return false, ""
		}
		if !consolidate.CheckAnchor(repoRoot, arg) {
			return true, fmt.Sprintf("path absent: %s", arg)
		}
		return false, ""

	case "file_contains":
		if strings.TrimSpace(repoRoot) == "" {
			return false, ""
		}
		return evaluateFileContains(repoRoot, arg)

	case "file_not_contains":
		if strings.TrimSpace(repoRoot) == "" {
			return false, ""
		}
		return evaluateFileNotContains(repoRoot, arg)

	case "branch_merged":
		if strings.TrimSpace(repoRoot) == "" {
			return false, ""
		}
		checker := consolidate.DefaultBranchMergedChecker(repoRoot)
		if checker(arg) {
			return true, fmt.Sprintf("branch merged: %s", arg)
		}
		return false, ""

	case "workorder_complete":
		if strings.TrimSpace(repoRoot) == "" {
			return false, ""
		}
		return evaluateWorkorderComplete(repoRoot, arg)

	case "command_passes", "test_passes":
		return evaluateCommandPasses(ctx, repoRoot, dataDir, repoID, epStore, entry, arg)

	case "outcome":
		if strings.EqualFold(arg, "build_or_test_pass") {
			return evaluateAnyBuildOrTestPass(epStore, dataDir, repoID, entry)
		}
		return false, ""

	default:
		return false, ""
	}
}

// resolveSafeRepoPath normalizes rawRelPath and verifies that the resolved target stays strictly inside repoRoot.
func resolveSafeRepoPath(repoRoot, rawRelPath string) (cleanRelSlash string, absTarget string, ok bool) {
	// --- Step 1: Reject empty or absolute input paths ---
	if strings.TrimSpace(repoRoot) == "" {
		return "", "", false
	}
	rawRelPath = strings.TrimSpace(rawRelPath)
	if rawRelPath == "" || filepath.IsAbs(rawRelPath) || strings.HasPrefix(rawRelPath, "/") || strings.HasPrefix(rawRelPath, "\\") {
		return "", "", false
	}

	// --- Step 2: Normalize slashes, strip leading "./", and reject ".." traversal ---
	slashed := strings.ReplaceAll(rawRelPath, "\\", "/")
	for strings.HasPrefix(slashed, "./") {
		slashed = strings.TrimPrefix(slashed, "./")
	}
	if slashed == "" || strings.HasPrefix(slashed, "/") {
		return "", "", false
	}
	cleaned := filepath.Clean(filepath.FromSlash(slashed))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || filepath.IsAbs(cleaned) {
		return "", "", false
	}

	// --- Step 3: Verify resolved absolute path remains within absRoot ---
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", false
	}
	absTarget = filepath.Join(absRoot, cleaned)
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", false
	}
	return filepath.ToSlash(cleaned), absTarget, true
}

// evaluateFileContains checks whether a repository file contains a literal or regex pattern.
func evaluateFileContains(repoRoot, arg string) (bool, string) {
	// --- Step 1: Split "<path>:<pattern>" and resolve safe repository path ---
	rawPath, pattern, ok := splitPathAndPattern(arg)
	if !ok {
		return false, ""
	}
	relPath, fullPath, ok := resolveSafeRepoPath(repoRoot, rawPath)
	if !ok {
		return false, ""
	}

	// --- Step 2: Read file content and test against pattern ---
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return false, ""
	}
	if matchFilePattern(string(data), pattern) {
		return true, fmt.Sprintf("file %s contains %s", relPath, pattern)
	}
	return false, ""
}

// evaluateFileNotContains checks whether a repository file is absent or does not contain a literal or regex pattern.
func evaluateFileNotContains(repoRoot, arg string) (bool, string) {
	// --- Step 1: Split "<path>:<pattern>" and resolve safe repository path ---
	rawPath, pattern, ok := splitPathAndPattern(arg)
	if !ok {
		return false, ""
	}
	relPath, fullPath, ok := resolveSafeRepoPath(repoRoot, rawPath)
	if !ok {
		return false, ""
	}

	// --- Step 2: Read file content (treating missing files as satisfied) and verify pattern absence ---
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, fmt.Sprintf("file %s is absent", relPath)
		}
		return false, ""
	}
	if !matchFilePattern(string(data), pattern) {
		return true, fmt.Sprintf("file %s does not contain %s", relPath, pattern)
	}
	return false, ""
}

// splitPathAndPattern splits a "<path>:<pattern>" argument into its path and pattern components.
func splitPathAndPattern(arg string) (string, string, bool) {
	idx := strings.IndexByte(arg, ':')
	if idx <= 0 {
		return "", "", false
	}
	rawPath := strings.TrimSpace(arg[:idx])
	pattern := arg[idx+1:]
	if rawPath == "" || pattern == "" {
		return "", "", false
	}
	return rawPath, pattern, true
}

// matchFilePattern matches pattern against content using regular expressions when prefixed with "re:" or substring search otherwise.
func matchFilePattern(content, pattern string) bool {
	if strings.HasPrefix(pattern, "re:") {
		rawRe := strings.TrimPrefix(pattern, "re:")
		if rawRe == "" {
			return false
		}
		re, err := regexp.Compile(rawRe)
		if err != nil {
			return false
		}
		return re.MatchString(content)
	}
	return strings.Contains(content, pattern)
}

// evaluateWorkorderComplete checks whether a Principal Architect workorder (referenced by path or slug)
// is marked complete in its WORKORDER.md or parent PROJECT.md.
func evaluateWorkorderComplete(repoRoot, pathOrSlug string) (bool, string) {
	// --- Step 1: Resolve pathOrSlug safely within repoRoot ---
	clean, directPath, ok := resolveSafeRepoPath(repoRoot, pathOrSlug)
	if !ok {
		return false, ""
	}
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return false, ""
	}
	isInsideRoot := func(p string) bool {
		rel, rErr := filepath.Rel(absRoot, p)
		return rErr == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}

	// --- Step 2: Gather candidate WORKORDER.md and PROJECT.md files from direct path and PA workspace ---
	var candidateWOFiles []string
	var candidateProjectFiles []string
	slug := filepath.Base(strings.TrimSuffix(clean, "/WORKORDER.md"))

	if info, err := os.Stat(directPath); err == nil {
		if info.IsDir() {
			woFile := filepath.Join(directPath, "WORKORDER.md")
			if isInsideRoot(woFile) {
				if _, err := os.Stat(woFile); err == nil {
					candidateWOFiles = append(candidateWOFiles, woFile)
				}
			}
			projFile := filepath.Join(filepath.Dir(directPath), "PROJECT.md")
			if isInsideRoot(projFile) {
				if _, err := os.Stat(projFile); err == nil {
					candidateProjectFiles = append(candidateProjectFiles, projFile)
				}
			}
		} else {
			if isInsideRoot(directPath) {
				candidateWOFiles = append(candidateWOFiles, directPath)
			}
			projFile := filepath.Join(filepath.Dir(filepath.Dir(directPath)), "PROJECT.md")
			if isInsideRoot(projFile) {
				if _, err := os.Stat(projFile); err == nil {
					candidateProjectFiles = append(candidateProjectFiles, projFile)
				}
			}
		}
	}

	paRoot := filepath.Join(repoRoot, ".agents", "skills", "principal-architect-workspace")
	if info, err := os.Stat(paRoot); err == nil && info.IsDir() {
		_ = filepath.WalkDir(paRoot, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return nil
			}
			name := d.Name()
			if name == "WORKORDER.md" {
				parentSlug := filepath.Base(filepath.Dir(path))
				relToPA, _ := filepath.Rel(paRoot, filepath.Dir(path))
				relToPASlash := filepath.ToSlash(relToPA)
				if strings.EqualFold(parentSlug, slug) || strings.EqualFold(relToPASlash, clean) || strings.HasSuffix(strings.ToLower(relToPASlash), "/"+strings.ToLower(clean)) {
					candidateWOFiles = append(candidateWOFiles, path)
				}
			} else if name == "PROJECT.md" {
				candidateProjectFiles = append(candidateProjectFiles, path)
			}
			return nil
		})
	}

	// --- Step 3: Inspect candidate WORKORDER.md files for completion status or report ---
	for _, woFile := range candidateWOFiles {
		data, err := os.ReadFile(woFile)
		if err != nil {
			continue
		}
		if isWorkorderMarkdownComplete(string(data)) {
			rel, err := filepath.Rel(repoRoot, woFile)
			if err != nil {
				rel = woFile
			}
			return true, fmt.Sprintf("workorder complete: %s", filepath.ToSlash(rel))
		}
	}

	// --- Step 4: Inspect candidate PROJECT.md files for completed workorder table rows ---
	for _, projFile := range candidateProjectFiles {
		data, err := os.ReadFile(projFile)
		if err != nil {
			continue
		}
		if isSlugMarkedCompleteInProject(string(data), slug, clean) {
			rel, err := filepath.Rel(repoRoot, projFile)
			if err != nil {
				rel = projFile
			}
			return true, fmt.Sprintf("workorder %s marked complete in %s", slug, filepath.ToSlash(rel))
		}
	}

	return false, ""
}

// isWorkorderMarkdownComplete reports whether a WORKORDER.md file has a "✅ Complete" status line
// or a populated "## Completion Report" section without unreplaced template placeholders.
func isWorkorderMarkdownComplete(content string) bool {
	// --- Step 1: Scan markdown lines outside fenced code blocks ---
	inFence := false
	inCompletionReport := false
	completionReportHasTemplatePlaceholder := false
	completionReportSawContent := false

	for _, rawLine := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(rawLine)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		// --- Step 2: Check for explicit Status: ✅ Complete metadata line ---
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "status:") && strings.Contains(trimmed, "✅ Complete") {
			return true
		}

		// --- Step 3: Track "## Completion Report" section body and template placeholders ---
		if strings.HasPrefix(trimmed, "## Completion Report (") || trimmed == "## Completion Report" {
			if strings.Contains(trimmed, "<YYYY-MM-DD>") {
				inCompletionReport = false
				continue
			}
			inCompletionReport = true
			completionReportHasTemplatePlaceholder = false
			completionReportSawContent = false
			continue
		}

		if inCompletionReport {
			if strings.HasPrefix(trimmed, "## ") {
				if completionReportSawContent && !completionReportHasTemplatePlaceholder {
					return true
				}
				inCompletionReport = false
				continue
			}
			if trimmed != "" {
				completionReportSawContent = true
				if strings.Contains(trimmed, "<sha>") || strings.Contains(trimmed, "<short summary>") {
					completionReportHasTemplatePlaceholder = true
				}
			}
		}
	}

	if inCompletionReport && completionReportSawContent && !completionReportHasTemplatePlaceholder {
		return true
	}
	return false
}

// isSlugMarkedCompleteInProject reports whether a PROJECT.md line outside code fences marks slug or clean as "✅ Complete".
func isSlugMarkedCompleteInProject(content, slug, clean string) bool {
	// --- Step 1: Normalize target identifiers ---
	inFence := false
	slugLower := strings.ToLower(strings.TrimSpace(slug))
	cleanLower := strings.ToLower(strings.TrimSpace(clean))

	// --- Step 2: Scan non-fenced lines for "✅ Complete" alongside slug or clean ---
	for _, rawLine := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(rawLine)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence || trimmed == "" {
			continue
		}
		if !strings.Contains(trimmed, "✅ Complete") {
			continue
		}
		lower := strings.ToLower(trimmed)
		if (slugLower != "" && strings.Contains(lower, slugLower)) ||
			(cleanLower != "" && strings.Contains(lower, cleanLower)) {
			return true
		}
	}
	return false
}

// evaluateCommandPasses checks whether targetCmd has succeeded at or after entry.CreatedAt
// across outcome state, episode steps, or corroborated procedural dossier entries.
func evaluateCommandPasses(ctx context.Context, repoRoot, dataDir, repoID string, epStore *episode.Store, entry dossiers.Entry, targetCmd string) (bool, string) {
	// --- Step 0: Normalize targetCmd and load outcome state / episode store ---
	targetCmd = strings.TrimSpace(targetCmd)
	if targetCmd == "" {
		return false, ""
	}

	var outcomeState *skills.OutcomeState
	if strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		outcomeState, _ = skills.LoadOutcomeState(dataDir, repoID)
	}

	if epStore == nil && strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		epStore = episode.NewStore(dataDir, repoID)
	}

	// --- Step 1: Check skills.LoadOutcomeState for direct command keys or conversation outcomes recorded at/after entry.CreatedAt ---
	if outcomeState != nil && outcomeState.Entries != nil {
		for key, eo := range outcomeState.Entries {
			if eo == nil || len(eo.SucceededConversations) == 0 {
				continue
			}
			if commandMatchesTarget(key, targetCmd) {
				if entry.CreatedAt.IsZero() || anyConversationAfterEntry(epStore, eo.SucceededConversations, entry.CreatedAt) {
					return true, fmt.Sprintf("command passed in outcome state: %s", targetCmd)
				}
			}
		}
	}

	// --- Step 2: Check episodes in epStore recorded at/after entry.CreatedAt ---
	if epStore != nil {
		episodes, err := epStore.ListEpisodes()
		if err == nil {
			for _, ep := range episodes {
				if ep == nil {
					continue
				}
				if !isEpisodeAfterEntry(ep, entry.CreatedAt) {
					continue
				}
				for _, step := range ep.Steps {
					stepCmd := extractStepCommand(step)
					if stepCmd == "" {
						continue
					}
					if !isStepPassing(step, ep.Header.Status) {
						continue
					}
					if commandMatchesTarget(stepCmd, targetCmd) {
						return true, fmt.Sprintf("command passed in episode %s: %s", ep.Header.SessionID, stepCmd)
					}
				}
			}
		}
	}

	// --- Step 3: Check corroborated procedural entries in projected dossiers with LastSucceededAt >= entry.CreatedAt ---
	if strings.TrimSpace(repoRoot) != "" {
		opsDir := filepath.Join(repoRoot, ".memory", "ops")
		if info, err := os.Stat(opsDir); err == nil && info.IsDir() {
			if projected, err := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx); err == nil {
				for _, d := range projected {
					if d == nil {
						continue
					}
					for _, e := range d.Entries {
						hasSuccess := e.SuccessCount > 0
						if !hasSuccess && outcomeState != nil && outcomeState.Entries != nil {
							if eo := outcomeState.Entries[e.ID]; eo != nil && len(eo.SucceededConversations) > 0 {
								hasSuccess = true
							}
						}
						if e.Retired || e.SupersededBy != "" || !hasSuccess {
							continue
						}
						if !entry.CreatedAt.IsZero() {
							lsa := parseRFC3339(e.LastSucceededAt)
							if lsa.IsZero() {
								lsa = e.UpdatedAt
							}
							if lsa.IsZero() || lsa.Before(entry.CreatedAt) {
								continue
							}
						}
						extracted := skills.ExtractRecipeCommands(e.Text)
						if skills.CommandMatchesRecipe(targetCmd, extracted) {
							return true, fmt.Sprintf("command passed via procedural outcome on %s: %s", e.ID, targetCmd)
						}
						for _, rc := range extracted {
							if skills.CommandMatchesRecipe(rc, []string{targetCmd}) {
								return true, fmt.Sprintf("command passed via procedural outcome on %s: %s", e.ID, targetCmd)
							}
						}
					}
				}
			}
		}
	}

	return false, ""
}

// anyConversationAfterEntry reports whether at least one conversation in convIDs occurred at or after createdAt.
func anyConversationAfterEntry(epStore *episode.Store, convIDs []string, createdAt time.Time) bool {
	// --- Step 1: Handle zero createdAt or nil episode store ---
	if createdAt.IsZero() {
		return len(convIDs) > 0
	}
	if epStore == nil {
		return true
	}

	// --- Step 2: Check each conversation's episode timestamp against createdAt ---
	checkedAny := false
	for _, cid := range convIDs {
		ep, err := epStore.Read(cid)
		if err != nil || ep == nil {
			continue
		}
		checkedAny = true
		if isEpisodeAfterEntry(ep, createdAt) {
			return true
		}
	}
	return !checkedAny
}

// parseRFC3339 parses an RFC3339 or RFC3339Nano timestamp string into UTC time.Time, returning zero time on failure.
func parseRFC3339(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// evaluateAnyBuildOrTestPass checks whether any build or test step passed in an episode at or after entry.CreatedAt.
func evaluateAnyBuildOrTestPass(epStore *episode.Store, dataDir, repoID string, entry dossiers.Entry) (bool, string) {
	// --- Step 1: Ensure episode store is available and list episodes ---
	if epStore == nil && strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		epStore = episode.NewStore(dataDir, repoID)
	}
	if epStore == nil {
		return false, ""
	}
	episodes, err := epStore.ListEpisodes()
	if err != nil {
		return false, ""
	}

	// --- Step 2: Scan episodes after entry.CreatedAt for a passing test or build step ---
	for _, ep := range episodes {
		if ep == nil || !isEpisodeAfterEntry(ep, entry.CreatedAt) {
			continue
		}
		for _, step := range ep.Steps {
			if !isStepPassing(step, ep.Header.Status) {
				continue
			}
			if step.Kind == harness.StepKindTest {
				return true, fmt.Sprintf("test passed in episode %s", ep.Header.SessionID)
			}
			stepCmd := extractStepCommand(step)
			if stepCmd != "" && consolidate.IsBuildOrTestCommand(stepCmd) {
				return true, fmt.Sprintf("command passed in episode %s: %s", ep.Header.SessionID, stepCmd)
			}
		}
	}
	return false, ""
}

// isEpisodeAfterEntry reports whether episode ep occurred at or after createdAt.
func isEpisodeAfterEntry(ep *episode.Episode, createdAt time.Time) bool {
	if createdAt.IsZero() {
		return true
	}
	epTime := ep.Header.Ended
	if epTime.IsZero() {
		epTime = ep.Header.Started
	}
	if epTime.IsZero() {
		return false
	}
	return !epTime.Before(createdAt)
}

// extractStepCommand extracts the command string from step.Command, step.Suite, or step.RawData.
func extractStepCommand(step harness.Step) string {
	// --- Step 1: Check primary Command and test Suite fields ---
	cmd := strings.TrimSpace(step.Command)
	if cmd != "" {
		return cmd
	}
	if step.Kind == harness.StepKindTest {
		if suite := strings.TrimSpace(step.Suite); suite != "" {
			return suite
		}
	}

	// --- Step 2: Fall back to RawData command keys ---
	if step.RawData != nil {
		for _, k := range []string{"cmd", "command", "command_line"} {
			if v, ok := step.RawData[k].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

// isStepPassing reports whether step succeeded according to failure counts, outcome string, exit code, or headerStatus.
func isStepPassing(step harness.Step, headerStatus string) bool {
	// --- Step 1: Check explicit failure indicators and exit code ---
	if step.Failed > 0 {
		return false
	}
	outcome := strings.ToLower(strings.TrimSpace(step.Outcome))
	if outcome == "fail" || outcome == "failed" || outcome == "error" || outcome == "timeout" {
		return false
	}
	if step.ExitCode != nil {
		return *step.ExitCode == 0
	}

	// --- Step 2: Check positive outcome labels, test pass counts, RawData status, and header status ---
	if outcome == "0" || outcome == "pass" || outcome == "passed" || outcome == "ok" || outcome == "success" || outcome == "succeeded" {
		return true
	}
	if step.Kind == harness.StepKindTest && step.Passed > 0 {
		return true
	}
	if step.RawData != nil {
		if rawStatus, ok := step.RawData["status"].(string); ok {
			rs := strings.ToLower(strings.TrimSpace(rawStatus))
			if rs == "success" || rs == "passed" || rs == "ok" || rs == "0" {
				return true
			}
			if rs == "failed" || rs == "error" {
				return false
			}
		}
	}
	hs := strings.ToLower(strings.TrimSpace(headerStatus))
	if outcome == "" && (hs == "success" || hs == "passed" || hs == "completed" || hs == "ok") {
		return true
	}
	return false
}

// commandMatchesTarget reports whether executedCmd matches targetCmd via recipe matching or normalized prefix/equality.
func commandMatchesTarget(executedCmd, targetCmd string) bool {
	// --- Step 1: Trim and check recipe matcher ---
	executedCmd = strings.TrimSpace(executedCmd)
	targetCmd = strings.TrimSpace(targetCmd)
	if executedCmd == "" || targetCmd == "" {
		return false
	}
	if skills.CommandMatchesRecipe(executedCmd, []string{targetCmd}) {
		return true
	}

	// --- Step 2: Compare whitespace-normalized command tokens ---
	execFields := strings.Join(strings.Fields(executedCmd), " ")
	targetFields := strings.Join(strings.Fields(targetCmd), " ")
	if execFields == targetFields {
		return true
	}
	if strings.HasPrefix(execFields, targetFields+" ") || strings.HasPrefix(targetFields, execFields+" ") {
		return true
	}
	return false
}
