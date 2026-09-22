.PHONY: all build test test-race lint clean env-example run run-dev kill

all: test build

build:
	go build -v ./...

test:
	go test -v ./...

test-race:
	go test -v -race ./...

lint:
	go vet ./...

env-example:
	go test -v -run TestSchemaEngine_IntrospectionAndTemplate ./internal/platform/config/...

run:
	go build -o bin/server ./cmd/server
	./bin/server

# Dev mode: pipe signals through go run for interactive local development.
run-dev:
	go run ./cmd/server

# POSIX-compliant termination (kill -TERM, never kill -SIGTERM in /bin/sh).
kill:
	pkill -TERM -f "bin/server" || true
	pkill -TERM -f "exe/server" || true

clean:
	go clean -cache -testcache
