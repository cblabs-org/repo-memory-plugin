## API Contract (exact identifiers for hidden-test compatibility)

The hidden tests import your package and reference these exact identifiers.
You MUST use these names and signatures.

### Types
- `type DeclaredScope struct` — with fields:
  - `Name string`
  - `Kind dossiers.DossierKind`
  - `Keywords []string`
  - `ToolNames []string`
  - `Commands []string`
  - `Description string`
- `type ScopeConfig struct` — with fields:
  - `Topics []DeclaredScope`
  - `Externals []DeclaredScope`
  - `Procedures []DeclaredScope`
- `type ModuleScope struct` — with fields:
  - `Name string`
  - `Kind dossiers.DossierKind`
  - `PathGlobs []string`
  - `Symbols []string`
- `type AliasMap struct`
- `type ScopeResolver struct`

### Constants
(No package-level constants; scope kinds use `dossiers.DossierKind` constants)

### Functions
- `func LoadScopeConfig(configPath string) (*ScopeConfig, error)`
- `func DeriveModuleScopes(anchors []string) []ModuleScope`
- `func NewAliasMap() *AliasMap`
- `func BuildAliasMap(opsReader *ops.Reader, ctx context.Context) (*AliasMap, error)`
- `func (a *AliasMap) SetAlias(oldScope, newScope string)`
- `func (a *AliasMap) Add(oldScope string, newDossiers []string)`
- `func (a *AliasMap) HasAlias(scope string) bool`
- `func (a *AliasMap) Targets(scope string) []string`
- `func (a *AliasMap) Resolve(scope string) string`
- `func (a *AliasMap) AllAliases() []string`
- `func NewScopeResolver(config *ScopeConfig, modules []ModuleScope, aliases *AliasMap) *ScopeResolver`
- `func (r *ScopeResolver) ResolveScope(anchors []string, keywords []string) string`
- `func (r *ScopeResolver) AllScopes() []string`
- `func (r *ScopeResolver) ScopeKind(scope string) dossiers.DossierKind`
