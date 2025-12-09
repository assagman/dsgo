.PHONY: all test integration-test build fmt vet lint check check-lint check-eof verify clean install-hooks test-matrix test-matrix-quick test-matrix-sample fmt-fix help integration-coverage

PACKAGES := $$(go list ./... | grep -v /examples | grep -v /scripts | grep -v /integration)
INTEGRATION_PACKAGES := $$(go list ./integration)

all: clean
	@$(MAKE) -j4 check check-lint test integration-test

test:
	@echo "Running unit tests (parallel, race detector + coverage, excluding integration tests)..."
	@go test -race -parallel $$(nproc) -covermode=atomic -coverpkg=./... -coverprofile=coverage.out $(PACKAGES) || exit 1
	@printf "\nCoverage: "
	@go tool cover -func=coverage.out | grep total | awk '{print $$3}'

integration-test:
	@echo "Running integration tests (parallel, race detector + coverage)..."
	@go test -race -parallel $$(nproc) -covermode=atomic -coverpkg=./internal/... -coverprofile=integration_coverage.out ./integration/... || exit 1
	@printf "\nIntegration Coverage (of main codebase): "
	@go tool cover -func=integration_coverage.out | grep total | awk '{print $$3}'

integration-coverage: integration-test

build:
	go build $(PACKAGES)

fmt:
	@FMT_FILES=$$(gofmt -s -l . 2>/dev/null || true); \
	if [ -n "$$FMT_FILES" ]; then \
		echo "The following files need formatting:"; \
		echo "$$FMT_FILES"; \
		exit 1; \
	fi

fmt-fix:
	gofmt -s -w $$(find . -name '*.go')

vet:
	go vet $(PACKAGES)

lint:
	@command -v golangci-lint >/dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run --timeout=5m

verify:
	go mod verify

check: verify fmt vet build check-eof

check-lint: check lint

check-eof:
	./scripts/check-eof.sh

clean:
	rm -f coverage.out integration_coverage.out
	go clean -testcache

install-hooks:
	./scripts/install-hooks.sh

help:
	@printf "DSGo Makefile - Available targets:\n\n"
	@printf "Core workflows:\n"
	@printf "  make all                 - Clean, check, lint, unit + integration tests (parallel, race, coverage)\n"
	@printf "Tests:\n"
	@printf "  make test                - Run all unit tests (parallel, race, coverage; excludes integration)\n"
	@printf "  make integration-test    - Run all integration tests (parallel, race, coverage)\n"
	@printf "  make integration-coverage - Alias for make integration-test\n"
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
	@printf "  make install-hooks       - Install git pre-commit hooks\n"
