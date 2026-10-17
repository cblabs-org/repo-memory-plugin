/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/pipeline/prepare/validate.go
 * Description: Validates raw sidecar distiller output against echo-token
 *               grounding, forbidden task-execution prose, step citation
 *               integrity, and copied instruction-file content.
 *
 * Exported Types / Functions:
 * - ErrMissingEchoToken:      Sentinel error when the distiller output lacks the session's echo_token.
 * - ErrTaskExecutionDetected: Sentinel error when tool calls or task-execution prose are detected.
 * - ErrInvalidStepCitation:   Sentinel error when an item cites missing, non-existent, or stubbed steps.
 * - ErrCopiedInstructionText: Sentinel error when an item copies text from a viewed instruction file.
 * - ValidateDistillerOutput(): Enforces all 4 post-call security and grounding checks on distiller output.
 */

package prepare

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"gopkg.in/yaml.v3"
)

var (
	// ErrMissingEchoToken is returned when the distiller output does not echo the session's random token.
	ErrMissingEchoToken = errors.New("distiller output missing valid echo_token")
	// ErrTaskExecutionDetected is returned when the distiller attempts tool calls or task execution prose.
	ErrTaskExecutionDetected = errors.New("distiller output contains forbidden tool call or task execution prose")
	// ErrInvalidStepCitation is returned when an item has missing, non-existent, or stubbed step citations.
	ErrInvalidStepCitation = errors.New("distiller item has invalid or stubbed step citation")
	// ErrCopiedInstructionText is returned when an item copies instruction text from a viewed file.
	ErrCopiedInstructionText = errors.New("distiller item copies instruction text from viewed file")
)

// Compiled regular expressions for extracting fenced note blocks, echo_token lines, and step citations.
var (
	fencedNoteBlockRe = regexp.MustCompile("(?is)```(?:repo-memory-note|yaml|yml|json)[ \\t]*\\r?\\n(.*?)\\r?\\n?```") // Matches fenced YAML/JSON note blocks.
	echoTokenLineRe   = regexp.MustCompile(`(?im)^[ \t]*["']?echo_token["']?[ \t]*:[ \t]*["']?[a-fA-F0-9]+["']?[ \t]*,?[ \t]*$`) // Matches standalone echo_token key-value lines.
	evidenceStepRe    = regexp.MustCompile(`(?i)\bsteps?\s+#?(\d+)`)                                                             // Extracts numeric step references from evidence prose.
	nonWordCharRe     = regexp.MustCompile(`[^a-z0-9]+`)                                                                         // Matches non-alphanumeric runs for word tokenization.
)

// ValidateDistillerOutput verifies all 4 post-call security and grounding checks
// from SPEC.md §6.4 and design-v2 §5.6 on raw repo-memory-distiller output.
// It returns ErrMissingEchoToken, ErrTaskExecutionDetected, ErrInvalidStepCitation,
// or ErrCopiedInstructionText (discoverable via errors.Is) when validation fails.
func ValidateDistillerOutput(raw string, session *PreparedSession) ([]inbox.NoteItem, error) {
	// --- Step 1: Verify session preconditions and echoed random token ---
	if session == nil || strings.TrimSpace(session.Token) == "" {
		return nil, errors.New("prepared session with valid token is required")
	}

	// 1. Verify echoed random token.
	if !hasEchoToken(raw, session.Token) {
		return nil, fmt.Errorf("%w: expected %s", ErrMissingEchoToken, session.Token)
	}

	// --- Step 2: Reject task-doing prose or tool call tags ---
	// 2. Reject task-doing prose or tool calls.
	if err := checkNoTaskDoingOrTools(raw); err != nil {
		return nil, err
	}

	// --- Step 3: Parse note items (or NONE) and validate step citations ---
	// 3. Parse items (or NONE) and verify step citations.
	items, isNone, err := parseDistillerItems(raw)
	if err != nil {
		return nil, err
	}
	if isNone {
		return nil, nil
	}

	for idx := range items {
		it := &items[idx]
		if len(it.StepCitations) == 0 && it.Evidence != "" {
			it.StepCitations = extractStepNumbersFromEvidence(it.Evidence)
		}
		if len(it.StepCitations) == 0 {
			return nil, fmt.Errorf("%w: item %q cites no steps", ErrInvalidStepCitation, it.Title)
		}
		for _, stepIdx := range it.StepCitations {
			if session.StubbedSteps[stepIdx] {
				return nil, fmt.Errorf("%w: item %q cites stubbed file-read step %d", ErrInvalidStepCitation, it.Title, stepIdx)
			}
			if !session.ValidStepIndices[stepIdx] {
				return nil, fmt.Errorf("%w: item %q cites non-existent step %d", ErrInvalidStepCitation, it.Title, stepIdx)
			}
		}

		// --- Step 4: Reject items that copy instruction text from viewed files ---
		// 4. Reject copied instruction text from viewed files.
		if isCopiedFromInstructions(it.Why, session.InstructionSnippets) ||
			isCopiedFromInstructions(it.Do, session.InstructionSnippets) {
			return nil, fmt.Errorf("%w: item %q copies viewed instruction content", ErrCopiedInstructionText, it.Title)
		}
	}

	return items, nil
}

// hasEchoToken reports whether raw contains an echo_token field matching token (case-insensitively).
func hasEchoToken(raw, token string) bool {
	// --- Step 1: Normalize raw output and expected token ---
	lowerRaw := strings.ToLower(raw)
	lowerTok := strings.ToLower(strings.TrimSpace(token))
	if lowerTok == "" {
		return false
	}

	// --- Step 2: Check YAML and JSON echo_token formatting variants ---
	patterns := []string{
		"echo_token: " + lowerTok,
		"echo_token:" + lowerTok,
		`"echo_token": "` + lowerTok + `"`,
		`"echo_token":"` + lowerTok + `"`,
		`echo_token: "` + lowerTok + `"`,
		`echo_token: '` + lowerTok + `'`,
	}
	for _, p := range patterns {
		if strings.Contains(lowerRaw, p) {
			return true
		}
	}
	return false
}

// checkNoTaskDoingOrTools verifies that raw contains no <tool_call> tags or task-execution prose outside the fenced note block.
func checkNoTaskDoingOrTools(raw string) error {
	// --- Step 1: Strip the fenced note block and check for <tool_call> tags anywhere in raw ---
	outside := raw
	if m := fencedNoteBlockRe.FindStringSubmatch(raw); len(m) >= 2 {
		outside = strings.Replace(raw, m[0], "", 1)
	}

	// Tool call tags are forbidden anywhere in raw; tool names and task-doing prose are forbidden outside the note block
	// (and also in raw if no fenced block was used).
	if strings.Contains(strings.ToLower(raw), "<tool_call>") || strings.Contains(strings.ToLower(raw), "</tool_call>") {
		return fmt.Errorf("%w: <tool_call> tag detected", ErrTaskExecutionDetected)
	}

	// --- Step 2: Check text outside the fenced block for forbidden tool names and task-doing phrases ---
	lowerOutside := strings.ToLower(outside)
	for _, forbidden := range []string{
		"<tool_call>",
		"invoke_subagent",
		"write_to_file",
		"run_command",
		"i will now",
		"i'll now",
		"let me edit",
		"here is the implementation",
	} {
		if strings.Contains(lowerOutside, forbidden) {
			return fmt.Errorf("%w: found %q", ErrTaskExecutionDetected, forbidden)
		}
	}
	return nil
}

// rawDistillerItem extends inbox.NoteItem with alternative step-citation field aliases used by LLMs.
type rawDistillerItem struct {
	inbox.NoteItem `yaml:",inline"`                            // Embedded canonical note item fields.
	Steps          []int `yaml:"steps,omitempty" json:"steps,omitempty"`               // Alternative "steps" citation field.
	StepIndices    []int `yaml:"step_indices,omitempty" json:"step_indices,omitempty"` // Alternative "step_indices" citation field.
	Citations      []int `yaml:"citations,omitempty" json:"citations,omitempty"`       // Fallback 1-based transcript step indices under "citations" key.
}

// rawDistillerEnvelope unmarshals top-level map envelopes containing an echo_token and items/notes list.
type rawDistillerEnvelope struct {
	EchoToken string             `yaml:"echo_token" json:"echo_token"` // Anti-injection nonce echoed back from <session-log-TOKEN>.
	Items     []rawDistillerItem `yaml:"items" json:"items"`           // Extracted note items under "items" key.
	Notes     []rawDistillerItem `yaml:"notes" json:"notes"`           // Extracted note items under "notes" key.
}

// parseDistillerItems extracts inbox.NoteItem records or a NONE sentinel from raw distiller output.
func parseDistillerItems(raw string) (items []inbox.NoteItem, isNone bool, err error) {
	// --- Step 1: Extract fenced block content and strip echo_token lines ---
	content := strings.TrimSpace(raw)
	if m := fencedNoteBlockRe.FindStringSubmatch(content); len(m) >= 2 {
		content = strings.TrimSpace(m[1])
	}

	cleaned := strings.TrimSpace(echoTokenLineRe.ReplaceAllString(content, ""))
	if cleaned == "" || strings.EqualFold(cleaned, "NONE") || strings.EqualFold(strings.Trim(cleaned, "`* \t\r\n."), "NONE") || cleaned == "[]" {
		return nil, true, nil
	}

	// Check if every remaining line is just NONE.
	lines := strings.Split(cleaned, "\n")
	if len(lines) == 1 && strings.EqualFold(strings.TrimSpace(lines[0]), "NONE") {
		return nil, true, nil
	}

	// --- Step 2: Attempt YAML unmarshaling as a list, envelope, or single item ---
	var parsedList []rawDistillerItem
	if err := yaml.Unmarshal([]byte(cleaned), &parsedList); err == nil && len(parsedList) > 0 {
		return convertRawItems(parsedList), false, nil
	}

	var env rawDistillerEnvelope
	if err := yaml.Unmarshal([]byte(content), &env); err == nil {
		if len(env.Items) > 0 {
			return convertRawItems(env.Items), false, nil
		}
		if len(env.Notes) > 0 {
			return convertRawItems(env.Notes), false, nil
		}
	}

	var single rawDistillerItem
	if err := yaml.Unmarshal([]byte(cleaned), &single); err == nil && (single.Title != "" || single.Type != "") {
		return convertRawItems([]rawDistillerItem{single}), false, nil
	}

	return nil, false, errors.New("failed to parse distiller output as repo-memory-note items or NONE")
}

// convertRawItems normalizes rawDistillerItem slices into canonical inbox.NoteItem values.
func convertRawItems(rawItems []rawDistillerItem) []inbox.NoteItem {
	out := make([]inbox.NoteItem, 0, len(rawItems))
	for _, r := range rawItems {
		item := r.NoteItem
		if len(item.StepCitations) == 0 {
			if len(r.Steps) > 0 {
				item.StepCitations = r.Steps
			} else if len(r.StepIndices) > 0 {
				item.StepCitations = r.StepIndices
			} else if len(r.Citations) > 0 {
				item.StepCitations = r.Citations
			}
		}
		out = append(out, item)
	}
	return out
}

// extractStepNumbersFromEvidence parses deduplicated step numbers ("step 12", "steps #4") from an evidence string.
func extractStepNumbersFromEvidence(evidence string) []int {
	matches := evidenceStepRe.FindAllStringSubmatch(evidence, -1)
	if len(matches) == 0 {
		return nil
	}
	var out []int
	seen := make(map[int]bool)
	for _, m := range matches {
		if len(m) >= 2 {
			if n, err := strconv.Atoi(m[1]); err == nil && !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

// isCopiedFromInstructions checks whether field copies a 50+ char substring or >=60% of words from any viewed instruction snippet.
func isCopiedFromInstructions(field string, snippets []string) bool {
	// --- Step 1: Normalize the candidate field into lowercase text and word tokens ---
	trimmed := strings.TrimSpace(field)
	if trimmed == "" || len(snippets) == 0 {
		return false
	}

	normFieldSpace := normalizeWhitespaceLower(trimmed)
	fieldWords := normalizeWords(trimmed)

	// --- Step 2: Compare against each instruction snippet via substring window and word overlap ---
	for _, rawSnippet := range snippets {
		if strings.TrimSpace(rawSnippet) == "" {
			continue
		}
		normSnippetSpace := normalizeWhitespaceLower(rawSnippet)

		// Check 1: Contiguous 50+ char substring match.
		if len(normFieldSpace) >= 50 && len(normSnippetSpace) >= 50 {
			if hasContiguousSubstringMatch(normFieldSpace, normSnippetSpace, 50) {
				return true
			}
		}

		// Check 2: >= 60% of normalized words (for strings >= 8 words) match snippet words.
		if len(fieldWords) >= 8 {
			snippetWordSet := make(map[string]bool)
			for _, w := range normalizeWords(rawSnippet) {
				snippetWordSet[w] = true
			}
			matched := 0
			for _, w := range fieldWords {
				if snippetWordSet[w] {
					matched++
				}
			}
			if float64(matched)/float64(len(fieldWords)) >= 0.60 {
				return true
			}
		}
	}

	return false
}

// normalizeWhitespaceLower collapses consecutive whitespace into single spaces and lowercases s.
func normalizeWhitespaceLower(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// normalizeWords splits s into lowercase alphanumeric word tokens.
func normalizeWords(s string) []string {
	cleaned := nonWordCharRe.ReplaceAllString(strings.ToLower(s), " ")
	return strings.Fields(cleaned)
}

// hasContiguousSubstringMatch reports whether any substring of length window in field appears in snippet.
func hasContiguousSubstringMatch(field, snippet string, window int) bool {
	// --- Step 1: Guard minimum length and check exact full-string containment ---
	if len(field) < window || len(snippet) < window {
		return false
	}
	if strings.Contains(snippet, field) {
		return true
	}

	// --- Step 2: Slide a stride-5 window across field and check the trailing window ---
	for i := 0; i <= len(field)-window; i += 5 {
		if strings.Contains(snippet, field[i:i+window]) {
			return true
		}
	}
	// Check tail window as well.
	if strings.Contains(snippet, field[len(field)-window:]) {
		return true
	}
	return false
}
