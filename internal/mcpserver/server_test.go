package mcpserver_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
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
