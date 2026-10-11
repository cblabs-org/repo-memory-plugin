## API Contract (exact identifiers for hidden-test compatibility)

The hidden tests import your package and reference these exact identifiers.
You MUST use these names and signatures.

### Types
- `type Channel string`
- `type Moment string`
- `type InjectionRecord struct` — with fields:
  - `EntryID string`
  - `Channel Channel`
  - `Moment Moment`
  - `StepIndex int`
  - `ContentHash string`
  - `Tag string`
  - `Timestamp time.Time`
- `type ExposureRecord struct` — with fields:
  - `EntryID string`
  - `Channel Channel`
  - `Moment Moment`
  - `Timestamp time.Time`
- `type Ledger struct` — with fields:
  - `ConversationID string`
  - `Injections []InjectionRecord`
  - `SeenDossiers map[string]bool`
  - `SeenAnchors map[string]bool`
  - `AcknowledgedGates map[string]bool`
  - `ExposureList []ExposureRecord`
  - `LastHandledCheckpointStep int`
  - `PendingGates map[string][]string`
  - `LastProcessedStepIndex int`
- `type RankedEntry struct` — with fields:
  - `Entry dossiers.Entry`
  - `Score int`
  - `AnchorMatch string`
- `type Engine struct`
- `type EngineOption func(*Engine)`

### Constants
- `const ChannelSystem Channel = "system"`
- `const ChannelEphemeral Channel = "ephemeral"`
- `const ChannelDeny Channel = "deny"`
- `const MomentTaskStart Moment = "task_start"`
- `const MomentFirstEntry Moment = "first_entry"`
- `const MomentFirstTouch Moment = "first_touch"`
- `const MomentAboutToEdit Moment = "about_to_edit"`
- `const MomentError Moment = "error"`
- `const MomentGate Moment = "gate"`

### Functions
- `func DefaultBudgets() map[Moment]int`
- `func NewLedger(conversationID string) *Ledger`
- `func (l *Ledger) RecordInjection(rec InjectionRecord)`
- `func (l *Ledger) RecordPull(entryID string)`
- `func (l *Ledger) HasSeenDossier(scope string) bool`
- `func (l *Ledger) MarkDossierSeen(scope string)`
- `func (l *Ledger) HasSeenAnchor(anchor string) bool`
- `func (l *Ledger) MarkAnchorSeen(anchor string)`
- `func (l *Ledger) IsGateAcknowledged(entryID string) bool`
- `func (l *Ledger) AcknowledgeGate(entryID string)`
- `func (l *Ledger) SetPendingGates(file string, entryIDs []string)`
- `func (l *Ledger) HasPendingGates(file string) bool`
- `func (l *Ledger) AcknowledgePendingGates(file string)`
- `func (l *Ledger) RearmDedup()`
- `func (l *Ledger) GetExposureList() []ExposureRecord`
- `func (l *Ledger) FindInjectionAfterStep(stepIndex int) []InjectionRecord`
- `func VerifyAnchor(anchor string, workspaceRoot string) bool`
- `func VerifyAnchors(anchors []string, workspaceRoot string) (valid []string, suspect []string)`
- `func RankEntries(entries []dossiers.Entry, targetAnchors []string) []RankedEntry`
- `func SelectWithinBudget(ranked []RankedEntry, tokenBudget int) []RankedEntry`
- `func FormatEntry(entry dossiers.Entry) string`
- `func FormatEntries(entries []RankedEntry) string`
- `func FormatGateDeny(entry dossiers.Entry) string`
- `func ContentHash(text string) string`
- `func NewEngine(idx *index.Index, workspaceRoot string, opts ...EngineOption) *Engine`
- `func WithBudgets(budgets map[Moment]int) EngineOption`
- `func WithScopeResolver(resolver *config.ScopeResolver) EngineOption`
- `func (e *Engine) TaskStartEntries(ctx context.Context, promptTerms []string, diffPaths []string) ([]RankedEntry, error)`
- `func (e *Engine) DossierEntries(ctx context.Context, scope string) ([]RankedEntry, error)`
- `func (e *Engine) AnchorEntries(ctx context.Context, anchor string) ([]RankedEntry, error)`
- `func (e *Engine) ErrorEntries(ctx context.Context, commandPrefix string, output string) ([]RankedEntry, error)`
- `func (e *Engine) GateEntries(ctx context.Context, targetFile string, ledger *Ledger) ([]RankedEntry, error)`
