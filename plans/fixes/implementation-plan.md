# para — noun-verb command surface (implementation plan)

Continues the numbering in [`docs/implementation-plan.md`](../../docs/implementation-plan.md), which
runs Phases 0–15. [`requirements.md`](requirements.md) is the requirement set; `para-design-v4.md`
becomes normative again once Phase 16 lands.

**Pre-1.0.** No migration, no back-compatibility, no deprecation window. Old-form locators become a
parse error on the same commit that stops writing them.

---

## 0. Ground rules for this work

Phase 0's rules in the parent plan still hold — test-first per package, `testscript` for the
surface, golden files with `-update`. Three additions specific to this refactor.

### 0.1 `Locator` is not the thing changing

`locator.Locator` is already the relative path (`internal/locator/locator.go:93`), and that is the
right *internal* representation: the walk, `tree`, `mutate`, `query`, `review`, and `doctor`'s
subject discovery all reason about position on disk, which is exactly what a `Locator` is.

The new form is an **external** one. It is parsed at input and formatted at output, and the
conversion lives in one package. Forty-nine non-test files mention `locator.Locator`; roughly
fifteen of them *format* an address for a human or for storage, and only those change.

This is the difference between a boundary change and a rewrite, and it is what keeps the tree
compiling and the tests green between phases.

### 0.2 The conversion is total and is property-tested

`Address ⇄ Locator` must round-trip in both directions for every legal address, including the
`skill` path exception and the archive qualifier. This is the same obligation Phase 1 carried for
`locator ↔ path`, and it is the property that makes it safe for the internals never to learn what a
noun is.

### 0.3 Phase 16 gates everything

`cmd/para/conformance_test.go` enumerates every fenced command line in §26 and fails if no
`testdata/script` file proves it — in both directions. The spec and the fixtures are therefore one
unit of work at the ends (Phases 16 and 22) even though the code between them is not.

Expect `conformance_test.go` to be red from Phase 16 until Phase 22. That is the one place in this
plan where a phase does not end green, and it is stated here so it is not rediscovered as a
surprise. Every other test stays green at every phase boundary.

---

## Phase 16 — Amend `para-design-v4.md`

**Status: not started.**

Spec first. The document is normative and doubles as the acceptance suite, so nothing downstream can
be reviewed against anything until it says the new thing.

**Tasks**

1. **§1.3 / §1.4 — replace the addressing model.** `locator = path, always` becomes
   `noun + id-chain → path`. Write requirements.md R1–R5 into the spec, including the per-noun arity
   table and the worked path derivations. State that the inverse mapping is total and why.
2. **§1.4 — grow the reserved word list** from ten to seventeen (R6), with the reason: an entity
   named `project` makes `list`'s grammar unparseable.
3. **§1.6 — archive addressing.** `--archived` as a locator qualifier (R7–R10); the three commands
   that refuse it and why; the stored `archive.project.acme` form; stubs lose addressability (R11)
   and `doctor` reports them by on-disk path.
4. **§5.2 — scope lists** carry the dotted form. One line, but it is the highest-traffic serialized
   address in the tree.
5. **§8.3 — `state.toml`** examples updated for the scope list. Nothing else in §8 moves: the
   on-disk layout is untouched.
6. **§13 — the verb table** replaced with R12's nineteen shapes.
7. **§14 — Addressing** rewritten: the two-token form, the argument table per verb, `.` surviving as
   a whole address (R16), a bare noun meaning the bucket (R17), the entity-required refusal.
8. **§16.2 / §17 — `list`.** The lookahead rule as a table (R19), the kind filter as a new
   capability (R20–R21), the noun column in output (R23).
9. **§22 — `config`** `--at` takes the dotted form.
10. **§26 — the worked examples**, every fenced block, rewritten in the new grammar. This is the
    acceptance suite; Phase 22's scripts are transcribed from it, so it is written to be executable
    rather than illustrative.
11. **§12 — a reversal row**: "Verb-first with locators, not noun-verb" → "Verb, noun, and a short
    id-chain", with what it costs (a seventeen-word reserved list, stubs no longer addressable, one
    lookahead rule in `list`) and what it buys (per-kind flags, per-kind help, per-kind completion,
    and a kind filter on `list`).
12. **§25 — retire** the "Verb-first with locators, not noun-verb" bullet and add the six decisions
    from requirements.md §11.
13. **`command-surface.md`** — add a header noting it is the v1/v2 surface, superseded, and that the
    v4 amendment arrives at a different noun-verb grammar by a different route. It has been sitting
    in the repo root unlabelled and will now read as current to anyone who opens it.

**Done when** §26 reads end to end in the new grammar, no section still describes a dotted-plural
locator, and the four refusals §26 states in prose rather than in a fence are still stated.

---

## Phase 17 — `internal/address`: the external form

**Status: not started.** Depends on Phase 16.

Pure functions. No filesystem, no clock. The smallest phase and the one everything else rests on.

**Tasks**

1. **`address.Noun`** — the seven-word vocabulary (R2), `String()`, `Parse`, and `AllNouns()` in a
   fixed order for help and completion output. Derived from `kindmeta.Kind` rather than declared
   beside it, so an eighth kind cannot produce a nounless address. *Test first:* a table over all
   seven plus the refusals.
2. **`address.Address`** — `{Noun, Chain []string, Archived bool}`. `Parse(noun, chain string)` for
   the two-token CLI form, `ParseDotted(s string)` for the one-token serialized form (R1), and
   `String()` emitting the dotted form. *Test first:* the arity table from R4, every legal shape and
   every refusal.
3. **Arity validation per noun** (R4), producing the same two error kinds §10 uses today —
   `KindConflict` for a reserved word in an id position, `KindValidation` for a chain the noun cannot
   take. The messages name the noun and the expected arity.
4. **`address.ToLocator` / `address.FromLocator`** — the conversion, including the `skill →
   .agents/skills/para-<id>` rule (R5) and the archive qualifier (R10). *Test first:* a round-trip
   property over generated addresses, both directions, both exceptions.
5. **Bucket addresses** (R3, R17): a noun with an empty chain converts to the bucket locator, and
   back. `skill` with no chain is `.agents/skills/`. *Test first:* all four buckets plus the three
   archived mirrors.
6. **Delete `kindmeta.IsContainer`** and fold `IsBucket` into `address`, per R5. `KindOf` keeps
   deriving kind from a `Locator` — the walk still needs it for directories it finds on disk — but
   nothing in the CLI path calls it any more.
7. **Grow `locator.ReservedWords`** to seventeen (R6). *Test first:* `doctor`'s `collision` finding
   fires on an entity directory named `project`.

**Done when** the round-trip property passes over every noun and arity, `go test ./internal/...` is
green, and no package outside `internal/address` and `internal/cli` imports the new type.

---

## Phase 18 — The stored and projected form

**Status: not started.** Depends on Phase 17.

Every site where an address must be one token (R24). Internals keep passing `Locator` around; these
are the ~15 files that format one.

**Tasks**

1. **`internal/truth` — skill `scope`.** Entries are parsed and written as dotted addresses.
   `check.go:155`'s `"scope entry %q is not a locator"` becomes an address refusal that names the
   noun it could not read. *Test first:* `truth_test.go:111`'s fixture in the new form, plus a
   malformed entry.
2. **`internal/render/readme.go` — frontmatter.** `readmeKeyLocator`'s value becomes the dotted form
   (`readme.go:67`). The `kind:` key is unchanged and now agrees with the locator's first segment by
   construction. *Test first:* golden frontmatter per kind.
3. **`internal/render/rule.go` — routing sentences.** Scope entries in generated rule files.
   *Test first:* golden rule for a scoped and an unscoped skill.
4. **`internal/render/activity.go` — `ACTIVITY.md` lines.** The per-line locator in the digest.
   *Test first:* golden `ACTIVITY.md` for a mixed-kind fold.
5. **`internal/journal` — `field = "locator"` events.** `move`, `archive`, and `unarchive` record old
   and new values (§25); both become dotted. *Test first:* a `move` and an `archive`, asserting the
   journal line.
6. **`internal/doctor` — findings.** Every finding that names a subject. Stubs are reported by
   on-disk relative path instead (R11), which is a new branch rather than a reformat. *Test first:*
   one finding per §10 category, plus the stub case.
7. **`internal/config` — the printable chain.** `config show` and `config list` print the level each
   value came from; those levels are addresses. *Test first:* the chain output for a value set at a
   project.
8. **`internal/cli/readjson.go` — the `locator` JSON key** (`readjson.go:28`, `:154`). Form changes,
   key does not, and no `noun` key is added (R26). *Test first:* `list --json` and `show --json`
   golden.
9. **Rebuild the projection goldens.** Every golden file under `internal/render` and
   `testdata/` regenerated with `-update`, then read rather than trusted.

**Done when** `go test ./internal/...` is green, every golden shows the dotted form, and a
`grep` for a dotted-plural locator in generated output finds nothing.

---

## Phase 19 — The CLI argument surface

**Status: not started.** Depends on Phase 18.

The phase that pays for the whole thing: per-noun flags, per-noun help, per-noun refusals.

**Tasks**

1. **A shared address-argument parser** in `internal/cli` — takes the noun and chain from `args`,
   applies `--archived`, resolves `.` (R16), and returns a `Locator` for the rest of the CLI. One
   function, because R12's table is one table and nineteen hand-rolled parsers is a table nobody can
   check. *Test first:* the argument table per verb, including arity refusals.
2. **`--archived` as a shared flag** (R7–R9): registered wherever an address is read, refused with a
   named message on `add`, `archive`, and `unarchive`. *Test first:* accepted on each of the thirteen,
   refused with the right message on the three.
3. **`add` becomes a noun-dispatching command.** One subcommand per addressable noun; `container`
   refused (R15). Each registers **only** the flags §15 gives that kind, via the matrix that already
   exists (`kindmeta.Requirement`). *Test first:* `para add project --help` shows seven flags and not
   eleven; `para add key-result --help` shows nine; `--type` on a project is an unknown flag rather
   than a runtime refusal.
4. **Delete the kind-specific prose from `fieldHelp`** (`internal/cli/write.go:49`). `"a key-result's
   baseline"` becomes `"the baseline; defaults to the first measurement"` — the command is now
   specific enough that the description does not have to name the kind. The `status` entry's
   "a key-result takes only dropped" clause disappears entirely, because the key-result subcommand
   registers only the settable value.
5. **`set` and `unset` become noun-dispatching**, the same way, over the same matrix. `unset`'s legal
   field list per noun comes from `kindmeta.UnsettableFields`, which already exists. *Test first:*
   `para set skill --help` shows five flags; `para unset project acme type` is an unknown field named
   as such.
6. **`move` — noun once, two chains** (R14). *Test first:* a same-kind move, a reparent, and the
   archive-boundary refusal.
7. **`measure` — bare chain, no noun** (R13). *Test first:* the chain resolves to a key-result; a
   chain of the wrong arity is refused by name.
8. **`note`, `remove`, `archive`, `unarchive`** take noun and chain, entity-only (R17). *Test first:*
   a bare noun is refused with the "names a bucket, not an entity" message.
9. **`show`, `log`, `path`, `activity`, `review`, `rebuild`, `doctor`** take noun and optional chain,
   accepting buckets and the root (R17–R18). *Test first:* each at all three arities.
10. **`config --at` takes the dotted form** (R24), since a flag value is one token.
11. **Root `--help`** unchanged in shape; `para add --help` and `para set --help` now list the nouns
    (R27). *Test first:* `help_internal_test.go` asserts the noun list.

**Done when** `para add project --help` shows exactly the project's fields, every §15 refusal that
was a runtime error is now an unknown-flag error, and `go test ./internal/cli/...` is green.

---

## Phase 20 — `list`'s grammar, the kind filter, and the noun column

**Status: not started.** Depends on Phase 19.

Separated from Phase 19 because it is the only place the grammar needs lookahead, and because the
kind filter is a **new capability** rather than a reshuffle — `list` has no kind filter today.

**Tasks**

1. **The lookahead rule** (R19) as a parser over 0–3 args, returning `(kindFilter, scope)`. It is a
   table, and it is tested as a table. *Test first:* all five rows, plus the refusals — a chain in
   the filter position, `container` as a filter (R20), four args.
2. **The kind filter reaches `query`.** `query.Options` gains a kind, and the walk filters on it.
   Container transparency is unaffected: containers were never rows (§25). *Test first:* every kind
   filtered tree-wide and inside a scope.
3. **The noun column in `list` output** (R23) — `printList` (`internal/cli/list.go:62`) emits noun and
   chain as separate cells. `table`'s alignment does the rest. *Test first:* a mixed-kind listing
   golden.
4. **`show` prints the same way**, so the two never disagree about how an address looks.
5. **`--json` unaffected** beyond Phase 18's form change (R26).
6. **The count line and the hidden line** are untouched — they answer different questions and stay
   two lines (`list.go:76`).

**Done when** all five rows of R19's table are tested, `para list key-result project acme` returns
only key-results, and the mixed-kind golden is aligned.

---

## Phase 21 — Completions

**Status: not started.** Depends on Phase 20.

**Tasks**

1. **`registerCompletions` reshaped** (`internal/cli/complete.go:504`) to R12's table: a noun at
   position 0, an id-chain at position 1 narrowed by the noun already typed. It stays one function
   for the reason its own comment gives — "a table implemented in nineteen places is a table nobody
   can check."
2. **Noun completion** per verb: the six addressable nouns for entity-only verbs, all seven where a
   container is legal, and `add` excluding `container` (R15).
3. **Chain completion** narrowed by noun — `para add key-result <TAB>` offers objective chains, which
   is a strictly better answer than today's `completeAddParent` offering every nesting place
   (`complete.go:162`).
4. **`move`'s second chain** — `completeMoveTarget` (`complete.go:179`) already narrows to
   kind-preserving destinations and gets simpler: the noun is given, so the kind filter is free.
5. **`list`'s variadic completion** — offer nouns at position 0, then either a noun or a chain at
   position 1 depending on what position 0 was.
6. **`--archived`** flips the candidate set to the archived side wherever it is registered.
7. **`--scope` and `config --at`** complete dotted addresses (R24).
8. **`registerFlagCompletions`' value/filter split preserved** (R29), and now per-noun on the value
   side.

**Done when** `completion.txtar` covers a noun position, a narrowed chain position, and the archived
flip for at least one verb of each shape.

---

## Phase 22 — Fixtures, the conformance sweep, and the README

**Status: not started.** Depends on Phase 21.

The phase that closes the loop Phase 16 opened. 378 `exec para` lines across 11 `testdata/script`
files.

**Tasks**

1. **Rewrite the eleven txtar scripts** in the new grammar: `init`, `write`, `read`, `relocate`,
   `repair`, `review`, `config`, `tree`, `claude`, `completion`, `version`. Transcribed from the
   amended §26 rather than mechanically translated, so a line the spec no longer claims does not
   survive by inertia.
2. **Rebuild `spec26Coverage`** (`cmd/para/conformance_test.go:78`) against the amended §26. The
   whole-line proof discipline is kept — the review that found the table's first version showed what
   substring proofs let through — and every entry re-verified rather than re-pointed.
3. **The four prose refusals** §26 states outside a fence are re-transcribed in `write.txtar`, now
   with a fifth: a chain of the wrong arity for its noun.
4. **`cmd/para/property_test.go`** — the address round trip joins the existing properties.
5. **`cmd/para/crash_test.go`** — command lines updated; the crash matrix itself is unaffected, since
   nothing about write ordering changes.
6. **`README.md`'s five-minute tour** regenerated end to end. It is a real pinned-clock transcript
   (`PARA_NOW`/`PARA_TZ` under `para_testhooks`), so it is re-run rather than hand-edited.
7. **`AGENTS.md` / `CLAUDE.md`** — any locator examples in the generated per-place prose
   (`render/agents.go`) updated; these are projections, so `rebuild` produces them and the goldens
   are refreshed.
8. **Full green.** `make test`, `make lint`, `make build`, and the conformance sweep — the first
   point since Phase 16 where `conformance_test.go` passes.

**Done when** `make test` is green, `spec26Coverage` covers every fenced §26 line in both directions,
and the README transcript is reproducible from a clean tree.

---

## Dependency graph, for beads

```
16 (spec)
 └── 17 (address type)
      └── 18 (stored + projected form)
           └── 19 (CLI argument surface)
                └── 20 (list grammar + kind filter)
                     └── 21 (completions)
                          └── 22 (fixtures + conformance + README)
```

Strictly linear, which is unusual for this repo and is a property of the change rather than a choice:
each phase's tests are written against the form the previous phase established.

Two places the line can be cut if the work needs parallelising:

- **Phase 18's tasks 1–9 are independent of each other** once the address type exists. They are one
  bead per site, and they can run concurrently.
- **Phase 22's task 1** is eleven independent scripts, and task 6 depends only on Phase 21 landing.

Suggested bead shape: one **epic per phase**, one **task per numbered item**, with the epic-level
dependency edges above. Phase 18's nine tasks and Phase 19's eleven are the two that most want
splitting; the rest are small enough to stay whole.
