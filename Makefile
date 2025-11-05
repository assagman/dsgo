.PHONY: test build fmt vet lint check check-eof verify clean all install-hooks test-matrix test-matrix-quick test-matrix-sample

PACKAGES := $$(go list ./... | grep -v /examples/ | grep -v /scripts)

all: clean check check-eof check-lint test

all-with-examples: clean check check-eof test test-matrix-quick

test:
	@echo "Running comprehensive tests (race detector + coverage)..."
	@go test -v -race -coverpkg=github.com/assagman/dsgo,./internal/...,./module/...,./providers/...,./logging/... -coverprofile=coverage.txt $(PACKAGES)
	@echo "\nTest coverage summary:"
	@go tool cover -func=coverage.txt | grep total

# Quick test: single model (default)
test-matrix-quick:
	@echo "\nRunning examples with single model (quick)..."
	@go run scripts/test_examples_matrix/main.go -n 1

# Sample test: N random models (usage: make test-matrix-sample N=3)
test-matrix-sample:
	@echo "\nRunning examples with $(N) random model(s)..."
	@go run scripts/test_examples_matrix/main.go -n $(N)

# Full test: all models (comprehensive)
test-matrix:
	@echo "Running comprehensive test matrix (all models) ❄︎"
	@go run scripts/test_examples_matrix/main.go -n 0

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
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run --timeout=5m

verify:
	go mod verify

check: verify fmt vet build

check-lint: check lint

check-eof:
	./scripts/check-eof.sh

clean:
	rm -f coverage.txt coverage.out
	rm -rf test_matrix_logs test_examples_logs
	go clean -testcache

install-hooks:
	./scripts/install-hooks.sh
