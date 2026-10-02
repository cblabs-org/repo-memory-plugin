package antigravity

import (
	"path/filepath"
	"testing"
)

func TestParseTranscript_FullNormal(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "fixtures", "antigravity", "v1", "full_normal")
	steps, err := ParseTranscript(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps (including checkpoint), got %d", len(steps))
	}
	if steps[0].StepIndex != 0 {
		t.Errorf("expected step 0, got %d", steps[0].StepIndex)
	}
	if !steps[1].IsCheckpoint {
		t.Error("expected step 1 to be checkpoint")
	}
	if steps[2].StepIndex != 1 {
		t.Errorf("expected step 1, got %d", steps[2].StepIndex)
	}
}

func TestParseTranscript_FullChunked(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "fixtures", "antigravity", "v1", "full_chunked")
	steps, err := ParseTranscript(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps (including gap), got %d", len(steps))
	}
	if steps[0].Content != "hello" {
		t.Errorf("expected content 'hello', got '%s'", steps[0].Content)
	}
	if !steps[1].IsGap {
		t.Error("expected step 1 to be gap")
	}
	if steps[1].GapSize != 2 {
		t.Errorf("expected gap size 2, got %d", steps[1].GapSize)
	}
	if steps[2].StepIndex != 3 {
		t.Errorf("expected step index 3, got %d", steps[2].StepIndex)
	}
}

func TestParseTranscript_Compact(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "fixtures", "antigravity", "v1", "compact")
	steps, err := ParseTranscript(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if len(steps[1].TruncatedFields) != 1 || steps[1].TruncatedFields[0] != "content" {
		t.Error("expected truncated field 'content'")
	}
}

func TestParseTranscript_Canary(t *testing.T) {
	// A canary test to ensure parsing behavior doesn't change unexpectedly across versions
	// if we introduce a v2 in the future.
	dir1 := filepath.Join("..", "..", "..", "testdata", "fixtures", "antigravity", "v1", "full_normal")
	steps1, err := ParseTranscript(dir1)
	if err != nil {
		t.Fatalf("v1 error: %v", err)
	}
	
	// If a v2 fixture exists, we expect it to parse similarly
	dir2 := filepath.Join("..", "..", "..", "testdata", "fixtures", "antigravity", "v2", "full_normal")
	steps2, err := ParseTranscript(dir2)
	if err == nil {
		if len(steps2) < len(steps1) {
			t.Errorf("canary: v2 parsed fewer steps than v1: %d vs %d", len(steps2), len(steps1))
		}
	}
}
