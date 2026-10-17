/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/entities/entity.go
 * Description: Defines the core Entity struct and Kind classification taxonomy
 *              for concepts extracted from a repository.
 *
 * Exported Types / Functions:
 * - Kind:          Categorical type of an extracted repository entity.
 * - KindSubsystem: Logical module or subsystem boundary.
 * - KindFile:      Tracked repository source or documentation file.
 * - KindSymbol:    Code symbol such as a function, method, or type.
 * - KindConfig:    Configuration key or dependency identifier.
 * - KindEnv:       Environment variable referenced by the codebase.
 * - KindTopic:     Domain topic declared in repository rules.
 * - KindTool:      External CLI or build tool declared in repository rules.
 * - KindError:     Canonical error signature declared in repository rules.
 * - KindExternal:  External system, service, or dependency reference.
 * - Entity:        Extracted concept with canonical ID, Kind, and Name.
 */

package entities

// Kind represents the type of an extracted entity.
type Kind string

const (
	KindSubsystem Kind = "subsystem" // Logical module or subsystem boundary.
	KindFile      Kind = "file"      // Tracked repository source or documentation file.
	KindSymbol    Kind = "symbol"    // Code symbol such as a function, method, or type.
	KindConfig    Kind = "config"    // Configuration key or dependency identifier.
	KindEnv       Kind = "env"       // Environment variable referenced by the codebase.
	KindTopic     Kind = "topic"     // Domain topic declared in repository rules.
	KindTool      Kind = "tool"      // External CLI or build tool declared in repository rules.
	KindError     Kind = "error"     // Canonical error signature declared in repository rules.
	KindExternal  Kind = "external"  // External system, service, or dependency reference.
)

// Entity represents an extracted concept from the repository.
type Entity struct {
	ID   string `json:"id"`   // Canonical prefixed identifier (e.g., "file:path/to/file.go").
	Kind Kind   `json:"kind"` // Classification category of the entity.
	Name string `json:"name"` // Unprefixed human-readable name or relative path.
}
