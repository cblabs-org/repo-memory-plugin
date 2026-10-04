package harness

import (
	"fmt"
	"errors"
)

// Stage represents a stage in the repository memory processing pipeline.
type Stage int

const (
	StageCapture Stage = iota
	StageCompact
	StageTriage
	StageExtract
	StageReconcile
	StageVerify
	StagePublish
)

func (s Stage) String() string {
	switch s {
	case StageCapture:
		return "capture"
	case StageCompact:
		return "compact"
	case StageTriage:
		return "triage"
	case StageExtract:
		return "extract"
	case StageReconcile:
		return "reconcile"
	case StageVerify:
		return "verify"
	case StagePublish:
		return "publish"
	default:
		return fmt.Sprintf("stage-%d", int(s))
	}
}

// StepKind represents the normalized kind of a transcript/episode step.
type StepKind string

const (
	StepKindUser      StepKind = "user"
	StepKindReasoning StepKind = "reasoning"
	StepKindAgent     StepKind = "agent"
	StepKindCommand   StepKind = "command"
	StepKindEdit      StepKind = "edit"
	StepKindRead      StepKind = "read"
	StepKindTest      StepKind = "test"
	StepKindDevice    StepKind = "device"
	StepKindSubagent  StepKind = "subagent"
	StepKindExposure  StepKind = "exposure"
	StepKindNote      StepKind = "note"
)

// Step represents a normalized conversation or transcript step.
type Step struct {
	Index    int            `json:"i"`
	Kind     StepKind       `json:"kind"`
	Author   string         `json:"author,omitempty"`
	Text     string         `json:"text,omitempty"`
	Command  string         `json:"cmd,omitempty"`
	ExitCode *int           `json:"exit,omitempty"`
	Outcome  string         `json:"outcome,omitempty"`
	Excerpt  string         `json:"excerpt,omitempty"`
	File     string         `json:"file,omitempty"`
	Commit   string         `json:"commit,omitempty"`
	Suite    string         `json:"suite,omitempty"`
	Passed   int            `json:"passed,omitempty"`
	Failed   int            `json:"failed,omitempty"`
	Action   string         `json:"action,omitempty"`
	Episode  string         `json:"episode,omitempty"`
	Via      string         `json:"via,omitempty"`
	Trigger  string         `json:"trigger,omitempty"`
	Claims   []string       `json:"claims,omitempty"`
	RawData  map[string]any `json:"raw_data,omitempty"`
}

// EventType represents the normalized hook event trigger.
type EventType string

const (
	EventPreInvocation EventType = "pre_invocation"
	EventPreToolUse    EventType = "pre_tool_use"
	EventPostToolUse   EventType = "post_tool_use"
	EventStop          EventType = "stop"
	EventPromptSubmit  EventType = "prompt_submit"
	EventSubagentStart EventType = "subagent_start"
	EventSubagentStop  EventType = "subagent_stop"
	EventSessionStart  EventType = "session_start"
	EventSessionEnd    EventType = "session_end"
)

// Event represents a normalized hook invocation event.
type Event struct {
	Type           EventType         `json:"type"`
	ConversationID string            `json:"conversation_id,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
	InvocationNum  int               `json:"invocation_num,omitempty"`
	TranscriptPath string            `json:"transcript_path,omitempty"`
	WorkspacePaths []string          `json:"workspace_paths,omitempty"`
	ToolName       string            `json:"tool_name,omitempty"`
	ToolInput      string            `json:"tool_input,omitempty"`
	ToolResponse   string            `json:"tool_response,omitempty"`
	StepIndex      int               `json:"step_index,omitempty"`
	Error          string            `json:"error,omitempty"`
	Prompt         string            `json:"prompt,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// Decision represents the hook response decision.
type Decision string

const (
	DecisionAllow    Decision = "allow"
	DecisionDeny     Decision = "deny"
	DecisionAsk      Decision = "ask"
	DecisionForceAsk Decision = "force_ask"
	DecisionContinue Decision = "continue"
)

// Injection represents normalized content or instructions returned to the harness.
type Injection struct {
	Decision          Decision          `json:"decision,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	EphemeralMessage  string            `json:"ephemeral_message,omitempty"`
	UserMessage       string            `json:"user_message,omitempty"`
	AdditionalContext string            `json:"additional_context,omitempty"`
	InjectSteps       []Step            `json:"inject_steps,omitempty"`
	Rewake            bool              `json:"rewake,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

// Capabilities describes features supported by a specific harness adapter.
type Capabilities struct {
	InjectBeforeModelCall bool // Antigravity PreInvocation
	InjectAtPrompt        bool // [R2] Claude Code UserPromptSubmit
	InjectAfterTool       bool // [R2] Claude Code PostToolUse additionalContext
	InjectAtSubagentStart bool // [R2] Claude Code SubagentStart
	SoftContextBeforeTool bool // [R2] Claude Code PreToolUse additionalContext
	BlockWithReason       bool // PreToolUse deny + reason (both)
	AsyncPush             bool // [R2] Claude Code asyncRewake
	HookPayloadHasToolIO  bool // false on Antigravity: read the transcript tail instead
}

// Usage captures LLM token spend metrics for a housekeeping execution.
type Usage struct {
	InputTokens     int `json:"input_tokens,omitempty"`
	OutputTokens    int `json:"output_tokens,omitempty"`
	ThinkingTokens  int `json:"thinking_tokens,omitempty"`
	CacheReadTokens int `json:"cache_read_tokens,omitempty"`
}

// Result represents the outcome of running a pipeline stage via a Housekeeper.
type Result struct {
	Payload string `json:"payload,omitempty"`
	Usage   Usage  `json:"usage,omitempty"`
}

// ReportStatus represents the polling state of an IDE-visible report.
type ReportStatus struct {
	ReportID string `json:"report_id"`
	Active   bool   `json:"active"`
	Messages []Step `json:"messages,omitempty"`
}


var ErrQuotaExhausted = errors.New("quota exhausted (HTTP 429)")
