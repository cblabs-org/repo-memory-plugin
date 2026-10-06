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

func extractBacktickPaths(s string) []string {
	var paths []string
	re := regexp.MustCompile("`([^`]+)`")
	matches := re.FindAllStringSubmatch(s, -1)
	
	seen := make(map[string]bool)
	for _, match := range matches {
		p := match[1]
		p = strings.TrimPrefix(p, "**")
		p = strings.TrimSuffix(p, "**:")
		p = strings.TrimSuffix(p, "**")
		
		if (strings.Contains(p, "/") || strings.Contains(p, ".")) && !strings.Contains(p, " ") {
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
