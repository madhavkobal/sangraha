BINARY     := sangraha
MODULE     := github.com/madhavkobal/sangraha
CMD        := ./cmd/sangraha
BIN_DIR    := bin

VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)

# Cross-compilation targets
PLATFORMS  := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

.PHONY: all build build-all web test lint vet clean release fmt e2e help

## build: Compile binary for current OS/arch into ./bin/sangraha
build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)

## build-all: Cross-compile for all target platforms
build-all:
	@mkdir -p $(BIN_DIR)
	$(foreach PLATFORM,$(PLATFORMS), \
		$(eval OS   = $(word 1,$(subst /, ,$(PLATFORM)))) \
		$(eval ARCH = $(word 2,$(subst /, ,$(PLATFORM)))) \
		$(eval EXT  = $(if $(filter windows,$(OS)),.exe,)) \
		GOOS=$(OS) GOARCH=$(ARCH) go build \
			-ldflags "$(LDFLAGS)" \
			-o $(BIN_DIR)/$(BINARY)-$(OS)-$(ARCH)$(EXT) \
			$(CMD) ; \
	)

## web: Build the web dashboard (outputs to web/dist/)
web:
	@if [ ! -d web/node_modules ]; then \
		echo "Running npm ci in web/..."; \
		cd web && npm ci; \
	fi
	cd web && npm run build

## test: Run all unit tests
test:
	go test ./...

## test-race: Run all unit tests with the race detector
test-race:
	go test -race ./...

## test-cover: Run unit tests and open an HTML coverage report
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

## lint: Run golangci-lint
lint:
	golangci-lint run

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format all Go source files
fmt:
	goimports -w . || gofmt -w .

## clean: Remove build artifacts
clean:
	rm -rf $(BIN_DIR) coverage.out
	rm -rf web/dist

## release: Build all platforms and generate SHA256 checksums
release: build-all
	@cd $(BIN_DIR) && sha256sum $(BINARY)-* > SHA256SUMS
	@echo "Release artifacts in $(BIN_DIR)/"
	@cat $(BIN_DIR)/SHA256SUMS

## e2e: Run Playwright e2e tests (auto-starts the Vite dev server)
e2e:
	@if [ ! -d web/node_modules ]; then cd web && npm ci; fi
	@if [ ! -d test/e2e/node_modules ]; then cd test/e2e && npm ci; fi
	cd test/e2e && npx playwright test

## help: Show this help message
help:
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^## ' Makefile | sed 's/## /  /'
