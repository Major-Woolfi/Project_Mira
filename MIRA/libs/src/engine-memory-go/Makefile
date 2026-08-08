.PHONY: build clean test lint

BINARY=../build/engine-memory-go/engine-memory-go
CMD=cmd/engine-memory-go

build:
	mkdir -p ../build/engine-memory-go
	go build -o $(BINARY) ./$(CMD)
	cp go.mod ../build/engine-memory-go/
	[ -f go.sum ] && cp go.sum ../build/engine-memory-go/ || true

clean:
	rm -rf bin ../build

test:
	go test -race ./internal/...

lint:
	golangci-lint run ./...

run: build
	$(BINARY)
