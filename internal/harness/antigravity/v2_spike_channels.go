package antigravity

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

// Removed: replaced by HandlePreInvocation in preinvocation.go
