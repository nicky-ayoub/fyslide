.PHONY: all build build-gui build-cli fmt vet lint check test run run-gui run-cli clean deps deploy help

# Variables
GO_CMD := go
GUI_OUTPUT := fyslide
CLI_OUTPUT := fyslide-cli
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

lint: ## Run the linter on the codebase.
	revive ./...

check: fmt vet lint ## Run all code quality checks (format, vet, lint).

test: ## Run tests with the race detector enabled.
	$(GO_CMD) test -v -race ./...

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

deps: ## Tidy Go module dependencies.
	$(GO_CMD) mod tidy

deploy: ## Deploy the application (placeholder).
	@echo "Deploy target is not yet implemented."

help: ## Show this help message.
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
