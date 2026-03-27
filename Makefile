.PHONY: build daemon cli clean test test-scripts test-all

build: daemon cli

daemon:
	go build -o bin/awnd ./cmd/awnd

cli:
	go build -o bin/awn ./cmd/awn

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

install: build
	cp bin/awn bin/awnd $(GOPATH)/bin/ 2>/dev/null || cp bin/awn bin/awnd ~/.local/bin/
