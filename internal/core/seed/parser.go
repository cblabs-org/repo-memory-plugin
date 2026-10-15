package seed

import (
	"path/filepath"
	"regexp"
	"strings"
)

type Change struct {
	Path        string
	Description string
}

type CompletionReport struct {
	Status        string
	Changes       []Change
	Decisions     []string
	DeadEnds      []string
	Findings      []string
	ProjectName   string
	WorkOrderName string
}

var backtickRe = regexp.MustCompile("`([^`]+)`")

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

var allowedSingleSegmentNames = map[string]bool{
	"Makefile":   true,
	"Dockerfile": true,
	"LICENSE":    true,
}

func isValidAnchorPathSyntax(p string) bool {
	p = strings.TrimSpace(p)
	p = strings.TrimSuffix(p, "/")
	if p == "" || p == "." || p == ".." || strings.Contains(p, " ") {
		return false
	}

	lower := strings.ToLower(p)
	if strings.HasPrefix(lower, "http:") || strings.HasPrefix(lower, "https:") {
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

func extractBacktickPaths(s string) []string {
	var paths []string
	matches := backtickRe.FindAllStringSubmatch(s, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		p := strings.TrimSuffix(strings.TrimSpace(match[1]), "/")
		if isValidAnchorPathSyntax(p) {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	return paths
}

func ParseCompletionReport(content string) (*CompletionReport, error) {
	return parseCompletionReportWithPath(content, "")
}

func parseCompletionReportWithPath(content string, path string) (*CompletionReport, error) {
	report := &CompletionReport{}

	if path != "" {
		dir := filepath.Dir(path)
		report.WorkOrderName = filepath.Base(dir)
		report.ProjectName = filepath.Base(filepath.Dir(dir))
	}

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
		return nil, nil // No completion report found
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

		// Template filtering
		itemVal := strings.TrimPrefix(trimLine, "- ")
		if itemVal == "Decision description and rationale" || 
		   itemVal == "What was attempted, why it failed, and what to avoid" ||
		   itemVal == "Finding description and suggested action" ||
		   strings.Contains(itemVal, "path/to/file") ||
		   itemVal == "`path/to/file` — what changed and why" {
			continue
		}

		switch currentSection {
		case "Status":
			report.Status = trimLine
		case "Changes Made":
			if strings.HasPrefix(trimLine, "- ") {
				item := strings.TrimPrefix(trimLine, "- ")
				
				paths := extractBacktickPaths(item)
				desc := item
				for _, p := range paths {
					report.Changes = append(report.Changes, Change{
						Path:        p,
						Description: desc,
					})
				}
			}
		case "Decisions Made":
			if strings.HasPrefix(trimLine, "- ") {
				report.Decisions = append(report.Decisions, strings.TrimPrefix(trimLine, "- "))
			}
		case "Dead Ends & Warnings":
			if strings.HasPrefix(trimLine, "- ") {
				report.DeadEnds = append(report.DeadEnds, strings.TrimPrefix(trimLine, "- "))
			}
		case "Out-of-Scope Findings":
			if strings.HasPrefix(trimLine, "- ") {
				report.Findings = append(report.Findings, strings.TrimPrefix(trimLine, "- "))
			}
		}
	}

	return report, nil
}
