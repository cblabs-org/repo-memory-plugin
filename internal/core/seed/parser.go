/*
 * repo-memory-plugin (c) 2026 CBLabs
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/seed/parser.go
 * Description: Parses Principal Architect WORKORDER.md and PROJECT.md completion
 *              reports across both legacy v1 Markdown H3 sections and v0.1.0+
 *              Split-Authority YAML frontmatter completion blocks.
 *
 * Exported Types / Functions:
 * - Change:                  Represents a file anchor path and its change description.
 * - CompletionReport:        Normalized decisions, dead ends, findings, and file changes.
 * - IsValidAnchorPathSyntax(): Validates repository-relative file or symbol anchor syntax.
 * - ParseCompletionReport(): Parses a WORKORDER.md completion report.
 * - ParseProjectReport():    Parses architectural distillation decisions from a PROJECT.md file.
 */

package seed

import (
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Change associates a repository-relative file path with its modification summary.
type Change struct {
	Path        string // Repository-relative path (optionally with #symbol or :symbol).
	Description string // Human-readable summary of the modification.
}

// CompletionReport holds normalized findings extracted from a PA work order or project report.
type CompletionReport struct {
	Status        string   // Completion status string (e.g. "complete" or "COMPLETED").
	Changes       []Change // Anchored file modifications.
	Decisions     []string // Architectural or implementation decisions made.
	DeadEnds      []string // Ruled-out approaches and warnings.
	Findings      []string // Out-of-scope findings and open issues.
	ProjectName   string   // Enclosing PR directory or slug name.
	WorkOrderName string   // Work order directory or identifier.
}

// workorderFrontmatter models the YAML frontmatter of a v0.1.0+ Split-Authority WORKORDER.md.
type workorderFrontmatter struct {
	WOID         string                   `yaml:"wo_id"`
	PRID         string                   `yaml:"pr_id"`
	Status       string                   `yaml:"status"`
	AllowedFiles []string                 `yaml:"allowed_files"`
	Completion   *workorderCompletionYAML `yaml:"completion"`
}

// workorderCompletionYAML models the structured Part II completion block in WORKORDER.md frontmatter.
type workorderCompletionYAML struct {
	Status             string   `yaml:"status"`
	FilesModified      []string `yaml:"files_modified"`
	DecisionsMade      []string `yaml:"decisions_made"`
	DeadEnds           []string `yaml:"dead_ends"`
	OutOfScopeFindings []string `yaml:"out_of_scope_findings"`
}

// projectFrontmatter models the YAML frontmatter of a v0.1.0+ Split-Authority PROJECT.md.
type projectFrontmatter struct {
	Status  string `yaml:"status"`
	Charter struct {
		PRID           string   `yaml:"pr_id"`
		Slug           string   `yaml:"slug"`
		AllowedModules []string `yaml:"allowed_modules"`
	} `yaml:"charter"`
}

var backtickRe = regexp.MustCompile("`([^`]+)`") // Matches inline code spans in Markdown text.

// allowedSingleSegmentExts enumerates file extensions valid for root-level single-segment paths.
var allowedSingleSegmentExts = map[string]bool{
	".go":        true,
	".md":        true,
	".yaml":      true,
	".yml":       true,
	".json":      true,
	".toml":      true,
	".sh":        true,
	".txt":       true,
	".mod":       true,
	".sum":       true,
	".sql":       true,
	".py":        true,
	".js":        true,
	".ts":        true,
	".c":         true,
	".h":         true,
	".cpp":       true,
	".rs":        true,
	".proto":     true,
	".mk":        true,
	".gitignore": true,
}

// allowedSingleSegmentNames enumerates extensionless root-level filenames valid as anchors.
var allowedSingleSegmentNames = map[string]bool{
	"Makefile":   true,
	"Dockerfile": true,
	"LICENSE":    true,
}

// isValidAnchorSymbol reports whether s is a clean identifier suitable for #symbol or :symbol anchoring.
func isValidAnchorSymbol(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}

// stripAnchorSymbol splits an optional #symbol or :symbol suffix from p and returns the file path portion.
func stripAnchorSymbol(p string) (filePart string, ok bool) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", false
	}
	lower := strings.ToLower(p)
	if strings.HasPrefix(lower, "http:") || strings.HasPrefix(lower, "https:") || strings.Contains(p, "://") {
		return "", false
	}
	sepCount := strings.Count(p, "#") + strings.Count(p, ":")
	if sepCount > 1 {
		return "", false
	}
	if sepCount == 1 {
		idx := strings.IndexAny(p, "#:")
		filePart = strings.TrimSuffix(p[:idx], "/")
		symPart := p[idx+1:]
		if filePart == "" || symPart == "" || !isValidAnchorSymbol(symPart) {
			return "", false
		}
		return filePart, true
	}
	return strings.TrimSuffix(p, "/"), true
}

// IsValidAnchorPathSyntax returns true when p looks like a valid repository-relative
// file or directory path (optionally with a #symbol or :symbol suffix) rather than
// an arbitrary code token, glob, or external path.
func IsValidAnchorPathSyntax(p string) bool {
	filePart, ok := stripAnchorSymbol(p)
	if !ok {
		return false
	}
	p = filePart
	if p == "" || p == "." || p == ".." || strings.Contains(p, " ") {
		return false
	}

	switch p[0] {
	case '~', '/', '*', '!', '<', '>', '-', '$':
		return false
	}

	if strings.ContainsAny(p, "<>*?()[]{}=;:\\\"'$|&") {
		return false
	}
	if strings.Contains(p, "...") || strings.Contains(p, "//") {
		return false
	}

	segments := strings.Split(p, "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}

	if len(segments) == 1 {
		if allowedSingleSegmentNames[p] {
			return true
		}
		ext := strings.ToLower(filepath.Ext(p))
		return allowedSingleSegmentExts[ext]
	}

	firstSeg := segments[0]
	if idx := strings.IndexByte(firstSeg, '.'); idx > 0 {
		return false
	}

	return true
}

func isValidAnchorPathSyntax(p string) bool {
	return IsValidAnchorPathSyntax(p)
}

// extractBacktickPaths extracts deduplicated valid anchor paths from backtick code spans in s.
func extractBacktickPaths(s string) []string {
	var paths []string
	matches := backtickRe.FindAllStringSubmatch(s, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		p := strings.TrimSuffix(strings.TrimSpace(match[1]), "/")
		if isValidAnchorPathSyntax(p) {
			if idx := strings.IndexAny(p, "#:"); idx >= 0 {
				p = strings.TrimSuffix(p[:idx], "/") + p[idx:]
			}
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	return paths
}

// isTemplatePlaceholder reports whether itemVal is an unpopulated PA template placeholder string.
func isTemplatePlaceholder(itemVal string) bool {
	return itemVal == "Decision description and rationale" ||
		itemVal == "What was attempted, why it failed, and what to avoid" ||
		itemVal == "Finding description and suggested action" ||
		strings.Contains(itemVal, "path/to/file") ||
		itemVal == "`path/to/file` — what changed and why"
}

// extractYAMLFrontmatter extracts the raw YAML frontmatter block delimited by leading "---" lines.
func extractYAMLFrontmatter(content string) (string, bool) {
	trimmed := strings.TrimPrefix(content, "\uFEFF")
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), true
		}
	}
	return "", false
}

// ParseCompletionReport parses a WORKORDER.md string into a normalized CompletionReport.
func ParseCompletionReport(content string) (*CompletionReport, error) {
	return parseCompletionReportWithPath(content, "")
}

// parseCompletionReportWithPath parses both v0.1.0+ YAML frontmatter completion blocks and
// legacy v1 Markdown H3 sections under "## Completion Report".
func parseCompletionReportWithPath(content string, filePath string) (*CompletionReport, error) {
	report := &CompletionReport{}

	if filePath != "" {
		dir := filepath.Dir(filePath)
		report.WorkOrderName = filepath.Base(dir)
		report.ProjectName = filepath.Base(filepath.Dir(dir))
	}

	seenChanges := make(map[string]bool)
	seenDecisions := make(map[string]bool)
	seenDeadEnds := make(map[string]bool)
	seenFindings := make(map[string]bool)

	addChange := func(rawPath, desc string) {
		p := strings.TrimSuffix(strings.TrimSpace(strings.Trim(rawPath, "`")), "/")
		if p == "" || !isValidAnchorPathSyntax(p) {
			return
		}
		if idx := strings.IndexAny(p, "#:"); idx >= 0 {
			p = strings.TrimSuffix(p[:idx], "/") + p[idx:]
		}
		if !seenChanges[p] {
			seenChanges[p] = true
			report.Changes = append(report.Changes, Change{
				Path:        p,
				Description: desc,
			})
		}
	}

	// --- Step 1: Parse v0.1.0+ Split-Authority YAML frontmatter completion block ---
	hasFrontmatterCompletion := false
	if yamlBlock, ok := extractYAMLFrontmatter(content); ok {
		var fm workorderFrontmatter
		if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err == nil && fm.Completion != nil {
			hasFrontmatterCompletion = true
			report.Status = strings.TrimSpace(fm.Completion.Status)
			if report.Status == "" {
				report.Status = strings.TrimSpace(fm.Status)
			}

			modFiles := fm.Completion.FilesModified
			if len(modFiles) == 0 {
				modFiles = fm.AllowedFiles
			}
			for _, rawP := range modFiles {
				addChange(rawP, rawP)
				for _, bp := range extractBacktickPaths(rawP) {
					addChange(bp, rawP)
				}
			}

			for _, dec := range fm.Completion.DecisionsMade {
				dec = strings.TrimSpace(dec)
				if dec == "" || isTemplatePlaceholder(dec) || seenDecisions[dec] {
					continue
				}
				seenDecisions[dec] = true
				report.Decisions = append(report.Decisions, dec)
				for _, bp := range extractBacktickPaths(dec) {
					addChange(bp, dec)
				}
			}

			for _, de := range fm.Completion.DeadEnds {
				de = strings.TrimSpace(de)
				if de == "" || isTemplatePlaceholder(de) || seenDeadEnds[de] {
					continue
				}
				seenDeadEnds[de] = true
				report.DeadEnds = append(report.DeadEnds, de)
				for _, bp := range extractBacktickPaths(de) {
					addChange(bp, de)
				}
			}

			for _, finding := range fm.Completion.OutOfScopeFindings {
				finding = strings.TrimSpace(finding)
				if finding == "" || isTemplatePlaceholder(finding) || seenFindings[finding] {
					continue
				}
				seenFindings[finding] = true
				report.Findings = append(report.Findings, finding)
				for _, bp := range extractBacktickPaths(finding) {
					addChange(bp, finding)
				}
			}
		}
	}

	// --- Step 2: Locate and parse Markdown "## Completion Report" H3 sections ---
	lines := strings.Split(content, "\n")
	lastReportIdx := -1
	inCodeFencePass1 := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			inCodeFencePass1 = !inCodeFencePass1
			continue
		}
		if !inCodeFencePass1 && strings.HasPrefix(trim, "## Completion Report") {
			lastReportIdx = i
		}
	}

	if lastReportIdx == -1 {
		if hasFrontmatterCompletion {
			return report, nil
		}
		return nil, nil
	}

	currentSection := ""
	inCodeFence := false

	for _, line := range lines[lastReportIdx+1:] {
		trimLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimLine, "```") {
			inCodeFence = !inCodeFence
			continue
		}

		if inCodeFence {
			continue
		}

		if strings.HasPrefix(trimLine, "## ") && currentSection != "" && !strings.HasPrefix(trimLine, "## Completion Report") {
			break
		}

		if strings.HasPrefix(trimLine, "### ") {
			currentSection = strings.TrimPrefix(trimLine, "### ")
			continue
		}

		if trimLine == "" || trimLine == "(none)" {
			continue
		}

		itemVal := strings.TrimPrefix(trimLine, "- ")
		if isTemplatePlaceholder(itemVal) {
			continue
		}

		switch currentSection {
		case "Status":
			if report.Status == "" {
				report.Status = trimLine
			}
		case "Changes Made":
			if strings.HasPrefix(trimLine, "- ") {
				item := strings.TrimPrefix(trimLine, "- ")
				paths := extractBacktickPaths(item)
				for _, p := range paths {
					addChange(p, item)
				}
			}
		case "Decisions Made":
			if strings.HasPrefix(trimLine, "- ") {
				val := strings.TrimPrefix(trimLine, "- ")
				if !seenDecisions[val] {
					seenDecisions[val] = true
					report.Decisions = append(report.Decisions, val)
				}
			}
		case "Dead Ends & Warnings":
			if strings.HasPrefix(trimLine, "- ") {
				val := strings.TrimPrefix(trimLine, "- ")
				if !seenDeadEnds[val] {
					seenDeadEnds[val] = true
					report.DeadEnds = append(report.DeadEnds, val)
				}
			}
		case "Out-of-Scope Findings":
			if strings.HasPrefix(trimLine, "- ") {
				val := strings.TrimPrefix(trimLine, "- ")
				if !seenFindings[val] {
					seenFindings[val] = true
					report.Findings = append(report.Findings, val)
				}
			}
		}
	}

	return report, nil
}

// ParseProjectReport parses architectural decisions and invariants from a PA PROJECT.md file.
func ParseProjectReport(content string) (*CompletionReport, error) {
	return parseProjectReportWithPath(content, "")
}

// parseProjectReportWithPath extracts Part III architectural distillation bullets from a PROJECT.md file.
func parseProjectReportWithPath(content string, filePath string) (*CompletionReport, error) {
	if !strings.Contains(content, "Key Architectural Decisions") {
		return nil, nil
	}

	report := &CompletionReport{}
	if filePath != "" {
		dir := filepath.Dir(filePath)
		report.ProjectName = filepath.Base(dir)
		report.WorkOrderName = "PROJECT"
	}

	seenChanges := make(map[string]bool)
	addChange := func(rawPath, desc string) {
		p := strings.TrimSuffix(strings.TrimSpace(strings.Trim(rawPath, "`")), "/")
		if p == "" || !isValidAnchorPathSyntax(p) {
			return
		}
		if idx := strings.IndexAny(p, "#:"); idx >= 0 {
			p = strings.TrimSuffix(p[:idx], "/") + p[idx:]
		}
		if !seenChanges[p] {
			seenChanges[p] = true
			report.Changes = append(report.Changes, Change{
				Path:        p,
				Description: desc,
			})
		}
	}

	// --- Step 1: Extract charter.allowed_modules from YAML frontmatter if present ---
	if yamlBlock, ok := extractYAMLFrontmatter(content); ok {
		var fm projectFrontmatter
		if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err == nil {
			report.Status = strings.TrimSpace(fm.Status)
			if fm.Charter.Slug != "" {
				report.ProjectName = strings.TrimSpace(fm.Charter.Slug)
			}
			for _, mod := range fm.Charter.AllowedModules {
				addChange(mod, mod)
			}
		}
	}

	// --- Step 2: Extract sub-bullets under "Key Architectural Decisions & Invariants Preserved" ---
	lines := strings.Split(content, "\n")
	inDecisionsBullet := false
	inDecisionsHeading := false
	inCodeFence := false
	seenDecisions := make(map[string]bool)

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			inCodeFence = !inCodeFence
			continue
		}
		if inCodeFence {
			continue
		}

		if strings.Contains(trim, "Key Architectural Decisions") {
			if strings.HasPrefix(trim, "## ") || strings.HasPrefix(trim, "### ") {
				inDecisionsHeading = true
				inDecisionsBullet = false
				continue
			}
			if strings.HasPrefix(trim, "-") {
				inDecisionsBullet = true
				inDecisionsHeading = false
				continue
			}
		}

		if !inDecisionsBullet && !inDecisionsHeading {
			continue
		}

		if strings.HasPrefix(trim, "## ") || strings.HasPrefix(trim, "### ") {
			inDecisionsBullet = false
			inDecisionsHeading = false
			continue
		}

		// Top-level bullet (no leading indentation) ends an indented sub-bullet block.
		if inDecisionsBullet && strings.HasPrefix(line, "- ") {
			inDecisionsBullet = false
			continue
		}

		if strings.HasPrefix(trim, "- ") {
			val := strings.TrimSpace(strings.TrimPrefix(trim, "- "))
			if val != "" && !isTemplatePlaceholder(val) && !seenDecisions[val] {
				seenDecisions[val] = true
				report.Decisions = append(report.Decisions, val)
				for _, bp := range extractBacktickPaths(val) {
					addChange(bp, val)
				}
			}
		}
	}

	if len(report.Decisions) == 0 && len(report.DeadEnds) == 0 && len(report.Findings) == 0 {
		return nil, nil
	}
	return report, nil
}
