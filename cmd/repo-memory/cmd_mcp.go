package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
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
		if entry, found, _ := config.FindRegistryEntry(dataDir, workspaceRoot); found && entry != nil {
			repoID = entry.RepoID
		}
		if repoID == "" {
			gitCmd := exec.Command("git", "rev-parse", "--show-toplevel")
			gitCmd.Dir = workspaceRoot
			if out, err := gitCmd.Output(); err == nil {
				repoID = filepath.Base(strings.TrimSpace(string(out)))
			}
		}
		if repoID == "" {
			return fmt.Errorf("could not determine repo ID; run from a git repository or enrol first")
		}

		// Ensure the index directory exists so NewIndex can create the DB.
		idxDir := filepath.Join(dataDir, repoID)
		if err := os.MkdirAll(idxDir, 0755); err != nil {
			return fmt.Errorf("creating index directory: %w", err)
		}

		idxPath := filepath.Join(idxDir, "index.db")
		idx, err := index.NewIndex(idxPath)
		if err != nil {
			return fmt.Errorf("opening index: %w", err)
		}
		defer idx.Close()

		opsDir := filepath.Join(dataDir, repoID, "ops")

		srv := mcpserver.NewServer(idx,
			mcpserver.WithOpsWriter(ops.NewWriter(opsDir)),
		)
		// Note: pull exposure tracking requires a per-conversation ledger.
		// The MCP server does not have a conversation ID at startup, so
		// pull exposures are not persisted until the harness passes one.
		// TODO: Accept conversation ID from client metadata in initialize.

		in := cmd.InOrStdin()
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var req jsonRPCRequest
			if err := json.Unmarshal(line, &req); err != nil {
				writeError(cmd, nil, -32700, "Parse error")
				continue
			}

			// Notifications have no id — do not respond (JSON-RPC 2.0 §4.1).
			if req.ID == nil {
				continue
			}

			switch req.Method {
			case "initialize":
				writeResult(cmd, req.ID, initializeResult(req.Params))

			case "ping":
				writeResult(cmd, req.ID, map[string]any{})

			case "tools/list":
				writeResult(cmd, req.ID, toolsListResult())

			case "tools/call":
				var call toolsCallParams
				if err := json.Unmarshal(req.Params, &call); err != nil {
					writeError(cmd, req.ID, -32602, "Invalid params: "+err.Error())
					continue
				}
				result, err := srv.DispatchTool(cmd.Context(), call.Name, call.Arguments)
				if err != nil {
					// MCP: tool execution errors are normal results with isError: true,
					// not JSON-RPC errors (which are for protocol problems).
					writeResult(cmd, req.ID, mcpToolError(err.Error()))
					continue
				}
				writeResult(cmd, req.ID, mcpToolResult(result))

			default:
				writeError(cmd, req.ID, -32601, "Method not found: "+req.Method)
			}
		}

		return scanner.Err()
	},
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
	// All handlers return structs with a Result field.
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

func writeResult(cmd *cobra.Command, id json.RawMessage, result any) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"result":  result,
	}
	out, _ := json.Marshal(resp)
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
}

func writeError(cmd *cobra.Command, id json.RawMessage, code int, message string) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	out, _ := json.Marshal(resp)
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
}
