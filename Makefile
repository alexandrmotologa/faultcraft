.PHONY: all build build-ui build-go test test-integration clean run

VERSION ?= 0.1.0
COMMIT ?= $(shell git rev-parse --short HEAD 2> /dev/null || echo "unknown")
LDFLAGS = -s -w -X main.Version=$(VERSION) -X main.GitCommit=$(COMMIT)

all: build

build-ui:
	@echo "Building React 19 Frontend..."
	cd web && npm install && npm run build

build-go:
	@echo "Compiling FaultCraft Binary..."
	go build -ldflags "$(LDFLAGS)" -o bin/faultcraft ./cmd/faultcraft

build: build-ui build-go

test:
	@echo "Running Unit and Integration Tests..."
	go test -v -race -timeout 60s ./...

test-coverage:
	@echo "Generating Test Coverage Report..."
	go test -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -func=coverage.txt

run:
	go run ./cmd/faultcraft start --api-addr ":8474"

clean:
	rm -rf bin/ web/dist/ coverage.txt test-results/
