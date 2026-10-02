import re

# Fix 1: item.Raw
with open("internal/core/tick/tick.go", "r") as f:
    code = f.read()
code = code.replace("Payload:        string(item.Raw),", "Payload:        item.Entry.ID,")
with open("internal/core/tick/tick.go", "w") as f:
    f.write(code)

# Fix 4: IndexClaim
with open("internal/core/index/index.go", "r") as f:
    code = f.read()
code = code.replace("func (idx *Index) InsertClaim", "func (idx *Index) IndexClaim")
with open("internal/core/index/index.go", "w") as f:
    f.write(code)

# Fix episode.NewStore arguments
# Check internal/core/episode/store.go
with open("internal/core/episode/store.go", "r") as f:
    st = f.read()
    if "func NewStore(" in st:
        print(re.search(r'func NewStore\(.*?\).*?{', st).group(0))

# Check internal/core/claims/store/store.go
with open("internal/core/claims/store/store.go", "r") as f:
    st = f.read()
    if "func New" in st:
        print(re.search(r'func New[A-Za-z0-9_]*\(.*?\).*?{', st).group(0))

# Check internal/core/pipeline/reconcile/reconciler.go
with open("internal/core/pipeline/reconcile/reconciler.go", "r") as f:
    st = f.read()
    print([line for line in st.split("\\n") if "Op" in line])

