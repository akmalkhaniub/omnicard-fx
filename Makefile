.PHONY: all build run test bench clean

GO ?= go

all: build test

build:
	@echo "Building OmniCard & FX server..."
	$(GO) build -v -o bin/server.exe ./cmd/server

run: build
	./bin/server.exe

test:
	@echo "Running card and authorization test suite..."
	$(GO) test -v ./tests/...

bench:
	@echo "Running JIT authorization latency benchmark..."
	$(GO) test -bench BenchmarkJITAuthorization -benchmem ./tests/...

clean:
	@rm -rf bin/
