package jobs_test

import (
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
)

func TestStateTransitions(t *testing.T) {
	tests := []struct {
		from  jobs.State
		to    jobs.State
		valid bool
	}{
		// From Queued
		{jobs.StateQueued, jobs.StateRunning, true},
		{jobs.StateQueued, jobs.StateQueued, true},
		{jobs.StateQueued, jobs.StateDone, false},
		{jobs.StateQueued, jobs.StateFailed, false},
		{jobs.StateQueued, jobs.StateDead, false},

		// From Running
		{jobs.StateRunning, jobs.StateDone, true},
		{jobs.StateRunning, jobs.StateFailed, true},
		{jobs.StateRunning, jobs.StateDead, true},
		{jobs.StateRunning, jobs.StateQueued, true}, // lease reclamation
		{jobs.StateRunning, jobs.StateSubmitted, true}, // report submitted

		// From Submitted (reports)
		{jobs.StateSubmitted, jobs.StateDone, true},
		{jobs.StateSubmitted, jobs.StateFailed, true},
		{jobs.StateSubmitted, jobs.StateDead, true},
		{jobs.StateSubmitted, jobs.StateRunning, false},

		// From Failed
		{jobs.StateFailed, jobs.StateRunning, true}, // retry
		{jobs.StateFailed, jobs.StateQueued, true},  // re-queue
		{jobs.StateFailed, jobs.StateDead, true},    // exceeded max attempts
		{jobs.StateFailed, jobs.StateDone, false},

		// From Dead
		{jobs.StateDead, jobs.StateQueued, true}, // operator re-queue
		{jobs.StateDead, jobs.StateRunning, false},
		{jobs.StateDead, jobs.StateDone, false},

		// From Done (terminal)
		{jobs.StateDone, jobs.StateQueued, false},
		{jobs.StateDone, jobs.StateRunning, false},
		{jobs.StateDone, jobs.StateFailed, false},
		{jobs.StateDone, jobs.StateDead, false},
		{jobs.StateDone, jobs.StateDone, true},

		// Unknown / invalid states
		{jobs.State("unknown"), jobs.StateQueued, false},
		{jobs.StateQueued, jobs.State("unknown"), false},
	}

	for _, tc := range tests {
		err := jobs.ValidateTransition(tc.from, tc.to)
		can := jobs.CanTransition(tc.from, tc.to)
		if tc.valid {
			if err != nil {
				t.Errorf("expected valid transition from %s to %s, got error: %v", tc.from, tc.to, err)
			}
			if !can {
				t.Errorf("expected CanTransition(%s, %s) == true", tc.from, tc.to)
			}
		} else {
			if err == nil {
				t.Errorf("expected invalid transition from %s to %s, got nil error", tc.from, tc.to)
			}
			if can {
				t.Errorf("expected CanTransition(%s, %s) == false", tc.from, tc.to)
			}
		}
	}
}

func TestStateHelpers(t *testing.T) {
	validStates := []jobs.State{
		jobs.StateQueued,
		jobs.StateRunning,
		jobs.StateDone,
		jobs.StateFailed,
		jobs.StateDead,
		jobs.StateSubmitted,
	}

	for _, s := range validStates {
		if !s.Valid() {
			t.Errorf("expected state %s to be valid", s)
		}
	}

	if jobs.State("invalid_state").Valid() {
		t.Error("expected invalid_state to be invalid")
	}

	if !jobs.StateDone.IsTerminal() {
		t.Error("expected StateDone to be terminal")
	}
	if !jobs.StateDead.IsTerminal() {
		t.Error("expected StateDead to be terminal")
	}
	if jobs.StateQueued.IsTerminal() {
		t.Error("expected StateQueued not to be terminal")
	}
	if jobs.StateRunning.IsTerminal() {
		t.Error("expected StateRunning not to be terminal")
	}
}

func TestDefaultBackoff(t *testing.T) {
	b0 := jobs.DefaultBackoff(0)
	if b0 != 5*time.Second {
		t.Errorf("expected attempt 0 to have 5s backoff, got %v", b0)
	}

	b1 := jobs.DefaultBackoff(1)
	if b1 != 5*time.Second {
		t.Errorf("expected attempt 1 to have 5s backoff, got %v", b1)
	}

	b2 := jobs.DefaultBackoff(2)
	if b2 != 15*time.Second {
		t.Errorf("expected attempt 2 to have 15s backoff, got %v", b2)
	}

	b3 := jobs.DefaultBackoff(3)
	if b3 != 45*time.Second {
		t.Errorf("expected attempt 3 to have 45s backoff, got %v", b3)
	}

	// High attempt should cap at 15m
	b10 := jobs.DefaultBackoff(10)
	if b10 > 15*time.Minute {
		t.Errorf("expected attempt 10 to be capped at 15m, got %v", b10)
	}
}
