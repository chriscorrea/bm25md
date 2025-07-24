.PHONY: test lint build clean examples coverage fmt deps

# default target
all: test lint build

# run tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# run linter
lint:
	golangci-lint run

# Build library
build:
	go build -v ./...

# build examples
examples:
	cd examples/basic && go build -v
	cd examples/custom && go build -v
	cd examples/bm25f && go build -v
	cd examples/tokenizer && go build -v

# Clean build artifacts
clean:
	go clean
	rm -f coverage.out
	find examples -type f -perm +111 -delete

# TODO: run benchmarks
# bench:
# 	go test -bench=. -benchmem ./...

# check coverage
coverage: test
	go tool cover -html=coverage.out

# format code
fmt:
	go fmt ./...

# install dependencies
deps:
	go mod download
	go mod tidy