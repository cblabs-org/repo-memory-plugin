package antigravity

import (
	"encoding/json"
	"fmt"
	"os"
)

// InjectStep represents a single step to inject via PreInvocation/PostInvocation/SessionStart.
type InjectStep struct {
	EphemeralMessage string              `json:"ephemeralMessage,omitempty"`
	UserMessage      string              `json:"userMessage,omitempty"`
	SystemMessage    *SystemMessageEntry `json:"systemMessage,omitempty"`
}

// SystemMessageEntry is the nested object required for systemMessage injection.
// Unlike ephemeralMessage/userMessage (plain strings), systemMessage MUST be a nested object.
type SystemMessageEntry struct {
	SystemMessage string            `json:"systemMessage"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// InjectStepsResponse is the JSON shape returned by PreInvocation/PostInvocation/SessionStart hooks.
type InjectStepsResponse struct {
	InjectSteps []InjectStep `json:"injectSteps,omitempty"`
}

// V2SpikePreInvocation returns a PreInvocation response that injects test messages
// via the injectSteps mechanism. Behind the REPO_MEMORY_V2_SPIKE flag (kill switch: =0).
//
// Message types tested:
//   - ephemeralMessage: plain string, visible for one invocation only
//   - userMessage: plain string, persists across turns until context compaction
//   - systemMessage: nested object, persists across turns until context compaction
func V2SpikePreInvocation(payload []byte) ([]byte, error) {
	if os.Getenv("REPO_MEMORY_V2_SPIKE") == "0" {
		return []byte("{}"), nil
	}

	var input struct {
		ConversationID string `json:"conversationId"`
		InvocationNum  int    `json:"invocationNum"`
	}
	if err := json.Unmarshal(payload, &input); err != nil {
		return []byte("{}"), nil
	}

	// Only inject on first invocation (invocationNum == 0) to avoid duplicates.
	// systemMessage and userMessage persist — re-injecting accumulates duplicates.
	if input.InvocationNum > 0 {
		return []byte("{}"), nil
	}

	marker := fmt.Sprintf("[X5-CHANNEL-TEST conv=%s]", input.ConversationID)

	response := InjectStepsResponse{
		InjectSteps: []InjectStep{
			{
				// Test A: systemMessage — nested object with metadata
				SystemMessage: &SystemMessageEntry{
					SystemMessage: fmt.Sprintf("%s This is a test system message injected by the repo-memory PreInvocation hook. If you can see this, the systemMessage channel is working.", marker),
					Metadata: map[string]string{
						"source": "repo-memory-x5-spike",
					},
				},
			},
			{
				// Test B: ephemeralMessage — plain string, single invocation scope
				EphemeralMessage: fmt.Sprintf("%s This ephemeral message should only be visible on the first invocation.", marker),
			},
		},
	}

	return json.Marshal(response)
}
