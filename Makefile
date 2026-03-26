SHELL := /bin/bash

APP_NAME := home-go
BIN_DIR := bin
TOOLS_DIR := .tools/bin
TOOLS_BIN := $(abspath $(TOOLS_DIR))
GOLANGCI_LINT_VERSION := v1.62.2
MOCKGEN_VERSION := v0.6.0
PKG := ./...

-include .env
export
PATH := $(TOOLS_BIN):$(PATH)
export PATH

.PHONY: all build run test clean fmt tidy lint deps tools mocks generate install-golangci-lint install-mockgen

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) .

run: build
	./$(BIN_DIR)/$(APP_NAME)

test:
	go test $(PKG) -cover

clean:
	rm -rf $(BIN_DIR)

tools: install-golangci-lint install-mockgen

install-golangci-lint:
	@mkdir -p $(TOOLS_BIN)
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

install-mockgen:
	@mkdir -p $(TOOLS_BIN)
	@echo "Installing mockgen $(MOCKGEN_VERSION)..."
	GOBIN=$(TOOLS_BIN) go install go.uber.org/mock/mockgen@$(MOCKGEN_VERSION)

fmt:
	go fmt $(PKG)

tidy:
	go mod tidy

lint: install-golangci-lint
	$(TOOLS_BIN)/golangci-lint run

mocks: install-mockgen
	@echo "Generating mocks..."
	@mkdir -p mocks
	go generate ./mocks/...
	@echo "Mocks generated successfully!"

generate:
	@echo "Generating entities from Home Assistant..."
	@go generate
