/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/pipeline/publish/local.go
 * Description: Enforces security, verification, and provenance gates before
 *               publishing verified memory items to the local pending directory
 *               and triggering immediate indexing.
 *
 * Exported Types / Functions:
 * - ErrSensitiveContent: Sentinel error when an item contains sensitive keywords or secrets.
 * - ErrUntrustedModel:   Sentinel error when an item originates from an untrusted model.
 * - ErrUnverifiedItem:   Sentinel error when an item has not been verified or confirmed.
 * - Provenance:          Records the extractor and reconciler models that produced an item.
 * - Review:              Records the review state, actor, and timestamp for a published item.
 * - Item:                Represents a candidate memory item ready for publication.
 * - Indexer:             Interface for indexing a newly published Item.
 * - LocalPublisher:      Writes verified items to <data-dir>/<repo-id>/pending and indexes them.
 *   - NewLocalPublisher(): Creates a new LocalPublisher instance.
 *   - Publish():           Validates, writes, and indexes a single Item.
 */

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

// Sentinel errors returned by LocalPublisher.Publish when security or verification gates fail.
var (
	ErrSensitiveContent = errors.New("item contains sensitive content") // Returned when title or body contains secret-related keywords.
	ErrUntrustedModel   = errors.New("item from untrusted model")       // Returned when extractor or reconciler provenance is untrusted.
	ErrUnverifiedItem   = errors.New("item is unverified")              // Returned when an item lacks a verified status and timestamp.
)

// Provenance records the model identifiers used during extraction and reconciliation.
type Provenance struct {
	ExtractorModel  string `json:"extractor_model,omitempty"`  // Model identifier that extracted the item.
	ReconcilerModel string `json:"reconciler_model,omitempty"` // Model identifier that reconciled the item.
}

// Review tracks the human or system review state of a published item.
type Review struct {
	State string `json:"state,omitempty"` // Review state (e.g. "unreviewed", "approved").
	By    string `json:"by,omitempty"`    // Actor that set the review state.
	At    string `json:"at,omitempty"`    // RFC3339 timestamp when the review state was recorded.
}

// Item represents a memory claim candidate submitted for local publication.
type Item struct {
	ID         string            `json:"id"`                    // Unique identifier for the item.
	Title      string            `json:"title"`                 // Short summary title of the item.
	Body       string            `json:"body"`                  // Full Markdown body of the item.
	Status     string            `json:"status"`                // Verification status (e.g. "verified", "merged", "confirmed").
	VerifiedAt string            `json:"verified_at,omitempty"` // Optional RFC3339 timestamp of verification.
	Provenance *Provenance       `json:"provenance,omitempty"`  // Optional model provenance metadata.
	Review     *Review           `json:"review,omitempty"`      // Optional review state metadata.
	Metadata   map[string]string `json:"metadata,omitempty"`    // Arbitrary key-value metadata attributes.
}

// Indexer indexes a published Item into the repository search index.
type Indexer interface {
	IndexItem(ctx context.Context, item *Item) error
}

// LocalPublisher validates and writes verified items to the local pending directory.
type LocalPublisher struct {
	dataDir string  // Base data directory path.
	repoID  string  // Repository slug determining the <dataDir>/<repoID>/pending staging path.
	indexer Indexer // Optional Indexer invoked immediately after writing an item.
}

// NewLocalPublisher creates a LocalPublisher targeting <dataDir>/<repoID>/pending.
func NewLocalPublisher(dataDir, repoID string, indexer Indexer) *LocalPublisher {
	return &LocalPublisher{
		dataDir: dataDir,
		repoID:  repoID,
		indexer: indexer,
	}
}

// Publish enforces security, verification, and provenance gates on item, writes it to the pending
// Markdown directory, and triggers immediate indexing if an Indexer is configured.
// It returns ErrSensitiveContent, ErrUnverifiedItem, or ErrUntrustedModel when gating fails.
func (p *LocalPublisher) Publish(ctx context.Context, item *Item) error {
	// --- Step 1: Enforce sensitive-content, verification, and provenance security gates ---
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

	// --- Step 2: Stamp initial unreviewed metadata and write the Markdown file ---
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

	// --- Step 3: Trigger immediate indexing if an Indexer is configured ---
	// Immediate indexing
	if p.indexer != nil {
		if err := p.indexer.IndexItem(ctx, item); err != nil {
			return fmt.Errorf("indexing item: %w", err)
		}
	}

	return nil
}

// isSensitive reports whether text contains any case-insensitive credential or secret keywords.
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
