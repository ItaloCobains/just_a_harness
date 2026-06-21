.DEFAULT_GOAL := build
BIN := bin

# Override on the command line, e.g. `make chat MODEL=qwen3:14b`
MODEL    ?= qwen2.5-coder:7b
ENDPOINT ?= http://localhost:11434
TEMP     ?= 0

export HARNESS_MODEL    := $(MODEL)
export HARNESS_ENDPOINT := $(ENDPOINT)
export HARNESS_TEMPERATURE := $(TEMP)

.PHONY: build chat agent run test fmt vet check clean tidy

## build: compile both binaries into bin/
build:
	go build -o $(BIN)/chat ./cmd/chat
	go build -o $(BIN)/agent ./cmd/agent

## chat: build and launch the interactive TUI (RESUME=latest to resume)
chat: build
	./$(BIN)/chat $(if $(RESUME),--resume=$(RESUME),)

## agent: run the single-turn CLI, e.g. `make agent TASK="list the files"`
agent: build
	./$(BIN)/agent $(TASK)

## run: alias for chat
run: chat

## test: run the full test suite
test:
	go test ./...

## fmt: format all Go sources
fmt:
	gofmt -w .

## vet: run go vet
vet:
	go vet ./...

## check: fmt, vet, and test in one shot
check: fmt vet test

## tidy: sync go.mod/go.sum
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf $(BIN)

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
