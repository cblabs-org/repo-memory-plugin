package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/coverage"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
)

// FormatFreshnessLine computes or reads the live A6 freshness state and formats the exact 1-line header
// from Addendum A §7.2:
//
//	memory: <index status> · <HEAD status> · <N> notes not yet consolidated · <M> entries suspect
func FormatFreshnessLine(ctx context.Context, dataDir, repoID, repoRoot string, idx *index.Index, inboxStore *inbox.Store) string {
	return tick.FormatFreshnessLine(ctx, dataDir, repoID, repoRoot, idx, inboxStore)
}

func (s *Server) resolveInboxStore() *inbox.Store {
	if s.inboxStore != nil {
		return s.inboxStore
	}
	if s.dataDir != "" && s.repoID != "" {
		return inbox.NewStore(s.dataDir, s.repoID)
	}
	return nil
}

func (s *Server) gapThreshold() float64 {
	if s.repoRoot != "" {
		if cfg, err := config.LoadRepoConfig(s.repoRoot); err == nil && cfg.Gaps.Threshold > 0 {
			return cfg.Gaps.Threshold
		}
	}
	return coverage.DefaultGapThreshold
}

type matchedNoteItem struct {
	note *inbox.Note
	item inbox.NoteItem
}

func noteItemMatchesScope(item inbox.NoteItem, scope string) bool {
	target := strings.ToLower(strings.TrimSpace(scope))
	if target == "" {
		return false
	}
	if strings.ToLower(strings.TrimSpace(item.Scope)) == target {
		return true
	}
	for _, a := range item.Anchors {
		clean := strings.ToLower(strings.TrimSpace(a))
		if idx := strings.Index(clean, "#"); idx >= 0 {
			clean = clean[:idx]
		}
		if idx := strings.Index(clean, ":l"); idx >= 0 {
			clean = clean[:idx]
		}
		clean = strings.Trim(filepath.ToSlash(clean), "/")
		if clean == target ||
			strings.HasPrefix(clean, target+"/") ||
			strings.HasSuffix(clean, "/"+target) ||
			strings.Contains(clean, "/"+target+"/") ||
			strings.ToLower(filepath.Base(filepath.Dir(clean))) == target {
			return true
		}
	}
	return false
}

func formatNoteItemWhyDo(item inbox.NoteItem) string {
	why := strings.TrimSpace(item.Why)
	do := strings.TrimSpace(item.Do)
	if why != "" && do != "" {
		return why + " / " + do
	}
	if why != "" {
		return why
	}
	if do != "" {
		return do
	}
	return strings.TrimSpace(item.Evidence)
}

func formatNoteBranch(note *inbox.Note) string {
	if note != nil && strings.TrimSpace(note.Branch) != "" {
		return strings.TrimSpace(note.Branch)
	}
	return "main"
}

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
	Scope          string   `json:"scope"`
	Text           string   `json:"text"`
	Type           string   `json:"type,omitempty"`
	Title          string   `json:"title,omitempty"`
	Why            string   `json:"why,omitempty"`
	Do             string   `json:"do,omitempty"`
	Anchors        []string `json:"anchors,omitempty"`
	ConversationID string   `json:"conversationId,omitempty"`
	StepIndex      int      `json:"stepIndex,omitempty"`
}

// MemoryNoteResponse represents the output of the memory.note tool.
type MemoryNoteResponse struct {
	Result string `json:"result"`
}

// MemoryStatusRequest represents the input to the memory_status tool.
type MemoryStatusRequest struct{}

// MemoryStatusResponse represents the output of the memory_status tool.
type MemoryStatusResponse struct {
	Result string `json:"result"`
}

var validEntryTypes = map[dossiers.EntryType]bool{
	dossiers.EntryTypeInvariant:         true,
	dossiers.EntryTypeContract:          true,
	dossiers.EntryTypeConvention:        true,
	dossiers.EntryTypeRuledOut:          true,
	dossiers.EntryTypeFailureSignature:  true,
	dossiers.EntryTypeGotcha:            true,
	dossiers.EntryTypeExternalBehaviour: true,
	dossiers.EntryTypeDecision:          true,
	dossiers.EntryTypeProcedure:         true,
	dossiers.EntryTypeRecipe:            true,
	dossiers.EntryTypeInterface:         true,
	dossiers.EntryTypeOpenIssue:         true,
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

	inboxStore := s.resolveInboxStore()
	var matchedNotes []matchedNoteItem
	if inboxStore != nil {
		if unconsolidated, uErr := inboxStore.ListUnconsolidated(); uErr == nil {
			for _, n := range unconsolidated {
				if n == nil {
					continue
				}
				for _, it := range n.Items {
					if noteItemMatchesScope(it, scope) {
						matchedNotes = append(matchedNotes, matchedNoteItem{note: n, item: it})
					}
				}
			}
		}
	}

	var b strings.Builder
	b.WriteString(FormatFreshnessLine(ctx, s.dataDir, s.repoID, s.repoRoot, s.idx, inboxStore))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Scope: %s (%d entries)\n", scope, len(active)))
	for _, e := range active {
		text := e.Text
		runes := []rune(text)
		if len(runes) > 200 {
			text = string(runes[:200])
		}
		b.WriteString(fmt.Sprintf("[%s] %s: %s\n", e.Type, e.ID, text))
	}

	if len(matchedNotes) > 0 {
		b.WriteString("Unconsolidated Inbox Notes (unconsolidated):\n")
		for _, m := range matchedNotes {
			b.WriteString(fmt.Sprintf("[unconsolidated %s] (%s) %s: %s — %s\n",
				m.note.ID,
				formatNoteBranch(m.note),
				m.item.Type,
				strings.TrimSpace(m.item.Title),
				formatNoteItemWhyDo(m.item),
			))
		}
	}

	gapLines := coverage.FormatGapLinesForScope(scope, active, len(matchedNotes), s.gapThreshold())
	for _, gl := range gapLines {
		b.WriteString(gl + "\n")
	}

	return MemoryBriefResponse{Result: strings.TrimRight(b.String(), "\n")}, nil
}

// HandleMemoryWhy implements the memory.why tool handler.
func (s *Server) HandleMemoryWhy(ctx context.Context, req MemoryWhyRequest) (MemoryWhyResponse, error) {
	entryID := strings.TrimSpace(req.EntryID)
	if entryID == "" {
		return MemoryWhyResponse{}, errors.New("entryId is required")
	}

	inboxStore := s.resolveInboxStore()
	freshnessLine := FormatFreshnessLine(ctx, s.dataDir, s.repoID, s.repoRoot, s.idx, inboxStore)

	entry, err := s.idx.GetEntry(ctx, entryID)
	if err != nil {
		if errors.Is(err, index.ErrNotFound) && inboxStore != nil {
			if noteResp, ok := findUnconsolidatedNoteWhy(inboxStore, entryID, freshnessLine); ok {
				return MemoryWhyResponse{Result: noteResp}, nil
			}
		}
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
	b.WriteString(freshnessLine + "\n")
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

func findUnconsolidatedNoteWhy(inboxStore *inbox.Store, target string, freshnessLine string) (string, bool) {
	unconsolidated, err := inboxStore.ListUnconsolidated()
	if err != nil || len(unconsolidated) == 0 {
		return "", false
	}
	targetLower := strings.ToLower(strings.TrimSpace(target))

	for _, n := range unconsolidated {
		if n == nil {
			continue
		}
		idMatch := strings.EqualFold(strings.TrimSpace(n.ID), targetLower)
		for _, it := range n.Items {
			itemMatch := idMatch || strings.Contains(strings.ToLower(it.Title), targetLower)
			if !itemMatch {
				for _, a := range it.Anchors {
					if strings.EqualFold(strings.TrimSpace(a), targetLower) || strings.Contains(strings.ToLower(a), targetLower) {
						itemMatch = true
						break
					}
				}
			}
			if itemMatch {
				var b strings.Builder
				b.WriteString(freshnessLine + "\n")
				b.WriteString(fmt.Sprintf("Note: %s\n", n.ID))
				b.WriteString("Status: unconsolidated\n")
				b.WriteString(fmt.Sprintf("Source: %s\n", n.Source))
				b.WriteString(fmt.Sprintf("Branch: %s\n", formatNoteBranch(n)))
				b.WriteString(fmt.Sprintf("Type: %s\n", it.Type))
				scope := strings.TrimSpace(it.Scope)
				if scope == "" {
					scope = "general"
				}
				b.WriteString(fmt.Sprintf("Scope: %s\n", scope))
				b.WriteString(fmt.Sprintf("Title: %s\n", strings.TrimSpace(it.Title)))
				if strings.TrimSpace(it.Why) != "" {
					b.WriteString(fmt.Sprintf("Why: %s\n", strings.TrimSpace(it.Why)))
				}
				if strings.TrimSpace(it.Do) != "" {
					b.WriteString(fmt.Sprintf("Do: %s\n", strings.TrimSpace(it.Do)))
				}
				if len(it.Anchors) > 0 {
					b.WriteString(fmt.Sprintf("Anchors: %s\n", strings.Join(it.Anchors, ", ")))
				} else {
					b.WriteString("Anchors: none\n")
				}
				if strings.TrimSpace(it.Evidence) != "" {
					b.WriteString(fmt.Sprintf("Evidence: %s\n", strings.TrimSpace(it.Evidence)))
				}
				if !n.CreatedAt.IsZero() {
					b.WriteString(fmt.Sprintf("Created: %s\n", n.CreatedAt.Format(time.RFC3339)))
				}
				return strings.TrimRight(b.String(), "\n"), true
			}
		}
	}
	return "", false
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

	// Use a plain read query — no write transaction needed.
	histEntries, err := s.idx.QueryHistoryByScope(ctx, scope)
	if err != nil {
		return result, err
	}
	for _, h := range histEntries {
		key := h.ID + ":" + h.RetiredReason + ":" + h.SupersededBy
		if !seen[key] && !seen[h.ID] {
			seen[key] = true
			result = append(result, h)
		}
	}

	return result, nil
}

// HandleMemorySearch implements the memory.search tool handler.
func (s *Server) HandleMemorySearch(ctx context.Context, req MemorySearchRequest) (MemorySearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return MemorySearchResponse{Result: "No results found."}, nil
	}

	inboxStore := s.resolveInboxStore()
	matchedNotes := searchUnconsolidatedNotes(inboxStore, query)

	results, err := s.idx.SearchEntries(ctx, query)
	if err != nil && len(matchedNotes) == 0 {
		return MemorySearchResponse{}, fmt.Errorf("searching entries with %q: %w", query, err)
	}

	if len(results) == 0 && len(matchedNotes) == 0 {
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
	b.WriteString(FormatFreshnessLine(ctx, s.dataDir, s.repoID, s.repoRoot, s.idx, inboxStore))
	b.WriteString("\n")
	for i, e := range results {
		text := e.Text
		runes := []rune(text)
		if len(runes) > 100 {
			text = string(runes[:100])
		}
		b.WriteString(fmt.Sprintf("%d. [%s] %s (%s): %s\n", i+1, e.Type, e.ID, e.DossierScope, text))
	}
	for _, m := range matchedNotes {
		scope := strings.TrimSpace(m.item.Scope)
		if scope == "" {
			scope = "general"
		}
		b.WriteString(fmt.Sprintf("[unconsolidated] %s (%s, branch: %s): %s — %s\n",
			m.note.ID,
			scope,
			formatNoteBranch(m.note),
			strings.TrimSpace(m.item.Title),
			formatNoteItemWhyDo(m.item),
		))
	}

	return MemorySearchResponse{Result: strings.TrimRight(b.String(), "\n")}, nil
}

func searchUnconsolidatedNotes(inboxStore *inbox.Store, query string) []matchedNoteItem {
	if inboxStore == nil {
		return nil
	}
	unconsolidated, err := inboxStore.ListUnconsolidated()
	if err != nil || len(unconsolidated) == 0 {
		return nil
	}
	qLower := strings.ToLower(strings.TrimSpace(query))
	if qLower == "" {
		return nil
	}
	tokens := strings.Fields(qLower)

	var out []matchedNoteItem
	for _, n := range unconsolidated {
		if n == nil {
			continue
		}
		for _, it := range n.Items {
			haystack := strings.ToLower(strings.Join([]string{
				n.ID,
				it.Title,
				it.Why,
				it.Do,
				it.Evidence,
				it.Scope,
				strings.Join(it.Anchors, " "),
			}, " "))
			matched := strings.Contains(haystack, qLower)
			if !matched && len(tokens) > 0 {
				allTokens := true
				for _, tok := range tokens {
					cleanTok := strings.Trim(tok, "*\"'")
					if cleanTok != "" && !strings.Contains(haystack, cleanTok) {
						allTokens = false
						break
					}
				}
				matched = allTokens
			}
			if matched {
				out = append(out, matchedNoteItem{note: n, item: it})
			}
		}
	}
	return out
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

	fields := map[string]any{
		"entry_type": string(entryType),
		"type":       string(entryType),
		"text":       text,
	}
	if len(req.Anchors) > 0 {
		fields["anchors"] = req.Anchors
	}

	op := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      entryID,
		DossierScope: scope,
		Timestamp:    now,
		Source:       "agent",
		Fields:       fields,
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
			Anchors:      req.Anchors,
			Source:       "agent",
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		firstLine := text
		if nl := strings.IndexByte(firstLine, '\n'); nl >= 0 {
			firstLine = strings.TrimSpace(firstLine[:nl])
		}
		if runes := []rune(firstLine); len(runes) > 120 {
			firstLine = string(runes[:120])
		}
		title = firstLine
	}

	inboxStore := s.inboxStore
	if inboxStore == nil && s.dataDir != "" && s.repoID != "" {
		inboxStore = inbox.NewStore(s.dataDir, s.repoID)
	}
	if inboxStore != nil {
		var anchorChecks []inbox.AnchorCheck
		if s.repoRoot != "" && len(req.Anchors) > 0 {
			anchorChecks = inbox.CheckAnchors(s.repoRoot, req.Anchors)
		}
		var stepsCovered [2]int
		if req.StepIndex > 0 {
			stepsCovered = [2]int{req.StepIndex, req.StepIndex}
		}
		var exposures []delivery.ExposureRecord
		if s.ledger != nil {
			exposures = s.ledger.GetExposureList()
		}
		note := &inbox.Note{
			Source:         inbox.SourceMCPNote,
			ConversationID: strings.TrimSpace(req.ConversationID),
			StepsCovered:   stepsCovered,
			ExposureList:   exposures,
			Items: []inbox.NoteItem{
				{
					Type:         entryType,
					Title:        title,
					Why:          strings.TrimSpace(req.Why),
					Do:           strings.TrimSpace(req.Do),
					Anchors:      req.Anchors,
					AnchorChecks: anchorChecks,
					Evidence:     text,
					Scope:        scope,
				},
			},
			CreatedAt:    now,
			Consolidated: false,
		}
		_, _ = inboxStore.Write(note)
	}

	convID := strings.TrimSpace(req.ConversationID)
	if s.dataDir != "" && s.repoID != "" && convID != "" {
		if mark, err := state.LoadMark(s.dataDir, s.repoID, convID); err == nil && mark != nil {
			mark.LastNoteAt = now
			mark.NoteCount++
			if title != "" {
				mark.NoteTitles = append(mark.NoteTitles, title)
			}
			if req.StepIndex > mark.StepIndex {
				mark.StepIndex = req.StepIndex
			}
			_ = state.SaveMark(s.dataDir, s.repoID, mark)
		}
	}

	return MemoryNoteResponse{
		Result: fmt.Sprintf("Note recorded: %s", entryID),
	}, nil
}

const ftsAllTokensQuery = "a* OR b* OR c* OR d* OR e* OR f* OR g* OR h* OR i* OR j* OR k* OR l* OR m* OR n* OR o* OR p* OR q* OR r* OR s* OR t* OR u* OR v* OR w* OR x* OR y* OR z* OR 0* OR 1* OR 2* OR 3* OR 4* OR 5* OR 6* OR 7* OR 8* OR 9*"

// HandleMemoryStatus implements the memory_status (alias memory_stats) tool handler.
func (s *Server) HandleMemoryStatus(ctx context.Context, _ MemoryStatusRequest) (MemoryStatusResponse, error) {
	seenIDs := make(map[string]dossiers.Entry)
	if s.idx != nil {
		if entries, err := s.idx.SearchEntries(ctx, ftsAllTokensQuery); err == nil {
			for _, e := range entries {
				seenIDs[e.ID] = e
			}
		}
		for _, src := range []string{"", "agent", "doc-seed", "consolidator"} {
			if entries, err := s.idx.ListEntriesBySource(ctx, src); err == nil {
				for _, e := range entries {
					seenIDs[e.ID] = e
				}
			}
		}
	}

	activeCount := 0
	semanticCount := 0
	proceduralCount := 0
	episodicCount := 0
	retiredCount := 0
	scopes := make(map[string]bool)
	retiredSeen := make(map[string]bool)

	for _, e := range seenIDs {
		if e.DossierScope != "" {
			scopes[e.DossierScope] = true
		}
		isRetired := e.Retired
		if !isRetired && s.idx != nil {
			if hists, err := s.idx.GetHistory(ctx, e.ID); err == nil && len(hists) > 0 {
				isRetired = true
			}
		}
		if isRetired {
			retiredSeen[e.ID] = true
			continue
		}
		activeCount++
		switch e.EffectiveKind() {
		case dossiers.CognitiveKindSemantic:
			semanticCount++
		case dossiers.CognitiveKindProcedural:
			proceduralCount++
		case dossiers.CognitiveKindEpisodic:
			episodicCount++
		default:
			semanticCount++
		}
	}

	if s.idx != nil {
		for scope := range scopes {
			if hists, err := s.idx.QueryHistoryByScope(ctx, scope); err == nil {
				for _, h := range hists {
					if h.ID != "" {
						retiredSeen[h.ID] = true
					}
				}
			}
		}
	}
	retiredCount = len(retiredSeen)

	pendingNotes := 0
	totalNotes := 0
	inboxStore := s.inboxStore
	if inboxStore == nil && s.dataDir != "" && s.repoID != "" {
		inboxStore = inbox.NewStore(s.dataDir, s.repoID)
	}
	if inboxStore != nil {
		if all, err := inboxStore.ListAll(); err == nil {
			totalNotes = len(all)
			for _, n := range all {
				if !n.Consolidated {
					pendingNotes++
				}
			}
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Active Entries: %d (semantic: %d, procedural: %d, episodic: %d)\n",
		activeCount, semanticCount, proceduralCount, episodicCount))
	b.WriteString(fmt.Sprintf("Scopes: %d\n", len(scopes)))
	b.WriteString(fmt.Sprintf("Retired Entries: %d\n", retiredCount))
	b.WriteString(fmt.Sprintf("Inbox Notes: %d pending (%d total)", pendingNotes, totalNotes))

	return MemoryStatusResponse{Result: b.String()}, nil
}
