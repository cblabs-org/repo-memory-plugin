package dossiers

import (
	"time"
)

type DossierKind string

const (
	DossierKindModule    DossierKind = "module"
	DossierKindTopic     DossierKind = "topic"
	DossierKindExternal  DossierKind = "external"
	DossierKindProcedure DossierKind = "procedure"
)

type Dossier struct {
	Scope   string
	Kind    DossierKind
	Title   string
	Purpose string
	Entries []Entry
	History []Entry
}

type AtlasEntry struct {
	Scope      string
	Kind       DossierKind
	Title      string
	EntryCount int
	LastUpdated time.Time
}

type Atlas struct {
	Dossiers []AtlasEntry
}
