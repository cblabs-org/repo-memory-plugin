/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/mcpserver/server.go
 * Description: Defines the MCP Server instance, functional configuration options,
 *              tool schema definitions, and JSON-RPC tool dispatcher for
 *              memory_brief, memory_why, memory_history, memory_search,
 *              memory_note, and memory_status.
 *
 * Exported Types / Functions:
 * - Server:            MCP server instance providing repository memory pull tools.
 *   - NewServer():     Constructs a Server with an index and functional options.
 *   - SetLedger():     Sets or replaces the exposure ledger on the server.
 *   - SetOpsWriter():  Sets or replaces the ops log writer on the server.
 *   - HandleMemoryAsk(): Legacy episode-search handler (deprecated).
 *   - DispatchTool():  Routes a named tool call and raw JSON parameters to its handler.
 * - ServerOption:      Functional option for configuring a Server.
 * - WithLedger():      Attaches an exposure ledger to record entry pulls.
 * - WithOpsWriter():   Attaches an ops log writer for recording agent notes.
 * - WithOpsDir():      Configures the directory for the ops log writer.
 * - WithInboxStore():  Attaches an inbox note store to the server.
 * - WithRepoContext(): Configures repository dataDir, repoID, and repoRoot.
 * - SupportedTools:    Canonical underscore-formatted MCP tool names.
 * - MemoryAskRequest:  Input parameters for the legacy memory.ask tool.
 * - MemoryAskResponse: Output payload for the legacy memory.ask tool.
 * - ToolSchema:        MCP tool name, description, and JSON Schema definition.
 * - ToolSchemas():     Returns schema definitions for all supported MCP pull tools.
 */

package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// Server represents the MCP server instance providing memory tools over the index.
type Server struct {
	idx        *index.Index     // SQLite FTS5 index backing memory pull queries.
	ledger     *delivery.Ledger // Optional conversation exposure ledger for recording pulls.
	opsWriter  *ops.Writer      // Optional operation log writer for persisting memory_note ops.
	dataDir    string           // Root repo-memory data directory path.
	repoID     string           // Canonical 16-hex repository identifier used to scope ledger and inbox writes.
	repoRoot   string           // Canonical root path of the enrolled repository.
	inboxStore *inbox.Store     // Optional inbox note store for unconsolidated notes.
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

// WithInboxStore attaches an inbox note store to the server.
func WithInboxStore(store *inbox.Store) ServerOption {
	return func(s *Server) {
		s.inboxStore = store
	}
}

// WithRepoContext configures the repository data directory, ID, and root path.
func WithRepoContext(dataDir, repoID, repoRoot string) ServerOption {
	return func(s *Server) {
		s.dataDir = dataDir
		s.repoID = repoID
		s.repoRoot = repoRoot
		if s.inboxStore == nil && dataDir != "" && repoID != "" {
			s.inboxStore = inbox.NewStore(dataDir, repoID)
		}
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

// SupportedTools lists the canonical tool names (underscore format for portability).
var SupportedTools = []string{
	"memory_brief",
	"memory_why",
	"memory_history",
	"memory_search",
	"memory_note",
	"memory_status",
}

// MemoryAskRequest represents the input to the legacy memory.ask tool.
type MemoryAskRequest struct {
	Query string `json:"query"` // Free-text episode search query.
}

// MemoryAskResponse represents the output of the legacy memory.ask tool.
type MemoryAskResponse struct {
	Result string `json:"result"` // Formatted search results or empty-match message.
}

// HandleMemoryAsk implements the memory.ask tool handler.
// Deprecated: memory.ask is superseded by v2 pull tools (memory.brief, memory.why, memory.history, memory.search, memory.note).
func (s *Server) HandleMemoryAsk(ctx context.Context, req MemoryAskRequest) (MemoryAskResponse, error) {
	// --- Step 1: Query the episode FTS index ---
	results, err := s.idx.SearchEpisodes(ctx, req.Query)
	if err != nil {
		return MemoryAskResponse{}, fmt.Errorf("searching episodes: %w", err)
	}

	if len(results) == 0 {
		return MemoryAskResponse{Result: "No results found."}, nil
	}

	// --- Step 2: Format matched episode snippets into a plain-text response ---
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

// ToolSchema describes an MCP tool's name, description, and input schema for registration.
type ToolSchema struct {
	Name        string         `json:"name"`        // Canonical underscore-formatted tool name.
	Description string         `json:"description"` // Human- and model-readable tool description.
	InputSchema map[string]any `json:"inputSchema"` // JSON Schema object describing tool parameters.
}

// ToolSchemas returns the schema descriptions for all supported MCP pull tools.
// Tool names use underscores for portability across all model APIs.
func ToolSchemas() []ToolSchema {
	// --- Step 1: Return static JSON Schema descriptors for the 6 MCP pull tools ---
	return []ToolSchema{
		{
			Name:        "memory_brief",
			Description: "Get a brief summary of active entries for a dossier scope",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"scope": map[string]any{"type": "string", "description": "The dossier scope to summarise"},
				},
				"required": []string{"scope"},
			},
		},
		{
			Name:        "memory_why",
			Description: "Get full details and provenance for a specific entry",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"entryId": map[string]any{"type": "string", "description": "The entry ID to look up"},
				},
				"required": []string{"entryId"},
			},
		},
		{
			Name:        "memory_history",
			Description: "Get retired and superseded entries for a scope",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"scope": map[string]any{"type": "string", "description": "The dossier scope to query history for"},
				},
				"required": []string{"scope"},
			},
		},
		{
			Name:        "memory_search",
			Description: "Search entries by keyword (FTS5)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search query string"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "memory_note",
			Description: "Record a new knowledge entry from the agent",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"scope":          map[string]any{"type": "string", "description": "The dossier scope for the note"},
					"text":           map[string]any{"type": "string", "description": "The note text to record"},
					"type":           map[string]any{"type": "string", "description": "Entry type (decision, invariant, contract, convention, gotcha, recipe, etc.)"},
					"title":          map[string]any{"type": "string", "description": "Optional short title for the note"},
					"why":            map[string]any{"type": "string", "description": "Optional rationale or cause"},
					"do":             map[string]any{"type": "string", "description": "Optional actionable remediation or steps"},
					"anchors":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional repository path anchors"},
					"conversationId": map[string]any{"type": "string", "description": "Optional conversation ID"},
					"stepIndex":      map[string]any{"type": "integer", "description": "Optional step index in the conversation"},
				},
				"required": []string{"scope", "text"},
			},
		},
		{
			Name:        "memory_status",
			Description: "Get repository memory status, entry counts by cognitive lifecycle, and pending inbox note count",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// DispatchTool routes a tool call by name to the appropriate handler.
// Accepts both underscore names (canonical) and dotted names (legacy aliases).
func (s *Server) DispatchTool(ctx context.Context, name string, params json.RawMessage) (any, error) {
	// --- Step 1: Verify server index availability ---
	if s.idx == nil {
		return nil, fmt.Errorf("repo not enrolled: run 'repo-memory enroll' first")
	}
	// --- Step 2: Unmarshal tool parameters and dispatch to the matching handler ---
	switch name {
	case "memory_brief", "memory.brief":
		var req MemoryBriefRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("parsing memory_brief params: %w", err)
		}
		return s.HandleMemoryBrief(ctx, req)
	case "memory_why", "memory.why":
		var req MemoryWhyRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("parsing memory_why params: %w", err)
		}
		return s.HandleMemoryWhy(ctx, req)
	case "memory_history", "memory.history":
		var req MemoryHistoryRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("parsing memory_history params: %w", err)
		}
		return s.HandleMemoryHistory(ctx, req)
	case "memory_search", "memory.search":
		var req MemorySearchRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("parsing memory_search params: %w", err)
		}
		return s.HandleMemorySearch(ctx, req)
	case "memory_note", "memory.note":
		var req MemoryNoteRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("parsing memory_note params: %w", err)
		}
		return s.HandleMemoryNote(ctx, req)
	case "memory_status", "memory_stats", "memory.status", "memory.stats":
		var req MemoryStatusRequest
		if len(params) > 0 {
			if err := json.Unmarshal(params, &req); err != nil {
				return nil, fmt.Errorf("parsing memory_status params: %w", err)
			}
		}
		return s.HandleMemoryStatus(ctx, req)
	case "memory_ask", "memory.ask":
		var req MemoryAskRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("parsing memory_ask params: %w", err)
		}
		return s.HandleMemoryAsk(ctx, req)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}
