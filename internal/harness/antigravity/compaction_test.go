/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package antigravity

import (
	"context"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

func TestDetectCompaction(t *testing.T) {
	ledger := delivery.NewLedger("test-conv")
	ledger.RecordInjection(delivery.InjectionRecord{StepIndex: 5, EntryID: "ent1"})
	ledger.RecordInjection(delivery.InjectionRecord{StepIndex: 10, EntryID: "ent2"})

	t.Run("No checkpoint", func(t *testing.T) {
		steps := []Step{
			{StepIndex: 1}, {StepIndex: 2}, {StepIndex: 15},
		}
		res := DetectCompaction(steps, ledger)
		if res.Detected {
			t.Errorf("Expected false, got true")
		}
	})

	t.Run("Checkpoint present, no prior injections", func(t *testing.T) {
		steps := []Step{
			{IsCheckpoint: true},
			{StepIndex: 2},
		}
		res := DetectCompaction(steps, ledger)
		if res.Detected {
			t.Errorf("Expected false, got true")
		}
	})

	t.Run("Checkpoint present, prior injections lost", func(t *testing.T) {
		steps := []Step{
			{IsCheckpoint: true},
			{StepIndex: 12}, // after checkpoint, we resumed at step 12. So 5 and 10 are lost.
		}
		res := DetectCompaction(steps, ledger)
		if !res.Detected {
			t.Errorf("Expected true, got false")
		}
		if len(res.LostInjections) != 2 {
			t.Errorf("Expected 2 lost, got %d", len(res.LostInjections))
		}
	})

	t.Run("Multiple checkpoints", func(t *testing.T) {
		steps := []Step{
			{IsCheckpoint: true},
			{StepIndex: 6}, // if this was the last checkpoint, ent2 (10) would be kept, ent1 (5) lost.
			{IsCheckpoint: true},
			{StepIndex: 12}, // but this is the last one. both 5 and 10 are lost.
		}
		res := DetectCompaction(steps, ledger)
		if !res.Detected {
			t.Errorf("Expected true, got false")
		}
		if len(res.LostInjections) != 2 {
			t.Errorf("Expected 2 lost, got %d", len(res.LostInjections))
		}
	})
}

func TestDetectRewind(t *testing.T) {
	steps := []Step{{}, {}}
	if !DetectRewind(steps, 5) {
		t.Errorf("Expected true, got false")
	}
	if DetectRewind(steps, 2) {
		t.Errorf("Expected false, got true")
	}
}

func TestBuildReinjection(t *testing.T) {
	idx, _ := index.NewIndex(":memory:")
	defer idx.Close()

	ent := dossiers.Entry{ID: "ent1", Text: "Some test data", Type: dossiers.EntryTypeInvariant}
	idx.UpsertEntry(context.Background(), ent)

	lost := []delivery.InjectionRecord{
		{EntryID: "ent1"},
	}

	step, err := BuildReinjection(context.Background(), idx, lost, "", 1000)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if step == nil {
		t.Fatalf("Expected step, got nil")
	}
	if step.SystemMessage == nil || !strings.Contains(step.SystemMessage.SystemMessage, "Some test data") {
		t.Errorf("Expected test data in systemMessage, got %+v", step.SystemMessage)
	}
	if step.SystemMessage != nil && len(step.SystemMessage.Metadata) > 0 {
		t.Errorf("Expected empty metadata on systemMessage, got %v", step.SystemMessage.Metadata)
	}
	if step.EphemeralMessage != "" {
		t.Errorf("Expected empty ephemeralMessage, got %s", step.EphemeralMessage)
	}

	// tag survived
	step2, err := BuildReinjection(context.Background(), idx, lost, "[repo-memory ent1]", 1000)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if step2 != nil {
		t.Errorf("Expected nil, got step")
	}
}
