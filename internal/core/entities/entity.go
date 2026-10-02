package entities

// Kind represents the type of an extracted entity.
type Kind string

const (
	KindSubsystem Kind = "subsystem"
	KindFile      Kind = "file"
	KindSymbol    Kind = "symbol"
	KindConfig    Kind = "config"
	KindEnv       Kind = "env"
	KindTopic     Kind = "topic"
	KindTool      Kind = "tool"
	KindError     Kind = "error"
	KindExternal  Kind = "external"
)

// Entity represents an extracted concept from the repository.
type Entity struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}
