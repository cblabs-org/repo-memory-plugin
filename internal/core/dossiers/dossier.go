/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/dossiers/dossier.go
 * Description: Domain model definitions for repository dossiers, dossier kinds,
 *              and repository-wide Atlas index entries.
 *
 * Exported Types / Functions:
 * - DossierKind:          Classification of a dossier's scope domain.
 * - DossierKindModule:    Module-scoped dossier kind ("module").
 * - DossierKindTopic:     Topic-scoped dossier kind ("topic").
 * - DossierKindExternal:  External-system dossier kind ("external").
 * - DossierKindProcedure: Procedure-scoped dossier kind ("procedure").
 * - Dossier:              In-memory representation of a projected dossier and its retired history.
 * - AtlasEntry:           Summary metadata row for a single dossier in ATLAS.md.
 * - Atlas:                Collection of AtlasEntry summaries rendered to ATLAS.md.
 */

package dossiers

import (
	"time"
)

// DossierKind classifies the scope category of a dossier.
type DossierKind string

const (
	// DossierKindModule identifies a code module dossier derived from file/symbol anchors.
	DossierKindModule DossierKind = "module"
	// DossierKindTopic identifies a cross-cutting architectural topic dossier.
	DossierKindTopic DossierKind = "topic"
	// DossierKindExternal identifies an external system or integration dossier.
	DossierKindExternal DossierKind = "external"
	// DossierKindProcedure identifies an operational procedure or workflow dossier.
	DossierKindProcedure DossierKind = "procedure"
)

// Dossier represents a projected repository dossier containing active entries
// and retired/superseded historical entries for a single scope.
type Dossier struct {
	Scope   string      // Canonical scope identifier (e.g. "core/config").
	Kind    DossierKind // Classification of the dossier scope.
	Title   string      // Optional human-readable title for the dossier.
	Purpose string      // Markdown text for the "Purpose and boundaries" section.
	Entries []Entry     // Active entries currently projected into this dossier.
	History []Entry     // Retired and superseded entries projected into <scope>.history.md.
}

// AtlasEntry summarizes a single active dossier for the repository-wide ATLAS.md table.
type AtlasEntry struct {
	Scope       string      // Slash-separated dossier scope slug (e.g. "core/dossiers").
	Kind        DossierKind // Structural scope category (module, topic, external, or procedure).
	Title       string      // Optional human-readable dossier title.
	EntryCount  int         // Number of active entries in the dossier.
	LastUpdated time.Time   // Timestamp of the most recently updated active entry.
}

// Atlas holds the list of active dossier summaries rendered into .memory/ATLAS.md.
type Atlas struct {
	Dossiers []AtlasEntry // Active dossier summary rows.
}
