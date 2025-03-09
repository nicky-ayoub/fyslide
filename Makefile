build:
	go build

lint:
	revive ./...

clean:
	@rm fyslide
