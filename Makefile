GOOS    ?= linux
GOARCH  ?= amd64
EXT     ?= .bin
BINDIR  ?= bin
LDFLAGS ?= -ldflags="-s -w" -trimpath

.PHONY: all forge forge-server forge-client clean clean-forge clean-forge-server clean-forge-client test test-forge hooks lint fix fmt help

##
##  ------------------------- Build and Clean ----------------------------
##

# Build all binaries
all: clean forge

# Build binaries for forge mode
forge: forge-server forge-client

# Build individual binaries
forge-server forge-client:
	@for OS in linux windows; do \
	  EXT="$$([ "$$OS" = "windows" ] && echo ".exe" || echo ".bin")"; \
	  GOOS=$$OS GOARCH=$(GOARCH) go build $(LDFLAGS) -o "$(BINDIR)/$@$$EXT" ./$@/; \
	done

# Clean all binaries
clean: clean-forge

# Clean forge binaries
clean-forge: clean-forge-server clean-forge-client

# Clean forge server binary
clean-forge-server:
	@rm -rf $(BINDIR)/forge-server*.*

# Clean forge client binary
clean-forge-client:
	@rm -rf $(BINDIR)/forge-client.*

##
## ------------------------- Tests ---------------------------------------
##

# Test all (linux only)
test: test-forge

# Test forge mode (server to client on localhost)
test-forge: clean-forge forge
	@echo "==> Starting test"
	@rm -rf test/
	@mkdir -p test
	@head -c 1024 /dev/urandom > test/in.txt
	@head -c 128 /dev/urandom | base64 | tr -d '/+=' > test/key.txt
	@echo "--> Starting server (in background)"
	@KEY=$$(cat test/key.txt); \
	$(BINDIR)/forge-server$(EXT) --file test/in.txt --key "$$KEY" --objects 25 > test/server.log 2>&1 &
	@echo "--> Starting client"
	@KEY=$$(cat test/key.txt); \
	$(BINDIR)/forge-client$(EXT) 127.0.0.1 --file test/out.txt --key "$$KEY" --wait 0.1
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

# Install repository provided git hooks
hooks:
	git config core.hooksPath .githooks

# Run golangci-lint to check for errors
lint:
	@golangci-lint run ./...

# Run golangci-lint to try to fix issues
fix:
	@golangci-lint run ./... --fix

# Run golangci-lint to format code
fmt:
	@golangci-lint fmt ./...

# Show this help
help:
	@cat $(MAKEFILE_LIST) | docker run --rm -i xanders/make-help
