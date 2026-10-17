/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/dossiers/lint.go
 * Description: Token estimation and structural linting checks for projected
 *              dossiers (purpose section presence, ~3k token ceiling, and
 *              per-entry 6-line concision limits).
 *
 * Exported Types / Functions:
 * - EstimateTokens(): Returns a deterministic token estimate (~4 chars per token) for text.
 * - LintDossier():    Validates a Dossier against structural and size constraints, returning warnings.
 */

package dossiers

import (
	"fmt"
	"strings"
)

// EstimateTokens returns a deterministic token estimate (~4 chars per token) for text.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

// estimateTokens delegates to EstimateTokens for package-internal callers.
func estimateTokens(text string) int {
	return EstimateTokens(text)
}

// LintDossier validates d against structural and concision rules, returning a
// slice of human-readable warning strings.
func LintDossier(d *Dossier) []string {
	var warnings []string

	// --- Step 1: Verify that a non-empty purpose section is present ---
	if strings.TrimSpace(d.Purpose) == "" {
		warnings = append(warnings, "dossier has no purpose section")
	}

	// --- Step 2: Check rendered dossier size against the ~3k token cap ---
	rendered := RenderDossier(d)
	if len(rendered) > 12000 {
		estTokens := len(rendered) / 3
		warnings = append(warnings, fmt.Sprintf("dossier %s exceeds ~3k token cap (estimated %d tokens)", d.Scope, estTokens))
	}

	// --- Step 3: Check each active entry against the 6-line concision limit ---
	for _, e := range d.Entries {
		lines := strings.Split(strings.TrimSpace(e.Text), "\n")
		if len(lines) > 6 {
			warnings = append(warnings, fmt.Sprintf("entry %s exceeds 6 lines", e.ID))
		}
	}

	return warnings
}
