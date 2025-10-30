.PHONY: test build fmt vet lint check check-eof verify clean all install-hooks

PACKAGES := $$(go list ./... | grep -v /examples/ | grep -v /scripts)

all: clean check check-eof test test-examples

test:
	@echo "Running comprehensive tests (race detector + coverage)..."
	@go test -v -race -coverpkg=./... -coverprofile=coverage.txt $(PACKAGES)
	@echo "\nTest coverage summary:"
	@go tool cover -func=coverage.txt | grep total

test-examples:
	@echo "\nRunning example tests..."
	@go run scripts/test_examples.go

build:
	go build ./...

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
	go clean -cache -testcache

install-hooks:
	./scripts/install-hooks.sh
