package events_test

import (
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/events"
)

func TestEventsStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := events.NewStore(tmpDir)

	now := time.Now().Truncate(time.Second) // truncate to avoid issues with json marshal/unmarshal precision

	// Test Review
	review := events.ReviewEvent{
		ClaimID: "claim-1",
		State:   "approved",
		By:      "user-1",
		At:      now,
		Reason:  "looks good",
	}

	err := store.WriteReview(review)
	if err != nil {
		t.Fatalf("WriteReview failed: %v", err)
	}

	reviews, err := store.ReadReviews("claim-1")
	if err != nil {
		t.Fatalf("ReadReviews failed: %v", err)
	}

	if len(reviews) != 1 {
		t.Fatalf("Expected 1 review, got %d", len(reviews))
	}

	if reviews[0].State != "approved" {
		t.Errorf("Expected state approved, got %s", reviews[0].State)
	}
	
	// Test Hash uniqueness
	err = store.WriteReview(review)
	if err != events.ErrAlreadyExists {
		t.Errorf("Expected ErrAlreadyExists, got %v", err)
	}
	
	hash := reviews[0].Hash
	if hash == "" {
		t.Errorf("Expected non-empty hash")
	}

	r, err := store.GetReviewByHash("claim-1", hash)
	if err != nil {
		t.Fatalf("GetReviewByHash failed: %v", err)
	}
	if r.Hash != hash {
		t.Errorf("Expected hash %s, got %s", hash, r.Hash)
	}

	// Test Answer
	answer := events.AnswerEvent{
		ClaimID: "claim-2",
		By:      "user-2",
		At:      now,
		Content: "42",
	}

	err = store.WriteAnswer(answer)
	if err != nil {
		t.Fatalf("WriteAnswer failed: %v", err)
	}

	answers, err := store.ReadAnswers("claim-2")
	if err != nil {
		t.Fatalf("ReadAnswers failed: %v", err)
	}

	if len(answers) != 1 {
		t.Fatalf("Expected 1 answer, got %d", len(answers))
	}

	if answers[0].Content != "42" {
		t.Errorf("Expected content 42, got %s", answers[0].Content)
	}

	// Test Hash uniqueness for Answer
	err = store.WriteAnswer(answer)
	if err != events.ErrAlreadyExists {
		t.Errorf("Expected ErrAlreadyExists, got %v", err)
	}

	aHash := answers[0].Hash
	if aHash == "" {
		t.Errorf("Expected non-empty hash")
	}

	a, err := store.GetAnswerByHash("claim-2", aHash)
	if err != nil {
		t.Fatalf("GetAnswerByHash failed: %v", err)
	}
	if a.Hash != aHash {
		t.Errorf("Expected hash %s, got %s", aHash, a.Hash)
	}
}
