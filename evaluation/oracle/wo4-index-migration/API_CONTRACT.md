## API Contract (exact identifiers for hidden-test compatibility)

The hidden tests import your package and reference these exact identifiers.
You MUST use these names and signatures.

### Types
- `type Index struct`
- `type ErrorPatternResult struct` — with fields:
  - `EntryID string`
  - `CommandPrefix string`
  - `Pattern string`
  - `DossierScope string`

### Constants
- `var ErrNotFound = errors.New("not found")`

### Functions
- `func NewIndex(dbPath string) (*Index, error)`
- `func NewReadOnlyIndex(dbPath string) (*Index, error)`
- `func (idx *Index) Close() error`
- `func (idx *Index) UpsertEntry(ctx context.Context, entry dossiers.Entry) error`
- `func (idx *Index) GetEntry(ctx context.Context, id string) (*dossiers.Entry, error)`
- `func (idx *Index) DeleteEntry(ctx context.Context, id string) error`
- `func (idx *Index) ListEntriesByScope(ctx context.Context, scope string) ([]dossiers.Entry, error)`
- `func (idx *Index) SearchEntries(ctx context.Context, query string) ([]dossiers.Entry, error)`
- `func (idx *Index) UpsertEntryAnchors(ctx context.Context, entryID string, anchors []string) error`
- `func (idx *Index) LookupByAnchor(ctx context.Context, anchor string) ([]string, error)`
- `func (idx *Index) LookupByAnchorExact(ctx context.Context, anchor string) ([]string, error)`
- `func (idx *Index) InsertErrorPattern(ctx context.Context, entryID, commandPrefix, pattern, scope string) error`
- `func (idx *Index) LookupErrorPatterns(ctx context.Context, commandPrefix string) ([]ErrorPatternResult, error)`
- `func (idx *Index) UpsertAlias(ctx context.Context, oldScope, newScope string) error`
- `func (idx *Index) ResolveAlias(ctx context.Context, scope string) (string, error)`
- `func (idx *Index) InsertHistory(ctx context.Context, entry dossiers.Entry, reason string) error`
- `func (idx *Index) GetHistory(ctx context.Context, entryID string) ([]dossiers.Entry, error)`
- `func (idx *Index) QueryHistoryByScope(ctx context.Context, scope string) ([]dossiers.Entry, error)`
- `func (idx *Index) RebuildFromOpLog(ctx context.Context, opsReader *ops.Reader) error`
