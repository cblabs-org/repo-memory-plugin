/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/consolidate/record.go
 * Description: Implements state-aware op validation, 6-line entry text cap
 *              enforcement, 4-layer secret scrubbing on candidate ops, append-only
 *              op-batch persistence, and dossier/ATLAS projection.
 *
 * Exported Types / Functions:
 * - SourcePASeed:          Provenance source value for Principal Architect seeded operations ("pa-seed").
 * - SourceHandEdit:        Provenance source value for hand-edit migration operations ("hand-edit").
 * - OpValidationFailure:   Describes a candidate operation rejected by validation or scrubbing.
 * - RecordAndRenderResult: Holds the written op batch, projected dossiers, lint warnings, and marked note IDs.
 * - ValidateAndPrepareOps(): Validates, normalizes, and scrubs candidate ops before op-log append.
 * - RecordAndRenderDirs(): Writes validOps to opsDir, projects dossiers to renderBaseDir, and marks notes consolidated.
 * - RecordAndRenderTier(): Convenience wrapper around RecordAndRenderDirs for a single tier base directory.
 */

package consolidate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/scrub"
)

const (
	// SourcePASeed is the provenance source value for Principal Architect seeded operations.
	SourcePASeed = "pa-seed"
	// SourceHandEdit is the provenance source value for hand-edit migration operations.
	SourceHandEdit = "hand-edit"
)

// OpValidationFailure describes a candidate operation rejected by deterministic validation or scrubbing.
type OpValidationFailure struct {
	Index  int    `json:"index"`  // Zero-based index of the rejected operation in candidateOps.
	Op     ops.Op `json:"op"`     // Candidate operation that failed validation or scrubbing.
	Reason string `json:"reason"` // Human-readable explanation of why the operation was rejected.
}

// ValidateAndPrepareOps performs state-aware deterministic validation, 6-line entry text cap enforcement,
// schema validation, and 4-layer secret/blob scrubbing on candidate ops before op-log append (SPEC §6.4 step 6).
func ValidateAndPrepareOps(candidateOps []ops.Op, existing map[string]*dossiers.Dossier, scrubCtx scrub.Context, now time.Time) (validOps []ops.Op, rejected []OpValidationFailure) {
	// --- Step 1: Initialize in-memory active state tracker from existing projected dossiers ---
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	activeByScope := make(map[string]map[string]bool)
	activeTypeByScope := make(map[string]map[string]dossiers.EntryType)
	allKnownIDs := make(map[string]bool)
	scopeHasPurpose := make(map[string]bool)

	ensureScope := func(scope string) {
		if activeByScope[scope] == nil {
			activeByScope[scope] = make(map[string]bool)
		}
		if activeTypeByScope[scope] == nil {
			activeTypeByScope[scope] = make(map[string]dossiers.EntryType)
		}
	}

	for scope, d := range existing {
		if d == nil {
			continue
		}
		s := strings.TrimSpace(scope)
		if s == "" {
			s = strings.TrimSpace(d.Scope)
		}
		ensureScope(s)
		if strings.TrimSpace(d.Purpose) != "" {
			scopeHasPurpose[s] = true
		}
		for _, e := range d.Entries {
			id := strings.TrimSpace(e.ID)
			if id != "" {
				allKnownIDs[id] = true
			}
			if !e.Retired && e.SupersededBy == "" && id != "" {
				activeByScope[s][id] = true
				activeTypeByScope[s][id] = e.Type
			}
		}
		for _, e := range d.History {
			if id := strings.TrimSpace(e.ID); id != "" {
				allKnownIDs[id] = true
			}
		}
	}

	// --- Step 2: Normalize each candidate op's fields, IDs, timestamps, and aliases ---
	for i, rawOp := range candidateOps {
		op := rawOp
		op.Fields = deepCopyFields(rawOp.Fields)
		op.DossierScope = strings.TrimSpace(op.DossierScope)
		op.EntryID = strings.TrimSpace(op.EntryID)

		if op.Timestamp.IsZero() {
			op.Timestamp = now
		} else {
			op.Timestamp = op.Timestamp.UTC()
		}

		if !isAllowedConsolidationOpSource(op.Source) {
			op.Source = ops.SourceConsolidation
		}

		switch op.Type {
		case ops.OpAdd:
			if op.Fields == nil {
				op.Fields = make(map[string]any)
			}
			entryType := syncEntryTypeFields(op.Fields, "")
			if op.EntryID == "" {
				prefix := entryPrefixForType(dossiers.EntryType(entryType))
				op.EntryID = ops.NewEntryIDWithTime(prefix, now.Add(time.Duration(i)*time.Millisecond))
			}
			if op.DossierScope != "" && !scopeHasPurpose[op.DossierScope] && strings.TrimSpace(stringField(op.Fields, "purpose")) == "" {
				op.Fields["purpose"] = fmt.Sprintf("Curated repository memory for %s.", op.DossierScope)
			}

		case ops.OpSupersede:
			if op.Fields == nil {
				op.Fields = make(map[string]any)
			}
			var inheritedType string
			if scopeTypes := activeTypeByScope[op.DossierScope]; scopeTypes != nil {
				if t, ok := scopeTypes[op.EntryID]; ok {
					inheritedType = string(t)
				}
			}
			entryType := syncEntryTypeFields(op.Fields, inheritedType)
			syncTextAndNewTextFields(op.Fields)
			if strings.TrimSpace(stringField(op.Fields, "new_entry_id")) == "" {
				prefix := entryPrefixForType(dossiers.EntryType(entryType))
				op.Fields["new_entry_id"] = ops.NewEntryIDWithTime(prefix, now.Add(time.Duration(i)*time.Millisecond))
			}

		case ops.OpMove:
			if op.Fields != nil {
				syncMoveScopeFields(op.Fields)
			}

		case ops.OpSplit:
			if op.Fields != nil {
				syncSplitScopeFields(op.Fields)
			}
		}

		// --- Step 3: Enforce the <= 6 lines per entry text cap ---
		if op.Type == ops.OpAdd || op.Type == ops.OpAmend || op.Type == ops.OpSupersede {
			exceeded := false
			for _, key := range []string{"text", "new_text"} {
				if val, ok := op.Fields[key].(string); ok && strings.TrimSpace(val) != "" {
					normalized, okCap := normalizeEntryTextLines(val)
					if !okCap {
						exceeded = true
						break
					}
					op.Fields[key] = normalized
				}
			}
			if exceeded {
				rejected = append(rejected, OpValidationFailure{
					Index:  i,
					Op:     op,
					Reason: "entry text exceeds 6-line cap",
				})
				continue
			}
		}

		// --- Step 4: Verify state-aware entry ID existence and uniqueness invariants ---
		switch op.Type {
		case ops.OpAdd:
			if op.EntryID != "" && allKnownIDs[op.EntryID] {
				rejected = append(rejected, OpValidationFailure{
					Index:  i,
					Op:     op,
					Reason: "duplicate entry_id",
				})
				continue
			}

		case ops.OpAmend, ops.OpSupersede, ops.OpRetire, ops.OpMove:
			if op.EntryID != "" {
				scopeActive := activeByScope[op.DossierScope]
				if scopeActive == nil || !scopeActive[op.EntryID] {
					rejected = append(rejected, OpValidationFailure{
						Index:  i,
						Op:     op,
						Reason: "target entry_id does not exist or is not active in dossier scope",
					})
					continue
				}
			}

		case ops.OpSplit:
			scopeActive := activeByScope[op.DossierScope]
			if len(scopeActive) == 0 {
				rejected = append(rejected, OpValidationFailure{
					Index:  i,
					Op:     op,
					Reason: "source dossier scope has no active entries to split",
				})
				continue
			}
			moves := extractEntryMovesMap(op.Fields)
			invalidMoveTarget := false
			for entryID := range moves {
				if !scopeActive[strings.TrimSpace(entryID)] {
					invalidMoveTarget = true
					break
				}
			}
			if invalidMoveTarget {
				rejected = append(rejected, OpValidationFailure{
					Index:  i,
					Op:     op,
					Reason: "target entry_id does not exist or is not active in dossier scope",
				})
				continue
			}
		}

		// --- Step 5: Run schema validation and the 4-layer scrubber gate ---
		if err := ops.ValidateOp(op); err != nil {
			rejected = append(rejected, OpValidationFailure{
				Index:  i,
				Op:     op,
				Reason: err.Error(),
			})
			continue
		}

		opScrubCtx := prepareOpScrubContext(scrubCtx, op)
		scrubErr := scrubOpStringFields(&op, opScrubCtx)
		if scrubErr != nil {
			if strings.TrimSpace(opScrubCtx.DataDir) != "" && strings.TrimSpace(opScrubCtx.RepoID) != "" {
				if rawBytes, mErr := json.Marshal(op); mErr == nil {
					_, _ = scrub.QuarantinePayload(opScrubCtx.DataDir, opScrubCtx.RepoID, opScrubCtx.Boundary, scrubErr.Error(), rawBytes)
				}
			}
			rejected = append(rejected, OpValidationFailure{
				Index:  i,
				Op:     op,
				Reason: fmt.Sprintf("scrubber rejected op: %v", scrubErr),
			})
			continue
		}

		// Re-verify schema in case scrubbing emptied a required string field.
		if err := ops.ValidateOp(op); err != nil {
			rejected = append(rejected, OpValidationFailure{
				Index:  i,
				Op:     op,
				Reason: err.Error(),
			})
			continue
		}

		// --- Step 6: Update live state tracker so subsequent ops in the same batch see the transition ---
		ensureScope(op.DossierScope)
		if strings.TrimSpace(stringField(op.Fields, "purpose")) != "" {
			scopeHasPurpose[op.DossierScope] = true
		}

		switch op.Type {
		case ops.OpAdd:
			activeByScope[op.DossierScope][op.EntryID] = true
			activeTypeByScope[op.DossierScope][op.EntryID] = dossiers.EntryType(stringField(op.Fields, "entry_type"))
			allKnownIDs[op.EntryID] = true

		case ops.OpAmend:
			if et := stringField(op.Fields, "entry_type"); et != "" {
				activeTypeByScope[op.DossierScope][op.EntryID] = dossiers.EntryType(et)
			}

		case ops.OpSupersede:
			oldType := activeTypeByScope[op.DossierScope][op.EntryID]
			delete(activeByScope[op.DossierScope], op.EntryID)
			delete(activeTypeByScope[op.DossierScope], op.EntryID)
			newID := strings.TrimSpace(stringField(op.Fields, "new_entry_id"))
			if newID != "" {
				activeByScope[op.DossierScope][newID] = true
				et := dossiers.EntryType(stringField(op.Fields, "entry_type"))
				if et == "" {
					et = oldType
				}
				activeTypeByScope[op.DossierScope][newID] = et
				allKnownIDs[newID] = true
			}

		case ops.OpRetire:
			delete(activeByScope[op.DossierScope], op.EntryID)
			delete(activeTypeByScope[op.DossierScope], op.EntryID)

		case ops.OpMove:
			targetScope := strings.TrimSpace(stringField(op.Fields, "to_dossier"))
			if targetScope == "" {
				targetScope = strings.TrimSpace(stringField(op.Fields, "target_scope"))
			}
			oldType := activeTypeByScope[op.DossierScope][op.EntryID]
			delete(activeByScope[op.DossierScope], op.EntryID)
			delete(activeTypeByScope[op.DossierScope], op.EntryID)
			if targetScope != "" {
				ensureScope(targetScope)
				activeByScope[targetScope][op.EntryID] = true
				activeTypeByScope[targetScope][op.EntryID] = oldType
			}

		case ops.OpSplit:
			moves := extractEntryMovesMap(op.Fields)
			for entryID, dstScope := range moves {
				cleanID := strings.TrimSpace(entryID)
				cleanDst := strings.TrimSpace(dstScope)
				oldType := activeTypeByScope[op.DossierScope][cleanID]
				delete(activeByScope[op.DossierScope], cleanID)
				delete(activeTypeByScope[op.DossierScope], cleanID)
				if cleanDst != "" {
					ensureScope(cleanDst)
					activeByScope[cleanDst][cleanID] = true
					activeTypeByScope[cleanDst][cleanID] = oldType
				}
			}
		}

		validOps = append(validOps, op)
	}

	return validOps, rejected
}

// isAllowedConsolidationOpSource reports whether src is an allowed provenance source for consolidated operations.
func isAllowedConsolidationOpSource(src string) bool {
	switch strings.TrimSpace(src) {
	case ops.SourceConsolidation, SourcePASeed, SourceHandEdit, ops.SourceDocSeed, ops.SourceTick, ops.SourceGardening:
		return true
	default:
		return false
	}
}

// deepCopyFields returns a deep copy of an operation's Fields map so validation does not mutate caller maps.
func deepCopyFields(fields map[string]any) map[string]any {
	if fields == nil {
		return nil
	}
	cp := make(map[string]any, len(fields))
	for k, v := range fields {
		cp[k] = deepCopyValue(v)
	}
	return cp
}

// deepCopyValue recursively clones nested maps and slices inside an operation field value.
func deepCopyValue(v any) any {
	// --- Step 1: Clone map types ---
	switch val := v.(type) {
	case map[string]any:
		return deepCopyFields(val)
	case map[string]string:
		m := make(map[string]string, len(val))
		for k, s := range val {
			m[k] = s
		}
		return m
	// --- Step 2: Clone slice types and return scalar values directly ---
	case []string:
		s := make([]string, len(val))
		copy(s, val)
		return s
	case []any:
		s := make([]any, len(val))
		for i, item := range val {
			s[i] = deepCopyValue(item)
		}
		return s
	default:
		return v
	}
}

// stringField extracts a string value from fields[key], returning "" if missing or not a string.
func stringField(fields map[string]any, key string) string {
	if fields == nil {
		return ""
	}
	v, ok := fields[key].(string)
	if !ok {
		return ""
	}
	return v
}

// syncEntryTypeFields synchronizes the "type" and "entry_type" keys in fields, falling back to fallback when both are empty.
func syncEntryTypeFields(fields map[string]any, fallback string) string {
	// --- Step 1: Resolve the effective entry type from "type", "entry_type", or fallback ---
	t := strings.TrimSpace(stringField(fields, "type"))
	et := strings.TrimSpace(stringField(fields, "entry_type"))
	resolved := t
	if resolved == "" {
		resolved = et
	}
	if resolved == "" {
		resolved = strings.TrimSpace(fallback)
	}

	// --- Step 2: Populate any missing alias key in fields ---
	if resolved != "" {
		if t == "" {
			fields["type"] = resolved
		}
		if et == "" {
			fields["entry_type"] = resolved
		}
	}
	return resolved
}

// syncTextAndNewTextFields mirrors "text" and "new_text" in fields when only one is populated on an OpSupersede.
func syncTextAndNewTextFields(fields map[string]any) {
	txt := strings.TrimSpace(stringField(fields, "text"))
	newTxt := strings.TrimSpace(stringField(fields, "new_text"))
	if txt != "" && newTxt == "" {
		fields["new_text"] = stringField(fields, "text")
	} else if newTxt != "" && txt == "" {
		fields["text"] = stringField(fields, "new_text")
	}
}

// syncMoveScopeFields mirrors "target_scope" and "to_dossier" in fields for OpMove operations.
func syncMoveScopeFields(fields map[string]any) {
	ts := strings.TrimSpace(stringField(fields, "target_scope"))
	td := strings.TrimSpace(stringField(fields, "to_dossier"))
	if ts != "" && td == "" {
		fields["to_dossier"] = ts
	} else if td != "" && ts == "" {
		fields["target_scope"] = td
	}
}

// syncSplitScopeFields mirrors "new_scopes" and "new_dossiers" in fields for OpSplit operations.
func syncSplitScopeFields(fields map[string]any) {
	_, hasNS := fields["new_scopes"]
	_, hasND := fields["new_dossiers"]
	if hasNS && !hasND {
		fields["new_dossiers"] = deepCopyValue(fields["new_scopes"])
	} else if hasND && !hasNS {
		fields["new_scopes"] = deepCopyValue(fields["new_dossiers"])
	}
}

// normalizeEntryTextLines strips blank lines when text exceeds 6 lines and reports whether the result fits within 6 lines.
func normalizeEntryTextLines(text string) (string, bool) {
	// --- Step 1: Return immediately when raw line count is already <= 6 ---
	trimmed := strings.TrimSpace(text)
	lines := strings.Split(trimmed, "\n")
	if len(lines) <= 6 {
		return text, true
	}

	// --- Step 2: Collapse blank lines and re-check the 6-line ceiling ---
	var nonEmpty []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	if len(nonEmpty) > 6 {
		return text, false
	}
	return strings.Join(nonEmpty, "\n"), true
}

// extractEntryMovesMap extracts a string-to-string entry_moves map from fields regardless of whether
// it was decoded as map[string]string or map[string]any.
func extractEntryMovesMap(fields map[string]any) map[string]string {
	// --- Step 1: Handle nil fields ---
	out := make(map[string]string)
	if fields == nil {
		return out
	}

	// --- Step 2: Copy entries from typed or untyped map representations ---
	switch v := fields["entry_moves"].(type) {
	case map[string]string:
		for k, s := range v {
			out[k] = s
		}
	case map[string]any:
		for k, raw := range v {
			if s, ok := raw.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

// prepareOpScrubContext clones base and adds the operation's own entry IDs, session ID, and evidence IDs to AllowedIDs.
func prepareOpScrubContext(base scrub.Context, op ops.Op) scrub.Context {
	// --- Step 1: Copy base scrub context and default Boundary to "op" ---
	ctx := base
	if strings.TrimSpace(ctx.Boundary) == "" {
		ctx.Boundary = "op"
	}
	allowed := make(map[string]bool, len(base.AllowedIDs)+4)
	for k, v := range base.AllowedIDs {
		allowed[k] = v
	}
	addID := func(id string) {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			allowed[trimmed] = true
		}
	}

	// --- Step 2: Allowlist EntryID, SessionID, new_entry_id, and evidence references ---
	addID(op.EntryID)
	addID(op.SessionID)
	if op.Fields != nil {
		addID(stringField(op.Fields, "new_entry_id"))
		switch ev := op.Fields["evidence"].(type) {
		case []string:
			for _, s := range ev {
				addID(s)
			}
		case []any:
			for _, item := range ev {
				if s, ok := item.(string); ok {
					addID(s)
				}
			}
		case string:
			for _, part := range strings.Split(ev, ",") {
				addID(part)
			}
		}
	}
	ctx.AllowedIDs = allowed
	return ctx
}

// scrubOpStringFields runs scrub.Scrub across all free-text fields in op.Fields.
func scrubOpStringFields(op *ops.Op, scrubCtx scrub.Context) error {
	// --- Step 1: Probe scrub.Scrub once so malformed Layer 3 scrub.yaml files always fail closed ---
	if _, err := scrub.Scrub("", scrubCtx); err != nil {
		return err
	}
	if op.Fields == nil {
		return nil
	}

	// --- Step 2: Scrub each populated string field in op.Fields ---
	for _, key := range []string{"text", "new_text", "title", "purpose", "reason", "closes_when"} {
		val, ok := op.Fields[key].(string)
		if !ok || val == "" {
			continue
		}
		res, err := scrub.Scrub(val, scrubCtx)
		if err != nil {
			return err
		}
		op.Fields[key] = res.Text
	}
	return nil
}

// RecordAndRenderResult holds the written op batch, projected dossiers, lint warnings, and marked inbox note IDs.
type RecordAndRenderResult struct {
	Batch        *ops.OpBatch                 `json:"batch,omitempty"`         // Written append-only OpBatch, or nil when validOps was empty.
	Dossiers     map[string]*dossiers.Dossier `json:"dossiers"`                // Projected dossiers keyed by canonical scope.
	LintWarnings map[string][]string          `json:"lint_warnings,omitempty"` // Per-scope LintDossier warnings for active dossiers.
	MarkedNotes  []string                     `json:"marked_notes,omitempty"`  // Deduplicated inbox note IDs marked Consolidated = true.
}

// RecordAndRenderDirs writes validOps as an append-only OpBatch into opsDir, re-projects all dossiers
// and ATLAS.md into renderBaseDir, runs LintDossier cap checks, and marks consolidatedNoteIDs as Consolidated.
func RecordAndRenderDirs(ctx context.Context, opsDir string, renderBaseDir string, validOps []ops.Op, inboxStore *inbox.Store, consolidatedNoteIDs []string, now time.Time) (*RecordAndRenderResult, error) {
	// --- Step 1: Normalize context and UTC timestamp and write the append-only OpBatch ---
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	res := &RecordAndRenderResult{
		Dossiers:     make(map[string]*dossiers.Dossier),
		LintWarnings: make(map[string][]string),
	}

	if len(validOps) > 0 {
		batch := ops.NewBatchWithTime(validOps, now)
		if err := ops.NewWriter(opsDir).Write(batch); err != nil {
			return nil, fmt.Errorf("writing op batch: %w", err)
		}
		res.Batch = &batch
	}

	// --- Step 2: Re-project all dossiers from opsDir and populate default Purpose strings ---
	if err := os.MkdirAll(opsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating ops directory %s: %w", opsDir, err)
	}

	projected, err := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx)
	if err != nil {
		return nil, fmt.Errorf("projecting dossiers from %s: %w", opsDir, err)
	}

	for scope, d := range projected {
		if d == nil {
			continue
		}
		if d.Scope == "" {
			d.Scope = scope
		}
		if strings.TrimSpace(d.Purpose) == "" && len(d.Entries) > 0 {
			d.Purpose = fmt.Sprintf("Curated repository memory for %s.", d.Scope)
		}
	}

	// --- Step 3: Render projected dossiers and ATLAS.md to renderBaseDir and collect lint warnings ---
	if err := os.MkdirAll(renderBaseDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating render directory %s: %w", renderBaseDir, err)
	}

	if err := dossiers.NewWriter(renderBaseDir).WriteAll(projected); err != nil {
		return nil, fmt.Errorf("writing projected dossiers to %s: %w", renderBaseDir, err)
	}

	for scope, d := range projected {
		if d == nil || len(d.Entries) == 0 {
			continue
		}
		if warnings := dossiers.LintDossier(d); len(warnings) > 0 {
			res.LintWarnings[scope] = warnings
		}
	}
	res.Dossiers = projected

	// --- Step 4: Mark source inbox notes as Consolidated ---
	if inboxStore != nil && len(consolidatedNoteIDs) > 0 {
		seen := make(map[string]bool, len(consolidatedNoteIDs))
		for _, rawID := range consolidatedNoteIDs {
			id := strings.TrimSpace(rawID)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			if err := inboxStore.MarkConsolidated(id); err != nil {
				return nil, fmt.Errorf("marking note %s consolidated: %w", id, err)
			}
			res.MarkedNotes = append(res.MarkedNotes, id)
		}
	}

	return res, nil
}

// RecordAndRenderTier is a convenience wrapper around RecordAndRenderDirs for a single tier base directory
// where the op log resides at <tierBaseDir>/ops and dossiers render under <tierBaseDir>/dossiers and <tierBaseDir>/ATLAS.md.
func RecordAndRenderTier(ctx context.Context, tierBaseDir string, validOps []ops.Op, inboxStore *inbox.Store, consolidatedNoteIDs []string, now time.Time) (*RecordAndRenderResult, error) {
	return RecordAndRenderDirs(ctx, filepath.Join(tierBaseDir, "ops"), tierBaseDir, validOps, inboxStore, consolidatedNoteIDs, now)
}
