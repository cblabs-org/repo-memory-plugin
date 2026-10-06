package dossiers

import (
	"context"
	"iter"
	"sort"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

type Projector struct {
	reader *ops.Reader
}

func NewProjector(opsReader *ops.Reader) *Projector {
	return &Projector{
		reader: opsReader,
	}
}

func (p *Projector) projectFromSeq(seq iter.Seq2[*ops.OpBatch, error]) (map[string]*Dossier, error) {
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

	for _, d := range dossiers {
		sortDossier(d)
	}

	return dossiers, nil
}

func (p *Projector) Project(ctx context.Context) (map[string]*Dossier, error) {
	return p.projectFromSeq(p.reader.Iterate(ctx))
}

func (p *Projector) ProjectBefore(ctx context.Context, beforeCommit string) (map[string]*Dossier, error) {
	return p.projectFromSeq(p.reader.IterateBefore(ctx, beforeCommit))
}

func applyBatch(dossiers map[string]*Dossier, entryIndex map[string]string, batch *ops.OpBatch) error {
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

func applyAdd(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	// If entry already exists, treat as amend (or move if scope differs)
	if existingScope, exists := entryIndex[op.EntryID]; exists {
		if existingScope != op.DossierScope && op.DossierScope != "" {
			// Scope changed — treat as move + amend
			applyMove(dossiers, entryIndex, op)
		}
		applyAmend(dossiers, entryIndex, op)
		return
	}

	d := getOrCreateDossier(dossiers, op.DossierScope)

	if kind, ok := op.Fields["kind"].(string); ok {
		d.Kind = DossierKind(kind)
	}
	if title, ok := op.Fields["title"].(string); ok {
		d.Title = title
	}
	if purpose, ok := op.Fields["purpose"].(string); ok {
		d.Purpose = purpose
	}

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

	d.Entries = append(d.Entries, entry)
	entryIndex[op.EntryID] = op.DossierScope
}

func applyAmend(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	scope := entryIndex[op.EntryID]
	if scope == "" {
		scope = op.DossierScope
	}
	if scope == "" {
		return // Cannot resolve scope
	}
	
	d := getOrCreateDossier(dossiers, scope)

	if kind, ok := op.Fields["kind"].(string); ok {
		d.Kind = DossierKind(kind)
	}
	if title, ok := op.Fields["title"].(string); ok {
		d.Title = title
	}
	if purpose, ok := op.Fields["purpose"].(string); ok {
		d.Purpose = purpose
	}

	for i, e := range d.Entries {
		if e.ID == op.EntryID {
			if t, ok := op.Fields["entry_type"].(string); ok {
				d.Entries[i].Type = EntryType(t)
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
			d.Entries[i].UpdatedAt = op.Timestamp
			break
		}
	}
}

func applySupersede(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	scope := entryIndex[op.EntryID]
	if scope == "" {
		scope = op.DossierScope
	}
	if scope == "" {
		return
	}
	
	d := getOrCreateDossier(dossiers, scope)
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
	if t, ok := op.Fields["entry_type"].(string); ok {
		newEntry.Type = EntryType(t)
	} else if found {
		newEntry.Type = entryToMove.Type
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

	d.Entries = append(d.Entries, newEntry)
	entryIndex[newID] = scope
}

func applyRetire(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
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

func applyMove(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	toScope, _ := op.Fields["to_dossier"].(string)
	if toScope == "" {
		return
	}
	
	fromScope := entryIndex[op.EntryID]
	if fromScope == "" {
		fromScope = op.DossierScope
	}
	if fromScope == "" {
		return
	}
	
	fromD := getOrCreateDossier(dossiers, fromScope)
	toD := getOrCreateDossier(dossiers, toScope)

	var entryToMove Entry
	found := false
	var keep []Entry
	for _, e := range fromD.Entries {
		if e.ID == op.EntryID {
			e.DossierScope = toScope
			e.UpdatedAt = op.Timestamp
			entryToMove = e
			found = true
		} else {
			keep = append(keep, e)
		}
	}
	if found {
		fromD.Entries = keep
		toD.Entries = append(toD.Entries, entryToMove)
		entryIndex[op.EntryID] = toScope
	}
}

func applySplit(dossiers map[string]*Dossier, entryIndex map[string]string, op ops.Op) {
	fromScope := op.DossierScope
	if fromScope == "" || dossiers[fromScope] == nil {
		// Just take the scope of one of the entries to move? Split operates on DossierScope primarily in the previous impl. 
		// If DossierScope is empty, we don't know which dossier is being split easily, unless we check entryMoves.
		// Let's see if we can resolve it from entryMoves.
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

var typeOrder = map[EntryType]int{
	EntryTypeInvariant:        1,
	EntryTypeRuledOut:         2,
	EntryTypeFailureSignature: 3,
	EntryTypeGotcha:           4,
	EntryTypeDecision:         5,
	EntryTypeProcedure:        6,
	EntryTypeInterface:        7,
	EntryTypeOpenIssue:        8,
}

func sortDossier(d *Dossier) {
	sortEntries(d.Entries)
	sortEntries(d.History)
}

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
