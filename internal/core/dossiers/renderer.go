package dossiers

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func RenderDossier(d *Dossier) string {
	return renderDossierInternal(d, d.Entries, false)
}

func RenderHistory(d *Dossier) string {
	return renderDossierInternal(d, d.History, true)
}

func renderDossierInternal(d *Dossier, entries []Entry, isHistory bool) string {
	var b strings.Builder

	title := d.Scope
	if isHistory {
		title += " (History)"
	}
	b.WriteString(fmt.Sprintf("# %s\n\n", title))

	if d.Purpose != "" {
		b.WriteString("## Purpose and boundaries\n")
		b.WriteString(d.Purpose + "\n\n")
	}

	groups := []struct {
		title string
		types []EntryType
	}{
		{"Invariants & contracts", []EntryType{EntryTypeInvariant, EntryTypeContract, EntryTypeConvention}},
		{"Ruled out, failure signatures and gotchas", []EntryType{EntryTypeRuledOut, EntryTypeFailureSignature, EntryTypeGotcha, EntryTypeExternalBehaviour}},
		{"Decisions", []EntryType{EntryTypeDecision}},
		{"Procedures / Recipes", []EntryType{EntryTypeProcedure, EntryTypeRecipe}},
		{"Interfaces and dependencies", []EntryType{EntryTypeInterface}},
		{"Open issues", []EntryType{EntryTypeOpenIssue}},
	}

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

func renderEntry(b *strings.Builder, e Entry) {
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

func RenderAtlas(atlas *Atlas) string {
	var b strings.Builder
	b.WriteString("# Atlas\n\n")
	b.WriteString("| Scope | Kind | Entries | Last Updated |\n")
	b.WriteString("|---|---|---|---|\n")

	// Sort dossiers by scope
	sorted := make([]AtlasEntry, len(atlas.Dossiers))
	copy(sorted, atlas.Dossiers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Scope < sorted[j].Scope
	})

	for _, d := range sorted {
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %s |\n", d.Scope, d.Kind, d.EntryCount, d.LastUpdated.Format(time.RFC3339)))
	}
	return b.String()
}
