SHELL := /bin/bash

APP_NAME := home-go
BIN_DIR := bin
PKG := ./...

.PHONY: all build run test clean fmt tidy lint deps tools generate-entities

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

generate:
	@echo "Generating entities from Home Assistant..."
	@cd entities && go generate
