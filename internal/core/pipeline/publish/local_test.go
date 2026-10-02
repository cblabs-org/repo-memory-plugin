package publish

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
)

type mockIndexer struct {
	indexed []*claims.Claim
}

func (m *mockIndexer) IndexClaim(ctx context.Context, claim *claims.Claim) error {
	m.indexed = append(m.indexed, claim)
	return nil
}

func TestLocalPublisher_Publish(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-publisher-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	idx := &mockIndexer{}
	pub := NewLocalPublisher(tempDir, "repo-123", idx)

	t.Run("success", func(t *testing.T) {
		claim := &claims.Claim{
			ID:         "claim-1",
			Title:      "Normal Claim",
			Body:       "This is a normal claim",
			Status:     claims.StatusVerified,
			VerifiedAt: "commit-123",
		}

		err := pub.Publish(context.Background(), claim)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}

		// check file
		pendingDir := filepath.Join(tempDir, "repo-123", "pending")
		filePath := filepath.Join(pendingDir, "claim-1.md")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Fatalf("expected file to exist at %s", filePath)
		}

		// check review status
		if claim.Review == nil || claim.Review.State != "unreviewed" {
			t.Errorf("expected review state unreviewed, got %v", claim.Review)
		}

		// check index
		if len(idx.indexed) != 1 {
			t.Errorf("expected 1 indexed claim, got %d", len(idx.indexed))
		}
	})

	t.Run("sensitive", func(t *testing.T) {
		claim := &claims.Claim{
			ID:    "claim-2",
			Title: "I have a password here",
			Body:  "secret",
		}
		err := pub.Publish(context.Background(), claim)
		if err != ErrSensitiveContent {
			t.Errorf("expected ErrSensitiveContent, got %v", err)
		}
	})
}
