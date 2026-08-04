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
  build tag — production must not read them. Local wall-clock with a recorded offset (§15.1) means the
  zone matters; fix it explicitly in tests and never rely on the host zone.

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

The phase with the most cross-cutting rules.

**Tasks**

1. `move`: one `rename(2)` where possible; same-kind only; refuses the archive boundary; logs
   `child moved` at both parents plus a `field = "locator"` change on the entity itself (§18.3).
2. **Scope rewriting** (§5.4): every `scope` entry naming the moved locator *or anything beneath it* is
   rewritten and the derived rules re-rendered. Table-test the subtree case explicitly.
3. `archive`: the whole subtree in one move; stubs created for live ancestors left behind (§1.6,
   §18.5).
4. `unarchive`: cascades upward and refuses a child whose parent is archived, naming the parent;
   refuses an id colliding with a live sibling, naming it.
5. `remove`: interactive confirmation naming the blast radius, `--force`, `--dry-run`; and
   `--keep-files` deleting every `.para/`, `ACTIVITY.md`, and `MEASUREMENTS.csv` **and stripping the
   frontmatter block from every `README.md` while keeping the body** (§18.4).
6. `--dry-run` on all four.

**Done when** every §26 archive/unarchive refusal matches, the stub lifecycle is tested in both
directions (stub → entity when the parent is archived; entity → stub when a child is unarchived), and
`--keep-files` is proven to leave human README bodies intact.

---

## Phase 10 — Reading

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
