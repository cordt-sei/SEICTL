# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOLINT=golangci-lint
BINARY_NAME=seictl

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(shell git describe --tags --always --dirty)"

.PHONY: all lint test clean build install

all: lint test build

# Install dependencies
setup:
	$(GOCMD) mod download
	$(GOCMD) mod tidy
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin

# Run linters
lint:
	$(GOLINT) run ./...

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Build binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) cmd/seictl/main.go

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out
	rm -f coverage.html

# Install binary
install: build
	mv $(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)