GO_FILES     = $(shell find . -name '*.go')
EXECUTABLE  ?= $(shell find dist/ -path "*$(shell go env GOOS)*$(shell go env GOARCH)*" -type f -name dingopie | head -n 1)
SCRIPTS_DIR := scripts
SEND_BASH   := $(SCRIPTS_DIR)/test-send.bash
SHELL_BASH  := $(SCRIPTS_DIR)/test-shell.bash
SEND_PS1    := $(SCRIPTS_DIR)/test-send.ps1

help:
	@echo "Makefile commands:"
	@echo
	@echo "Develop:"
	@echo "  make setup                Setup development environment"
	@echo "  make lint                 Run golangci-lint to check for issues"
	@echo "  make fix                  Run golangci-lint to auto-fix fixable issues"
	@echo "  make spell                Run codespell to check for spelling errors"
	@echo "  make check                Run lint and spell checks"
	@echo
	@echo "Build:"
	@echo "  make clean                Remove built binaries and test files"
	@echo "  make build                Build binaries for current platform (fast)"
	@echo "  make release              Build binaries for all platforms"
	@echo
	@echo "Tests:"
	@echo "  make test                 Run all tests on linux"
	@echo "  make test-send            Run send/receive tests on linux"
	@echo "  make test-shell           Run shell/connect tests on linux"
	@echo "  make test-windows         Run Windows send/receive tests"
	@echo

## ------------------------- Develop -------------------------------------

setup:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.7.2
	sudo apt-get install lsof codespell
	go install github.com/goreleaser/goreleaser/v2@latest

fix:
	$$(go env GOPATH)/bin/golangci-lint run ./... --fix

lint:
	$$(go env GOPATH)/bin/golangci-lint run ./...

spell:
	codespell -I .codespellignore .

check: lint spell

##  ------------------------- Build  -------------------------------------

clean:
	@rm -rf dist
	@rm -rf test/
	@kill $$(lsof -t -i :20000) 2>/dev/null || true

# Build binaries for current platform using goreleaser (fast)
build: $(GO_FILES)
	@echo "=================================================================="
	@CGO_ENABLED=0 goreleaser build --snapshot --single-target --clean
	@echo "=================================================================="

# Build binaries for all platforms using goreleaser
release: $(GO_FILES)
	@echo "=================================================================="
	@goreleaser build --snapshot --clean
	@echo "=================================================================="

## ------------------------- Test ----------------------------------------

# Default to linux tests
test: test-send test-shell

test-send: test-send-primary test-send-secondary

test-send-%:
	@echo "=================================================================="
	@echo "Running $@"
	@EXECUTABLE=$(EXECUTABLE) bash $(SEND_BASH) "$*"
	@echo "=================================================================="

test-shell: test-shell-primary test-shell-secondary

test-shell-%:
	@echo "=================================================================="
	@echo "Running $@"
	@EXECUTABLE=$(EXECUTABLE) bash $(SHELL_BASH) "$*"
	@echo "=================================================================="

test-windows: test-windows-send

test-windows-send: test-windows-send-primary test-windows-send-secondary

test-windows-send-%:
	@echo "=================================================================="
	@echo "Running $@"
	@EXECUTABLE=$(EXECUTABLE) powershell -File $(SEND_PS1) -TestType "$*"
	@echo "=================================================================="

# Windows cannot run shell and we can't do cross-runner tests

.PHONY: help setup hooks fix lint spell check clean build release test test-send test-shell test-windows test-windows-send
