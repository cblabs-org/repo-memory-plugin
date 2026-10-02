package events

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrNotFound      = errors.New("event not found")
	ErrAlreadyExists = errors.New("event already exists")
)

type Store struct {
	baseDir string
	mu      sync.RWMutex
}

func NewStore(baseDir string) *Store {
	return &Store{
		baseDir: baseDir,
	}
}

// WriteReview appends a ReviewEvent to .memory/events/reviews/{claimID}.jsonl.
func (s *Store) WriteReview(e ReviewEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e.Hash = e.ComputeHash()

	path := filepath.Join(s.baseDir, "reviews", e.ClaimID+".jsonl")

	// Check if already exists to ensure it's keyed by content hash
	existing, err := s.readReviewsUnsafe(path)
	if err == nil {
		for _, ex := range existing {
			if ex.Hash == e.Hash {
				return ErrAlreadyExists
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(e)
	if err != nil {
		return err
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}

	return nil
}

// ReadReviews reads all ReviewEvents for a given claim ID.
func (s *Store) ReadReviews(claimID string) ([]ReviewEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.baseDir, "reviews", claimID+".jsonl")
	return s.readReviewsUnsafe(path)
}

func (s *Store) readReviewsUnsafe(path string) ([]ReviewEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var events []ReviewEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e ReviewEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		events = append(events, e)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

// WriteAnswer appends an AnswerEvent to .memory/events/answers/{claimID}.jsonl.
func (s *Store) WriteAnswer(e AnswerEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e.Hash = e.ComputeHash()

	path := filepath.Join(s.baseDir, "answers", e.ClaimID+".jsonl")

	existing, err := s.readAnswersUnsafe(path)
	if err == nil {
		for _, ex := range existing {
			if ex.Hash == e.Hash {
				return ErrAlreadyExists
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(e)
	if err != nil {
		return err
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}

	return nil
}

// ReadAnswers reads all AnswerEvents for a given claim ID.
func (s *Store) ReadAnswers(claimID string) ([]AnswerEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.baseDir, "answers", claimID+".jsonl")
	return s.readAnswersUnsafe(path)
}

func (s *Store) readAnswersUnsafe(path string) ([]AnswerEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var events []AnswerEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e AnswerEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		events = append(events, e)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

// GetReviewByHash returns a specific review event by its hash.
func (s *Store) GetReviewByHash(claimID, hash string) (*ReviewEvent, error) {
	events, err := s.ReadReviews(claimID)
	if err != nil {
		return nil, err
	}
	for _, e := range events {
		if e.Hash == hash {
			return &e, nil
		}
	}
	return nil, ErrNotFound
}

// GetAnswerByHash returns a specific answer event by its hash.
func (s *Store) GetAnswerByHash(claimID, hash string) (*AnswerEvent, error) {
	events, err := s.ReadAnswers(claimID)
	if err != nil {
		return nil, err
	}
	for _, e := range events {
		if e.Hash == hash {
			return &e, nil
		}
	}
	return nil, ErrNotFound
}
