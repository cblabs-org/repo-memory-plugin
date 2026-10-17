/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/types.go
 * Description: Defines normalized pipeline stages, transcript steps, hook events,
 *              hook decisions, context injections, adapter capabilities, token usage
 *              metrics, and sentinel errors shared across harness implementations.
 *
 * Exported Types / Functions:
 * - Stage: Pipeline stage enum for repository memory processing.
 *
 *   Stringer (fmt.Stringer):
 *   - String(): Returns the lowercase name of the pipeline stage.
 *
 * - StageCapture, StageCompact, StageTriage, StageExtract, StageReconcile,
 *   StageVerify, StagePublish, StageDistill, StageConsolidate: Stage enum constants.
 * - StepKind: Normalized transcript/episode step kind.
 * - StepKindUser, StepKindReasoning, StepKindAgent, StepKindCommand, StepKindEdit,
 *   StepKindRead, StepKindTest, StepKindDevice, StepKindSubagent, StepKindExposure,
 *   StepKindNote: StepKind constants.
 * - Step: Normalized conversation or transcript step record.
 * - EventType: Normalized hook event trigger type.
 * - EventPreInvocation, EventPreToolUse, EventPostToolUse, EventStop,
 *   EventPromptSubmit, EventSubagentStart, EventSubagentStop, EventSessionStart,
 *   EventSessionEnd: EventType constants.
 * - Event: Normalized hook invocation event payload.
 * - Decision: Hook response decision value.
 * - DecisionAllow, DecisionDeny, DecisionAsk, DecisionForceAsk,
 *   DecisionContinue: Decision constants.
 * - Injection: Normalized context or decision returned to the harness.
 * - Capabilities: Feature support matrix for a harness adapter.
 * - Usage: LLM token spend metrics for a housekeeping execution.
 * - Result: Outcome payload and token usage from a Housekeeper stage run.
 * - ReportStatus: Polling state and messages of an IDE-visible report.
 * - ErrQuotaExhausted: Sentinel error returned when the LLM quota is exhausted (HTTP 429).
 */

package harness

import (
	"errors"
	"fmt"
)

// Stage represents a stage in the repository memory processing pipeline.
type Stage int

const (
	StageCapture     Stage = iota // Raw transcript capture stage.
	StageCompact                  // Deterministic transcript compaction stage.
	StageTriage                   // Candidate episode triage stage.
	StageExtract                  // Knowledge candidate extraction stage.
	StageReconcile                // Candidate-to-dossier reconciliation stage.
	StageVerify                   // Citation and anchor verification stage.
	StagePublish                  // Dossier and index publication stage.
	StageDistill                  // Stage 1 / Stage 2 episode distillation stage.
	StageConsolidate              // Stage 2 inbox-to-dossier consolidation stage.
)

// String returns the canonical lowercase name of the pipeline stage.
func (s Stage) String() string {
	// --- Step 1: Map known Stage enum values to their canonical string names ---
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
	case StageDistill:
		return "distill"
	case StageConsolidate:
		return "consolidate"
	default:
		return fmt.Sprintf("stage-%d", int(s))
	}
}

// StepKind represents the normalized kind of a transcript/episode step.
type StepKind string

const (
	StepKindUser      StepKind = "user"      // User prompt or explicit user input step.
	StepKindReasoning StepKind = "reasoning" // Model internal chain-of-thought reasoning step.
	StepKindAgent     StepKind = "agent"     // Model response or planner output step.
	StepKindCommand   StepKind = "command"   // Shell command execution step.
	StepKindEdit      StepKind = "edit"      // File creation or modification step.
	StepKindRead      StepKind = "read"      // File or directory read step.
	StepKindTest      StepKind = "test"      // Automated test suite execution step.
	StepKindDevice    StepKind = "device"    // Hardware or device interaction step.
	StepKindSubagent  StepKind = "subagent"  // Subagent invocation or reply step.
	StepKindExposure  StepKind = "exposure"  // Memory entry injection or pull exposure step.
	StepKindNote      StepKind = "note"      // Agent-authored memory note step.
)

// Step represents a normalized conversation or transcript step.
type Step struct {
	Index    int            `json:"i"`                  // 0-based sequential step index.
	Kind     StepKind       `json:"kind"`               // Normalized category of the step.
	Author   string         `json:"author,omitempty"`   // Actor responsible for the step (e.g., "user", "model").
	Text     string         `json:"text,omitempty"`     // Primary text content of the step.
	Command  string         `json:"cmd,omitempty"`      // Executed command string for command steps.
	ExitCode *int           `json:"exit,omitempty"`     // Process exit code for command steps.
	Outcome  string         `json:"outcome,omitempty"`  // High-level execution outcome (e.g., "pass", "fail").
	Excerpt  string         `json:"excerpt,omitempty"`  // Truncated output excerpt from a tool or command.
	File     string         `json:"file,omitempty"`     // Target file path for read or edit steps.
	Commit   string         `json:"commit,omitempty"`   // Git commit hash associated with the step.
	Suite    string         `json:"suite,omitempty"`    // Test suite name for test steps.
	Passed   int            `json:"passed,omitempty"`   // Count of passing tests in a test step.
	Failed   int            `json:"failed,omitempty"`   // Count of failing tests in a test step.
	Action   string         `json:"action,omitempty"`   // Device or subagent action name.
	Episode  string         `json:"episode,omitempty"`  // Target episode ID (e.g. "ep-0001") accessed during a memory_use step.
	Via      string         `json:"via,omitempty"`      // Delivery channel or mechanism used for exposure steps.
	Trigger  string         `json:"trigger,omitempty"`  // Delivery moment or trigger for exposure steps.
	Claims   []string       `json:"claims,omitempty"`   // Entry or claim IDs exposed in this step.
	RawData  map[string]any `json:"raw_data,omitempty"` // Unstructured harness-specific metadata.
}

// EventType represents the normalized hook event trigger.
type EventType string

const (
	EventPreInvocation EventType = "pre_invocation" // Fired before each model invocation in a turn.
	EventPreToolUse    EventType = "pre_tool_use"   // Fired before a tool call is executed.
	EventPostToolUse   EventType = "post_tool_use"  // Fired after a tool call completes.
	EventStop          EventType = "stop"           // Fired when the agent finishes a turn or halts.
	EventPromptSubmit  EventType = "prompt_submit"  // Fired when the user submits a prompt.
	EventSubagentStart EventType = "subagent_start" // Fired when a subagent is spawned.
	EventSubagentStop  EventType = "subagent_stop"  // Fired when a subagent completes.
	EventSessionStart  EventType = "session_start"  // Fired when a new session begins.
	EventSessionEnd    EventType = "session_end"    // Fired when a session terminates.
)

// Event represents a normalized hook invocation event.
type Event struct {
	Type           EventType         `json:"type"`                      // Normalized hook trigger type.
	ConversationID string            `json:"conversation_id,omitempty"` // Canonical thread or conversation UUID from the host harness.
	SessionID      string            `json:"session_id,omitempty"`      // Ephemeral process or runtime session handle when distinct from ConversationID.
	InvocationNum  int               `json:"invocation_num,omitempty"`  // 0-based invocation number within the turn.
	TranscriptPath string            `json:"transcript_path,omitempty"` // Filesystem path to the session transcript.
	WorkspacePaths []string          `json:"workspace_paths,omitempty"` // Active workspace root paths.
	ToolName       string            `json:"tool_name,omitempty"`       // Invoked tool name for tool hooks.
	ToolInput      string            `json:"tool_input,omitempty"`      // Serialized tool input arguments.
	ToolResponse   string            `json:"tool_response,omitempty"`   // Serialized tool execution output.
	StepIndex      int               `json:"step_index,omitempty"`      // Transcript step index at the time of the event.
	Error          string            `json:"error,omitempty"`           // Error string reported by the harness or tool.
	Prompt         string            `json:"prompt,omitempty"`          // User prompt text associated with the event.
	Metadata       map[string]string `json:"metadata,omitempty"`        // Additional harness-specific key-value metadata.
}

// Decision represents the hook response decision.
type Decision string

const (
	DecisionAllow    Decision = "allow"     // Permits the tool call or turn to proceed.
	DecisionDeny     Decision = "deny"      // Blocks the tool call with a reason.
	DecisionAsk      Decision = "ask"       // Prompts the user for confirmation if not auto-approved.
	DecisionForceAsk Decision = "force_ask" // Forces an interactive user confirmation prompt.
	DecisionContinue Decision = "continue"  // Instructs the Stop hook to continue with an injected prompt.
)

// Injection represents normalized content or instructions returned to the harness.
type Injection struct {
	Decision          Decision          `json:"decision,omitempty"`           // Hook gate or continuation decision.
	Reason            string            `json:"reason,omitempty"`             // Explanation for a deny, force_ask, or continue decision.
	EphemeralMessage  string            `json:"ephemeral_message,omitempty"`  // Transient single-turn context message.
	UserMessage       string            `json:"user_message,omitempty"`       // Persistent user-role message injected into the transcript.
	AdditionalContext string            `json:"additional_context,omitempty"` // Supplemental context attached to a prompt or tool result.
	InjectSteps       []Step            `json:"inject_steps,omitempty"`       // Normalized steps to inject into the trajectory.
	Rewake            bool              `json:"rewake,omitempty"`             // Whether to asynchronously wake the agent session.
	Metadata          map[string]string `json:"metadata,omitempty"`           // Optional metadata attached to the injection response.
}

// Capabilities describes features supported by a specific harness adapter.
type Capabilities struct {
	InjectBeforeModelCall bool // Supports ephemeral context injection before each LLM call (Antigravity PreInvocation).
	InjectAtPrompt        bool // [R2] Claude Code UserPromptSubmit.
	InjectAfterTool       bool // [R2] Claude Code PostToolUse additionalContext.
	InjectAtSubagentStart bool // [R2] Claude Code SubagentStart.
	SoftContextBeforeTool bool // [R2] Claude Code PreToolUse additionalContext.
	BlockWithReason       bool // PreToolUse deny + reason (both).
	AsyncPush             bool // [R2] Claude Code asyncRewake.
	HookPayloadHasToolIO  bool // False on Antigravity: read the transcript tail instead.
}

// Usage captures LLM token spend metrics for a housekeeping execution.
type Usage struct {
	InputTokens     int `json:"input_tokens,omitempty"`      // Prompt input tokens consumed.
	OutputTokens    int `json:"output_tokens,omitempty"`     // Model completion output tokens generated.
	ThinkingTokens  int `json:"thinking_tokens,omitempty"`   // Reasoning/thinking tokens consumed.
	CacheReadTokens int `json:"cache_read_tokens,omitempty"` // Input tokens served from prompt cache.
}

// Result represents the outcome of running a pipeline stage via a Housekeeper.
type Result struct {
	Payload string `json:"payload,omitempty"` // Raw string or JSON output returned by the stage.
	Usage   Usage  `json:"usage,omitempty"`   // Token spend metrics for the stage run.
}

// ReportStatus represents the polling state of an IDE-visible report.
type ReportStatus struct {
	ReportID string `json:"report_id"`          // Unique identifier of the report session.
	Active   bool   `json:"active"`             // Whether the report is still active/running.
	Messages []Step `json:"messages,omitempty"` // Steps or messages recorded in the report.
}

// ErrQuotaExhausted is returned when an LLM housekeeping invocation fails due to HTTP 429 quota exhaustion.
var ErrQuotaExhausted = errors.New("quota exhausted (HTTP 429)")
