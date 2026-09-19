BINARY   := ritual
PKG      := github.com/HarjjotSinghh/ritual
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -s -w \
	-X $(PKG)/internal/version.Version=$(VERSION) \
	-X $(PKG)/internal/version.Commit=$(COMMIT) \
	-X $(PKG)/internal/version.Date=$(DATE)

.DEFAULT_GOAL := build

.PHONY: build
build: ## Build the binary into ./dist
	@mkdir -p dist
	go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$(BINARY) ./cmd/$(BINARY)
	@echo "built dist/$(BINARY) $(VERSION)"

.PHONY: install
install: ## Install the binary into GOBIN
	go install -trimpath -ldflags '$(LDFLAGS)' ./cmd/$(BINARY)

.PHONY: test
test: ## Run the test suite
	go test ./...

.PHONY: race
race: ## Run the test suite under the race detector
	go test -race ./...

.PHONY: cover
cover: ## Run tests with coverage and print the summary
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: lint
lint: ## Vet, format check, and staticcheck when available
	go vet ./...
	@unformatted=$$(gofmt -l . | grep -v '^website/' || true); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

.PHONY: tidy
tidy: ## Tidy modules and verify nothing changed
	go mod tidy
	git diff --exit-code go.mod go.sum

.PHONY: snapshot
snapshot: ## Build a local multi-platform snapshot with goreleaser
	goreleaser release --snapshot --clean --skip=publish

.PHONY: clean
clean: ## Remove build output
	rm -rf dist coverage.out

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
