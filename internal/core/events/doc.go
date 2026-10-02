// Package events provides storage and schemas for claim reviews and answers.
//
// Replay Order (for PR-10):
// When replaying events to reconstruct state, the events MUST be replayed
// in chronological order based on their 'at' timestamps. If timestamps are
// identical, they should be replayed in the order they appear in the jsonl
// file (append order). Review events and Answer events should be interleaved
// chronologically if a combined timeline is needed.
package events
