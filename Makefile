GOFUMPT_VERSION := v0.7.0
GOLANGCI_LINT_VERSION := v1.62.2

.PHONY: test lint build install cover

test:
	go test ./... -race -count=1 -tags para_testhooks

lint:
	go vet ./...
	@out="$$(go run mvdan.cc/gofumpt@$(GOFUMPT_VERSION) -l .)"; \
	if [ -n "$$out" ]; then \
		echo "gofumpt -l found unformatted files:"; \
		echo "$$out"; \
		exit 1; \
	fi
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

build:
	go build -o bin/para ./cmd/para

install:
	go install ./cmd/para

cover:
	go test ./... -race -count=1 -tags para_testhooks -coverprofile=coverage.out
	go tool cover -func=coverage.out
