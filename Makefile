.PHONY: build run test docker-build

build:
	go build -o bin/lab_gear ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

docker-build:
	docker build -t lab_gear .
