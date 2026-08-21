GOFUMPT_VERSION := v0.7.0
GOLANGCI_LINT_VERSION := v1.62.2

GORELEASER_VERSION := v2.12.5

# How long each fuzz target runs in `make fuzz`. Eleven targets, so the default
# is a CI-sized total rather than a search.
FUZZTIME ?= 20s

.PHONY: test lint build install cover fuzz release-check snapshot

# go test's default is 10 minutes *per package*, and cmd/para runs close to nine
# on a fast laptop: the crash matrix and the four properties both drive the real
# binary thousands of times, which is what makes them tests of the command
# surface rather than of the library. The first CI run blew the default on
# ubuntu — a slower box, not a hang — so the limit is stated rather than
# inherited. It is a backstop against a genuine deadlock, so it is generous
# without being absent.
TIMEOUT ?= 30m

# No -race: para is one command, one goroutine (production carries none outside
# a test-only sync.Once in the crash harness), and the crash matrix/property
# tests drive that binary as independent subprocesses under t.Parallel(), not
# shared in-process state. -race never found anything here and costs a ~3-4x
# wall-clock tax on every run — add it back if real concurrency shows up.
test:
	go test ./... -count=1 -timeout $(TIMEOUT) -tags para_testhooks

lint:
	go vet ./...
	# The test-hook build is a second compilation of every test file, and the
	# one the suite actually runs under (§0.2). Vetting only the untagged build
	# leaves the harness's own code unchecked.
	go vet -tags para_testhooks ./...
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
	go test ./... -count=1 -tags para_testhooks -coverprofile=coverage.out
	go tool cover -func=coverage.out

# Every fuzz target, briefly. `make test` runs their seed corpora, which is what
# catches a regression; this is the live search, and it is a loop because `go
# test -fuzz` takes one target at a time.
#
# The list is discovered rather than written down: a target nobody added here
# would be a target that never runs, and that failure is silent.
fuzz:
	@set -eu; \
	grep -rn '^func Fuzz' --include='*_test.go' . | while IFS= read -r line; do \
		file="$${line%%:*}"; \
		fn=$$(printf '%s' "$$line" | sed 's/.*func \(Fuzz[A-Za-z0-9_]*\).*/\1/'); \
		pkg="./$$(dirname "$$file")"; \
		echo "==> $$fn ($$pkg)"; \
		go test "$$pkg" -tags para_testhooks -run '^$$' -fuzz "^$$fn$$" -fuzztime $(FUZZTIME); \
	done

# The release configuration, checked without cutting a release. `snapshot`
# builds the whole artifact set into dist/ — the same names scripts/install.sh
# constructs its URLs from.
release-check:
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) check

snapshot:
	go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION) release --snapshot --clean --skip=publish
