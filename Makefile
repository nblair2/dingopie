GOOS    ?= linux
GOARCH  ?= amd64
BINDIR  ?= bin
LDFLAGS ?= -ldflags="-s -w" -trimpath# To make it harder to reverse


ifeq ($(GOOS),windows)
	EXT = .exe
else
	EXT = .bin
endif

.PHONY: all clean forge-outstation forge-master #filter-outstation filter-master

all: forge-outstation forge-master #filter-outstation filter-master

forge-outstation:
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o "$(BINDIR)/dingopie-$@$(EXT)" ./forge/outstation/

forge-master:
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o "$(BINDIR)/dingopie-$@$(EXT)" ./forge/master/

clean:
	@rm -rf $(BINDIR)/*