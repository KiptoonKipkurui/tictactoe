GO ?= go
BIN_DIR ?= bin
SERVER_BIN := $(BIN_DIR)/server
CLIENT_BIN := $(BIN_DIR)/client

.PHONY: help build build-server build-client run-server run-client test clean

help:
	@printf "Available targets:\n"
	@printf "  make build         Build server and client binaries\n"
	@printf "  make build-server  Build the server binary\n"
	@printf "  make build-client  Build the client binary\n"
	@printf "  make run-server    Run the server locally\n"
	@printf "  make run-client    Run the client locally\n"
	@printf "  make test          Run the test suite\n"
	@printf "  make clean         Remove built binaries\n"

build: build-server build-client

build-server:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(SERVER_BIN) ./cmd/server

build-client:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(CLIENT_BIN) ./cmd/client

run-server:
	$(GO) run ./cmd/server

run-client:
	$(GO) run ./cmd/client

test:
	$(GO) test ./...

clean:
	rm -rf $(BIN_DIR)
