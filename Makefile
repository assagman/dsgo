.PHONY: test test-cover test-race build fmt vet lint check check-eof verify clean all install-hooks

PACKAGES := $$(go list ./... | grep -v /examples/)

all: clean check test-race check-eof

test:
	go test $(PACKAGES)

test-cover:
	go test -v -cover $(PACKAGES)

test-race:
	go test -v -race -coverpkg=./... -coverprofile=coverage.out $(PACKAGES)

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
	rm -f coverage.out
	go clean -cache -testcache

install-hooks:
	./scripts/install-hooks.sh
