.PHONY: help install build test format lint check fix clean version patch minor major release-version bench test-coverage security

GO ?= go

help: ## Show available commands
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install dev tools (golangci-lint, goimports)
	$(GO) get -tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint
	$(GO) get -tool golang.org/x/tools/cmd/goimports

build: ## Verify the package compiles
	$(GO) build ./...

test: ## Run tests
	$(GO) test ./...

format: ## Format code and organize imports
	$(GO) tool goimports -w .
	$(GO) fmt ./...

lint: ## Run linters (golangci-lint)
	$(GO) tool golangci-lint run

check: lint test ## Run all checks (lint + test)

fix: ## Auto-fix all fixable issues
	$(GO) tool goimports -w .
	$(GO) fmt ./...
	$(GO) tool golangci-lint run --fix
	$(GO) mod tidy

clean: ## Clean build artifacts
	$(GO) clean ./...

# Version management
CURRENT_VERSION := $(shell git describe --tags --exact-match 2>/dev/null || git describe --tags 2>/dev/null || echo "v0.0.0")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

version: ## Show current version
	@echo "Current version: $(CURRENT_VERSION)"
	@echo "Git commit: $(GIT_COMMIT)"
	@echo "Git branch: $(GIT_BRANCH)"

release-version:
	@if [ -z "$(VERSION)" ]; then echo "VERSION is required"; exit 1; fi
	@echo "New version: $(VERSION)"
	@git tag -a "$(VERSION)" -m "Release $(VERSION)"
	@echo "Run 'git push --tags' to publish"

patch: ## Bump patch version and create tag
	@CURRENT_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	echo "Current version: $$CURRENT_TAG"; \
	if [ "$$CURRENT_TAG" = "v0.0.0" ]; then \
		NEW_VERSION="v0.0.1"; \
	else \
		PATCH=$$(echo $$CURRENT_TAG | sed 's/v[0-9]*\.[0-9]*\.\([0-9]*\)/\1/'); \
		MAJOR_MINOR=$$(echo $$CURRENT_TAG | sed 's/\(v[0-9]*\.[0-9]*\)\.[0-9]*/\1/'); \
		NEW_VERSION="$$MAJOR_MINOR.$$(expr $$PATCH + 1)"; \
	fi; \
	$(MAKE) release-version VERSION=$$NEW_VERSION

minor: ## Bump minor version and create tag
	@CURRENT_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	echo "Current version: $$CURRENT_TAG"; \
	if [ "$$CURRENT_TAG" = "v0.0.0" ]; then \
		NEW_VERSION="v0.1.0"; \
	else \
		MAJOR=$$(echo $$CURRENT_TAG | sed 's/v\([0-9]*\)\.[0-9]*\.[0-9]*/\1/'); \
		MINOR=$$(echo $$CURRENT_TAG | sed 's/v[0-9]*\.\([0-9]*\)\.[0-9]*/\1/'); \
		NEW_VERSION="v$$MAJOR.$$(expr $$MINOR + 1).0"; \
	fi; \
	$(MAKE) release-version VERSION=$$NEW_VERSION

major: ## Bump major version and create tag
	@CURRENT_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	echo "Current version: $$CURRENT_TAG"; \
	if [ "$$CURRENT_TAG" = "v0.0.0" ]; then \
		NEW_VERSION="v1.0.0"; \
	else \
		MAJOR=$$(echo $$CURRENT_TAG | sed 's/v\([0-9]*\)\.[0-9]*\.[0-9]*/\1/'); \
		NEW_VERSION="v$$(expr $$MAJOR + 1).0.0"; \
	fi; \
	$(MAKE) release-version VERSION=$$NEW_VERSION

# --- Repo-specific custom targets ---

bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./...

test-coverage: ## Run tests with HTML coverage report
	@mkdir -p build
	$(GO) test -race -coverprofile=build/coverage.out ./...
	$(GO) tool cover -html=build/coverage.out -o build/coverage.html
	@echo "Coverage report: build/coverage.html"

security: ## Run security scan (gosec)
	@which gosec > /dev/null || (echo "Installing gosec..." && $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec ./...
