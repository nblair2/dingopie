include .github/versions.env

GO_FILES            = $(shell find . -name '*.go')
EXECUTABLE         ?= "$(shell pwd)/$(shell find dist/ -path "*$(shell go env GOOS)*$(shell go env GOARCH)*" -type f -name dingopie | head -n 1)"
GARBLE_SEED        ?= $(shell openssl rand -base64 8 | tr -d '=')
export GARBLE_SEED
SCRIPTS_DIR        := test/scripts
DIRECT_SEND_BASH   := $(SCRIPTS_DIR)/test-direct-send.bash
DIRECT_SHELL_BASH  := $(SCRIPTS_DIR)/test-direct-shell.bash
DIRECT_SEND_PS1    := $(SCRIPTS_DIR)/test-direct-send.ps1
INJECT_SEND_BASH   := $(SCRIPTS_DIR)/test-inject-send.bash
DOCKER_COMPOSE     := docker compose -f test/docker/docker-compose.yml
DOCKER_TEST_IMAGE  := dingopie-test:latest
DOCKER_TEST_DIR    := test/docker

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
	@echo "  make test-unit            Run Go unit tests"
	@echo "  make test-direct-send     Run direct send/receive tests on linux"
	@echo "  make test-direct-shell    Run direct shell/connect tests on linux"
	@echo "  make test-inject-send     Run inject send/receive tests on linux"
	@echo "  make test-windows         Run Windows direct send/receive tests"
	@echo
	@echo "Docker (inject testing apparatus):"
	@echo "  make docker-build         Build test docker image (no-op if present)"
	@echo "  make docker-rebuild       Force rebuild test docker image"
	@echo "  make docker-up            Start test docker containers"
	@echo "  make docker-down          Stop test docker containers"
	@echo

## ------------------------- Develop -------------------------------------

setup:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)
	go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)
	go install mvdan.cc/garble@$(GARBLE_VERSION)
	go install github.com/boumenot/gocover-cobertura@$(GOCOVERCOBERTURA_VERSION)
	pip install codespell==$(CODESPELL_VERSION)
	sudo apt-get install -y lsof docker.io docker-compose-plugin

fix:
	codespell -w .
	$$(go env GOPATH)/bin/golangci-lint run ./... --fix

lint:
	$$(go env GOPATH)/bin/golangci-lint run ./...

spell:
	codespell .

check: lint spell

##  ------------------------- Build  -------------------------------------

clean: docker-down
	@rm -rf dist/
	@rm -rf test/results
	@rm -rf coverage.out coverage.xml
	@kill $$(lsof -t -i :20000) 2>/dev/null || true

# Build binaries for current platform using goreleaser (fast)
build: $(GO_FILES)
	@echo "=================================================================="
	@echo "GARBLE_SEED=$(GARBLE_SEED)"
	@CGO_ENABLED=0 goreleaser build --snapshot --single-target --clean
	@echo "=================================================================="

# Build binaries for all platforms using goreleaser
release: $(GO_FILES)
	@echo "=================================================================="
	@echo "GARBLE_SEED=$(GARBLE_SEED)"
	@goreleaser build --snapshot --clean
	@echo "=================================================================="


## ------------------------- Test ----------------------------------------

test: test-unit test-direct test-inject

# Go unit tests
test-unit:
	@echo "=================================================================="
	@echo "Running $@"
	@go test -v -race -coverpkg=./... -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out
	@gocover-cobertura < coverage.out > coverage.xml
	@echo "=================================================================="

# Linux E2E tests
test-direct: test-direct-send test-direct-shell

test-direct-send: test-direct-send-primary test-direct-send-secondary

test-direct-send-%:
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

test-inject: docker-build test-inject-send

test-inject-send: test-inject-send-primary test-inject-send-secondary

test-inject-send-%: docker-build docker-down
	@echo "=================================================================="
	@echo "Running $@"
	@EXECUTABLE=$(EXECUTABLE) bash $(INJECT_SEND_BASH) "$*"
	@echo "=================================================================="

# Windows E2E tests
test-windows: test-windows-direct-send

test-windows-direct-send: test-windows-direct-send-primary test-windows-direct-send-secondary

test-windows-direct-send-%:
	@echo "=================================================================="
	@echo "Running $@"
	@EXECUTABLE='$(EXECUTABLE)' powershell -File $(DIRECT_SEND_PS1) -TestType "$*"
	@echo "=================================================================="

# Inject testing with docker containers
docker-build:
	@docker image inspect $(DOCKER_TEST_IMAGE) >/dev/null 2>&1 || \
	  docker build -t $(DOCKER_TEST_IMAGE) $(DOCKER_TEST_DIR)

docker-rebuild:
	docker rmi -f $(DOCKER_TEST_IMAGE) 2>/dev/null || true
	docker build -t $(DOCKER_TEST_IMAGE) $(DOCKER_TEST_DIR)

docker-up: docker-build
	@EXECUTABLE=$(EXECUTABLE) $(DOCKER_COMPOSE) up --detach

docker-down:
	@EXECUTABLE=$(EXECUTABLE) $(DOCKER_COMPOSE) down

docker-logs:
	@EXECUTABLE=$(EXECUTABLE) $(DOCKER_COMPOSE) logs --follow

.PHONY: help setup hooks fix lint spell check clean build release test test-unit test-direct-send test-direct-shell test-windows test-windows-send docker-build docker-rebuild docker-up docker-down docker-logs
