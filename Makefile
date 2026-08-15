.PHONY: all build build-gui build-cli fmt vet lint check test run run-gui run-cli clean deps deploy help \
	ci coverage verify-generated cross-build release install bench docker-build fmt-check watch

# Variables
GO_CMD := go
GUI_OUTPUT := bin/fyslide
CLI_OUTPUT := bin/fyslide-cli
GUI_MAIN_DIR := ./cmd/fyslide
CLI_MAIN_DIR := ./cmd/fyslide-cli
GENERATED_ASSETS := internal/ui/bundle*.go

# Default LDFLAGS for release builds (strip debug symbols and DWARF table)
# Run `make DEBUG=1 build` for a debug build.
LDFLAGS := -s -w
ifeq ($(DEBUG), 1)
	LDFLAGS = -gcflags="all=-N -l"
endif

# Default target when `make` is run without arguments
.DEFAULT_GOAL := help

# Build Targets
all: build ## Build both GUI and CLI applications.

build: deps build-gui build-cli ## Build both applications after checking dependencies.

build-gui: gen ## Build the GUI application.
	@echo "Building GUI application..."
	$(GO_CMD) build -ldflags="$(LDFLAGS)" -o $(GUI_OUTPUT) $(GUI_MAIN_DIR)

build-cli: ## Build the CLI application.
	@echo "Building CLI application..."
	$(GO_CMD) build -ldflags="$(LDFLAGS)" -o $(CLI_OUTPUT) $(CLI_MAIN_DIR)

# Development Targets
gen: ## Generate bundled assets from the assets directory.
	@echo "Generating bundled assets..."
	$(GO_CMD) generate ./...

fmt: ## Format the Go source code.
	$(GO_CMD) fmt ./...

vet: ## Run go vet to check for suspicious constructs.
	$(GO_CMD) vet ./...

lint: ## Run the linter on the codebase (prefer golangci-lint, fallback to revive).
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... || true; \
	else \
		revive ./... | grep -v _test.go || true; \
	fi

lint-all: ## Run both golangci-lint (if available) and revive for maximum coverage.
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... || true; \
	fi
	@revive ./... | grep -v _test.go || true

## install-linters removed — install golangci-lint manually or via CI container

check: fmt vet lint ## Run all code quality checks (format, vet, lint).

test: ## Run tests with the race detector enabled.
	$(GO_CMD) test -v -race ./...

# CI / Extra Targets
ci: verify-generated fmt-check vet lint test coverage ## Run full CI-style checks (verify generated assets, format, vet, lint, tests, coverage)

coverage: ## Run tests with coverage and print summary
	@echo "Running tests with coverage..."
	$(GO_CMD) test ./... -coverprofile=coverage.out
	@$(GO_CMD) tool cover -func=coverage.out

verify-generated: gen ## Ensure generated assets are up-to-date (fail if not)
	@git diff --exit-code -- $(GENERATED_ASSETS)

fmt-check: ## Fail if any files need `gofmt` formatting
	@files=$$(gofmt -l .); if [ -n "$$files" ]; then echo "gofmt needs to be run on:"; echo "$$files"; exit 1; fi

cross-build: ## Cross-compile release artifacts for common platforms
	@echo "Cross-building binaries..."
	GOOS=linux GOARCH=amd64 $(GO_CMD) build -ldflags="$(LDFLAGS)" -o bin/fyslide-linux-amd64 $(GUI_MAIN_DIR)
	GOOS=darwin GOARCH=amd64 $(GO_CMD) build -ldflags="$(LDFLAGS)" -o bin/fyslide-darwin-amd64 $(GUI_MAIN_DIR)
	GOOS=windows GOARCH=amd64 $(GO_CMD) build -ldflags="$(LDFLAGS)" -o bin/fyslide-windows-amd64.exe $(GUI_MAIN_DIR)

release: cross-build ## Package cross-built artifacts into a tarball
	@mkdir -p release
	tar -C bin -czf release/$(shell git describe --tags --always).tar.gz .

install: ## Install both binaries with `go install`
	$(GO_CMD) install $(GUI_MAIN_DIR)
	$(GO_CMD) install $(CLI_MAIN_DIR)

bench: ## Run benchmarks
	$(GO_CMD) test ./... -bench . -run ^$ -benchmem

docker-build: ## Build a docker image (requires Dockerfile)
	docker build -t fyslide:latest .

watch: ## Rerun tests on file changes (requires `entr`)
	find . -name '*.go' | entr -c $(MAKE) test

# Run Targets
run: run-gui ## Run the GUI application (default run action).

run-gui: build-gui ## Build and run the GUI application.
	./$(GUI_OUTPUT)

run-cli: build-cli ## Build and run the CLI application.
	./$(CLI_OUTPUT)

# Housekeeping Targets
clean: ## Clean up build artifacts and generated files.
	@echo "Cleaning up..."
	@rm -f $(GUI_OUTPUT) $(CLI_OUTPUT)
	@rm -f $(GENERATED_ASSETS)
	@go clean -testcache

deps: ## Tidy Go module dependencies.
	$(GO_CMD) mod tidy

deploy: ## Deploy the application (placeholder).
	@echo "Deploy target is not yet implemented."

full-rebuild: clean deps build test ## Clean, update dependencies, and rebuild both applications.
	@echo "Full rebuild completed."

help: ## Show this help message.
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

-include Makefile.ci
