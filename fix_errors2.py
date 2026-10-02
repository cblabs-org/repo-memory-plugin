with open("internal/core/tick/pipeline.go", "r") as f:
    code = f.read()

# Fix episode.NewStore
code = code.replace('epStore := episode.NewStore(filepath.Join(repoDataDir, "episodes"))', 'epStore := episode.NewStore(repoRoot, filepath.Join(repoDataDir, "episodes"))')

# Fix claimstore.NewStore (it's actually claimstore.New(dbPath))
code = code.replace('claimstore.NewStore', 'claimstore.New')

# Fix reconcile.OpPropose and OpAmend
code = code.replace('action.Op == reconcile.OpPropose || action.Op == reconcile.OpAmend', 'action.Op == reconcile.OpCreate || action.Op == reconcile.OpRefine || action.Op == reconcile.OpPromote')

with open("internal/core/tick/pipeline.go", "w") as f:
    f.write(code)

