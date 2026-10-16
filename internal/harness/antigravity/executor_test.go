package antigravity_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/antigravity"
)

func TestHousekeeperRun_StageDistill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script mock not supported on windows")
	}

	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	capturedArgsFile := filepath.Join(tmpDir, "args.txt")
	capturedEnvFile := filepath.Join(tmpDir, "env.txt")

	fakeAgy := filepath.Join(binDir, "agy")
	script := `#!/bin/sh
printf "%s\n" "$@" > "` + capturedArgsFile + `"
printf "HK=%s\nINT=%s\n" "$REPO_MEMORY_HOUSEKEEPING" "$REPO_MEMORY_INTERNAL" > "` + capturedEnvFile + `"
cat << 'EOF'
{"event":"result","result":{"status":"SUCCESS","response":"` + "```repo-memory-note\\n- type: gotcha\\n  title: Keep curly braces in {code}\\n  why: Reason\\n  do: Action\\n  anchors: [a.go]\\n  step_citations: [1]\\n```\\necho_token: abcdef0123456789" + `","usage":{"input_tokens":120,"output_tokens":45}}}
EOF
`
	if err := os.WriteFile(fakeAgy, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	hk := antigravity.NewHousekeeper(tmpDir, "repo-1", "flash", "1m")
	inputContainer := "<session-log-abcdef0123456789>\n{\"i\":1,\"actor\":\"user\",\"kind\":\"user\",\"text\":\"hi\"}\n</session-log-abcdef0123456789>"
	res, err := hk.Run(context.Background(), harness.StageDistill, inputContainer)
	if err != nil {
		t.Fatalf("hk.Run(StageDistill) failed: %v", err)
	}

	if !strings.Contains(res.Payload, "```repo-memory-note") || !strings.Contains(res.Payload, "echo_token: abcdef0123456789") {
		t.Errorf("expected full YAML + echo_token preserved in Payload, got:\n%s", res.Payload)
	}
	if res.Usage.InputTokens != 120 || res.Usage.OutputTokens != 45 {
		t.Errorf("unexpected usage: %+v", res.Usage)
	}

	argsBytes, err := os.ReadFile(capturedArgsFile)
	if err != nil {
		t.Fatal(err)
	}
	argsStr := string(argsBytes)
	if !strings.Contains(argsStr, "--agent\nrepo-memory-distiller") {
		t.Errorf("expected --agent repo-memory-distiller in args, got:\n%s", argsStr)
	}
	if !strings.Contains(argsStr, "<session-log-abcdef0123456789>") {
		t.Errorf("expected raw session-log container in prompt arg without JSON double-escaping, got:\n%s", argsStr)
	}

	envBytes, err := os.ReadFile(capturedEnvFile)
	if err != nil {
		t.Fatal(err)
	}
	envStr := string(envBytes)
	if !strings.Contains(envStr, "HK=1") || !strings.Contains(envStr, "INT=1") {
		t.Errorf("expected REPO_MEMORY_HOUSEKEEPING=1 and REPO_MEMORY_INTERNAL=1, got:\n%s", envStr)
	}
}

func TestHousekeeperRunMultiTurn_StageConsolidate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script mock not supported on windows")
	}

	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	capturedArgsFile := filepath.Join(tmpDir, "args.txt")
	capturedStdinFile := filepath.Join(tmpDir, "stdin.txt")

	fakeAgy := filepath.Join(binDir, "agy")
	script := `#!/bin/sh
printf "%s\n" "$@" > "` + capturedArgsFile + `"
cat > "` + capturedStdinFile + `"
cat << 'EOF'
{"event":"result","result":{"status":"SUCCESS","response":"{\"ops\":[],\"echo_token\":\"tok1\"}","usage":{"input_tokens":1000,"output_tokens":50,"thinking_tokens":10,"cache_read_tokens":0}}}
{"event":"result","result":{"status":"SUCCESS","response":"{\"ops\":[],\"echo_token\":\"tok2\"}","usage":{"input_tokens":1300,"output_tokens":90,"thinking_tokens":25,"cache_read_tokens":900}}}
EOF
`
	if err := os.WriteFile(fakeAgy, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	hk := antigravity.NewHousekeeper(tmpDir, "repo-1", "flash", "1m")
	prompts := []string{
		"<consolidation-batch-tok1>\n{\"scope\":\"mod/a\"}\n</consolidation-batch-tok1>",
		"<consolidation-batch-tok2>\n{\"scope\":\"mod/b\"}\n</consolidation-batch-tok2>",
	}

	results, err := hk.RunMultiTurn(context.Background(), harness.StageConsolidate, prompts)
	if err != nil {
		t.Fatalf("hk.RunMultiTurn(StageConsolidate) failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Payload != `{"ops":[],"echo_token":"tok1"}` {
		t.Errorf("unexpected turn 0 payload: %s", results[0].Payload)
	}
	if results[0].Usage.InputTokens != 1000 || results[0].Usage.OutputTokens != 50 || results[0].Usage.ThinkingTokens != 10 {
		t.Errorf("unexpected turn 0 usage: %+v", results[0].Usage)
	}

	if results[1].Payload != `{"ops":[],"echo_token":"tok2"}` {
		t.Errorf("unexpected turn 1 payload: %s", results[1].Payload)
	}
	// Cumulative delta check: 1300 - 1000 = 300 input, 90 - 50 = 40 output, 25 - 10 = 15 thinking, 900 - 0 = 900 cache read
	if results[1].Usage.InputTokens != 300 || results[1].Usage.OutputTokens != 40 || results[1].Usage.ThinkingTokens != 15 || results[1].Usage.CacheReadTokens != 900 {
		t.Errorf("unexpected turn 1 delta usage: %+v", results[1].Usage)
	}

	argsBytes, err := os.ReadFile(capturedArgsFile)
	if err != nil {
		t.Fatal(err)
	}
	argsStr := string(argsBytes)
	for _, wantArg := range []string{
		"--input-format\nstream-json",
		"--output-format\nstream-json",
		"--agent\nrepo-memory-distiller",
	} {
		if !strings.Contains(argsStr, wantArg) {
			t.Errorf("expected %q in args, got:\n%s", wantArg, argsStr)
		}
	}

	stdinBytes, err := os.ReadFile(capturedStdinFile)
	if err != nil {
		t.Fatal(err)
	}
	stdinLines := strings.Split(strings.TrimSpace(string(stdinBytes)), "\n")
	if len(stdinLines) != 2 {
		t.Fatalf("expected 2 NDJSON lines on stdin, got %d:\n%s", len(stdinLines), string(stdinBytes))
	}
	for i, line := range stdinLines {
		if !strings.Contains(line, `"event":"user"`) || !strings.Contains(line, `"message":{"content":`) {
			t.Errorf("stdin line %d missing stream-json user event shape: %s", i, line)
		}
	}
}

