.PHONY: build install uninstall run clean help test

BINARY_NAME=claude-move
INSTALL_PATH=$(HOME)/go/bin

help: ## Show this help message
	@echo "Claude Code Session Migration Tool"
	@echo ""
	@echo "Usage:"
	@echo "  make build      - Build the binary"
	@echo "  make install    - Build and install to ~/go/bin"
	@echo "  make uninstall  - Remove installed binary"
	@echo "  make run        - Run without installing"
	@echo "  make test       - Run in dry-run mode"
	@echo "  make clean      - Remove built binary"
	@echo ""

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .
	@echo "✓ Built: ./$(BINARY_NAME)"

install: build ## Build and install to ~/go/bin
	@echo "Installing to $(INSTALL_PATH)/$(BINARY_NAME)..."
	@mkdir -p $(INSTALL_PATH)
	@cp $(BINARY_NAME) $(INSTALL_PATH)/
	@chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✓ Installed: $(INSTALL_PATH)/$(BINARY_NAME)"
	@echo ""
	@echo "Usage:"
	@echo "  $(BINARY_NAME)                           # Interactive mode (current dir)"
	@echo "  $(BINARY_NAME) --from /old --to /new    # With arguments"

uninstall: ## Remove installed binary
	@echo "Removing $(INSTALL_PATH)/$(BINARY_NAME)..."
	@rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✓ Uninstalled"

run: build ## Run locally without installing
	./$(BINARY_NAME)

test: build ## Test with current directory
	@echo "Testing migration tool..."
	@echo "This will show you sessions from current directory"
	./$(BINARY_NAME) --from $(HOME) --to $(HOME)/claude-move-tool

clean: ## Remove built binary
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@echo "✓ Cleaned"

# Migration examples
move: install ## Example: Move current session to claude-move-tool
	$(BINARY_NAME) --from $(HOME) --to $(HOME)/claude-move-tool
