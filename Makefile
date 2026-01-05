GO_FILES            = $(shell find . -name '*.go')
EXECUTABLE         ?= "$(shell pwd)/$(shell find dist/ -path "*$(shell go env GOOS)*$(shell go env GOARCH)*" -type f -name dingopie | head -n 1)"
SCRIPTS_DIR        := test/scripts
DIRECT_SEND_BASH   := $(SCRIPTS_DIR)/test-direct-send.bash
DIRECT_SHELL_BASH  := $(SCRIPTS_DIR)/test-direct-shell.bash
DIRECT_SEND_PS1    := $(SCRIPTS_DIR)/test-direct-send.ps1

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
	@echo "  make test-direct-send     Run direct send/receive tests on linux"
	@echo "  make test-direct-shell    Run direct shell/connect tests on linux"
	@echo "  make test-windows         Run Windows direct send/receive tests"
	@echo
	@echo "Docker (inject testing apparatus):"
	@echo "  make docker-push          Build and push test docker image"
	@echo "  make docker-up            Start test docker containers"
	@echo "  make docker-down          Stop test docker containers"
	@echo

## ------------------------- Develop -------------------------------------

setup:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.7.2
	sudo apt-get install lsof codespell
	go install github.com/goreleaser/goreleaser/v2@latest
	# docker, compose, etc

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
	@rm -rf test/results
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

test: test-direct-send test-direct-shell

test-direct-send: test-direct-send-primary test-direct-send-secondary

test-direct-send-%:
	@echo $(EXECUTABLE)
	@echo "=================================================================="
	@echo "Running $@"
	@EXECUTABLE=$(EXECUTABLE) bash $(DIRECT_SEND_BASH) "$*"
	@echo "=================================================================="

test-direct-shell: test-direct-shell-primary test-direct-shell-secondary

test-direct-shell-%:
	@echo "=================================================================="
	@echo "Running $@"
	@EXECUTABLE=$(EXECUTABLE) bash $(DIRECT_SHELL_BASH) "$*"
	@echo "=================================================================="

test-windows: test-windows-send

test-windows-send: test-windows-send-primary test-windows-send-secondary

test-windows-send-%:
	@echo "=================================================================="
	@echo "Running $@"
	@EXECUTABLE='$(EXECUTABLE)' powershell -File $(DIRECT_SEND_PS1) -TestType "$*"
	@echo "=================================================================="

# Windows cannot run shell and we can't do cross-runner tests

# Inject testing with docker containers
docker-push:
	@echo "=================================================================="
	@IMAGE_TAG="${IMAGE_TAG:-latest}" bash test/scripts/build-container.bash "${IMAGE_TAG}"
	@echo "=================================================================="

docker-up:
	@EXECUTABLE=$(EXECUTABLE) docker compose -f test/docker/docker-compose.yml up -d

docker-down:
	@EXECUTABLE=$(EXECUTABLE) docker compose -f test/docker/docker-compose.yml down

.PHONY: help setup hooks fix lint spell check clean build release test test-direct-send test-direct-shell test-windows test-windows-send docker-push docker-up docker-down
