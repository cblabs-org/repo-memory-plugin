/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package jobs_test

import (
	"encoding/json"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestCostFromUsageRoundTrip(t *testing.T) {
	usage := harness.Usage{
		InputTokens:     1250,
		OutputTokens:    340,
		ThinkingTokens:  180,
		CacheReadTokens: 4200,
	}

	cost := jobs.FromUsage(usage)

	if cost.InputTokens != usage.InputTokens {
		t.Errorf("expected InputTokens %d, got %d", usage.InputTokens, cost.InputTokens)
	}
	if cost.OutputTokens != usage.OutputTokens {
		t.Errorf("expected OutputTokens %d, got %d", usage.OutputTokens, cost.OutputTokens)
	}
	if cost.ThinkingTokens != usage.ThinkingTokens {
		t.Errorf("expected ThinkingTokens %d, got %d", usage.ThinkingTokens, cost.ThinkingTokens)
	}
	if cost.CacheReadTokens != usage.CacheReadTokens {
		t.Errorf("expected CacheReadTokens %d, got %d", usage.CacheReadTokens, cost.CacheReadTokens)
	}

	// Round-trip back to harness.Usage
	roundTripUsage := cost.ToUsage()
	if roundTripUsage != usage {
		t.Errorf("round trip mismatch: got %+v, want %+v", roundTripUsage, usage)
	}

	// Verify JSON marshaling / unmarshaling preserves all fields
	b, err := json.Marshal(cost)
	if err != nil {
		t.Fatalf("marshaling cost: %v", err)
	}

	var decoded jobs.Cost
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshaling cost: %v", err)
	}

	if decoded != cost {
		t.Errorf("JSON round-trip mismatch: got %+v, want %+v", decoded, cost)
	}

	// Verify SQLite Value and Scan
	val, err := cost.Value()
	if err != nil {
		t.Fatalf("cost.Value failed: %v", err)
	}

	var scanned jobs.Cost
	if err := scanned.Scan(val); err != nil {
		t.Fatalf("cost.Scan failed: %v", err)
	}

	if scanned != cost {
		t.Errorf("Scan round-trip mismatch: got %+v, want %+v", scanned, cost)
	}
}
