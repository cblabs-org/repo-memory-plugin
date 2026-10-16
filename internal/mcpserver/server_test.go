package mcpserver_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
	"github.com/cblabs-org/repo-memory-plugin/internal/mcpserver"
)

func TestHandleMemoryAsk_FallbackSearch(t *testing.T) {
	// Setup in-memory index
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	// Insert test data
	err = idx.InsertEpisode(context.Background(), index.EpisodeRecord{
		EpisodeID: "ep-123",
		SessionID: "sess-1",
		Content:   "This is an important architectural decision about database choice.",
		Date:      time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to insert episode: %v", err)
	}

	srv := mcpserver.NewServer(idx)

	req := mcpserver.MemoryAskRequest{
		Query: "database choice",
	}

	resp, err := srv.HandleMemoryAsk(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleMemoryAsk failed: %v", err)
	}

	if !strings.Contains(resp.Result, "unverified (episode search)") {
		t.Errorf("Expected response to contain 'unverified (episode search)', got: %s", resp.Result)
	}

	if !strings.Contains(resp.Result, "ep-123") {
		t.Errorf("Expected response to contain episode ID 'ep-123', got: %s", resp.Result)
	}
}

func TestMemoryNote_V2EntryTypesInboxAndMark(t *testing.T) {
	ctx := context.Background()
	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory index: %v", err)
	}
	defer idx.Close()

	dataDir := t.TempDir()
	repoID := "repo-mcp-v2"
	repoRoot := t.TempDir()
	opsDir := filepath.Join(repoRoot, ".memory", "ops")

	// Create an anchor file in repoRoot
	anchorFile := filepath.Join(repoRoot, "internal", "api", "handler.go")
	if err := os.MkdirAll(filepath.Dir(anchorFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(anchorFile, []byte("package api\n"), 0644); err != nil {
		t.Fatal(err)
	}

	srv := mcpserver.NewServer(idx,
		mcpserver.WithOpsDir(opsDir),
		mcpserver.WithRepoContext(dataDir, repoID, repoRoot),
	)

	allTypes := []string{
		string(dossiers.EntryTypeInvariant),
		string(dossiers.EntryTypeContract),
		string(dossiers.EntryTypeConvention),
		string(dossiers.EntryTypeRuledOut),
		string(dossiers.EntryTypeFailureSignature),
		string(dossiers.EntryTypeGotcha),
		string(dossiers.EntryTypeExternalBehaviour),
		string(dossiers.EntryTypeDecision),
		string(dossiers.EntryTypeProcedure),
		string(dossiers.EntryTypeRecipe),
		string(dossiers.EntryTypeInterface),
		string(dossiers.EntryTypeOpenIssue),
	}

	for i, typ := range allTypes {
		_, err := srv.HandleMemoryNote(ctx, mcpserver.MemoryNoteRequest{
			Scope:          "api",
			Text:           "Verified entry for type " + typ,
			Type:           typ,
			Title:          "Title for " + typ,
			Why:            "Rationale for " + typ,
			Do:             "Action for " + typ,
			Anchors:        []string{"internal/api/handler.go#ServeHTTP", "internal/api/missing.go"},
			ConversationID: "conv-mcp-test",
			StepIndex:      i + 1,
		})
		if err != nil {
			t.Fatalf("HandleMemoryNote failed for type %q: %v", typ, err)
		}
	}

	// Verify inbox notes were created with source "mcp-note" and anchor checks
	inboxStore := inbox.NewStore(dataDir, repoID)
	notes, err := inboxStore.ListUnconsolidated()
	if err != nil {
		t.Fatalf("ListUnconsolidated failed: %v", err)
	}
	if len(notes) != len(allTypes) {
		t.Fatalf("expected %d unconsolidated mcp-note items, got %d", len(allTypes), len(notes))
	}
	first := notes[0]
	if first.Source != inbox.SourceMCPNote {
		t.Errorf("expected source %q, got %q", inbox.SourceMCPNote, first.Source)
	}
	if len(first.Items) != 1 || len(first.Items[0].AnchorChecks) != 2 {
		t.Fatalf("expected 2 anchor checks on note item, got %+v", first.Items)
	}
	if !first.Items[0].AnchorChecks[0].Exists || first.Items[0].AnchorChecks[1].Exists {
		t.Errorf("unexpected anchor check results: %+v", first.Items[0].AnchorChecks)
	}

	// Verify conversation mark was updated and advanced
	mark, err := state.LoadMark(dataDir, repoID, "conv-mcp-test")
	if err != nil {
		t.Fatalf("LoadMark failed: %v", err)
	}
	if mark.NoteCount != len(allTypes) {
		t.Errorf("expected NoteCount %d, got %d", len(allTypes), mark.NoteCount)
	}
	if mark.StepIndex != len(allTypes) {
		t.Errorf("expected StepIndex %d, got %d", len(allTypes), mark.StepIndex)
	}
	if len(mark.NoteTitles) != len(allTypes) {
		t.Errorf("expected %d NoteTitles, got %d", len(allTypes), len(mark.NoteTitles))
	}
	if mark.LastNoteAt.IsZero() {
		t.Error("expected non-zero LastNoteAt on mark")
	}

	// Verify memory_status and memory_stats dispatch
	for _, toolName := range []string{"memory_status", "memory_stats", "memory.status", "memory.stats"} {
		rawOut, err := srv.DispatchTool(ctx, toolName, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("DispatchTool(%q) failed: %v", toolName, err)
		}
		statusResp, ok := rawOut.(mcpserver.MemoryStatusResponse)
		if !ok {
			t.Fatalf("expected MemoryStatusResponse, got %T", rawOut)
		}
		if !strings.Contains(statusResp.Result, "Active Entries: 12") {
			t.Errorf("expected 'Active Entries: 12' in %s output, got:\n%s", toolName, statusResp.Result)
		}
		if !strings.Contains(statusResp.Result, "Inbox Notes: 12 pending (12 total)") {
			t.Errorf("expected 'Inbox Notes: 12 pending (12 total)' in %s output, got:\n%s", toolName, statusResp.Result)
		}
	}
}
