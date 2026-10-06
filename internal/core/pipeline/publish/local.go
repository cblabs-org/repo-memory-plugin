package publish

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrSensitiveContent = errors.New("item contains sensitive content")
	ErrUntrustedModel   = errors.New("item from untrusted model")
	ErrUnverifiedItem   = errors.New("item is unverified")
)

type Provenance struct {
	ExtractorModel  string `json:"extractor_model,omitempty"`
	ReconcilerModel string `json:"reconciler_model,omitempty"`
}

type Review struct {
	State string `json:"state,omitempty"`
	By    string `json:"by,omitempty"`
	At    string `json:"at,omitempty"`
}

type Item struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	Status     string            `json:"status"`
	VerifiedAt string            `json:"verified_at,omitempty"`
	Provenance *Provenance       `json:"provenance,omitempty"`
	Review     *Review           `json:"review,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type Indexer interface {
	IndexItem(ctx context.Context, item *Item) error
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

func (p *LocalPublisher) Publish(ctx context.Context, item *Item) error {
	// Security gating: check for sensitive content
	if isSensitive(item.Title) || isSensitive(item.Body) {
		return ErrSensitiveContent
	}

	// Security gating: block unverified items
	if item.Status != "verified" && item.Status != "merged" && item.Status != "confirmed" {
		if item.VerifiedAt == "" {
			return ErrUnverifiedItem
		}
	}

	// External provenance gating
	if item.Provenance != nil {
		if strings.Contains(strings.ToLower(item.Provenance.ExtractorModel), "untrusted") ||
			strings.Contains(strings.ToLower(item.Provenance.ReconcilerModel), "untrusted") {
			return ErrUntrustedModel
		}
	}

	// Mark as unreviewed
	if item.Review == nil {
		item.Review = &Review{}
	}
	item.Review.State = "unreviewed"
	item.Review.By = "system"
	item.Review.At = time.Now().Format(time.RFC3339)

	// Ensure pending directory exists
	pendingDir := filepath.Join(p.dataDir, p.repoID, "pending")
	if err := os.MkdirAll(pendingDir, 0755); err != nil {
		return fmt.Errorf("creating pending directory: %w", err)
	}

	// Write item
	filePath := filepath.Join(pendingDir, item.ID+".md")
	content := fmt.Sprintf("# %s\n\n%s\n", item.Title, item.Body)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing item: %w", err)
	}

	// Immediate indexing
	if p.indexer != nil {
		if err := p.indexer.IndexItem(ctx, item); err != nil {
			return fmt.Errorf("indexing item: %w", err)
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
