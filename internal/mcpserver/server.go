package mcpserver

import (
	"context"
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

type Server struct {
	idx *index.Index
}

// NewServer creates a new MCP server skeleton.
func NewServer(idx *index.Index) *Server {
	return &Server{idx: idx}
}

// MemoryAskRequest represents the input to the memory.ask tool.
type MemoryAskRequest struct {
	Query string `json:"query"`
}

// MemoryAskResponse represents the output of the memory.ask tool.
type MemoryAskResponse struct {
	Result string `json:"result"`
}

// HandleMemoryAsk implements the memory.ask tool handler.
func (s *Server) HandleMemoryAsk(ctx context.Context, req MemoryAskRequest) (MemoryAskResponse, error) {
	// TODO: Check if there are verified claims to return.
	// Currently there are none, so we immediately fall back to the episode index.
	
	results, err := s.idx.SearchEpisodes(ctx, req.Query)
	if err != nil {
		return MemoryAskResponse{}, fmt.Errorf("searching episodes: %w", err)
	}

	if len(results) == 0 {
		return MemoryAskResponse{Result: "No results found."}, nil
	}

	var output string
	for i, res := range results {
		output += fmt.Sprintf("Result %d [unverified (episode search)]:\n", i+1)
		output += fmt.Sprintf("Episode ID: %s\n", res.EpisodeID)
		if res.Snippet != "" {
			output += fmt.Sprintf("Match: %s\n", res.Snippet)
		} else {
			output += fmt.Sprintf("Match: %s\n", res.Content)
		}
		output += "\n"
	}

	return MemoryAskResponse{Result: output}, nil
}
