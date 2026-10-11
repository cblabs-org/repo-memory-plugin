## API Contract (exact identifiers for hidden-test compatibility)

The hidden tests import your package and reference these exact identifiers.
You MUST use these names and signatures.

### Types
- `type OpType string`
- `type Op struct` — with fields:
  - `Type OpType`
  - `EntryID string`
  - `DossierScope string`
  - `Timestamp time.Time`
  - `Source string`
  - `SessionID string`
  - `Fields map[string]any`
- `type OpBatch struct` — with fields:
  - `Version int`
  - `Ops []Op`
  - `CreatedAt time.Time`
  - `BatchID string`
- `type Writer struct`
- `type Reader struct`
- `type Iterator struct`

### Constants
- `const OpAdd OpType = "add"`
- `const OpAmend OpType = "amend"`
- `const OpSupersede OpType = "supersede"`
- `const OpRetire OpType = "retire"`
- `const OpMove OpType = "move"`
- `const OpSplit OpType = "split"`
- `const SourceTurnNote = "turn-note"`
- `const SourceSidecar = "sidecar"`
- `const SourceSeed = "seed"`
- `const SourcePA = "pa"`
- `const SourceHuman = "human"`
- `const SourceConsolidation = "consolidation"`

### Functions
- `func ValidateOp(op Op) error`
- `func ValidateBatch(batch OpBatch) error`
- `func NewBatchID() string`
- `func NewBatchIDWithTime(t time.Time) string`
- `func NewEntryID(prefix string) string`
- `func NewEntryIDWithTime(prefix string, t time.Time) string`
- `func NewBatch(ops []Op) OpBatch`
- `func NewBatchWithTime(batchOps []Op, t time.Time) OpBatch`
- `func NewWriter(baseDir string) *Writer`
- `func (w *Writer) Write(batch OpBatch) error`
- `func NewReader(baseDir string) *Reader`
- `func (r *Reader) ReadBatch(path string) (*OpBatch, error)`
- `func (r *Reader) WithGitDir(gitDir string) *Reader`
- `func (r *Reader) Iterate(ctx context.Context) iter.Seq2[*OpBatch, error]`
- `func (r *Reader) IterateAfter(ctx context.Context, afterBatchID string) iter.Seq2[*OpBatch, error]`
- `func (r *Reader) IterateBefore(ctx context.Context, beforeCommit string) iter.Seq2[*OpBatch, error]`
- `func (r *Reader) IterateBeforeTime(ctx context.Context, cutoff time.Time) iter.Seq2[*OpBatch, error]`
- `func NewIterator(r *Reader, ctx context.Context) *Iterator`
- `func (it *Iterator) Next() (*OpBatch, error)`
- `func (it *Iterator) Close()`
- `func (it *Iterator) Seq() iter.Seq2[*OpBatch, error]`
- `func MigrateV1Events(eventsDir, archiveDir string) error`
