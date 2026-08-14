# para — noun-verb command surface (requirements)

The CLI grammar changes from `para <verb> <locator>` to `para <verb> <noun> <id-chain>`. This
document is the requirement set; `para-design-v4.md` is amended to match it, and
[`implementation-plan.md`](implementation-plan.md) breaks the work down.

**Pre-1.0.** para has one user and no released consumers. There is no migration path, no
back-compatibility shim, no dual-acceptance period, and no deprecation notice. The old locator form
stops being accepted and stops being written on the same commit. Existing trees are re-created or
`rebuild`-ed; nothing is written to carry them across.

---

## 1. Why

Kind is derived from the locator (`kindmeta.KindOf`, §1.3), and the §15 field matrix knows exactly
which of the eleven fields each of the seven kinds has. But `fieldFlags.register`
(`internal/cli/write.go:67`) walks `AllFields()` and registers **all eleven flags unconditionally**
on both `add` and `set`. The matrix is consulted only at execution time, inside `mutate`, where it
produces a refusal.

The consequences are all visible in `para add --help`:

- Someone adding a project is offered `--type`, `--start`, `--target`, and `--scope`.
- Someone adding a resource is offered `--status`, `--priority`, and `--due`.
- The flag descriptions apologise for being there — `"a skill's scope"`, `"a key-result's baseline"`,
  `"a key-result's measurement grammar"`. Prose is patching a partition the command surface should
  have made.
- `--help` never enumerates the six things you can add. You have to already know that `projects.x`
  yields a project.

`set` is worse: the same eleven flags, on the command run most often.

Speaking the noun partitions the flags, the help text, the completions, and the refusals — all from
the matrix that already exists.

## 2. What this reverses

§25 records **"Verb-first with locators, not noun-verb"** as a deliberate v2→v4 decision, and §13.1
records deleting `para skill add|list|remove` specifically. `command-surface.md` in the repo root is
the v1/v2 surface built on `para <noun> <verb> <locator>`.

§25's argument is about machine necessity — *"the locator carries the kind, so the verb never needs
to."* That is true and is not contested here. The claim this change makes is that the noun is not
for the machine: it is what partitions the help, the flags, and the completions for a reader. §25
never weighed discoverability, so the decision is amended rather than overturned on its own terms.

## 3. The address model

**R1.** An address is a **(noun, id-chain) pair**, not a string. On the command line it is two
argument tokens. When serialized — in TOML, in frontmatter, in a journal event, in a flag value, in
a doctor finding — it is one token: `<noun>.<id-chain>`.

**R2.** The noun vocabulary is exactly seven words, the singular of each kind in `kindmeta`:

    project  area  resource  objective  key-result  skill  container

**R3.** Given the noun, the arity and shape of the id-chain determine the path completely. This is
the inverse of §1.3's "location is kind", and it is total:

| CLI form | stored form | path |
| --- | --- | --- |
| `project acme` | `project.acme` | `projects/acme` |
| `objective acme.q1-growth` | `objective.acme.q1-growth` | `projects/acme/objectives/q1-growth` |
| `key-result acme.q1-growth.signups` | `key-result.acme.q1-growth.signups` | `projects/acme/objectives/q1-growth/key-results/signups` |
| `area health` | `area.health` | `areas/health` |
| `area health.training` | `area.health.training` | `areas/health/training` |
| `resource papers.kafka` | `resource.papers.kafka` | `resources/papers/kafka` |
| `skill signups-report` | `skill.signups-report` | `.agents/skills/para-signups-report` |
| `container acme.objectives` | `container.acme.objectives` | `projects/acme/objectives` |
| `container acme.q1-growth.key-results` | `container.acme.q1-growth.key-results` | `projects/acme/objectives/q1-growth/key-results` |
| `project` *(no chain)* | `project` | `projects/` — the bucket |
| `area` *(no chain)* | `area` | `areas/` |
| `resource` *(no chain)* | `resource` | `resources/` |
| `skill` *(no chain)* | `skill` | `.agents/skills/` |

**R4.** Arity is fixed per noun and is the validation: `project` and `skill` take exactly one
segment; `objective` two; `key-result` three; `area` and `resource` one or more; `container` two or
three, whose last segment must be `objectives` (under a project id) or `key-results` (under an
objective id). A chain of the wrong arity is the refusal that `misplaced` is today.

**R5.** Two existing warts are absorbed rather than carried:

- The `skills.<id> → .agents/skills/para-<id>` exception (`locator.go:97`) stops being an exception
  buried in `Path()` and becomes the `skill` noun's ordinary path rule.
- `kindmeta.IsContainer` exists only because "a container's name is always a reserved word and a
  reserved word can never be an id, so every container locator is one `KindOf` is defined to refuse"
  (`container.go:11`). When the noun is spoken there is nothing to infer, and the function goes away.

## 4. Reserved words

**R6.** The seven singular nouns join `ReservedWords` (`internal/locator/locator.go:20`). The list
grows from ten words to seventeen:

    projects areas resources archive objectives key-results skills logs .para .agents
    project area resource objective key-result skill container

Without this, an entity legitimately named `project` makes `para list project project` unparseable,
because the `list` grammar (R12) depends on being able to tell a noun from an id.

## 5. Archive

**R7.** Archival is a place, not a kind (§1.6), so it stays out of the noun slot. `--archived` is a
**locator qualifier flag** and is accepted wherever an address is read: `show`, `list`, `log`,
`activity`, `path`, `doctor`, `rebuild`, `review`, `set`, `unset`, `note`, `move`, `remove`.

    para show project acme --archived
    para list --archived                  # the whole archive
    para list project --archived          # archived projects

**R8.** It is refused where the side is already implied, each with its own message: `add` (nothing is
created under `archive/`), `archive` (the source is live by definition), `unarchive` (the source is
archived by definition).

**R9.** `move` continues to refuse crossing the boundary (§25). `--archived` on `move` means both
ends are archived.

**R10.** The stored form prepends the place: `archive.project.acme`, `archive.area.health.training`.

**R11.** Archive stubs — bare directories with no `.para/`, §1.6's "one place a locator segment has
no entity behind it" — get **no noun and no address**. `doctor` reports them by on-disk relative
path, and `path` drops stub support. This is the only capability the change gives up, and it is
deliberate: a stub has no kind, so nothing can name it in a grammar whose first token is a kind.

## 6. Command shapes

**R12.** The nineteen commands, amended. `<chain>` is an id-chain; `<noun>` is one of R2's seven.

| Command | Shape |
| --- | --- |
| `init` | `para init [path]` |
| `add` | `para add <noun> <chain> --name … [--field …]` |
| `show` | `para show <noun> [<chain>]` |
| `list` | `para list [<kind>] [<noun> [<chain>]] [filters]` |
| `set` | `para set <noun> <chain> --field value […]` |
| `unset` | `para unset <noun> <chain> <field> […]` |
| `move` | `para move <noun> <from-chain> <to-chain>` |
| `remove` | `para remove <noun> <chain> [--keep-files] [--force]` |
| `archive` | `para archive <noun> <chain>` |
| `unarchive` | `para unarchive <noun> <chain>` |
| `note` | `para note <noun> <chain> "text" [--at …]` |
| `measure` | `para measure <chain> <value> [--at …] [--note …]` |
| `log` | `para log <noun> [<chain>] [--kind …] [--limit n] [--reverse]` |
| `activity` | `para activity [<noun> [<chain>]] [--recursive] [--since …]` |
| `review` | `para review [<noun> [<chain>]] [--stale｜--blocked｜--overdue｜--behind｜--skills]` |
| `rebuild` | `para rebuild [<noun> [<chain>]] [--dry-run]` |
| `path` | `para path <noun> [<chain>]` |
| `doctor` | `para doctor [<noun> [<chain>]]` |
| `config` | `set` / `unset` / `list [--prefix …]` / `show <key> [<noun.chain>]` |

**R13.** `measure` takes **no noun**. Only a key-result can be measured, so the noun would carry no
information: `para measure acme.q1-growth.signups 880/11000`. This is the one command where the
address is a bare chain, and it is worth the irregularity — `measure` is the highest-frequency write
in the tool.

**R14.** `move` speaks its noun **once**, because §18.3 makes `move` same-kind only. A second noun
could only ever repeat the first or be a refusal:

    para move area health.training fitness.training
    para move project acme acme-migration

**R15.** `add` refuses the `container` noun. Containers are created eagerly by `add` on their parent
(§18.1) and are never created directly.

**R16.** `.` survives unchanged as a whole address: `para show .`, `para note . "text"`. It is
self-describing — it resolves by walking up from `$PWD` to the nearest `.para/state.toml` (§14) — so
it takes no noun and occupies one argument slot.

**R17.** A noun with no chain is the bucket, and is accepted by the commands whose argument §14 calls
"entity or container" or "a place to look inside": `show`, `list`, `log`, `activity`, `review`,
`rebuild`, `path`, `doctor`. The commands that require an entity (`add`, `set`, `unset`, `move`,
`remove`, `archive`, `unarchive`, `note`) refuse a bare noun by name, the way naming a container
where an entity is required is an error today (§14).

**R18.** No arguments at all, where the command allows it, is the tree root (§8.1), unchanged.

## 7. `list`

**R19.** `list` is the one command whose meaning depends on lookahead, and the rule is stated rather
than discovered. Scan left to right:

| args | reading |
| --- | --- |
| *(none)* | the whole tree |
| `<noun>` | that kind, tree-wide — which for the four buckets is the bucket |
| `<noun> <noun>` | first is a kind filter, second is a bucket scope |
| `<noun> <chain>` | the pair is a scope |
| `<noun> <noun> <chain>` | first is a kind filter, the pair is a scope |

    para list                          # everything
    para list project                  # every project
    para list key-result               # every key-result, tree-wide
    para list project acme             # inside project acme
    para list key-result project       # key-results anywhere under projects/
    para list key-result project acme  # key-results inside project acme

**R20.** The kind-filter position accepts the six **addressable** kinds only. `container` is not a
legal filter: containers are transparent to `list` and are never rows (§25).

**R21.** This subsumes the kind filter `list` does not have today. It is a new capability, not a
reshuffle.

## 8. Output and storage

**R22.** Everything para prints or writes uses the new form. There is no place that keeps the old
one.

**R23.** `list` and `show` print the noun as its own column rather than as a dotted prefix, because
the column is what makes a mixed-kind listing scannable:

```
$ para list project acme
project     acme                      in-progress  2d
objective   acme.q1-growth            in-progress  2d
key-result  acme.q1-growth.signups    at-risk      1d
showing 3 of 3
```

**R24.** Everywhere the address must be a single token, R1's dotted form is used:

| Site | Example |
| --- | --- |
| skill `scope` in `state.toml` | `scope = ["project.acme-migration", "area.growth"]` |
| README frontmatter `locator:` (`render/readme.go:34`) | `locator: key-result.acme.q1-growth.signups` |
| journal `field = "locator"` events (`move`/`archive`/`unarchive`, §25) | old and new values both dotted |
| generated rule routing sentences (`render/rule.go`) | scope entries dotted |
| `ACTIVITY.md` per-line locators (`render/activity.go`) | dotted |
| `doctor` findings | dotted |
| `--scope` flag values | `--scope project.acme,area.growth` |
| `config --at` flag value | `--at project.acme` |
| `list --json` / `show --json` `locator` key | dotted |

**R25.** `para path` output is unchanged — a bare filesystem path on one line, shaped for `$(…)`.

**R26.** `--json` output gains a `noun` key beside the existing `locator` and `kind` keys only if
they would otherwise disagree; `kind` already carries it, so the default is **no new key** and
`locator` simply changes form.

## 9. Help and completion

**R27.** `para add --help` and `para set --help` list the nouns. Each noun's own help
(`para add project --help`) registers **only the flags the §15 matrix gives that kind**, with the
kind-specific prose deleted from the descriptions since the command is now specific enough not to
need it.

**R28.** Completion offers the noun at position 0 and id-chains at position 1, narrowed by the noun
already typed. `registerCompletions` (`internal/cli/complete.go:504`) stays a single readable
transcription of R12's table.

**R29.** The `--status`/`--priority` split that `registerFlagCompletions` already maintains — value
vocabularies for `add`/`set` versus filter vocabularies for `list`/`review` — is preserved, and gets
easier: the value vocabulary is now per-noun.

## 10. Non-goals

- **No migration.** Nothing reads or rewrites the old form. Pre-1.0, one user.
- **No dual acceptance.** The old locator form is a parse error, not a deprecated spelling.
  Accepting both would be the "two spellings for one thing" §0 principle 5 forbids.
- **No change to the on-disk layout.** Paths, `.para/` contents, file names, and the walk are
  untouched. Only how an address is typed, printed, and serialized changes.
- **No change to the field matrix.** §15 is the input to this work, not an output of it.
- **No new verbs.** The nineteen commands stay nineteen.

## 11. Decisions

Recorded here so nobody re-litigates them.

- **Noun plus short chain, not noun plus whole locator.** `para add key-result acme.q1-growth.signups`
  rather than `para add key-result projects.acme.objectives.q1-growth.key-results.signups`. The
  bucket and container segments are recoverable from the noun, so spelling them is a second copy of
  the kind — which principle 1 forbids for the same reason §8.4 refuses to name truth files after
  their kind.
- **Output shortens too.** The alternative — type short, print long — keeps §14's paste-what-you-read
  property in one direction and breaks it in the other, which is worse than changing both ends.
- **Archive is a flag, not a noun prefix.** `archived-project` would read as though archival were a
  kind, which §1.6 explicitly denies.
- **The stored form is singular-noun-dotted, not colon-separated.** `project.acme` is the CLI form
  with the first space as a dot, so there is one string to learn. Singular-versus-plural also makes a
  stale old-form locator in a commit message or a README body visibly stale rather than silently
  wrong.
- **A bare noun is the bucket.** This collapses "filter by kind" and "the bucket" into one idea, and
  gives `doctor`, `rebuild`, and `path` a bucket spelling for free.
- **`measure` keeps no noun**, and `move` speaks its noun once. Both irregularities are bought by a
  constraint the model already enforces — only key-results are measured, `move` is same-kind only —
  rather than by convenience.
- **Stubs lose addressability**, because a stub has no kind and the grammar's first token is a kind.
- **`Locator` stays the internal representation.** It is already the relative path
  (`locator.go:93`), which is the right internal form. The new type is an *external* form, converted
  at input and output. The walk, the tree, `mutate`, and `doctor`'s subject discovery never learn
  what a noun is.
