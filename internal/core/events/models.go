package events

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// ReviewEvent represents a review action on a claim.
type ReviewEvent struct {
	ClaimID string    `json:"claim_id"`
	State   string    `json:"state"`
	By      string    `json:"by"`
	At      time.Time `json:"at"`
	Reason  string    `json:"reason,omitempty"`
	Hash    string    `json:"hash"`
}

// ComputeHash computes the SHA256 hash of the event contents (excluding the hash itself).
func (e *ReviewEvent) ComputeHash() string {
	clone := *e
	clone.Hash = ""
	data, _ := json.Marshal(clone)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// AnswerEvent represents an answer provided for a claim.
type AnswerEvent struct {
	ClaimID string    `json:"claim_id"`
	By      string    `json:"by"`
	At      time.Time `json:"at"`
	Content string    `json:"content"`
	Hash    string    `json:"hash"`
}

// ComputeHash computes the SHA256 hash of the event contents (excluding the hash itself).
func (e *AnswerEvent) ComputeHash() string {
	clone := *e
	clone.Hash = ""
	data, _ := json.Marshal(clone)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
