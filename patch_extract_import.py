with open("internal/core/pipeline/extract/extractor.go", "r") as f:
    code = f.read()

code = code.replace('"fmt"', '"fmt"\n\t"sync"')

with open("internal/core/pipeline/extract/extractor.go", "w") as f:
    f.write(code)
