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

**Status: done** (branch `impl`). `internal/view` (the read-side derivation layer), `internal/query`
(§17's filters, sort, and limit over the §8.5 walk), and `show`/`list`/`log`/`activity` land, each with
`--json` and `--local`. Three shared seams were extracted on the way — see below.

### Three seams extracted, and the defect that made the case for them

The read path derives the same things the write path does, and the two disagreeing about one entity is
a defect no test of either alone would catch. One was already there:

- **`ptime.Deadline`** is the last instant a stored `due` admits. It existed inside `mutate`, and the
  copy that did *not* use it was `measure`'s derived status, which parsed `due` at midnight — so a
  key-result due today read `missed` from the moment the day began, contradicting the whole-day
  reading the same package enforces on `created ≤ due` (Phase 8's own recorded decision).
- **`krvalue.Assess`** is §4.2 and §4.3 in one call, and **`krvalue.Baseline`** is §4.1's start. The
  baseline rule had three copies (`mutate`, `render`, and the one `view` was about to add), which is
  how the deadline defect could live in one copy and not the others. The write path prints a status
  back from a reading it has not yet appended (§18.6) and the read path prints one for the same
  reading a moment later; they are now one function of one set of inputs.
- **`journal.LatestOf`/`OldestOf`** are the two ends of a history — §3.6's clock, §4.2's `current`,
  §4.1's default baseline — and nothing in §16's output is a function of what lies between them.

**A bug in the cheap read, worth recording because §3.4 makes the shortcut look safe.** The first
attempt read backwards and stopped at the newest *file* holding a match, reasoning that a file is
named for its own first event and rotation only ever opens a new one. That does order the files — *by
write time*. `at` is a different clock: it is bounded above (§15.1 refuses the future) and not below,
so `para note x --at 2026-01-05` written after a rotation puts a January event in the newest file while
the real newest note sits in the file before it. Stopping there is the reader violating §3.1's
"ordering comes from `at`, never from file position". Both functions now scan every file, and neither
decodes a history into a slice or sorts one.

### Decisions taken during the phase

- **`activity` without `--recursive` prints exactly what `ACTIVITY.md` holds, byte for byte**, and
  with `--recursive` prints §26's three-column chronology. §16.4 says the unfiltered case "is just
  `cat` on a generated file, and that is fine: it means the two agree by construction", so the two
  shapes are the file's and the rollup's, not one compromise between them. It is re-derived rather
  than read, per the reading settled before the phase, and the script test asserts the two are
  identical.
- **`render.Digest` is the digest as data, and the line templates take a style.** `activity
  --recursive` needs one entity's lines labelled with its locator and merged with another's, which is
  not a shape any file has — but they have to be the *same* lines, or §16.4's "agree by construction"
  would be a claim rather than a fact. So the six templates live once and the callers differ only in
  spelling: the file is markdown and sentence-cased, the terminal is neither. A test undresses one
  into the other.
- **A container is not a `list` row and *is* an `activity --recursive` subject.** Not an
  inconsistency: §3.3's `child` events land in the container's own journal, so §26's own first line —
  "added objective q1-growth" — is a container's event and would be missing otherwise. What §16.2
  refuses is a *row you can neither set nor act on*, which is about the entity table and not about
  history.
- **`show`'s children summary is headed by the plural of the child's kind** — §16.1's `objectives`
  line. §1.3 gives every kind exactly one kind of child, so the heading is the container's name where
  there is one and reads the same way where there is not.
- **Absence sorts last in both directions.** §17 fixes that an undefined `pace` "sorts last", and
  `--sort pace --reverse` asks for the worst pace first, not for a column of dashes. So presence is
  settled before direction, and `--reverse` flips only the order of the values that exist. Likewise
  `--reverse` flips *within* a per-kind group and leaves the grouping alone, since where the fallback
  group goes is the one thing §17 says about it.
- **`--json` carries UTC and `--local` does not reach it.** §16.2.1 makes terminal output negotiable
  "because nothing compares it and nothing commits it"; JSON is the one output shape that is neither,
  because it is read by a program, which wants the instant. The day counts beside each timestamp are
  the part that was ever about a wall clock, and they are carried as numbers.
- **`due` is never converted by `--local`.** It is a date somebody chose rather than an instant
  something happened at (§15.1, and Phase 8's store-as-typed decision), so rendering it in another
  zone would move a deadline nobody moved.
- **`show` names the threshold key in full** — `stale (project.stale-after 14, from …)` where §16.1
  writes `stale-after 14`. §20 makes the point that a skill reads `review.cadence` and an entity reads
  `<kind>.stale-after`, and a line that says which knob fired is the line that makes §7's chain
  visible. The value prints in the spelling a `config set` would take, not to a fixed precision.
- **A `list` scope naming nothing is an error, not an empty list.** An empty list reads as "you have
  none of those", which is a different and wrong answer to a mistyped locator.
- **`--sort status` on key-results ranks §4.3's derived statuses worst first** — `missed`, `at-risk`,
  `on-track`, `achieved`, `dropped`. Neither §4.3 nor §17 gives an order, so this is a decision rather
  than an implementation. It is taken because the settable vocabularies are already ranked by §1.7's
  own progression and a key-result's derived set is the one kind that has none: leaving it unranked
  would mean `--sort status` fell back to locator order for exactly the kind whose status is most
  worth sorting by. Worst first, because the reason to sort by status is to find what needs attention.

### Found by review, and fixed

Four defects and a dishonest claim, all in this phase's own code:

- **The elapsed window lost an end.** Folding `mutate`'s derivation into `krvalue.Assess` dropped the
  `hasCreated` guard the old code passed to `Elapsed`, so a `created` that will not parse — doctor's
  `invalid` (§10) — arrived as the zero time and ran the window from the year 1. Every such
  key-result got an elapsed fraction of almost exactly 1 and a confident pace. `Assessment` now
  carries `HasCreated`, and §4.2's undefined-pace set has its member back.
- **`--local` was inert on `list` and `activity`**, against §16.2.1's "available on every read
  command". Fixing it exposed the larger half: an age is a count of days on *somebody's* calendar,
  so `show --local` printed `attention 2026-03-04  today` — the date moved and the count did not.
  `DaysSinceIn`/`DaysUntilIn` take the display zone; `DaysSince` stays UTC, because a threshold is
  committed and §3.5's argument applies to it unchanged.
- **`log --json` ignored §23's count contract.** It has a `--limit`, so it carries `total` and
  `shown`; a bare array had nowhere to put them.
- **`list --status done` could never return a row.** The filter selected exactly what the terminal
  hiding then removed. §16.2's sentence is unconditional, but a flag combination that provably
  returns nothing is a defect rather than a strict reading — naming a terminal status *is* the
  request `--all` signals, said more precisely. So an entity whose effective status is what
  `--status` asked for is never hidden, and nothing wider is exempted.
- **`activity --since` was a third output shape**, contradicting the claim above that there are two.
  `--recursive` alone now chooses the shape and `--since` narrows within it, via a read-only `Since`
  on `ActivityRenderer` that drops day sections *after* every line is derived — so a key-result keeps
  the baseline its whole history gives it (§4.1) instead of acquiring one from the oldest reading
  that survived the filter. It is refused in incremental mode: a truncated `ACTIVITY.md` is precisely
  the drift `doctor` exists to report.

One §26 divergence the review surfaced, left as it stands: `--created` does not backdate the parent's
`child` event, so §26's `2026-01-05  …objectives  added objective q1-growth` cannot be reproduced by
creating the objective with `--created 2026-01-05`. That is correct. A `child` event is the parent's
record of an act it performed (§3.3), and the act happened when it happened; `created` is the child's
claim about itself. §26's block is a history in which the two coincided.

### Carry-forward obligations from Phase 10

- **Phase 11 inherits `view.Env.Stale`**, which resolves through `config.StaleKey` and returns the
  threshold *with* the level that supplied it — so no review group re-derives either.
- **Phase 12's `doctor` owes the findings these reads step around.** `view` treats unreadable truth —
  a `created` that will not parse, a `start` outside its type's grammar, a status outside its kind's
  vocabulary — as absent rather than as a failure, on the grounds that it is `invalid` for `doctor` to
  report (§10) and not a reason a read fails. Nothing reports it yet.
- **Phase 14's scale check has a target here**: a `list` opens each entity's journal once for §3.6's
  clock and a key-result's twice more for §4's two ends, and nothing else opens one except `--match`,
  which §17 names as the exception.

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

**Status: done** (branch `impl`). `internal/review` lands as a classifier over Phase 10's read layer,
plus `para review` with `--json`. It is the smallest phase since Phase 2, and deliberately: every value
§20's table tests is one `view` already derives, so the only new rules are which kinds a group admits
and the ordering within one.

### The seam, and why the package is this thin

`review` is the command that decides whether something needs looking at, and `show` is the command that
prints the same verdict for one entity. The two disagreeing about one key-result is a defect neither
one's tests would catch, so nothing here re-derives: staleness is `view.Env.Stale` (which resolves
through `config.StaleKey`, so no group re-derives which knob a kind reads), overdue is
`view.Entity.Overdue`, the pace is `krvalue.Assess`'s, and the entities are `query.List`'s. What is
genuinely this package's is §20's membership table and its "ordered within a group by distance past
the threshold, because the ordering is the point".

**`query` grew the two options `list` never sets, rather than `review` growing a walk of its own.**
What an entity row *is* — a container is transparent, a stub is not a thing, a symlink is never
followed — is one set of rules, and two walks answering it separately is the drift the read path
exists to avoid. The options are `IncludeSelf` and `IncludeArchived`, and both are decisions:

- **`review <locator>` includes the entity you named; `list <locator>` does not.** Not an
  inconsistency: §16.1 carves `list` out explicitly ("`show` is how you see the thing you named"),
  and §21.1's "regenerates every projection **under** the locator" plainly includes the locator's own
  — so "under" is inclusive everywhere the document does not say otherwise. `review` is a question
  about a region of the tree, and the root of the region is in the region.
- **`--all` reaches `archive/` without naming it.** §16.2 keeps archived things out of `list` because
  they are somewhere else rather than hidden (§1.6); §20 lists them beside terminal ones as two things
  one flag brings back. Naming an archived locator still makes what is under it legitimate, for the
  reason Phase 10 gave: an empty result is a wrong answer to an explicit question.

### Decisions worth recording

- **No flag means every group**, including `--skills`. §20's `|` reads as exclusive but §26's own
  example passes two, so the flags name a subset — and naming none of them is not the same as naming
  an empty one. §20's "skills are reached by `--skills` and nothing else" is about which group can
  *contain* a skill, not about which groups a bare `review` runs.
- **`--limit` truncates each group, not the answer.** §20 groups by reason, so a limit spent on the
  first group would silence whole reasons rather than shortening the output — the opposite of what a
  limit is for once the output is already partitioned. The heading carries both counts when it fires:
  `stale (1 of 5)`.
- **`--blocked` has no threshold, so it is ordered by the same clock as everything else.** §20 gives
  it no timer and therefore no distance to be past; longest untouched first is §20's ordering with a
  threshold of zero rather than an ordering of its own. It reads the *effective* status (§1.7) for the
  same reason `--status` does: a blocked objective under a dropped project is not blocked any more.
- **`--behind` is spelled exactly as §4.3's `at-risk` is** — strictly below, undefined pace never
  qualifying — because they are the same question asked twice. A key-result that reads `at-risk` and
  is not listed as behind would be para contradicting itself about one number. The script test asserts
  the `at-risk` status and the `behind` row for the same reading.
- **The threshold key is named in full**, `area.stale-after 30` where §26 writes `stale-after 30`.
  Phase 10 settled this for `show` and the argument is stronger here: one `stale` group mixes kinds,
  and an area and a project in the same list are reading two different knobs. Where the value came
  from is carried by `--json` rather than printed on every row — `show` is the command for one thing's
  provenance, and a path repeated down twenty rows would bury the numbers.
- **One table across every group, not one per group.** §26 prints the `stale` and `behind` rows with
  their second columns at the same offset, which per-group sizing cannot produce. `table` gained
  headings that take no part in column sizing.
- **`--local` is registered and has nothing to convert.** §16.2.1 names `review` among the commands
  carrying it, so it is there; but every number `review` prints is a comparison against a committed
  threshold, and §7's answer to "is this stale" must not depend on which continent asked — the
  argument §3.5 makes for the generated files, and the one `show --local` already follows by keeping
  its staleness verdict in UTC while printing local dates. The only date on a row is a `due`, which is
  never converted at all (§15.1). Stated plainly because a silently inert flag is a defect; this one
  is inert by construction, says so in the code, and says so in `--help` — the shared flag's usage
  string is overridden on this command, because a help line promising a conversion that never happens
  is worse than one admitting there is nothing to convert.
- **Empty groups are omitted and an empty review says so.** A heading with no rows claims there is a
  group to look at; printing nothing at all makes "did it run" and "is there nothing" look identical,
  which is the wrong pair to conflate for the command you run every morning.
- **`kindmeta.StatusBlocked` now exists.** `blocked` was spelled by hand in `mutate` (§18.2's refusal)
  and would have been spelled again here (§20's group). §1.7's vocabulary lives in `kindmeta`, so the
  word does too.

- **"Always exits 0" is about findings, not about whether the command could run.** §20 and §23 both
  say it flatly, and the reading taken is §0.5's: a mistyped locator and a negative `--limit` are
  exit 1, exactly as they are on every other command. What §20 is arguing against is a command that
  fails *because you have work* — "a command that fails whenever you have work is a command you stop
  running" — and an unparseable request is not work. Recorded rather than left in a test comment,
  because it is a literal contradiction of a sentence in bold.
- **`--tags` and `--match` are not on `review`, and that is a decision.** §17 offers them ("on
  `list`, and on `log`/`activity`/`review` **where the flag makes sense**") and they arguably do make
  sense — `review --tags kafka` is a real question. They are omitted because §20 gives this command
  an explicit syntax line listing five flags and no filters, §17's permission is not an obligation,
  and §17's own closing note says the direction filters should grow is one `--where` over all fields
  rather than more flags. Adding them later is backwards-compatible, which is the same shape of
  argument Phase 8 used for `priority`; if it proves wrong, the repair is additive.

**A defect in Phase 10's walk, found by review and fixed here.** `query`'s scoped walk had a
`scope[0] == "skills"` branch that returned *every* skill and ignored the segments after it, so
`list skills.signups-report` was a listing of all skills — and `checkScope` refused a bare `skills`
outright, since §1.4 gives a path to `skills.<id>` and to nothing shorter, so the branch was only ever
reached by the case it got wrong. `review <skill>` would have inherited both. Now `skills` is the
address of the second root and always resolves, and a scope naming one skill is about that skill:
§5.1 gives a skill no children, so anything else answers a question about one thing with an answer
about all of them.

**A §26 divergence found while transcribing, and left as §20 has it:** §26's block shows
`para review --stale --behind` with a `stale (3)` group whose rows are elided. Nothing about the
elided rows is checkable, so the script test transcribes the *shape* — heading with count, rows
indented two spaces, measure then threshold — against a tree whose every date is stated, rather than
inventing three rows to match a number.

**Tasks**

1. The five groups (§20): `--stale`, `--blocked` (no timer), `--overdue`, `--behind`, `--skills`.
2. Grouped by reason, ordered within a group by distance past threshold; `--limit`, no `--sort`.
3. Terminal and archived excluded unless `--all`.
4. **Always exits 0.**

**Done when** the §26 review output matches, a test proves a blown deadline is unhideable (`missed` is
not terminal, §1.7), and `--skills` fires off `review.cadence` resolved through the chain.

### Carry-forward obligations from Phase 11

- **Phase 12's `doctor` owes the findings `review` steps around**, unchanged from Phase 10's note:
  `view` treats truth it cannot parse as absent, so a `created` that will not read makes an entity
  quietly un-stale rather than reporting anything. Nothing reports it yet.
- **Phase 14's conformance sweep should assert the `at-risk`/`behind` agreement as a property**, not
  only on the one key-result the script test measures. The two read one knob through one function
  today; a property over generated histories is what keeps them there.

---

## Phase 12 — `rebuild` and `doctor`

**Status: done** (branch `impl`). `internal/rebuild` and `internal/doctor` land, plus `para rebuild`
and `para doctor` with `--dry-run`, `--json`, and the 0/1/2 exit contract.

### The seam, and why `doctor` renders nothing

`stale-projection` is defined as "a generated file differs from what would be written now" (§10), and
`rebuild` is the command that writes it. So the two must agree exactly about what "would be written
now" means, and the way they are made to agree is that **doctor asks rebuild**: `rebuild.Derive` is the
one function that turns a subject on disk into the bytes its files should hold, and a `stale-projection`
finding is precisely an artifact it reports as stale. A doctor with a renderer of its own would be a
second opinion about the same bytes, which is the thing principle 3 exists to prevent.

`rebuild` reads no clock, and the absence is load-bearing rather than incidental. A projection is a
function of truth alone — §2.5 keeps everything clock-dependent out of the files — so a rebuild today
and a rebuild tomorrow over the same tree produce the same bytes. If that ever stops being true,
`doctor` starts reporting drift on a tree nobody touched.

### Shared seams this phase added or moved

Four rules had two prospective callers each, and each became one function rather than two copies —
the discipline Phase 10 and 11 both paid for learning:

- **`truth.Check`** is §15's rules over a *stored* state, and `mutate.checkState` now delegates its two
  whole-state rules to it. A state a `set` accepts and a `doctor` faults would be para disagreeing with
  itself about one pair of values.
- **`render.RuleFilenames`** is `CLAUDE.md`'s import list, built from the *skills* — one derived rule
  per skill (§5.3, §6.1) — and used by the write path and by `rebuild` alike. `tree.Rules`, which
  lists the rule files actually on disk, has exactly one legitimate caller: `doctor`'s `orphan-rule`.
  The distinction is the phase's sharpest lesson and is written up below.
- **`tree.Resolves`** answers "does this locator address something that is actually there", for the
  three ways of existing (entity, stub, the second root). `query`'s scope check, `rebuild`'s, and
  `doctor`'s `scope-unresolved` all read it; a skill whose scope doctor calls broken and `list` happily
  lists would be para contradicting itself about one locator.
- **`render.Collect`** is the two-phase render — read every artifact's current bytes, *then* render —
  that the write path and rebuild both need. Two renderers copy a human-owned part of the file already
  on disk and one copies every prior day of it, so interleaving the phases is wrong in a way that
  stays invisible until two renderers on one subject read each other's files.

### The defect the review found in this phase's own code

**`CLAUDE.md` was a projection derived from a directory of projections.** The import list came from
listing `.agents/rules/`, and both review agents found it independently — which per the Phase 11 note
is the strongest signal a run produces. The failure it caused is exactly the one §21.1 legislates
against in the words "never reads a projection to produce one": on a tree planted as truth alone —
§2.4's own "a merge resolved truth and left the projections wrong" — `rebuild` writes the root and the
buckets before it reaches `.agents/skills/`, so all eight `CLAUDE.md` files were derived from a rules
directory it had not repaired yet, and the run needed a second pass to converge. `para rebuild && para
doctor` exited 1. The same bug from the other side had `rebuild` writing an `@` import for a rule file
`doctor` was simultaneously reporting as `orphan-rule`.

The fix is one line of principle: the list comes from the skills, through `render.RuleFilenames`. Worth
recording as a general lesson, because "one rule, one function" was already being applied and did not
prevent it — `tree.Rules` *was* a single shared function, and it was reading from the wrong side of
§2.1. **A shared derivation is only as sound as the side of the truth/projection line it reads from.**

### Two defects this phase exposed in older code

- **The walk could not see through a stubbed *container*.** Archiving one objective out of a live
  project leaves `archive/projects/acme/objectives/` as a bare directory (§1.6), and `walkChildren`
  refused to descend into any bare directory whose name was reserved — so the archived objective and
  everything under it were invisible to every read, and `doctor` reported them as orphans on a tree
  para itself produced. The stub branch now descends through legal container positions
  (`kindmeta.IsContainer`), and `walkArchiveStub` classifies with `tree.KindAt` rather than
  `kindmeta.KindOf`, which is defined to refuse every container.
- **A reserved word used as an id was invisible.** `tree.KindAt` called anything with a reserved last
  segment a container, so `projects/skills/` — §10's own `collision` — walked as a legal container and
  nothing ever reported it. Containers are now decided by *position* (`kindmeta.IsContainer`: the four
  buckets, the three archived mirrors, `objectives/` under a project, `key-results/` under an
  objective), and the error `kindmeta.KindOf` already returns distinguishes the two findings — tagged
  `KindConflict` for a collision and `KindValidation` for a misplacement.
- **An id that was not a legal locator segment derived a kind anyway**, found by the review.
  `kindmeta.checkID` tested only the reserved-word list, never the `[a-z0-9-]` charset `locator.Parse`
  enforces on what you type — so `projects/UPPER/` walked as a project, `rebuild` manufactured a
  `README.md` and an `ACTIVITY.md` for it, `list` printed the locator, and `show` then refused to parse
  the locator `list` had just printed. `locator.ValidSegment` is now one function with two callers, and
  the directory is §10's `misplaced`, which is what it is. The same hole existed at the second root:
  `walkAgentsSkills` admitted any `para-*` directory, so a reserved word used as a *skill* id was
  blessed by `doctor` and refused by `add`; it now classifies through `tree.KindAt` like every other
  position.

### Decisions worth recording

- **`orphan` vs `misplaced` is decided by reachability first.** §10 gives three answers for a
  `state.toml` the fast walk did not visit and they name different repairs, so the discriminator has to
  be sharp: if the walk cannot *descend* to the directory's parent it is an `orphan` whatever its shape,
  and only a directory the walk could have reached is judged on whether its locator derives a kind. A
  skill is where descent stops — §1.3 gives skills one level and §5.1 makes everything inside one the
  author's — so an entity planted under a skill is beneath content exactly as one under a project's
  `notes/` is.
- **Everything beneath an orphan is its own finding.** Moving an objective into `notes/` produces three
  orphans: the objective, its `key-results/` container, and the key-result. Folding them into one would
  be a guess about which directory the repair addresses, and the report is a list of what a read gets
  wrong rather than a list of edits.
- **`doctor` skips only `.git` in the deep scan.** §21.2's "walks every directory including inside
  content" is taken at its word, including the directories para itself owns: an entity hand-`mv`'d into
  a `.para/` or a `.claude/` is exactly the vanishing §8.5 names as the walk's weakness, and skipping
  them hid it. An earlier version skipped both, and the review showed the stated reason for `.claude/`
  was false: §6.1's "**`.para/` is never mirrored**" means a `copy`-mode mirror cannot be read as a
  second entity, and a `symlink`-mode one is never descended because `WalkDir` reports a symlink as a
  non-directory whatever it points at. §6.1 already prevents the phantom; the skip bought nothing.
  `.git` stays, and is a different kind of thing: a repository's own object store is not the tree,
  cannot hold an entity, and is thousands of directories deep. Walking `.para/` needs one guard — a
  directory whose name is a reserved word is never `untracked`, because `projects/.para/` sits directly
  in a bucket and is para's own.
- **`config.toml` is checked too, and its failure suppresses the drift comparison.** §10's `invalid`
  row opens with "unparseable TOML" and does not say which truth file it means; §2.2 makes `config.toml`
  the other one. Leaving it unchecked was worse than an omission — an unparseable config broke the §7
  chain for every descendant, so it surfaced as one `stale-projection` per subject, naming directories
  rather than the file and leaking an absolute path. Now the file is reported once, at the level that
  owns it, with the parse error; and a subject whose chain will not resolve is not compared, on the same
  ground as unreadable state.
- **A subject whose truth or journal will not read is not compared for drift.** Every projection is
  derived from those two, so "what would be written now" is unanswerable — reporting drift would be
  reporting a comparison that was never made, and reporting the same broken line twice.
- **A projection para cannot *produce* is reported as `stale-projection`, not `invalid`.** The case is a
  delimited block whose markers were edited out of an `AGENTS.md` or a `.gitattributes`: §10's `invalid`
  is about truth files, and a generated file that cannot be regenerated differs from what would be
  written now in the strongest possible sense.
- **A missing generated file is stale.** §10 asks whether the file "differs from what would be written
  now", and an absent file differs from every possible answer. This is what discharges Phase 8's debt:
  `config set emit.claude true` leaves eight `CLAUDE.md` files unwritten, `doctor` now reports each as
  missing, and `rebuild` writes them.
- **`rebuild` writes as it goes rather than deriving the whole tree first**, so a tree too big to hold
  in memory is still one that can be rebuilt. The consequence is stated rather than hidden: if a
  subject's truth will not read, the subjects already visited have been written and the ones after it
  have not — the same degraded state a crash produces, with the same repair (§2.4).
- **`rebuild` writes; it does not delete.** An `orphan-rule`, an `orphan-mirror`, or a `CLAUDE.md` left
  behind by turning `emit.claude` off are residue rather than drift, and Phase 13 owns removing them —
  its task 3 is "a config change plus `rebuild` removes the old shape and writes the new one, leaving
  no residue". Reporting them is this phase's job and it does; sweeping them is not.
- **`doctor`'s scoped scan does not check the rules or the mirror.** A derived rule lives in
  `.agents/rules/` and a mirror in `.claude/skills/`, neither of which is inside any entity's subtree —
  so `doctor projects.acme` cannot reach them, and reporting them anyway would make a scoped scan
  quietly tree-wide.
- **An unknown key in a truth file is `invalid`.** §10's list does not name it and §8.3 does not declare
  the schema closed, so this is an addition rather than a transcription. It earns its place because
  `DecodeState` *ignores* unknown keys — one stray key must not make every read of an entity fail — and
  ignoring is not accepting: an unknown key is either a typo that silently does nothing (`descripton`)
  or a field written by a newer para, and both are worth a word.
- **A stub is never a subject, at any scope.** `tree.Subtree` used to classify the scope root from the
  locator alone, so `doctor archive.projects.acme` read a bare ancestry placeholder as a project and
  reported a spurious `invalid` for the `state.toml` a stub is defined not to have (§1.6) — while `list`
  answered the same locator correctly. Found by the review; the scope root is now classified against the
  filesystem like every other node.
- **`broken-link` consults neither `emit.claude` nor `emit.claude-skills`.** A symlink that does not
  resolve is broken whatever the config says, and a mirror materialised as a plain file is §10's named
  `core.symlinks=false` checkout. §10 scopes the finding to "a `symlink`-mode mirror", and this is
  wider by one case: a plain file where a `copy`-mode mirror belongs is wrong too, and the entry claims
  to be a mirror while not being one, which is what the finding is about. Reading the mode to decide
  *which name to give the same broken file* would be a distinction with no different repair. What the
  config does decide is whether the mirror should be there at all, which is `orphan-mirror`'s question
  and Phase 13's repair.
- **`journal.Decode` now requires `at`.** §3.1 makes it load-bearing — "ordering comes from `at`, never
  from file position" — so a line without one would sort as the year 1 and head every digest. Refusing
  in the codec is what lets doctor name the file and the line, since the only thing that can point at a
  line is whatever refused it.
- **`paraerr.Status` carries an exit code with no message.** `doctor`'s findings *are* its output and
  the code is a second channel saying how to read them (§21.2); printing `error: 3 findings` beneath a
  report that already lists them is noise §26's own transcript does not show.
- **`doctor --json` carries `exit`.** It is the only way a test can tell 1 from 2, since testscript
  distinguishes zero from non-zero and no more — and the difference between the two is the whole point
  of the contract.
- **`--local` is registered on `doctor` and has nothing to convert**, for the reason Phase 11 gave
  `review`: the one timestamp a finding can print is the `at` of a journal line, quoted back exactly as
  the line stores it so that the line can be found. The help string says so.

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

### Carry-forward obligations from Phase 12

- **Phase 13 owes `CLAUDE.md` on a skill mutation, and it is now a reported defect rather than a
  theoretical one.** With `emit.claude` on, `add skills.<id>` and `remove skills.<id>` change the set of
  derived rules and therefore every `CLAUDE.md` import list — and a mutation writes its subject's files
  and no others (§2.3), so all eight are left stale until `rebuild`. This is the §6.1-versus-§2.3 slip
  already recorded under Phase 8: §6.1 says the list "regenerates from the same scope walk that produces
  the rules … with no separate bookkeeping", §2.3 forbids a mutation walking to root. The resolution
  stands — the eight locations are a *fixed set*, not a tree walk, so Phase 13 can honour §6.1 at
  constant cost — and it is Phase 13's because it is Phase 13's surface. Until then `rebuild` is the
  honest repair, `doctor` names every stale file, and the script test asserts exactly that sequence.
- **Phase 13 owes the pruning half of `rebuild`.** This phase's `rebuild` writes and never deletes, so
  turning `emit.claude` off leaves eight `CLAUDE.md` files and a `.claude/skills/` mirror that nothing
  removes and nothing reports. Phase 13's task 3 already requires "no residue" on a mode switch; that is
  where the removal belongs, together with pruning a rule and a mirror when a skill goes.
- **Phase 13 should decide whether `orphan-mirror` and `broken-link` deserve to fire when
  `emit.claude` is off.** They currently report what is on disk regardless, which is right for a broken
  link and arguable for a mirror whose skill exists but whose surface has been turned off — that case is
  residue, and residue is the finding Phase 13 will have a repair for.
- **Phase 14's crash matrix has its assertion ready.** `doctor` reporting exactly `stale-projection`
  and `rebuild` restoring cleanliness is now a thing the binary can be asked, and `doctor --json`'s
  `exit` field is how a harness reads the verdict without parsing prose.
- **Phase 14 should property-test the two modes of `ACTIVITY.md` against each other through
  `doctor`.** The dated drift report is only meaningful because incremental and full mode produce
  identical bytes for the same history; that equivalence is asserted today by `render`'s own tests and
  by "doctor is clean after a mutation", and a property over generated histories is what keeps it.

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
