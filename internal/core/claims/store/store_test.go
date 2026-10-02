package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims/store"
)

func TestStore_CRUD(t *testing.T) {
	tempDir := t.TempDir()

	s, err := store.New(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	claim := &claims.Claim{
		ID:    "test-123",
		Type:  claims.TypeFact,
		Title: "Test Claim",
		Body:  "This is a test claim body",
		Relations: &claims.Relations{
			Supports: []string{"target-1"},
		},
		Created: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}

	// Create
	if err := s.Create(claim); err != nil {
		t.Fatalf("failed to create claim: %v", err)
	}

	// Check path
	expectedPath := filepath.Join(tempDir, "2026-10", "test-123.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("expected file to be created at %s", expectedPath)
	}

	// Read
	readClaim, err := s.Read("test-123")
	if err != nil {
		t.Fatalf("failed to read claim: %v", err)
	}
	if readClaim.Title != claim.Title {
		t.Fatalf("expected title %q, got %q", claim.Title, readClaim.Title)
	}
	if readClaim.Body != claim.Body {
		t.Fatalf("expected body %q, got %q", claim.Body, readClaim.Body)
	}

	// Inverse Relations
	inv := s.GetInverseRelations("target-1")
	if len(inv.SupportedBy) != 1 || inv.SupportedBy[0] != "test-123" {
		t.Fatalf("expected SupportedBy to contain test-123, got %v", inv.SupportedBy)
	}

	// Update
	claim.Title = "Updated Claim"
	claim.Relations.Supports = []string{"target-2"}
	if err := s.Update(claim); err != nil {
		t.Fatalf("failed to update claim: %v", err)
	}

	readClaim, err = s.Read("test-123")
	if err != nil {
		t.Fatalf("failed to read updated claim: %v", err)
	}
	if readClaim.Title != "Updated Claim" {
		t.Fatalf("expected title to be updated, got %q", readClaim.Title)
	}

	// Inverse Relations after update
	invOld := s.GetInverseRelations("target-1")
	if len(invOld.SupportedBy) != 0 {
		t.Fatalf("expected SupportedBy for target-1 to be empty, got %v", invOld.SupportedBy)
	}
	invNew := s.GetInverseRelations("target-2")
	if len(invNew.SupportedBy) != 1 || invNew.SupportedBy[0] != "test-123" {
		t.Fatalf("expected SupportedBy for target-2 to contain test-123, got %v", invNew.SupportedBy)
	}

	// Delete
	if err := s.Delete("test-123"); err != nil {
		t.Fatalf("failed to delete claim: %v", err)
	}

	if _, err := s.Read("test-123"); err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound after deletion, got %v", err)
	}
}
