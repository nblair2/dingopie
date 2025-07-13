GOOS    ?= linux
GOARCH  ?= amd64
BINDIR  ?= bin
LDFLAGS ?= -ldflags="-s -w" -trimpath# To make it harder to reverse


ifeq ($(GOOS),windows)
	EXT = .exe
else
	EXT = .bin
endif

.PHONY: all clean forge-server forge-client #filter-server filter-client

all: forge-server forge-client #filter-server filter-client

forge-server:
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o "$(BINDIR)/dingopie-$@$(EXT)" ./forge/server/

forge-client:
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o "$(BINDIR)/dingopie-$@$(EXT)" ./forge/client/

clean:
	@rm -rf $(BINDIR)/*