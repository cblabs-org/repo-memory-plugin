## API Contract (exact identifiers for hidden-test compatibility)

The hidden tests import your package and reference these exact identifiers.
You MUST use these names and signatures.

### Types
- `type Server struct`
- `type ServerOption func(*Server)`
- `type MemoryBriefRequest struct` — with fields:
  - `Scope string`
- `type MemoryBriefResponse struct` — with fields:
  - `Result string`
- `type MemoryWhyRequest struct` — with fields:
  - `EntryID string`
- `type MemoryWhyResponse struct` — with fields:
  - `Result string`
- `type MemoryHistoryRequest struct` — with fields:
  - `Scope string`
- `type MemoryHistoryResponse struct` — with fields:
  - `Result string`
- `type MemorySearchRequest struct` — with fields:
  - `Query string`
- `type MemorySearchResponse struct` — with fields:
  - `Result string`
- `type MemoryNoteRequest struct` — with fields:
  - `Scope string`
  - `Text string`
  - `Type string`
- `type MemoryNoteResponse struct` — with fields:
  - `Result string`
- `type ToolSchema struct` — with fields:
  - `Name string`
  - `Description string`
  - `InputSchema map[string]any`

### Constants
- `var SupportedTools = []string{"memory_brief", "memory_why", "memory_history", "memory_search", "memory_note"}`

### Functions
- `func NewServer(idx *index.Index, opts ...ServerOption) *Server`
- `func WithLedger(l *delivery.Ledger) ServerOption`
- `func WithOpsWriter(w *ops.Writer) ServerOption`
- `func WithOpsDir(dir string) ServerOption`
- `func (s *Server) SetLedger(l *delivery.Ledger)`
- `func (s *Server) SetOpsWriter(w *ops.Writer)`
- `func (s *Server) HandleMemoryBrief(ctx context.Context, req MemoryBriefRequest) (MemoryBriefResponse, error)`
- `func (s *Server) HandleMemoryWhy(ctx context.Context, req MemoryWhyRequest) (MemoryWhyResponse, error)`
- `func (s *Server) HandleMemorySearch(ctx context.Context, req MemorySearchRequest) (MemorySearchResponse, error)`
- `func (s *Server) HandleMemoryHistory(ctx context.Context, req MemoryHistoryRequest) (MemoryHistoryResponse, error)`
- `func (s *Server) HandleMemoryNote(ctx context.Context, req MemoryNoteRequest) (MemoryNoteResponse, error)`
- `func (s *Server) DispatchTool(ctx context.Context, name string, params json.RawMessage) (any, error)`
- `func ToolSchemas() []ToolSchema`
