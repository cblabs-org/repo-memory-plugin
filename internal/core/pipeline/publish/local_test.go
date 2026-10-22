/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package publish

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type mockIndexer struct {
	indexed []*Item
}

func (m *mockIndexer) IndexItem(ctx context.Context, item *Item) error {
	m.indexed = append(m.indexed, item)
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
		item := &Item{
			ID:         "item-1",
			Title:      "Normal Item",
			Body:       "This is a normal item",
			Status:     "verified",
			VerifiedAt: "commit-123",
		}

		err := pub.Publish(context.Background(), item)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}

		// check file
		pendingDir := filepath.Join(tempDir, "repo-123", "pending")
		filePath := filepath.Join(pendingDir, "item-1.md")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Fatalf("expected file to exist at %s", filePath)
		}

		// check review status
		if item.Review == nil || item.Review.State != "unreviewed" {
			t.Errorf("expected review state unreviewed, got %v", item.Review)
		}

		// check index
		if len(idx.indexed) != 1 {
			t.Errorf("expected 1 indexed item, got %d", len(idx.indexed))
		}
	})

	t.Run("sensitive", func(t *testing.T) {
		item := &Item{
			ID:    "item-2",
			Title: "I have a password here",
			Body:  "secret",
		}
		err := pub.Publish(context.Background(), item)
		if err != ErrSensitiveContent {
			t.Fatalf("expected ErrSensitiveContent, got %v", err)
		}
	})

	t.Run("unverified", func(t *testing.T) {
		item := &Item{
			ID:     "item-3",
			Title:  "Unverified item",
			Status: "observed",
		}
		err := pub.Publish(context.Background(), item)
		if err != ErrUnverifiedItem {
			t.Fatalf("expected ErrUnverifiedItem, got %v", err)
		}
	})

	t.Run("untrusted model", func(t *testing.T) {
		item := &Item{
			ID:         "item-4",
			Title:      "Untrusted item",
			Status:     "verified",
			VerifiedAt: "commit-123",
			Provenance: &Provenance{
				ExtractorModel: "untrusted-v1",
			},
		}
		err := pub.Publish(context.Background(), item)
		if err != ErrUntrustedModel {
			t.Fatalf("expected ErrUntrustedModel, got %v", err)
		}
	})
}
