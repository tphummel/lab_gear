.PHONY: build run test cover docker-build

build:
	go build -o bin/lab_gear ./cmd/server

run:
	go run ./cmd/server

# Run tests only (no coverage).
# Note: run 'go mod download' first if you see missing go.sum entries.
test:
	go test -race ./...

# Run tests and produce an HTML coverage report (coverage.html).
cover:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

docker-build:
	docker build -t lab_gear .
