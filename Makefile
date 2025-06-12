# Makefile for the git-proxy project

# --- Variables ---

# Output directory for binaries
GOBIN := $(CURDIR)/bin

# Go commands
GO := go
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean

# Target binaries
GOPROXY_BIN := $(GOBIN)/git-proxy
LOGSERVER_BIN := $(GOBIN)/log-server

# List of symlinks to create for the git-proxy
GIT_SYMLINKS := git git-upload-pack git-receive-pack git-cat-file git-for-each-ref git-rev-parse git-log git-lfs

# --- Build Targets ---

# The default target, executed when you just run `make`
all: $(GOPROXY_BIN) $(LOGSERVER_BIN) symlinks

# Build the git-proxy binary
$(GOPROXY_BIN):
	@echo "==> Building git-proxy..."
	@mkdir -p $(GOBIN)
	$(GOBUILD) -o $@ ./cmd/git-proxy

# Build the log-server binary
$(LOGSERVER_BIN):
	@echo "==> Building log-server..."
	@mkdir -p $(GOBIN)
	$(GOBUILD) -o $@ ./cmd/log-server

# Create symlinks for common git commands
symlinks: $(GOPROXY_BIN)
	@echo "==> Creating git symlinks in $(GOBIN)..."
	@# Loop over the list of symlinks and create them inside the bin directory.
	@# The symlinks are relative, pointing to 'git-proxy' within the same directory.
	$(foreach link, $(GIT_SYMLINKS), ln -sf git-proxy $(GOBIN)/$(link);)
	@echo "    Done."

# --- Housekeeping ---

# Clean up build artifacts
clean:
	@echo "==> Cleaning up..."
	@rm -rf $(GOBIN)
	$(GOCLEAN)
	@echo "    Done."

# Phony targets are not files. This prevents `make` from getting confused if
# a file named 'all' or 'clean' exists.
.PHONY: all clean symlinks