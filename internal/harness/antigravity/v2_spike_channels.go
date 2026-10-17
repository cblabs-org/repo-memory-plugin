/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/antigravity/v2_spike_channels.go
 * Description: Defines the Antigravity PreInvocation, PostInvocation, and SessionStart
 *              step-injection payload types (`InjectStep`, `SystemMessageEntry`, and
 *              `InjectStepsResponse`).
 *
 * Exported Types / Functions:
 * - InjectStep:          Single step payload injected via PreInvocation/PostInvocation/SessionStart.
 * - SystemMessageEntry:  Nested object required for persistent systemMessage injection.
 * - InjectStepsResponse: Top-level JSON response returned by step-injecting hooks.
 */

package antigravity

// InjectStep represents a single step to inject via PreInvocation/PostInvocation/SessionStart.
type InjectStep struct {
	// EphemeralMessage is a 1-invocation transient message dropped after the next planner response.
	EphemeralMessage string `json:"ephemeralMessage,omitempty"`
	// UserMessage is a persistent plain-string message recorded as a USER_INPUT step.
	UserMessage string `json:"userMessage,omitempty"`
	// SystemMessage is a persistent nested system message recorded as a SYSTEM_MESSAGE step.
	SystemMessage *SystemMessageEntry `json:"systemMessage,omitempty"`
}

// SystemMessageEntry is the nested object required for systemMessage injection.
// Unlike ephemeralMessage/userMessage (plain strings), systemMessage must be a nested object,
// and Metadata must remain omitted when targeting Antigravity CLI 1.3.x (where non-empty
// metadata maps trigger a type-mismatch panic during hook output processing).
type SystemMessageEntry struct {
	// SystemMessage is the persistent system message text injected into the trajectory.
	SystemMessage string `json:"systemMessage"`
	// Metadata is optional key-value metadata; keep nil/omitted for CLI 1.3.x compatibility.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// InjectStepsResponse is the JSON shape returned by PreInvocation/PostInvocation/SessionStart hooks.
type InjectStepsResponse struct {
	// InjectSteps is the ordered list of steps to inject before or after the invocation.
	InjectSteps []InjectStep `json:"injectSteps,omitempty"`
}
