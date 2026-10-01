BINARY_NAME := repo-memory
OUTPUT_DIR := bin

TARGETS := \
	darwin/arm64 \
	darwin/amd64 \
	linux/amd64 \
	linux/arm64 \
	windows/amd64

.PHONY: all build test clean $(TARGETS)

all: build

build: $(TARGETS)

$(TARGETS):
	@mkdir -p $(OUTPUT_DIR)
	$(eval OS := $(word 1,$(subst /, ,$@)))
	$(eval ARCH := $(word 2,$(subst /, ,$@)))
	$(eval EXT := $(if $(filter windows,$(OS)),.exe,))
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-$(OS)-$(ARCH)$(EXT) ./cmd/repo-memory

test:
	go test ./...

clean:
	rm -rf $(OUTPUT_DIR)
