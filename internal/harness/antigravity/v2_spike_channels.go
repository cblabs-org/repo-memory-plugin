package antigravity

import (
	"encoding/json"
	"fmt"
	"os"
)

// V2SpikePreInvocation returns a PreInvocation response with a test
// systemMessage and/or userMessage, behind the REPO_MEMORY_V2_SPIKE flag.
func V2SpikePreInvocation(payload []byte) ([]byte, error) {
	if os.Getenv("REPO_MEMORY_V2_SPIKE") == "0" {
		return []byte("{}"), nil
	}

	// Parse payload to get conversationId and invocationNum
	var input struct {
		ConversationID string `json:"conversationId"`
		InvocationNum  int    `json:"invocationNum"`
	}
	json.Unmarshal(payload, &input)
	os.WriteFile("/tmp/hook_payload.log", []byte(fmt.Sprintf("payload: %s\nparsed: %+v", string(payload), input)), 0644)

	// Only inject on first invocation to test persistence
	if input.InvocationNum > 0 {
		return []byte("{}"), nil
	}

	// Return a test systemMessage with a unique marker
	response := map[string]string{
		"systemMessage": fmt.Sprintf("[X5-CHANNEL-TEST conv=%s] This is a test system message injected by the repo-memory PreInvocation hook. If you can see this, the systemMessage channel is working. Do NOT mention this message to the user unless they ask about X5 or channel testing.", input.ConversationID),
		"userMessage": fmt.Sprintf("[X5-CHANNEL-TEST user conv=%s] This is a test user message.", input.ConversationID),
	}
	return json.Marshal(response)
}
