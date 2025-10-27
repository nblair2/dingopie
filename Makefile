GOARCH  ?= amd64
BINDIR  ?= bin
LDFLAGS ?= -ldflags="-s -w" -trimpath

.PHONY: build clean test test-forge hooks lint fix fmt help

# Show this help
help:
	@cat $(MAKEFILE_LIST) | docker run --rm -i xanders/make-help

##
##  ------------------------- Build and Clean ----------------------------
##

# Build
build: clean
	@for OS in linux windows; do \
	  EXT="$$([ "$$OS" = "windows" ] && echo "exe" || echo "bin")"; \
	  GOOS=$$OS GOARCH=$(GOARCH) go build $(LDFLAGS) -o "$(BINDIR)/dingopie.$$EXT" .; \
	done

# Clean
clean:
	@rm -rf $(BINDIR)


##
## ------------------------- Tests ---------------------------------------
##

# Test all (linux only)
test: test-forge

# Test forge mode (server to client on localhost)
test-forge: build
	@echo "==> Starting test"
	@rm -rf test/
	@mkdir -p test
	@head -c $$(shuf -i 256-65536 -n 1) /dev/urandom > test/in.txt
	@head -c $$(shuf -i 8-128 -n 1) /dev/urandom | base64 | tr -d '/+=' > test/key.txt
	@echo "--> Starting server (in background)"
	@KEY=$$(cat test/key.txt); \
	$(BINDIR)/dingopie.bin forge server --file test/in.txt --key "$$KEY" --objects $$(shuf -i 8-128 -n 1) > test/server.log 2>&1 &
	@echo "--> Starting client"
	@KEY=$$(cat test/key.txt); \
	$(BINDIR)/dingopie.bin forge client 127.0.0.1 --file test/out.txt --key "$$KEY" --wait 0.1
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

# Run golangci-lint to format code
fmt:
	@golangci-lint fmt ./...

