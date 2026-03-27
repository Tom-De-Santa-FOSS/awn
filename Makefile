.PHONY: build daemon cli clean

build: daemon cli

daemon:
	go build -o bin/awnd ./cmd/awnd

cli:
	go build -o bin/awn ./cmd/awn

clean:
	rm -rf bin/

run: daemon
	./bin/awnd

install: build
	cp bin/awn bin/awnd $(GOPATH)/bin/ 2>/dev/null || cp bin/awn bin/awnd ~/.local/bin/
