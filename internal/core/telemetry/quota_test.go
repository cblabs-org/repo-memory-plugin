/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package telemetry_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/telemetry"
)

func TestCheckQuota_AgyNotInPATH(t *testing.T) {
	restore := telemetry.SetLookPathForTesting(func(file string) (string, error) {
		return "", exec.ErrNotFound
	})
	defer restore()

	ctx := context.Background()
	status, err := telemetry.CheckQuota(ctx)
	if err == nil {
		t.Fatalf("expected error when agy not in PATH, got nil (status: %+v)", status)
	}

	if !strings.Contains(err.Error(), "agy not found in PATH") {
		t.Errorf("expected 'agy not found in PATH' in error, got %v", err)
	}
}

func TestParseQuotaOutput_Valid(t *testing.T) {
	rawJSON := `{
  "conversation_id": "",
  "status": "SUCCESS",
  "response": "All Models\tGemini 3.8 Flash (High)\t60%\t2026-10-04T17:52:28Z\n",
  "duration_seconds": 0,
  "num_turns": 0,
  "usage": {
    "input_tokens": 0,
    "output_tokens": 0,
    "thinking_tokens": 0,
    "cache_read_tokens": 0,
    "total_tokens": 0
  },
  "command": {
    "name": "usage",
    "data": {
      "description": "View your available model quota.",
      "groups": [
        {
          "name": "All Models",
          "description": "Quota pools",
          "buckets": [
            {
              "id": "gemini-3.8-flash-high",
              "name": "Gemini 3.8 Flash (High)",
              "description": "Quota resets in 4 hours, 8 minutes.",
              "remaining_fraction": 0.602,
              "reset_time": "2026-10-04T17:52:28Z"
            },
            {
              "id": "gemini-pro-agent",
              "name": "Gemini 3.1 Pro (High)",
              "description": "Quota resets in 5 hours.",
              "remaining_fraction": 1.0,
              "reset_time": "2026-10-04T18:43:28Z"
            }
          ]
        }
      ]
    }
  }
}`

	qs, err := telemetry.ParseQuotaOutput([]byte(rawJSON))
	if err != nil {
		t.Fatalf("ParseQuotaOutput failed: %v", err)
	}

	if qs.Status != "SUCCESS" {
		t.Errorf("status: got %s, want SUCCESS", qs.Status)
	}
	if len(qs.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(qs.Groups))
	}
	if len(qs.Buckets) != 2 {
		t.Fatalf("expected 2 flattened buckets, got %d", len(qs.Buckets))
	}

	b, ok := qs.GetBucket("gemini-3.8-flash-high")
	if !ok {
		t.Fatal("expected bucket gemini-3.8-flash-high to be found")
	}
	if b.Name != "Gemini 3.8 Flash (High)" {
		t.Errorf("bucket name: got %s, want Gemini 3.8 Flash (High)", b.Name)
	}
	if b.RemainingFraction != 0.602 {
		t.Errorf("bucket fraction: got %f, want 0.602", b.RemainingFraction)
	}
	if b.ResetTime == nil {
		t.Fatal("expected non-nil ResetTime")
	}
	expectedTime, _ := time.Parse(time.RFC3339, "2026-10-04T17:52:28Z")
	if !b.ResetTime.Equal(expectedTime) {
		t.Errorf("reset time: got %v, want %v", b.ResetTime, expectedTime)
	}

	frac, ok := qs.RemainingFraction("gemini-pro-agent")
	if !ok || frac != 1.0 {
		t.Errorf("RemainingFraction: got %f, ok=%v, want 1.0, true", frac, ok)
	}

	_, ok = qs.RemainingFraction("non-existent")
	if ok {
		t.Error("expected non-existent bucket to return false")
	}
}

func TestParseQuotaOutput_NonSuccess(t *testing.T) {
	rawJSON := `{
  "status": "ERROR",
  "response": "Authentication failed"
}`

	_, err := telemetry.ParseQuotaOutput([]byte(rawJSON))
	if err == nil {
		t.Fatal("expected error for non-success status, got nil")
	}
	if !strings.Contains(err.Error(), "Authentication failed") {
		t.Errorf("expected error message to contain reason, got %v", err)
	}
}

func TestParseQuotaOutput_Malformed(t *testing.T) {
	_, err := telemetry.ParseQuotaOutput([]byte("not-json"))
	if err == nil {
		t.Fatal("expected error for malformed json, got nil")
	}
}

func TestCheckQuota_LiveIfAgyPresent(t *testing.T) {
	// If agy is actually present in PATH on the host running the test,
	// verify that CheckQuota runs cleanly without error.
	if _, err := exec.LookPath("agy"); err != nil {
		t.Skip("agy not in PATH, skipping live execution test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	qs, err := telemetry.CheckQuota(ctx)
	if err != nil {
		// agy might fail if no auth, but if it runs, let's log
		t.Logf("live CheckQuota returned: %v", err)
		return
	}

	if qs.Status != "SUCCESS" {
		t.Errorf("unexpected status: %s", qs.Status)
	}
	t.Logf("live CheckQuota found %d buckets in %d groups", len(qs.Buckets), len(qs.Groups))
}
