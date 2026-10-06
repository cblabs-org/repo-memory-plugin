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

	// If we have a path, we can try to derive ProjectName and WorkOrderName
	// Path pattern: .../pr-07-v2-storage/wo6-seeding/WORKORDER.md
	if path != "" {
		dir := filepath.Dir(path)
		report.WorkOrderName = filepath.Base(dir)
		report.ProjectName = filepath.Base(filepath.Dir(dir))
	}

	lines := strings.Split(content, "\n")
	
	inCompletionReport := false
	currentSection := ""
	inCodeFence := false

	for _, line := range lines {
		trimLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimLine, "```") {
			inCodeFence = !inCodeFence
			continue
		}

		if inCodeFence {
			continue
		}
		
		if strings.HasPrefix(trimLine, "## Completion Report") {
			inCompletionReport = true
			continue
		}
		
		if !inCompletionReport {
			continue
		}

		if strings.HasPrefix(trimLine, "## ") && currentSection != "" && !strings.HasPrefix(trimLine, "## Completion Report") {
			// A new ## section means we exited Completion Report
			inCompletionReport = false
			break
		}

		if strings.HasPrefix(trimLine, "### ") {
			currentSection = strings.TrimPrefix(trimLine, "### ")
			continue
		}

		if trimLine == "" || trimLine == "(none)" {
			continue
		}

		switch currentSection {
		case "Status":
			report.Status = trimLine
		case "Changes Made":
			if strings.HasPrefix(trimLine, "- ") {
				item := strings.TrimPrefix(trimLine, "- ")
				// Format: `path/to/file` — description
				// or `path/to/file` - description
				// or `path/to/file`: description
				
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

	if !inCompletionReport && report.Status == "" && len(report.Changes) == 0 && len(report.Decisions) == 0 {
		return nil, nil // No completion report found
	}

	return report, nil
}
