.PHONY: build daemon cli mcp clean test test-scripts test-all lint fmt

LDFLAGS := -ldflags "-s -w -X main.version=$(shell cat VERSION) -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)"

build: daemon cli mcp

daemon:
	go build $(LDFLAGS) -o bin/awnd ./cmd/awnd

cli:
	go build $(LDFLAGS) -o bin/awn ./cmd/awn

mcp:
	go build $(LDFLAGS) -o bin/awn-mcp ./cmd/awn-mcp

clean:
	rm -rf bin/

run: daemon
	./bin/awnd

test:
	go test ./... -v -race

test-scripts:
	bash scripts/release_test.sh
	bash scripts/install_test.sh

test-all: test test-scripts

lint:
	golangci-lint run ./...

fmt:
	gofumpt -w .
	goimports -w .

install: build
	cp bin/awn bin/awnd bin/awn-mcp $(GOPATH)/bin/ 2>/dev/null || cp bin/awn bin/awnd bin/awn-mcp ~/.local/bin/
