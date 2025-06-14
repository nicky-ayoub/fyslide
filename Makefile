.PHONY: all build build-gui build-cli fmt vet lint check test run run-gui run-cli clean deps deploy

# Variables
GO_CMD := go
GUI_OUTPUT := fyslide
CLI_OUTPUT := fyslide-cli
GUI_MAIN_SRC := main.go
CLI_MAIN_DIR := ./cmd/fyslide-cli

# Default LDFLAGS for release builds (strip debug symbols and DWARF table)
# For debug builds, you might want to remove these.
LDFLAGS := -s -w

# Default target when `make` is run without arguments
.DEFAULT_GOAL := build

# Default build target can build everything or just the GUI
all: build

build: deps build-gui build-cli

build-gui:
	$(GO_CMD) build -ldflags="$(LDFLAGS)" -o $(GUI_OUTPUT) $(GUI_MAIN_SRC)

build-cli:
	$(GO_CMD) build -ldflags="$(LDFLAGS)" -o $(CLI_OUTPUT) $(CLI_MAIN_DIR)

fmt:
	$(GO_CMD) fmt ./...

vet:
	$(GO_CMD) vet ./...

lint:
	revive ./...

check: fmt vet lint

test:
	$(GO_CMD) test -v ./...

run: run-gui

run-gui:
	$(MAKE) build-gui
	./$(GUI_OUTPUT)

run-cli:
	$(MAKE) build-cli
	./$(CLI_OUTPUT)

clean:
	@echo "Cleaning up..."
	@rm -f $(GUI_OUTPUT) $(CLI_OUTPUT)

deps:
	$(GO_CMD) mod tidy

# deploy target is defined but has no recipe yet.
