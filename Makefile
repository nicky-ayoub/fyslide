.PHONY: build lint test run clean
build:
	go build -o fyslide main.go
lint:
	revive ./...
test:
	go test -v ./...
run:
	go run main.go
clean:
	@rm fyslide
