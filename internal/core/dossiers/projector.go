package dossiers

import (
	"context"
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

func (p *Projector) Project(ctx context.Context) (map[string]*Dossier, error) {
	dossiers := make(map[string]*Dossier)

	for batch, err := range p.reader.Iterate(ctx) {
		if err != nil {
			return nil, err
		}
		if err := applyBatch(dossiers, batch); err != nil {
			return nil, err
		}
	}

	for _, d := range dossiers {
		sortDossier(d)
	}

	return dossiers, nil
}

func (p *Projector) ProjectBefore(ctx context.Context, beforeCommit string) (map[string]*Dossier, error) {
	dossiers := make(map[string]*Dossier)

	for batch, err := range p.reader.IterateBefore(ctx, beforeCommit) {
		if err != nil {
			return nil, err
		}
		if err := applyBatch(dossiers, batch); err != nil {
			return nil, err
		}
	}

	for _, d := range dossiers {
		sortDossier(d)
	}

	return dossiers, nil
}

func applyBatch(dossiers map[string]*Dossier, batch *ops.OpBatch) error {
	for _, op := range batch.Ops {
		switch op.Type {
		case ops.OpAdd:
			applyAdd(dossiers, op)
		case ops.OpAmend:
			applyAmend(dossiers, op)
		case ops.OpSupersede:
			applySupersede(dossiers, op)
		case ops.OpRetire:
			applyRetire(dossiers, op)
		case ops.OpMove:
			applyMove(dossiers, op)
		case ops.OpSplit:
			applySplit(dossiers, op)
		}
	}
	return nil
}

func getOrCreateDossier(dossiers map[string]*Dossier, scope string) *Dossier {
	d, ok := dossiers[scope]
	if !ok {
		d = &Dossier{
			Scope: scope,
			// Default kind, can be overridden by ops
			Kind: DossierKindModule,
		}
		dossiers[scope] = d
	}
	return d
}

func applyAdd(dossiers map[string]*Dossier, op ops.Op) {
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
}

func applyAmend(dossiers map[string]*Dossier, op ops.Op) {
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

func applySupersede(dossiers map[string]*Dossier, op ops.Op) {
	d := getOrCreateDossier(dossiers, op.DossierScope)
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
		DossierScope: op.DossierScope,
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
	d.Entries = append(d.Entries, newEntry)
}

func applyRetire(dossiers map[string]*Dossier, op ops.Op) {
	d := getOrCreateDossier(dossiers, op.DossierScope)
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

func applyMove(dossiers map[string]*Dossier, op ops.Op) {
	toScope, _ := op.Fields["to_dossier"].(string)
	if toScope == "" {
		return
	}
	
	fromD := getOrCreateDossier(dossiers, op.DossierScope)
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
	}
}

func applySplit(dossiers map[string]*Dossier, op ops.Op) {
	fromD := getOrCreateDossier(dossiers, op.DossierScope)

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
