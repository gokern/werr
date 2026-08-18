.DEFAULT_GOAL := help

GO_BIN ?= $(shell go env GOPATH)/bin

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

$(GO_BIN)/golangci-lint:
	@echo "==> Installing golangci-lint within "${GO_BIN}""
	@go install -v github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

.PHONY: lint
lint: $(GO_BIN)/golangci-lint ## Run linting on Go files
	@echo "==> Linting Go source files"
	@golangci-lint run -v --fix -c .golangci.yaml ./...

.PHONY: test
test: ## Run tests
	go test -race -v ./... -coverprofile ./coverage.txt

.PHONY: bench
bench: ## Run werr microbenchmarks
	go test ./... -bench . -benchtime 1s -timeout 0 -run=XXX -cpu 1 -benchmem

.PHONY: bench-full
bench-full: ## Run cross-library comparison benchmarks and render SVG charts
	cd benchmark && go test -bench '.' -benchtime 1s -cpu 1 -count 10 -run=XXX -benchmem -timeout 0 ./... > RESULTS.txt 2>&1
	@echo "==> wrote benchmark/RESULTS.txt"
	cd benchmark && go run ./cmd/bench-chart RESULTS.txt charts
	@echo "==> wrote benchmark/charts/*.svg"
