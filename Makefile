.PHONY: build lint test run clean
build:
	go build

lint:
	revive ./...
test:
	go test -v ./...
run:
	go run main.go
clean:
	@rm fyslide
