package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/mcpserver"
)

func TestServeMCP(t *testing.T) {
	// Build a sequence of JSON-RPC requests.
	var input bytes.Buffer

	// 1. initialize
	writeJSON(&input, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"protocolVersion": "2024-11-05"},
	})

	// 2. notification (no id) — should be silently dropped, no response
	writeJSON(&input, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})

	// 3. ping
	writeJSON(&input, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "ping",
	})

	// 4. tools/list
	writeJSON(&input, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/list",
	})

	// 5. tools/call on unenrolled server — should return isError result
	writeJSON(&input, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params":  map[string]any{"name": "memory_brief", "arguments": map[string]any{"scope": "test"}},
	})

	// 6. unknown method — should return JSON-RPC error
	writeJSON(&input, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "nonexistent/method",
	})

	var output bytes.Buffer

	// Use a nil-index server (unenrolled repo).
	srv := mcpserver.NewServer(nil)
	err := serveMCP(context.Background(), &input, &output, srv)
	if err != nil {
		t.Fatalf("serveMCP returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")

	// Expect 5 responses (notification produces none).
	if len(lines) != 5 {
		t.Fatalf("expected 5 response lines, got %d:\n%s", len(lines), output.String())
	}

	// 1. initialize
	resp1 := parseResponse(t, lines[0])
	result1 := assertResult(t, resp1, "initialize")
	if result1["protocolVersion"] != "2024-11-05" {
		t.Errorf("initialize: expected protocolVersion 2024-11-05, got %v", result1["protocolVersion"])
	}
	serverInfo, ok := result1["serverInfo"].(map[string]any)
	if !ok || serverInfo["name"] != "repo-memory" {
		t.Errorf("initialize: expected serverInfo.name=repo-memory, got %v", result1["serverInfo"])
	}

	// 2. ping
	resp2 := parseResponse(t, lines[1])
	result2 := assertResult(t, resp2, "ping")
	if len(result2) != 0 {
		t.Errorf("ping: expected empty object {}, got %v", result2)
	}

	// 3. tools/list
	resp3 := parseResponse(t, lines[2])
	result3 := assertResult(t, resp3, "tools/list")
	toolsRaw, ok := result3["tools"].([]any)
	if !ok || len(toolsRaw) == 0 {
		t.Fatalf("tools/list: expected non-empty tools array, got %v", result3)
	}
	for _, tr := range toolsRaw {
		tool, ok := tr.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tool["name"].(string)
		if strings.Contains(name, ".") {
			t.Errorf("tools/list: tool name %q contains dot, expected underscores", name)
		}
		if _, hasSchema := tool["inputSchema"]; !hasSchema {
			t.Errorf("tools/list: tool %q missing inputSchema", name)
		}
	}

	// 4. tools/call (unenrolled) — isError result, not JSON-RPC error
	resp4 := parseResponse(t, lines[3])
	result4 := assertResult(t, resp4, "tools/call")
	if result4["isError"] != true {
		t.Errorf("tools/call: expected isError=true for unenrolled repo, got %v", result4)
	}
	content, ok := result4["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("tools/call: expected content array, got %v", result4)
	}
	firstContent, _ := content[0].(map[string]any)
	text, _ := firstContent["text"].(string)
	if !strings.Contains(text, "not enrolled") {
		t.Errorf("tools/call: expected 'not enrolled' in error text, got %q", text)
	}

	// 5. unknown method — JSON-RPC error
	resp5 := parseResponse(t, lines[4])
	errObj, ok := resp5["error"].(map[string]any)
	if !ok {
		t.Fatalf("unknown method: expected JSON-RPC error object, got %v", resp5)
	}
	if code, _ := errObj["code"].(float64); code != -32601 {
		t.Errorf("unknown method: expected code -32601, got %v", errObj["code"])
	}
}

func writeJSON(buf *bytes.Buffer, v any) {
	b, _ := json.Marshal(v)
	buf.Write(b)
	buf.WriteByte('\n')
}

func parseResponse(t *testing.T, line string) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nline: %s", err, line)
	}
	return resp
}

func assertResult(t *testing.T, resp map[string]any, label string) map[string]any {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("%s: expected result object, got %v", label, resp)
	}
	return result
}
