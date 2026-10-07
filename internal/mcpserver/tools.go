package mcpserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// MemoryBriefRequest represents the input to the memory.brief tool.
type MemoryBriefRequest struct {
	Scope string `json:"scope"`
}

// MemoryBriefResponse represents the output of the memory.brief tool.
type MemoryBriefResponse struct {
	Result string `json:"result"`
}

// MemoryWhyRequest represents the input to the memory.why tool.
type MemoryWhyRequest struct {
	EntryID string `json:"entryId"`
}

// MemoryWhyResponse represents the output of the memory.why tool.
type MemoryWhyResponse struct {
	Result string `json:"result"`
}

// MemoryHistoryRequest represents the input to the memory.history tool.
type MemoryHistoryRequest struct {
	Scope string `json:"scope"`
}

// MemoryHistoryResponse represents the output of the memory.history tool.
type MemoryHistoryResponse struct {
	Result string `json:"result"`
}

// MemorySearchRequest represents the input to the memory.search tool.
type MemorySearchRequest struct {
	Query string `json:"query"`
}

// MemorySearchResponse represents the output of the memory.search tool.
type MemorySearchResponse struct {
	Result string `json:"result"`
}

// MemoryNoteRequest represents the input to the memory.note tool.
type MemoryNoteRequest struct {
	Scope string `json:"scope"`
	Text  string `json:"text"`
	Type  string `json:"type,omitempty"`
}

// MemoryNoteResponse represents the output of the memory.note tool.
type MemoryNoteResponse struct {
	Result string `json:"result"`
}

var validEntryTypes = map[dossiers.EntryType]bool{
	dossiers.EntryTypeInvariant:        true,
	dossiers.EntryTypeRuledOut:         true,
	dossiers.EntryTypeFailureSignature: true,
	dossiers.EntryTypeGotcha:           true,
	dossiers.EntryTypeDecision:         true,
	dossiers.EntryTypeProcedure:        true,
	dossiers.EntryTypeInterface:        true,
	dossiers.EntryTypeOpenIssue:        true,
}

// HandleMemoryBrief implements the memory.brief tool handler.
func (s *Server) HandleMemoryBrief(ctx context.Context, req MemoryBriefRequest) (MemoryBriefResponse, error) {
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		return MemoryBriefResponse{}, errors.New("scope is required")
	}

	entries, err := s.idx.ListEntriesByScope(ctx, scope)
	if err != nil {
		return MemoryBriefResponse{}, fmt.Errorf("listing entries for scope %s: %w", scope, err)
	}

	if len(entries) == 0 {
		resolved, err := s.idx.ResolveAlias(ctx, scope)
		if err == nil && resolved != "" && resolved != scope {
			scope = resolved
			entries, err = s.idx.ListEntriesByScope(ctx, scope)
			if err != nil {
				return MemoryBriefResponse{}, fmt.Errorf("listing entries for resolved scope %s: %w", scope, err)
			}
		}
	}

	var active []dossiers.Entry
	for _, e := range entries {
		if e.Retired {
			continue
		}
		if hists, err := s.idx.GetHistory(ctx, e.ID); err == nil && len(hists) > 0 {
			continue
		}
		active = append(active, e)
	}

	if s.ledger != nil {
		for _, e := range active {
			s.ledger.RecordPull(e.ID)
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Scope: %s (%d entries)\n", scope, len(active)))
	for _, e := range active {
		text := e.Text
		runes := []rune(text)
		if len(runes) > 200 {
			text = string(runes[:200])
		}
		b.WriteString(fmt.Sprintf("[%s] %s: %s\n", e.Type, e.ID, text))
	}

	return MemoryBriefResponse{Result: strings.TrimRight(b.String(), "\n")}, nil
}

// HandleMemoryWhy implements the memory.why tool handler.
func (s *Server) HandleMemoryWhy(ctx context.Context, req MemoryWhyRequest) (MemoryWhyResponse, error) {
	entryID := strings.TrimSpace(req.EntryID)
	if entryID == "" {
		return MemoryWhyResponse{}, errors.New("entryId is required")
	}

	entry, err := s.idx.GetEntry(ctx, entryID)
	if err != nil {
		return MemoryWhyResponse{}, fmt.Errorf("getting entry %s: %w", entryID, err)
	}

	history, err := s.idx.GetHistory(ctx, entryID)
	if err != nil {
		history = nil
	}

	if s.ledger != nil {
		s.ledger.RecordPull(entry.ID)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Entry: %s\n", entry.ID))
	b.WriteString(fmt.Sprintf("Type: %s\n", entry.Type))
	b.WriteString(fmt.Sprintf("Scope: %s\n", entry.DossierScope))
	b.WriteString(fmt.Sprintf("Text: %s\n", entry.Text))

	if len(entry.Anchors) > 0 {
		b.WriteString(fmt.Sprintf("Anchors: %s\n", strings.Join(entry.Anchors, ", ")))
	} else {
		b.WriteString("Anchors: none\n")
	}

	evidenceStr := "none"
	if len(entry.Evidence) > 0 {
		evidenceStr = strings.Join(entry.Evidence, ", ")
	} else if entry.Source != "" {
		evidenceStr = entry.Source
	}
	b.WriteString(fmt.Sprintf("Evidence: %s\n", evidenceStr))

	if entry.Verified != "" {
		b.WriteString(fmt.Sprintf("Verification Status: %s\n", entry.Verified))
	} else {
		b.WriteString("Verification Status: unverified\n")
	}

	if !entry.CreatedAt.IsZero() {
		b.WriteString(fmt.Sprintf("Created: %s\n", entry.CreatedAt.Format(time.RFC3339)))
	}
	if !entry.UpdatedAt.IsZero() {
		b.WriteString(fmt.Sprintf("Updated: %s\n", entry.UpdatedAt.Format(time.RFC3339)))
	}

	if entry.Retired {
		b.WriteString(fmt.Sprintf("Retired: true (Reason: %s)\n", entry.RetiredReason))
	}
	if entry.SupersededBy != "" {
		b.WriteString(fmt.Sprintf("Superseded By: %s\n", entry.SupersededBy))
	}

	if len(history) > 0 {
		b.WriteString("\nHistory:\n")
		for _, h := range history {
			var details []string
			if h.RetiredReason != "" {
				details = append(details, fmt.Sprintf("retired reason: %s", h.RetiredReason))
			}
			if h.SupersededBy != "" {
				details = append(details, fmt.Sprintf("superseded by: %s", h.SupersededBy))
			}
			detailStr := ""
			if len(details) > 0 {
				detailStr = fmt.Sprintf(" (%s)", strings.Join(details, ", "))
			}
			b.WriteString(fmt.Sprintf("- [%s] %s: %s%s\n", h.Type, h.ID, h.Text, detailStr))
		}
	}

	return MemoryWhyResponse{Result: strings.TrimRight(b.String(), "\n")}, nil
}

// HandleMemoryHistory implements the memory.history tool handler.
func (s *Server) HandleMemoryHistory(ctx context.Context, req MemoryHistoryRequest) (MemoryHistoryResponse, error) {
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		return MemoryHistoryResponse{}, errors.New("scope is required")
	}

	retiredEntries, resolvedScope, err := s.findRetiredEntries(ctx, scope)
	if err != nil {
		return MemoryHistoryResponse{}, err
	}

	if s.ledger != nil {
		for _, e := range retiredEntries {
			s.ledger.RecordPull(e.ID)
		}
	}

	if len(retiredEntries) == 0 {
		return MemoryHistoryResponse{Result: fmt.Sprintf("No retired or superseded entries found for scope: %s", resolvedScope)}, nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("History for scope %s (%d retired/superseded entries):\n", resolvedScope, len(retiredEntries)))
	for _, e := range retiredEntries {
		var details []string
		if e.RetiredReason != "" {
			details = append(details, fmt.Sprintf("retired reason: %s", e.RetiredReason))
		}
		if e.SupersededBy != "" {
			details = append(details, fmt.Sprintf("superseded by: %s", e.SupersededBy))
		}
		detailStr := ""
		if len(details) > 0 {
			detailStr = fmt.Sprintf(" (%s)", strings.Join(details, ", "))
		}
		b.WriteString(fmt.Sprintf("[%s] %s: %s%s\n", e.Type, e.ID, e.Text, detailStr))
	}

	return MemoryHistoryResponse{Result: strings.TrimRight(b.String(), "\n")}, nil
}

func (s *Server) findRetiredEntries(ctx context.Context, scope string) ([]dossiers.Entry, string, error) {
	retired, err := s.collectRetiredForScope(ctx, scope)
	if err != nil {
		return nil, scope, err
	}
	if len(retired) > 0 {
		return retired, scope, nil
	}

	resolved, err := s.idx.ResolveAlias(ctx, scope)
	if err == nil && resolved != "" && resolved != scope {
		resolvedRetired, err := s.collectRetiredForScope(ctx, resolved)
		if err != nil {
			return nil, resolved, err
		}
		return resolvedRetired, resolved, nil
	}

	return retired, scope, nil
}

func (s *Server) collectRetiredForScope(ctx context.Context, scope string) ([]dossiers.Entry, error) {
	seen := make(map[string]bool)
	var result []dossiers.Entry

	entries, err := s.idx.ListEntriesByScope(ctx, scope)
	if err == nil {
		for _, e := range entries {
			if e.Retired || e.RetiredReason != "" || e.SupersededBy != "" {
				if !seen[e.ID] {
					seen[e.ID] = true
					result = append(result, e)
				}
			}
		}
	}

	// Use a plain read query — no write transaction needed for a SELECT.
	rows, err := s.idx.QueryHistoryByScope(ctx, scope)
	if err != nil {
		return result, fmt.Errorf("querying entry_history for scope %s: %w", scope, err)
	}
	defer rows.Close()

	for rows.Next() {
		var eID, typeStr, dScope, text, reason, supersededBy, retiredAt sql.NullString
		if err := rows.Scan(&eID, &typeStr, &dScope, &text, &reason, &supersededBy, &retiredAt); err != nil {
			return result, fmt.Errorf("scanning entry_history row: %w", err)
		}
		key := eID.String + ":" + reason.String + ":" + supersededBy.String
		if !seen[key] && !seen[eID.String] {
			seen[key] = true
			result = append(result, dossiers.Entry{
				ID:            eID.String,
				Type:          dossiers.EntryType(typeStr.String),
				DossierScope:  dScope.String,
				Text:          text.String,
				Retired:       true,
				RetiredReason: reason.String,
				SupersededBy:  supersededBy.String,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	return result, nil
}

// HandleMemorySearch implements the memory.search tool handler.
func (s *Server) HandleMemorySearch(ctx context.Context, req MemorySearchRequest) (MemorySearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return MemorySearchResponse{Result: "No results found."}, nil
	}

	results, err := s.idx.SearchEntries(ctx, query)
	if err != nil {
		return MemorySearchResponse{}, fmt.Errorf("searching entries with %q: %w", query, err)
	}

	if len(results) == 0 {
		return MemorySearchResponse{Result: "No results found."}, nil
	}

	if len(results) > 10 {
		results = results[:10]
	}

	if s.ledger != nil {
		for _, e := range results {
			s.ledger.RecordPull(e.ID)
		}
	}

	var b strings.Builder
	for i, e := range results {
		text := e.Text
		runes := []rune(text)
		if len(runes) > 100 {
			text = string(runes[:100])
		}
		b.WriteString(fmt.Sprintf("%d. [%s] %s (%s): %s\n", i+1, e.Type, e.ID, e.DossierScope, text))
	}

	return MemorySearchResponse{Result: strings.TrimRight(b.String(), "\n")}, nil
}

// HandleMemoryNote implements the memory.note tool handler.
func (s *Server) HandleMemoryNote(ctx context.Context, req MemoryNoteRequest) (MemoryNoteResponse, error) {
	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		return MemoryNoteResponse{}, errors.New("scope is required")
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		return MemoryNoteResponse{}, errors.New("text is required")
	}

	entryTypeStr := strings.TrimSpace(req.Type)
	if entryTypeStr == "" {
		entryTypeStr = string(dossiers.EntryTypeDecision)
	}

	entryType := dossiers.EntryType(entryTypeStr)
	if !validEntryTypes[entryType] {
		return MemoryNoteResponse{}, fmt.Errorf("invalid entry type %q", entryTypeStr)
	}

	prefix := dossiers.EntryTypePrefixes[entryType]
	if prefix == "" {
		prefix = "dec-"
	}
	entryID := ops.NewEntryID(prefix)
	now := time.Now().UTC()

	op := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      entryID,
		DossierScope: scope,
		Timestamp:    now,
		Source:       "agent",
		Fields: map[string]any{
			"entry_type": string(entryType),
			"type":       string(entryType),
			"text":       text,
		},
	}

	batch := ops.NewBatch([]ops.Op{op})

	writer := s.opsWriter
	if writer == nil {
		writer = ops.NewWriter(filepath.Join(".memory", "ops"))
	}
	if err := writer.Write(batch); err != nil {
		return MemoryNoteResponse{}, fmt.Errorf("writing op batch: %w", err)
	}

	if s.idx != nil {
		_ = s.idx.UpsertEntry(ctx, dossiers.Entry{
			ID:           entryID,
			Type:         entryType,
			DossierScope: scope,
			Text:         text,
			Source:       "agent",
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	return MemoryNoteResponse{
		Result: fmt.Sprintf("Note recorded: %s", entryID),
	}, nil
}
