package publish

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
)

var (
	ErrSensitiveContent = errors.New("claim contains sensitive content")
	ErrUntrustedModel   = errors.New("claim from untrusted model")
	ErrUnverifiedClaim  = errors.New("claim is unverified")
)

type Indexer interface {
	IndexClaim(ctx context.Context, claim *claims.Claim) error
}

type LocalPublisher struct {
	dataDir string
	repoID  string
	indexer Indexer
}

func NewLocalPublisher(dataDir, repoID string, indexer Indexer) *LocalPublisher {
	return &LocalPublisher{
		dataDir: dataDir,
		repoID:  repoID,
		indexer: indexer,
	}
}

func (p *LocalPublisher) Publish(ctx context.Context, claim *claims.Claim) error {
	// Security gating: check for sensitive content
	if isSensitive(claim.Title) || isSensitive(claim.Body) {
		return ErrSensitiveContent
	}

	// Security gating: block unverified claims
	if claim.Status != claims.StatusVerified && claim.Status != claims.StatusMerged && claim.Status != claims.StatusConfirmed {
		if claim.VerifiedAt == "" {
			return ErrUnverifiedClaim
		}
	}

	// External provenance gating
	if claim.Provenance != nil {
		if strings.Contains(strings.ToLower(claim.Provenance.ExtractorModel), "untrusted") || 
		   strings.Contains(strings.ToLower(claim.Provenance.ReconcilerModel), "untrusted") {
			return ErrUntrustedModel
		}
	}

	// Mark as unreviewed
	if claim.Review == nil {
		claim.Review = &claims.Review{}
	}
	claim.Review.State = "unreviewed"
	claim.Review.By = "system"
	claim.Review.At = time.Now().Format(time.RFC3339)

	// Ensure pending directory exists
	pendingDir := filepath.Join(p.dataDir, p.repoID, "pending")
	if err := os.MkdirAll(pendingDir, 0755); err != nil {
		return fmt.Errorf("creating pending directory: %w", err)
	}

	// Write claim
	data, err := claims.Write(claim)
	if err != nil {
		return fmt.Errorf("formatting claim: %w", err)
	}

	filePath := filepath.Join(pendingDir, claim.ID+".md")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("writing claim: %w", err)
	}

	// Immediate indexing
	if p.indexer != nil {
		if err := p.indexer.IndexClaim(ctx, claim); err != nil {
			return fmt.Errorf("indexing claim: %w", err)
		}
	}

	return nil
}

func isSensitive(text string) bool {
	lower := strings.ToLower(text)
	sensitiveWords := []string{"password", "secret", "token", "api_key", "private_key"}
	for _, word := range sensitiveWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}
