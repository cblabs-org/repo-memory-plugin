package antigravity_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/packaging/antigravity"
)

func TestGenerateHooksJSON(t *testing.T) {
	binPath := "/usr/local/bin/repo-memory"
	content := antigravity.GenerateHooksJSON(binPath)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("GenerateHooksJSON produced invalid JSON: %v\nContent:\n%s", err, content)
	}

	pluginConfig, ok := parsed["repo-memory"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'repo-memory' key in hooks.json")
	}

	events := []string{"PreInvocation", "PreToolUse", "PostToolUse", "Stop"}
	for _, event := range events {
		if _, exists := pluginConfig[event]; !exists {
			t.Errorf("expected event %s in hooks.json", event)
		}
	}

	if !strings.Contains(content, "/usr/local/bin/repo-memory hook antigravity PreInvocation") {
		t.Errorf("expected PreInvocation command with binary path, got:\n%s", content)
	}
	if !strings.Contains(content, "/usr/local/bin/repo-memory hook antigravity PreToolUse") {
		t.Errorf("expected PreToolUse command with binary path, got:\n%s", content)
	}
	if !strings.Contains(content, "/usr/local/bin/repo-memory hook antigravity PostToolUse") {
		t.Errorf("expected PostToolUse command with binary path, got:\n%s", content)
	}
	if !strings.Contains(content, "/usr/local/bin/repo-memory hook antigravity Stop") {
		t.Errorf("expected Stop command with binary path, got:\n%s", content)
	}

	// Verify matchers
	preToolUseList, ok := pluginConfig["PreToolUse"].([]interface{})
	if !ok || len(preToolUseList) == 0 {
		t.Fatalf("invalid PreToolUse structure")
	}
	preToolUseItem := preToolUseList[0].(map[string]interface{})
	matcher := preToolUseItem["matcher"].(string)
	if matcher != "run_command|write_to_file|replace_file_content|call_mcp_tool" {
		t.Errorf("unexpected PreToolUse matcher: %s", matcher)
	}

	postToolUseList, ok := pluginConfig["PostToolUse"].([]interface{})
	if !ok || len(postToolUseList) == 0 {
		t.Fatalf("invalid PostToolUse structure")
	}
	postToolUseItem := postToolUseList[0].(map[string]interface{})
	postMatcher := postToolUseItem["matcher"].(string)
	if postMatcher != "run_command" {
		t.Errorf("unexpected PostToolUse matcher: %s", postMatcher)
	}
}

func TestGenerateMCPConfigJSON(t *testing.T) {
	binPath := "/opt/repo-memory/bin/repo-memory"
	content := antigravity.GenerateMCPConfigJSON(binPath)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("GenerateMCPConfigJSON produced invalid JSON: %v", err)
	}

	servers, ok := parsed["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mcpServers object")
	}

	server, ok := servers["repo-memory"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected repo-memory server config")
	}

	cmd := server["command"].(string)
	if cmd != binPath {
		t.Errorf("expected command %s, got %s", binPath, cmd)
	}

	args, ok := server["args"].([]interface{})
	if !ok || len(args) != 1 || args[0].(string) != "mcp" {
		t.Errorf("expected args ['mcp'], got %v", server["args"])
	}
}

func TestGenerateSidecarJSON(t *testing.T) {
	binPath := "/opt/repo-memory/bin/repo-memory"
	content := antigravity.GenerateSidecarJSON(binPath)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("GenerateSidecarJSON produced invalid JSON: %v", err)
	}

	if parsed["builtin"] != "schedule" {
		t.Errorf("expected builtin: 'schedule', got %v", parsed["builtin"])
	}

	args, ok := parsed["args"].([]interface{})
	if !ok || len(args) < 3 {
		t.Fatalf("expected at least 3 args in sidecar.json, got %v", parsed["args"])
	}
	if args[0] != "* * * * *" {
		t.Errorf("expected cron '* * * * *', got %v", args[0])
	}
	if args[1] != binPath {
		t.Errorf("expected binary %s, got %v", binPath, args[1])
	}
	if args[2] != "tick" {
		t.Errorf("expected subcommand 'tick', got %v", args[2])
	}

	if parsed["restart_policy"] != "always" {
		t.Errorf("expected restart_policy 'always', got %v", parsed["restart_policy"])
	}
	if parsed["display_name"] != "Repo Memory Tick" {
		t.Errorf("expected display_name 'Repo Memory Tick', got %v", parsed["display_name"])
	}

	permissions, ok := parsed["agent_permissions"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected agent_permissions object")
	}
	grants, ok := permissions["access_grants"].([]interface{})
	if !ok || len(grants) == 0 {
		t.Errorf("expected access_grants in agent_permissions")
	}
	if _, ok := permissions["workspace_uris"]; !ok {
		t.Errorf("expected workspace_uris in agent_permissions")
	}
}

func TestGeneratePluginJSON(t *testing.T) {
	content := antigravity.GeneratePluginJSON()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("GeneratePluginJSON produced invalid JSON: %v", err)
	}

	if parsed["name"] != "repo-memory" {
		t.Errorf("expected name 'repo-memory', got %v", parsed["name"])
	}
}

func TestWindowsPathsEscaping(t *testing.T) {
	binPath := `C:\Program Files\Repo Memory\repo-memory.exe`
	hooks := antigravity.GenerateHooksJSON(binPath)
	mcp := antigravity.GenerateMCPConfigJSON(binPath)
	sidecar := antigravity.GenerateSidecarJSON(binPath)

	var hMap, mMap, sMap map[string]interface{}
	if err := json.Unmarshal([]byte(hooks), &hMap); err != nil {
		t.Fatalf("Windows path produced invalid hooks JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(mcp), &mMap); err != nil {
		t.Fatalf("Windows path produced invalid mcp JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(sidecar), &sMap); err != nil {
		t.Fatalf("Windows path produced invalid sidecar JSON: %v", err)
	}
}
