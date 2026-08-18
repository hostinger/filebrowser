VERSION ?= $(shell git describe --tags --always --match=v* 2> /dev/null || cat $(CURDIR)/.version 2> /dev/null || echo v0)
VERSION_HASH = $(shell git rev-parse HEAD)
VERSION_TAG ?= $(VERSION)

MODULE = $(shell env GO111MODULE=on go list -m)

LDFLAGS += -X "$(MODULE)/version.Version=$(VERSION_TAG)" -X "$(MODULE)/version.CommitSHA=$(VERSION_HASH)"

go = GOGC=off go

goimports=goimports
$(goimports):
	@if ! command -v goimports >/dev/null 2>&1; then \
		$(go) install golang.org/x/tools/cmd/goimports@latest; \
	fi

golangci-lint=golangci-lint
$(golangci-lint):
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		$(go) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; \
	fi

## Build:

.PHONY: build
build: | build-frontend build-backend ## Build binary

.PHONY: build-frontend
build-frontend: ## Build frontend
	cd frontend && pnpm install --frozen-lockfile && pnpm run build

.PHONY: build-staging
build-staging: | build-frontend-staging build-backend ## Build binary with Phrase ICE embedded (staging only, never production)

.PHONY: build-frontend-staging
build-frontend-staging: ## Build frontend with Phrase In-Context Editor (staging only)
	cd frontend && pnpm install --frozen-lockfile && pnpm run build:staging

.PHONY: build-backend
build-backend: ## Build backend
	$(go) build -ldflags '$(LDFLAGS)' -o .

.PHONY: test
test: | test-frontend test-backend ## Run all tests

.PHONY: test-frontend
test-frontend: ## Run frontend tests
	cd frontend && pnpm install --frozen-lockfile && pnpm run typecheck

.PHONY: test-backend
test-backend: ## Run backend tests
	$(go) test -v ./...

.PHONY: lint
lint: lint-frontend lint-backend ## Run all linters

.PHONY: lint-frontend
lint-frontend: ## Run frontend linters
	cd frontend && pnpm install --frozen-lockfile && pnpm run lint

.PHONY: lint-backend
lint-backend: | $(golangci-lint) ## Run backend linters
	$(golangci-lint) run -v

.PHONY: lint-commits
lint-commits: $(commitlint) ## Run commit linters
	./scripts/commitlint.sh

fmt: $(goimports) ## Format source files
	$(goimports) -local $(MODULE) -w $$(find . -type f -name '*.go' -not -path "./vendor/*")

## Release:

define build_release_bin
mkdir -p bin dist
GO111MODULE=on GOOS=linux GOARCH=amd64 $(go) build -trimpath -ldflags '$(LDFLAGS)' -o bin/filebrowser-$(VERSION_TAG)
tar -C bin -czf "dist/filebrowser-$(VERSION_TAG).tar.gz" "filebrowser-$(VERSION_TAG)"
endef

.PHONY: build-release-bins
build-release-bins: ## Build staging and production release binaries
	$(MAKE) build-release-bin-staging
	$(MAKE) build-release-bin-prod

.PHONY: build-release-bin-staging
build-release-bin-staging: VERSION_TAG = $(VERSION)-staging
build-release-bin-staging: build-frontend-staging
	$(build_release_bin)

.PHONY: build-release-bin-prod
build-release-bin-prod: build-frontend
	$(build_release_bin)
