## API Contract (exact identifiers for hidden-test compatibility)

The hidden tests import your package and reference these exact identifiers.
You MUST use these names and signatures.

### Types
- `type EntryType string`
- `type DossierKind string`
- `type Entry struct` — with fields:
  - `ID string`
  - `Type EntryType`
  - `DossierScope string`
  - `Text string`
  - `Anchors []string`
  - `Evidence []string`
  - `Verified string`
  - `ClosesWhen string`
  - `Source string`
  - `CreatedAt time.Time`
  - `UpdatedAt time.Time`
  - `Retired bool`
  - `RetiredReason string`
  - `SupersededBy string`
- `type Dossier struct` — with fields:
  - `Scope string`
  - `Kind DossierKind`
  - `Title string`
  - `Purpose string`
  - `Entries []Entry`
  - `History []Entry`
- `type AtlasEntry struct` — with fields:
  - `Scope string`
  - `Kind DossierKind`
  - `Title string`
  - `EntryCount int`
  - `LastUpdated time.Time`
- `type Atlas struct` — with fields:
  - `Dossiers []AtlasEntry`
- `type Projector struct`
- `type Writer struct`

### Constants
- `const EntryTypeInvariant EntryType = "invariant"`
- `const EntryTypeRuledOut EntryType = "ruled-out"`
- `const EntryTypeFailureSignature EntryType = "failure-signature"`
- `const EntryTypeGotcha EntryType = "gotcha"`
- `const EntryTypeDecision EntryType = "decision"`
- `const EntryTypeProcedure EntryType = "procedure"`
- `const EntryTypeInterface EntryType = "interface"`
- `const EntryTypeOpenIssue EntryType = "open-issue"`
- `const DossierKindModule DossierKind = "module"`
- `const DossierKindTopic DossierKind = "topic"`
- `const DossierKindExternal DossierKind = "external"`
- `const DossierKindProcedure DossierKind = "procedure"`
- `var EntryTypePrefixes map[EntryType]string`

### Functions
- `func NewProjector(opsReader *ops.Reader) *Projector`
- `func (p *Projector) Project(ctx context.Context) (map[string]*Dossier, error)`
- `func (p *Projector) ProjectBefore(ctx context.Context, beforeCommit string) (map[string]*Dossier, error)`
- `func RenderDossier(d *Dossier) string`
- `func RenderHistory(d *Dossier) string`
- `func RenderAtlas(atlas *Atlas) string`
- `func NewWriter(baseDir string) *Writer`
- `func (w *Writer) WriteDossier(d *Dossier) error`
- `func (w *Writer) WriteAtlas(atlas *Atlas) error`
- `func (w *Writer) WriteAll(dossiers map[string]*Dossier) error`
- `func DetectHandEdits(rendered string, onDisk string) ([]ops.Op, error)`
- `func LintDossier(d *Dossier) []string`
