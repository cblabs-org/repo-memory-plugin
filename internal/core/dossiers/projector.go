/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/dossiers/projector.go
 * Description: Deterministic state projection of ordered operation batches
 *              (OpAdd, OpAmend, OpSupersede, OpRetire, OpMove, OpSplit) into
 *              in-memory Dossier structures sorted by section priority and ID.
 *
 * Exported Types / Functions:
 * - Projector:          Projects operation log batches into active and historical dossier state.
 *   - NewProjector():   Creates a Projector backed by an ops.Reader.
 *   - Project():        Replays all operation batches from the reader into a scope-keyed Dossier map.
 *   - ProjectBefore():  Replays operation batches committed prior to beforeCommit.
 */

package dossiers

import (
	"context"
	"iter"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// Projector folds ordered operation batches from an ops.Reader into a map of
// projected Dossier instances.
type Projector struct {
	reader *ops.Reader // Underlying operation batch reader.
}

// NewProjector returns a new Projector backed by opsReader.
func NewProjector(opsReader *ops.Reader) *Projector {
	return &Projector{
		reader: opsReader,
	}
}

// projectFromSeq iterates through seq, applies each batch in order, and sorts
// all resulting dossiers deterministically.
func (p *Projector) projectFromSeq(seq iter.Seq2[*ops.OpBatch, error]) (map[string]*Dossier, error) {
	// --- Step 1: Replay operation batches into the in-memory dossier map ---
	dossiers := make(map[string]*Dossier)
	entryIndex := make(map[string]string)

	for batch, err := range seq {
		if err != nil {
			return nil, err
		}
		if err := applyBatch(dossiers, entryIndex, batch); err != nil {
			return nil, err
		}
	}

	// --- Step 2: Sort active and historical entries in each projected dossier ---
	for _, d := range dossiers {
		sortDossier(d)
	}

	return dossiers, nil
}

// Project replays all operation batches from the underlying reader and returns
// the projected dossiers keyed by scope name.
func (p *Projector) Project(ctx context.Context) (map[string]*Dossier, error) {
	return p.projectFromSeq(p.reader.Iterate(ctx))
}

// ProjectBefore replays all operation batches committed prior to beforeCommit
// and returns the projected dossiers keyed by scope name.
func (p *Projector) ProjectBefore(ctx context.Context, beforeCommit string) (map[string]*Dossier, error) {
	return p.projectFromSeq(p.reader.IterateBefore(ctx, beforeCommit))
}

// applyBatch applies every operation in batch to dossiers and updates entryIndex.
func applyBatch(dossiers map[string]*Dossier, entryIndex map[string]string, batch *ops.OpBatch) error {
	// --- Step 1: Dispatch each operation in the batch by its OpType ---
	for _, op := range batch.Ops {
		switch op.Type {
		case ops.OpAdd:
			applyAdd(dossiers, entryIndex, op)
		case ops.OpAmend:
			applyAmend(dossiers, entryIndex, op)
		case ops.OpSupersede:
			applySupersede(dossiers, entryIndex, op)
		case ops.OpRetire:
			applyRetire(dossiers, entryIndex, op)
		case ops.OpMove:
			applyMove(dossiers, entryIndex, op)
		case ops.OpSplit:
			applySplit(dossiers, entryIndex, op)
		}
	}
	return nil
}

// getOrCreateDossier returns the existing Dossier for scope or initializes a new
// module-kind Dossier when not yet present.
func getOrCreateDossier(dossiers map[string]*Dossier, scope string) *Dossier {
	d, ok := dossiers[scope]
	if !ok {
		d = &Dossier{
			Scope: scope,
			Kind:  DossierKindModule,
		}
		dossiers[scope] = d
	}
	return d
}

// extractIntField reads an integer value (int, int64, or float64) from fields[key].
func extractIntField(fields map[string]any, key string) (int, bool) {
	// --- Step 1: Lookup key and convert numeric JSON/YAML types to int ---
	v, exists := fields[key]
	if !exists {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// extractKinds inspects fields for `kind`, `cognitive_kind`, and `dossier_kind`
// values and returns any recognized CognitiveKind and DossierKind overrides.
func extractKinds(fields map[string]any) (ck CognitiveKind, hasCK bool, dk DossierKind, hasDK bool) {
	// --- Step 1: Inspect generic `kind` field for either CognitiveKind or DossierKind ---
	if k, ok := fields["kind"].(string); ok {
		if CognitiveKind(k).Valid() {
			ck = CognitiveKind(k)
			hasCK = true
		} else {
			switch DossierKind(k) {
			case DossierKindModule, DossierKindTopic, DossierKindExternal, DossierKindProcedure:
				dk = DossierKind(k)
				hasDK = true
			}
		}
	}

	// --- Step 2: Check explicit `cognitive_kind` and `dossier_kind` overrides ---
	if k, ok := fields["cognitive_kind"].(string); ok && CognitiveKind(k).Valid() {
		ck = CognitiveKind(k)
		hasCK = true
	}
	if k, ok := fields["dossier_kind"].(string); ok && k != "" {
		dk = DossierKind(k)
		hasDK = true
	}
	return ck, hasCK, dk, hasDK
}

// applyAdd inserts a new Entry into its target dossier (or amends/moves an
// existing entry if op.EntryID was already projected).
func applyAdd(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	// --- Step 1: Handle duplicate EntryID as an amendment (with cross-scope move if needed) ---
	if existingScope, exists := entryIndex[op.EntryID]; exists {
		if existingScope != op.DossierScope && op.DossierScope != "" {
			// Scope changed — move entry across scopes before amending
			moveEntry(dossiers, entryIndex, op.EntryID, existingScope, op.DossierScope, op.Timestamp)
		}
		applyAmend(dossiers, entryIndex, op)
		return
	}

	// --- Step 2: Resolve target dossier and apply optional dossier-level metadata ---
	d := getOrCreateDossier(dossiers, op.DossierScope)

	ck, hasCK, dk, hasDK := extractKinds(op.Fields)
	if hasDK {
		d.Kind = dk
	}
	if title, ok := op.Fields["title"].(string); ok {
		d.Title = title
	}
	if purpose, ok := op.Fields["purpose"].(string); ok {
		d.Purpose = purpose
	}

	// --- Step 3: Populate new Entry fields from operation payload ---
	entry := Entry{
		ID:           op.EntryID,
		DossierScope: op.DossierScope,
		Source:       op.Source,
		CreatedAt:    op.Timestamp,
		UpdatedAt:    op.Timestamp,
	}

	if t, ok := op.Fields["entry_type"].(string); ok {
		entry.Type = EntryType(t)
	}
	if hasCK {
		entry.Kind = ck
	}
	if entry.Kind == "" {
		entry.Kind = DeriveCognitiveKind(entry.Type)
	}
	if text, ok := op.Fields["text"].(string); ok {
		entry.Text = text
	}
	if anchors, ok := op.Fields["anchors"].([]any); ok {
		for _, a := range anchors {
			if s, ok := a.(string); ok {
				entry.Anchors = append(entry.Anchors, s)
			}
		}
	} else if anchorsStr, ok := op.Fields["anchors"].([]string); ok {
		entry.Anchors = append(entry.Anchors, anchorsStr...)
	}

	if evidence, ok := op.Fields["evidence"].([]any); ok {
		for _, e := range evidence {
			if s, ok := e.(string); ok {
				entry.Evidence = append(entry.Evidence, s)
			}
		}
	} else if evStr, ok := op.Fields["evidence"].([]string); ok {
		entry.Evidence = append(entry.Evidence, evStr...)
	}

	if verified, ok := op.Fields["verified"].(string); ok {
		entry.Verified = verified
	}
	if closesWhen, ok := op.Fields["closes_when"].(string); ok {
		entry.ClosesWhen = closesWhen
	}
	if sdp, ok := op.Fields["source_doc_path"].(string); ok {
		entry.SourceDocPath = sdp
	}
	if sdsha, ok := op.Fields["source_doc_blob_sha"].(string); ok {
		entry.SourceDocBlobSHA = sdsha
	}
	if sc, ok := extractIntField(op.Fields, "success_count"); ok {
		entry.SuccessCount = sc
	}
	if fc, ok := extractIntField(op.Fields, "failure_count"); ok {
		entry.FailureCount = fc
	}
	if lsa, ok := op.Fields["last_succeeded_at"].(string); ok {
		entry.LastSucceededAt = lsa
	}
	if ss, ok := op.Fields["stale_suspect"].(bool); ok {
		entry.StaleSuspect = ss
	}
	if ras, ok := op.Fields["render_as_skill"].(string); ok {
		entry.RenderAsSkill = ras
	}

	// --- Step 4: Append entry and index its scope ---
	for _, e := range d.Entries {
		if e.ID == entry.ID {
			entryIndex[op.EntryID] = op.DossierScope
			applyAmend(dossiers, entryIndex, op)
			return
		}
	}

	d.Entries = append(d.Entries, entry)
	entryIndex[op.EntryID] = op.DossierScope
}

// applyAmend updates an existing active entry and/or dossier metadata in place.
func applyAmend(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	// --- Step 1: Resolve target dossier scope and apply dossier-level metadata ---
	scope := entryIndex[op.EntryID]
	if scope == "" {
		scope = op.DossierScope
	}
	if scope == "" {
		return // Cannot resolve scope
	}

	d := getOrCreateDossier(dossiers, scope)

	ck, hasCK, dk, hasDK := extractKinds(op.Fields)
	if hasDK {
		d.Kind = dk
	}
	if title, ok := op.Fields["title"].(string); ok {
		d.Title = title
	}
	if purpose, ok := op.Fields["purpose"].(string); ok {
		d.Purpose = purpose
	}

	// --- Step 2: Locate matching active entry and apply field updates ---
	for i, e := range d.Entries {
		if e.ID == op.EntryID {
			typeAmended := false
			if t, ok := op.Fields["entry_type"].(string); ok {
				d.Entries[i].Type = EntryType(t)
				typeAmended = true
			}
			if hasCK {
				d.Entries[i].Kind = ck
			} else if typeAmended || d.Entries[i].Kind == "" {
				d.Entries[i].Kind = DeriveCognitiveKind(d.Entries[i].Type)
			}
			if text, ok := op.Fields["text"].(string); ok {
				d.Entries[i].Text = text
			}
			if anchors, ok := op.Fields["anchors"].([]any); ok {
				var aStrs []string
				for _, a := range anchors {
					if s, ok := a.(string); ok {
						aStrs = append(aStrs, s)
					}
				}
				d.Entries[i].Anchors = aStrs
			} else if anchorsStr, ok := op.Fields["anchors"].([]string); ok {
				d.Entries[i].Anchors = anchorsStr
			}
			if evidence, ok := op.Fields["evidence"].([]any); ok {
				var eStrs []string
				for _, ev := range evidence {
					if s, ok := ev.(string); ok {
						eStrs = append(eStrs, s)
					}
				}
				d.Entries[i].Evidence = eStrs
			} else if evStr, ok := op.Fields["evidence"].([]string); ok {
				d.Entries[i].Evidence = evStr
			}
			if verified, ok := op.Fields["verified"].(string); ok {
				d.Entries[i].Verified = verified
			}
			if closesWhen, ok := op.Fields["closes_when"].(string); ok {
				d.Entries[i].ClosesWhen = closesWhen
			}
			if sdp, ok := op.Fields["source_doc_path"].(string); ok {
				d.Entries[i].SourceDocPath = sdp
			}
			if sdsha, ok := op.Fields["source_doc_blob_sha"].(string); ok {
				d.Entries[i].SourceDocBlobSHA = sdsha
			}
			if sc, ok := extractIntField(op.Fields, "success_count"); ok {
				d.Entries[i].SuccessCount = sc
			}
			if fc, ok := extractIntField(op.Fields, "failure_count"); ok {
				d.Entries[i].FailureCount = fc
			}
			if lsa, ok := op.Fields["last_succeeded_at"].(string); ok {
				d.Entries[i].LastSucceededAt = lsa
			}
			if ss, ok := op.Fields["stale_suspect"].(bool); ok {
				d.Entries[i].StaleSuspect = ss
			}
			if ras, ok := op.Fields["render_as_skill"].(string); ok {
				d.Entries[i].RenderAsSkill = ras
			}
			d.Entries[i].UpdatedAt = op.Timestamp
			break
		}
	}
}

// applySupersede moves the superseded entry into History and inserts the
// replacement entry (`new_entry_id`) into active Entries, inheriting any
// unspecified fields from the prior entry.
func applySupersede(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	// --- Step 1: Resolve target dossier and retire the superseded entry into History ---
	scope := entryIndex[op.EntryID]
	if scope == "" {
		scope = op.DossierScope
	}
	if scope == "" {
		return
	}

	d := getOrCreateDossier(dossiers, scope)
	ck, hasCK, dk, hasDK := extractKinds(op.Fields)
	if hasDK {
		d.Kind = dk
	}
	var newID string
	if nid, ok := op.Fields["new_entry_id"].(string); ok {
		newID = nid
	}

	var entryToMove Entry
	found := false
	var keep []Entry
	for _, e := range d.Entries {
		if e.ID == op.EntryID {
			e.Retired = true
			e.RetiredReason = "superseded"
			e.SupersededBy = newID
			e.UpdatedAt = op.Timestamp
			entryToMove = e
			found = true
		} else {
			keep = append(keep, e)
		}
	}
	if found {
		d.Entries = keep
		d.History = append(d.History, entryToMove)
	}

	// --- Step 2: Construct replacement entry with inheritance from the superseded entry ---
	newEntry := Entry{
		ID:           newID,
		DossierScope: scope,
		Source:       op.Source,
		CreatedAt:    op.Timestamp,
		UpdatedAt:    op.Timestamp,
	}

	if text, ok := op.Fields["text"].(string); ok {
		newEntry.Text = text
	}
	typeOverridden := false
	if t, ok := op.Fields["entry_type"].(string); ok {
		newEntry.Type = EntryType(t)
		typeOverridden = true
	} else if found {
		newEntry.Type = entryToMove.Type
	}
	if hasCK {
		newEntry.Kind = ck
	} else if !typeOverridden && found && entryToMove.Kind.Valid() {
		newEntry.Kind = entryToMove.Kind
	} else {
		newEntry.Kind = DeriveCognitiveKind(newEntry.Type)
	}

	if anchors, ok := op.Fields["anchors"].([]any); ok {
		for _, a := range anchors {
			if s, ok := a.(string); ok {
				newEntry.Anchors = append(newEntry.Anchors, s)
			}
		}
	} else if anchorsStr, ok := op.Fields["anchors"].([]string); ok {
		newEntry.Anchors = append(newEntry.Anchors, anchorsStr...)
	} else if found {
		newEntry.Anchors = entryToMove.Anchors
	}

	if evidence, ok := op.Fields["evidence"].([]any); ok {
		for _, ev := range evidence {
			if s, ok := ev.(string); ok {
				newEntry.Evidence = append(newEntry.Evidence, s)
			}
		}
	} else if evStr, ok := op.Fields["evidence"].([]string); ok {
		newEntry.Evidence = append(newEntry.Evidence, evStr...)
	} else if found {
		newEntry.Evidence = entryToMove.Evidence
	}

	if verified, ok := op.Fields["verified"].(string); ok {
		newEntry.Verified = verified
	} else if found {
		newEntry.Verified = entryToMove.Verified
	}

	if closesWhen, ok := op.Fields["closes_when"].(string); ok {
		newEntry.ClosesWhen = closesWhen
	} else if found {
		newEntry.ClosesWhen = entryToMove.ClosesWhen
	}

	if sdp, ok := op.Fields["source_doc_path"].(string); ok {
		newEntry.SourceDocPath = sdp
	} else if found {
		newEntry.SourceDocPath = entryToMove.SourceDocPath
	}
	if sdsha, ok := op.Fields["source_doc_blob_sha"].(string); ok {
		newEntry.SourceDocBlobSHA = sdsha
	} else if found {
		newEntry.SourceDocBlobSHA = entryToMove.SourceDocBlobSHA
	}
	if sc, ok := extractIntField(op.Fields, "success_count"); ok {
		newEntry.SuccessCount = sc
	}
	if fc, ok := extractIntField(op.Fields, "failure_count"); ok {
		newEntry.FailureCount = fc
	}
	if lsa, ok := op.Fields["last_succeeded_at"].(string); ok {
		newEntry.LastSucceededAt = lsa
	}
	if ss, ok := op.Fields["stale_suspect"].(bool); ok {
		newEntry.StaleSuspect = ss
	}
	if ras, ok := op.Fields["render_as_skill"].(string); ok {
		newEntry.RenderAsSkill = ras
	} else if found {
		newEntry.RenderAsSkill = entryToMove.RenderAsSkill
	}

	// --- Step 3: Append replacement entry to active Entries and update entryIndex ---
	d.Entries = append(d.Entries, newEntry)
	entryIndex[newID] = scope
}

// applyRetire moves an active entry into its dossier's History slice and records
// the retirement reason.
func applyRetire(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	// --- Step 1: Resolve target dossier scope and retirement reason ---
	scope := entryIndex[op.EntryID]
	if scope == "" {
		scope = op.DossierScope
	}
	if scope == "" {
		return
	}

	d := getOrCreateDossier(dossiers, scope)
	var reason string
	if r, ok := op.Fields["reason"].(string); ok {
		reason = r
	}

	// --- Step 2: Remove entry from active Entries and append to History ---
	var entryToMove Entry
	found := false
	var keep []Entry
	for _, e := range d.Entries {
		if e.ID == op.EntryID {
			e.Retired = true
			e.RetiredReason = reason
			e.UpdatedAt = op.Timestamp
			entryToMove = e
			found = true
		} else {
			keep = append(keep, e)
		}
	}
	if found {
		d.Entries = keep
		d.History = append(d.History, entryToMove)
	}
}

// moveEntry transfers entryID from fromScope to toScope and prunes fromScope if
// it becomes completely empty.
func moveEntry(dossiers map[string]*Dossier, entryIndex map[string]string, entryID, fromScope, toScope string, ts time.Time) {
	// --- Step 1: Validate source and destination scopes ---
	if fromScope == "" || toScope == "" || fromScope == toScope {
		return
	}

	fromD := getOrCreateDossier(dossiers, fromScope)
	toD := getOrCreateDossier(dossiers, toScope)

	// --- Step 2: Detach entry from source dossier ---
	var entryToMove Entry
	found := false
	var keep []Entry
	for _, e := range fromD.Entries {
		if e.ID == entryID {
			e.DossierScope = toScope
			e.UpdatedAt = ts
			entryToMove = e
			found = true
		} else {
			keep = append(keep, e)
		}
	}

	// --- Step 3: Prune empty source dossier and attach entry to destination dossier ---
	if found {
		fromD.Entries = keep
		if len(fromD.Entries) == 0 && len(fromD.History) == 0 && fromD.Title == "" && fromD.Purpose == "" {
			delete(dossiers, fromScope)
		}
		var destKeep []Entry
		for _, e := range toD.Entries {
			if e.ID != entryID {
				destKeep = append(destKeep, e)
			}
		}
		toD.Entries = append(destKeep, entryToMove)
		entryIndex[entryID] = toScope
	}
}

// applyMove handles an OpMove operation by transferring op.EntryID to `to_dossier`.
func applyMove(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	toScope, _ := op.Fields["to_dossier"].(string)
	fromScope := entryIndex[op.EntryID]
	if fromScope == "" {
		fromScope = op.DossierScope
	}
	moveEntry(dossiers, entryIndex, op.EntryID, fromScope, toScope, op.Timestamp)
}

// applySplit handles an OpSplit operation by creating target dossiers and
// reassigning entries specified in `entry_moves`.
func applySplit(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	// --- Step 1: Resolve source dossier scope from op or entryMoves ---
	fromScope := op.DossierScope
	if fromScope == "" || dossiers[fromScope] == nil {
		if entryMoves, ok := op.Fields["entry_moves"].(map[string]any); ok {
			for id := range entryMoves {
				fromScope = entryIndex[id]
				break
			}
		} else if entryMovesStr, ok := op.Fields["entry_moves"].(map[string]string); ok {
			for id := range entryMovesStr {
				fromScope = entryIndex[id]
				break
			}
		}
	}
	if fromScope == "" {
		return
	}

	fromD := getOrCreateDossier(dossiers, fromScope)

	// --- Step 2: Ensure all declared replacement dossiers exist ---
	if newDossiers, ok := op.Fields["new_dossiers"].([]any); ok {
		for _, nd := range newDossiers {
			if scope, ok := nd.(string); ok {
				getOrCreateDossier(dossiers, scope)
			}
		}
	} else if newDossiersStr, ok := op.Fields["new_dossiers"].([]string); ok {
		for _, scope := range newDossiersStr {
			getOrCreateDossier(dossiers, scope)
		}
	}

	// --- Step 3: Extract entry-to-destination-scope map and redistribute entries ---
	moves := make(map[string]string)
	if entryMoves, ok := op.Fields["entry_moves"].(map[string]any); ok {
		for id, dest := range entryMoves {
			if destStr, ok := dest.(string); ok {
				moves[id] = destStr
			}
		}
	} else if entryMovesStr, ok := op.Fields["entry_moves"].(map[string]string); ok {
		moves = entryMovesStr
	}

	var keep []Entry
	for _, e := range fromD.Entries {
		if destScope, ok := moves[e.ID]; ok {
			toD := getOrCreateDossier(dossiers, destScope)
			e.DossierScope = destScope
			e.UpdatedAt = op.Timestamp
			toD.Entries = append(toD.Entries, e)
			entryIndex[e.ID] = destScope
		} else {
			keep = append(keep, e)
		}
	}
	fromD.Entries = keep
}

// typeOrder defines the canonical section ordering for entries within a dossier.
var typeOrder = map[EntryType]int{
	EntryTypeInvariant:         1,
	EntryTypeContract:          1,
	EntryTypeConvention:        1,
	EntryTypeRuledOut:          2,
	EntryTypeFailureSignature:  3,
	EntryTypeGotcha:            4,
	EntryTypeExternalBehaviour: 4,
	EntryTypeDecision:          5,
	EntryTypeProcedure:         6,
	EntryTypeRecipe:            6,
	EntryTypeInterface:         7,
	EntryTypeOpenIssue:         8,
}

// sortDossier sorts both active and historical entries of d in place.
func sortDossier(d *Dossier) {
	sortEntries(d.Entries)
	sortEntries(d.History)
}

// sortEntries sorts entries by canonical section priority (typeOrder) and then
// lexicographically by entry ID.
func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		oi := typeOrder[entries[i].Type]
		oj := typeOrder[entries[j].Type]
		if oi == 0 {
			oi = 99
		}
		if oj == 0 {
			oj = 99
		}
		if oi != oj {
			return oi < oj
		}
		return strings.Compare(entries[i].ID, entries[j].ID) < 0
	})
}
