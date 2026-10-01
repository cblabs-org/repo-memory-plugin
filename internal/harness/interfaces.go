package harness

import "context"

// Harness defines the top-level contract for interacting with an agent harness (e.g. Antigravity, Claude Code, Fake).
type Harness interface {
	Name() string
	Capabilities() Capabilities
	Transcripts() TranscriptSource
	Hooks() HookCodec
	Reporter() Reporter
}

// TranscriptSource provides discovery and parsing of agent transcripts into normalized steps.
type TranscriptSource interface {
	Discover(ctx context.Context, location string) ([]string, error)
	Parse(ctx context.Context, path string) ([]Step, error)
}

// HookCodec handles conversion between harness-native JSON hook payloads and normalized types.
type HookCodec interface {
	DecodeEvent(payload []byte) (Event, error)
	EncodeInjection(injection Injection) ([]byte, error)
}

// Reporter manages starting and polling IDE-visible review reports.
type Reporter interface {
	StartReport(ctx context.Context, title string, prompt string) (string, error)
	PollReport(ctx context.Context, reportID string) (ReportStatus, error)
}

// Housekeeper executes a pipeline stage using an LLM or deterministic executor.
type Housekeeper interface {
	Run(ctx context.Context, stage Stage) (Result, error)
}
