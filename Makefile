.PHONY: all build run test test-coverage lint vet fmt fmt-check arch debt \
        debt-guard debt-coverage mutation codecharta verify docs docs-serve docs-openapi-check \
        doc-drift hooks security vuln licenses release-dry help

BINARY_NAME := tempus
MODULE      := github.com/jobrunner/tempus
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME  := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS     := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

GO       := go
GOLINT   := golangci-lint
COVERAGE_DIR := coverage
MKDOCS   := uvx --with mkdocs-material mkdocs

all: verify build

## Build
build: ## Build the binary
	$(GO) build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/$(BINARY_NAME)

run: build ## Build and run
	./$(BINARY_NAME)

## Test
test: ## Run all tests
	$(GO) test ./...

test-coverage: ## Tests with coverage report
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=$(COVERAGE_DIR)/coverage.out

## Lint / format
lint: ## golangci-lint
	$(GOLINT) run --timeout=5m ./...

vet: ## go vet
	$(GO) vet ./...

fmt: ## Format
	$(GO) fmt ./...
	goimports -w -local $(MODULE) ./cmd ./internal

fmt-check: ## Check formatting without changing (CI/hook)
	@unformatted=$$(gofmt -l cmd internal); \
	if [ -n "$$unformatted" ]; then echo "not formatted:"; echo "$$unformatted"; exit 1; fi

## Architecture fitness: import boundaries + module hygiene
arch: ## depguard + gomodguard + go.mod tidiness
	$(GOLINT) run --enable-only depguard,gomodguard_v2 ./...
	$(GO) mod tidy -diff
	@echo "arch ok."

## Debt ratchets
debt: debt-guard debt-coverage ## Suppression budget + coverage floors

debt-guard: ## Fast grep-based ratchet (suppression budget, debt markers)
	@./scripts/debt-guard.sh

debt-coverage: ## Per-package coverage floors (own test run)
	@mkdir -p $(COVERAGE_DIR)
	@$(GO) test -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./... >/dev/null
	@./scripts/coverage-gate.sh $(COVERAGE_DIR)/coverage.out

mutation: ## Mutation testing (ubuntu only — gremlins panics on macOS)
	$(GO) install github.com/go-gremlins/gremlins/cmd/gremlins@v0.5.1
	@rc=0; \
	 gremlins unleash --threshold-efficacy 90 --threshold-mcover 95 ./internal/domain || rc=1; \
	 gremlins unleash --threshold-efficacy 77 --threshold-mcover 94 ./internal/application || rc=1; \
	 exit $$rc

# Prefer an already-installed ccsh; otherwise run it through npx, which needs no
# global install and no admin rights. Needs node and a JRE (ccsh is a JVM tool).
CCSH_VERSION ?= 1.143.0
CCSH ?= $(shell command -v ccsh 2>/dev/null || echo "npx --yes codecharta-analysis@$(CCSH_VERSION)")

codecharta: ## CodeCharta map (structure+complexity+coverage+git) -> tempus.cc.json.gz, then the ratchet gate (needs node+java)
	$(GO) install github.com/jandelgado/gcov2lcov@v1.1.1
	$(CCSH) unifiedparser . -fe=go -e='_test\.go,third_party,\.claude' -nc -o base.cc.json
	$(CCSH) gitlogparser repo-scan --repo-path=. --add-author --silent -nc -o git.cc.json
	@$(GO) test -coverprofile=coverage.out ./... || true; \
	 gobin=$$($(GO) env GOBIN); [ -n "$$gobin" ] || gobin=$$($(GO) env GOPATH)/bin; \
	 inputs="base.cc.json git.cc.json"; \
	 if [ -s coverage.out ]; then \
	   "$$gobin"/gcov2lcov -infile=coverage.out -outfile=coverage.info; \
	   $(CCSH) coverageimport coverage.info -f lcov -nc -o coverage.cc.json; \
	   inputs="$$inputs coverage.cc.json"; \
	 else echo "WARN: no coverage.out — map without coverage"; fi; \
	 $(CCSH) merge $$inputs -o tempus.cc.json.gz
	python3 scripts/codecharta-ratchet.py tempus.cc.json.gz .codecharta-ratchet.json
	@echo "-> tempus.cc.json.gz  (load in https://maibornwolff.github.io/codecharta/visualization/)"

## Canonical, non-mutating "is it green?" — mirror this in CI.
verify: fmt-check vet lint test arch debt-guard ## Authoritative green check
	@echo "Compile-check (go build ./...)…"
	@$(GO) build ./...
	@echo "verify passed."

## Security
security: vuln ## All security checks
vuln: ## Known vulnerabilities
	govulncheck ./...
licenses: ## Dependency license compliance
	go-licenses check ./cmd/$(BINARY_NAME) \
		--allowed_licenses=Apache-2.0,MIT,BSD-3-Clause,BSD-2-Clause,ISC,MPL-2.0 --ignore $(MODULE)

## Docs (Diátaxis, MkDocs Material). --strict fails on broken links/nav.
docs-openapi-check: ## Fail if the two OpenAPI spec copies have drifted
	@./scripts/openapi-mirror-check.sh

docs: docs-openapi-check ## Build docs strictly (runs OpenAPI mirror check first)
	$(MKDOCS) build --strict
docs-serve: ## Serve docs with live reload
	$(MKDOCS) serve
doc-drift: ## Doc-drift harness (if the doc-drift-check skill is present)
	@bash .claude/skills/doc-drift-check/scripts/check-doc-drift.sh

## Release (dry run; real releases go through release-please + goreleaser in CI)
release-dry: ## goreleaser snapshot
	goreleaser release --snapshot --clean

## Git hooks
hooks: ## Install the pre-commit hook (.githooks)
	git config core.hooksPath .githooks
	@chmod +x .githooks/pre-commit
	@echo "pre-commit hook active."

help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

include deploy/dev/dev.mk
