.PHONY: fmt test build

fmt:
	gofmt -w $$(find . -name '*.go' -print)

test:
	go test ./...

build:
	go build ./...
