package dossiers

import (
	"fmt"
	"strings"
)

func LintDossier(d *Dossier) []string {
	var warnings []string

	if strings.TrimSpace(d.Purpose) == "" {
		warnings = append(warnings, "dossier has no purpose section")
	}

	rendered := RenderDossier(d)
	if len(rendered) > 12000 {
		estTokens := len(rendered) / 3
		warnings = append(warnings, fmt.Sprintf("dossier %s exceeds ~3k token cap (estimated %d tokens)", d.Scope, estTokens))
	}

	for _, e := range d.Entries {
		lines := strings.Split(strings.TrimSpace(e.Text), "\n")
		if len(lines) > 6 {
			warnings = append(warnings, fmt.Sprintf("entry %s exceeds 6 lines", e.ID))
		}
	}

	return warnings
}
