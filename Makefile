GOOS    ?= linux
GOARCH  ?= amd64
BINDIR  ?= bin
LDFLAGS ?= -ldflags="-s -w" -trimpath


ifeq ($(GOOS),windows)
	EXT = .exe
else
	EXT = .bin
endif

.PHONY: all forge forge-server forge-client clean clean-forge clean-forge-server clean-forge-client test test-forge

all: clean forge

forge: forge-server forge-client

forge-server forge-client:
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o "$(BINDIR)/$@$(EXT)" ./$@/

clean: clean-forge

clean-forge: clean-forge-server clean-forge-client

clean-forge-server:
	@rm -rf $(BINDIR)/forge-server*.*

clean-forge-client:
	@rm -rf $(BINDIR)/forge-client*.*

test: test-forge

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