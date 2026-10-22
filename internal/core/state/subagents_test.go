/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package state_test

import (
	"context"
	"os"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
)

func TestSubagentRegistry(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	repoID := "repo-subagents"

	parentID := "11111111-1111-1111-1111-111111111111"
	child1 := "22222222-2222-2222-2222-222222222222"
	child2 := "33333333-3333-3333-3333-333333333333"

	// Fast return when DB file does not exist yet (and must not create the DB file)
	isSub, gotParent, err := state.IsSubagent(ctx, dataDir, repoID, child1)
	if err != nil {
		t.Fatalf("IsSubagent on missing DB failed: %v", err)
	}
	if isSub || gotParent != "" {
		t.Errorf("expected false, '', got %v, %q", isSub, gotParent)
	}
	if _, statErr := os.Stat(state.SubagentDBPath(dataDir, repoID)); !os.IsNotExist(statErr) {
		t.Errorf("expected IsSubagent not to create sqlite file when absent")
	}

	// Record two child subagents
	if err := state.RecordSubagent(ctx, dataDir, repoID, child1, parentID, "Worker 1"); err != nil {
		t.Fatalf("RecordSubagent child1 failed: %v", err)
	}
	if err := state.RecordSubagent(ctx, dataDir, repoID, child2, parentID, "Code Reviewer"); err != nil {
		t.Fatalf("RecordSubagent child2 failed: %v", err)
	}

	// Verify child1 is recognized as subagent
	isSub, gotParent, err = state.IsSubagent(ctx, dataDir, repoID, child1)
	if err != nil {
		t.Fatalf("IsSubagent child1 failed: %v", err)
	}
	if !isSub || gotParent != parentID {
		t.Errorf("expected child1 to be subagent of %s, got isSub=%v parent=%q", parentID, isSub, gotParent)
	}

	// Verify parentID itself is not a subagent
	isParentSub, _, err := state.IsSubagent(ctx, dataDir, repoID, parentID)
	if err != nil {
		t.Fatalf("IsSubagent parentID failed: %v", err)
	}
	if isParentSub {
		t.Error("expected parentID not to be a subagent")
	}

	// Verify ListSubagentChildren returns both children
	children, err := state.ListSubagentChildren(ctx, dataDir, repoID, parentID)
	if err != nil {
		t.Fatalf("ListSubagentChildren failed: %v", err)
	}
	if len(children) != 2 || children[0] != child1 || children[1] != child2 {
		t.Errorf("expected children [%s, %s], got %v", child1, child2, children)
	}
}
