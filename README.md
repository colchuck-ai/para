# para

`para` is a single Go binary that manages a [PARA-method](https://fortelabs.com/blog/para/) tree of
projects, areas, resources, and archive on disk — plain directories and files, readable without the
tool and safe to commit to git.

The full behavior is specified in [`para-design-v4.md`](para-design-v4.md), which is normative.
[`docs/implementation-plan.md`](docs/implementation-plan.md) tracks build-out against that spec.

## Install

**Go toolchain (>= 1.24):**

```sh
go install github.com/colchuck-ai/para/cmd/para@latest
```

**Install script** (also builds from source; a prebuilt-binary fast path is planned):

```sh
curl -sSf https://raw.githubusercontent.com/colchuck-ai/para/main/scripts/install.sh | sh
```

Pass flags through the pipe with `sh -s --`, e.g. `sh -s -- --dir "$HOME/bin"`. Run
`scripts/install.sh --help` for the full option list, including `--ref`, `--dry-run`, and
`--uninstall`.

## Status

Under active development, phase by phase, against
[`docs/implementation-plan.md`](docs/implementation-plan.md). Not yet ready for use.

## Development

```sh
make test    # go test ./... -race -count=1 -tags para_testhooks
make lint    # go vet, gofumpt, golangci-lint
make build   # bin/para
make install # go install ./cmd/para
make cover   # coverage report
```
