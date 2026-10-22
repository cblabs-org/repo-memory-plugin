/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package spool

import (
	"encoding/json"
	"testing"
)

func TestEntry_JSONSerialization(t *testing.T) {
	entry := NewCaptureEntry("antigravity", "conv-123", "/path/to/transcript.jsonl", "/repo", "main", "abc1234")
	entry.ID = "test-id-1"

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal entry: %v", err)
	}

	var parsed Entry
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal entry: %v", err)
	}

	if parsed.ID != "test-id-1" {
		t.Errorf("expected ID 'test-id-1', got %s", parsed.ID)
	}
	if parsed.Kind != KindCapture {
		t.Errorf("expected kind %s, got %s", KindCapture, parsed.Kind)
	}
	if parsed.ConversationID != "conv-123" {
		t.Errorf("expected conv-123, got %s", parsed.ConversationID)
	}
	if parsed.TranscriptPath != "/path/to/transcript.jsonl" {
		t.Errorf("expected /path/to/transcript.jsonl, got %s", parsed.TranscriptPath)
	}
	if parsed.RepoRoot != "/repo" {
		t.Errorf("expected /repo, got %s", parsed.RepoRoot)
	}
	if parsed.Branch != "main" {
		t.Errorf("expected main, got %s", parsed.Branch)
	}
	if parsed.Head != "abc1234" {
		t.Errorf("expected abc1234, got %s", parsed.Head)
	}
}

func TestEntry_DefaultKind(t *testing.T) {
	// Raw JSON without explicit kind field
	raw := `{"conversation_id":"c1","transcript_path":"/t.jsonl"}`
	var parsed Entry
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if parsed.Kind != KindCapture {
		t.Errorf("expected default kind %s, got %s", KindCapture, parsed.Kind)
	}
}

func TestConstructors(t *testing.T) {
	b := NewBriefEntry("antigravity", "c2", "/repo", 4, "planning refactor")
	if b.Kind != KindBrief || b.StepIndex != 4 || b.Reasoning != "planning refactor" {
		t.Errorf("unexpected brief entry: %+v", b)
	}
	if b.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}

	d := NewDejavuEntry("antigravity", "c3", "ent-456", "re-trying ruled-out approach", 10)
	if d.Kind != KindDejavu || d.EntityID != "ent-456" || d.StepIndex != 10 {
		t.Errorf("unexpected dejavu entry: %+v", d)
	}

	r := NewRecheckEntry("antigravity", "c4", "/repo", "feat", "def5678", "git pull", 2)
	if r.Kind != KindRecheck || r.Command != "git pull" || r.Branch != "feat" {
		t.Errorf("unexpected recheck entry: %+v", r)
	}
}
