/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/entities/relation.go
 * Description: Defines directed relationship types and the Relation edge struct
 *              connecting pairs of repository entities.
 *
 * Exported Types / Functions:
 * - RelationKind:       Semantic category of a directed link between two entities.
 * - RelationContains:   Parent entity structurally contains the target entity.
 * - RelationDependsOn:  Source entity depends on the target entity.
 * - RelationImplements: Source entity implements the contract of the target entity.
 * - RelationConfigures: Source configuration entity governs the target entity.
 * - Relation:           Directed edge linking a SourceID entity to a TargetID entity.
 */

package entities

// RelationKind represents the type of relationship between two entities.
type RelationKind string

const (
	RelationContains   RelationKind = "contains"   // Source entity structurally contains target entity.
	RelationDependsOn  RelationKind = "depends_on" // Source entity depends on target entity.
	RelationImplements RelationKind = "implements" // Source entity implements target interface or spec.
	RelationConfigures RelationKind = "configures" // Source config entity configures target entity.
)

// Relation describes a directed link from a Source entity to a Target entity.
type Relation struct {
	SourceID string       `json:"source_id"` // Canonical ID of the origin entity.
	TargetID string       `json:"target_id"` // Canonical ID of the destination entity.
	Kind     RelationKind `json:"kind"`      // Semantic relationship kind linking source to target.
}
