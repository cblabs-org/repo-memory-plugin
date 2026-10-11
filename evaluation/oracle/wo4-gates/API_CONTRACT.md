## API Contract (exact identifiers for hidden-test compatibility)

The hidden tests import your package and reference these exact identifiers.
You MUST use these names and signatures.

### Types
- `type PreToolUsePayload struct` — with fields:
  - `ConversationID string`
  - `ConversationIDSnake string`
  - `InvocationNum int`
  - `ToolName string`
  - `ToolCall struct { Name string; Args json.RawMessage }`
  - `TranscriptPath string`
  - `TranscriptPathSnake string`
  - `WorkspacePaths []string`
  - `WorkspacePathsSnake []string`
- `type PreToolUseResponse struct` — with fields:
  - `Decision string`
  - `Reason string`
- `type HookOption func(*HookOptions)`

### Constants
- `var allowResponse = []byte("{\"decision\":\"allow\"}")`

### Functions
- `func (p *PreToolUsePayload) Normalize()`
- `func HandlePreToolUse(ctx context.Context, payload []byte, opts ...HookOption) ([]byte, error)`
- `func WithWorkspace(ws string) HookOption`
