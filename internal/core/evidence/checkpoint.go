package evidence

import (
	"regexp"
	"strings"
)

// Canonical checkpoint section names.
const (
	SectionOutstandingUserRequests = "Outstanding User Requests"
	SectionUserKnowledge           = "User Knowledge"
	SectionWorkAccomplished         = "Work Accomplished"
	SectionModelKnowledge          = "Model Knowledge"
	SectionFilesAndCode            = "Files and Code"
	SectionCurrentWork             = "Current Work"
)

var canonicalSections = []string{
	SectionOutstandingUserRequests,
	"User Requests",
	SectionUserKnowledge,
	SectionWorkAccomplished,
	SectionModelKnowledge,
	SectionFilesAndCode,
	"Current Work and Next Steps",
	SectionCurrentWork,
}

var leadingNumberRegex = regexp.MustCompile(`^\d+[\.\)]\s*`)

// ParseCheckpointSections extracts structured sections from a checkpoint content block.
// It detects Markdown headers (e.g., "## User Knowledge", "### 2. User Knowledge") and maps
// the section titles to their corresponding text content.
func ParseCheckpointSections(content string) map[string]string {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	sections := make(map[string]string)
	lines := strings.Split(content, "\n")

	var currentSection string
	var currentLines []string

	flush := func() {
		if currentSection != "" {
			trimmed := strings.TrimSpace(strings.Join(currentLines, "\n"))
			if trimmed != "" {
				sections[currentSection] = trimmed
			}
			currentLines = nil
		}
	}

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Terminate parsing at conversation log markers or footer disclaimers
		if strings.HasPrefix(trimmedLine, "# Conversation Logs") ||
			strings.HasPrefix(trimmedLine, "## Conversation Logs") ||
			strings.HasPrefix(trimmedLine, "**IMPORTANT: this summary is just for your reference") {
			flush()
			currentSection = ""
			break
		}

		if headerName, ok := matchSectionHeader(trimmedLine); ok {
			flush()
			currentSection = headerName
			continue
		}

		if currentSection != "" {
			currentLines = append(currentLines, line)
		}
	}
	flush()

	// Normalize synonyms to canonical section names if not already set
	if val, ok := sections["Current Work and Next Steps"]; ok && sections[SectionCurrentWork] == "" {
		sections[SectionCurrentWork] = val
	}
	if val, ok := sections["User Requests"]; ok && sections[SectionOutstandingUserRequests] == "" {
		sections[SectionOutstandingUserRequests] = val
	}

	return sections
}

func matchSectionHeader(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}

	// Count leading '#'
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	headerLevel := i
	// Only consider #, ##, ### (levels 1-3)
	if headerLevel > 3 {
		return "", false
	}

	rawTitle := strings.TrimSpace(line[i:])
	cleanedTitle := cleanTitle(rawTitle)
	if cleanedTitle == "" {
		return "", false
	}

	// 1. Check against canonical sections
	for _, canonical := range canonicalSections {
		if strings.EqualFold(cleanedTitle, canonical) {
			return canonical, true
		}
	}

	// 2. Check if cleanedTitle starts with a canonical section name (e.g., "Current Work and Next Steps")
	for _, canonical := range canonicalSections {
		if strings.HasPrefix(strings.ToLower(cleanedTitle), strings.ToLower(canonical)) {
			return canonical, true
		}
	}

	// 3. For level 2 headers ("## Custom Section"), treat as a top-level section if it's not a generic delimiter
	if headerLevel == 2 && !strings.HasPrefix(cleanedTitle, "---") {
		return cleanedTitle, true
	}

	return "", false
}

func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	title = leadingNumberRegex.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}
