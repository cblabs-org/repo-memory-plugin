package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/mcpserver"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Stdio MCP server for memory pull tools",
	Long: `Runs a JSON-RPC/stdio MCP server exposing the v2 pull tools:
  memory.brief, memory.why, memory.history, memory.search, memory.note

Each line on stdin is a JSON-RPC request; responses are written to stdout.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := config.DataDir()
		workspaceRoot, _ := os.Getwd()

		repoID := ""
		if entry, found, _ := config.FindRegistryEntry(dataDir, workspaceRoot); found && entry != nil {
			repoID = entry.RepoID
		}
		if repoID == "" {
			repoID = filepath.Base(workspaceRoot)
		}

		idxPath := filepath.Join(dataDir, repoID, "index.db")
		idx, err := index.NewIndex(idxPath)
		if err != nil {
			return fmt.Errorf("opening index: %w", err)
		}
		defer idx.Close()

		opsDir := filepath.Join(dataDir, repoID, "ops")
		ledger := delivery.NewLedger("mcp-session")

		srv := mcpserver.NewServer(idx,
			mcpserver.WithLedger(ledger),
			mcpserver.WithOpsWriter(ops.NewWriter(opsDir)),
		)

		// Simple JSON-RPC/stdio loop: read one JSON object per line, dispatch, respond.
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var req struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(line, &req); err != nil {
				writeJSONRPCError(cmd, nil, -32700, "Parse error")
				continue
			}

			result, err := srv.DispatchTool(cmd.Context(), req.Method, req.Params)
			if err != nil {
				writeJSONRPCError(cmd, req.ID, -32603, err.Error())
				continue
			}

			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      json.RawMessage(req.ID),
				"result":  result,
			}
			out, _ := json.Marshal(resp)
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
		}

		return scanner.Err()
	},
}

func writeJSONRPCError(cmd *cobra.Command, id json.RawMessage, code int, message string) {
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
