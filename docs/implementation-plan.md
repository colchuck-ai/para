# para — implementation plan

Target: a single Go binary, `para`, implementing `command-surface.md` in full.

This plan is organized into **14 phases**. Each phase is sized for one working session, states its own definition of done, and leaves the tree green (`go test ./... -race` passes, `para` builds and installs). Phases are ordered so that every phase's dependencies are already merged.

---

## 0. Ground rules

### 0.1 Test-driven, structurally

TDD is not an aspiration here; it is enforced by how the work is sequenced.

- **No production code without a failing test that demanded it.** Every task below is written as *test first, then implementation*.
- **The spec's §11 is the acceptance suite.** `command-surface.md` §11 contains ~250 worked commands. Those become executable conformance scripts (`testdata/script/*.txtar`) under [`rogpeppe/go-internal/testscript`][testscript]. At the start of each phase, transcribe that phase's §11 commands into a script file **before** writing any implementation. Commands not yet implemented are gated behind a `[para:phaseN]` testscript condition, flipped on when the phase lands. When every gate is on, the CLI is spec-complete by construction.
- **Two test altitudes, and both are mandatory:**
  - *Unit* — table-driven Go tests against pure functions. Phases 1–3 are almost entirely this. No filesystem, no clock, no I/O.
  - *Script* — `testscript` cases that run the real binary against a real temp tree, asserting stdout, stderr, exit code, and resulting files. Every user-visible behavior gets one.
- **Golden files** for anything with formatted output (`show`, `list`, `review`, `doctor`, `emit`, `--json`), regenerated with a `-update` flag, reviewed as diffs.
- **Red-green-refactor per commit.** A commit that adds behavior contains the test and the implementation. A commit that only adds a failing test is fine and expected at the start of a phase.
- CI runs `go test ./... -race -count=1`, `go vet`, `gofumpt -l`, and `golangci-lint`. All four gate merges.

[testscript]: https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript

### 0.2 The two decisions that make the whole thing testable

Both must land in Phase 0, before any domain code exists.

**Injected clock.** The spec is saturated with *now*: `created` defaults to now, future timestamps are rejected (§5.4), `elapsed` and therefore `pace` and derived status are functions of today (§6.2), and every review reason compares a stored timestamp against now (§7.4). Nothing is testable if `time.Now()` is called from library code.

- A `Clock` interface is threaded through the root command into every subsystem. Production is a real clock; tests inject a fixed instant.
- `testscript` sets `PARA_NOW=2026-03-02T14:40:00-08:00` (RFC3339) and `PARA_TZ`, which the binary honors **only** when built for tests or when an explicit build tag is set — production must not read `PARA_NOW`. (Simplest safe form: the env var is read in `cmd/para` behind a `//go:build para_testhooks` file, and script tests build with that tag.)
- Local wall-clock time with a recorded offset (§5.4, §10.4) means the zone matters. Fix it explicitly in tests; never rely on the host zone.

**Metadata tables, not hand-written commands.** §1.2, §1.3, and §1.4 are three matrices — noun × containment, noun × field, noun × verb. Encode each **as data**, and derive the command tree, the flag sets, the `set`/`unset` field validation, the sort-key validation, and the filter applicability from it.

The payoff is direct: "naming a field that does not exist on that noun is an error" (§1.3), "`--parent` is an error on project" (§3.2), "`--priority` and `--sort priority` do not apply to key-result" (§4.2), and "`--status` and `--overdue` apply only to nouns that have those fields" (§7.1) all become one lookup instead of forty-eight hand-maintained special cases. Hand-writing 6 nouns × 8 verbs of Cobra commands guarantees drift from the spec.

### 0.3 Package layout

```
cmd/para/                  thin main; version, clock wiring, exit codes
internal/
  paraerr/                 error taxonomy + exit-code mapping
  locator/                 parse/format, segments, buckets, reserved names
  nounmeta/                the §1.2/§1.3/§1.4 tables — the single source of truth
  ptime/                   log ids, progressive precision, offsets, comparison
  tagexpr/                 boolean tag expressions (§4.4)
  krvalue/                 number|ratio|boolean grammar, progress, pace, status (§6)
  mdfile/                  frontmatter+body codec with stable key order
  entityfile/              entity.md read/write
  logstore/                log dir read/write, ids, suffixes, invariants
  config/                  .para/config.toml
  tree/                    root discovery, the walk, locator resolution, placement rules
  projection/              log → the five/seven stored scalars (§10.3)
  mutate/                  the mutation engine: validate → log → reproject → write
  query/                   filter, sort, limit for list/search
  review/                  the four reasons and their ordering
  emit/                    driver interface + claude-code, agents-md, cursor
  doctor/                  the deep scan and its findings
  cli/                     Cobra tree generated from nounmeta; output + --json
scripts/install.sh
testdata/script/*.txtar
```

Module path: `github.com/colchuck-ai/para` (matches the remote). Binary at `cmd/para` so `go install github.com/colchuck-ai/para/cmd/para@latest` works.

### 0.4 Library choices

| Concern | Choice | Why |
| --- | --- | --- |
| CLI | `spf13/cobra` + `pflag` | Noun/verb tree maps directly; generated, not hand-written |
| YAML frontmatter | `gopkg.in/yaml.v3` for *reading*; hand-rolled ordered writer for *writing* | §10.3 shows a specific key order, and emit determinism + git diff churn (§8.4) demand byte-stable output. Marshaling a map reorders keys. |
| TOML | `pelletier/go-toml/v2` | Config is a flat dotted namespace; write sorted |
| Script tests | `rogpeppe/go-internal/testscript` | Runs the real binary against real trees |
| Golden files | plain `os.ReadFile` + `-update` flag | No dependency needed |

### 0.5 Error taxonomy and exit codes

Fixed in Phase 0, used everywhere after.

| Code | Meaning |
| --- | --- |
| `0` | Success. Includes `review` with findings (§7.4) and the no-op rule's `already in-progress; nothing written` (§3.3). Includes `doctor` clean. |
| `1` | Any command error, and `doctor` with at least one **error**-severity finding (§10.8) |
| `2` | `doctor` with only **advisory** findings (§10.8) |

Errors carry a kind so script tests can assert on them without matching prose.

---

## Phase 0 — Bootstrap, installability, harness

Nothing domain-specific. The goal is a repo where a test can be written on day one of Phase 1, and a binary the author can install and dogfood immediately.

**Tasks**

1. `go mod init github.com/colchuck-ai/para`; Go 1.24; `cmd/para/main.go` printing version only.
2. Version reporting: `para --version` prints version, commit, build date. Read from `runtime/debug.ReadBuildInfo()` so `go install ...@v1.2.3` reports correctly with no ldflags, overridable by `-ldflags -X main.version=...` for script builds. *Test first:* a script test asserting `para --version` output shape.
3. `internal/paraerr`: error kinds + `ExitCode(err) int`. *Test first:* table test over the mapping in §0.5.
4. `internal/clock`: `Clock` interface, `Fixed`, `System`. Test-hook env parsing behind the `para_testhooks` build tag.
5. `testscript` harness: `TestMain` building the binary with the test-hooks tag, a `testdata/script/` directory, custom conditions for phase gating, and custom commands as needed (e.g. `exists`, `cmpenv` come free). *Test first:* a trivial `version.txtar`.
6. Golden-file helper with `-update`.
7. `scripts/install.sh`:
   - `curl -fsSL https://raw.githubusercontent.com/colchuck-ai/para/main/scripts/install.sh | sh`
   - Flags/env: `--ref <branch|tag>` (default `main`), `--dir <install dir>` (default `$PARA_INSTALL_DIR`, else `$HOME/.local/bin`).
   - Requires a Go toolchain; fails with a clear message if absent. Fetches the ref (tarball via `codeload`, or `git clone --depth 1 --branch`), builds with `-ldflags` stamping version/commit/date from the ref, installs to the target dir, `chmod +x`, and warns if the dir is not on `PATH` with the exact line to add.
   - Idempotent; safe to re-run; `--dry-run` prints what it would do.
   - *Test first:* a shell test (or a Go test shelling out) that runs the script against a local checkout with `--ref` pointed at the working tree and asserts a working binary lands in a temp dir on `PATH`.
8. `Makefile` / `Taskfile`: `test`, `lint`, `build`, `install`, `cover`.
9. GitHub Actions: test matrix (linux/macos), lint, and a build check that `go install ./cmd/para` succeeds from a clean module cache.
10. `README.md` stub with both install paths documented.

**Done when** `go install github.com/colchuck-ai/para/cmd/para@main` and `scripts/install.sh` both produce a `para` on `PATH` that prints its version, and `go test ./... -race` is green in CI.

---

## Phase 1 — Locators, nouns, and the metadata tables

Pure code. No filesystem. This phase is where the spec's matrices become executable.

**Tasks**

1. `internal/locator`: parse and format. *Test first*, from §2:
   - Segment grammar: `[a-z0-9-]`, dots separate segments, hyphens separate words.
   - Reserved names `root`, `projects`, `areas`, `resources`, `.para` rejected as ids (§1.1, §10.8 `collision`).
   - Bucket-prefixed vs bare forms; `Qualify(noun, loc)` and `IsQualified(loc)`.
   - The two acceptance modes: noun commands accept either form; `path`, `review`, `doctor`, `emit`, `search --in` accept fully-qualified only (§2 table).
   - Log-entry locators: entity locator + a final timestamp segment (§2, §5.1).
   - Output always prints the fully-qualified form (§2).
2. `internal/nounmeta`:
   - `Noun` enum: project, area, objective, key-result, dir, skill (+ `config` handled separately, §1.2).
   - **Containment table** from §1.2: buckets per noun, `--parent` disposition (`none` / `optional` / `required`), the `Contains` set in both directions, `import` support, `log` support.
   - **Field table** from §1.3: for each (noun, field) — required-at-add / optional-with-default / absent; whether settable; whether unsettable; the value parser; whether it is a projection rather than a field.
   - **Verb table** from §1.4.
   - Derived predicates with their own tests: `CanParent(child, container)`, `IsContainer(noun)`, `FieldExists(noun, field)`, `IsSettable`, `IsUnsettable`, `SortKeys(noun)`, `FilterApplies(noun, filter)`.
   - *Test first:* a table test that walks §1.2/§1.3/§1.4 literally, cell by cell. Transcribe the tables into the test as data. This test is the spec.
3. Encode the specific carve-outs as named tests so they cannot regress:
   - `project add --parent` and `project unset parent` are errors (§1.2, §3.2).
   - `objective`/`key-result` `unset parent` is an error (§3.2).
   - `dir add`/`skill add` under an objective or key-result is an error (§1.2).
   - `key-result` has no `priority`; `--priority` and `--sort priority` are errors on it (§4.2).
   - `key-result.type` is fixed at add and never settable (§6.1).
   - `unset <locator> created` is an error (§1.3).
   - `skill` has `description`+`body` instead of `summary` (§1.3).

**Done when** the §1.2/§1.3/§1.4 transcription tests pass and no other package needs to know a noun's shape.

---

## Phase 2 — Value types: time, tags, key-result arithmetic

Still pure. Three independent value domains, all heavily specified and all trivially TDD-able.

**Tasks**

1. `internal/ptime` — log ids and instants (§5.1, §5.3, §5.4, §10.4):
   - Id format `2026-01-01T081502`, optional `-N` suffix. Parse, format, validate, filename-safety.
   - Progressive precision on `--at` and `log set id`: `2026-01-01`, `…T08`, `…T0815`, `…T081502`, zero-filled.
   - `created` takes the same progressive precision (§1.3).
   - Instant = filename + recorded `offset` (§10.4). Comparison is by instant, **not** by lexical id — ids are not monotonic across DST or travel (§5.4, §12).
   - Future timestamps rejected (§5.4). Requires the injected clock.
   - Suffix allocation: `max-existing + 1`, reusable after deletion, never renumbered (§5.3).
   - *Test first:* a DST-crossing case and a travel case that prove instant-ordering diverges from lexical-ordering, and that the code follows instants.
2. `internal/tagexpr` — the boolean expression language (§4.4):
   - Tokens: bare tags `[a-z0-9-]`, keywords `and`, `or`, `not`. Keywords are reserved and cannot be tags.
   - Precedence `not` > `and` > `or`. **No parentheses**, deliberately (§4.4, §12).
   - A bare comma list means `or`: `rust,reference` ≡ `"rust or reference"`.
   - Distinguish the *assignment* form (`--tags` on `add`/`import`/`set`, a plain comma list, §4.4) from the *filter* form (an expression). Two entry points, not one.
   - *Test first:* the DNF examples from §11 verbatim, plus error cases (`a and`, `not not a` if disallowed, a tag named `and`).
3. `internal/krvalue` — key-result values and derived quantities (§6):
   - Grammar per type: `number` (`42`), `ratio` (`90/11000`), `boolean` (`true`). Values are **stored as strings** so a ratio keeps its denominator (§10.3); arithmetic converts on demand.
   - A ratio's denominator belongs to the reading, not the key-result (§6.1).
   - `progress = (current − start) / (target − start)`; unclamped, so overshoot > 1 and regression < 0 (§6.2).
   - No measurements ⇒ `progress = 0` (§6.2).
   - `elapsed = (today − start-date) / (due − start-date)`; `pace = progress / elapsed`; pace **undefined** when `elapsed ≤ 0` (§6.2).
   - Derived status table (§6.3): `achieved` / `missed` / `at-risk` / `on-track`, plus `dropped` as the only settable one. **Does not latch** (§6.3, §12).
   - No `--due` ⇒ only `achieved` or `on-track`. `boolean` type ⇒ no pace; `achieved`/`missed`/`on-track` (§6.3).
   - Validation: `target` required; `target == start` rejected (§6.1); `boolean` requires `target true` and rejects `--start` (§6.1); `start` defaults to the first logged measurement (§6.1).
   - *Test first:* a matrix over (type × has-due × has-measurements × at-risk-after) asserting the derived status, including the non-latching regression case and the boolean lifecycle.

**Done when** all three packages are complete with no filesystem dependency, and the §6.3 status matrix test is exhaustive.

---

## Phase 3 — On-disk codecs

Read and write the three file formats, byte-stably.

**Tasks**

1. `internal/mdfile`: split/join frontmatter and body. **Ordered** writer — key order is fixed by the format, not by map iteration. Round-trip tests, CRLF handling, empty body, body containing `---`.
2. `internal/entityfile` (§10.3):
   - Fields in the order §10.3 shows: `noun`, `name`, `status`, `priority`, `due`, `tags`, `log-sla`, then the projections `created`, `updated`, `sla-reset-at`, `status-changed-at`; key-results add `type`, `start`, `target`, `current`, `measured-at`.
   - Summary is the **body**, not a frontmatter key (§10.3).
   - Absent key means unset and falls back to config (§10.3).
   - Key-result numeric fields are strings (§10.3).
   - Skills store only `noun`, `status`, `tags`, and the projection — `name`/`description`/`body` live in the sibling `SKILL.md`, which is authoritative (§10.5).
   - Atomic write: temp file + `rename(2)`, same directory.
   - *Test first:* golden files for one entity of each noun; a round-trip property test (parse → write → identical bytes).
3. `internal/logstore` codec half (§10.4):
   - Entry frontmatter: `offset`, `kind`, and per-kind `field`/`from`/`to` (change), `value` (measurement); `create` and `note` carry none. Body is the note.
   - A `change` with no `--note` has an empty body and still exists (§10.4).
   - *Test first:* golden per kind, plus round-trip.
4. `internal/config` (§9):
   - `.para/config.toml`. Dotted keys in three families: `<noun>.<field>`, `max-depth.<noun>`, `emit.<provider>.<field>`.
   - **Exactly eight `<noun>.<field>` keys are legal** (Decision 10): `log-sla` on project/area/objective/key-result, `blocked-after` on project/objective, `at-risk-pace` and `at-risk-after` on key-result. Validated against `nounmeta`, rejected at `config set` time with a message naming the eligible fields.
   - **Emit targets carry `enabled` and `tags` only** (Decision 4). Path and scope are driver constants, not config. Any other key under `emit.<provider>` is rejected at set time.
   - *Test first:* a table over all `<noun>.<field>` combinations asserting exactly eight are accepted.
   - Prefix matching on **whole dot-segments**: `--prefix project` matches `project.log-sla`; `--prefix key` matches nothing (§9).
   - `config unset emit.<provider>` removes a whole namespace — the one namespace-taking unset (§9).
   - Deterministic serialization (sorted).
   - *Test first:* the §11 `config` command list, exercised at the package level.

**Done when** every file the tool will ever write has a golden test and a round-trip test.

---

## Phase 4 — The tree: discovery, walk, resolution, placement

First phase with real filesystem behavior and the first end-to-end commands.

**Tasks**

1. Root discovery (§1.1): walk up looking for `.para/config.toml`; `$PARA_HOME` overrides the search. *Test first:* nested dirs, no tree found, `$PARA_HOME` pointing at a non-tree.
2. `para init` (§11): creates `projects/`, `areas/`, `resources/`, `.para/config.toml`. Takes an optional path. `--dry-run`. **Refuses inside an existing tree** (§1.1) — including when `$PARA_HOME` is set.
3. The walk (§10.6): read each entity's immediate subdirectories; descend only into those holding `.para/entity.md`; never recurse into adopted content. O(entities). One `entity.md` open per entity.
4. Locator resolution: locator → filesystem path, and path → locator (the locator is computed from the path; nothing stores it, §10.2). Bucket handling for the four namespaces including `root`.
5. Placement legality (§3.2, §1.2): does this container accept this noun? A container path matches at **any depth**; `--direct` restricts to immediate containment.
6. Depth gate (§3.2): `max-depth.area`, `max-depth.dir`, refusal at `add` naming the two mistakes and offering `--force`. **Depth is locator segments below the bucket, 1-based** (Decision 2) — every segment counts regardless of noun, so `projects.p.d1` is a depth-2 dir. Shipped defaults should exceed §11's example `3`. Same convention drives doctor's `depth` advisory (§10.8).
7. `para path <fully-qualified-locator>` — one bare line, shaped for `$(...)` (§3.6, §7.5).
8. `para config set|unset|list|show` wired to Phase 3's package, with `--json` on the reads.
9. First real script tests: `init.txtar`, `path.txtar`, `config.txtar` transcribed from §11.

**Done when** `para init && para config set project.log-sla 14 && para config list` works end to end, and the walk has a benchmark proving it does not open log directories.

---

## Phase 5 — The log store, the projection, and `rebuild`

The authoritative half of §10.3's bargain. Build this **before** any mutation command, so that mutations can only ever refresh the projection through the same code path `rebuild` uses.

**Tasks**

1. `internal/logstore` write half: append an entry, allocate the `-N` suffix, enforce id collision rules (§5.1, §5.3), rename for re-timing (§10.4), remove.
2. Invariants, each its own test (§5.2, §5.3):
   - Exactly one `create` entry per entity; `log remove` on it is an error.
   - The `create` entry must remain the **earliest** entry. `set created` is rejected unless the new timestamp is strictly before the oldest other entry — which is also why it can never collide and needs no `--dry-run` (§5.2).
   - Measurements must be unique in time; a duplicate timestamp is an error (§5.3).
   - Future timestamps rejected (§5.4).
3. `internal/projection` — **one function**, `Compute(entries) Projection`, producing (§10.3, §5.4):
   - `created` — the `create` entry's timestamp.
   - `updated` — newest entry of any kind.
   - `sla-reset-at` — newest resetting entry: a hand-written `note`, a `measurement`, or a `change` to `status`. Falls back to `created` when none (§5.4). A `create` entry does not reset it; it *is* the start (§3.3).
   - `status-changed-at` — newest `status` change.
   - Key-results add `current` and `measured-at` from the newest measurement, **plus a baseline key from the earliest measurement** (Decision 6) — required so progress on a key-result with no explicit `start` stays one file open.
   - A backdated entry never resets a clock, because it is not the newest (§5.4).
   - *Test first:* the resetting/non-resetting matrix from §3.3 — note ✓, measurement ✓, status change ✓, every other field change ✗, create ✗ — and the backdating case.
4. `para rebuild` (§10.7): recompute every projection from the log. Whole tree, one subtree, `--dry-run`. Writes nothing else, changes no content, **idempotent**.
   - *Test first:* a property test — for any tree, `rebuild; rebuild` is a no-op on the second run; and hand-corrupting a projection then rebuilding restores it exactly.

**Done when** `Compute` is the only place a projection is ever derived, and a fixture tree with hand-written logs rebuilds to the expected `entity.md` bytes.

---

## Phase 6 — The mutation engine, and `add` / `set` / `unset`

The core. Every mutation in the system flows through one pipeline.

**The pipeline** (§3.3): `resolve → validate against nounmeta → no-op check → write log entry → recompute projection via Phase 5 → atomically write entity.md`.

A crash between the log write and the `entity.md` write leaves a divergent projection — which is precisely the state `doctor` detects and `rebuild` repairs (§10.3). No transaction machinery is needed, and the plan should not invent any.

**Tasks**

1. `internal/mutate`: the pipeline above, generic over noun and field, driven by `nounmeta`.
2. **The no-op rule** (§3.3, §12), its own test file:
   - Setting a field to its current value writes nothing, resets no clock, exits `0` with `already in-progress; nothing written`.
   - Same for `unset` on an already-unset field, and `log set` to identical text.
   - **Not** applicable to re-measuring a key-result — logging `42` again at a new time is a fresh reading (§3.3).
3. **Creation writes exactly one entry** (§3.3): `add`/`import` write one `kind: create` whose timestamp is `created`; `--note` is its body. Initial field values are **not** logged as changes.
4. **Every field change writes a `change` entry** recording field, `from`, and `to` (§3.3). `--note` optional and supplies the body.
5. **`--note` is required when setting `status` to `blocked`** — the one exception (§3.3, §12).
6. `created` mutation (§1.3, §5.2): progressive precision, re-times the `create` entry (a file rename), bounded by the oldest other entry, rejected if in the future or past `due`. `unset created` is an error.
7. `add` for all six nouns, with per-noun required flags (§1.3):
   - all: `--name`; most: `--summary`; skill: `--name`, `--description`, `--body`; key-result: `--type`, `--target`.
   - `--parent` disposition per noun (§3.2), including omission placing at top of bucket / `root`.
   - Defaults per §1.3 and per-noun config fallback for the four SLA-family fields.
   - `skill add` writes `SKILL.md` **and** `.para/entity.md` (§10.5).
8. `set` / `unset` for every settable field, taking **bare field names** (§3.1), with `nounmeta` rejecting fields that do not exist on the noun.
9. `unset` semantics (§4.5): removes the explicit value; falls back to the config default if one exists, else to empty; empty means the check never fires.
   - **`unset key-result.start` reverts to the derived baseline** (Decision 6), erroring if that baseline would equal `target` (§6.1's zero-denominator rule).
10. Script tests: the full `set`/`unset`/`add` blocks of §11 for project, area, objective, key-result, dir, skill. This is the largest single transcription in the plan — budget for it.

**Done when** every `add`/`set`/`unset` line in §11 runs, and the log after each is exactly what §3.3 says it should be.

---

## Phase 7 — The `log` command family

`log` is a sub-noun with its own five verbs (§1.4), available on every noun.

**Tasks**

1. `log add` — `--note`, `--at` with progressive precision, defaulting to now (§5.1). Resets the SLA clock (§3.3).
2. `log add --value` on key-results — writes a `measurement`, uniqueness in time enforced, resets the SLA clock, updates `current`/`measured-at` (§5.3, §10.3). `--value` on a non-key-result is an error (and a `doctor` `log` finding if hand-written, §10.8).
3. `log set <locator> note <text>` — edits the body of any kind of entry, including a `change` (§11). Subject to the no-op rule.
4. `log set <locator> id <timestamp>` — re-timing is a rename (§5.1, §10.4). Progressive precision. Collision is an error. Re-timing changes ordering and therefore can change the projection — the pipeline recomputes.
   - **Bounded below by the create entry** (Decision 9) — the mirror of §5.2's bound on `set created`, which the spec states in only one direction. A target time at or before the create entry is rejected rather than producing the state §10.8's `log` finding calls an error.
   - **The create entry is re-timeable here too**, sharing `set created`'s validation including the `due` check, so the two spellings cannot diverge.
   - **A re-time that moves `current` prints an informational line**: `current is now 10/1000 (was 90/1000)`. No prompt; scripts unaffected.
   - *Test first:* re-time the newest measurement to before an older one and assert `current`, `progress`, and derived status all follow the log.
5. `log set <locator> value <v>` on measurements (§11).
6. `log remove` — error on the `create` entry (§5.2). Recomputes the projection, which can move `current` backwards.
7. `log show`, `log list` — `--limit`, `--json`, and the one descending default: `--sort at --reverse`. Naming `--sort at` explicitly drops the `--reverse` and gives oldest-first (§7.2, §12).
8. Ordering is by recorded **instant**, tie-broken by suffix (§5.4). Script test crossing a DST boundary.

**Done when** every `log` block in §11 runs, and re-timing an entry visibly moves `updated` in `entity.md`.

---

## Phase 8 — `import`, `remove`, and re-placement

The file-moving verbs, all of which take `--dry-run` (§3.4).

**Tasks**

1. `import` (§3.4, §1.2): **moves**. One `rename(2)` inside a tree; copy-then-unlink across devices. Source may be any readable path, inside the tree or out. Destination already existing is an error — **never merges** (§3.4).
   - Offered on project, area, dir, skill only (§1.2).
   - `--id` optional, defaulting to a slugified basename (§2).
   - `dir import` requires `--name` and `--summary`; `skill import` does not, because `SKILL.md` carries its own frontmatter and the flags are overrides (§1.3, §10.5).
   - **`skill import` errors when the source has no `SKILL.md`**, and errors if the post-override result leaves `name`, `description`, or `body` empty (Decision 3). The exemption from required flags is conditional on there being something to override; without it, a synthesized skill would be exactly the inert one §1.3 warns about, and `emit` would install it everywhere.
   - An imported `dir` gets a `.para/` written into it and nothing else (§10.5).
   - Writes one `create` entry; `--created` can backdate it (§1.3, §11).
   - Id colliding with an existing sibling is an error, **reported by `--dry-run` before anything moves** (§2).
2. `set <locator> id` and `set <locator> parent` — a single `rename(2)`; no descendant file is rewritten, because nothing stores a locator (§10.2). Both write `change` entries.
3. `unset <locator> parent` — promotes to top of bucket; for a dir or skill sitting directly in a bucket this moves it out of the bucket entirely, which is a **reclassification** and `--dry-run` must show it as one (§3.2).
4. `remove` (§3.5):
   - Takes the whole subtree: the entity, every nested entity, and every other file in the directory.
   - Interactive confirmation naming the **blast radius** (Decision 8) — descendant entity counts by noun, then content files, then log entries on their own line. "Other files" means content para never created, per §3.5's justification; the separate log count keeps history loss visible rather than buried in one number. `--force` skips it.

     ```
     Removing projects.my-project — "My Project"
       3 objectives, 7 key-results, 2 dirs, 1 skill
       14 other files
       41 log entries
     Remove? [y/N]
     ```
   - `--dry-run` prints the full list instead of the counts.
   - `--keep-files` removes only `.para/` directories throughout the subtree, leaving every other byte. Legal on objectives and key-results, where it leaves an empty directory (§3.5). **It prompts too**, with its own message leading with what survives, since it still destroys log history.

     ```
     Untracking resources.kafka-notes — "Kafka Notes"
       removes 1 .para directory (23 log entries), keeps 43 files
     Untrack? [y/N]
     ```
   - Takes no `--note` (§3.3).
5. Script tests: every `import`, `remove`, `set id`, `set parent`, `unset parent` line in §11, each in both `--dry-run` and real forms, with filesystem assertions on both.

**Done when** a `--dry-run` of each of these commands provably touches nothing (assert on directory mtimes/contents) and its output matches the real run's effect.

---

## Phase 9 — Reading: `show`, `list`, `search`

**Tasks**

1. **Effective status** (§4.1), the load-bearing computation: a thing's effective status is its own unless **any ancestor's is terminal**. Nothing is stored; it is computed from the path (§4.1, §10.2). `show` prints the entity's own status and says when it is dormant under a terminal ancestor. `--status` filters on effective status.
   - *Test first:* archive an area, assert every descendant is quieted; restore it, assert the previous picture returns exactly, including an independently-archived sub-area (§4.1).
2. **Terminal-status hiding**, one rule for every browsing read: `list`, `search`, and `review` hide terminal items by default and all take `--all`; `--status` filters. `show` and `doctor` are unaffected (§4.1).
   - `missed` is **not** terminal (§4.1, §12).
3. `list` (§7.1, §7.2): container path positional; `--tags` (filter expression), `--match`, `--status`, `--priority`, `--overdue`, `--direct`, `--all`, `--sort`, `--reverse`, `--limit`, `--json`.
   - `--status`, `--priority`, `--overdue` apply only to nouns with those fields (`nounmeta`).
   - `--match` searches name, summary, tags, **and log bodies**; on a skill, `description` and `body` stand in for `summary` (§7.1). This is the named exception to §10.6 (Decision 1) — it opens log directories and pays O(entries), while every structured read stays one open per entity.
   - **No container path means the noun's bucket(s)** (Decision 7), so `--direct` restricts to top-of-bucket and never errors. `para dir list --direct` returns dirs sitting directly in `root`, `projects`, `areas`, and `resources` — a query with no other phrasing. No-op for project, objective, and key-result.
4. `show` — the thing's own path positionally; symmetric with `list` (§3.1). For key-results, computes and displays progress, pace, and derived status at read time (§10.2).
5. Sorting (§7.2): keys on every noun are `id`, `name`, `created`, `updated`, both read from `entity.md`. Field-dependent keys are `due`, `priority`, `status`; key-results add `progress` and `pace`. Ascending by default; `--reverse` flips. Direction is never folded into a key. Default is `--sort id`.
   - Undefined `pace` sorts **last** and prints `—` (§6.2).
6. `--limit` truncation, with the count line `showing 20 of 143`, and `total`/`shown` in JSON (§3.6, §7.2).
7. `search` (§7.3): the cross-noun form of `list --match`. Same filters, plus `--in <fully-qualified-container>`. Results **group by noun**.
   - Cross-noun sort fallback per [ADR 0002](adr/0002-cross-noun-sort-fallback.md): each noun-group sorts independently, and a group whose noun lacks the sort key falls back to `id` for that group. A mixed-noun search never hard-errors on a sort key.
8. Stable `--json` schema for every read command, golden-tested.

**Done when** every `list`, `show`, and `search` line in §11 runs with golden output, and the effective-status cascade tests pass.

---

## Phase 10 — `review`

**Tasks**

1. The four reasons (§7.4), each with its own tests:
   - `--stale` — no resetting log entry within `log-sla`, measured from `sla-reset-at` (§5.4).
   - `--overdue` — open and past `due`.
   - `--at-risk` — a key-result whose pace is below `at-risk-pace`, only after `at-risk-after` days since the start date (§6.3).
   - `--blocked` — `status` has been `blocked` longer than `blocked-after`, measured from `status-changed-at`. **Nothing but leaving `blocked` resets it** — notes and status churn do not (§4.5, §7.4, §12).
2. Naming a reason restricts to it; naming none reports all four.
3. Terminal items excluded by default; `--all` includes them (§4.1).
4. Optional fully-qualified locator scopes to one subtree (§7.4).
5. Output grouped by reason, ordered within each group by **how far past the threshold** an item is. Takes `--limit` but **not** `--sort` (§7.4).
6. **Always exits 0** (§7.4, §12).
7. Assert the anti-gaming properties directly, since they are the reason these rules exist (§3.3, §12):
   - Pushing `due` out does not quiet `--stale`.
   - Tuning `log-sla` does not reset the clock it retimed.
   - `set status in-progress` on something already in-progress writes nothing and buys no silence.
   - Repeatedly annotating a blocked project keeps it in `--blocked`.

**Done when** every `review` line in §11 runs and the four anti-gaming tests pass.

---

## Phase 11 — `emit`

The phase with the most novel design surface. Everything here is deterministic by requirement (§8.4), which makes golden tests the natural form.

**Tasks**

1. Driver interface + a fixed provider enum. A provider para ships no driver for is a `doctor` `target` finding, never a config error at set time (§8.1, §9).
2. **Target config is `enabled` and `tags` only** (Decision 4). `tags` is a filter expression (§4.4). Path and scope are driver constants, not configuration — so a target is working the moment `enabled true` is set.
3. Drivers (§8.2), each with its path and scope fixed in code:
   - **`claude-code`** — `.claude/skills`, nested. Installs each skill once at its own position: `projects/my-project/.claude/skills/<id>/SKILL.md`. Claude Code resolves most-specific-wins itself, so the driver hands it the tree's shape. **Root scope is dropped** (Decision 4); nested is the mode that makes placement mean anything.
   - **`agents-md`** — `AGENTS.md`, nested. Nearest-only resolution, so each file carries the **union** of every active skill at or above its directory; on an id clash the **nearer** wins.
   - **`cursor`** — `.cursor/rules`, root. One directory at the tree root, each rule carrying a glob covering the subtree it came from.
4. `AGENTS.md` placement (§8.2): emitted at **every entity directory, plus the three buckets, plus the tree root** — not only where a skill exists. Unmanaged content directories then resolve correctly to their nearest managed ancestor.
5. File contents (§8.2): inlined skills plus a stable identity block — locator, name, summary, noun, child names. **Nothing volatile.** No status, progress, pace, or overdue snapshot; point at `para show <locator>` and `para review <locator>` instead.
6. Skills are **copied, not symlinked** (§8.2).
7. Archived skills are skipped, and the effective-status cascade means archiving any container disables every skill beneath it (§4.1).
8. Ownership and pruning (§8.3):
   - Whole files carry `generated-by: para` in frontmatter.
   - Regions in shared files are delimited by `<!-- para:begin -->` … `<!-- para:end -->`.
   - Emit rewrites its own regions and **prunes marked files it did not produce this run**. Anything unmarked is untouchable.
   - **The prune scan walks the full tree**, including adopted content, skipping `.git` (Decision 5). The cheaper managed-directories-only scan permanently leaks output from `remove --keep-files`, from entities hand-moved into content (§10.6's weakness), and from any future change to a driver's output path — the exact orphaning §8.3's markers-over-manifest argument promises to prevent.
   - No manifest, ever (§8.3, §12).
   - *Test first:* delete a skill, re-emit, assert its installed copy is gone; hand-write an unmarked `AGENTS.md`, re-emit, assert it is byte-identical afterward.
9. Determinism (§8.4): same tree, same bytes. *Test first:* emit twice into two temp trees from the same source and assert byte equality, including map-ordering hazards.
10. `para emit [<fully-qualified-locator>]` and `--dry-run`. Emit **never** runs as a side effect of a mutation (§8.4).
11. `para emit` with no targets enabled writes nothing and tells you how to add one (§8.1).

**Done when** the three drivers round-trip against golden trees, pruning is proven in both directions, and the double-emit byte-equality test passes.

---

## Phase 12 — `doctor`

Read-only, two severities, no `--fix` (§10.8, §12).

**Tasks**

1. The deep scan: walk **every** directory, including inside adopted content, ignoring the fast path's rules (§10.8).
2. Error findings (exit `1`), one test each (§10.8):
   - `orphan` — an `entity.md` the fast walk cannot reach.
   - `misplaced` — objective outside a project, key-result outside an objective, dir/skill under an objective or key-result, project below the top of `projects`, area outside `areas`.
   - `invalid` — unparseable `entity.md`, missing/unknown `noun`, missing required field, out-of-range enum, a key-result `type` not matching its `start`/`target` grammar.
   - `log` — illegal timestamp filename, unparseable frontmatter, future timestamp, missing or duplicated `create`, a `create` that is not earliest, a `value` on a non-key-result, two measurements at one instant.
   - `collision` — a reserved name used as an id.
   - `projection` — stored timestamps or `current` disagree with the log → prints `para rebuild`.
   - `stale` — emitted output differs from what `emit` would write now → prints `para emit`. Computed by **rendering and comparing**, never by writing, which is what keeps doctor read-only (§8.4). Uses the same full-tree scan as pruning, so orphaned marked output is reported rather than missed.
   - `target` — **narrowed** (Decision 4) to: unknown provider, an unknown key under `emit.<provider>`, or a malformed `tags` expression. "Missing a required field" and "a `scope` the driver rejects" are both unreachable now that only `enabled` and `tags` exist.
   - `config` — **new finding kind** (Decision 10): a hand-edited ineligible `<noun>.<field>` key, an unknown key family, or an unparseable value. Fills a category §10.8 left unowned, so a misspelled `project.log-sia` no longer sits silently dead in a tree doctor calls clean.
3. Advisory findings (exit `2` when nothing worse is present) (§10.8):
   - `untracked` — a directory with no `.para/entity.md` sitting **directly** in `projects/`, `areas/`, or `resources/`. Content inside a project is content and is never reported (§12).
   - `depth` — nesting past `max-depth.*` that arrived by `import` or by hand. Advisory, never a refusal, so adopting a deep repo works on day one (§3.2, §12).
4. Exit codes `0` / `1` / `2` exactly as §10.8 specifies, so CI can gate on `1` and ignore `2`.
5. Every finding prints the command that fixes it. There is no `--fix`.
6. `para doctor [<fully-qualified-locator>]`, `--json`.
7. *Test first:* a corpus of deliberately broken fixture trees, one per finding kind, asserting finding set and exit code. Plus a "clean tree exits 0" test run against the tree built by the Phase 6–8 script tests.

**Done when** each finding kind has a fixture that produces it and only it.

---

## Phase 13 — Release engineering and docs

**Tasks**

1. Harden `scripts/install.sh`: checksum verification where a release asset exists, `--ref` accepting tags and commit SHAs, clear failure when the Go toolchain is missing or too old, and an uninstall path.
2. Optional but recommended: GoReleaser for prebuilt binaries on tags, with the install script preferring a matching prebuilt asset and falling back to building from source. This removes the Go-toolchain requirement for most users.
3. Tagged releases; `para --version` reports the tag for `go install ...@vX.Y.Z`, the ref for script installs.
4. `README.md`: both install paths, a five-minute tour, and a pointer to `command-surface.md` as normative.
5. Shell completions (`para completion bash|zsh|fish`) — free from Cobra, and worth having given the locator grammar.
6. `man`/`--help` review: every command's help text agreeing with §11.

---

## Phase 14 — Conformance sweep and hardening

**Tasks**

1. Flip on every remaining `[para:phaseN]` gate; assert **every** command in §11 executes with the documented effect. A test that enumerates §11's fenced bash blocks and fails if any line is not covered by a script test.
2. Fuzz `tagexpr` and `ptime` parsers.
3. Property tests:
   - Any mutation sequence followed by `rebuild` is a no-op (the projection invariant, §10.3).
   - `emit; emit` is a no-op.
   - `doctor` is clean after any sequence of legal commands.
   - `import` then `remove --keep-files` returns the directory to its original contents.
4. Concurrency and merge behavior (§10.3, [ADR 0001](adr/0001-git-precondition-manual-merge-resolution.md)): simulate two branches each adding a log entry, merge, assert the log merges cleanly and `entity.md` conflicts; resolve by taking the later timestamp and assert `rebuild` repairs it. No merge driver ships — that is a settled decision.
5. Large-tree benchmark for the walk; assert log directories are never opened on a read path that does not need them.

---

## Resolved decisions

All ten open questions are settled. Each should be recorded as an ADR in `docs/adr/`, matching the existing two.

| # | Decision | Phase |
| --- | --- | --- |
| 1 | **`--match`/`search` do open log bodies.** §10.6's blanket claim narrows to: *no read opens the log directory unless it asked for log content.* Structured reads (the walk, `--sort updated`, `--sort pace`, `review --stale`, `review --blocked`) stay one-open-per-entity; `--match` and `search` are named exceptions paying O(entries), alongside `log list` and `rebuild`. | 9 |
| 2 | **Depth = locator segments below the bucket, 1-based.** `areas.a` is 1, `areas.a.b.c` is 3, `areas.a.b.c.d` is refused at `max-depth.area 3`. Every segment counts regardless of noun, so `projects.p.d1` is a depth-2 dir. This caps *total* tree depth rather than per-noun nesting — a dir inside a nested area spends budget the dir didn't create. Ship a default higher than §11's example `3`, since a dir in a project starts at 2. | 4 |
| 3 | **`skill import` requires a usable `SKILL.md`.** Error if the source has none; also error if, after `--name`/`--description`/`--body` overrides are applied, any of the three is still empty. Guarantees no imported skill is inert (§1.3), with `doctor`'s `invalid` finding catching later hand-edits. | 8 |
| 4 | **Emit config is `enabled` and `tags` only.** Path and scope are hard-coded per driver: `.claude/skills`, `.cursor/rules`, `AGENTS.md` at every managed directory; nested for `claude-code` and `agents-md`, root for `cursor`. **`claude-code`'s root-scope mode is dropped.** Any other key under `emit.<provider>` is rejected at `config set` time and is a doctor finding if hand-written. Config can open up later if a need appears. | 3, 11 |
| 5 | **Pruning scans the full tree**, including adopted content, skipping `.git`. Any file carrying the para marker that this run did not produce is pruned. The cheaper managed-directories-only scan permanently leaks output from `remove --keep-files` and hand-moved entities, which is exactly what §8.3's markers-over-manifest argument promises not to do. | 11 |
| 6 | **`unset key-result.start` is allowed**, reverting to the derived baseline; errors if the resulting baseline would equal `target` (§6.1). **The earliest measurement becomes a new projection key in `entity.md`**, computed by `projection.Compute`, repaired by `rebuild`, checked by doctor's `projection` finding — otherwise progress on a baseline-less key-result would require opening the log on every read, breaking §10.6. | 5, 6 |
| 7 | **`list` with no container path means the noun's bucket(s)**, so `--direct` restricts to top-of-bucket and never errors. `para dir list --direct` returns dirs sitting directly in `root`, `projects`, `areas`, and `resources` — a query with no other phrasing, since no locator spans all four. No-op for project, objective, and key-result, which are always immediate children. | 9 |
| 8 | **`remove` confirmation** names descendant entities by noun, then content files, then log entries on their own line. "Other files" means content para never created (§3.5's justification); the separate log count keeps history loss visible. `--keep-files` prompts too, with its own message leading with what survives. `--force` skips both. | 8 |
| 9 | **`log set <locator> id` is bounded below by the create entry** — the mirror of §5.2's bound on `set created`, which the spec states in only one direction. The create entry is re-timeable through `log set id` as well, sharing `set created`'s validation (including the `due` check) so the two spellings cannot diverge. Measurement uniqueness and the no-future rule still apply. A re-time that changes `current` prints an informational line: `current is now 10/1000 (was 90/1000)`. | 7 |
| 10 | **Ineligible config keys are rejected at `config set` time.** Exactly eight `<noun>.<field>` keys are legal: `log-sla` on project/area/objective/key-result, `blocked-after` on project/objective, `at-risk-pace` and `at-risk-after` on key-result. **A new `config` error finding is added to §10.8** covering hand-edited ineligible keys, unknown key families, unparseable values, and malformed emit keys — a category that currently has no owner, so a misspelled `project.log-sia` would otherwise sit silently dead in a tree doctor calls clean. | 3, 12 |

## Spec amendments required

`command-surface.md` is normative, so these resolutions need to land in it before or alongside the phases that implement them.

| § | Change |
| --- | --- |
| §8.1 | Target fields narrow from four to two: `enabled` and `tags`. Path and scope become driver-fixed. |
| §8.2 | Drop `claude-code`'s root-scope paragraph. Scope stops being a per-provider policy and becomes a driver fact. |
| §9 | Remove `emit.<provider>.path` and `.scope` from the examples; state the eight legal `<noun>.<field>` keys explicitly. |
| §10.3 | Add the earliest-measurement projection key to the key-result extras. |
| §10.6 | Narrow "the log directory is never opened on a read" to exempt `--match` and `search`. |
| §10.8 | Add the `config` error finding. Narrow `target` to unknown-provider, unknown-key, and malformed-`tags`. |
| §5.2 | State the lower bound on `log set id`, not just the upper bound on `set created`. |
| §6.1 | Note that `unset start` reverts to the derived baseline. |
| §3.2 | Pin the depth-counting convention and reconsider the example default. |
| §11 | Drop the five `emit.*.path` / `emit.*.scope` config lines. |

---

## Dependency graph

```
0 bootstrap
├─ 1 locators + nounmeta
│  └─ 2 ptime, tagexpr, krvalue
│     └─ 3 codecs
│        └─ 4 tree + init/path/config
│           └─ 5 logstore + projection + rebuild
│              └─ 6 mutation engine + add/set/unset
│                 ├─ 7 log commands
│                 └─ 8 import/remove/re-placement
│                    └─ 9 show/list/search
│                       ├─ 10 review
│                       └─ 11 emit
│                          └─ 12 doctor        (needs emit for `stale`)
13 release engineering  (independent after phase 0; do it whenever)
14 conformance sweep    (last)
```

Phases 7 and 8 are independent of each other and can be done in either order. Phases 10 and 11 are independent of each other. Phase 13 can be pulled forward at any point after Phase 0.

## Rough sizing

| Phase | Size | Notes |
| --- | --- | --- |
| 0 | M | Mostly plumbing, but the clock and harness decisions matter |
| 1 | M | Transcription-heavy, low risk |
| 2 | L | Three independent domains; could split if a session runs short |
| 3 | M | Codecs are mechanical once ordering is decided |
| 4 | M | First filesystem phase |
| 5 | L | The projection is load-bearing; do not rush it |
| 6 | **XL** | The largest phase. Consider splitting into 6a (engine + `add`) and 6b (`set`/`unset` across all nouns) |
| 7 | M | |
| 8 | L | Dry-run correctness doubles the test count |
| 9 | L | |
| 10 | M | |
| 11 | **XL** | Three drivers plus pruning plus determinism. Consider splitting per driver |
| 12 | L | Many findings, each small |
| 13 | S | |
| 14 | M | |
