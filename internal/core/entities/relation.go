package entities

// RelationKind represents the type of relationship between two entities.
type RelationKind string

const (
	RelationContains   RelationKind = "contains"
	RelationDependsOn  RelationKind = "depends_on"
	RelationImplements RelationKind = "implements"
	RelationConfigures RelationKind = "configures"
)

// Relation describes a directed link from a Source entity to a Target entity.
type Relation struct {
	SourceID string       `json:"source_id"`
	TargetID string       `json:"target_id"`
	Kind     RelationKind `json:"kind"`
}
