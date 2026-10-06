package seed

import (
	"path/filepath"
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
	
	_ = true
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
				
				var pathPart, descPart string
				if idx := strings.Index(item, " — "); idx != -1 {
					pathPart = item[:idx]
					descPart = item[idx+len(" — "):]
				} else if idx := strings.Index(item, " - "); idx != -1 {
					pathPart = item[:idx]
					descPart = item[idx+len(" - "):]
				} else {
					pathPart = item
					descPart = ""
				}
				
				pathPart = strings.Trim(pathPart, "` ")
				descPart = strings.TrimSpace(descPart)
				
				if strings.Contains(pathPart, " ") || (!strings.Contains(pathPart, "/") && !strings.Contains(pathPart, ".")) {
					continue
				}
				
				report.Changes = append(report.Changes, Change{
					Path:        pathPart,
					Description: descPart,
				})
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
