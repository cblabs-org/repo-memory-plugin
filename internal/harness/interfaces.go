/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/harness/interfaces.go
 * Description: Defines the core adapter interfaces for interacting with agent
 *              harnesses (transcript discovery/parsing, hook payload encoding/decoding,
 *              IDE review reporting, and single/multi-turn LLM housekeeping).
 *
 * Exported Types / Functions:
 * - Harness:              Top-level contract for an agent harness adapter.
 * - TranscriptSource:     Discovers and parses agent transcripts into normalized steps.
 * - HookCodec:            Encodes and decodes harness-native hook JSON payloads.
 * - Reporter:             Starts and polls IDE-visible review reports.
 * - Housekeeper:          Executes a single pipeline stage via an LLM or deterministic runner.
 * - MultiTurnHousekeeper: Executes multiple prompts sequentially in a warm session.
 */

package harness

import "context"

// Harness defines the top-level contract for interacting with an agent harness (e.g. Antigravity, Claude Code, Fake).
type Harness interface {
	// Name returns the canonical identifier of the harness adapter.
	Name() string
	// Capabilities returns the feature flags supported by this harness adapter.
	Capabilities() Capabilities
	// Transcripts returns the transcript discovery and parsing source for this harness.
	Transcripts() TranscriptSource
	// Hooks returns the hook event and injection codec for this harness.
	Hooks() HookCodec
	// Reporter returns the IDE review report manager for this harness.
	Reporter() Reporter
}

// TranscriptSource provides discovery and parsing of agent transcripts into normalized steps.
type TranscriptSource interface {
	// Discover locates candidate transcript file paths under location.
	Discover(ctx context.Context, location string) ([]string, error)
	// Parse reads the transcript at path and returns its normalized steps.
	Parse(ctx context.Context, path string) ([]Step, error)
}

// HookCodec handles conversion between harness-native JSON hook payloads and normalized types.
type HookCodec interface {
	// DecodeEvent parses a raw harness hook JSON payload into a normalized Event.
	DecodeEvent(payload []byte) (Event, error)
	// EncodeInjection serializes a normalized Injection into a harness-native JSON response.
	EncodeInjection(injection Injection) ([]byte, error)
}

// Reporter manages starting and polling IDE-visible review reports.
type Reporter interface {
	// StartReport initiates a new review report with title and prompt, returning its report ID.
	StartReport(ctx context.Context, title string, prompt string) (string, error)
	// PollReport queries the current status and messages of reportID.
	PollReport(ctx context.Context, reportID string) (ReportStatus, error)
}

// Housekeeper executes a pipeline stage using an LLM or deterministic executor.
type Housekeeper interface {
	// Run executes stage with input and returns the stage Result and token usage.
	Run(ctx context.Context, stage Stage, input any) (Result, error)
}

// MultiTurnHousekeeper executes multiple prompts sequentially in a single warm stream-json session.
type MultiTurnHousekeeper interface {
	Housekeeper
	// RunMultiTurn executes prompts in order within a single warm session and returns per-turn results.
	RunMultiTurn(ctx context.Context, stage Stage, prompts []string) ([]Result, error)
}

