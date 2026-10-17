/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/pipeline/compact/compact.go
 * Description: Implements deterministic transcript step compaction, including
 *               author classification, exposure/file-read payload stripping,
 *               command output truncation, repeated test/device collapsing,
 *               and pattern-based secret scrubbing.
 *
 * Exported Types / Functions:
 * - Compactor:        Performs deterministic compaction on transcript steps.
 *   - NewCompactor(): Creates a new Compactor instance.
 *   - Compact():      Reduces a slice of harness.Step records to a compacted footprint.
 */

package compact

import (
	"regexp"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// Compactor performs deterministic compaction on transcript steps.
type Compactor struct {
}

// NewCompactor creates a stateless Compactor configured with the SPEC §5.1 deterministic compaction rules.
func NewCompactor() *Compactor {
	return &Compactor{}
}

// Compact reduces a list of steps to a compacted footprint.
func (c *Compactor) Compact(steps []harness.Step) []harness.Step {
	// --- Step 1: Initialize repeat-collapsing state for consecutive test/device outputs ---
	var compacted []harness.Step

	// For collapsing repeated identical test/build/flash/soak outputs
	var lastOutput string
	var lastKind harness.StepKind
	var repeatCount int
	var lastStep *harness.Step

	flushRepeats := func() {
		if lastStep != nil {
			if repeatCount > 1 {
				// We don't have a count field in Step explicitly, but we can append to Excerpt
				lastStep.Excerpt = lastStep.Excerpt + " (repeated " + string(rune(repeatCount)) + " times)" // rough, but actually we should just collapse
			}
			compacted = append(compacted, *lastStep)
			lastStep = nil
			repeatCount = 0
		}
	}

	// --- Step 2: Compact each step and collapse consecutive identical test/device runs ---
	for _, s := range steps {
		s = c.compactStep(s)
		if s.Kind == "" {
			continue // dropped
		}

		// If it's test/build/flash/soak (Test, Device), collapse identical outputs
		if s.Kind == harness.StepKindTest || s.Kind == harness.StepKindDevice {
			if lastStep != nil && lastKind == s.Kind && s.Excerpt == lastOutput {
				repeatCount++
				continue
			} else {
				flushRepeats()
				kind := s.Kind
				excerpt := s.Excerpt
				sCopy := s
				lastStep = &sCopy
				lastKind = kind
				lastOutput = excerpt
				repeatCount = 1
				continue
			}
		} else {
			flushRepeats()
			compacted = append(compacted, s)
		}
	}
	flushRepeats()

	return compacted
}

// compactStep applies per-step compaction rules (author classification, payload dropping, command truncation, and secret scrubbing).
func (c *Compactor) compactStep(s harness.Step) harness.Step {
	// --- Step 1: Classify user prompt author and strip exposure or file-read payloads ---
	// 1. Author classification
	if s.Kind == harness.StepKindUser {
		s.Author = classifyAuthor(s.Text)
	}

	// 2. Exposure markers
	if s.Kind == harness.StepKindExposure || s.Via != "" {
		// Exposure markers: drop actual memory content but keep references
		s.Text = ""
	}

	// 3. File Reads and Edits
	if s.Kind == harness.StepKindRead || s.Kind == harness.StepKindEdit {
		// Keep the path (in s.File) but drop the content (s.Text)
		s.Text = ""
	}

	// --- Step 2: Truncate command output excerpts and scrub secrets across all text fields ---
	// 4. Commands
	if s.Kind == harness.StepKindCommand {
		// Drop command output, except error/failure lines and summary lines (bounded excerpt)
		s.Excerpt = truncateCommandExcerpt(s.Excerpt, s.ExitCode)
	}

	// 5. Secret scrubber
	if s.Text != "" {
		s.Text = scrubSecrets(s.Text)
	}
	if s.Excerpt != "" {
		s.Excerpt = scrubSecrets(s.Excerpt)
	}
	if s.Command != "" {
		s.Command = scrubSecrets(s.Command)
	}

	return s
}

// truncateCommandExcerpt retains up to 10 error, failure, or summary lines (or the trailing 5 lines on non-zero exit).
func truncateCommandExcerpt(excerpt string, exitCode *int) string {
	// --- Step 1: Filter command output lines for diagnostic keywords ---
	if excerpt == "" {
		return ""
	}

	lines := strings.Split(excerpt, "\n")
	var kept []string

	// Simple bounded excerpt logic:
	// If exit code is 0 (success) or nil, maybe just keep last few lines or nothing?
	// The rule: "Command output, except error/failure lines and summary lines"
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") ||
			strings.Contains(lower, "fail") ||
			strings.Contains(lower, "fatal") ||
			strings.Contains(lower, "exception") ||
			strings.Contains(lower, "summary") {
			kept = append(kept, line)
		}
	}

	// --- Step 2: Fall back to trailing lines on failure and bound total excerpt length ---
	// If it's a failure (exit != 0), and we found no explicit errors, keep the last 5 lines.
	if len(kept) == 0 && exitCode != nil && *exitCode != 0 {
		start := 0
		if len(lines) > 5 {
			start = len(lines) - 5
		}
		kept = lines[start:]
	}

	// Bound the excerpt
	if len(kept) > 10 {
		kept = kept[len(kept)-10:]
	}

	return strings.Join(kept, "\n")
}

// classifyAuthor determines whether a user step originated from "automation" or a "human".
func classifyAuthor(text string) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "agentapi") ||
		strings.Contains(lower, "bug-fixer") ||
		strings.Contains(lower, "desktop scheduled task") ||
		strings.Contains(lower, "-p run") ||
		strings.Contains(lower, "system generated") ||
		strings.HasPrefix(lower, "<system_message>") {
		return "automation"
	}
	return "human"
}

// scrubSecrets replaces AWS keys, UUIDs, bearer tokens, and long hex strings with redacted placeholders.
func scrubSecrets(text string) string {
	// --- Step 1: Redact structured cloud keys and UUIDs ---
	// Secret scrubber (pattern + entropy) replacing matches with «secret:type»

	// Mock AWS key
	awsKeyRegex := regexp.MustCompile(`(?i)(?:AKIA|A3T|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`)
	text = awsKeyRegex.ReplaceAllString(text, "«secret:aws»")

	// UUID
	uuidRegex := regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	text = uuidRegex.ReplaceAllString(text, "«secret:uuid»")

	// --- Step 2: Redact bearer tokens and high-entropy hexadecimal strings ---
	// High entropy base64-like strings (tokens)
	// Just a simple heuristic for tests
	tokenRegex := regexp.MustCompile(`(?i)(?:bearer|token)\s+([a-zA-Z0-9\-\.]{20,})`)
	text = tokenRegex.ReplaceAllStringFunc(text, func(m string) string {
		parts := strings.SplitN(m, " ", 2)
		if len(parts) == 2 {
			return parts[0] + " «secret:token»"
		}
		return "«secret:token»"
	})

	// Simple hex key entropy
	hexRegex := regexp.MustCompile(`[0-9a-fA-F]{32,}`)
	text = hexRegex.ReplaceAllString(text, "«secret:hex»")

	return text
}
