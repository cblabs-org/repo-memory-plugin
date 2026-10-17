/*
cmd_mcp.go implements the `repo-memory mcp` stdio server command for the Model
Context Protocol (JSON-RPC 2.0), resolving the active enrolled repository from
either the current working directory or the most recently active registry entry
when spawned from a plugin root directory.
*/

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

// mcpCmd is the cobra command that starts the stdio MCP server.
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Stdio MCP server for memory pull tools",
	Long: `Runs an MCP stdio server exposing the v2 pull tools:
  memory_brief, memory_why, memory_history, memory_search, memory_note

The server speaks the Model Context Protocol (JSON-RPC 2.0 over stdio),
handling initialize, ping, notifications/initialized, tools/list, and tools/call.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Resolve data directory and initial working directory ---
		dataDir := config.DataDir()
		workspaceRoot, _ := os.Getwd()

		var activeIdx *index.Index
		defer func() {
			if activeIdx != nil {
				_ = activeIdx.Close()
			}
		}()

		// --- Step 2: Define lazy repository binding closure ---
		tryBindServer := func() *mcpserver.Server {
			repoID, repoRoot, ok := resolveEnrolledRepoForMCP(dataDir, workspaceRoot)
			if !ok {
				return nil
			}
			idxPath := index.IndexPath(dataDir, repoID)
			if err := os.MkdirAll(filepath.Dir(idxPath), 0755); err != nil {
				return nil
			}
			idx, err := index.NewIndex(idxPath)
			if err != nil {
				return nil
			}
			if activeIdx != nil {
				_ = activeIdx.Close()
			}
			activeIdx = idx
			opsDir := filepath.Join(repoRoot, ".memory", "ops")
			return mcpserver.NewServer(idx,
				mcpserver.WithOpsWriter(ops.NewWriter(opsDir)),
				mcpserver.WithRepoContext(dataDir, repoID, repoRoot),
			)
		}

		// --- Step 3: Bind initial MCP server and run JSON-RPC stdio loop ---
		srv := tryBindServer()
		if srv == nil {
			// Non-git workspace or unenrolled repo: serve tools and lazily re-check
			// on tools/call in case `repo-memory enroll` runs while MCP is alive.
			srv = mcpserver.NewServer(nil)
		}

		return serveMCPWithRebind(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), srv, tryBindServer)
	},
}

// resolveEnrolledRepoForMCP resolves the active (repoID, repoRoot) for the MCP stdio server.
// Direct cwd matches take precedence; when Antigravity spawns a plugin MCP server with cwd set
// to PLUGIN_ROOT (~/.gemini/config/plugins/...), it falls back to the most recently active
// enrolled repository in <data-dir>/registry.json.
func resolveEnrolledRepoForMCP(dataDir, workspaceRoot string) (string, string, bool) {
	// --- Step 1: Check whether workspaceRoot directly matches an enrolled repo ---
	if entry, found, _ := config.FindRegistryEntry(dataDir, workspaceRoot); found && entry != nil {
		if (entry.Enabled == nil || *entry.Enabled) && config.IsEnrolledFast(entry.RootPath) {
			return entry.RepoID, entry.RootPath, true
		}
	}
	if root, err := gitops.FindRepoRoot(workspaceRoot); err == nil && config.IsEnrolledFast(root) {
		if repoID, err := gitops.GenerateRepoID(root); err == nil && repoID != "" {
			return repoID, root, true
		}
	}

	// --- Step 2: Fall back to the most recently active enrolled repo in registry.json ---
	reg, err := config.LoadRegistry(dataDir)
	if err != nil || len(reg) == 0 {
		return "", "", false
	}

	var bestRepoID, bestRoot string
	var bestMtime int64 = -1
	seenRepoID := make(map[string]bool)

	for _, entry := range reg {
		if entry.RepoID == "" || entry.RootPath == "" || seenRepoID[entry.RepoID] {
			continue
		}
		if entry.Enabled != nil && !*entry.Enabled {
			continue
		}
		if !config.IsEnrolledFast(entry.RootPath) {
			continue
		}
		seenRepoID[entry.RepoID] = true

		mtime := latestRepoActivityUnixNano(dataDir, entry.RepoID)
		if mtime > bestMtime || (mtime == bestMtime && (bestRoot == "" || entry.RootPath < bestRoot)) {
			bestMtime = mtime
			bestRepoID = entry.RepoID
			bestRoot = entry.RootPath
		}
	}

	if bestRepoID == "" {
		return "", "", false
	}
	return bestRepoID, bestRoot, true
}

// latestRepoActivityUnixNano returns the newest modification timestamp (in Unix nanoseconds)
// across the repository's SQLite index and state directories.
func latestRepoActivityUnixNano(dataDir, repoID string) int64 {
	// --- Step 1: Define file and directory mtime helpers ---
	var latest int64
	checkPath := func(p string) {
		if fi, err := os.Stat(p); err == nil {
			if t := fi.ModTime().UnixNano(); t > latest {
				latest = t
			}
		}
	}
	checkDir := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if info, err := e.Info(); err == nil {
				if t := info.ModTime().UnixNano(); t > latest {
					latest = t
				}
			}
		}
	}

	// --- Step 2: Inspect index.db, freshness.json, ledgers, and marks ---
	checkPath(index.IndexPath(dataDir, repoID))
	for _, base := range []string{
		filepath.Join(dataDir, "repos", repoID),
		filepath.Join(dataDir, repoID),
	} {
		checkPath(filepath.Join(base, "state", "freshness.json"))
		checkDir(filepath.Join(base, "state", "ledgers"))
		checkDir(filepath.Join(base, "state", "marks"))
	}
	return latest
}

// serveMCP runs the MCP stdio loop, reading JSON-RPC requests from in and
// writing responses to out. Extracted from RunE for testability.
func serveMCP(ctx context.Context, in io.Reader, out io.Writer, srv *mcpserver.Server) error {
	return serveMCPWithRebind(ctx, in, out, srv, nil)
}

// serveMCPWithRebind runs the MCP stdio loop and invokes rebind before each tools/call.
func serveMCPWithRebind(ctx context.Context, in io.Reader, out io.Writer, srv *mcpserver.Server, rebind func() *mcpserver.Server) error {
	// --- Step 1: Configure line scanner with 1 MiB buffer ---
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	// --- Step 2: Dispatch incoming JSON-RPC 2.0 requests ---
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
			if rebind != nil {
				if rebound := rebind(); rebound != nil {
					srv = rebound
				} else {
					srv = mcpserver.NewServer(nil)
				}
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

// jsonRPCRequest represents an inbound JSON-RPC 2.0 request or notification.
type jsonRPCRequest struct {
	// JSONRPC is the protocol version string ("2.0").
	JSONRPC string `json:"jsonrpc"`
	// ID is the request identifier (nil for notifications).
	ID json.RawMessage `json:"id,omitempty"`
	// Method is the RPC method name.
	Method string `json:"method"`
	// Params is the raw JSON method parameters.
	Params json.RawMessage `json:"params,omitempty"`
}

// toolsCallParams represents the parameters of an MCP tools/call request.
type toolsCallParams struct {
	// Name is the tool name to invoke.
	Name string `json:"name"`
	// Arguments is the raw JSON tool arguments object.
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// --- MCP protocol responses ---

// supportedProtocolVersion is the MCP protocol version we implement.
const supportedProtocolVersion = "2024-11-05"

// initializeResult builds the MCP initialize response payload.
func initializeResult(params json.RawMessage) map[string]any {
	// --- Step 1: Negotiate protocol version ---
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
	// --- Step 2: Return server info and capabilities ---
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

// toolsListResult returns the MCP tools/list response payload.
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

// mcpToolResult wraps a tool execution result into an MCP content array.
func mcpToolResult(result any) map[string]any {
	text := extractResultText(result)
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
}

// mcpToolError wraps a tool error message into an MCP isError content response.
func mcpToolError(message string) map[string]any {
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": message},
		},
		"isError": true,
	}
}

// extractResultText extracts the string `"result"` field from a tool response struct,
// falling back to its JSON serialization.
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
