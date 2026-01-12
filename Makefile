# Makefile for the eidolon project

# --- Variables ---

# Go commands
GO := go
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
INSTALL := install

# Target binaries
EIDOLON_BIN := eidolon
EIDOLON_SERVER_BIN := eidolon-server

# Installation paths
PREFIX ?= /usr/local
BINDIR := $(PREFIX)/bin

# --- Build Targets ---

# The default target, executed when you just run `make`
all: $(EIDOLON_BIN) $(EIDOLON_SERVER_BIN)

# Build the eidolon binary
$(EIDOLON_BIN):
	@echo "==> Building eidolon..."
	$(GOBUILD) -o $@ ./cmd/eidolon

# Build the eidolon-server binary
$(EIDOLON_SERVER_BIN):
	@echo "==> Building eidolon-server..."
	$(GOBUILD) -o $@ ./cmd/eidolon-server

# --- Installation ---

install: all
	@echo "==> Installing to $(BINDIR)..."
	@$(INSTALL) -d $(BINDIR)
	@$(INSTALL) -m 0755 $(EIDOLON_BIN) $(BINDIR)/$(EIDOLON_BIN)
	@$(INSTALL) -m 0755 $(EIDOLON_SERVER_BIN) $(BINDIR)/$(EIDOLON_SERVER_BIN)
	@echo "    Done."

uninstall:
	@echo "==> Uninstalling from $(BINDIR)..."
	@rm -f $(BINDIR)/$(EIDOLON_BIN)
	@rm -f $(BINDIR)/$(EIDOLON_SERVER_BIN)
	@echo "    Done."

# --- Housekeeping ---

# Clean up build artifacts
clean:
	@echo "==> Cleaning up..."
	@rm -f $(EIDOLON_BIN) $(EIDOLON_SERVER_BIN)
	$(GOCLEAN)
	@echo "    Done."

# Phony targets are not files. This prevents `make` from getting confused if
# a file named 'all' or 'clean' exists.
.PHONY: all clean install uninstall