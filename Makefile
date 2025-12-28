GOARCH  ?= amd64
BINDIR  ?= $(PWD)/bin
LDFLAGS ?= -ldflags="-s -w" -trimpath
GO_FILES := $(shell find . -name '*.go')

# Show this help
help:
	@echo "Makefile commands:"
	@echo
	@echo "Build:"
	@echo "  make clean                Remove built binaries and test files"
	@echo "  make build                Build binaries"
	@echo
	@echo "Tests:"
	@echo "  make test                 Run all tests"
	@echo "  make test-send-primary    Test send/receive sending from client to server"
	@echo "  make test-send-secondary  Test send/receive sending from server to client"
	@echo "  make test-shell-primary   Test shell/connect with server shell"
	@echo "  make test-shell-secondary Test shell/connect with client shell"
	@echo
	@echo "Development tools:"
	@echo "  make setup                Setup development environment"
	@echo "  make hooks                Install git hooks"
	@echo "  make lint                 Run golangci-lint to check for issues"
	@echo "  make fix                  Run golangci-lint to auto-fix fixable issues"
	@echo "  make spell                Run codespell to check for spelling errors"
	@echo "  make check  			   Run lint and spell checks"
	@echo

##
##  ------------------------- Build and Clean ----------------------------
##

# Remove binaries
clean:
	@rm -rf $(BINDIR)
	@rm -rf test/
	@kill $$(lsof -t -i :20000) 2>/dev/null || true

# Build binaries
build: $(BINDIR)/dingopie.bin $(BINDIR)/dingopie.exe

# Build Linux binary
$(BINDIR)/dingopie.bin: $(GO_FILES)
	@mkdir -p $(BINDIR)
	@GOOS=linux GOARCH=$(GOARCH) go build $(LDFLAGS) -o "$@" .

# Build Windows binary
$(BINDIR)/dingopie.exe: $(GO_FILES)
	@mkdir -p $(BINDIR)
	@GOOS=windows GOARCH=$(GOARCH) go build $(LDFLAGS) -o "$@" .

##
## ------------------------- Tests ---------------------------------------
##

define test_send
	@echo "=================================================================="
	@echo "==> Starting test $@"
	@rm -rf test/
	@mkdir -p test
	@head -c $$(shuf -i 256-8192 -n 1) /dev/urandom | base64 > test/in.txt
	@head -c $$(shuf -i 8-128 -n 1) /dev/urandom | base64 > test/key.txt
	@echo "--> Starting server in background (mode: server direct $(1))"
	@KEY=$$(cat test/key.txt); \
	$(BINDIR)/dingopie.bin server direct $(1) --key $$KEY 2>&1 > test/server.log &
	@sleep 1
	@echo "--> Starting client (mode: client direct $(2))"
	@echo
	@KEY=$$(cat test/key.txt); \
	$(BINDIR)/dingopie.bin client direct $(2) --key $$KEY --server-ip 127.0.0.1 --wait "$$(shuf -i 10-500 -n 1)ms"
	@echo
	@sleep 1
	@kill $$(lsof -t -i :20000) 2>/dev/null && echo "--> Server stopped by force (unexpected)" || echo "--> Server already stopped on its own (expected)"
	@echo "--> Server log:"
	@echo 
	@cat test/server.log
	@echo
	@echo "--> Verifying outputs match"
	@if cmp -s test/in.txt test/out.txt; then echo "==> PASSED $@"; else echo "==> FAILED $@"; exit 1; fi
	@echo "--> Cleaning up"
	@rm -rf test/
	@echo "==> Complete $@"
	@echo "=================================================================="
	@sleep 1
endef

define test_shell
    @echo "=================================================================="
    @echo "==> Starting test $@"
    @rm -rf test/
    @mkdir -p test
	@head -c $$(shuf -i 256-8192 -n 1) /dev/urandom | base64 > test/in.txt
    @head -c $$(shuf -i 8-128 -n 1) /dev/urandom | base64 > test/key.txt
    @echo "--> Starting server in background ($(1))"
    @KEY=$$(cat test/key.txt); \
    $(1) --key $$KEY 2>&1 > test/server.log &
    @sleep 1
    @echo "--> Starting client ($(2))"
	@echo
    @KEY=$$(cat test/key.txt); \
    $(2) --key $$KEY --server-ip 127.0.0.1 2>&1 | tee test/client.log
	@echo
	@sleep 1
	@kill $$(lsof -t -i :20000) 2>/dev/null && echo "--> Server stopped by force (unexpected)" || echo "--> Server already stopped on its own (expected)"
	@echo "--> Server log:"
	@echo 
	@cat test/server.log
	@echo
    @echo "--> Verifying output"
    @if grep -q -f test/in.txt $(3); then echo "==> PASSED $@"; else echo "==> FAILED $@"; exit 1; fi
    @echo "--> Cleaning up"
    @rm -rf test/
    @echo "==> Complete $@"
    @echo "=================================================================="
    @sleep 1
endef

test: clean build test-send-primary test-send-secondary test-shell-primary test-shell-secondary

test-send-primary:
	$(call test_send,receive --file test/out.txt,send --file test/in.txt --objects $$(shuf -i 4-48 -n 1))

test-send-secondary:
	$(call test_send,send --file test/in.txt --objects $$(shuf -i 4-60 -n 1),receive --file test/out.txt)

test-shell-primary:
	$(call test_shell,$(BINDIR)/dingopie.bin server direct shell,echo "cat test/in.txt; exit;" | timeout 10s $(BINDIR)/dingopie.bin client direct connect,test/client.log)

test-shell-secondary:
	$(call test_shell,echo "cat test/in.txt; exit;" | timeout 10s $(BINDIR)/dingopie.bin server direct connect,$(BINDIR)/dingopie.bin client direct shell,test/server.log)

##
## ------------------------- Developer tools -----------------------------
##

setup: hooks
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.7.2
	sudo apt-get install lsof codespell

hooks:
	git config core.hooksPath .githooks

fix:
	$$(go env GOPATH)/bin/golangci-lint run ./... --fix

lint:
	$$(go env GOPATH)/bin/golangci-lint run ./...

spell:
	codespell -I .codespellignore .

check: lint spell


.PHONY: clean build test test-send-primary test-send-secondary test-shell-primary test-shell-secondary setup hooks fix lint spell check
