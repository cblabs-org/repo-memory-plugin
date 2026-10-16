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
