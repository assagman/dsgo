.PHONY: all test test-race test-unit test-unit-with-race build fmt vet lint check check-lint check-eof verify clean install-hooks fmt-fix help

PACKAGES := $$(go list ./... | grep -v /examples | grep -v /scripts)

all: clean check check-lint test-race

# Fast unit testing (without race detector) - for development iterations
test-unit:
	@echo "Running unit tests (parallel, no race detector, coverage)..."
	@go test -parallel $$(nproc) -covermode=atomic -coverpkg=./... -coverprofile=coverage.out $(PACKAGES) || exit 1
	@printf "\nUnit Coverage: "
	@go tool cover -func=coverage.out | grep total | awk '{print $$3}'

# Unit testing with race detector - for CI and final testing
test-unit-with-race:
	@echo "Running unit tests (parallel, race detector + coverage)..."
	@go test -race -parallel $$(nproc) -covermode=atomic -coverpkg=./... -coverprofile=coverage.out $(PACKAGES) || exit 1
	@printf "\nUnit Coverage: "
	@go tool cover -func=coverage.out | grep total | awk '{print $$3}'

# Fast testing (unit only, no race) - for development iterations
test: test-unit

# Full testing (unit only, with race) - for CI and final validation
test-race: test-unit-with-race

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
	rm -f coverage.out
	go clean -cache -testcache

install-hooks:
	./scripts/install-hooks.sh

help:
	@printf "DSGo Makefile - Available targets:\n\n"
	@printf "Core workflows:\n"
	@printf "  make all                 - Clean, check, lint, test-race (full validation)\n"
	@printf "Testing:\n"
	@printf "  make test                - Fast testing: test-unit (no race)\n"
	@printf "  make test-race           - Full testing: test-unit-with-race\n"
	@printf "  make test-unit           - Fast unit tests (no race detector)\n"
	@printf "  make test-unit-with-race - Unit tests with race detector (CI/final)\n"
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
