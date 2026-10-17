/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/evidence/checkpoint.go
 * Description: Parses Markdown-structured checkpoint blocks from agent
 *              transcripts into canonical named sections.
 *
 * Exported Types / Functions:
 * - SectionOutstandingUserRequests: Canonical key for outstanding user requests.
 * - SectionUserKnowledge:           Canonical key for user-supplied knowledge.
 * - SectionWorkAccomplished:        Canonical key for completed work summary.
 * - SectionModelKnowledge:          Canonical key for agent-discovered knowledge.
 * - SectionFilesAndCode:            Canonical key for touched files and code notes.
 * - SectionCurrentWork:             Canonical key for active work and next steps.
 * - ParseCheckpointSections():      Extracts canonical and custom Markdown sections from checkpoint text.
 */

package evidence

import (
	"regexp"
	"strings"
)

// Canonical checkpoint section names.
const (
	SectionOutstandingUserRequests = "Outstanding User Requests" // Outstanding requests from the user.
	SectionUserKnowledge           = "User Knowledge"            // Durable facts provided by the user.
	SectionWorkAccomplished        = "Work Accomplished"         // Work completed so far in the session.
	SectionModelKnowledge          = "Model Knowledge"           // Facts discovered by the model during execution.
	SectionFilesAndCode            = "Files and Code"            // Key files and symbols inspected or modified.
	SectionCurrentWork             = "Current Work"              // In-progress tasks and immediate next steps.
)

// canonicalSections lists recognized checkpoint header titles and common synonyms.
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

// leadingNumberRegex strips leading numeric prefixes (e.g., "1. " or "2) ") from Markdown headings.
var leadingNumberRegex = regexp.MustCompile(`^\d+[\.\)]\s*`)

// ParseCheckpointSections extracts structured sections from a checkpoint content block.
// It detects Markdown headers (e.g., "## User Knowledge", "### 2. User Knowledge") and maps
// the section titles to their corresponding text content.
func ParseCheckpointSections(content string) map[string]string {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	// --- Step 1: Initialize section accumulator and line flush helper ---
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

	// --- Step 2: Scan lines for section headers and log termination markers ---
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Terminate parsing at conversation log markers or footer disclaimers.
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

	// --- Step 3: Normalize known heading synonyms to canonical section names ---
	if val, ok := sections["Current Work and Next Steps"]; ok && sections[SectionCurrentWork] == "" {
		sections[SectionCurrentWork] = val
	}
	if val, ok := sections["User Requests"]; ok && sections[SectionOutstandingUserRequests] == "" {
		sections[SectionOutstandingUserRequests] = val
	}

	return sections
}

// matchSectionHeader checks whether line is a level 1–3 Markdown heading and returns
// its normalized section title.
func matchSectionHeader(line string) (string, bool) {
	// --- Step 1: Verify Markdown heading level (1 to 3 '#' characters) ---
	if !strings.HasPrefix(line, "#") {
		return "", false
	}

	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	headerLevel := i
	if headerLevel > 3 {
		return "", false
	}

	// --- Step 2: Strip heading hashes and numeric prefixes ---
	rawTitle := strings.TrimSpace(line[i:])
	cleanedTitle := cleanTitle(rawTitle)
	if cleanedTitle == "" {
		return "", false
	}

	// --- Step 3: Match against canonical section names or level-2 custom sections ---
	for _, canonical := range canonicalSections {
		if strings.EqualFold(cleanedTitle, canonical) {
			return canonical, true
		}
	}

	for _, canonical := range canonicalSections {
		if strings.HasPrefix(strings.ToLower(cleanedTitle), strings.ToLower(canonical)) {
			return canonical, true
		}
	}

	if headerLevel == 2 && !strings.HasPrefix(cleanedTitle, "---") {
		return cleanedTitle, true
	}

	return "", false
}

// cleanTitle trims whitespace and strips any leading numbered list prefix from title.
func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	title = leadingNumberRegex.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}
