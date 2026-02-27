.PHONY: build run test cover smoke-test docker-build

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

smoke-test: build
	@echo "Starting server with in-memory database..."
	API_TOKEN=smoke-test-token DB_PATH=:memory: ./bin/lab_gear & echo $$! > /tmp/lab_gear_smoke.pid; \
	sleep 2; \
	k6 run -e API_TOKEN=smoke-test-token -e BASE_URL=http://localhost:8080 k6/scripts/smoke.js; \
	EXIT=$$?; \
	kill $$(cat /tmp/lab_gear_smoke.pid) 2>/dev/null || true; \
	rm -f /tmp/lab_gear_smoke.pid; \
	exit $$EXIT

docker-build:
	docker build -t lab_gear .
