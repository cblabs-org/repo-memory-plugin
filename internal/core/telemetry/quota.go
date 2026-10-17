/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/telemetry/quota.go
 * Description: Queries and parses model pool quota remaining fractions via the
 *              Antigravity CLI (`agy -p "/usage" --output-format json`).
 *
 * Exported Types / Functions:
 * - QuotaBucket:                  Represents an individual model or quota bucket.
 * - QuotaGroup:                   Represents a logical group of quota buckets.
 * - QuotaStatus:                  Represents the parsed quota state returned by agy.
 *   - GetBucket():                Returns a quota bucket by its identifier.
 *   - RemainingFraction():        Returns the remaining quota fraction for a bucket ID.
 * - SetLookPathForTesting():      Overrides exec.LookPath in unit tests and returns a restore func.
 * - SetExecCommandForTesting():   Overrides exec.CommandContext in unit tests and returns a restore func.
 * - CheckQuota():                 Executes `agy -p "/usage"` and parses the resulting QuotaStatus.
 * - ParseQuotaOutput():           Parses raw JSON output from `agy -p "/usage"` into a QuotaStatus.
 */

// Package telemetry provides quota inspection and IDE token usage metadata parsing.
package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

var (
	lookPath    = exec.LookPath       // Injectable binary path lookup function for testing.
	execCommand = exec.CommandContext // Injectable subprocess constructor for testing.
)

// SetLookPathForTesting allows tests to override lookPath and returns a restore function.
func SetLookPathForTesting(fn func(string) (string, error)) func() {
	prev := lookPath
	lookPath = fn
	return func() { lookPath = prev }
}

// SetExecCommandForTesting allows tests to override execCommand and returns a restore function.
func SetExecCommandForTesting(fn func(context.Context, string, ...string) *exec.Cmd) func() {
	prev := execCommand
	execCommand = fn
	return func() { execCommand = prev }
}

// QuotaBucket represents an individual model or quota bucket.
type QuotaBucket struct {
	ID                string     `json:"id"`                    // Unique identifier of the quota bucket.
	Name              string     `json:"name"`                  // Human-readable display name of the bucket.
	Description       string     `json:"description,omitempty"` // Optional description of the bucket pool.
	RemainingFraction float64    `json:"remaining_fraction"`    // Remaining quota fraction in the range [0.0, 1.0].
	ResetTime         *time.Time `json:"reset_time,omitempty"`  // Optional UTC timestamp when the quota bucket resets.
}

// QuotaGroup represents a logical group of quota buckets (e.g., "All Models").
type QuotaGroup struct {
	Name        string        `json:"name"`                  // Display name of the quota group.
	Description string        `json:"description,omitempty"` // Optional summary of the group.
	Buckets     []QuotaBucket `json:"buckets"`               // Quota buckets belonging to this group.
}

// QuotaStatus represents the parsed quota state returned by agy.
type QuotaStatus struct {
	Status      string        `json:"status"`                // Top-level command status string (e.g. "SUCCESS").
	Description string        `json:"description,omitempty"` // Summary description from the usage command payload.
	Groups      []QuotaGroup  `json:"groups"`                // Grouped quota buckets returned by agy.
	Buckets     []QuotaBucket `json:"buckets,omitempty"`     // Flattened list of all quota buckets across groups.
}

// GetBucket returns a bucket by its identifier.
func (q *QuotaStatus) GetBucket(id string) (QuotaBucket, bool) {
	for _, b := range q.Buckets {
		if b.ID == id {
			return b, true
		}
	}
	return QuotaBucket{}, false
}

// RemainingFraction returns the remaining fraction for a bucket by ID.
func (q *QuotaStatus) RemainingFraction(id string) (float64, bool) {
	b, ok := q.GetBucket(id)
	if !ok {
		return 0, false
	}
	return b.RemainingFraction, true
}

// rawAgyResponse captures the JSON output from `agy -p "/usage" --output-format json`.
type rawAgyResponse struct {
	ConversationID string `json:"conversation_id"` // Ephemeral conversation ID reported by agy.
	Status         string `json:"status"`          // Execution status string ("SUCCESS" on success).
	Response       string `json:"response"`        // Raw text or error message from agy.
	Command        struct {
		Name string `json:"name"` // Executed slash command name ("/usage").
		Data struct {
			Description string `json:"description"` // Human-readable usage summary.
			Groups      []struct {
				Name        string `json:"name"`        // Quota group name.
				Description string `json:"description"` // Quota group description.
				Buckets     []struct {
					ID                string  `json:"id"`                 // Bucket identifier.
					Name              string  `json:"name"`               // Bucket display name.
					Description       string  `json:"description"`        // Bucket description.
					RemainingFraction float64 `json:"remaining_fraction"` // Remaining quota fraction in [0.0, 1.0].
					ResetTime         string  `json:"reset_time"`         // RFC3339 timestamp string when quota resets.
				} `json:"buckets"` // Raw bucket entries within the group.
			} `json:"groups"` // Raw quota groups returned by the command.
		} `json:"data"` // Structured command output data.
	} `json:"command"` // Nested slash-command payload.
}

// CheckQuota queries `agy -p "/usage" --output-format json` (zero tokens)
// and returns the current quota state across model pools.
// If agy is not in PATH, it returns an error.
func CheckQuota(ctx context.Context) (*QuotaStatus, error) {
	agyPath, err := lookPath("agy")
	if err != nil {
		return nil, fmt.Errorf("agy not found in PATH: %w", err)
	}

	cmd := execCommand(ctx, agyPath, "-p", "/usage", "--output-format", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("executing agy usage: %w", err)
	}

	return ParseQuotaOutput(out)
}

// ParseQuotaOutput parses the raw JSON output from agy usage into a QuotaStatus.
func ParseQuotaOutput(data []byte) (*QuotaStatus, error) {
	// --- Step 1: Unmarshal raw JSON response and verify command status ---
	var raw rawAgyResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decoding agy usage response: %w", err)
	}

	if raw.Status != "" && raw.Status != "SUCCESS" {
		return nil, fmt.Errorf("agy returned non-success status %q: %s", raw.Status, raw.Response)
	}

	// --- Step 2: Convert raw groups and buckets into QuotaStatus ---
	qs := &QuotaStatus{
		Status:      raw.Status,
		Description: raw.Command.Data.Description,
		Groups:      make([]QuotaGroup, 0, len(raw.Command.Data.Groups)),
		Buckets:     make([]QuotaBucket, 0),
	}

	for _, g := range raw.Command.Data.Groups {
		group := QuotaGroup{
			Name:        g.Name,
			Description: g.Description,
			Buckets:     make([]QuotaBucket, 0, len(g.Buckets)),
		}

		for _, b := range g.Buckets {
			bucket := QuotaBucket{
				ID:                b.ID,
				Name:              b.Name,
				Description:       b.Description,
				RemainingFraction: b.RemainingFraction,
			}

			if b.ResetTime != "" {
				if t, err := time.Parse(time.RFC3339, b.ResetTime); err == nil {
					bucket.ResetTime = &t
				} else if t, err := time.Parse(time.RFC3339Nano, b.ResetTime); err == nil {
					bucket.ResetTime = &t
				}
			}

			group.Buckets = append(group.Buckets, bucket)
			qs.Buckets = append(qs.Buckets, bucket)
		}

		qs.Groups = append(qs.Groups, group)
	}

	return qs, nil
}
