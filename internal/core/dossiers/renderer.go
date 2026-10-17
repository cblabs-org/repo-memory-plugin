/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/dossiers/renderer.go
 * Description: Deterministic Markdown rendering for active module/topic dossiers,
 *              retired entry history files, and the repository-wide ATLAS.md table.
 *
 * Exported Types / Functions:
 * - RenderDossier(): Renders active entries of a Dossier into canonical Markdown.
 * - RenderHistory(): Renders retired/superseded entries of a Dossier into Markdown.
 * - RenderAtlas():   Renders the repository-wide ATLAS.md summary table with UTC date-only timestamps.
 */

// Package dossiers projects, renders, and validates repository memory dossiers.
package dossiers

import (
	"fmt"
	"sort"
	"strings"
)

// RenderDossier renders the active entries of d into canonical v2.2 Markdown,
// omitting any optional section headings that have zero matching entries.
func RenderDossier(d *Dossier) string {
	return renderDossierInternal(d, d.Entries, false)
}

// RenderHistory renders the retired and superseded history entries of d into Markdown.
func RenderHistory(d *Dossier) string {
	return renderDossierInternal(d, d.History, true)
}

// renderDossierInternal formats either active or historical entries grouped by
// canonical v2.2 section headings.
func renderDossierInternal(d *Dossier, entries []Entry, isHistory bool) string {
	var b strings.Builder

	// --- Step 1: Render dossier title and optional purpose block ---
	title := d.Scope
	if isHistory {
		title += " (History)"
	}
	b.WriteString(fmt.Sprintf("# %s\n\n", title))

	if d.Purpose != "" {
		b.WriteString("## Purpose and boundaries\n")
		b.WriteString(d.Purpose + "\n\n")
	}

	// --- Step 2: Partition entries into canonical v2.2 section groups ---
	groups := []struct {
		title string      // Section H2 heading title.
		types []EntryType // Entry types belonging to this section.
	}{
		{"Invariants & contracts", []EntryType{EntryTypeInvariant, EntryTypeContract, EntryTypeConvention}},
		{"Ruled out, failure signatures and gotchas", []EntryType{EntryTypeRuledOut, EntryTypeFailureSignature, EntryTypeGotcha, EntryTypeExternalBehaviour}},
		{"Decisions", []EntryType{EntryTypeDecision}},
		{"Procedures / Recipes", []EntryType{EntryTypeProcedure, EntryTypeRecipe}},
		{"Interfaces and dependencies", []EntryType{EntryTypeInterface}},
		{"Open issues", []EntryType{EntryTypeOpenIssue}},
	}

	// --- Step 3: Emit non-empty sections in deterministic order ---
	for _, g := range groups {
		var groupEntries []Entry
		for _, e := range entries {
			for _, t := range g.types {
				if e.Type == t {
					groupEntries = append(groupEntries, e)
					break
				}
			}
		}

		if len(groupEntries) > 0 {
			b.WriteString(fmt.Sprintf("## %s\n\n", g.title))
			for _, e := range groupEntries {
				renderEntry(&b, e)
				b.WriteString("\n")
			}
		}
	}

	return strings.TrimSpace(b.String()) + "\n"
}

// renderEntry writes a single entry's H3 heading, backtick metadata line, and
// optional multiline body into b.
func renderEntry(b *strings.Builder, e Entry) {
	// --- Step 1: Split the first summary line from optional body lines ---
	lines := strings.Split(strings.TrimSpace(e.Text), "\n")
	titleLine := ""
	restLines := ""
	if len(lines) > 0 {
		titleLine = lines[0]
		if len(lines) > 1 {
			restLines = strings.Join(lines[1:], "\n")
		}
	}

	b.WriteString(fmt.Sprintf("### [%s] %s\n", e.ID, titleLine))

	// --- Step 2: Assemble the structured backtick metadata line ---
	var meta []string
	meta = append(meta, string(e.Type))
	meta = append(meta, "kind: "+string(e.EffectiveKind()))
	if len(e.Anchors) > 0 {
		meta = append(meta, "anchors: "+strings.Join(e.Anchors, ", "))
	}
	if e.SourceDocPath != "" {
		meta = append(meta, "source_doc: "+e.SourceDocPath)
	}
	if e.Verified != "" {
		meta = append(meta, "verified "+e.Verified)
	}
	if len(e.Evidence) > 0 {
		meta = append(meta, "evidence: "+strings.Join(e.Evidence, ", "))
	}

	b.WriteString(fmt.Sprintf("`%s`\n", strings.Join(meta, " · ")))
	if restLines != "" {
		b.WriteString(restLines + "\n")
	}
}

// RenderAtlas formats the repository-wide ATLAS.md table sorted by scope.
// LastUpdated is rendered at UTC date granularity ("YYYY-MM-DD") so intra-day
// op amendments do not churn .memory/ATLAS.md or invalidate the system-prompt
// KV cache prefix when ATLAS.md is included via GEMINI.md.
func RenderAtlas(atlas *Atlas) string {
	// --- Step 1: Write the Markdown table header ---
	var b strings.Builder
	b.WriteString("# Atlas\n\n")
	b.WriteString("| Scope | Kind | Entries | Last Updated |\n")
	b.WriteString("|---|---|---|---|\n")

	// --- Step 2: Sort dossiers deterministically by scope ---
	sorted := make([]AtlasEntry, len(atlas.Dossiers))
	copy(sorted, atlas.Dossiers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Scope < sorted[j].Scope
	})

	// --- Step 3: Format each dossier row with UTC date granularity ---
	for _, d := range sorted {
		dateStr := d.LastUpdated.UTC().Format("2006-01-02")
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %s |\n", d.Scope, d.Kind, d.EntryCount, dateStr))
	}
	return b.String()
}
