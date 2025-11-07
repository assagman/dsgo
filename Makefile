.PHONY: test build fmt vet lint check check-eof verify clean all install-hooks test-matrix test-matrix-quick test-matrix-sample fmt-fix check-lint all-with-examples help

PACKAGES := $$(go list ./... | grep -v /examples/ | grep -v /scripts)

all: clean check check-eof check-lint test

all-with-examples: clean check check-eof test test-matrix-quick

test:
	@echo "Running comprehensive tests (race detector + coverage)..."
	@go test -race -covermode=atomic -coverpkg=./... -coverprofile=coverage.out $(PACKAGES) || exit 1
	@printf "\nCoverage: "
	@go tool cover -func=coverage.out | grep total | awk '{print $$3}'

# Quick test: single model (default)
test-matrix-quick:
	@go run examples/test_matrix/main.go -n 1

# Sample test: N random models (usage: make test-matrix-sample N=3)
test-matrix-sample:
	@go run examples/test_matrix/main.go -n $(N)

# Full test: all models (comprehensive)
test-matrix:
	@go run examples/test_matrix/main.go -n 0

build:
	go build $(PACKAGES)

fmt:
	@FMT_FILES=$$(gofmt -s -l . 2>/dev/null | grep -v 'examples/' || true); \
	if [ -n "$$FMT_FILES" ]; then \
		echo "The following files need formatting:"; \
		echo "$$FMT_FILES"; \
		exit 1; \
	fi

fmt-fix:
	gofmt -s -w $$(find . -name '*.go' | grep -v /examples/)

vet:
	go vet $(PACKAGES)

lint:
	@command -v golangci-lint >/dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run --timeout=5m

verify:
	go mod verify

check: verify fmt vet build

check-lint: check lint

check-eof:
	./scripts/check-eof.sh

clean:
	rm -f coverage.out
	rm -rf test_matrix_logs test_examples_logs
	go clean -testcache

install-hooks:
	./scripts/install-hooks.sh

help:
	@printf "DSGo Makefile - Available targets:\n\n"
	@printf "Testing:\n"
	@printf "  make test                - Run all tests with race detector and coverage\n"
	@printf "  make test-matrix-quick   - Test examples with 1 model (fast)\n"
	@printf "  make test-matrix-sample N=3 - Test examples with N random models\n"
	@printf "  make test-matrix         - Test examples with all models (comprehensive)\n\n"
	@printf "Code Quality:\n"
	@printf "  make fmt                 - Check code formatting\n"
	@printf "  make fmt-fix             - Auto-fix code formatting\n"
	@printf "  make vet                 - Run go vet\n"
	@printf "  make lint                - Run golangci-lint\n"
	@printf "  make check               - Run verify, fmt, vet, and build\n"
	@printf "  make check-lint          - Run check + lint\n"
	@printf "  make check-eof           - Check files end with newline\n\n"
	@printf "Build:\n"
	@printf "  make build               - Build all packages\n"
	@printf "  make verify              - Verify go.mod dependencies\n\n"
	@printf "Maintenance:\n"
	@printf "  make clean               - Remove coverage files and test cache\n"
	@printf "  make install-hooks       - Install git pre-commit hooks\n\n"
	@printf "Common Workflows:\n"
	@printf "  make all                 - Clean, check, lint, and test\n"
	@printf "  make all-with-examples   - Above + test examples\n"
