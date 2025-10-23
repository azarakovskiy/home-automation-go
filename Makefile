SHELL := /bin/bash

APP_NAME := home-go
BIN_DIR := bin
PKG := ./...

include .env
export

.PHONY: all build run test clean fmt tidy lint deps tools generate-entities mocks install-mockgen

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
	rm -rf mocks

fmt:
	go fmt $(PKG)

tidy:
	go mod tidy

GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)
lint:
	@if [ -z "$(GOLANGCI_LINT)" ]; then \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(HOME)/go/bin v1.62.2; \
	fi
	$(HOME)/go/bin/golangci-lint run

MOCKGEN := $(shell command -v mockgen 2>/dev/null)
install-mockgen:
	@if [ -z "$(MOCKGEN)" ]; then \
		echo "Installing mockgen..."; \
		go install go.uber.org/mock/mockgen@latest; \
	else \
		echo "mockgen already installed"; \
	fi

mocks: install-mockgen
	@echo "Generating mocks..."
	@mkdir -p mocks
	go generate ./mocks/...
	@echo "Mocks generated successfully!"

generate:
	@echo "Generating entities from Home Assistant..."
	@go generate
