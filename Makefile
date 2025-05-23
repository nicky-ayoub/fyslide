.PHONY: build build-gui build-cli lint test run clean

# Default build target can build everything or just the GUI
build:
	$(MAKE) build-gui
	$(MAKE) build-cli

build-gui:
	go build -o fyslide main.go
build-cli:
	go build -o fyslide-cli ./cmd/fyslide-cli
lint:
	revive ./...
test:
	go test -v ./...
run:
	go run main.go
clean:
	@rm -f fyslide fyslide-cli
