# para

`para` is a single Go binary that manages a [PARA-method](https://fortelabs.com/blog/para/) tree of
projects, areas, resources, and archive on disk — plain directories and files, readable without the
tool and safe to commit to git.

The full behavior is specified in [`para-design-v4.md`](para-design-v4.md), which is normative.
[`docs/implementation-plan.md`](docs/implementation-plan.md) tracks build-out against that spec.

## Install

**Install script** — downloads the prebuilt binary for your platform, verifies its SHA-256, and
falls back to building from source when no release matches. No Go toolchain needed on the fast path:

```sh
curl -sSf https://raw.githubusercontent.com/colchuck-ai/para/main/scripts/install.sh | sh
```

Pass flags through the pipe with `sh -s --`, e.g. `sh -s -- --dir "$HOME/bin"`. Run
`scripts/install.sh --help` for the full list, including `--ref` to pin a release, `--from-source`,
`--dry-run`, and `--uninstall`.

**Go toolchain (>= 1.24):**

```sh
go install github.com/colchuck-ai/para/cmd/para@latest
```

Prebuilt archives for linux, macOS, and Windows — amd64 and arm64 — are attached to each
[release](https://github.com/colchuck-ai/para/releases), with `checksums.txt` beside them.

**Shell completion** knows every locator in the tree, which is worth having given that a locator can
run six segments deep:

```sh
para completion zsh > "${fpath[1]}/_para"      # bash, zsh, fish, powershell
```

## A five-minute tour

Everything below is a real transcript. Your timestamps will differ and nothing else will: the clock
here was pinned to 2026-03-05T17:00Z in America/Los_Angeles, which is what the ages and deadlines
are relative to. Pinning it is a test-build affordance, not a flag — `PARA_NOW` and `PARA_TZ` are
read only by a binary built with `-tags para_testhooks`, so a released para always tells the time.

**Make a tree.** `init` writes the four buckets, the archive's mirror of the live three, and an
`AGENTS.md` at each of those places explaining what belongs there:

```console
$ para init brain
created  brain/projects/.para/state.toml
         …
         brain/.para/tree.toml
         …
         brain/README.md
         brain/AGENTS.md
         brain/ACTIVITY.md
         brain/.gitattributes

no CLAUDE.md, no .claude/ — enable with `para config set emit.claude true`
```

**Add something.** A thing is addressed by a locator — the path with dots for slashes — and every
command takes one and prints them the same way, so anything you read pastes into anything you type:

```console
$ cd brain
$ para add projects.acme-migration --name 'Acme migration' \
    --description 'Rebuild the consumer so it stops falling over under replay load.' \
    --status in-progress --priority high --due 2026-09-30 --tags consumer,kafka \
    --created 2026-01-05
added  projects.acme-migration  project

wrote  projects/acme-migration/.para/state.toml
       projects/acme-migration/.para/config.toml
       projects/acme-migration/objectives/.para/state.toml
       projects/acme-migration/objectives/.para/config.toml
       projects/acme-migration/README.md
       projects/acme-migration/ACTIVITY.md
       projects/acme-migration/objectives/README.md
       projects/acme-migration/objectives/ACTIVITY.md
       projects/.para/logs/20260305T170000Z.jsonl
       projects/ACTIVITY.md
```

Every mutation prints what it wrote, and a mutation touches a handful of files no matter how large
the tree is.

**Measure something.** A project may carry objectives, an objective may carry key results, and a key
result is a number with a baseline, a target, and a history:

```console
$ para add projects.acme-migration.objectives.q1-growth --name 'Grow signups' \
    --description 'Move the top of the funnel.' --status in-progress --created 2026-01-05
$ para add projects.acme-migration.objectives.q1-growth.key-results.signups \
    --name 'Weekly signups' --description 'Sign-ups per week.' \
    --type number --start 480 --target 2000 --due 2026-09-30 --created 2026-01-05
$ para measure projects.acme-migration.objectives.q1-growth.key-results.signups 880 --at 2026-02-10
$ para measure projects.acme-migration.objectives.q1-growth.key-results.signups 1120 \
    --at 2026-03-02 --note 'post-launch bump'
$ para note projects.acme-migration 'checked in with the ingest team'
```

`--created` is here because the transcripts below quote real ages and a real
pace, and a tree made today has neither. Left off, everything is created now.

**Read it back.** `show` prints one thing — what is stored, what is derived from it, and what is
underneath:

```console
$ para show projects.acme-migration
projects.acme-migration  project
Acme migration
  Rebuild the consumer so it stops falling over under replay load.

status       in-progress
priority     high
due          2026-09-30        in 209 days
tags         consumer, kafka
created      2026-01-05
attention    2026-03-05        today

objectives
  q1-growth  Grow signups    in-progress
    signups  Weekly signups  on-track
             480 → 1120 / 2000   progress 0.42   pace 1.91
```

`progress` and `pace` are computed, never stored: 0.42 of the way from the baseline to the target,
and running 1.91× the rate the deadline requires. `on-track` follows from the pace rather than from
anything anybody typed.

`list` is the same tree flattened, at any depth, with containers traversed through rather than
printed — two more things have been added since, an area and a resource, so that it has something
to flatten:

```console
$ para add areas.health --name Health --description 'Staying in one piece.' --created 2026-01-05
$ para add resources.rust --name Rust --description 'Notes on the language.' \
    --tags rust,reference --created 2026-01-05
$ para list --sort attention
areas.health                                                      area        —            59 days ago
projects.acme-migration.objectives.q1-growth                      objective   in-progress  59 days ago
resources.rust                                                    resource    —            59 days ago
projects.acme-migration.objectives.q1-growth.key-results.signups  key-result  on-track     3 days ago
projects.acme-migration                                           project     in-progress  today
showing 5 of 5
```

**Ask what needs attention.** `review` groups by *why*. The thresholds are configuration, and a
check nobody configures never fires:

```console
$ para add projects.website --name Website --description 'A refresh.' \
    --due 2026-02-20 --created 2026-02-01
$ para set projects.website --status blocked --note 'waiting on the brand review'
$ para config set area.stale-after 30
$ para config set project.stale-after 14
$ para review
stale (2)
  areas.health      59 days       area.stale-after 30
  projects.website  32 days       project.stale-after 14
blocked (1)
  projects.website  32 days
overdue (1)
  projects.website  13 days over  due 2026-02-20
```

**Look at what is on disk.** Nothing above is hidden in a database:

```console
$ cat projects/acme-migration/README.md
---
kind: "project"
locator: "projects.acme-migration"
name: "Acme migration"
description: "Rebuild the consumer so it stops falling over under replay load."
status: "in-progress"
priority: "high"
due: "2026-09-30"
tags: ["consumer", "kafka"]
created: "2026-01-05T08:00:00Z"
---

# Acme migration

Rebuild the consumer so it stops falling over under replay load.

$ cat projects/acme-migration/objectives/q1-growth/key-results/signups/MEASUREMENTS.csv
at,value,decimal,progress,note
2026-02-10T08:00:00Z,880,880.0000,0.2632,
2026-03-02T08:00:00Z,1120,1120.0000,0.4211,post-launch bump
```

The frontmatter is generated and the body below it is yours. `ACTIVITY.md` — a per-directory digest
of that directory's own history — and `MEASUREMENTS.csv` are generated whole.

**Skills route instructions to places.** A skill is a directory you author under `.agents/skills/`;
para renders its scope into a one-line rule file that an agent reads:

```console
$ para add skills.signups-report --name 'Signups report' \
    --description 'when asked for the weekly signups number' --scope projects.acme-migration
$ cat .agents/rules/para-signups-report.md
---
generated_from: "para-signups-report"
---
When working under `projects/acme-migration/`, use the **Signups report** skill (`.agents/skills/para-signups-report/SKILL.md`) when asked for the weekly signups number.
```

`para config set emit.claude true` adds the Claude Code surface on top of that: a `CLAUDE.md` that
imports the rules, and `.claude/skills/` mirroring the skills.

**Check the tree.** The truth is `.para/state.toml` and an append-only journal; everything else is
derived from them, so it can be checked and, when it drifts, rebuilt:

```console
$ para doctor
clean
$ para rebuild --dry-run
nothing to rewrite (18 places up to date)
```

That is also the answer to a merge conflict: resolve the truth files, run `para rebuild`, and never
hand-resolve a generated one.

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
make test          # go test ./... -race -count=1 -tags para_testhooks
make lint          # go vet, gofumpt, golangci-lint
make build         # bin/para
make install       # go install ./cmd/para
make cover         # coverage report
make fuzz          # every fuzz target, briefly (FUZZTIME=20s)
make release-check # validate .goreleaser.yaml
make snapshot      # build the whole release into dist/, publishing nothing
```
