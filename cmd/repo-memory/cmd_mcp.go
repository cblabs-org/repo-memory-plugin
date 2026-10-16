package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/mcpserver"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Stdio MCP server for memory pull tools",
	Long: `Runs an MCP stdio server exposing the v2 pull tools:
  memory_brief, memory_why, memory_history, memory_search, memory_note

The server speaks the Model Context Protocol (JSON-RPC 2.0 over stdio),
handling initialize, ping, notifications/initialized, tools/list, and tools/call.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := config.DataDir()
		workspaceRoot, _ := os.Getwd()

		// Derive repoID consistently with the hooks: use registry or git rev-parse.
		repoID := ""
		repoRoot := workspaceRoot
		if entry, found, _ := config.FindRegistryEntry(dataDir, workspaceRoot); found && entry != nil {
			repoID = entry.RepoID
			repoRoot = entry.RootPath
		}
		if repoID == "" {
			if root, err := gitops.FindRepoRoot(workspaceRoot); err == nil {
				repoRoot = root
				repoID, _ = gitops.GenerateRepoID(root)
			}
		}

		var srv *mcpserver.Server
		if repoID == "" || !config.IsEnrolledFast(workspaceRoot) {
			// Non-git workspace or unenrolled repo: serve tools but each
			// returns "repo not enrolled", without creating any files.
			srv = mcpserver.NewServer(nil)
		} else {
			// Enrolled repo: create index and serve normally.
			idxPath := index.IndexPath(dataDir, repoID)
			if err := os.MkdirAll(filepath.Dir(idxPath), 0755); err != nil {
				return fmt.Errorf("creating index directory: %w", err)
			}

			idx, err := index.NewIndex(idxPath)
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer idx.Close()

			opsDir := filepath.Join(repoRoot, ".memory", "ops")
			srv = mcpserver.NewServer(idx,
				mcpserver.WithOpsWriter(ops.NewWriter(opsDir)),
				mcpserver.WithRepoContext(dataDir, repoID, repoRoot),
			)
		}
		// Note: pull exposure tracking requires a per-conversation ledger.
		// The MCP server does not have a conversation ID at startup, so
		// pull exposures are not persisted until the harness passes one.
		// TODO: Accept conversation ID from client metadata in initialize.

		return serveMCP(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), srv)
	},
}

// serveMCP runs the MCP stdio loop, reading JSON-RPC requests from in and
// writing responses to out. Extracted from RunE for testability.
func serveMCP(ctx context.Context, in io.Reader, out io.Writer, srv *mcpserver.Server) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeJSONRPC(out, nil, -32700, "Parse error", nil)
			continue
		}

		// Notifications have no id — do not respond (JSON-RPC 2.0 §4.1).
		if req.ID == nil {
			continue
		}

		switch req.Method {
		case "initialize":
			writeResult2(out, req.ID, initializeResult(req.Params))

		case "ping":
			writeResult2(out, req.ID, map[string]any{})

		case "tools/list":
			writeResult2(out, req.ID, toolsListResult())

		case "tools/call":
			var call toolsCallParams
			if err := json.Unmarshal(req.Params, &call); err != nil {
				writeJSONRPC(out, req.ID, -32602, "Invalid params: "+err.Error(), nil)
				continue
			}
			result, err := srv.DispatchTool(ctx, call.Name, call.Arguments)
			if err != nil {
				// MCP: tool execution errors are normal results with isError: true,
				// not JSON-RPC errors (which are for protocol problems).
				writeResult2(out, req.ID, mcpToolError(err.Error()))
				continue
			}
			writeResult2(out, req.ID, mcpToolResult(result))

		default:
			writeJSONRPC(out, req.ID, -32601, "Method not found: "+req.Method, nil)
		}
	}

	return scanner.Err()
}

// --- JSON-RPC types ---

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // nil for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// --- MCP protocol responses ---

// supportedProtocolVersion is the MCP protocol version we implement.
const supportedProtocolVersion = "2024-11-05"

func initializeResult(params json.RawMessage) map[string]any {
	// Echo the client's requested protocol version if we support it.
	version := supportedProtocolVersion
	if len(params) > 0 {
		var initParams struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(params, &initParams) == nil && initParams.ProtocolVersion != "" {
			if initParams.ProtocolVersion == supportedProtocolVersion {
				version = initParams.ProtocolVersion
			}
		}
	}
	return map[string]any{
		"protocolVersion": version,
		"serverInfo": map[string]any{
			"name":    "repo-memory",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
	}
}

func toolsListResult() map[string]any {
	schemas := mcpserver.ToolSchemas()
	tools := make([]map[string]any, 0, len(schemas))
	for _, schema := range schemas {
		tool := map[string]any{
			"name":        schema.Name,
			"description": schema.Description,
			"inputSchema": schema.InputSchema,
		}
		tools = append(tools, tool)
	}
	return map[string]any{"tools": tools}
}

func mcpToolResult(result any) map[string]any {
	text := extractResultText(result)
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
}

func mcpToolError(message string) map[string]any {
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": message},
		},
		"isError": true,
	}
}

func extractResultText(result any) string {
	b, _ := json.Marshal(result)
	var generic map[string]string
	if json.Unmarshal(b, &generic) == nil {
		if text := generic["result"]; text != "" {
			return text
		}
	}
	return string(b)
}

// --- JSON-RPC helpers ---

// writeResult2 writes a JSON-RPC success response to a writer.
func writeResult2(out io.Writer, id json.RawMessage, result any) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"result":  result,
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintln(out, string(b))
}

// writeJSONRPC writes a JSON-RPC error response to a writer.
func writeJSONRPC(out io.Writer, id json.RawMessage, code int, message string, _ any) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintln(out, string(b))
}

// writeResult writes a JSON-RPC success response to a cobra command's stdout.
// Kept for backward compatibility with existing callers.
func writeResult(cmd *cobra.Command, id json.RawMessage, result any) {
	writeResult2(cmd.OutOrStdout(), id, result)
}

// writeError writes a JSON-RPC error response to a cobra command's stdout.
// Kept for backward compatibility with existing callers.
func writeError(cmd *cobra.Command, id json.RawMessage, code int, message string) {
	writeJSONRPC(cmd.OutOrStdout(), id, code, message, nil)
}
