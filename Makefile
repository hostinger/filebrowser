LDFLAGS += -X "$(MODULE)/version.Version=$(VERSION)" -X "$(MODULE)/version.CommitSHA=$(VERSION_HASH)"

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

.PHONY: build-release-bin
build-release-bin: build-frontend
	GO111MODULE=on GOOS=linux GOARCH=amd64 $(go) build -trimpath -ldflags '$(LDFLAGS)' -o bin/filebrowser-$(VERSION)
	tar -C bin -czf "dist/filebrowser-$(VERSION).tar.gz" "filebrowser-$(VERSION)"
