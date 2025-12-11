GOARCH  ?= amd64
BINDIR  ?= $(PWD)/bin
LDFLAGS ?= -ldflags="-s -w" -trimpath
GO_FILES := $(shell find . -name '*.go')

.PHONY: clean build test test-direct-server-send-client-receive hooks lint fix fmt help

# Show this help
help:
	@cat $(MAKEFILE_LIST) | docker run --rm -i xanders/make-help

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

test: clean build test-direct-server-send-client-receive

test-direct-server-send-client-receive:
	@echo "==> Starting test"
	@rm -rf test/
	@mkdir -p test
	@head -c $$(shuf -i 256-65535 -n 1) /dev/urandom > test/in.txt
	@head -c $$(shuf -i 8-128 -n 1) /dev/urandom | base64 | tr -d '/+=' > test/key.txt
	@echo "--> Starting server (in background)"
	@KEY=$$(cat test/key.txt); \
	$(BINDIR)/dingopie.bin server direct send --file test/in.txt --objects $$(shuf -i 4-60 -n 1) > test/server.log 2>&1 &
	@sleep 1
	@echo "--> Starting client"
	@KEY=$$(cat test/key.txt); \
	$(BINDIR)/dingopie.bin client direct receive --server-ip 127.0.0.1 --file test/out.txt --wait "$$(shuf -i 10-500 -n 1)ms"
	@sleep 1
	@kill $(lsof -t -i :20000) 2>/dev/null && echo "--> Server stopped by force (unexpected)" || echo "--> Server already stopped on its own (expected)"
	@echo "--> Server log:"
	@cat test/server.log
	@echo "--> Verifying outputs match"
	@if cmp -s test/in.txt test/out.txt; then echo "==> Test PASSED"; else echo "==> Test FAILED"; exit 1; fi
	@echo "--> Cleaning up"
	@rm -rf test/
	@echo "==> Test complete"

##
## ------------------------- Developer tools -----------------------------
##

# Setup tools for development
setup: hooks
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.7.2
	sudo apt-get install lsof

# Install repository provided git hooks
hooks:
	git config core.hooksPath .githooks

# Run golangci-lint to check for errors
lint:
	$$(go env GOPATH)/bin/golangci-lint run ./...

# Run golangci-lint to auto-fix fixable issues
fix:
	$$(go env GOPATH)/bin/golangci-lint run ./... --fix
