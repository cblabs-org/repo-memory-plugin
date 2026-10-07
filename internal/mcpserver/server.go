package mcpserver

import (
	"context"
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// Server represents the MCP server instance providing memory tools over the index.
type Server struct {
	idx       *index.Index
	ledger    *delivery.Ledger
	opsWriter *ops.Writer
}

// ServerOption is a functional option for configuring a Server.
type ServerOption func(*Server)

// WithLedger attaches an exposure ledger to the server to record entry pulls.
func WithLedger(l *delivery.Ledger) ServerOption {
	return func(s *Server) {
		s.ledger = l
	}
}

// WithOpsWriter attaches an ops log writer for recording agent notes.
func WithOpsWriter(w *ops.Writer) ServerOption {
	return func(s *Server) {
		s.opsWriter = w
	}
}

// WithOpsDir configures the ops directory for the ops log writer.
func WithOpsDir(dir string) ServerOption {
	return func(s *Server) {
		s.opsWriter = ops.NewWriter(dir)
	}
}

// NewServer creates a new MCP server with the provided index and options.
func NewServer(idx *index.Index, opts ...ServerOption) *Server {
	s := &Server{idx: idx}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// SetLedger sets or replaces the exposure ledger on the server.
func (s *Server) SetLedger(l *delivery.Ledger) {
	s.ledger = l
}

// SetOpsWriter sets or replaces the ops log writer on the server.
func (s *Server) SetOpsWriter(w *ops.Writer) {
	s.opsWriter = w
}

// SupportedTools lists the names of MCP pull tools supported by this server.
var SupportedTools = []string{
	"memory.brief",
	"memory.why",
	"memory.history",
	"memory.search",
	"memory.note",
}

// MemoryAskRequest represents the input to the legacy memory.ask tool.
type MemoryAskRequest struct {
	Query string `json:"query"`
}

// MemoryAskResponse represents the output of the legacy memory.ask tool.
type MemoryAskResponse struct {
	Result string `json:"result"`
}

// HandleMemoryAsk implements the memory.ask tool handler.
// Deprecated: memory.ask is superseded by v2 pull tools (memory.brief, memory.why, memory.history, memory.search, memory.note).
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
