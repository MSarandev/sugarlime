BINARY := sugarlime
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build run install test cover lint fmt gen tidy clean snapshot release-check

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/sugarlime

run:
	go run ./cmd/sugarlime

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/sugarlime

test:
	go test ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run

fmt:
	gofmt -l -w .

# Regenerate internal/schema/registry_gen.go after refreshing settings_values.md.
gen:
	go run ./cmd/gen-schema

tidy:
	go mod tidy

# Build every release artifact locally, publishing nothing. Needs goreleaser.
snapshot:
	goreleaser release --snapshot --clean

# Check .goreleaser.yaml without building anything.
release-check:
	goreleaser check

clean:
	rm -rf $(BINARY) coverage.out dist
