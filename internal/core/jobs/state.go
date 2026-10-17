/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/jobs/state.go
 * Description: Implements the job lifecycle state machine transition validator
 *               and exponential retry backoff calculator.
 *
 * Exported Types / Functions:
 * - ValidateTransition(): Validates whether a state transition from -> to is legal.
 * - CanTransition():      Reports whether transitioning from -> to is permitted.
 * - DefaultBackoff():     Computes exponential retry backoff duration capped at 15 minutes.
 */

package jobs

import (
	"fmt"
	"math"
	"time"
)

// ValidateTransition checks if transitioning from one state to another is permitted.
// It returns ErrInvalidTransition (wrapped) when the source, target, or transition edge is disallowed.
// State machine according to SPEC §4.3:
// queued -> running -> done, or failed -> retried with backoff -> dead after N attempts.
// Reports: submitted(conversation_id) -> done when conversation goes idle.
// Recovery: running -> queued (lease reclaim).
func ValidateTransition(from, to State) error {
	// --- Step 1: Validate that both source and target states are known enum values ---
	if !from.Valid() {
		return fmt.Errorf("%w: unknown source state %q", ErrInvalidTransition, from)
	}
	if !to.Valid() {
		return fmt.Errorf("%w: unknown target state %q", ErrInvalidTransition, to)
	}
	if from == to {
		return nil
	}

	// --- Step 2: Check permitted state-machine edges ---
	switch from {
	case StateQueued:
		// A queued job can be leased (running).
		if to == StateRunning {
			return nil
		}
	case StateRunning:
		// A running job can succeed (done), fail transiently (failed),
		// fail permanently (dead), be reclaimed (queued), or transition to submitted (reports).
		if to == StateDone || to == StateFailed || to == StateDead || to == StateQueued || to == StateSubmitted {
			return nil
		}
	case StateSubmitted:
		// A report waiting in IDE can succeed (done) or fail (failed/dead).
		if to == StateDone || to == StateFailed || to == StateDead {
			return nil
		}
	case StateFailed:
		// A failed job can be picked up for retry (running), re-queued, or marked permanently dead.
		if to == StateRunning || to == StateQueued || to == StateDead {
			return nil
		}
	case StateDead:
		// A dead job can be manually re-queued for retry by operator.
		if to == StateQueued {
			return nil
		}
	case StateDone:
		// Terminal state: done jobs cannot transition further.
		return fmt.Errorf("%w: cannot transition from terminal state %q to %q", ErrInvalidTransition, from, to)
	}

	return fmt.Errorf("%w: transition from %q to %q is not allowed", ErrInvalidTransition, from, to)
}

// CanTransition returns true if the transition from -> to is valid.
func CanTransition(from, to State) bool {
	return ValidateTransition(from, to) == nil
}

// DefaultBackoff calculates backoff duration for failed attempts with exponential progression.
// attempt 1: 5s
// attempt 2: 15s
// attempt 3: 45s
// attempt 4: ~2m15s
// capped at 15m.
func DefaultBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 5 * time.Second
	}
	// base 5s * 3^(attempt-1)
	factor := math.Pow(3, float64(attempt-1))
	backoffSec := 5.0 * factor
	maxSec := 15.0 * 60.0 // 15 minutes
	if backoffSec > maxSec {
		backoffSec = maxSec
	}
	return time.Duration(backoffSec) * time.Second
}
