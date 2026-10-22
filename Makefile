BINARY_NAME := repo-memory
PLUGIN_NAME := repo-memory-daemon
OUTPUT_DIR  := bin
DIST_DIR    := dist

RAW_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
VERSION     ?= $(patsubst v%,%,$(RAW_VERSION))

TARGETS := \
	darwin/arm64 \
	darwin/amd64 \
	linux/amd64 \
	linux/arm64 \
	windows/amd64

.PHONY: all build install dist-zip test clean $(TARGETS)

all: build

build: $(TARGETS)

$(TARGETS):
	@mkdir -p $(OUTPUT_DIR)
	$(eval OS := $(word 1,$(subst /, ,$@)))
	$(eval ARCH := $(word 2,$(subst /, ,$@)))
	$(eval EXT := $(if $(filter windows,$(OS)),.exe,))
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o $(OUTPUT_DIR)/$(BINARY_NAME)-$(OS)-$(ARCH)$(EXT) ./cmd/repo-memory

install:
	./scripts/install.sh

dist-zip: build
	@rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@for target in $(TARGETS); do \
		os=$$(echo $$target | cut -d/ -f1); \
		arch=$$(echo $$target | cut -d/ -f2); \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		stage="$(DIST_DIR)/stage-$$os-$$arch/$(PLUGIN_NAME)"; \
		mkdir -p "$$stage/bin"; \
		cp LICENSE "$$stage/LICENSE"; \
		cp "$(OUTPUT_DIR)/$(BINARY_NAME)-$$os-$$arch$$ext" "$$stage/bin/$(BINARY_NAME)$$ext"; \
		chmod 0755 "$$stage/bin/$(BINARY_NAME)$$ext"; \
		REPO_MEMORY_PLUGIN_DIR="$$stage" go run -ldflags="-X 'main.Version=$(VERSION)'" ./cmd/repo-memory install --portable --os="$$os"; \
		(cd "$(DIST_DIR)/stage-$$os-$$arch" && zip -qr "../$(PLUGIN_NAME)-$$os-$$arch.zip" "$(PLUGIN_NAME)"); \
		rm -rf "$(DIST_DIR)/stage-$$os-$$arch"; \
		echo "Created $(DIST_DIR)/$(PLUGIN_NAME)-$$os-$$arch.zip"; \
	done

test:
	go test ./...

clean:
	rm -rf $(OUTPUT_DIR) $(DIST_DIR)
