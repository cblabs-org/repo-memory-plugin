/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
)

func TestCopyTranscripts(t *testing.T) {
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	dataDir := filepath.Join(tempDir, "data")
	repoRoot := filepath.Join(tempDir, "my-repo")
	repoID := "repo-claude-123"

	// Mock ~/.claude/projects/
	projName := projectDirNameForRoot(repoRoot)
	projDir := filepath.Join(homeDir, ".claude", "projects", projName)
	subagentsDir := filepath.Join(projDir, "session-abc", "subagents")
	_ = os.MkdirAll(subagentsDir, 0755)

	// Create a top-level transcript and a subagent transcript
	topTranscript := filepath.Join(projDir, "session-abc.jsonl")
	_ = os.WriteFile(topTranscript, []byte("{\"role\":\"user\",\"content\":\"hello claude\"}\n"), 0644)

	subTranscript := filepath.Join(subagentsDir, "sub-123.jsonl")
	_ = os.WriteFile(subTranscript, []byte("{\"role\":\"assistant\",\"content\":\"subagent work\"}\n"), 0644)

	// Also create a memory file that should be skipped
	memoryDir := filepath.Join(projDir, "memory")
	_ = os.MkdirAll(memoryDir, 0755)
	_ = os.WriteFile(filepath.Join(memoryDir, "MEMORY.md"), []byte("auto memory"), 0644)

	enrolledRepos := map[string]config.RegistryEntry{
		repoRoot: {
			RepoID:   repoID,
			RootPath: repoRoot,
		},
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	ctx := context.Background()
	if err := CopyTranscripts(ctx, dataDir, enrolledRepos); err != nil {
		t.Fatalf("CopyTranscripts failed: %v", err)
	}

	// Verify top-level transcript copied to raw/claude-code/
	dstTop := filepath.Join(dataDir, repoID, "raw", "claude-code", "session-abc.jsonl")
	if data, err := os.ReadFile(dstTop); err != nil {
		t.Errorf("failed to read copied top-level transcript: %v", err)
	} else if string(data) != "{\"role\":\"user\",\"content\":\"hello claude\"}\n" {
		t.Errorf("unexpected top-level transcript content: %q", string(data))
	}

	// Verify subagent transcript copied to raw/claude-code/session-abc/subagents/
	dstSub := filepath.Join(dataDir, repoID, "raw", "claude-code", "session-abc", "subagents", "sub-123.jsonl")
	if data, err := os.ReadFile(dstSub); err != nil {
		t.Errorf("failed to read copied subagent transcript: %v", err)
	} else if string(data) != "{\"role\":\"assistant\",\"content\":\"subagent work\"}\n" {
		t.Errorf("unexpected subagent transcript content: %q", string(data))
	}

	// Verify memory file was NOT copied
	dstMemory := filepath.Join(dataDir, repoID, "raw", "claude-code", "memory", "MEMORY.md")
	if _, err := os.Stat(dstMemory); !os.IsNotExist(err) {
		t.Errorf("expected memory file NOT to be copied into raw/claude-code/")
	}
}
