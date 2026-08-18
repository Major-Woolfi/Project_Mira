.PHONY: build clean test lint cshared

BINARY=../build/engine-memory-go/engine-memory-go
CMD=cmd/engine-memory-go

build:
	mkdir -p ../build/engine-memory-go
	go build -o $(BINARY) ./$(CMD)
	cp go.mod ../build/engine-memory-go/
	[ -f go.sum ] && cp go.sum ../build/engine-memory-go/ || true

cshared:
	mkdir -p ../build/engine-memory-go
	CGO_ENABLED=1 go build -buildmode=c-shared -o ../build/engine-memory-go/mira_memory.dll ./$(CMD)
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
