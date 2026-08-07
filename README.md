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

## Scale

Measured against a generated tree of **2,480 entities** — 200 projects with their objectives and key
results, 600 areas, 400 resources, 20 skills, 60 archived things, and roughly 5,600 files — on an
Apple M-series laptop. Reproduce with `go test ./internal/scale/ -run TestScaleTimings -v`.

| | |
| --- | --- |
| `para rebuild` — cold, writes every projection | 1.4 s |
| `para rebuild` — idempotent, writes nothing | 0.46 s |
| `para doctor` — deep scan of the whole tree | 0.98 s |
| `para list` — whole tree, 2,420 rows | 0.33 s |
| `para list projects.<one>` — 6 rows | 0.001 s |
| any single mutation | **2–10 files, whatever the size of the tree** |

The last row is the one that matters and the only one asserted as a test rather than reported as a
number: write-through touches the subject's own files and, when containment changed, its parent's —
never a subtree, never a walk to the root. The same `para note` writes the same two files on a tree of
four entities and on a tree of 2,480.

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
