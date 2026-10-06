package antigravity

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestV2SpikePreInvocation(t *testing.T) {
	os.Setenv("REPO_MEMORY_V2_SPIKE", "1")
	defer os.Unsetenv("REPO_MEMORY_V2_SPIKE")

	t.Run("invocation 0 returns injectSteps with systemMessage and ephemeralMessage", func(t *testing.T) {
		os.Setenv("REPO_MEMORY_V2_SPIKE", "1")
		out, err := V2SpikePreInvocation([]byte(`{"conversationId": "test-123", "invocationNum": 0}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var resp InjectStepsResponse
		if err := json.Unmarshal(out, &resp); err != nil {
			t.Fatalf("failed to parse response: %v\nraw: %s", err, string(out))
		}

		if len(resp.InjectSteps) != 2 {
			t.Fatalf("expected 2 injectSteps, got %d", len(resp.InjectSteps))
		}

		// Step 0: systemMessage (nested object)
		step0 := resp.InjectSteps[0]
		if step0.SystemMessage == nil {
			t.Fatal("step 0 should have systemMessage")
		}
		if !strings.Contains(step0.SystemMessage.SystemMessage, "X5-CHANNEL-TEST conv=test-123") {
			t.Errorf("systemMessage missing marker: %s", step0.SystemMessage.SystemMessage)
		}
		if step0.SystemMessage.Metadata["source"] != "repo-memory-x5-spike" {
			t.Errorf("systemMessage metadata source: %v", step0.SystemMessage.Metadata)
		}

		// Step 1: ephemeralMessage (plain string)
		step1 := resp.InjectSteps[1]
		if step1.EphemeralMessage == "" {
			t.Fatal("step 1 should have ephemeralMessage")
		}
		if !strings.Contains(step1.EphemeralMessage, "X5-CHANNEL-TEST conv=test-123") {
			t.Errorf("ephemeralMessage missing marker: %s", step1.EphemeralMessage)
		}
	})

	t.Run("invocation > 0 returns empty", func(t *testing.T) {
		os.Setenv("REPO_MEMORY_V2_SPIKE", "1")
		out, err := V2SpikePreInvocation([]byte(`{"conversationId": "test-123", "invocationNum": 1}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(out) != "{}" {
			t.Errorf("expected {}, got %s", string(out))
		}
	})

	t.Run("disabled by env flag", func(t *testing.T) {
		os.Setenv("REPO_MEMORY_V2_SPIKE", "0")
		out, err := V2SpikePreInvocation([]byte(`{"conversationId": "test-123", "invocationNum": 0}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(out) != "{}" {
			t.Errorf("expected {}, got %s", string(out))
		}
	})

	t.Run("malformed payload returns empty (graceful)", func(t *testing.T) {
		os.Setenv("REPO_MEMORY_V2_SPIKE", "1")
		out, err := V2SpikePreInvocation([]byte(`malformed`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Malformed payload fails json.Unmarshal → returns {}
		if string(out) != "{}" {
			t.Errorf("expected {} for malformed payload, got %s", string(out))
		}
	})
}
