# para — implementation plan (v4)

Target: a single Go binary, `para`, implementing `para-design-v4.md` in full.

Supersedes the v2-era plan, which was built on `entity.md`, a projection of five scalars, and three
emit drivers — all three deleted by v4. What carries over unchanged is §0: the language, the
test-first sequencing, and the conformance harness. Section references below are to
`para-design-v4.md` unless stated otherwise.

**15 phases.** Each is sized for one working session, states its own definition of done, and leaves
the tree green (`go test ./... -race` passes, `para` builds and installs). Phases are ordered so every
phase's dependencies are already merged.

---

## 0. Ground rules

### 0.1 Test-driven, structurally

- **No production code without a failing test that demanded it.** Every task below is written as
  *test first, then implementation*.
- **§26 is the acceptance suite.** The design document says so outright, and its worked examples are
  transcribed into executable conformance scripts (`testdata/script/*.txtar`) under
  [`rogpeppe/go-internal/testscript`][testscript]. At the start of each phase, transcribe that phase's
  §26 commands **before** writing any implementation. Unimplemented commands sit behind a
  `[para:phaseN]` testscript condition, flipped on when the phase lands. When every gate is on, the
  CLI is spec-complete by construction.
- **Three test altitudes, all mandatory:**
  - *Unit* — table-driven tests against pure functions. Phases 1–3 are almost entirely this. No
    filesystem, no clock, no I/O.
  - *Renderer golden* — every projection renderer is a pure function from truth to bytes (§0.2), so
    each gets golden-file coverage with a `-update` flag, reviewed as diffs.
  - *Script* — `testscript` cases running the real binary against a real temp tree, asserting stdout,
    stderr, exit code, and the resulting file set.
- **Red-green-refactor per commit.** A commit that adds behavior contains the test and the
  implementation. A commit that only adds a failing test is fine and expected at the start of a phase.
- CI runs `go test ./... -race -count=1`, `go vet`, `gofumpt -l`, and `golangci-lint`. All four gate
  merges.

[testscript]: https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript

### 0.2 The three decisions that make the whole thing testable

All three land in Phase 0, before any domain code exists.

**Injected clock.** The design is saturated with *now*: `created` defaults to now and is rejected if
future (§15), `attention` is the newest qualifying event (§3.6), `elapsed` and therefore `pace` and
derived status are functions of today (§4.2), every `review` group compares a stored timestamp against
now (§20), and journal filenames are timestamps (§3.4). Nothing is testable if `time.Now()` is called
from library code.

- A `Clock` interface threads through the root command into every subsystem. Production is a real
  clock; tests inject a fixed instant.
- `testscript` sets `PARA_NOW` (RFC 3339) and `PARA_TZ`, honored **only** under a `para_testhooks`
  build tag — production must not read them. `--at`'s zero-filling happens in the *local* offset
  (§15.1) even though what gets stored and rendered is UTC (§3.5), so the zone still decides which
  instant a bare date means; fix it explicitly in tests and never rely on the host zone.

**Byte-stable rendering, and rendering as a pure function.** This is v4's load-bearing testability
decision, and it falls out of a spec requirement rather than taste: `doctor`'s `stale-projection` is
**full fidelity** (§10) — it re-derives every generated file in memory and compares byte for byte. So
every renderer must be a pure, deterministic function:

```go
type Renderer interface {
    Render(t Truth, j Journal, cfg Resolved) ([]byte, error)
}
```

Consequences to enforce from Phase 0:

- **No map iteration in output.** TOML *and* frontmatter writing need a fixed key order, not
  marshaling from a map. This is stricter than v2, where only frontmatter was rendered.
- **Round-trip identity**: parse → render must be a fixed point for every generated file, property-
  tested with a fuzz corpus. If it isn't, `doctor` reports drift on a clean tree.
- Renderers take no clock and no filesystem. Anything time-dependent is passed in.

**Truth-first write ordering.** A mutation touches four or five files (§2.3). Rather than reach for a
transaction, exploit a property the design already has: the tree has exactly one defined degraded
state — *truth is correct, projections are stale* — and that state already has a defined repair,
`doctor` + `rebuild` (§2.4, §19). So:

> **Write order within a mutation is: journal append → `state.toml` → projections, with the parent's
> journal and `ACTIVITY.md` last.** A crash at any point leaves the tree in the stale-projection
> state, never in a state where truth is lost or half-written.

Individual file writes are atomic (temp file in the same directory, `fsync`, `rename`). The journal
append is `O_APPEND` of a single line. Crash-consistency gets an explicit phase (14) that kills the
process between writes and asserts `doctor` reports exactly `stale-projection` and `rebuild` restores
cleanliness.

### 0.3 Package layout

```
cmd/para/                  thin main; version, clock wiring, exit codes
internal/
  paraerr/                 error taxonomy + exit-code mapping
  locator/                 locator↔path, segments, reserved words, skills.* mapping (§1.4)
  kindmeta/                kind-from-path (§1.3) and the §15 field matrix — one source of truth
  ptime/                   progressive precision, offsets, comparison, journal filenames
  tagexpr/                 boolean tag expressions (§17)
  krvalue/                 number|ratio|boolean grammar, progress, pace, derived status (§4)
  ptoml/                   byte-stable TOML writer; pelletier for reads
  mdfile/                  frontmatter+body codec; delimited-block codec for AGENTS.md
  csvfile/                 MEASUREMENTS.csv writer
  truth/                   state.toml, config.toml, tree.toml read/write
  journal/                 JSONL append, rotation, ordered read across files (§3)
  tree/                    root discovery via tree.toml, the walk, resolution, placement legality
  config/                  ancestor-chain resolution + printable chain (§7, §22)
  render/                  readme, activity, measurements, skill, rule, agents, claude, gitattributes
  mirror/                  .claude/skills symlink|copy modes (§6.1)
  writeset/                truth-first ordered write of a mutation's file set (§0.2)
  mutate/                  add/set/unset/note/measure/move/remove/archive/unarchive
  query/                   filter, sort, limit for list; activity rollup
  review/                  the five groups and their ordering (§20)
  rebuild/                 full re-derivation, --dry-run
  doctor/                  the deep scan and its findings (§10)
  cli/                     Cobra tree; output shapes, --json, exit codes
scripts/install.sh
testdata/script/*.txtar
```

Module path `github.com/colchuck-ai/para`; binary at `cmd/para` so
`go install github.com/colchuck-ai/para/cmd/para@latest` works.

**`kindmeta` is much smaller than v2's `nounmeta`**, and that is a saving the design bought. Kind
derives from the path (§1.3), so there is no noun×containment and no noun×verb matrix to encode — only
the field matrix (§15), from which `add`'s legal flags, `set`/`unset` validation, `--priority`
applicability, and `--sort` key validation all fall out as one lookup.

### 0.4 Library choices

| Concern | Choice | Why |
| --- | --- | --- |
| CLI | `spf13/cobra` + `pflag` | Verb-first with locators (§13) is a flat command set; field flags come from `kindmeta` |
| TOML | `pelletier/go-toml/v2` for reading; ordered writer for writing | §0.2 requires byte-stable output; marshaling a map reorders keys |
| Frontmatter | `gopkg.in/yaml.v3` for reading; ordered writer for writing | Same reason; §2.2 lists files whose frontmatter is compared byte for byte |
| JSONL | `encoding/json` with sorted keys | Journals are append-only and diffed by git; unstable key order would churn |
| CSV | `encoding/csv` | Fixed column order from §4.4 |
| Script tests | `rogpeppe/go-internal/testscript` | Runs the real binary against real trees |
| Golden files | `os.ReadFile` + `-update` | No dependency needed |
| Git | **none** | Para never invokes git (§9). `.gitattributes` is written as text like any other file. |

### 0.5 Error taxonomy and exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success. Includes `review` with findings (§20), `doctor` clean, and a no-op `set` (§15) |
| `1` | Any command error, and `doctor` with at least one error-severity finding (§21.2) |
| `2` | `doctor` with only advisory findings (§21.2) |

Errors carry a kind so script tests assert on the kind, not on prose.

---

## Phase 0 — Bootstrap, installability, harness

**Status: done** (`a1f9278`, branch `impl`). `go install`/`scripts/install.sh` both produce a working
`para --version`; `make test`, `make lint`, `make build` are green; CI runs the linux/macos matrix, lint,
and a clean-module-cache install check.

Nothing domain-specific. The goal is a repo where a test can be written on day one of Phase 1, and a
binary the author can install and dogfood immediately.

**Tasks**

1. `go mod init github.com/colchuck-ai/para`; Go 1.24; `cmd/para/main.go` printing version only.
2. Version from `runtime/debug.ReadBuildInfo()` so `go install ...@v1.2.3` reports correctly with no
   ldflags, overridable via `-ldflags -X`. *Test first:* a script test on `para --version` shape.
3. `internal/paraerr`: error kinds + `ExitCode(err) int`. *Test first:* table test over §0.5.
4. `internal/clock`: `Clock`, `Fixed`, `System`; test-hook env parsing behind `para_testhooks`.
5. `testscript` harness: `TestMain` building with the test-hooks tag, `testdata/script/`, phase-gate
   conditions. *Test first:* a trivial `version.txtar`.
6. Golden-file helper with `-update`, plus the round-trip property harness §0.2 requires.
7. `scripts/install.sh` — `curl … | sh`, `--ref`, `--dir`, `--dry-run`, idempotent, warns when the
   target dir is off `PATH`, clear failure when the Go toolchain is missing or too old, uninstall path.
   *Test first:* a test shelling out against a local checkout.
8. `Makefile`: `test`, `lint`, `build`, `install`, `cover`.
9. GitHub Actions: linux/macos matrix, lint, and a clean-module-cache `go install ./cmd/para` check.
10. `README.md` stub documenting both install paths.

**Done when** `go install …/cmd/para@main` and `scripts/install.sh` both produce a `para` on `PATH`
that prints its version, and CI is green.

---

## Phase 1 — Locators, kind derivation, reserved words

**Status: done** (branch `impl`). `internal/locator` and `internal/kindmeta` land as pure,
filesystem-free packages with full unit coverage.

Pure functions. No filesystem.

**Tasks**

1. `locator.Parse` / `.String()`: dot segments, charset `[a-z0-9-]`, no empty segments (§1.4).
2. Reserved words rejected as ids: `projects`, `areas`, `resources`, `archive`, `objectives`,
   `key-results`, `skills`, `logs`, `.para`, `.agents` (§1.4; the `collision` finding).
3. `locator ↔ relative path` — the isomorphism (§1.5), including both stated exceptions:
   - `skills.<id>` ↔ `.agents/skills/para-<id>/`, the `para-` prefix belonging to the directory and
     never to the locator;
   - archive stub segments, which are ids with no entity behind them (§1.6).
4. `kindmeta.KindOf(locator)` implementing §1.3, including illegal positions (depth 2 under
   `projects/`, a key-result outside a `key-results/` container).
5. The §15 field matrix as data, with `Has(kind, field)`, and the sort-key set derived from it.

**Done when** a table test covers every row of §1.3 and §15, the locator↔path round trip is
property-tested including both exceptions, and every §26 refusal about reserved words or illegal
nesting has a unit test.

---

## Phase 2 — Value types: time, tags, key-result arithmetic

**Status: done** (branch `impl`). `internal/ptime`, `internal/tagexpr`, and `internal/krvalue` land as
pure, filesystem-free packages with full unit coverage.

Still pure.

**Tasks**

1. `ptime`: progressive precision (§15.1) — date, +hour, +minute, +second, full RFC 3339; zero-fill in
   the local offset; a bare date is midnight local. Not-future and not-after-`due` bounds. Journal
   filename formatting.
2. `tagexpr`: word operators, precedence `not` → `and` → `or`, no parentheses, comma as `or`;
   `and`/`or`/`not` reserved as tags (§17).
3. `krvalue`: the three types and their grammars; `progress` unclamped; `pace`; every undefined-pace
   case; derived status and its non-latching rule (§4.1–4.3). Ratios stored as strings.

**Done when** every worked case in §4 and §17 is a unit test, ratio denominators survive round-trip as
written, and `progress` is verified negative below baseline and above 1 on overshoot.

---

## Phase 3 — Codecs, and the byte-stability guarantee

**Status: done** (branch `impl`). `internal/ptoml`, `internal/mdfile` (frontmatter+body codec and the
`AGENTS.md` delimited-block codec), `internal/csvfile`, and `internal/journal` land with full unit and
fuzz coverage.

The phase that makes §0.2 real.

**Tasks**

1. `ptoml`: ordered writer with a declared key order per file kind; reader via `pelletier`.
2. `mdfile`: frontmatter+body split, preserving the body byte for byte on rewrite; ordered frontmatter
   writer.
3. `mdfile` delimited-block codec for `AGENTS.md`: para owns between the markers and **never touches a
   byte outside them** (§6). Property test: arbitrary human prose outside the markers survives an
   arbitrary number of rewrites.
4. `csvfile`: the fixed `at,value,decimal,progress,note` column order (§4.4).
5. JSONL encode/decode for the four event kinds and their fields (§3.1), sorted keys, one object per
   line.

**Done when** every codec is a proven round-trip fixed point, the `AGENTS.md` body-preservation
property test passes, and a golden test proves two runs of every writer produce identical bytes.

---

## Phase 4 — The tree: discovery, walk, resolution, placement

**Status: done** (branch `impl`). `internal/tree` lands: root discovery via `.para/tree.toml` with
`$PARA_HOME` override, the §8.5 walk (container/entity classification, archive-stub recognition,
symlink safety, and a parallel scan of `.agents/skills/`), `Exists`/`ParentExists` placement-legality
checks, `.`-resolution, and the `para path` command.

First phase to touch a filesystem.

**Tasks**

1. Root discovery: walk up for `.para/tree.toml`; `$PARA_HOME` override; `init` refuses inside an
   existing tree (§1.1, §8.1). Include the `.agents/`-break case §8.1 names — resolution from inside a
   skill must still find the real root.
2. The walk (§8.5): immediate children, descend into those holding `.para/state.toml`, never into
   content. Container-vs-entity by reserved name (§1.2).
3. **Never follow para-owned symlinks** (§6.1, §21.2) — the rule that stops a mirrored skill from
   manufacturing a phantom entity.
4. Locator resolution against a real tree; placement legality (parent must exist; entity chains are
   entities all the way up, §1.5).
5. Stub recognition in `archive/` — a bare directory with no `.para/`, which `doctor` must never report
   as malformed (§1.6).
6. `.` resolution from `$PWD` (§14), and `para path`.

**Done when** script tests cover root discovery from an entity, from content, from inside a skill, and
from outside any tree; the walk ignores untracked directories entirely (§1.5); and a hand-planted
`state.toml` under content is invisible to the walk but found by the deep scan.

---

## Phase 5 — The journal, rotation, and the one clock

**Status: done** (branch `impl`). `internal/journal` gains append with rotation, ordered multi-file
reads, event constructors for the four kinds, measurement uniqueness-in-time, and `attention`
derivation on top of the codec landed in Phase 3.

**Tasks**

1. Append: `O_APPEND` single-line write into the newest file in `logs/`.
2. Rotation on `log.rotate-bytes` (default 4 MiB), each new file named for its own first event; closed
   files immutable (§3.4).
3. Ordered read across all of an entity's files, by `at` and never by position (§3.1).
4. Event construction for the four kinds, with `note` as a field on all of them (§3.1).
5. Measurement uniqueness in time — refuse and name the collision (§3.1, §15.1).
6. `attention` (§3.6): newest `note` or `measurement`, else `created`. Explicit tests that `change` and
   `child` events do **not** move it.

**Done when** a rotation test asserts filenames and immutability, an ordering test proves `at` beats
file position, and a table test walks every event kind against the clock rule.

---

## Phase 6 — The projection engine

**Status: done** (branch `impl`). `internal/truth` (state.toml + tree.toml codecs), `internal/render`
(the `Renderer` interface and all eight artifacts, with `ACTIVITY.md`'s two modes), and
`internal/writeset` (the truth-first ordered atomic write) land, plus `journal.ReadOnOrAfter` for the
cheap incremental read and a marker-parameterised delimited-block codec in `mdfile` for
`.gitattributes`.

Three decisions worth recording, because later phases inherit them:

- **`created` contributes a line to `ACTIVITY.md`**, so the file and `activity` agree by construction
  (§16.4) and §26's `created` line has a home. Its cost is a constraint on the write path:
  `render.CreatedDay` must be included in `ActivityRenderer.Days` when creating the file (`add`) or
  changing `created` (`set`), or the incremental path drops a line the full re-derivation produces.
- **`progress` follows §4.2's formula**, `(current − start) / (target − start)`. The worked lines in
  §3.5, §4.4, and §16.1 print `0.47` for values the formula puts at `0.235`; they were computed as
  `current ÷ target` with `start` dropped. The formula is normative and `krvalue` already implements
  it, so the examples' arithmetic is the thing that is wrong.
- **A skill owns `SKILL.md`, an `ACTIVITY.md`, and its derived rule.** Its identity file is
  `SKILL.md` rather than `README.md` — §2.2 gives it its own row for that — but it is otherwise an
  entity like any other. §5.1's file listing omits `ACTIVITY.md`, and that omission is not evidence:
  the listing is arguing why a skill cannot be generated from a TOML string, and its "everything
  except `SKILL.md`'s frontmatter is yours" already excludes the `.para/` beside it. What decides it
  is that §5.1 calls a skill "a real entity — it has state, config, and a journal", §3.6 defines its
  `attention` as its newest note, and `review --skills` measures `review.cadence` against exactly
  that: a skill's staleness is a feature, so the file answering "when did I last touch this" should
  exist. Omitting it would also buy `rebuild` and `doctor` a per-kind exception, which is what §8.4's
  uniform filenames exist to avoid. Its `SKILL.md` frontmatter stays `name` and `description` alone,
  since that file is read by agent harnesses with their own schema.
- **`ACTIVITY.md`'s measurement line carries no locator**: `- Measured 880/11000 (8.0%) — 24% of
  target.` §3.5's example shows `Measured **signups** at …`, but that example is a composite of three
  events that cannot share one journal (a `child` on a container, a `note`, a `measurement` on the
  key-result), and §26's `activity --recursive` output puts the locator in its own column. A locator
  inside the line would be duplicated there, and redundant in the file itself, which is by definition
  one entity's own fold.

**Timestamps are UTC throughout, and that was decided here.** §3.4 and §3.5 now say so, and three
things forced it. Journal filenames are compared as *strings*, so a name in the writer's own zone
makes "the newest file is the last one lexically" false the moment a tree crosses a zone or a DST
fall-back hour — and `Append` then reopens a closed file. `ACTIVITY.md`'s day sections stop
partitioning the timeline when grouped by each event's own offset, so a newest-day-first file can
present an earlier event above a later one, contradicting §3.1. And the file is committed: readers in
different zones view one rendered artifact, so the day boundaries must be ones they all agree on.
`--local` on the read commands (§16.2.1, Phase 10) is where the wall clock comes back, since terminal
output is neither compared nor committed.

The spine. Every later phase writes through it.

**Tasks**

1. The `Renderer` interface (§0.2) and renderers for README frontmatter, `ACTIVITY.md`,
   `MEASUREMENTS.csv`, `SKILL.md` frontmatter, derived rule files, `AGENTS.md` blocks, `CLAUDE.md`, and
   `.gitattributes`.
2. **`ACTIVITY.md` append semantics** (§3.5): newest-day-first; a mutation re-derives **only today's
   section** from the current journal file, and prior days are never recomputed. This is the one
   renderer that is not a whole-file function of the whole journal on the write path — it has two
   modes, incremental (write) and full (`rebuild`, `doctor`), and both must produce identical bytes for
   the same history. **That equivalence is this phase's central property test**, and it is what makes
   §10's dated drift report meaningful.
3. `writeset`: the truth-first ordered write (§0.2), atomic per file, with the parent's journal and
   `ACTIVITY.md` written last and only when containment changed (§3.3).
4. The write-through invariant as an enforced test: a mutation touches exactly one entity's files plus
   its parent's two, and **nothing walks a subtree or walks to root** (§2.3). Assert by counting
   filesystem writes.

**Done when** the incremental/full `ACTIVITY.md` equivalence property holds over generated histories
that span rotations, and the write-count test pins the invariant.

---

## Phase 7 — Config, and the printable chain

**Status: done** (branch `impl`). `internal/config` lands: §7's table as a closed key registry, a
byte-stable `config.toml` codec that preserves keys para does not recognise, chain resolution that
returns the winning value *and* every level consulted, and `para config set|unset|list|show` with
`--at`, `--prefix`, and `--json`. `ptoml.Document` gains `Keys()`, and `writeset` exports `WriteFile`.

Four decisions worth recording, because later phases inherit them:

- **The key set is closed; the file's key set is not.** `config set` refuses a key para does not
  read, because a `project.stale-aftr` that writes cleanly is a knob the user believes in and nothing
  ever consults. But a key already *in* a file survives every rewrite: it may belong to a newer para,
  and deleting it would make `config set` a data-loss operation. Reporting it is `doctor`'s `invalid`
  finding (§10). The one thing `Decode` refuses outright is a value it cannot carry — a TOML datetime,
  a mixed array — since accepting it would mean dropping it silently at the next write.
- **Only §7's four emit/log knobs have defaults.** The three thresholds (`<kind>.stale-after`,
  `key-result.at-risk-pace`, `review.cadence`) deliberately have none: §7 says "unset everywhere means
  the check never fires", so a default would make `review` fire on a tree that never asked it to.
  `Resolution.Found` is false for such a key, and that is the answer, not a missing case.
- **The built-in default is a level of the chain, printed as `(default)`.** §22's example only shows
  chains whose winner is a file, but the arrow marks the winner, and a value coming from nowhere with
  no arrow anywhere would leave `config show emit.claude-skills` unexplainable.
- **`config set` prints `wrote <file>`, though §26's first example shows it printing nothing.** §23's
  prose — "mutations print what they wrote, one line per file" — wins over the example, because a
  `set --at` that writes a file and says nothing is precisely the case where the user cannot tell
  whether `--at` landed where they meant.

Two slips in §22 found while transcribing it, both resolved in favour of the prose:

- §22's first code line comments `para config set project.stale-after 30 # in the nearest
  config.toml`, which its own next paragraph contradicts — "`set`/`unset` write the **root's**
  `config.toml` unless `--at <locator>` names a level" — as does §25's "**`config set` defaults to the
  root**". Root wins. "Nearest" is also the more surprising rule: it would make the file a `set`
  writes depend on the working directory.
- §26's `config set --at projects …` prints nothing; §23 says mutations print what they wrote. See
  above.

**Four codec defects the review found, all of the same shape: para rewriting a file it had misread,
leaving a `config.toml` no reader could parse and no `para config` command could repair.** Each is now
a test.

- A float that is not a number was written as Go spells it. `strconv.FormatFloat` gives `NaN`, which
  carries no decimal point, so the make-it-look-like-a-float fixup appended one: `NaN.0`, which is not
  TOML. `ptoml.FormatFloat` now emits TOML's `nan`/`inf`/`-inf`, and the thresholds refuse a
  non-finite value outright, since a threshold that is not a number is a check that can never fire.
- `"a.b" = 1` and `[a]` with `b = 2` are two different facts that flatten to one dotted string, so
  `Keys()` reported one name twice and a rewrite dropped a value and emitted a duplicate key.
  `ptoml.Document.Keys` now returns an error for any key outside TOML's bare-key charset — which also
  fixes the quieter version, a quoted key that read back fine and then failed to encode, stranding the
  whole level.
- `File.Set` appended blindly, so `emit = "x"` (a hand edit) plus `config set emit.claude true` wrote
  a value and a table under one name. It now refuses, naming the key it collides with.
- Float equality used `==`, so setting NaN over NaN reported a change on every run and `-0.0` over
  `0.0` reported none. The no-op rule asks whether the file's *bytes* would change, so floats now
  compare by their bits.

### Carry-forward obligations from Phase 7

- **Phase 8 owes the journal event for a config change.** §8.1 says the root's journal carries "init,
  config changes, and `child` events for the four buckets", and `config set` currently appends
  nothing. It was deferred rather than guessed at: `change` carries `field`/`from`/`to` (§3.1), which
  are §15 field names, and the `ACTIVITY.md` line template for a config change is prose §27 defers.
  Phase 8 owns both the `change` event and the write path that re-renders `ACTIVITY.md` with it.
- **Phase 13 owes the emit consequences of `config set emit.claude true`.** §26 shows the same command
  writing eight `CLAUDE.md` files and the `.claude/skills` mirror; this phase writes the config file
  alone. Until then the honest repair is `rebuild` (Phase 12).
- **Phase 8+ should resolve through `config.Resolver`, one per command.** It caches each `config.toml`
  it reads, which is what keeps a `list` over a few thousand entities from re-reading the root's
  config once per row. `RenderConfig` and `RotateBytes` are the two ready-made seams: the first fills
  `render.Config`, the second `writeset.Mutation.RotateBytes`.
- **Phase 10/11 read provenance from `Resolution.Source()`**, whose `Level.File` is the root-relative
  `…/.para/config.toml` path §16.1 prints. `config.StaleKey(kind)` is where §20's "a skill's threshold
  is `review.cadence`, not `stale-after`, and a container has none" is encoded, so no review group
  should re-derive it.

**Tasks**

1. `config.toml` read/write at any level; the root's alongside `tree.toml`.
2. Ancestor-chain resolution, nearest wins (§7), including the `.agents/` chain (skill → root, nothing
   in between).
3. The chain as a first-class value rather than a side effect: resolution returns the winning value
   **and every level consulted**, because §7 makes printability the condition of the design being
   defensible.
4. `para config set|unset|list|show`, with `--at <locator>` and the chain output shape from §22.

**Done when** the §22 worked example matches byte for byte, and a test asserts every resolved threshold
`show` reports (§16.1) names the level it came from.

---

## Phase 8 — Creation and field mutation

**Status: done** (branch `impl`). `internal/mutate` lands with `add`, `set`, `unset`, `note`, and
`measure` on one shared write path, plus the `config` change event Phase 7 deferred here.
`writeset.Mutation` is now a list of subjects, `journal.Append` reports the file its line landed in,
`kindmeta` gains §1.7's status vocabulary and the priority set, and `tree.KindAt` becomes the one
classifier the walk and every mutation share.

Six decisions worth recording, because later phases inherit them:

- **A projection is written only when its bytes differ from what is on disk.** Write-through stays
  complete — every projection the subject owns is re-derived on every mutation — but a file whose
  bytes did not change is a file nothing wrote. That is what makes §23's "mutations print what they
  wrote" honest, and it settles §2.3's prose, which names `state.toml` and `README.md` among what a
  `measure` rewrites: a key-result stores no `current`, `progress`, or derived status (§2.5), so
  there is nothing in either file for a measurement to change. Rewriting them with identical bytes
  would churn two mtimes and print two lines that were not true.
- **A mutation with more than one subject writes every subject's truth before any subject's
  projections.** `add` is the one such mutation — a project and its eager `objectives/` (§18.1) —
  and interleaving per subject would let a crash leave the container missing altogether. That is not
  the one degraded state the design defines a repair for: `rebuild` regenerates projections from
  truth and cannot invent truth that was never written.
- **`ACTIVITY.md` falls back to full mode in three cases**, and this is where §3.5's cheap write is
  actually made safe: a key-result (whose measurement lines depend on `start`, `target`, and the
  oldest reading), a mutation that moves `created` (whose line lives in whichever day section
  `created` names), and a file that does not exist yet (which has no prior days to splice onto, so
  incremental mode would silently drop every day before today). The equivalence of the two modes on
  the *write path* is now a test that replays six mutations across six days and re-derives in full
  after each one.
- **`created` is stored as a UTC RFC 3339 instant; `due` is stored exactly as typed.** §15.1's
  store-as-UTC rule is about *instants* — something happened, and every reader must agree when — and
  it names only `note`, `measure`, and `created`. A deadline is a date somebody chose, and §8.3's two
  truth files write `due = "2026-09-30"` while §16.1 prints it back the same way. Resolving it in the
  typist's offset would make the stored value depend on where they were sitting (`2026-09-30T07:00:00Z`
  in summer, `T08:00:00Z` in winter) for a deadline that meant neither; a date on disk already means
  the same day to every reader, which is what §3.5 was asking for. `due` takes §15.1's grammar for
  validation and neither of §15's bounds directly. `created ≤ due` is checked from both sides, and a
  bare `due` date counts as the whole day — so a deadline of today is legal for something created
  today, which a midnight reading would have refused.
- **Every timestamp para writes is truncated to the second.** §3.1's journal lines, §3.4's filenames,
  §4.4's CSV column, and §8.3's truth files all stop there. Carrying the wall clock's microseconds
  would make two readings a millisecond apart *distinct* to §3.1's exact-instant uniqueness check and
  *identical* in the `MEASUREMENTS.csv` rows they produce — one duplicate row no `rebuild` could
  remove, because both events are genuinely there.
- **§1.7's default status is stored at creation, not defaulted at read time.** A project has a status
  from the moment it exists, `state.toml` is where what a thing *is* lives (§2.2), and a generated
  README frontmatter that omitted it would misinform every reader of the file. §2.5's ban covers what
  is *derived*; a default is chosen, not derived.
- **A config change lands in the journal of the level whose `config.toml` changed**, as a `change`
  event whose `field` is the config key. §8.1 describes the root's case and §3.2 is the general rule
  it is an instance of. A config key always contains a dot and a §15 field name never does, so the
  two namespaces cannot collide, and `ACTIVITY.md`'s existing change line renders it without a
  template of its own — which is why §27's deferred prose turned out not to be needed. Only
  `ACTIVITY.md` is re-rendered: a config change alters no state, and the emit knobs' whole-tree
  consequences stay `rebuild`'s (Phase 13).

**One spec gap closed here, and it goes beyond the letter of the phase.** `priority` is now a closed,
ordered set — `high`, `medium`, `low`. The design never enumerates it (§8.3's example writes `high`),
so this is a decision rather than an implementation. It is taken here because this is the phase that
first *writes* the field, and §17 makes `priority` a `--sort` key: a free-text priority has no order
that means anything, and lexical order would rank high, low, medium. Widening a closed set later is
backwards-compatible; narrowing a free one is not. **If it proves wrong, Phase 10 is where it will
show**, and the repair is deleting the enumeration, not adding to it.

**Three §26 slips, all resolved in favour of the prose, as Phase 7 resolved the same disagreement:**

- §26's `set` block prints only the change line; §23 says mutations print what they wrote, one line
  per file. The example is showing the part it is about.
- §23's `measure` example lists `state.toml` and `README.md` among the files written. See above.
- §15 says a no-op `set` writes nothing at all — "no event, no projection rewrite, no clock
  movement" — while §26 prints `no change (status already blocked); note recorded`. Both halves are
  kept: the *field* writes nothing, and the `--note` becomes a `note` event of its own, because it is
  an act of attention the user performed and there is no change event left to carry it. Dropping it
  would discard the only new information in the command. **Stated plainly, because it is the cost:**
  this re-opens the loop §15's rule closes — `set --status planned --note ping` on a timer buys
  silence from `review`. It is the same loop `para note x ping` has always allowed, and the design
  accepts that one, because moving the clock is what a note *is* (§3.6). What §3.6 actually forbids is
  a *field change* resetting it, and that still holds: no `change` event moves `attention`.

### Carry-forward obligations from Phase 8

- **Phase 12 owes `rebuild` the whole-tree emit walk** that `config set emit.claude true` does not
  do, and `doctor` the `stale-projection` finding that names it. §26 shows one config write producing
  eight `CLAUDE.md` files; a mutation may not walk the tree (§2.3), so `rebuild` is the honest repair.
- **A known divergence from §6.1, live until Phase 13, and it is not merely an emit consequence.**
  §6.1 says of `CLAUDE.md`'s import list: "it regenerates from the same scope walk that produces the
  rules (§5.4), so adding or removing a skill keeps it correct with no separate bookkeeping" — and
  §2.3 says there is "no deferred emit, no dirty flag". `add skills.x` writes the skill's derived rule
  and leaves every `CLAUDE.md` untouched, so with `emit.claude` on the import list is stale until
  `rebuild`. §6.1 and §2.3's write-through invariant genuinely conflict here, and §2.3 wins for now
  because the Claude surface is opt-in, off by default, and separable (§6.1) — which is why the plan
  put it in Phase 13 at all. **Phase 13 must close it**, and the eight `CLAUDE.md` locations are a
  fixed set rather than a tree walk, so honouring §6.1 costs a constant number of writes and does not
  actually violate §2.3's "nothing walks to root" once it is written deliberately. Until then
  `doctor`'s `stale-projection` is what reports it.
- **The script suite now needs `-tags para_testhooks`**, because every script from here on asserts
  journal filenames and dated `ACTIVITY.md` sections. `TestScripts` skips with that reason when the
  tag is absent; `make test` and CI supply it.
- **Phase 9 inherits `mutate`'s shape**: a verb decides what changed and builds events; `plan` and
  `apply` decide how it is written. `move`, `archive`, and `unarchive` will need a second parent
  (§18.3's "both parents"), which is the one thing `writeset.Mutation` does not yet express.

**Tasks**

1. `add`: directory, `.para/{state.toml, config.toml, logs/}`, README (generated frontmatter, stub
   body), `ACTIVITY.md`, **and the eager child container** — `objectives/` for a project,
   `key-results/` for an objective (§18.1).
2. Refusals with the §26 messages: existing locator, missing parent, reserved word, sibling id
   collision, missing required field, `--type` fixed at creation.
3. The new entity's journal starts empty; the **parent** logs `child added` (§18.1, §3.3).
4. `set` / `unset`: many fields at once, one event per changed field, one write-through pass; a no-op
   writes nothing and exits 0; `--note` required for `blocked` (§18.2).
5. `unset` rules: `created` is an error; `unset skills.x scope` widens to the whole tree (§15).
6. `note` and `measure` (§18.6), including the type-mismatch and duplicate-`--at` refusals.

**Done when** every §26 `add`/`set`/`measure` line and every refusal in those blocks passes as a script
test, and the no-op rule is proven to write zero bytes.

---

## Phase 9 — Relocation: move, remove, archive, unarchive

**Status: done** (branch `impl`). The phase with the most cross-cutting rules, and it changed the
design in one place: §1.6's "unarchiving cascades upward" refused the cascade rather than performing
it. See the decision note below.

`writeset` gains the second parent Phase 8 owed it (`Parents []Subject`) and a `Relocate` call for the
byte-moving phase — renames, stub directories, recursive deletes. `tree` gains `Subtree`, `Children`,
`Entries`, `Skills`, `IsStub`, and `DirExists`: the questions a relocation asks and nothing else does.
`mutate` gains a plan/apply split for all four verbs, so `--dry-run` reports the same value an apply
would rather than re-deriving its own answer.

**Tasks**

1. `move`: one `rename(2)` where possible; same-kind only; refuses the archive boundary; logs
   `child moved` at both parents plus a `field = "locator"` change on the entity itself (§18.3).
2. **Scope rewriting** (§5.4): every `scope` entry naming the moved locator *or anything beneath it* is
   rewritten and the derived rules re-rendered. Table-test the subtree case explicitly.
3. `archive`: the whole subtree in one move; stubs created for live ancestors left behind (§1.6,
   §18.5).
4. `unarchive`: cascades upward, reinstating the archived ancestor chain and adopting any ancestor
   already live; refuses an id colliding with a live sibling, naming it.
5. `remove`: interactive confirmation naming the blast radius, `--force`, `--dry-run`; and
   `--keep-files` deleting every `.para/`, `ACTIVITY.md`, and `MEASUREMENTS.csv` **and stripping the
   frontmatter block from every `README.md` while keeping the body** (§18.4).
6. `--dry-run` on all four.

**Done when** every §26 archive/unarchive refusal matches, the stub lifecycle is tested in both
directions (stub → entity when the parent is archived; entity → stub when a child is unarchived), and
`--keep-files` is proven to leave human README bodies intact.

### Six decisions worth recording

- **§1.6's unarchive rule was self-contradictory, and the heading won.** Its heading said
  "Unarchiving cascades upward" and its body then *refused* the cascade — "you cannot unarchive a child
  whose parent is archived; para refuses and names the parent" — which makes the heading a lie and
  leaves nothing cascading. §1.6 and §18.5 now say what the heading did: `unarchive` reinstates every
  archived ancestor it needs as a live entity, adopts any ancestor already live, and leaves a stub
  wherever an archived sibling stays put. That makes the two verbs exact mirrors — each drags what
  belongs to the thing you named, each leaves a stub on the other side — and it is what makes this
  phase's "entity → stub when a child is unarchived" reachable at all. §26's refusal example is
  replaced by the successful cascade.
- **An id collision refuses for the entity you named and adopts for an ancestor.** §1.6's hard error
  stands for the thing being unarchived. An ancestor is ancestry, and a live `areas/health` already
  existing is precisely the condition under which nothing needs reinstating — refusing there would
  refuse the ordinary case where the live parent never left.
- **A relocation re-renders every descendant's `README.md`, and only that.** §2.3 forbids a field
  mutation from walking a subtree; §19 says these four verbs are the ones that "touch more than one
  entity's worth of bytes", and §18.3 requires a move to rewrite README frontmatter. On a same-kind
  move the locator is the only thing in that frontmatter that *can* have changed, so §18.3's clause is
  only meaningful if the locator is there — and a descendant left carrying a locator resolving to
  nothing is the same defect one level down. No other projection names a locator, so the walk costs one
  file per descendant and no journal reads.
- **Scope rewriting logs a `change` event on each skill it touches.** §18.3 enumerates the moved
  entity and its two parents, but §5.4 makes para own the rename of every enumerated locator, and a
  skill's stored `scope` genuinely changed. Without the event the skill's scope would differ from what
  its own history says was last set, with nothing anywhere to say why.
- **`Relocate` is a call of its own, not a fifth field of `writeset.Mutation`.** Everything `Apply`
  writes is *read* from the post-relocation tree: a README body, ACTIVITY.md's prior days, and the
  chain of `config.toml` files deciding a rotation threshold all live at the new path. That ordering is
  also the safe one — the rename is the truth change (a locator is a path), and the journal line is the
  record of it, so a crash between them leaves a stale projection rather than a record of a move that
  never happened.
- **A cross-device rename is refused rather than emulated.** §18.3 says "one `rename(2)` where
  possible"; a copy-and-delete fallback would have to reproduce modes, times, and hard links to be a
  move rather than an approximation of one. A tree spanning two filesystems is moved by hand, then
  `para rebuild`.

### Carry-forward obligations from Phase 9

- **`doctor` (Phase 12) owes `scope-unresolved`.** `remove` deliberately does not rewrite a `scope`
  entry naming what it deleted: §5.4 makes para own the rename, and there is no locator to rewrite to.
  The entry naming nothing is the finding, and until Phase 12 lands nothing reports it.
- **`remove` of a skill leaves `CLAUDE.md`'s import list stale**, exactly as `add skills.x` does. This
  is the same §6.1 divergence Phase 8 recorded and Phase 13 owes; the two ends are now symmetric, which
  is the most that can be said for it before Phase 13.
- **Phase 10's `list` and `show` inherit stubs as a shape they must not mistake for an entity.**
  `tree.IsStub` answers the narrow question (a directory with no truth behind it); `tree.Walk`'s `Stub`
  flag answers the walk's harder one (a placeholder versus content that merely sits at a legal
  position). A read command wants the second.

---

## Phase 10 — Reading

### Decisions taken before the phase started

§16, §17, and §23 leave four things open that every read command depends on. Settled up front so no
command invents its own answer:

1. **`--json` carries truth *and* everything derived.** An object is the stored fields plus every value
   the human output computed to print — `effective-status`, `attention`, `dormant`, and a key-result's
   `current`, `progress`, `pace`, and derived status. §2.5 already makes derive-at-read-time the rule
   rather than the exception, so the alternative would only push §4.2's arithmetic onto every caller.
   Keys are kebab-case, matching every other name para prints: §15 field names, §7 config keys, locator
   segments.
2. **`show`'s children summary is bounded by kind.** The whole subtree for the project → objective →
   key-result chain, because §1.3 closes it at three levels and a project's objectives without their
   numbers say nothing — which is exactly what §16.1's example shows. Immediate children plus a count of
   what lies beneath for areas and resources, which nest without limit. §16.1 already draws this line
   for siblings ("it does not print … its siblings (`list`)"); `list <locator>` is how you see the rest.
3. **The count line and terminal hiding are two different numbers.** `showing N of M` keeps meaning
   what §17 says — what the filters matched, before `--limit`. Items hidden by terminal status get a
   line of their own naming the count and the flag (`9 hidden (done, dropped) — --all to include`), and
   `--json` carries `hidden` beside `total` and `shown`. Folding them together would leave one number
   with two causes and two different fixes. Nothing is said about `archive/`: archived things are not
   hidden, they are somewhere else (§1.6), and `list archive.projects` is the address.
4. **Output never adapts to the terminal.** No TTY detection, no colour, no width probing; columns
   sized to the content of the rows printed. The same bytes piped as interactive, which is the only way
   §0.2's determinism argument reaches output and the only way §26's examples can be golden files. The
   cost, stated plainly: a deep locator on an 80-column terminal wraps, and para will not shorten it.

Three further readings, resolved from evidence already in the tree rather than by choice:

- **§26's and §23's `…` is the document eliding, not para.** §26 writes `para measure
  …key-results.signups 880/11000` as an *input* line, and `…` cannot be typed; §23's `wrote
  projects/…/key-results/signups/.para/logs/…` is the same shorthand, and Phase 8's `write.txtar`
  already pins para printing those paths in full. So read commands print locators whole.
- **`activity` re-derives from the journal; it does not `cat` `ACTIVITY.md`.** §16.4's "without
  `--recursive` it is just `cat` on a generated file" describes the result, not the implementation. The
  journal is truth and `ACTIVITY.md` is a projection (§2.1), so a read command sourcing the projection
  would report drift that `doctor` exists to find — and `--since` and `--json` need structured events
  regardless. "They agree by construction" is then a property the renderer guarantees, which is what
  Phase 6's mode-equivalence property test already asserts.
- **`list`'s default order is locator order**, which is the §8.5 walk's own stable, lexical order.
  §17 makes `--sort` explicit and lists `locator` among its keys; the default is that key, not an
  unstated one.

**Tasks**

1. `show` (§16.1): stored fields, computed lines, children summary, the skills line naming the scope
   entry that reached it, the dormancy note, threshold provenance. Unaffected by terminal hiding.
2. `list` (§16.2): entities at any depth; **containers transparent** — never rows, always traversed;
   `archive/` not traversed unless named; terminal items hidden without `--all`.
3. `log` (§16.3): raw journal, newest first, across every rotated file; `--kind`, `--limit`,
   `--reverse`.
4. `activity` (§16.4): the digest; `--recursive` merges descendants with per-line locators — the only
   rollup, computed on demand, nothing written.
5. Filters, sort, limit (§17), including the `showing N of M` count line.
6. `--json` on every read command, carrying `total` and `shown` when truncated (§23).

**Done when** golden files cover every read shape in §16 and §26, and a test asserts `list` never emits
a container row while still finding entities beneath one.

---

## Phase 11 — `review`

**Tasks**

1. The five groups (§20): `--stale`, `--blocked` (no timer), `--overdue`, `--behind`, `--skills`.
2. Grouped by reason, ordered within a group by distance past threshold; `--limit`, no `--sort`.
3. Terminal and archived excluded unless `--all`.
4. **Always exits 0.**

**Done when** the §26 review output matches, a test proves a blown deadline is unhideable (`missed` is
not terminal, §1.7), and `--skills` fires off `review.cadence` resolved through the chain.

---

## Phase 12 — `rebuild` and `doctor`

**Tasks**

1. `rebuild [<locator>] [--dry-run]`: regenerate every projection from `state.toml`, `tree.toml`, and
   the journals; idempotent; never reads a projection to produce one (§21.1). `ACTIVITY.md` re-derived
   in full from every rotated file.
2. `doctor`: the deep scan and all eleven findings (§10) — `orphan`, `misplaced`, `invalid`, `journal`,
   `collision`, `scope-unresolved`, `orphan-rule`, `orphan-mirror`, `broken-link`,
   `stale-projection`, and advisory `untracked`.
3. **`stale-projection` at full fidelity**: every generated file re-derived in memory and compared byte
   for byte, with `ACTIVITY.md` compared against *every* backing journal and the report naming the
   earliest differing day (§3.5, §10).
4. The 0/1/2 exit contract; `--json`; no `--fix`.

**Done when** a hand-mutated tree produces exactly the expected finding set, the dated `ACTIVITY.md`
drift report is tested against a hand-edited prior day, and `rebuild` is proven idempotent by running
it twice and diffing.

---

## Phase 13 — The Claude Code surface

Late and separable, because it is opt-in and touches nothing else (§6.1).

**Tasks**

1. `emit.claude`: `CLAUDE.md` as a pointer plus one `@` import per derived rule, regenerated from the
   same scope walk that produces the rules.
2. `emit.claude-skills`: `symlink` (default) and `copy`; **`.para/` never mirrored** in either mode.
3. Mode switching: a config change plus `rebuild` removes the old shape and writes the new one, leaving
   no residue.
4. Pruning on skill removal; `orphan-mirror` and `broken-link` wired into Phase 12's `doctor`.
5. A `core.symlinks=false` simulation: replace a link with a plain file containing its target path,
   assert `broken-link`, repair with `rebuild`.

**Done when** both modes round-trip through `rebuild`, switching modes leaves no residue of the other,
and the hostile-checkout test passes.

---

## Phase 14 — Crash consistency, properties, conformance sweep

**Tasks**

1. Crash-injection harness: kill the process between each write in a `writeset` and assert the tree is
   always either clean or reporting exactly `stale-projection`, and that `rebuild` restores cleanliness
   (§0.2).
2. Property tests, carried forward from the v2 plan and re-aimed at v4:
   - any legal mutation sequence followed by `rebuild` is a no-op;
   - `rebuild; rebuild` is a no-op;
   - `doctor` is clean after any sequence of legal commands;
   - `add` then `remove --keep-files` returns a directory to its pre-para contents, README body
     included.
3. Flip every remaining `[para:phaseN]` gate. A test enumerates §26's fenced blocks and fails if any
   line is not covered by a script test.
4. Fuzz the codecs, `tagexpr`, and `ptime`.
5. Scale check on a generated tree of a few thousand entities: assert the write-through invariant holds
   (constant writes per mutation regardless of tree size), assert read paths never open journals they
   do not need, and time `doctor` and `list`.
6. Cross-platform CI: add a Windows job, primarily to exercise `emit.claude-skills = "copy"` and the
   `broken-link` path.

**Done when** the crash matrix is green, all four properties hold, no phase gates remain, and the scale
numbers are recorded in the README.

---

## Phase 15 — Release engineering and docs

**Tasks**

1. Tagged releases; GoReleaser for linux/macos/windows binaries, with `install.sh` preferring a
   matching prebuilt asset and falling back to building from source — which removes the Go-toolchain
   requirement for most users.
2. `para init` templates: the `AGENTS.md` generated blocks for the root and each bucket. §27 correctly
   defers this prose out of the spec; it wants drafting against a real tree.
3. `ACTIVITY.md` line templates per event kind, and the derived-rule template.
4. Shell completions (`para completion bash|zsh|fish`) — free from Cobra, and worth having given the
   six-segment locators.
5. `--help` review: every command's help text agreeing with §13–§23.
6. README: both install paths, a five-minute tour, and `para-design-v4.md` linked as normative.

**Done when** a tagged release installs on all three platforms and `para init` produces a tree whose
`AGENTS.md` a fresh agent can act on without further explanation.

---

## Spec gaps — closed

All six gaps this plan opened are now resolved in the design document. Recorded here so the phases that
depended on them point at the decision rather than re-deriving it.

| Gap | Resolution | Where |
| --- | --- | --- |
| `--sort` on a cross-kind `list` contradicted ADR 0002 | Error only when **no** matched kind has the field; otherwise per-kind groups sort independently and a group lacking it falls back to `locator` order, last | §17, §25 |
| `--direct` under transparent containers | Apply transparency first, then take immediate children — so `list projects.acme --direct` shows the objectives | §17 |
| Whether containers and skills appear in `review` | Entities only; skills via `--skills` alone, on `review.cadence`; containers never | §20 |
| Measurement uniqueness precision | The exact stored instant, not the day | §3.1 |
| `--match` reading every journal | Allowed and unrestricted; documented as the one read path that opens journals, with a locator as the scoping answer | §17 |
| `attention` for a skill | The same clock rule — newest `note`, else `created`, since a skill takes no measurements. Containers resolve to `created`. | §3.6 |

Two consequences for the phase plan:

- **Phase 10** implements the per-group sort fallback and transparency-first `--direct`, and needs a
  test for the *no-kind-has-it* error as well as the mixed case.
- **Phase 11** excludes containers from every group and reads `review.cadence` — not `stale-after` —
  for `--skills`.

## ADR actions — done

- **ADR 0001 is superseded by [ADR 0003](adr/0003-no-merge-driver-git-not-a-precondition.md)**, which
  keeps the no-merge-driver conclusion, discards the git-as-precondition premise, and records why
  `merge=union` and `merge=ours` are definitions rather than heuristics — the same test ADR 0001 used to
  decline a driver.
- **[ADR 0002](adr/0002-cross-noun-sort-fallback.md) carries an amendment**: it now governs `list`
  rather than a separate `search` verb, the fallback order is `locator` rather than `id`, and a key no
  matched kind has remains a hard error.

---

## What changed from the v2 plan, and why

| v2 plan | v4 plan | Cause |
| --- | --- | --- |
| `nounmeta`: noun×containment, noun×field, noun×verb | `kindmeta`: field matrix only | Kind derives from path (§1.3) — two of three matrices are gone |
| `projection/`: log → five stored scalars | `render/`: truth → eight generated artifacts | Projections are deliverables, not a cache (§0 principle 3) |
| `emit/` with three drivers | `render/` + `mirror/`, one output shape per artifact | Drivers died in v2; write-through then killed `emit` itself |
| No `rebuild` | `rebuild` is a phase | Projections exist again (§2.4) |
| Byte-stability needed for frontmatter only | Needed for TOML, frontmatter, JSONL, and CSV | `doctor` compares every generated file byte for byte (§10) |
| Git a precondition; merge behavior tested against it | Para never invokes git; `.gitattributes` is just a file | §9 |
| — | Crash-consistency phase | Write-through touches four or five files per mutation (§0.2) |
| — | Windows CI job | `emit.claude-skills` has a platform-sensitive mode (§6.1) |
| 14 phases | 15 | `rebuild`/`doctor` and the Claude surface split out; `import` phase deleted |
