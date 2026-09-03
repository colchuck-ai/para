# para — design v4

Status: **complete draft.** Every structural decision is settled. §0–§12 are the model and storage;
§13–§26 are the command surface, rebuilt from scratch with v2's twenty-two verbs and
`command-surface.md`'s spec treated as candidates that had to re-earn their place. §26's worked
examples double as the acceptance suite.

Supersedes `para-design-v2.md` and `command-surface.md`. Where v4 reverses v2, §12 says so and
carries the cost, so nobody has to reconstruct the argument from two documents.

The one job, restated for v4 because the emphasis moved:

> **para is a filing system you can read without running it.** A disciplined PARA tree on disk with
> stable-shaped locators, where every fact has one home and every human- or agent-facing file is
> generated from that home. You, `grep`, GitHub's file browser, and a coding agent all have one
> predictable place to look — and three of those four will never invoke the CLI.

That last clause is the whole pivot. v2 optimised for *nothing stored that could be derived*. v4
optimises for *the tree explaining itself to a reader who has no tooling*, and pays the price of
derived files to get it.

---

## 0. Principles

Five rules. Every section below is downstream of one of them.

1. **Location is identity.** Where a directory sits says *which* thing it is — its id, its parent,
   and its `Locator`, its path with `/` swapped for `.` — and nothing else names it (§1.3). The noun
   and id-chain you type or read (§1.4) are a fixed encoding of that same path — total and invertible
   in the direction that needs no tree, address to path, with §1.4 stating precisely what the inverse
   now costs — never a second fact that could disagree with it: no elision table, and no format
   choice for a command's own address argument, which is always two tokens on the command line and
   one dotted token wherever it is serialized, including as another command's flag value (§1.4).

   **Position does not say *what* a directory is.** The kind is stored, in `state.toml`, and §30 is
   the argument for why that one fact left the path while the other three stayed: id, parent, and
   locator are facts *about* the path and a second copy could contradict it, whereas a position that
   is a legal home for two kinds states nothing to contradict. The entity-`--kind` flag this
   principle has always forbidden stays forbidden, for the reason it always was: an address carries a
   noun, and the noun is the kind, spoken once. (`log --kind` is a different word — a journal
   *event*'s kind, §3 — and is untouched by any of this.)
2. **One source of truth per fact.** For anything with a `.para/`, that is its `.para/state.toml`.
   For history, the append-only journal. Nothing else is authoritative, ever.
3. **Generated files are the product, not a cache.** README frontmatter, `ACTIVITY.md`,
   `MEASUREMENTS.csv`, `AGENTS.md`, rule files — these exist for readers who will never run para.
   They are written through on every mutation and rebuildable from truth at any time.
4. **One home per event.** An event is logged at the entity it happened to, and nowhere else.
   Rollups are computed when asked for, never stored.
5. **One spelling per thing.** One locator form (§1.3's internal `Locator`, not §1.4's two-token
   address — see Principle 1). One place scope is declared. One clock.

Principle 3 is a direct reversal of v2's principle 2 ("no caches, no projections, no `rebuild`").
The distinction that makes the reversal honest: v1's projection was a **cache** — it existed to make
reads fast, so every piece of machinery guarding it was pure overhead. v4's projections are
**deliverables** — they are the reason the tree has this shape at all. `rebuild` is not repair
machinery apologising for a cache; it is the renderer.

---

## 1. Model

### 1.1 The tree

```
brain/
├── .para/
│   ├── tree.toml                    ← the root marker: schema + tree identity
│   ├── config.toml
│   └── logs/20260101T160801Z.jsonl
├── .agents/
│   ├── rules/
│   │   ├── para-signups-report.md          generated from the skill. that is all rules ever are.
│   │   └── my-own-constraint.md            no para- prefix, so para never reads or writes it
│   └── skills/
│       └── para-signups-report/
│           ├── SKILL.md                     frontmatter generated, body yours
│           ├── scripts/  references/        yours, untouched
│           └── .para/{state.toml, config.toml, logs/}
├── .claude/                         only if emit.claude (§6.1); holds skill symlinks
│   └── skills/para-signups-report → ../../.agents/skills/para-signups-report
├── .cursor/                         only if emit.cursor (§6.2); holds skill mirrors and rule files
│   ├── skills/para-signups-report → ../../.agents/skills/para-signups-report
│   └── rules/para-signups-report.mdc
├── README.md  AGENTS.md  CLAUDE.md  ACTIVITY.md
├── projects/
│   ├── README.md  AGENTS.md  CLAUDE.md  ACTIVITY.md
│   ├── .para/{state.toml, config.toml, logs/}
│   └── acme-migration/                          project.acme-migration
│       ├── README.md  ACTIVITY.md
│       ├── .para/{state.toml, config.toml, logs/}
│       ├── design.md                            yours. para never touches it.
│       └── objectives/                          container.acme-migration.objectives
│           ├── README.md  ACTIVITY.md
│           ├── .para/{state.toml, config.toml, logs/}
│           └── q1-growth/                       objective.acme-migration.q1-growth
│               ├── README.md  ACTIVITY.md
│               ├── .para/{state.toml, config.toml, logs/}
│               └── key-results/                 container.acme-migration.q1-growth.key-results
│                   ├── README.md  ACTIVITY.md
│                   ├── .para/{state.toml, config.toml, logs/}
│                   └── signups/                 key-result.acme-migration.q1-growth.signups
│                       ├── README.md  ACTIVITY.md  MEASUREMENTS.csv
│                       └── .para/{state.toml, config.toml, logs/}
├── areas/
│   ├── README.md  AGENTS.md  CLAUDE.md  ACTIVITY.md
│   ├── .para/{state.toml, config.toml, logs/}
│   └── health/                                  area.health
│       ├── README.md  ACTIVITY.md
│       ├── .para/{state.toml, config.toml, logs/}
│       ├── training/                            area.health.training
│       │   └── … same shape, nests freely
│       └── scans/                               untracked. invisible to para.
├── resources/
│   └── … same shape as areas/
└── archive/
    ├── README.md  AGENTS.md  CLAUDE.md  ACTIVITY.md
    ├── .para/{state.toml, config.toml, logs/}
    ├── projects/
    │   ├── README.md  AGENTS.md  CLAUDE.md  ACTIVITY.md
    │   ├── .para/{state.toml, config.toml, logs/}
    │   └── old-migration/                       archive.project.old-migration
    ├── areas/
    │   ├── README.md  AGENTS.md  CLAUDE.md  ACTIVITY.md
    │   ├── .para/{state.toml, config.toml, logs/}
    │   └── health/                              ← a stub. no README, no .para/ (§1.6)
    │       └── training/                        archive.area.health.training
    └── resources/
        └── … same shape
```

- **Every `.para/` holds the same two filenames** — `state.toml` and `config.toml`. The kind is in
  `state.toml` and appears nowhere else, so nothing can desync (§8.4, §30).
- **Tree root** = the directory holding `.para/tree.toml`. Every command walks up to find it, the way
  git finds `.git`. `$PARA_HOME` overrides. `init` refuses inside an existing tree. `tree.toml` exists
  because uniform state filenames leave nothing else to test for: it is a marker with a genuinely
  different job — the identity of the *tree*, not the state of a thing in it — and it appears exactly
  once (§8.1).
- **Four buckets**, not three: `archive/` is a real place now, because the A in PARA is a place and
  because "where did this go" should be answerable by looking (§1.6).
- `.agents/` sits beside the buckets rather than inside `.para/`, because it is the directory a
  coding agent is expected to find by convention. Its contents are namespaced by a `para-` prefix on
  the containing directory, so anything without that prefix is yours and para never reads or writes
  it (§5).

### 1.2 Six kinds of directory

| Shape | Recognised by | Examples |
| --- | --- | --- |
| **root** | holds `.para/tree.toml` | `brain/` |
| **bucket** | a container at depth 1 under the root | `projects/`, `areas/`, `resources/`, `archive/` |
| **container** | holds `.para/state.toml` **and** has a reserved name | `objectives/`, `key-results/`, `archive/areas/` |
| **entity** | holds `.para/state.toml` and has an id for a name | a project, area, resource, objective, key-result, link, skill |
| **stub** | in `archive/`, holds no `.para/` at all | `archive/areas/health/` above |
| **content** | anything else inside an entity or a bucket | `design.md`, `scans/` |

Containers and entities hold the same two filenames, and it is the **stored kind** that separates
them: `kind = "container"` or one of the entity words (§8.3, §30). One copy, in one place, and the
only place a kind is written down.

**The reserved-name rule survives, doing a different job.** A container's name is always one of the
reserved words (§1.4), and a reserved word can never be an id — and since `"container"` is the word
for *all* of them (§8.2), the name is what says *which* container this is. `objectives/` may hold an
objective and `links/` may not, and no `kind` value distinguishes those two. The rule is also what
keeps §10's findings apart: a directory whose name is a reserved word but whose stored kind is an
entity kind is a `collision`, where a container sitting somewhere no container of that name may sit is
`misplaced`.

What the rule no longer buys is a *file-free* test. That was the payoff of a derived kind — name it
and you knew — and §30.3's walk reads every entity's `state.toml` anyway, so nothing is spent by
asking the file.

Three of the six kinds above are still read off position, because they have no `state.toml` to store
anything in: **root** holds `tree.toml` (§8.1), **stub** holds no `.para/` at all (§1.6), and
**content** is recognised by exactly that absence. A **bucket** is a container at depth 1, so its
depth is what makes it a bucket and its stored kind is `"container"` like any other. Only entities and
containers carry a kind, because only they have somewhere to put one.

### 1.3 Kind is stored; location is constrained

| Location | Kind | Nests? |
| --- | --- | --- |
| `projects/X` | **project** | no — depth 1 only |
| `projects/X/objectives/Y` | **objective** | no — depth 1 under the container |
| `projects/X/objectives/Y/key-results/Z` | **key-result** | no — leaf |
| `areas/X`, `areas/X/Y/…` | **area** | yes, no depth limit |
| `resources/X`, `resources/X/…` | **resource** | yes, no depth limit |
| `projects/X/links/Z`, `areas/X/…/links/Z`, `resources/X/…/links/Z` | **link** (§29) | no — leaf |
| `archive/{projects,areas,resources}/…` | the same kinds, dormant (§1.6) | as above |
| `.agents/skills/para-X` | **skill** — an entity | one level |
| `.agents/rules/para-X.md` | **derived rule** — a projection, not an entity (§5.3) | — |
| `.agents/**` without a `para-` prefix | **yours**. para does not read or write it, ever. | — |
| anything else | **content**. para does not track it, ever. | — |

**For the rows naming an entity or container kind, the table above states where each may live — not
how its kind is derived.** That kind comes from `state.toml`'s `kind` key, and the table is the
containment rule §30.2 makes explicit and `add`, `move` and `doctor` all consult. Read the kind, then
ask whether it is allowed where it sits.

The remaining rows are unchanged and still positional, because their subjects have no `state.toml` to
read: a derived rule is a projection (§5.3), `.agents/**` without a `para-` prefix is yours, and
everything else is content — recognised by the absence of a state file, which is a fact about position
and nothing else. Dormancy stays positional too (§1.6): archival is `archive/`, not a stored flag.
This is the same restriction §1.4 places on the table for its own purposes.

A state file records its own **kind**. It records no **id**, no **parent**, and no **locator**: those
three come from the path, and they are the half of v2 §1.2's load-bearing idea this design keeps
whole. *(§30 says why the other half was given up, and what it bought; §1.5 has the caveat about what
a hand-`mv` costs.)*

The two halves are not the same bet. id, parent, and locator are facts *about the path* — a second
copy of any of them could contradict the bytes' own location, which is the desync §8.4 refuses. A
kind is not a fact about the path: `areas/relationships/people` is a legal home for an area and for a
Managed Directory alike, so the position states nothing to contradict.

**`max-depth` remains gone**, with its two config keys, its creation-time refusal, its `--force`, and
its `doctor` finding. Depth was a proxy for a filing judgement it could not make. *(v2 §5.3.)*

`Locator` (the relative path) stays the representation the walk, the tree, and every internal package
reason about, and §1.4 gives it a second, external half. What changes is that a locator alone no
longer answers *what kind is this*. The walk reads each entity's stored kind and carries a
locator→kind index for the command's lifetime (§30); every internal caller that used to derive a kind
from a path now consults that index.

The direction that matters to §1.4 is unaffected: a spoken address still names exactly one path with
no tree read at all, because the noun is spoken. It is only the inverse — a path, asked what it is —
that now needs the index.

### 1.4 Addressing: noun + id-chain → path

An address is a **(noun, id-chain) pair**, not a string. On the command line it is two argument
tokens. Wherever it must be a single token — in TOML, in frontmatter, in a journal event, in a flag
value, in a `doctor` finding — it is written `<noun>.<id-chain>`, dots separating id-chain segments,
hyphens separating words within a segment, charset `[a-z0-9-]`.

The noun vocabulary is exactly eight words, the singular of each kind above:

    project  area  resource  objective  key-result  link  skill  container

Reserved and unusable as an id: the eleven places and structural names —
`projects`, `areas`, `resources`, `archive`, `objectives`, `key-results`, `links`, `skills`, `logs`,
`.para`, `.agents` — plus the eight nouns above. Nineteen words in total. `links` joins the structural
names for the reason `key-results` did: it is a container word, not an id, and §29.3 is where its own
chain shape is defined.

Without reserving the eight nouns, an entity legitimately named `project` would make `para list
project project` unparseable: `list`'s grammar (§16.2) tells a kind filter from a scope by lookahead,
and that only works if a bare noun can never also be a live id. Reserving the nouns is what keeps "is
this token a noun or an id" a lexical question instead of a contextual one.

Given the noun, the arity and shape of the id-chain determine the path completely. This is the
**inverse** of §1.3's location table, restricted to the rows §1.3 gives an entity or container kind
to — a rule, a piece of content, and anything under `.agents/**` without a `para-` prefix have no noun
and no address (§5.3), and an archived entity's stored form carries `archive.` as an extra prefix on
top of this table (§1.6). Within that restriction, **address → path is total and needs no tree**:
every legal `(noun, chain)` pair names exactly one path, because each noun's path template starts with
its own fixed, noun-specific prefix — a distinct bucket name, or `.agents/skills/para-` for `skill` —
so no two nouns can ever produce the same path, and a chain's arity fixes the rest.

**Path → address is where the stored kind is felt, and only there.** It was once equally total, on the
argument that a badly-shaped chain never reaches a path at all, leaving one legal chain shape per path.
That argument held while position derived kind. It does not now: `areas/relationships/people` is a
legal path for an area and for a Managed Directory alike, and only the stored kind separates them
(§1.3, §30). So a path names exactly one `(noun, chain)` *given the kind index* — which every caller
asking a path what it is already holds, because it walked to find the path in the first place.

What survives untouched is the property §1.5 calls load-bearing. A stored `<noun>.<chain>` still
resolves to its path with no resolution step, so scope entries, journal event ids, `doctor` findings,
and README frontmatter keep working exactly as before. It is only the reverse question — handed a
bare path, what is this — that consults the index:

| Address | Stored form | Path |
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
| `link project.acme.jira-epic` (§29.3) | `link.project.acme.jira-epic` | `projects/acme/links/jira-epic` |
| `link area.health.training.blog` (§29.3) | `link.area.health.training.blog` | `areas/health/training/links/blog` |
| `container area.health.training.links` (§29.3) | `container.area.health.training.links` | `areas/health/training/links` |
| `project` *(no chain)* | `project` | `projects/` — the bucket |
| `area` *(no chain)* | `area` | `areas/` |
| `resource` *(no chain)* | `resource` | `resources/` |
| `skill` *(no chain)* | `skill` | `.agents/skills/` |

**Arity is fixed per noun, and arity is the validation.** `project` and `skill` take exactly one
id-chain segment; `objective` two; `key-result` three; `area` and `resource` one or more; `container`
two or three, whose last segment must be `objectives` (under a project id) or `key-results` (under an
objective id) — or, per §29.3, a parent kind (`project`/`area`/`resource`) followed by that kind's own
id-chain and a trailing `links`. `link`'s own arity is the same parent-chain rule plus one more
segment, its own id: three segments under `project` exactly, three or more under `area`/`resource`. A
chain of the wrong arity for its noun is refused by naming the noun and the arity it expects — the same
kind of refusal §10's `misplaced` finding describes today for a `state.toml` already sitting at a bad
location, now caught before any path is built rather than discovered on disk afterward.

**The noun replaces the container segments, it does not repeat them.** `key-result acme.q1-growth.signups`
is three segments, not the five you would get by spelling `objectives` and `key-results` into the
chain. The bucket and the container names are recoverable from the noun and the chain's own arity, so
spelling them again would be a second copy of the kind — the thing principle 1 forbids. This reverses
what §1.4 said before nouns existed: with no noun to carry that information, un-eliding every segment
was the only way to keep the mapping unambiguous, and typing the bucket and container names once and
pasting them thereafter was the offered trade. Speaking the noun removes the need for the trade.

**`skill` folds in what was the locator's one exception.** `.agents/skills/para-<id>/` used to be a
special case in the path derivation, carved out because a locator segment cannot contain a dot and
`.agents` could therefore never appear in one. Spoken as a noun, it needs no carve-out: `skill` is
simply the noun whose path rule prepends `.agents/skills/para-` instead of a bucket name. There is no
`container` value that means "rules" and no way to address a rule at all, because a rule is a
projection and nothing addresses a projection (§5.3).

**Naming the container is now unnecessary rather than merely elided, for address resolution.** §1.2
distinguished a container from an entity by name; that distinction still holds on disk exactly as
before, since it is what the walk, `doctor`, and anything else that finds a directory rather than being
told about it must still test. What changes is one direction only: resolving a *spoken* address to a
path no longer has to infer "this segment must be a container" from seeing a reserved word, because the
caller already said `container`.

One address, two written forms — spoken (`<noun> <chain>`) and stored (`<noun>.<chain>`) — and every
command, every output, and every generated file uses one or the other consistently: typed on the
command line, printed and serialized as the single dotted token.

A log entry's identifier is its entity's stored address plus the entry timestamp as a final segment:
`project.acme-migration.20260101T081502`.

### 1.5 Entities all the way up

An entity's parent chain is entities and containers, never content. `areas/health/scans/training/`
with a `.para/` in `training/` is **illegal**, not merely discouraged: `area.health.scans.training`
would contain a segment that is not an id, and `area.health.training` would be a lie about where the
bytes are. `doctor` reports it as `orphan`.

The consequence is that **locators are isomorphic to disk paths**, with archive stubs the single
stated exception (§1.6). That isomorphism is what lets scope lists, `doctor` messages, log entry ids,
and README frontmatter all use one string with no resolution step.

Moving the kind into `state.toml` (§1.3, §30) does not touch this. The isomorphism is a claim about
locators and paths, and a kind is neither: id, parent, and locator still come from the path, so the
one string still resolves with no step. This is the invariant the stored kind was designed around
rather than through, and the reason `kind` was the only fact allowed to move.

**Adoption is gone.** v2 had `import`, and `add` on an existing directory writing only `.para/`. v4
has neither: an untracked directory is invisible, and the way to make something an entity is to
create the entity and move your files into it. This costs the "point para at my existing repo"
story, and buys the guarantee above.

**A hand-`mv` now requires `para rebuild`.** In v2, `mv` to a legal position was simply correct
because nothing was stored. In v4, a move invalidates README frontmatter, `ACTIVITY.md` headers, any
`scope` entry naming the thing, and every derived rule that mentions it. `para move` does all of
that; a hand-`mv` leaves the tree readable but stale, and `doctor` reports `stale-projection`. This
is a real regression against v2 and it is accepted, because it is the same bill principle 3 already
signed.

**What a hand-`mv` no longer costs is the kind.** The `kind` key travels with the state file, so
`mv areas/health/acme projects/acme` leaves a project that is still a project. Under position-derived
kinds the same `mv` silently *reinterpreted* it — an area with objectives, measurements, and key-results
hanging off it, and no finding raised anywhere. Now the kind survives the move and `doctor` reports the
position as illegal (§10, §30). The stale-projection bill above is unchanged; the silent-reinterpretation
bill is withdrawn.

### 1.6 Archive is a place

Archiving **moves bytes**. `projects/acme` becomes `archive/projects/acme`; the address is still
`project acme`, and its stored form gains `archive.` as a prefix: `archive.project.acme`. Archival
stays out of the noun slot — `archived-project` would read as though archival were a kind, which this
section already denies — so `--archived` is instead an **address qualifier flag**, accepted wherever an
address is read: `show`, `list`, `log`, `activity`, `path`, `doctor`, `rebuild`, `review`, `set`,
`unset`, `note`, `move`, `remove`.

```
para show project acme --archived
para list --archived                  # the whole archive
para list project --archived          # archived projects
```

It is refused, each with its own message, wherever the side is already implied: `add` (nothing is ever
created under `archive/`), `archive` (the source is live by definition), `unarchive` (the source is
archived by definition). `move` keeps refusing to cross the archive boundary either way (§18.3);
`--archived` on `move` means both ends are archived.

- **Archived things are first-class.** They have addresses, they are addressable, they can be shown
  and listed, and a skill's `scope` may name them.
- **Archiving drags the whole subtree.** Archiving an area takes its sub-areas and its content with
  it, in one move. There is no partial state.
- **Unarchiving cascades upward**, which is the exact mirror of the sentence above. Unarchiving
  something reinstates every archived ancestor it needs as a live entity and brings its own subtree
  with it, in one operation. An ancestor that is already live is adopted rather than duplicated.
  Archived siblings stay archived, and the reinstated ancestor's archive directory is left behind as a
  stub to record *their* ancestry — the same stub archive leaves for a live ancestor that stays put.
  So the two verbs are symmetric: each drags what belongs to the thing you named, and each leaves a
  stub for whatever stays on the other side.
- **Id collision on unarchive is a hard error — for the thing you named.** If a live sibling has taken
  its id, para refuses and names it; you rename the sibling or unarchive nothing. An **ancestor** whose
  id is taken gets the other answer: the live one is adopted as the parent. An ancestor is ancestry
  rather than the thing being unarchived, and `areas/health` existing is precisely the condition under
  which nothing needs reinstating — refusing there would refuse the ordinary case where a live parent
  never left.
- **Stubs preserve ancestry.** Archive a sub-area whose parent stays live and para creates
  `archive/areas/<parent>/` as a bare directory — no `README.md`, no `.para/` — purely to record
  where the thing came from. Unarchive the last thing beneath such a stub and it is removed, because a
  stub that records nothing records nothing. Stubs are the one place a locator segment has no entity
  behind it. `doctor` must recognise them and never report them as malformed.
- **A stub has no noun and no address.** A stub has no kind — it is bytes with no `.para/` behind
  them — and the grammar's first token is always a kind, so nothing can name one. `doctor` reports a
  stub by its on-disk relative path instead of an address, and `para path` drops stub support
  entirely: there is no address to resolve into a path in the first place. This is the one capability
  the noun-verb grammar gives up, and it is deliberate rather than an oversight.

  This sentence needed no revising when §1.3 changed, and that is the interesting thing about it. It
  already grounded a stub's kindlessness in the *absence of a `.para/`* rather than in its position,
  at a time when position was what derived a kind everywhere else — so it was the one row of §1.2 that
  never depended on location-is-kind, and §30 takes nothing away from it. What §30 removes is the only
  mechanism that could have contradicted it: a stub's position is a real position, and a rule reading
  kinds off positions could have insisted a stub had one.

  Mechanically: `KindAt` reports it the way it already reports the tree root — no kind, and no error —
  because a stub is the same category of thing, a real directory that is not an entity. Nothing writes
  a `.para/` into a stub to give it one, either: the walk and `Exists` both use `state.toml`'s presence
  as their test, so a stub carrying one would become "existing", which is the one thing a stub is not.
- **Areas and resources have no status field.** Location *is* archival state: in `areas/` it is
  active, in `archive/areas/` it is archived. This deletes v2's `active | archived` enum outright —
  a second copy of the answer, which principle 1 forbids.
- **Effective dormancy is derived from the path**, exactly as v2 derived terminal cascade: everything
  under `archive/` is dormant, and unarchiving gives back the previous picture without having touched
  a descendant's own fields.

The cost, stated plainly: **locators are no longer stable for life.** v2 promised they were, and
bought that promise by making archival a status. v4 spends it. `scope` lists are rewritten
automatically on archive/unarchive/rename (§5.4), but a locator you wrote into a README body, a
commit message, or an external ticket goes stale silently. That is the price of the archive being
somewhere you can look.

### 1.7 Status

| Kind | Values | Default | Terminal |
| --- | --- | --- | --- |
| project, objective | `planned` `in-progress` `blocked` `done` `dropped` | `planned` | `done` `dropped` |
| key-result | derived: `on-track` `at-risk` `missed` `achieved`; settable: `dropped` | — | `achieved` `dropped` |
| area, resource | none — location is the answer (§1.6) | — | — |
| container, skill, rule | none | — | — |

- Status says **how something ended**; the archive says **where it lives**. A dropped project can sit
  in `projects/` for a month before you file it, and that is not an inconsistency.
- `missed` is **not** terminal. A blown deadline is the one thing that should not be hideable.
- Terminal status cascades by derivation from the path, nothing written to descendants.
- Derived key-result status does not latch: hit the target and regress and you read `on-track` again,
  because for "hold p99 under 200ms" that is the only true answer.
- **Skills have no status and no off switch.** The `.agents/` convention has no notion of a disabled
  skill, and para does not invent one: a skill that exists applies, and a skill you want gone is one
  you `remove`. There is no archiving verb for skills either.

---

## 2. Truth and projection

### 2.1 The rule

> **Para owns frontmatter. Humans own bodies.** No exceptions. Files that are wholly generated —
> `ACTIVITY.md`, `MEASUREMENTS.csv`, rule files — have no human-authored body to own.

### 2.2 Every generated file

Two independent questions sort a generated file, and they decide different things.

**Whether the file is a pure projection of `.para/` state, with no room for anyone else's content, is
the *body* question.** It decides whether there is human-owned prose for para to leave alone, and
nothing else — not merge posture, not delete-versus-shorten. Letting it decide those too would measure
where the bytes come *from*, when the property that actually decides them is who else writes to the
*path*.

**Whether any tool other than para writes to this path is the *interop* question, and it is the one
that decides merge posture and delete-versus-shorten.** If yes, para owns a marker-delimited block and
never the whole file: appending its block to whatever is there, shortening to just the block on
removal, and deleting only when shortening would leave nothing. If no, para owns the file outright and
either a `merge=ours` posture or an outright removal is safe, because nothing else has a stake in the
bytes.

`AGENTS.md` landed on the block-and-append side by accident: it has human prose, so nobody had to ask
the interop question to get its treatment right. `CLAUDE.md` did not — it has no prose of its own, so
the body question alone put it beside `ACTIVITY.md` and gave it `merge=ours`, and that was wrong: a
tool as ordinary as an issue tracker's own setup command writes a marker-delimited block to the same
path, and `merge=ours` silently discarded it on every merge. Applied, the interop question sorts every
row without exception: `AGENTS.md`, `.gitattributes`, and `CLAUDE.md` are shared paths and get blocks;
everything else is a path para alone writes, and keeps what it has.

| File | Generated from | Human-owned part | Merge posture (§9) |
| --- | --- | --- | --- |
| `README.md` (entity, container, bucket, root) | frontmatter ← `state.toml` (root: `tree.toml`) | the body — everything after the frontmatter | normal; your prose is at stake |
| `ACTIVITY.md` | the entity's own journal (§3.5) | none | `merge=ours` |
| `MEASUREMENTS.csv` | measurement events in the journal (§4.4) | none | `merge=ours` |
| `AGENTS.md` (root + 4 buckets + `archive/{projects,areas,resources}`) | a delimited para-owned block | everything outside the markers (§6) | normal; your prose is at stake |
| `CLAUDE.md` | a delimited para-owned block | everything outside the markers (§6) | normal; your prose is at stake |
| `.claude/skills/para-X` | wholly — one mirror per skill, if `emit.claude` (§6.1) | none | `merge=ours` in `copy` mode; n/a for a symlink |
| `.cursor/skills/para-X` | wholly — one mirror per skill, if `emit.cursor` (§6.2) | none | `merge=ours` in `copy` mode; n/a for a symlink |
| `.cursor/rules/para-X.mdc` | wholly ← the skill's `state.toml` (§6.2) | none | `merge=ours` |
| `SKILL.md` | frontmatter ← the skill's `state.toml` | the body, plus `scripts/`, `references/`, anything else in the directory | normal; your prose is at stake |
| `.agents/rules/para-X.md` | wholly ← the skill's `state.toml` | none | `merge=ours` |
| `.gitattributes` | wholly, if enabled | append-only to an existing file | normal |

Two truth files per entity, mirroring v3's split, and the split is: **`state.toml` is what the thing
*is*, `config.toml` is policy *about* it.** State is what generates the projections. Policy is what
para consults when deciding whether a check fires (§7).

### 2.3 Write-through

Every mutating operation rewrites, in the same operation, every projection affected by it. There is
no deferred emit, no dirty flag, no `--emit` flag. A `para measure` on a key-result rewrites that
key-result's `state.toml`, appends to its journal, and rewrites its `README.md` frontmatter,
`ACTIVITY.md`, and `MEASUREMENTS.csv` — and touches nothing else on disk.

That last clause is the property worth stating as an invariant, because it is what keeps
write-through affordable:

> **A mutation touches exactly one entity's files, plus its parent's journal and `ACTIVITY.md` if and
> only if containment changed.**

Nothing walks a subtree on write. Nothing walks to root on write.

### 2.4 `rebuild`

Regenerates every projection in the tree from the truth files and the journals. It is the answer to:

- someone hand-edited a generated file;
- a hand-`mv` left frontmatter stale;
- a merge resolved truth and left the projections wrong;
- a new para version renders a template differently.

`rebuild` is idempotent and it never reads a projection to produce a projection. If `rebuild` changes
anything, `doctor` would have reported `stale-projection` first.

### 2.5 What is never stored

Derived at read time, always, because a stored copy rots with no event having occurred — with the one
exception §28 makes, and makes safe:

- `updated`, and anything else clock-dependent — `attention` was here through v4's settling and moved
  out under §28, which is the only entry this list has ever lost;
- a key-result's `current`, `progress`, `pace`, and derived status (§4);
- effective status and archival dormancy, which are ancestor-dependent (§1.6, §1.7);
- roll-ups of any kind: descendant counts, subtree activity, aggregate progress;
- the set of skills that apply to an entity, and therefore the rules that govern it (§5.2).

Note the asymmetry with §2.2 and be clear about it: `MEASUREMENTS.csv` *does* contain derived columns,
and `ACTIVITY.md` *does* contain a rendered summary. Those are projections — files whose only job is
to be read by something that will not compute. Truth files contain none of it, with the same one
exception: `attention` and `[suppression]` (§28) are computed, cached in `state.toml`, and held to the
same "one function computes it, `doctor` checks it" discipline a projection gets — the file just holds
both a truth part and a generated part now, the way `README.md` always has (§2.1).

---

## 3. The journal

### 3.1 An append-only event stream

`.para/logs/<first-event-timestamp-in-UTC>.jsonl`, one JSON object per line, per entity and per
container.

```jsonl
{"at":"2026-01-01T16:15:02Z","kind":"change","field":"status","from":"planned","to":"in-progress","note":"kickoff done"}
{"at":"2026-01-03T17:02:11Z","kind":"measurement","value":"880/11000"}
{"at":"2026-01-05T01:40:00Z","kind":"note","note":"waiting on the ingest team"}
{"at":"2026-01-05T19:00:00Z","kind":"child","op":"added","child":"q1-growth"}
```

Four kinds. `note` is also a **field** available on every kind, so any mutation can carry a reason
without inventing an event.

| Kind | Where it lands | Carries |
| --- | --- | --- |
| `change` | the entity whose field changed | `field`, `from`, `to` |
| `measurement` | the key-result | `value`, in the type's grammar (§4) |
| `note` | any entity or container | `note` |
| `child` | the **parent** container or entity | `op` ∈ `added｜removed｜moved｜archived｜unarchived`, `child`, and `from`/`to` locators for moves |

- Ordering comes from `at`, never from file position. This is what makes union merge correct (§9).
- `created` is a field in the truth file, not an event. There is no `create` kind. *(v2 §5.1.)*
- **Measurements must be unique in time**, compared at the **exact stored instant** — not the day.
  Progressive precision (§15.1) makes a bare date an exact midnight, so `--at 2026-01-03` twice is a
  collision while `2026-01-03` and `2026-01-03T09:02` are not. A collision is refused and named. Notes
  and changes may collide freely.
- Setting a field to its value it already has writes nothing — no event, no projection rewrite.
  Otherwise a loop buys silence from every check. *(v1's find, kept by v2, kept here.)*
- Deliberately absent: an `actor` field. Nothing reads it yet, and a field nothing reads is a field
  that will be wrong. Revisit when `ACTIVITY.md` has a reason to say who.

### 3.2 One home per event

An event is written to the journal of the entity it happened to. It is **not** copied to ancestors,
and no ancestor's journal is a rollup.

The two alternatives, and why they lost:

- **Full propagation** — append the same line at key-result, `key-results/`, objective,
  `objectives/`, project, `projects/`, root. Seven copies of one fact, a seven-file diff for every
  measurement, and a root journal that is a firehose. Not complex, just wasteful, and it pays for a
  reader who does not exist yet.
- **Derived propagation** — the ancestor logs *its own* state delta with a pointer to the cause.
  Elegant, and rejected: it needs a second entry shape, cause-pointer integrity across moves and
  archival, and a `doctor` check to verify the pointers still resolve. Three pieces of machinery for
  a convenience.

So: one home, and the tree *is* the rollup. If you want to know what happened under a project, the
directories are right there, which is the reason this design has directories at all.

### 3.3 Containment events are not propagation

A `child` event on a parent is the **parent's own** event, because the parent genuinely changed: it
has a different set of children than it did. `projects/` logs a project being added; a project's
`objectives/` logs an objective being added; the root logs a bucket being created at `init`.

The boundary is exact: containment changed, or nothing is written. A note on a key-result writes one
line, in one file. A field change on a key-result writes one line, in one file.

### 3.4 Rotation

One journal file grows until it exceeds `log.rotate-bytes` (default 4 MiB, §7), at which point the
next event opens a new file named for **its own** timestamp, **in UTC**, with a trailing `Z`:
`20260101T160801Z.jsonl`. So a directory listing of `logs/` reads as a chronology, and the newest
file is the last one lexically.

Rotation never rewrites a closed file. Closed journal files are immutable.

Both of those sentences are claims about **string** order, which is why the name is UTC and not the
writer's wall clock. With no offset in the name there is no zone to compare against: an event at
`23:00+13:00` (10:00Z) would sort *after* a later event at `12:00-07:00` (19:00Z), so the newest file
would not be the last one lexically and the next append would reopen a closed file. The DST
fall-back hour reproduces the same inversion annually without anyone leaving their desk. UTC is the
only zone in which lexical order is total, so it is the only zone in which these two guarantees hold.
A filename is an ordering key that happens to be legible, not a wall clock.

### 3.5 `ACTIVITY.md` is a local fold

A human-readable digest of *this entity's own* journal, grouped by day, newest day first. Days with
no events are absent. **Days are UTC days**, and every timestamp in a generated file is UTC.

```markdown
# Activity

## 2026-01-05
- Added objective **q1-growth**.

## 2026-01-04
- Note: waiting on the ingest team.

## 2026-01-03
- Measured 880/11000 (8.0%) — 24% of target.
```

Two mechanics that matter:

- **Only today's section is re-derived on a mutation.** Prior days are already written and are never
  recomputed, so a write reads one journal file, not all of them. `rebuild` re-derives the whole file
  from every journal.
- **Because prior days are never recomputed, they are never checked either** — which is exactly where
  drift would hide. So `doctor` carries a full-fidelity check: re-derive the entire `ACTIVITY.md` from
  every journal file backing it and compare, reporting the first day that differs
  (`stale-projection`, §10). Today's section is guarded by write-through; every day before it is
  guarded by `doctor` and repaired by `rebuild`. Without that check, the cheap write is the design's
  one silent failure mode.
- **Newest-first** because that is what a reader wants and what `CHANGELOG.md` taught everyone to
  expect. It costs a whole-file rewrite per mutation rather than an append; these files are small,
  and the expensive half — reading history — stays bounded.
- **The day is a UTC day, not the author's**, and this is not a stylistic choice. Grouping by each
  event's own recorded offset stops the sections partitioning the timeline: an event at
  `2026-03-05T23:00-08:00` (07:00Z) would file under `03-05` while an *earlier* event at
  `2026-03-06T09:00+09:00` (00:00Z) filed under `03-06`, so a newest-day-first file would present the
  earlier event as the newer one — contradicting §3.1's "ordering comes from `at`, never from file
  position". This file is also committed and read from several zones off one commit, so the
  boundaries have to be ones every reader agrees on. UTC is the only such boundary.

  The wall clock is not lost, only moved: journals keep each event's own offset, so a read command
  converts to local time on request (§16). What a *file* says is fixed; what a *terminal* shows is the
  reader's business.

`ACTIVITY.md` is also the reason the journal does not have to be pretty. Machine truth is JSONL,
human truth is this file, and neither is asked to be both.

### 3.6 One clock

```
attention = the newest `at` among events of kind `note` or `measurement` that count, else `created`
```

A `note` counts unless it was recorded with `--no-attention` (§18.6). Without that flag, `attention`
cannot distinguish the activity a `stale-after` threshold is watching from any other truthful note: an
area's threshold set to catch "have I journalled lately?" reads satisfied by a note about unrelated
housekeeping in the same area, because the rule as stated is "the newest note or measurement, full
stop." `--no-attention` is the escape hatch — a note that records something true about the entity
without asserting the entity was tended. A `measurement` has no equivalent flag: a key-result's own
reading is inherently the activity its clock exists to detect, so there is nothing for it to opt out of.

The rule is uniform across kinds, including the ones that cannot produce every event: a **skill**
takes no measurements, so its `attention` is its newest attending `note`, else `created` — which is
what `review --skills` measures `review.cadence` against (§20). A **container** has a journal too, but
only `child` events land in it, so its `attention` is always `created`; nothing reads it. A
**key-result** is not a special case either, and is worth stating precisely because it is the kind most
likely to be assumed otherwise: its `stale-after` reads the same newest-attending-note-or-measurement
clock every other kind does, not measurements alone. A `note` on a key-result moves its clock exactly as
a `measure` does, `--no-attention` aside — nothing here has ever asked for a measurement-only key-result
clock.

That is the whole rule, unchanged from v2 apart from the flag. `change` events never count — **including
status changes** — because if any entry reset the clock, `para set x --due 2027-01-01` would buy silence
from every check. `child` events do not count either: filing an objective under a project is not the
same as attending to the project, and v2 reached the same answer when adding an objective was a
`change`.

Two threshold knobs, still:

- `stale-after` — days since `attention` before something reads stale.
- `at-risk-pace` — the pace below which a key-result reads `at-risk`.

Gone and staying gone: `blocked-after`, `at-risk-after`, `log-sla`, `sla-reset-at`,
`status-changed-at`, and all three of v1's stored clocks.

As of §28, the *rule* above is still the whole rule — nothing about how `attention` is computed
changes. What changes is that the answer is now written into `state.toml` on every mutation able to
move it, with `doctor` checking the cached value against a fresh derivation rather than trusting it
unconditionally. That is not one of the stored clocks this paragraph just buried: those were
maintained by logic separate from the fold above and could drift from it silently. This one has no
second implementation to drift from — see §28.4.

---

## 4. Key results and measurements

v2's §7 survives the pivot intact. It is restated here because v4 is self-contained, and extended
only where directories and `MEASUREMENTS.csv` are new.

### 4.1 Three types

`type` ∈ `number | ratio | boolean`, required at creation and **never settable** — changing it would
invalidate every measurement already logged. Delete and recreate instead.

| Type | `start` / `target` / `value` grammar | `start` | Pace |
| --- | --- | --- | --- |
| `number` | `42`, `0.024`, `480` | optional | yes, with `due` |
| `ratio` | `880/11000` | optional | yes, with `due` |
| `boolean` | `true` / `false` | **rejected** — `false` is the only baseline | **no** |

- **A ratio's denominator belongs to each reading**, not to the key-result — it legitimately varies.
  So values are stored as strings: `"880/11000"`, never `0.08`. Six months later the denominator is
  the thing you want.
- `target` is required. `start` defaults to the first logged measurement. `target == start` is
  rejected: it is not a target, and it makes §4.2's denominator zero.
- `boolean` requires `target = true` — a target of `false` is achieved at birth.

### 4.2 Derived quantities

- `progress = (current − start) / (target − start)`. Direction falls out of the arithmetic; there is
  no up/down flag. **Not clamped**: overshoot reads above 1 and a regression below baseline reads
  negative, because both are true and both are worth seeing.
  **`start` is subtracted from both sides**, which is the one thing easy to get wrong and the reason
  the worked example is spelled out here rather than left to the reader. For the `signups`
  key-result used throughout this document — `start 480/9000`, `target 2000/12000`, a reading of
  `880/11000` — the three decimals are `0.0533`, `0.1667`, and `0.0800`, so
  `progress = (0.0800 − 0.0533) / (0.1667 − 0.0533) = 0.2353`. Not `0.0800 / 0.1667 = 0.48`: that
  is progress toward the target *from zero*, which is a different and less useful question, because
  it credits a key-result for the ground it had already covered before you committed to it.
- **No measurements yet → progress 0.** No progress has been demonstrated, and saying so plainly is
  what lets an untouched key-result go `at-risk` instead of sitting quiet.
- `pace = progress / elapsed`, where `elapsed = (today − created) / (due − created)`.
- The start *date* is the key-result's `created` — the day you committed, not the day you got around
  to baselining. So the start value and the start date can come from different moments,
  deliberately: baseline four weeks late and you have genuinely burned four weeks.
- Pace is **undefined** when `elapsed ≤ 0`, when there is no `due`, or when the type is `boolean`.
  Undefined pace never reads `at-risk` and sorts last, printing `—`.

`boolean` skips pace because its progress is 0 until done and 1 after, so pace would read `at-risk`
for the key-result's entire life and then flip to `achieved` — noise, not signal. That exemption is
also why `boolean` survives rather than being spelled `start 0 / target 1`: only a declared type can
carry it.

### 4.3 Derived status

Never set by hand. `dropped` is the only settable key-result status.

| Status | Condition |
| --- | --- |
| `achieved` | `progress >= 1` |
| `missed` | past `due` with `progress < 1` |
| `at-risk` | `pace < at-risk-pace`, where pace is defined |
| `on-track` | otherwise |

Without `due` there is no pace and no deadline, so a key-result reads only `achieved` or `on-track`.
A `boolean` has no pace but keeps its deadline, so it reads `achieved`, `missed`, or `on-track`.

### 4.4 `MEASUREMENTS.csv`

A projection of the key-result's `measurement` events, oldest first, one row per reading:

```csv
at,value,decimal,progress,note
2026-01-03T17:02:11Z,880/11000,0.0800,0.2353,
2026-01-17T17:10:04Z,1320/12400,0.1065,0.4687,denominator grew after the launch
```

- `at` is UTC, like every timestamp in a generated file (§3.5). A column of mixed offsets does not
  sort or plot as one axis, and charting is this file's entire job.
- `value` is the reading exactly as logged, in the type's grammar.
- `decimal` and `progress` are derived and belong here for one reason: a spreadsheet, a notebook, or
  GitHub's CSV viewer will chart this file and will not compute anything. It is a projection, which
  §2.5 permits precisely because nothing reads it back.
- It lives only at the key-result. Nothing aggregates measurements upward.

---

## 5. Skills and rules

### 5.1 The skill is the thing you author

```
.agents/skills/para-signups-report/
├── SKILL.md                        frontmatter generated; body yours
├── scripts/pull-signups.sh         yours
├── references/metric-defs.md       yours
└── .para/
    ├── state.toml                  ← the only place scope lives
    ├── config.toml
    └── logs/
```

```toml
# .agents/skills/para-signups-report/.para/state.toml
kind        = "skill"
name        = "Signups report"
description = "when asked for the weekly signups number"
scope       = [
  "objective.acme-migration.q1-growth",
  "area.growth",
]
tags    = ["growth", "reporting"]
created = "2026-01-01T16:15:00Z"
```

- `description` is the when-to-use hook and the only part that ever enters an agent's context
  automatically. The body is read on demand.
- A skill is a **real entity** — it has state, config, and a journal — because unlike v2's skills it
  now carries scope and tags, both of which want a history.
- **`scope` is optional, and omitting it means the whole tree.** A skill with no scope applies
  everywhere, and its derived rule says so. That is the right default: the reason to write a skill is
  that you want it used, so narrowing is the deliberate act and breadth is the resting state.
- Everything except `SKILL.md`'s frontmatter is yours. Bundled scripts and references are exactly why
  a skill cannot be generated from a TOML string the way a rule can.

### 5.2 Scope is an explicit address list, and it covers the subtree

```toml
scope = ["project", "area.health.training"]
```

- Entries are addresses in their stored form — the same dotted strings every command prints and
  parses (§1.4). A single language, no second matcher. `"project"` is the bare-noun form: no chain,
  the whole bucket (§1.4).
- **Omitting `scope` entirely means the whole tree.** Breadth is the resting state; narrowing is the
  deliberate act. There is no `scope = ["*"]` spelling, because absence already says it.
- **An entry covers that address and everything beneath it.** `["project"]` means every project,
  now and in future. This is containment, not a glob language: no wildcards, no character classes, no
  precedence rules, nothing to document beyond the previous sentence.
- **No exclusions.** You cannot exempt one project from a matching entry. If you want to, narrow the
  entry or write a second skill — and if you want to often, the skill is miscarved, which is signal
  rather than a gap. Adding `exclude = [...]` later is backwards-compatible; removing an exclusion
  engine later is not, so v4 starts without.
- **Matching on location and kind only, never tags.** Tag-matched scope would mean retagging a
  project silently changes which rules govern it, with nothing in the diff to say so. "Which rules
  apply here" must be answerable by looking at the path.
- **Scope has exactly one home.** An entity's `config.toml` says nothing about skills or rules.
  Adding a skill touches one file. There is no entity-side opt-in, no two-source resolution, and
  therefore no need for a command that explains why something fired.
- **Which skills apply to an entity is derived** — walk from the entity to root collecting matches,
  computed when asked, never stored (§2.5).

### 5.3 A rule is a projection of a skill. That is all a rule ever is.

Every fact a routing rule needs — where it applies, what it is for, what to read — is already in the
skill's `state.toml`. So a rule is not a thing you author, own, or address. It is one generated file:

```markdown
<!-- .agents/rules/para-signups-report.md -->
---
generated_from: para-signups-report
---
When working under `projects/acme-migration/objectives/q1-growth/` or `areas/growth/`, use the
**Signups report** skill (`.agents/skills/para-signups-report/SKILL.md`) when asked for the weekly
signups number.
```

With no `scope`, the same skill renders without the location clause — "Use the **Commit style** skill
(…) when writing a commit message" — which is the whole difference between a tree-wide skill and a
scoped one, and it is visible in the rule file rather than inferred.

`generated_from` is what makes ownership checkable: para knows which rule files are its own, so
removing a skill removes its rule, and `doctor` reports one that outlived its skill (`orphan-rule`,
§10). One file, one line of provenance, no state, no config, no journal.

**There are no authored rules**, and the reason is worth keeping because it was almost decided the
other way. A rule directory with its own `state.toml` would hold an id, a scope, a description, and a
body — every one of which the skill it points at already has. The only rule that genuinely has no
skill behind it is a standing constraint ("in `area.finance`, never commit a number you did not
source"), and the tempting fix — an `always` mode on a skill that inlines its text into the rule — is
worse than it looks: the inlined text would have to live in `state.toml`, but `SKILL.md`'s body is
human-owned, so you would get the same prose in two truth sources or a special case where *some*
skills' bodies are generated. Both break §2.1.

So para does not manage standing constraints at all. The `para-` prefix already means everything
without it is yours: write `.agents/rules/my-constraint.md` and para never reads or writes that file.

What that costs, stated plainly: your own rules get no scope-rewriting on rename, no `doctor` check,
and no journal. What it buys: one authored thing in the whole mechanism, one shape in
`.agents/rules/`, and no exception to §2.1. If the pain shows up, the promotion path is a skill —
which is the right shape for anything worth managing.

### 5.4 Para runs no matcher at write time

`scope` is an input to *rendering*. Para expands it into the sentence in the rule file that tells an
agent where the skill applies, and stops. There is no entity→skill index, nothing cached, nothing to
invalidate.

What that leaves para responsible for:

- **Rewriting scope entries on `move`, `archive`, `unarchive`, and id changes.** An enumerated
  locator is a second copy of a path — the exact thing v2 §1.4 refused — and this is the mitigation:
  para owns the rename.
- **Reporting unresolvable entries.** `doctor` flags a `scope` entry that names nothing. Globs
  degrade silently; an explicit list can be *checked*, which is the compensating benefit of
  enumeration rather than merely its tax.

Why enumeration at all, given v2 refused it: v2's skills lived *inside* the scope they applied to, so
placement expressed scope and no second copy existed. v4 centralises skills in `.agents/` because
that is where an agent looks for them, and centralisation makes placement unavailable as a
mechanism. Something has to carry scope; an explicit checkable list is the least-bad something.

---

## 6. `AGENTS.md` and `CLAUDE.md`

Emitted at the root, the four buckets, and `archive/{projects,areas,resources}`. **Nowhere else** —
not on entities, not on `objectives/`, not on `key-results/`.

That restriction is deliberate against the uniformity of §1.2. `AGENTS.md` exists to orient an agent
to the *framework*, and the root plus the buckets cover every concept there is. Per-objective prose
would be two more files per project plus one per objective, all saying nearly the same thing, in a
tree where these files are supposed to be as small as possible. Uniformity applies to the para
machinery, not to the documentation surface.

**Structure** is the frontmatter/body split applied to prose, since there is no frontmatter to use:

```markdown
<!-- para:begin — generated, do not edit; run `para rebuild` -->
This directory holds PARA **projects**: work with a finish line…
<!-- para:end -->

In this repo every project links its Jira epic in the README frontmatter.
```

Para owns what is between the markers, refreshes it on `rebuild`, and never touches a byte outside
them. That is what lets a new para version ship a better explanation to an existing tree without
eating your house rules.

**`AGENTS.md` never names the CLI** and never prints a locator. It describes the place. *(v2 §9.1,
kept — with the amendment that the root's block does explain the PARA framework, because that is the
one place a reader needs the concept.)*

### 6.1 `emit.claude` — the Claude Code compatibility surface

Everything in this subsection is **off by default** and turns on together, because it is one concern:
making the tree legible to Claude Code specifically. Enable it in the root's `config.toml`:

```toml
emit.claude = true
```

The flag is named for the surface, not for a file, because it emits more than one thing.

**`CLAUDE.md`** is marker-scoped, the same split §6 already gives `AGENTS.md` — because `CLAUDE.md` is
a path other tools write to as readily as para does, and a whole-file claim on a shared path is wrong
regardless of how little of the file para's half turns out to be. Para owns the block between the
markers and never touches a byte outside them; inside, it is a pointer file with no prose of its own,
carrying `@AGENTS.md` plus one `@` import per derived rule file:

```markdown
<!-- para:begin — generated, do not edit; run `para rebuild` -->
@AGENTS.md
@.agents/rules/para-signups-report.md
@.agents/rules/para-commit-style.md
<!-- para:end -->
```

Rules reach Claude Code by import rather than by being copied or linked anywhere. The import list
regenerates from the same scope walk that produces the rules (§5.4), so adding or removing a skill
keeps it correct with no separate bookkeeping, and there is no second copy of a rule to drift.

**`emit.claude` changes what it answers, not what it does.** Before, the flag answered "does
`CLAUDE.md` exist" — a whole-file question that only made sense when para believed it was the file's
only writer. Marker-scoped, it answers "is para's block present". The two questions have the same
answer in a tree where para is the only writer, which is why turning the flag on or off there is
unobservable at the file-presence level — an emptied file is deleted (§9 records the general rule,
which `emit.gitattributes` follows too). Where something else has written to the path, they diverge:
turning the flag off removes only para's lines, and whatever else was there stays.

**Skills are mirrored into `.claude/skills/`**, because an import cannot express a directory that
ships scripts and references. *How* they are mirrored is a second knob:

```toml
emit.claude        = true
emit.claude-skills = "symlink"   # or "copy"
```

Two flat keys rather than an `[emit.claude]` table, because TOML cannot hold both `emit.claude = true`
and a table at the same path. The shape is forced; do not "fix" it into a table.

| Mode | What lands in `.claude/skills/para-X` | Cost |
| --- | --- | --- |
| `symlink` (default) | a link to `../../.agents/skills/para-X` | breaks on checkouts that do not support links |
| `copy` | a full copy of the skill directory, minus `.para/` | doubles the bytes, and every skill edit shows up in two trees |

**`symlink` is the default** because it cannot drift and costs nothing: there is exactly one copy of
the skill on disk, so editing `SKILL.md` cannot leave a stale duplicate behind, and a skill edit
touches one tree in git rather than two. Its failure mode is confined to checkout: a
`core.symlinks=false` checkout — the Windows default — materialises each link as a plain file
containing its target path, which reads as a corrupt skill to anything that opens it. That failure is
loud rather than silent, because `doctor` reports it as `broken-link` and `rebuild` repairs it.

**`copy` is the escape hatch** for platforms and workflows where links do not survive. It trades
duplication for portability, and duplication is acceptable here for the same reason it is acceptable
in `MEASUREMENTS.csv`: a copy is a **projection**, and nothing ever reads it back as truth. Editing
the copy is not an error, just pointless — the next `rebuild` overwrites it, and `doctor` reports the
divergence as `stale-projection` before that happens.

**Auto-detecting the platform and choosing per machine is deliberately not an option.** It would make
the tree's shape depend on which machine last ran `rebuild`, so two contributors on different
platforms would fight over every commit. The mode is configuration, checked in, the same for everyone.

Three further consequences, all because a mirrored skill is the only artifact that is not a plain
generated file:

- **Ownership is a different test.** `generated_from` frontmatter cannot live on a symlink, so
  ownership is *anything under `.claude/skills/` whose name carries the `para-` prefix* — a link whose
  target resolves inside `.agents/skills/`, or a copy of a directory that exists there. Checkable
  either way, which is what matters, and `doctor` carries findings for orphaned and broken mirrors
  (§10).
- **`.para/` is never mirrored.** In `copy` mode it is excluded, and in `symlink` mode `doctor` does
  not follow para-owned links. Both rules exist for one reason: a reachable
  `.claude/skills/para-X/.para/state.toml` would satisfy §1.2's entity test, and `doctor`'s deep scan
  would report a phantom `orphan` entity living outside the tree.
- **Both modes are projections**: regenerated by `rebuild`, pruned when a skill is removed, and
  reported by `doctor` when they drift or dangle. Switching modes is a config change plus a
  `rebuild` — para removes the old shape and writes the new one.

Nothing else in the tree changes when the flag is on. `.claude/` holds only mirrored skills, and para
never reads or writes anything else under it.

### 6.2 `emit.cursor` — the Cursor IDE compatibility surface

Everything in this subsection is **off by default** and turns on together, because it is one concern:
making the tree legible to Cursor specifically. Enable it in the root's `config.toml`:

```toml
emit.cursor = true
```

The flag is named for the surface, not for a file, because it emits more than one thing. It is
**independent of `emit.claude`** — both may be on at once, and neither implies the other.

Cursor has no shared-path pointer file like `CLAUDE.md`. Rules reach Cursor as **`.cursor/rules/para-X.mdc`
files** — wholly generated, one per skill — and skills reach Cursor as **mirrors under
`.cursor/skills/`**, because a rule file cannot express a directory that ships scripts and references.
There is no `@`-import layer; the `.mdc` body points at the mirrored skill's `SKILL.md` directly.

**Rule files** live at `.cursor/rules/para-X.mdc`, where `X` is the skill id. Each file carries:

- **`alwaysApply: true`** when the skill's scope is the whole tree (no explicit `scope` entries, or
  scope that resolves to "everywhere");
- **`globs:`** otherwise — one `path/**` glob per resolved scope entry, comma-separated in the
  frontmatter Cursor reads;
- **`generated_from:`** — the skill id, same as §5.3's rule files, for `doctor`'s `orphan-rule`
  check (§10);
- a body that references `.cursor/skills/para-X/SKILL.md`, so Cursor loads the skill text from the
  mirror rather than from `.agents/`.

Scope is the same input §5.4 already defines: para expands enumerated locators into paths on `move`,
`archive`, and id changes, and `doctor` flags entries that name nothing. The difference from
`.agents/rules/para-X.md` is only the output shape — globs and `alwaysApply` rather than a prose
sentence — not a second copy of scope logic.

**Skills are mirrored into `.cursor/skills/`** under the same rules as §6.1's Claude mirror. *How*
they are mirrored is a second knob:

```toml
emit.cursor        = true
emit.cursor-skills = "symlink"   # or "copy"
```

Two flat keys rather than an `[emit.cursor]` table, for the same TOML collision reason §6.1 gives.
The shape is forced; do not "fix" it into a table.

| Mode | What lands in `.cursor/skills/para-X` | Cost |
| --- | --- | --- |
| `symlink` (default) | a link to `../../.agents/skills/para-X` | breaks on checkouts that do not support links |
| `copy` | a full copy of the skill directory, minus `.para/` | doubles the bytes, and every skill edit shows up in two trees |

**`symlink` is the default** for the same reasons as §6.1: one copy on disk, no drift, one tree in git.
**`copy` is the escape hatch** where links do not survive checkout. **Per-machine auto-detection is
deliberately not an option** — the mode is configuration, checked in, the same for everyone.

Three further consequences mirror §6.1 exactly, applied to `.cursor/skills/`:

- **Ownership is a different test.** `generated_from` frontmatter cannot live on a symlink, so
  ownership is *anything under `.cursor/skills/` whose name carries the `para-` prefix* — a link whose
  target resolves inside `.agents/skills/`, or a copy of a directory that exists there. `doctor`
  carries findings for orphaned and broken mirrors (§10).
- **`.para/` is never mirrored.** In `copy` mode it is excluded; in `symlink` mode `doctor` does not
  follow para-owned links — for the same phantom-entity reason §6.1 gives.
- **Both modes are projections**: regenerated by `rebuild`, pruned when a skill is removed, and
  reported by `doctor` when they drift or dangle. Switching modes is a config change plus a
  `rebuild` — para removes the old shape and writes the new one.

With `emit.cursor` off, every `para-` entry under `.cursor/skills/` and every `.cursor/rules/para-*.mdc`
is **residue** — the same posture §6.1 takes for a turned-off Claude surface. `doctor` names them;
`rebuild` removes them.

Nothing else in the tree changes when the flag is on. `.cursor/` holds only mirrored skills and
para-owned rule files, and para never reads or writes anything else under it.

---

## 7. Config

`config.toml` in any `.para/`, dotted keys, including the root's.

**Resolution walks the full ancestor chain, nearest wins.** Asking for `stale-after` on
`key-result.acme.q1-growth.signups` consults, in order: the key-result, its `key-results/` container,
the objective, `objectives/`, the project, `projects/`, and the root. First value found is the answer.

This is chained resolution, which §5.2 refused for rule scoping, and the difference is exact:
config resolves along **one axis** — a single chain where nearest wins — whereas rule scope would
have had **two sources** able to disagree. One chain is explainable in a sentence and printable as a
list. Two sources need a paragraph and a `--why` flag.

Consequently, the ability to print the chain and the winning value is not optional; it is what keeps
this defensible. A skill's chain is its own `config.toml`, then the root —
`.agents/` is not in the PARA tree, so there is nothing in between.

| Family | Example | Where it usefully lives |
| --- | --- | --- |
| `log.rotate-bytes` | `4194304` | root |
| `emit.claude` | `false` | root |
| `emit.claude-skills` | `"symlink"` (default) or `"copy"` | root |
| `emit.cursor` | `false` | root |
| `emit.cursor-skills` | `"symlink"` (default) or `"copy"` | root |
| `emit.gitattributes` | `true` | root |
| `<kind>.stale-after` | `project.stale-after = 14` | root, or any subtree |
| `key-result.at-risk-pace` | `0.8` | root, or one key-result |
| `review.cadence` | `90` | one skill |

`unset` removes a value and resolution continues up the chain. Unset everywhere means the check
never fires.

**`<kind>.stale-after` and `review.cadence` are day counts, and say so.** `config set` accepts a bare
non-negative integer as before, or the same count suffixed `d` (days) or `w` (weeks) — `30d`, `2w` —
both sugar over the identical stored integer, so an existing bare-number `config.toml` still reads
exactly as it always did. Every resolved display — `config list`, `config show`, and `show`'s own
`stale (…)` line — prints the value with its unit, `30 days` rather than a bare `30` that does not say
what it counts (para-xbb). `log.rotate-bytes` needed no such change: its unit is already in the key.

---

## 8. Truth files

### 8.1 The tree — `.para/tree.toml`, at the root and nowhere else

Schema version and tree identity, and nothing else. No counters, no caches, no index; everything
else is a walk.

```toml
schema       = 2
para-version = "0.7.0"          # the version that last wrote here
name         = "max's brain"
description  = "Everything I am carrying."
created      = "2026-01-01T16:00:00Z"
```

This is the root marker, and it is the root's state — the root has no `state.toml`, because the root
is not an entity, it is the tree. Its README frontmatter generates from here.

**`schema` is the migration gate**, and the reason it is stated here rather than inferred. A binary
reads it before anything else and refuses a tree it does not understand — `schema = 1` is a tree whose
entities carry no stored `kind` (§1.3), and no read or write proceeds until `para migrate` has
backfilled them (§30.5). `para-version` records who last wrote and is informational only: a tree
written by 0.6.1 and one written by 0.6.9 differ in nothing that matters to a reader, whereas the
schema number is the format contract itself. Gating on the version string instead would be asking a
changelog a question only the format can answer.

**Why a distinct marker, rather than testing for `.para/` and walking to the outermost one.** That
alternative is tempting — §1.5 guarantees the `.para/` chain is contiguous from the root down to any
entity — but the chain is deliberately broken under `.agents/`, which has no `.para/` of its own
because it is shared with rules para does not own. Walking up from `.agents/skills/para-x/` would stop
at the skill and conclude the skill directory is the tree root. Closing that without a marker means
either giving `.agents/` and its two subdirectories their own `.para/`, which claims a directory left
half-yours, or hardcoding "walk past directories named `.agents`, `rules`, `skills`" — a worse
exception than one named file.

Two lesser reasons, both real. The marker costs **no extra file**: the root's `.para/` holds two files
plus `logs/` either way, and the only question is whether the name tells the truth about contents that
genuinely differ — `schema` and `para-version` must be readable before anything else is parsed, and no
entity's `state.toml` holds anything of the kind. And **"outermost marker wins" inverts every
convention** — git, `package.json`, `pyproject.toml` all resolve to the nearest marker, whereas
nearest `.para/` is always the entity you happen to be standing in.

The root's journal is thin but real: `init`, config changes, and `child` events for the four buckets.

### 8.2 Container — `.para/state.toml`

```toml
kind        = "container"
name        = "Objectives"
description = "What acme-migration is trying to move."
created     = "2026-01-01T16:15:00Z"
```

Identity for the generated README frontmatter, and a place for its config sibling to hang. No status,
no rollups, no child list — the child list is `ls`.

A container carries a `kind` like anything else with a `state.toml` (§8.3, §30), and `"container"` is
the word for all of them — which is why the reserved *name* rule survives §1.3 (§1.2): the kind says
that this is a container, and the directory's name is what says *which*. `objectives/` may hold an
objective and `links/` may not, and no `kind` value distinguishes them.

### 8.3 Entity — `.para/state.toml`

```toml
# projects/acme-migration/.para/state.toml
kind     = "project"
name     = "Acme migration"
status   = "in-progress"
priority = "high"
due      = "2026-09-30"
tags     = ["kafka", "consumer"]
created  = "2026-01-01T16:15:00Z"
```

```toml
# …/key-results/signups/.para/state.toml
kind    = "key-result"
name    = "Weekly signups"
type    = "ratio"
start   = "480/9000"
target  = "2000/12000"
due     = "2026-09-30"
created = "2026-01-01T16:15:00Z"
```

```toml
# .agents/skills/para-signups-report/.para/state.toml
kind    = "skill"
name    = "Signups report"
scope   = ["objective.acme-migration.q1-growth", "area.growth"]
created = "2026-01-01T16:15:00Z"
```

**`scope` is the one field any `state.toml` stores an address in** — a skill's, per §5.1/§5.2 — and it
takes the dotted stored form §1.4 defines, the same as every other place an address must be a single
token.

**Present, and the only copy**: `kind` (§1.3, §30). It leads the file, because it is the one field
that has to be readable before anything else in the file means anything — a `type` or a `start` is
interpretable only once you know you are reading a key-result.

```toml
# areas/relationships/.para/state.toml
kind    = "area"
name    = "Relationships"
created = "2026-01-01T16:15:00Z"
```

**Absent by construction**: `id`, `parent`, `locator` — all in the path; `updated`, `current`,
`progress`, derived status — all read from the journal or computed, and none of them cached here. An
area's or resource's `archived` — that is the path too (§1.6).

**Present despite being computed**: `attention` and an entity's `[suppression]` table, if it has one
(§28). Both are written here by the same derivation that answers `review`'s and `show`'s questions,
never by `set`, and both are exactly as hand-editable as anything else in this file — which is to say,
legal, and pointless the moment the next mutation or a `rebuild` overwrites them (§19).

### 8.4 Uniform filenames, and the kind in exactly one place

`state.toml` and `config.toml`, in every `.para/`, for every kind. Not `project.toml`,
`key-result.toml`, `archive-projects.toml`.

The alternative — naming each file for its kind — reads pleasantly in a directory listing, and v3
sketched it that way. It loses on principle 1: the filename would be a **second copy of the kind**,
and two copies with nothing able to adjudicate is the failure v2 named as the load-bearing risk of the
whole design. And the filename repeats something that changes: it would have to be re-derived on every
rename into a different bucket, where `state.toml`'s own name never changes at all.

That second half is the part worth keeping precisely. A hand-`mv` leaves a stored `kind` exactly as
unrewritten as it would have left a filename — §1.5 and §30.4 say so, and call the result an
improvement over silently reinterpreting the thing — so "a hand-`mv` cannot rewrite it" was never the
argument that separated the two. What separates them is that one of them is a *second* copy.

**This section's argument is why `kind` lives in `state.toml` and not in the filename, and §1.3's
move did not weaken it.** The heading still means what it says: the kind is in exactly one place. What
changed is *which* place. When the path stated the kind, the filename would have been the second copy
and the path the first; now `state.toml` holds the only copy and the path states nothing to
contradict. The count is what mattered — one — and it is still one. A reader arriving here from §1.3
should not conclude that this section was overridden; it was the section that constrained how §1.3
was allowed to change.

Uniform names also make one rule serve everywhere: a projection generator, the walk, `doctor`, and
`rebuild` all open the same path relative to any `.para/` without first deciding what they are looking
at. The cost is that a directory listing no longer announces the kind. `state.toml`'s first line
states it outright and `README.md`'s frontmatter prints it for anyone browsing, but neither is visible
in an `ls`, and since §1.3 the position does not announce it either — so this cost is strictly higher
than it was when v3 sketched per-kind filenames, and it is still the right trade, for the counting
reason above rather than for a compensating announcement that no longer exists.

The single non-uniform file is `.para/tree.toml` at the root (§8.1), which names a different kind of
fact and exists once.

### 8.5 The walk

Read a directory's immediate children; descend into those holding `.para/state.toml`; never descend
into content. O(entities).

As of §30 it also **reads** the file whose presence it is already testing, and emits a locator→kind
index for the command's lifetime (§30.3) — a stat becomes a read, and the walk is where every other
caller's kind now comes from.

Its one weakness is v2's, unchanged: hand-`mv` an entity into a content subdirectory and it vanishes
from every read. `doctor` is the deep scan that finds it, and §1.5 makes it an error rather than a
curiosity.

---

## 9. Git

**Git is not a precondition.** No clean-tree check, no refusal to mutate a dirty tree, and para
**never invokes git** — not to stage, not to commit, not behind a config flag. It writes files. What
you do with them is yours.

The one thing para writes for git's benefit is `.gitattributes`, on by default and disableable with
`emit.gitattributes = false`:

```
# journals: order lives in the data, so keeping both sides is the definition of the merge
**/logs/*.jsonl merge=union

# wholly generated: any side is as good as any other, because `para rebuild` produces the truth
**/ACTIVITY.md          merge=ours linguist-generated=true
**/MEASUREMENTS.csv     merge=ours linguist-generated=true
.agents/rules/**/*.md   merge=ours linguist-generated=true
.cursor/rules/para-*.mdc merge=ours linguist-generated=true
```

`CLAUDE.md` and `AGENTS.md` carry no line here, and for the same reason: git attributes are per-file
and cannot be scoped to a marker range, so no whole-file attribute is correct for a path para shares
with something else (§2.2).

Two arguments, both narrow enough to be safe:

- **Union merge for journals** is not a heuristic, it is the definition of merging two append streams
  whose order is carried in the data. *(v2 §6.4.)*
- **`merge=ours` for wholly generated files** is not a heuristic either, because the file is fully
  derivable: whichever side wins, `para rebuild` produces the correct bytes. There is no judgement to
  make, which is exactly the test ADR 0001 applied when it declined a merge driver.

**Symlinks get no `.gitattributes` treatment**, because there is nothing useful to say about merging
one. A conflicting or wrongly-materialised link is fixed by `para rebuild`, which is also the repair
for a `core.symlinks=false` checkout (§6.1). Copied skills, being ordinary generated files, take the
`merge=ours` line above.

**Partly generated files are deliberately left to conflict normally** — `README.md`, `SKILL.md`,
`AGENTS.md`, `CLAUDE.md`. Their human-authored bodies are at stake, and no automatic rule may choose
between two people's prose.

**Turning a block-scoped file's flag off deletes the file once the block's removal empties it.** This
applies uniformly to `CLAUDE.md` (`emit.claude`) and `.gitattributes` (`emit.gitattributes`). The
alternative — leave a zero-byte file behind — sounds safer but isn't: a file with nothing in it carries
no information, so deleting it destroys nothing, while *not* deleting it means every tree that ever
turns the flag off accumulates an empty file nothing will remove. Deletion happens only when removing
para's block leaves the file empty; a file still holding a third party's content is only ever
shortened, never deleted, whatever the flag says.

The resolution procedure, which belongs in `AGENTS.md`'s generated block so an agent knows it:
**resolve the truth files and the journals, then run `para rebuild`.** Never hand-resolve a
projection.

---

## 10. What `doctor` must be able to find

The verb is specified in the second half; these are the findings the *model* makes necessary, so that
the model can be checked against them.

**Errors** — a read would be wrong or incomplete.

| Finding | Means |
| --- | --- |
| `orphan` | a `state.toml` the fast walk cannot reach — an entity beneath content (§1.5) |
| `misplaced` | a `state.toml` whose stored `kind` is not allowed where it sits, against §30.2's containment table — e.g. a project at depth 2 under `projects/`, or a key-result outside a `key-results/` container |
| `no-kind` | a `state.toml` in a `schema = 2` tree with no `kind` key at all — there is nothing to read the entity as (§30). A `kind` that is *present* but outside the vocabulary is `invalid` above, where every other closed vocabulary's out-of-range value already goes |
| `invalid` | unparseable TOML, missing required field — `kind` excepted, which is `no-kind` below — enum out of range — a stored `kind` outside §30's vocabulary among them — a measurement whose shape contradicts its key-result's `type`, a skill with no `description` (it would be inert) |
| `journal` | a line that is not valid JSON, or has no `at`/`kind`, or an unknown `kind`, or an `at` in the future — reported with file and line |
| `collision` | a reserved name used as an id (§1.4) |
| `scope-unresolved` | a `scope` entry naming a locator that does not exist (§5.4) |
| `orphan-rule` | a derived rule file — under `.agents/rules/` or `.cursor/rules/` — whose `generated_from` skill is gone (§5.3, §6.2) |
| `orphan-mirror` | something under `.claude/skills/` or `.cursor/skills/` carrying the `para-` prefix whose skill is gone (§6.1, §6.2) |
| `broken-link` | a `symlink`-mode mirror that does not resolve — including one materialised as a plain file by a `core.symlinks=false` checkout (§6.1, §6.2) |
| `stale-projection` | a generated file differs from what would be written now → `para rebuild` (§2.4) |

`stale-projection` is the load-bearing one, because principle 3 rests on it, and it is **full
fidelity**: every generated file is re-derived in memory from truth and compared byte for byte, with
`ACTIVITY.md` re-derived from *every* journal file backing it rather than only the newest. Write-through
guarantees today; this check is what guarantees every day before it (§3.5). Its report names the file
and, for `ACTIVITY.md`, the earliest day that differs — so drift is dated rather than merely detected.

As of §28, the same finding also covers `state.toml`'s computed fields — `attention` and
`[suppression]` — compared against the same derivation the entity's own `state.toml` write and
`review`/`show` all call. This is the same partial-file comparison `README.md`'s frontmatter already
gets, not a new mechanism: a file can be truth in one part and checked like a projection in another.

**Advisory** — your filing is loose.

| Finding | Means |
| --- | --- |
| `untracked` | a plain directory sitting **directly** in a bucket — the one place only entities belong. Content inside an entity is content and is never reported. |

Gone from v2's list: `target` (no emit drivers), `depth` (no depth limits), and v2's own
`stale-emit`, renamed `stale-projection` and promoted from a curiosity to the guard on principle 3.
Returned from v1: a projection finding, because there are projections again.

---

## 11. Decision log — v4

Settled in the session that produced this half. Nothing structural is open.

**The pivot itself**

- **Directory-per-entity is for readers, not for para.** GitHub-level inspectability and `.agents/`
  navigability. Not a repair of anything broken in v2.
- **`.para/state.toml` is the sole source of truth; every human-facing file is generated from it.**
- **Uniform filenames**: `state.toml` and `config.toml` in every `.para/`, with `tree.toml` at the
  root as the one marker. The kind is written in exactly one place, and as of §30 that place is
  `state.toml` rather than the path (§8.4).
- **Write-through, not deferred emit.** No `emit` verb, no dirty flag; `rebuild` is the renderer and
  the repair path.
- **Para owns frontmatter, humans own bodies**, with no exceptions — wholly generated files have no
  body anyone else owns.

**Journal**

- **One home per event**, with parents logging only their own containment changes. Full propagation
  and cause-pointer propagation both rejected, with reasons (§3.2).
- **Rollups are read-time, never stored.**
- **JSONL, `.jsonl`, rotated on size, filename = first event's timestamp.** Human readability is
  `ACTIVITY.md`'s job.
- **`ACTIVITY.md` is newest-day-first, and only today's section is re-derived per mutation** — with a
  full-fidelity `doctor` check over every backing journal, because prior days are otherwise never
  recomputed and therefore never verified (§3.5, §10).
- **One clock unchanged from v2**; `child` events do not reset it.
- **No `actor` field** until something reads it.

**Skills and rules**

- **`para-` prefix on the containing directory** as the namespace. No dot-directories inside
  `.agents/`.
- **Scope lives on the skill**, as a TOML array of fully-qualified locators.
- **A scope entry covers its subtree.** No exclusions in v4.
- **Location and kind only — never tags.**
- **Omitting scope means the whole tree.** No wildcard spelling.
- **Skills have no `enabled` flag and no status.** The `.agents/` convention has none, so para invents
  none: a skill that exists applies, and `remove` is the off switch.
- **No entity-side wiring.** v3's "entity config references its skills" is deleted.
- **A rule is only ever a projection of a skill** — a bare `.md` with `generated_from`, no `.para/`,
   no locator, nothing to author. Authored rule entities were designed and then deleted: every field
   they would hold already lives on the skill (§5.3).
- **Standing constraints are unmanaged.** Write them in `.agents/rules/` without the `para-` prefix
  and para never touches them. Accepted loss: no scope-rewriting, no `doctor` check.
- **Skills are addressable as `skill.<id>`**, so every universal verb works on them and the command
  surface grows by nothing (§1.4, §14).
- **Para runs no matcher at write time**; it rewrites scope entries on move/rename/archive and
  `doctor` reports unresolvable ones.

**Structure**

- **`objectives/` and `key-results/` are both tracked containers**, uniform with the buckets.
- **Locator = path, always — superseded by §1.4's noun+chain address (§12).** `Locator` remains the
  internal path-with-dots-for-slashes representation; external addressing now elides the container
  segments via the noun, which this original decision predates.
- **Entities may not live beneath untracked directories**; untracked directories are invisible;
  **adoption/`import` does not survive**.
- **Archive is a place.** Locators change on archival, subtrees move whole, unarchive cascades
  upward, id collisions are hard errors, and stubs preserve ancestry.
- **Areas and resources lose their status field** — location is archival state.
- **`AGENTS.md` only at root, the four buckets, and `archive/{projects,areas,resources}`**, with a
  delimited para-owned block.
- **`emit.claude` is one flag for one concern** — Claude Code compatibility, off by default. It writes
  a block into `CLAUDE.md` (pointing at `AGENTS.md` and `@`-importing every derived rule) and one
  symlink per skill into `.claude/skills/`. Rules travel by import, not by symlink, so the only
  mirrored things are the ones an import cannot express (§6.1).
- **`emit.claude-skills` chooses `symlink` (default) or `copy`.** Symlink cannot drift and does not
  double the bytes; copy is the escape hatch where links do not survive checkout. **Per-machine
  auto-detection was rejected** — it would make the tree's shape depend on which machine last ran
  `rebuild`. Two flat config keys, because TOML cannot hold `emit.claude = true` and an
  `[emit.claude]` table at once.
- **`emit.cursor` is one flag for one concern** — Cursor IDE compatibility, off by default,
  independent of `emit.claude`. It writes one `.cursor/rules/para-X.mdc` per skill and one mirror
  per skill into `.cursor/skills/`. Rules travel as `.mdc` files with globs or `alwaysApply`, not by
  import — so the mirrored skills are the ones a rule reference cannot replace (§6.2).
- **`emit.cursor-skills` chooses `symlink` (default) or `copy`**, under the same rules and for the
  same reasons as `emit.claude-skills`.

**Config and git**

- **Config resolves up the full ancestor chain, nearest wins**, and the chain must be printable.
- **No git precondition, and para never invokes git.**
- **`.gitattributes` on by default**: `merge=union` for journals, `merge=ours` for wholly generated
  files, normal merge for partly generated ones.

**Decided while drafting, and easy to overturn**

- **Four event kinds**, with `child` as the containment kind and `note` as a field on all of them.
- **`MEASUREMENTS.csv` columns**: `at,value,decimal,progress,note`.
- **`log.rotate-bytes` default 4 MiB.**
- **Containers carry `name`, `description`, `created` and nothing else.**
- **`doctor` finding names** in §10.

---

## 12. Reversals from v2, and what each one costs

Carried here rather than buried, because each is a decision v2 argued for in the opposite direction
and a future reader deserves both sides.

| v2 said | v4 says | What it costs |
| --- | --- | --- |
| Nothing is projected. No caches, no `rebuild`. | Everything human-facing is projected, write-through, with `rebuild`. | `rebuild`, a `stale-projection` finding, a merge story, and hand-`mv` no longer being simply correct. Bought: a tree that explains itself to a reader with no tooling. |
| Placement expresses scope; an `applies-to` locator list is a second copy every rename breaks. | Scope is an explicit locator list on the skill. | Para must rewrite scope entries on every move; `doctor` must check them. Bought: skills where an agent actually looks for them. |
| Objectives and key-results are rows, not directories. | Both are directories, under tracked containers. | Six-segment locators, four more files per key-result, and empty-ish directories. Bought: an OKR you can browse, diff, and link to on GitHub. |
| No archive bucket. Status archives; locators are stable for life. | `archive/` is a place; archiving moves bytes. | Locators are no longer stable for life, and stubs are an exception to locator↔entity correspondence. Bought: "where did it go" answerable by looking. |
| Skills are not entities. | Skills are entities. | A `.para/` per skill. Bought: scope and tags get a history. |
| Three buckets. | Four. | One more reserved name. |
| Kind, id, parent and locator all come from the path; a state file records none of them. | Id, parent and locator still do. Kind is stored in `state.toml` (§30). | A `schema` bump and one `migrate` run per tree; asking a bare path what it is now needs the walk's kind index; a new `doctor` finding for a kind illegal in its position. Bought: a kind whose position implies nothing, one kind mapping where there were two, adoption becoming expressible, and a hand-`mv` that no longer silently changes what a thing is. |
| `import`/adoption exists. | It does not. | The "point para at my existing repo" story. Bought: entities-all-the-way-up, and therefore locator↔path isomorphism. |
| Git is a precondition. | It is not, and para never invokes git. | Nothing. v2's precondition guarded a projection-merge problem that `merge=ours` now answers. |
| One `entity.md` per entity, fields in its frontmatter. | Two truth files — `state.toml` and `config.toml` — and a generated `README.md`. | Three files where there was one, and a `config.toml` that is usually empty. Bought: policy separable from identity at any level of the tree, and a README body that is yours. |
| Nouns partition the command surface: `para <noun> <verb> <locator>`. | The noun is restored, but as an argument after the verb, not a leading sub-command: `para <verb> <noun> <chain>`. | A nineteen-word reserved list, stubs no longer addressable, and one lookahead rule in `list`. Bought: per-kind flags, per-kind help, per-kind completion, and a kind filter on `list`. |
| `CLAUDE.md` wholly generated, no body to own. | `CLAUDE.md` marker-scoped, exactly like `AGENTS.md`. | A marker pair in a pointer file, and `emit.claude` no longer implies the file's absence. Bought: composability with every other tool that writes to the same path, and no silent `merge=ours` loss. |

**This one has an intermediate step the other rows don't.** v4 first reversed v2's row above to a bare
`para <verb> <locator>` — no noun at all — recorded in §25 as "Verb-first with locators, not
noun-verb," reasoned as "the locator carries the kind, so the verb never needs to." This amendment
reverses *that* decision, landing the noun back in the grammar, but not where v2 had it: v2's noun led
as a sub-command choosing among per-noun verbs; this amendment's noun follows the verb as an ordinary
argument, because the locator still carries the kind for the machine — the noun's job is partitioning
the help, the flags, and the completions for a reader, which §25 never weighed when it dropped the
noun the first time.

**The `CLAUDE.md` row is not a v2 reversal either — v2 predates Claude Code compatibility (§11), so
there is no v2 position to reverse.** It sits in this table anyway because the shape — what changed,
what it costs, what it buys — is the same shape as every other row.

Kept from v2 without amendment, so nobody re-litigates them: id, parent, and locator all derived from
the path and written nowhere else; one locator form; setting a field to its current value writes nothing; pushing
a due date must not buy quiet; `missed` is not terminal; terminal status cascades by derivation;
derived key-result status does not latch; word operators rather than `&`/`|` in any filter; all three
key-result types; one clock and two knobs; no `max-depth`; union merge for append streams; generated
output declares itself in-band rather than via a manifest.

---

## 13. The verb set

Twenty-two commands, twenty-five shapes counting `config`'s four. Every one of them is derived from the
model above rather than inherited: where a v2 verb survives, it survives because §1–§10 still needs
it, and where it does not, §13.1 says what deleted it. `suppress`/`unsuppress` (§28) are the two added
after v4 was otherwise settled.

`<noun>` is one of §1.4's eight; `<chain>` is an id-chain, and it is *short* — the noun carries the
container segments, so the chain is not the whole path (§1.4).

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
| `suppress` | `para suppress <noun> <chain> --until <date> --note "…"` |
| `unsuppress` | `para unsuppress <noun> <chain> --note "…"` |
| `log` | `para log <noun> [<chain>] [--kind …] [--limit n] [--reverse]` |
| `activity` | `para activity [<noun> [<chain>]] [--recursive] [--since …]` |
| `review` | `para review [<noun> [<chain>]] [--stale｜--blocked｜--overdue｜--behind｜--skills｜--suppressed]` |
| `rebuild` | `para rebuild [<noun> [<chain>]] [--dry-run]` |
| `path` | `para path <noun> [<chain>]` |
| `doctor` | `para doctor [<noun> [<chain>]]` |
| `migrate` | `para migrate [--dry-run]` |
| `config` | `set` / `unset` / `list [--prefix …]` / `show <key> [<noun>.<chain>]` |

Five are new against v2, and each is demanded by a specific decision in the first half:

- **`archive` / `unarchive`** — archival moves bytes now (§1.6), so it is an operation rather than a
  field value. `move` cannot serve: two spellings for one thing violates principle 5, so `move`
  refuses to cross the archive boundary and these two verbs own it.
- **`rebuild`** — projections exist (§2.4), so something has to render them from truth.
- **`activity`** — §3.2 promised the recursive rollup would be a read-time command, and this is it.
  Nothing about it is stored.
- **`migrate`** — `schema` is a format contract a binary must be able to refuse (§8.1), and a refusal
  is only defensible if something can satisfy it (§30.5). It takes no noun and no chain because it
  operates on the tree itself rather than on anything in it, which is why `init` is the only other
  command shaped that way.

Three shapes in the table above are irregular, and each is bought by a constraint the model already
enforces rather than by convenience:

- **`measure` takes no noun.** Only a key-result can be measured, so a noun would carry no
  information beyond what the bare chain already does — `para measure acme.q1-growth.signups
  880/11000`. This is the one command whose address is a bare chain, and it is worth the
  irregularity: `measure` is the highest-frequency write in the tool.
- **`move` speaks its noun once**, not once per chain, because §18.3 makes `move` same-kind only — a
  second noun could only ever repeat the first or be a refusal, so the table gives it one:
  `para move area health.training fitness.training`, `para move project acme acme-migration`.
- **`add` refuses the `container` noun.** Containers are created eagerly by `add` on their parent
  (§18.1) and are never created directly, so `container` is a legal noun everywhere in the table
  above except here.

### 13.1 What did not survive, and what deleted it

- **`emit`** — deleted by write-through (§2.3). There is no state in which projections are pending, so
  there is nothing to trigger. `rebuild` inherits the only job `emit` still had, and is honest about
  being repair rather than routine.
- **`para skill add|list|remove`** — deleted by skills becoming entities (§5.1) and getting addresses
  (§1.4). v2 justified the sub-noun on the grounds that a skill shared none of the field vocabulary;
  in v4 it shares `name`, `description`, `tags`, `created`, and has a journal. `para add skill
  signups-report --scope …` is the same verb doing the same job, so three shapes disappear
  without anything replacing them.
- **`para rule …`** — never existed, because a rule is a projection (§5.3). Nothing addresses a
  projection.
- **`import`** — deleted by §1.5. There is no adoption, so there is nothing to import.
- **`search`** — deleted by v2 already: `list --match` is cross-kind and is the same command.
- **`--body` and any `$EDITOR` invocation** — para never opens an editor and never takes prose on the
  command line. Bodies are files; you edit files with your editor. v2's principle 3 said the
  filesystem already does this better, and that principle survived the pivot even though principle 2
  did not.
- **`para rules <noun> [<chain>]`** — considered and folded into `show`, which already has a line for
  the skills that apply (§16.1). A verb whose whole output is one line of another verb is not a verb.

---

## 14. Addressing

Every command takes the two-token form §1.4 defines — a noun, then a short id-chain — and every
command prints and serializes it as the one-token dotted form, so anything you read pastes into
anything you type. One irregularity survives from before nouns existed, and one is new:

- **`.` survives unchanged as a whole address**, replacing `<noun> [<chain>]` or `<noun> <chain>`
  wholesale rather than filling one slot of it: `para show .`, `para note . "text"`. It is
  self-describing — it resolves by walking up from `$PWD` to the nearest ancestor with a
  `.para/state.toml`, the same upward-ancestor-chain style §7 uses to resolve a config value — so it
  takes **no noun**. On a command like `note`, where both a noun and a chain are
  otherwise mandatory, that is one token standing in for two; on a command like `show`, where the chain
  was already optional, `.` is not reducing an argument count so much as supplying a token that is not
  drawn from the eight-word noun vocabulary at all. Cheap either way, and it is the difference between
  para being usable from inside a project and not.
- **`.` at the tree root is `show`'s alone to answer with the tree itself, not an error** (para-xbb).
  Every other container and entity satisfies "." by holding its own `.para/state.toml`; the root holds
  `tree.toml` instead; and standing there, or anywhere the walk reaches the root without finding one,
  used to refuse with "no entity or container contains …" — true, but read by a new user as "this tree
  is not set up," when the root is exactly the kind of thing `.` already promises: "the thing containing
  the working directory." `para show .` at the root prints that thing — name, created date, config, and
  its container children — the same summary any other `show` gives. Every other verb `.` reaches keeps
  the refusal: `para note .` at the root still has nothing to attach a note to.
- **A noun with no chain is the bucket.** `para list project`, `para doctor area`, `para path skill`.
  This collapses "filter by kind" and "the bucket" into one idea (§1.4's table gives the four buckets'
  paths this way), and it is accepted by every command whose argument the table below calls "entity,
  container, or bucket" — the bucket being a legitimate thing to log, roll up, or render a bare path
  for — and by `list`, a place to look inside (§16.2).
- **`para path <noun> [<chain>]`** is the inverse of the noun+chain → path mapping, printing one bare
  line shaped for `$(…)`. A stub is the one thing `path` cannot produce a line for, because a stub has
  no noun and therefore no address to resolve in the first place (§1.6) — there is nothing to type, not
  merely something `path` fails on.

What each command accepts as its argument:

| Argument | Verbs |
| --- | --- |
| entity (bare noun refused by name) | `add`, `set`, `unset`, `move`, `remove`, `archive`, `unarchive`, `note` |
| entity, container, or bucket — a bare noun is the bucket; `activity`, `rebuild`, `doctor`, `review` also take nothing at all, meaning the root | `show`, `log`, `activity`, `path`, `rebuild`, `doctor`, `review` |
| entity, container, bucket, or root — a place to look inside, by §16.2's own lookahead | `list` |
| a bare id-chain, no noun | `measure` |

**Naming a container where an entity is required is an error that says so**: containers hold `name`,
`description`, and `created` and nothing you would want to set (§8.2). **Naming a bucket where an
entity is required is the same refusal, by name** — `para add project` or `para remove skill` name a
kind, not a thing, the same way naming a container does. This is independent of `add` banning the
`container` noun outright (§13): that refusal bars the noun itself, for any chain, because `container`
has no bucket form to begin with (§1.4's bucket rows are project, area, resource, and skill only) —
`para add container acme.objectives` is refused by noun, not by an empty chain.

**No arguments at all, where the command allows it, is the tree root**, unchanged from before nouns
existed. `list`, `activity`, `review`, `rebuild`, and `doctor` all take a fully optional noun and
chain (§13), so any of them run with nothing after the verb and mean the whole tree: `para list`,
`para doctor`, `para review`, and so on.

---

## 15. Field vocabulary

One spelling per field, shared by `add` and `set`. `unset` takes bare names.

**`kind` is deliberately not a row here**, though every `state.toml` now carries one (§8.3, §30). This
table is the vocabulary `add` and `set` dispatch on, and a kind is not settable: the noun supplies it
(§0 principle 1), so a row would imply a `--kind` flag that does not and must not exist. Its
vocabulary lives in §30.2 and its out-of-range value is `invalid` (§10) on the same rule every closed
field here is held to — it is only the *settability* that differs.

| Field | project | area | resource | objective | key-result | link | skill | container |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `name` | req | req | req | req | req | opt (id) | req | req |
| `description` | req | req | req | req | opt | opt | req | req |
| `status` | opt | — | — | opt | derived (`dropped` settable) | — | — | — |
| `priority` | opt | opt | — | opt | — | — | — | — |
| `due` | opt | — | — | opt | opt | — | — | — |
| `tags` | opt | opt | opt | opt | opt | opt | opt | — |
| `created` | opt (now) | opt (now) | opt (now) | opt (now) | opt (now) | opt (now) | opt (now) | opt (now) |
| `type` | — | — | — | — | **req, fixed** | **req, fixed** | — | — |
| `start` | — | — | — | — | opt | — | — | — |
| `target` | — | — | — | — | req | — | — | — |
| `scope` | — | — | — | — | — | — | opt (all) | — |
| `ref` | — | — | — | — | — | req | — | — |
| `direction` | — | — | — | — | — | req | — | — |

- **`description` replaces v2's `summary`**, and it is a field rather than the body because it has to
  reach README frontmatter, `list` output, `--match`, and a derived rule's routing sentence. The
  *body* of `README.md` is the long version and is yours (§2.1). One line in truth, unlimited prose
  in the file.
- **`stale-after` and `at-risk-pace` are no longer fields.** v2 needed them per-entity because config
  was root-only; §7's ancestor chain subsumes that entirely — `para config set project.stale-after 30`
  inside a project's `config.toml` does what a per-entity field used to. Two fields deleted by a
  decision made in the other half, which is the kind of saving worth pointing at.
- **`created`** defaults to now, is settable, and takes `--at`'s progressive precision (§15.1). Two
  bounds and no others: not in the future, and not after `due`. `unset created` is an error —
  everything has a creation time.
- **`type` is fixed at creation** and never settable; changing it would invalidate every measurement
  already logged (§4.1). Delete and recreate. A link's `type` is fixed for the same reason in spirit
  but a different one in fact (§29.2): nothing measures it, but it is the tag tooling built on `para`
  keys its own interpretation off of, and letting it change out from under that tooling silently would
  be the same "the label lied about what's underneath" failure with a different reader.
- **`name` is Optional on `link` alone** (§29.2) — it defaults to the link's own id, because a link's
  slug is usually name enough and requiring one besides would be a second required flag for a fact
  the chain already states.
- **`scope` replaces wholly on `set`**, and `para unset skill x scope` widens it back to the whole
  tree (§5.2). `para set skill x --scope a,b` is the full new list.
  `--scope-add` / `--scope-remove` are deliberately absent: every other field replaces, and a list
  short enough to be worth managing by hand is short enough to retype. If scope lists get long enough
  that this hurts, that is evidence the skill is miscarved (§5.2).
- Naming a field a kind does not have is an error. **Setting a field to the value it already holds
  writes nothing and exits 0** — no event, no projection rewrite, no clock movement. Without that
  rule a shell loop buys permanent silence from `review`. *(v1's find, kept by v2, kept here.)*

### 15.1 `--at` and progressive precision

`--at` (on `note`, `measure`, and `created`) accepts increasing precision, and everything absent is
zero-filled in the local offset: `2026-01-03`, `2026-01-03T09`, `2026-01-03T09:02`,
`2026-01-03T09:02:11`, or a full RFC 3339 timestamp with an explicit offset. A date alone is midnight
local.

Bounds: never in the future. `measure --at` must not collide with an existing measurement on the same
key-result (§3.1); notes and changes may collide freely.

**`due` alone accepts two precisions coarser than this floor**: a bare year and a year-month, each
meaning by the end of that period — `2027` is the last instant of 2027, `2027-04` the last instant of
April. A deadline that granular is a real plan's honest resolution, and forcing it to a fuller date
would invent precision that was never there (para-xbb). `--at` itself stays floored at a full date:
"by the end of April" answers when something is due, not when it happened.

**What you type is local; what is stored is UTC.** The zero-filling above happens in your offset, and
the resolved instant is then written as UTC — so `--at 2026-01-03` in `-08:00` stores
`2026-01-03T08:00:00Z`. One representation on disk means one answer to "which day is this" for every
reader of a committed file (§3.5), and no comparison anywhere has to reason about two offsets. Errors
and read commands convert back for display, so the round trip is invisible unless you look in the
file.

---

## 16. Reading

### 16.1 `show`

Prints the thing: stored fields, what is derived at read time, its children in summary, and the skills
whose scope reaches it. It does **not** print the journal (`log`), the digest (`activity`), or its
siblings (`list`).

```
$ para show project acme-migration
project      acme-migration
Acme migration
  Rebuild the consumer so it stops falling over under replay load.

status       in-progress
priority     high
due          2026-09-30        in 181 days
tags         consumer, kafka
created      2026-01-01
attention    2026-03-02        31 days ago
             stale (stale-after 14 days, from .para/config.toml)

objectives
  q1-growth  Grow signups                          in-progress
    signups  Weekly signups                        at-risk
             480/9000 → 880/11000 / 2000/12000     progress 0.24   pace 0.70

skills       signups-report (from skill.signups-report, scope project)
```

- Derived values announce themselves by being *computed lines* — `in 181 days`, `31 days ago`, `stale`,
  `progress`, `pace`, and a key-result's status are never stored (§2.5).
- **`stale` names where its threshold came from**, because §7's chain resolution is only defensible if
  it is visible (§7). Same for any other resolved knob `show` reports.
- **`attention` also names what set it** — `note` or `measurement`, and a note's own text — next to its
  `days ago` (§3.6, §20), so a reader does not have to open `ACTIVITY.md` to find out.
- **The `skills` line is `para rules` folded in** (§13.1): each entry names the skill and the scope
  entry that reached it, so "why is this rule in my context" is answerable without a second command.
- `show` says when an entity is dormant under a terminal ancestor or lives under `archive/`, because
  its own fields do not explain why it stopped appearing in `list` (§1.6, §1.7).
- `show` is unaffected by terminal-status hiding. You named the thing.
- **`para show .` at the tree root summarises the tree itself** (§14, para-xbb) rather than refusing:

```
$ para show .
tree  max's brain
Everything I am carrying.

created      2026-01-01
config       3 keys set — see `para config list`

containers
projects   Projects    —
areas      Areas       —
resources  Resources   —
archive    Archive     —
```

  Its containers print flush left rather than indented under a name the way `q1-growth` is under
  `acme-migration` above: they are the four buckets §8.1's own `child` events name, and every one of
  them is a reserved locator segment on its own, which is what makes a child's *depth* — the same
  count `list` and `show` compute everywhere else — zero here rather than one.

### 16.2 `list`

```
para list [<kind>] [<noun> [<chain>]] [filters]
```

Lists **entities** beneath the given scope, at any depth, defaulting to the whole tree. `<kind>` and
`<noun>` above are the same eight-word vocabulary (§1.4) filling two different roles — a filter, and a
scope — which is exactly what makes `list` the one command whose meaning depends on lookahead: the
rule is stated rather than discovered. Scan left to right:

| Args | Reading |
| --- | --- |
| *(none)* | the whole tree |
| `<noun>` | that kind, tree-wide — which for the four buckets is the bucket |
| `<noun> <noun>` | first is a kind filter, second is a bucket scope |
| `<noun> <chain>` | the pair is a scope |
| `<noun> <noun> <chain>` | first is a kind filter, the pair is a scope |

```
para list                          # everything
para list project                  # every project
para list key-result               # every key-result, tree-wide
para list project acme             # inside project acme
para list key-result project       # key-results anywhere under projects/
para list key-result project acme  # key-results inside project acme
```

- **The kind-filter position accepts the seven addressable kinds only.** `container` is not a legal
  filter: containers are transparent to `list` and are never rows (§25), so filtering for one would
  ask for something the output can never contain.
- **This subsumes a capability `list` did not have before**: a kind filter, tree-wide or scoped, is
  new rather than a reshuffle of what dotted-plural locators already let you ask for.
- **Containers are transparent.** `objectives/` and `key-results/` are never rows in the output and
  are always traversed through, because a row you can neither set nor act on is noise. Name one
  explicitly with `show` when you want it.
- `archive/` is not traversed unless `--archived` says so: `para list project --archived` for
  archived projects, `para list --archived` for the whole archive. Archived things are not hidden,
  they are simply somewhere else, which is the whole point of §1.6.
- Terminal-status items are hidden unless `--all`.
- **A row carries a bare `note`/`measurement` marker for what set `attention`** (§3.6, §20), or `—` when
  nothing has beaten `created` yet — terser than `review`'s own column, to keep `list` at one row per
  line.
- **`list` and `show` print the noun as its own column**, not folded into the address, because the
  column is what makes a mixed-kind listing scannable:

```
$ para list project acme
project     acme                      in-progress  2d
objective   acme.q1-growth            in-progress  2d
key-result  acme.q1-growth.signups    at-risk      1d
showing 3 of 3
```

### 16.2.1 Timestamps in output, and `--local`

Every timestamp para *stores* is UTC and every timestamp it *generates into a file* is UTC (§3.5,
§15.1). Terminal output is the one place that is negotiable, because nothing compares it and nothing
commits it:

```
para show project acme-migration --local
para log project acme-migration --local
```

`--local` converts every timestamp in the output to the reader's own zone, with `$PARA_TZ` overriding
the host zone. It is available on every read command — `show`, `list`, `log`, `activity`, `review` —
and on nothing that writes, because a mutation's job is to record an instant, not to render one.

It is a flag and not a config key on purpose. Config is checked in and the same for everyone (§6.1's
argument against per-machine behaviour applies unchanged), whereas which zone you want to read in is a
property of *you*, not of the tree. `$PARA_TZ` covers the case where you always want it.

### 16.3 `log`

Prints the raw journal for one entity — the JSONL, rendered as lines, **newest first**, matching
`ACTIVITY.md`'s direction so the two never read in opposite orders. `--reverse` for chronological,
`--kind` to filter, `--limit` to truncate, `--json` to get the events back unchanged.

`log` reads every rotated file for that entity, in order. It is the one command that does.

### 16.4 `activity`

Prints the digest — the same fold `ACTIVITY.md` contains (§3.5) — for an entity or container.

```
para activity project acme-migration --recursive --since 2026-01-01
```

`--recursive` merges the digests of everything beneath into one chronology, each line labelled with
the locator it came from. **This is the only rollup in the system, it is computed on demand, and
nothing about it is written to disk** (§3.2). Without `--recursive` it is just `cat` on a generated
file, and that is fine: it means the two agree by construction.

---

## 17. Filters, sorting, limiting

On `list`, and on `log`/`activity`/`review` where the flag makes sense.

| Flag | Meaning |
| --- | --- |
| `--tags <expr>` | boolean expression over tags — see below |
| `--match <text>` | the thing's own text: name, description, tags, journal note bodies (see below) |
| `--status <s>` | filters on **effective** status (§1.7) |
| `--priority <p>` | only on kinds that have one |
| `--overdue` | open and past `due` |
| `--direct` | immediate containment **after** container transparency; default matches at any depth |
| `--all` | include terminal-status items |
| `--sort <key>` | ascending; `--reverse` flips. Direction is never folded into a key |
| `--limit <n>` | truncates, and the count line says so: `showing 20 of 143` |
| `--json` | on every read command; carries `total` and `shown` when truncated |

**`--direct` counts containment as a reader sees it, not as the disk does.** `list project acme
--direct` shows that project's objectives — one container down — because §16.2 makes containers
transparent and a flag that returned nothing here would be measuring a structure the output never
shows. The rule is: apply transparency, then take immediate children.

Sort keys: `name`, `created`, `attention`, `locator` everywhere; `due`, `priority`, `status` where the
field exists; `progress` and `pace` on key-results.

**A sort key a kind does not have is an error only when *no* matched kind has it.** `list` is cross-kind
by default, so a hard error would make `progress` and `pace` almost unusable — `list --sort progress`
spans projects and key-results, and only one of those has it. So:

- no matched kind has the field → error, naming the key and the kinds it applies to;
- some do → results group by kind, each group sorts by the key independently, and a group whose kind
  lacks the field falls back to `locator` order and sorts after the groups that have it.

This is the same precedent that makes an undefined `pace` print `—` and sort last (§4.2) rather than
failing the command: *field absent on this kind* is treated exactly like *field present with no value*.
*(Settled for v2's separate `search` verb in ADR 0002, and inherited here because v4's `list` absorbed
it.)*

**`--tags` is a boolean expression.** Word operators, precedence `not` → `and` → `or`, and **no
parentheses**, because every expression has a disjunctive normal form: `a and (b or c)` is written
`a and b or a and c`. A bare comma list is the short form and means `or`.

```bash
para list --tags rust
para list --tags rust,reference                        # or
para list --tags "kafka and not deprecated"
para list --tags "rust and reference or kafka and not deprecated"
```

**`--match` is the one read path that must open journals**, because note bodies live only there. Every
other filter answers from `state.toml` and the path. That makes an unscoped `para list --match foo` the
most expensive read in the surface — it reads every journal file in the tree, and `--limit` does not
help, since truncation happens after matching. It is **not restricted**: a filter that refuses to run
without an address would be a guess about tree size, and the cost is proportional to what you asked
for. `--match` under a scope narrows the read to that subtree, which is the answer when it hurts.

Operators are words, not `&`/`|`/`!`, because those are shell metacharacters and forgetting to quote
them does not fail — it backgrounds the command and returns a confident wrong answer. With keywords
the same mistake becomes a stray positional argument, which every command taking `--tags` rejects
loudly. `and`, `or`, and `not` are therefore reserved and cannot be tags. A tag is `[a-z0-9-]`, the
same shape as a locator segment. *(v1 §4.4, kept whole through two rewrites.)*

If filters keep accreting, the direction this grows is one `--where` over all fields with the same
vocabulary — not more operators on more flags.

---

## 18. Writing

### 18.1 `add`

```bash
para add project acme-migration --name "Acme migration" --description "Rebuild the consumer."
```

Creates the directory, its `.para/{state.toml, config.toml, logs/}`, its `README.md` (generated
frontmatter, stub body) and `ACTIVITY.md`, **and its child container** — `objectives/` for a project,
`key-results/` for an objective, each with the same shape.

**Containers are created eagerly, not on first child.** A project always has an `objectives/`, empty
or not. The alternative saves four files per childless project and costs a predictable shape: an agent
or a human looking for objectives would have to know that absence means "none" rather than
"elsewhere", and `add` would have to decide whether to create a container as a side effect. Uniform
shape is the reason the tree looks like this at all (§0).

- `add` on an existing address is an error, not an edit. One verb, one behaviour.
- A missing parent is an error, not an implicit creation. You create a project before its objectives.
- Reserved words (§1.4) and ids colliding with a live sibling are refused by name.
- The new entity's journal starts **empty**: `created` is a field, not an event (§3.1), so
  `attention` falls back to it. The **parent** logs `child added` (§3.3).
- No `--body`, no editor. The README body is a stub you edit yourself; a skill's `SKILL.md` body is
  the same (§13.1).

### 18.2 `set` and `unset`

```bash
para set project acme-migration --status blocked --note "waiting on the ingest team"
para set skill signups-report --scope project,area.growth
para unset project acme-migration due priority
```

Any number of fields at once, one event per field changed, one write-through pass at the end.
`--note` is **required** when setting `status` to `blocked` — a blocker with no recorded reason is
worthless in six months. *(v1's one exception, kept twice.)*

A skill has no status and no enabled flag (§1.7). `unset skill x scope` widens it back to the whole
tree; `remove skill x` is how a skill stops applying, and it takes its derived rule with it (§5.3).

### 18.3 `move`

```bash
para move area health.training fitness.training
```

One `rename(2)` where possible, plus the projection work that a hand-`mv` cannot do (§1.5): README
frontmatter, both parents' journals and `ACTIVITY.md`, and **every `scope` entry naming the moved
address or anything beneath it** (§5.4).

- **Same-kind only, and the grammar makes it structural rather than a runtime check.** `move` speaks
  its noun once for both ends (§13, R14), so there is no longer a spelling for "move this project into
  `areas/`" to even parse as — the old `move projects.acme areas.acme` refusal has no CLI form left to
  trigger it. What survives is which *fields* are legal staying fixed across a move, since the kind
  cannot change.
- **Refuses to cross the archive boundary** in either direction — that is `archive`/`unarchive`; a
  mismatched `--archived` (§1.6) is the refusal that reaches this now.
- Logs `child moved` at the old parent and the new one, and a `change` on the entity itself with
  `field = "locator"`. That last one is the single place a derived value enters a journal, and it
  earns the exception: a journal is a record of what happened, not a copy of current state, and the
  entity's own history is the one place a move must remain visible after the fact. Like every
  `change`, it does not move the clock (§3.6).
- **A move does not follow the entity's archived shadow**, and that is a limit rather than an
  oversight. Archive `resource a.b`, then rename `resource a` to `resource c`, and
  `archive/resources/a/b` still records an ancestry no live entity has. Rewriting it would mean a
  move reaching into `archive/` — the boundary the bullet above refuses to cross in either
  direction — so para leaves it alone and refuses the later `unarchive`, naming the ancestor it
  cannot reinstate. The repair is to recreate that ancestor under its old id, `unarchive` into it,
  `move` the reinstated entity where it belongs, and remove the placeholder — every step a verb
  already has, and no step reaching into `archive/` to rewrite an ancestry there. A refusal a
  sequence of otherwise legal commands can reach is worth writing down; a silent rewrite across the
  boundary would be worse.

### 18.4 `remove`

```bash
para remove project acme-migration --keep-files
```

Confirms interactively, naming the blast radius; `--force` skips; `--dry-run` rehearses.

`--keep-files` leaves your content and deletes para's footprint throughout the subtree: every
`.para/`, every `ACTIVITY.md`, every `MEASUREMENTS.csv`, and **the frontmatter block from every
`README.md`, leaving the body**. That last clause is v4's cost for generating into files you also
write in — v2's `--keep-files` was a single `rm -rf .para/`, and it no longer can be.

The parent logs `child removed`.

### 18.5 `archive` and `unarchive`

```bash
para archive project acme-migration          # → archive.project.acme-migration
para unarchive area health.training
```

Exactly the semantics of §1.6, and the verbs exist to make them unmistakable:

- `archive` takes the **whole subtree** in one move, creating stubs for any live ancestors that stay
  behind.
- `unarchive` **cascades upward**: it reinstates every archived ancestor the thing needs, adopts any
  ancestor that is already live, and leaves a stub behind wherever an archived sibling stays put.
  Each reinstated ancestor is an entity that moved, so each gets its own `field = "locator"` change
  and each container whose child set changed logs at both ends — which falls out of §3.3 rather than
  needing a rule of its own.
- An id colliding with a live sibling on unarchive is refused, naming the sibling — for the entity
  named, not for a reinstated ancestor (§1.6).
- Both rewrite `scope` entries, both log `child` events at the old and new parents plus a
  `field = "locator"` change on the entity, and both take `--dry-run`.
- Neither touches a single field. Status says how something ended; the archive says where it lives
  (§1.7).

### 18.6 `note` and `measure`

```bash
para note area health.training "swapped the tempo block for intervals"
para note area me.daily-entries "retired the old beads IDs from the ported entries" --no-attention
para measure acme.q1-growth.signups 880/11000 --at 2026-01-03
```

The two verbs that move the clock (§3.6), which is why they are verbs of their own rather than
`set` on a field. `measure`'s value grammar follows the key-result's `type` and a mismatch is an
error naming both. A duplicate `--at` on the same key-result is refused (§15.1).

`note` alone takes `--no-attention`: it records the note exactly as normal — same journal, same
`ACTIVITY.md` line — but the event carries a flag `attention` (§3.6) skips. It exists for a note that is
true about the entity without being the recurring activity a `stale-after` threshold on it is watching
for, so that recording it honestly cannot buy false silence from the check. `measure` has no equivalent
flag: every verb that touches truth other than `note` and `measure` writes a `change` or `child` event,
neither of which ever moved the clock, so `--no-attention` is `note`'s alone to need.

Correcting a past event is appending a corrected one, or editing that one line of the journal by hand
and running `rebuild`. There are no log-entry verbs, no entry ids to pass, and no re-timing flag.

### 18.7 `suppress` and `unsuppress`

```bash
para suppress project acme-migration --until 2027-03-01 --note "paused, resumes with Q1 relaunch"
para unsuppress project acme-migration --note "relaunch moved up"
```

Added after v4 was otherwise settled; full rationale in §28. Mechanically:

- **`--note` required on both, `--until` required on `suppress`.** Same rule as `blocked` (§18.2):
  a suppression with no recorded reason is worthless in six months.
- **`--until` takes `due`'s precision** (§15.1) — a bare year or year-month is a legal answer.
- One journal event kind, `suppress`. `unsuppress` appends the same kind with `until` empty; state
  folds to the newest event, so there is nothing to delete and no second kind to define.
- **Neither moves `attention`.** A suppression is a fact about whether a signal fires, not an
  attending event — folding the two together would reopen the hole `change` events were excluded
  from `attention` to close (§3.6).
- **No `--dry-run`.** Like `note`, `set`, and `measure`, this writes inside one entity's `.para/` and
  needs no rehearsal (§19).
- Legal on every addressable kind (§1.4), ungated by §15's field matrix — the same as `note`.

---

## 19. Safety

- **`--dry-run` on `move`, `remove`, `archive`, `unarchive`, and `rebuild`** — the operations that
  touch more than one entity's worth of bytes — and nowhere else. A field change writes inside one
  `.para/` and needs no rehearsal.
- **`remove` confirms interactively**, naming what will be deleted; `--force` skips it. It is the only
  interactive prompt in the surface.
- **`--note` required for `blocked`** (§18.2), **and for `suppress`/`unsuppress`, which also requires
  `--until`** (§18.7, §28).
- **Hand-editing truth is legal.** `state.toml` and the journals are yours to edit; they are truth, not
  projections. What you get is stale projections, which `doctor` reports as `stale-projection` and
  `rebuild` repairs. Nothing is corrupted and nothing is lost — which is the property that makes the
  whole write-through design safe to live in. As of §28, `state.toml`'s `attention` and `[suppression]`
  fields are the one part of it that is computed rather than typed: hand-editing them is still not an
  error, but it is pointless in the same way hand-editing `ACTIVITY.md` is — the next mutation or
  `rebuild` overwrites it, and `doctor` reports the mismatch as `stale-projection` in the meantime.
- **Hand-editing a projection is not an error either**, just pointless: the next mutation or `rebuild`
  overwrites it. `doctor` tells you before that happens.

---

## 20. `review`

```
para review [<noun> [<chain>]] [--stale | --blocked | --overdue | --behind | --skills | --suppressed]
```

| Group | Fires when |
| --- | --- |
| `--stale` | no `note` or `measurement` within `stale-after` (§3.6), **and no unexpired suppression** (§28) |
| `--blocked` | `status` is `blocked`. **No timer** — blocked is always listed, suppression or not |
| `--overdue` | open and past `due`, suppression or not |
| `--behind` | a key-result whose pace is below `at-risk-pace` |
| `--skills` | a skill untouched for longer than its `review.cadence`, **and no unexpired suppression** (§28) |
| `--suppressed` | an unexpired suppression, sorted by `until`, soonest first (§28) |

Grouped by reason, ordered within a group by distance past the threshold — **`--suppressed` is the one
exception, ordered by distance *to* `until` instead, soonest first, because "what's coming off the
shelf" is the question that group exists to answer** (§28). Takes `--limit`, not `--sort` — the
ordering is the point. Terminal items and archived things are excluded unless `--all`.

**Every row names what set `attention`** — `note` or `measurement`, and a note's own text truncated to a
recognisable length — so a reader can tell a threshold's clock apart from the elapsed count without
opening `ACTIVITY.md`. `list` carries the same information as a bare kind marker in its own column,
terser to keep one row per line; `show`'s `attention` line carries the same source `review` does, in
full next to its own `days ago`.

**What each group can contain**, since not everything with a journal is reviewable:

- `--stale`, `--blocked`, `--overdue`, `--behind` cover **entities only**.
- **Skills are reached by `--skills` and nothing else.** A skill has an `attention` (§3.6) but no
  `status` and no `due`, so it can only ever be stale — and its threshold is `review.cadence`, not
  `stale-after`. Putting it in `--stale` would mean one group reading two different knobs.
- **`--suppressed` covers anything `suppress` can reach**, entities and skills alike (§28) — it is not
  restricted to what `--stale` covers, because a suppression is legal on anything with an `attention`,
  which is every kind except a container.
- **Containers never appear in any group.** They carry no status, no due date, and an `attention` that
  is always `created` (§3.6), so a suppression on one would have nothing to suppress. A row you can
  neither act on nor set is the same noise §16.2 keeps out of `list`.

**Always exits 0.** Having work is not a failure, and a command that fails whenever you have work is a
command you stop running. `doctor` is where the gate belongs.

`--skills` is new, and it exists because §7's config table has a `review.cadence` key: a knob nothing
reads is a knob that will be wrong, so either the key goes or something surfaces it. A skill nobody
has touched in a year is either load-bearing and worth re-reading, or dead and worth deleting.

---

## 21. `rebuild` and `doctor`

### 21.1 `rebuild`

```
para rebuild [<noun> [<chain>]] [--dry-run]
```

Regenerates every projection under the address — the whole tree by default — from `state.toml`'s input
fields, `tree.toml`, and the journals. Idempotent, and it never reads a projection to produce one
(§2.4). As of §28, that includes rewriting `state.toml`'s own `attention` and `[suppression]` fields
from the same derivation — the one part of `rebuild`'s output that lands back in a truth file rather
than a projection. `--dry-run` lists what would change without writing.

`ACTIVITY.md` is re-derived **in full**, from every rotated journal file, which is the one thing
write-through never does (§3.5).

### 21.2 `doctor`

```
para doctor [<noun> [<chain>]]
```

Read-only, no `--fix`, walks every directory including inside content, and reports the findings in
§10. It does **not** follow para-owned symlinks — see §6.1 for why that would otherwise manufacture a
phantom entity. `--json` for the same findings as data.

- **Exit `0`** clean, **`1`** on any error, **`2`** when only advisories are present. CI gates on `1`
  and ignores `2`; an agent learns from the code alone whether judgement is required.
- No `--fix`, deliberately, even though `stale-projection` has exactly one remedy. `rebuild` is that
  remedy and it is one word. A `--fix` that repaired one finding class and not the others would teach
  people that `doctor` cleans up after itself, which for `orphan`, `misplaced`, `invalid`, and
  `scope-unresolved` it cannot.

---

## 22. `config`

```bash
para config set project.stale-after 30              # in the nearest config.toml — see below
para config set --at project.acme project.stale-after 30
para config unset --at project.acme project.stale-after
para config list [--prefix emit]
para config show project.stale-after objective.acme.q1-growth
```

- `set`/`unset` write the **root's** `config.toml` unless `--at <noun>.<chain>` names a level to write
  at, taking the dotted stored form (§1.4) since a flag value is one token. Root is the default
  because that is where a knob usually belongs, and `--at` is how §7's chain gets built deliberately
  rather than by accident.
- **`config show <key> [<noun>.<chain>]` prints the resolved value and the chain that produced it**,
  marking the level that won:

```
$ para config show project.stale-after project.acme-migration
30 days

  project.acme-migration       —
→ project                      30 days
  <root>                       14 days
```

§7 says the chain must be printable or chained resolution is not defensible. This is that command, and
it is the reason `show` also names where a resolved threshold came from (§16.1).

---

## 23. Output and exit codes

- **`--json` on every read command**: `show`, `list`, `log`, `activity`, `review`, `doctor`,
  `config list`, `config show`. When `--limit` truncates, the JSON carries `total` and `shown`.
- **`path` prints one bare line**, no decoration, already shaped for `$(…)`.
- **Mutations print what they wrote**, one line per file, because write-through touches four or five
  files and a user who cannot see that will not believe it:

```
$ para measure acme.q1-growth.signups 880/11000
measured signups = 880/11000   progress 0.24   at-risk

wrote  projects/…/key-results/signups/.para/logs/20260101T161502Z.jsonl
       projects/…/key-results/signups/.para/state.toml
       projects/…/key-results/signups/README.md
       projects/…/key-results/signups/ACTIVITY.md
       projects/…/key-results/signups/MEASUREMENTS.csv
```

- **Exit codes**: `0` success, `1` error, `2` advisory-only (`doctor` alone). `review` always exits
  `0`. A no-op `set` exits `0` and says `no change`.

---

## 24. Scale, across three drafts

| | v1 | v2 | v4 |
| --- | --- | --- | --- |
| command shapes | 72 | 22 | 25 |
| sub-nouns | `log`, `config`, 6 nouns | `skill`, `config` | `config` |
| locator forms | 2 | 1 | 1 |
| addressable kinds | 6 | 5 | 8 (skills, links and containers included) |
| threshold knobs | 5 | 2 | 2 |
| per-entity threshold *fields* | 5 | 2 | 0 — the config chain absorbed them (§15) |
| stored clocks | 3 | 0 | 0 |
| generated artifact kinds | `AGENTS.md` + provider dirs + skill copies | `AGENTS.md` | 11 (§2.2) — 9 files + the Claude and Cursor skill mirrors |
| repair commands | `rebuild` | none | `rebuild` |
| files per entity | `entity.md` + N logs | `entity.md` + `log.jsonl` | 2 truth + 2–3 generated + N logs |
| doctor findings | 10 | 7 | 12 |
| authored things in the skills mechanism | skill + rule + wiring | skill | skill |
| interactive prompts | several | `remove` | `remove` |
| editor invocations | `skill add` | `skill add` | 0 |

The honest reading: v4 costs **files** and buys **legibility**. It spends almost nothing on command
surface — twenty-five shapes against v2's twenty-two, with one fewer sub-noun and three more
addressable kinds — and nothing on knobs. The three added shapes are `archive`/`unarchive`, which
§1.6 requires once archival moves bytes, and `migrate`, which §30 requires once `schema` is a
contract a binary can refuse. Every increase in the table is a file count, which was the trade §0 signed for.

---

## 25. Decision log — command surface

- **Verb, noun, and a short id-chain — not verb-first with locators.** Reverses this document's own
  earlier decision (§12); the locator still carries the kind for the machine, but the noun was never
  for the machine, and partitioning the help, the flags, and the completions for a reader is worth a
  nineteen-word reserved list and a lookahead rule in `list`.
- **Noun plus short chain, not noun plus whole locator.** `para add key-result acme.q1-growth.signups`
  rather than spelling the container segments back in — they are recoverable from the noun and the
  chain's own arity, so spelling them again would be a second copy of the kind — the thing principle 1
  forbids.
- **Output shortens too.** The alternative — type short, print long — keeps one direction of §14's
  paste-what-you-read property and breaks the other, which is worse than changing both ends.
- **Archive is a flag, not a noun prefix.** `archived-project` would read as though archival were a
  kind, which §1.6 already denies.
- **A bare noun is the bucket.** This collapses "filter by kind" and "the bucket" into one idea, and
  gives `doctor`, `rebuild`, and `path` a bucket spelling for free.
- **`measure` keeps no noun, and `move` speaks its noun once.** Both irregularities are bought by a
  constraint the model already enforces — only key-results are measured, `move` is same-kind only —
  rather than by convenience.
- **Stubs lose addressability**, because a stub has no kind and the grammar's first token is a kind.
- **Skills use the universal verbs** via `skill.<id>`; the `skill` sub-noun is deleted (§13.1).
- **Five new verbs, each demanded by a first-half decision**: `archive`, `unarchive` (§1.6),
  `rebuild` (§2.4), `activity` (§3.2), `migrate` (§30.5).
- **`emit` is deleted by write-through**, not renamed.
- **`para rules` folded into `show`**, which names both the skill and the scope entry that reached it.
- **`move` is same-kind only and refuses the archive boundary.** Two spellings for one operation would
  violate principle 5.
- **`move`/`archive`/`unarchive` log `field = "locator"` on the entity itself** — the single place a
  derived value enters a journal, justified because a journal records events rather than state, and
  the entity's own history is where a move must stay visible.
- **Containers are created eagerly** by `add`, and are **transparent to `list`** — traversed, never
  listed, addressable by `show`.
- **`description` is a field; the README body is yours.** Replaces v2's `summary`.
- **`stale-after` and `at-risk-pace` are no longer fields at all** — §7's config chain absorbed them.
- **No `--body`, no `$EDITOR`, ever.** Bodies are files.
- **`scope` replaces wholly on `set`**; no `--scope-add`/`--scope-remove`. `unset … scope` widens to
  the whole tree.
- **No `enabled` flag on skills** — `remove` is the only off switch (§1.7).
- **`.` resolves to the entity containing `$PWD`.**
- **`log` and `ACTIVITY.md` both read newest-first**, so the two never disagree about direction.
- **A sort key absent on a kind errors only when *no* matched kind has it**; otherwise per-kind groups
  sort independently and a group lacking the field falls back to `locator` order, last. Extends the
  undefined-`pace` precedent; inherited from ADR 0002 (§17).
- **`--direct` applies container transparency first**, then takes immediate children — it measures
  containment as the output shows it, not as the disk stores it (§17).
- **`--match` is the only read that opens journals**, and it is deliberately unrestricted; scoping it
  with a locator is the answer when the cost bites (§17).
- **`review` covers entities only**, plus skills via `--skills` alone. Containers never appear in any
  group (§20).
- **Measurement uniqueness is compared at the exact instant**, not the day (§3.1).
- **`review` gains `--skills`**, because `review.cadence` needs a reader.
- **`doctor` has no `--fix`**, and `rebuild` is not a `doctor` flag.
- **Mutations print every file they wrote**, because write-through is invisible otherwise.
- **`config set` defaults to the root**, with `--at <noun>.<chain>` to build the chain deliberately.
- **Hand-editing truth is supported and hand-editing projections is harmless** — the property that
  makes write-through safe to live in (§19).

---

## 26. Worked examples

These double as the acceptance suite, the way v2 §15 did.

### `para init`

```
$ para init brain
created  brain/.para/{tree.toml, config.toml, logs/}
         brain/{README.md, AGENTS.md, ACTIVITY.md}
         brain/.gitattributes
         brain/projects/{README.md, AGENTS.md, ACTIVITY.md, .para/}
         brain/areas/{…}  brain/resources/{…}
         brain/archive/{…}  brain/archive/{projects,areas,resources}/{…}
         brain/.agents/{rules/, skills/}

no CLAUDE.md, no .claude/ — enable with `para config set emit.claude true`
```

The root journal records `init` and a `child` event per bucket (§8.1).

### `para add`

```bash
# a project, and its objectives/ container, in one operation
para add project acme-migration --name "Acme migration" \
  --description "Rebuild the consumer so it stops falling over under replay load."

# an objective — the parent must already exist; key-results/ comes with it
para add objective acme-migration.q1-growth \
  --name "Grow signups" --description "Move the top of the funnel."

# a key-result — type is required and permanent
para add key-result acme-migration.q1-growth.signups \
  --name "Weekly signups" --type ratio --start 480/9000 --target 2000/12000 --due 2026-09-30

# areas and resources nest freely
para add area health --name "Health" --description "Staying in one piece."
para add area health.training --name "Training" --description "The weekly plan."

# a skill — scope is an address list, and this also writes .agents/rules/para-signups-report.md
para add skill signups-report --name "Signups report" \
  --description "when asked for the weekly signups number" \
  --scope project.acme-migration,area.growth

# a skill with no scope applies to the whole tree, and its rule says so
para add skill commit-style --name "Commit style" \
  --description "when writing a commit message"
```

Refusals, each naming the problem: `para add project acme-migration` again (exists);
`para add project acme.b` (wrong arity — a project is one segment, it cannot nest);
`para add project objectives` (reserved word);
`para add key-result x.y.z --type ratio` with no `--target` (required field).

### `para show` / `list` / `path`

```
$ para list project --tags kafka --sort attention
project     acme-migration                    in-progress   31 days ago
objective   acme-migration.q1-growth          in-progress   31 days ago
key-result  acme-migration.q1-growth.signups  at-risk       12 days ago
showing 3 of 3

$ para list --status blocked --all
$ para list project --archived
$ para path area health.training
/Users/max/brain/areas/health/training
$ para show .            # from inside areas/health/training
```

Note what `list` does not show: `objectives`, `key-results`, or any other container (§16.2).

### `para set` / `unset` / `move`

```
$ para set project acme-migration --status blocked
error: --note is required when setting status to blocked

$ para set project acme-migration --status blocked --note "waiting on the ingest team"
project.acme-migration  status in-progress → blocked

$ para set project acme-migration --status blocked --note "still waiting"
no change (status already blocked); note recorded

$ para move area health.training fitness.training
moved  area.health.training → area.fitness.training
       rewrote 1 scope entry in skill.training-plan

$ para move project acme-migration acme-renamed --archived
error: --archived means both ends are archived; project.acme-migration is live
```

Naming a second noun to change kind is not a refusal `move` can even reach any more — one noun applies
to both ends (§13, R14), so there is no spelling left for "move this project into `areas/`" to parse
as. The archive boundary above is the refusal that survives.

### `para note` / `measure` / `log` / `activity`

```
$ para measure acme-migration.q1-growth.signups 880/11000 --at 2026-01-03
measured signups = 880/11000   decimal 0.0800   progress 0.24   at-risk

$ para measure acme-migration.q1-growth.signups 0.08
error: value 0.08 is not a ratio (type ratio expects <numerator>/<denominator>)

$ para measure acme-migration.q1-growth.signups 900/11000 --at 2026-01-03
error: a measurement already exists at 2026-01-03T08:00:00Z (2026-01-03T00:00:00-08:00 local)

$ para log project acme-migration --kind change --limit 3
$ para activity project acme-migration --recursive --since 2026-01-01
2026-01-05  objective.acme-migration.q1-growth            added objective q1-growth
2026-01-03  key-result.acme-migration.q1-growth.signups   measured 880/11000 (8.0%) — 24% of target
2026-01-01  project.acme-migration                        created
```

### `para archive` / `unarchive`

```
$ para archive area health.training
archived  area.health.training → archive.area.health.training
          created stub archive/areas/health/ (parent area.health is live)

$ para unarchive area health
error: nothing to unarchive — archive/areas/health/ is a stub, not an entity

$ para archive area health
archived  area.health → archive.area.health   (3 descendants moved with it)
          stub archive/areas/health/ became the entity

$ para unarchive area health.training
unarchived  archive.area.health.training → area.health.training
            reinstated area.health
            archive/areas/health/ became a stub

$ para unarchive project old-migration
error: project.old-migration exists; rename it or leave this archived
```

`archive`/`unarchive` take no `--archived`: the source's side is implied by which verb you ran
(§1.6), so the address on the command line is the plain noun and chain either way — never
`archive.<noun>.<chain>`, which is a stored form, not something you type. The stub in the second
command has no address to type in the first place (§1.6, §14); it is named by its on-disk path because
that is the only thing it has.

### `para review` / `rebuild` / `doctor`

```
$ para review --stale --behind
stale (3)
  area.fitness.training                          61 days   stale-after 30 days
  …
behind (1)
  key-result.acme-migration.q1-growth.signups    pace 0.70   at-risk-pace 0.80

$ echo "hand-edited" >> projects/acme-migration/ACTIVITY.md
$ para doctor
error  stale-projection  projects/acme-migration/ACTIVITY.md differs from journal (from 2026-01-05)
exit 1

$ para rebuild project acme-migration --dry-run
would rewrite  projects/acme-migration/ACTIVITY.md

$ para rebuild project acme-migration
rewrote  projects/acme-migration/ACTIVITY.md

$ para doctor
clean
exit 0
```

### `para suppress` / `unsuppress` (§28)

```
$ para suppress project acme-migration --until 2027-03-01 --note "paused, resumes with Q1 relaunch"
suppressed  project.acme-migration  until 2027-03-01

$ para review --stale
stale (2)
  area.fitness.training                          61 days   stale-after 30 days
  …
  (project.acme-migration excluded — suppressed until 2027-03-01)

$ para review --suppressed
suppressed (1)
  project.acme-migration    until 2027-03-01   paused, resumes with Q1 relaunch

$ para review --overdue
overdue (1)
  project.acme-migration    14 days past due   (suppression does not apply here)

$ cat projects/acme-migration/.para/state.toml
kind       = "project"
name       = "Acme migration"
status     = "in-progress"
priority   = "high"
due        = "2026-09-30"
tags       = ["kafka", "consumer"]
created    = "2026-01-01T16:15:00Z"
attention  = "2026-08-20T10:00:00Z"

[suppression]
until = "2027-03-01"
note  = "paused, resumes with Q1 relaunch"

$ para unsuppress project acme-migration --note "relaunch moved up"
unsuppressed  project.acme-migration
```

### `para config`

```
$ para config set --at project project.stale-after 30
$ para config show project.stale-after project.acme-migration
30 days

  project.acme-migration      —
→ project                     30 days
  <root>                      14 days

$ para config set emit.claude true
wrote  .para/config.toml
       CLAUDE.md, projects/CLAUDE.md, areas/CLAUDE.md, resources/CLAUDE.md,
       archive/CLAUDE.md, archive/{projects,areas,resources}/CLAUDE.md   (8 files)
linked .claude/skills/para-signups-report → ../../.agents/skills/para-signups-report
       .claude/skills/para-commit-style   → ../../.agents/skills/para-commit-style
       (emit.claude-skills = symlink; set "copy" where links do not survive checkout)
```

---

## 27. Still open

Nothing structural. What remains is implementation-shaped:

- The exact wording of each `AGENTS.md` generated block, which is prose and wants drafting against a
  real tree rather than in a spec.
- The rendering templates for `ACTIVITY.md` lines per event kind, and for a derived rule file.
- Whether `--where` eventually replaces the filter flags (§17), which should wait until the flags
  actually hurt.
- `docs/implementation-plan.md` is written against this document. The six spec gaps it opened are now
  closed in §3.1, §3.6, §17, and §20, and recorded in §25.
- ADR 0001 is superseded by ADR 0003 (§9's no-git-invocation posture keeps its conclusion — no merge
  driver, resolve truth then `rebuild` — and discards its premise). ADR 0002 carries an amendment noting
  it now governs `list`.

---

## 28. Suppression

Added after v4 was declared structurally settled (§11, §27). Recorded as its own section, touching
§2.5, §3.6, §8.3, §10, §13, §18, §19, §20, and §21.1 by reference rather than folded into them
silently, because it reverses one of them and that is worth being able to find later.

### 28.1 The problem

`stale-after` (§3.6) has no way to tell "neglected" apart from "deliberately dormant." A project whose
relaunch was pushed a quarter out is neither, but `review --stale` cannot see the difference — the
only levers that move `attention` are `note` and `measure`, and both mean something true happened. A
note whose only purpose is to move the clock is dishonest, and writing one every week is exactly the
toil `review` exists to eliminate, not to create.

`due` is deliberately excluded from `attention` (§3.6) for the opposite reason: letting it count would
let any `set --due` buy silence from every check, on every entity, forever. Suppression has to solve
the first problem without reopening the second.

### 28.2 `suppress` and `unsuppress`

Full command shape in §13 and §18.7. In outline: two verbs shaped like `note`/`measure` rather than
`set`, because a suppression is an event, not a field. `--note` is required on both, the same rule
§18.2 already applies to `blocked`; `--until` is required on `suppress` and takes `due`'s precision
(§15.1). One journal event kind, `suppress`; `unsuppress` is the same kind with `until` empty, since
state folds to the newest event exactly as `attention` already does (§3.6) — there is no second kind
and no delete-style operation to define. Neither verb moves `attention`: suppression and attention are
independent facts, and letting one imply the other would reopen the hole `change` events were excluded
from `attention` to close.

### 28.3 What it changes in `review`

Full table in §20. In outline: `--stale` and `--skills` — the two groups whose entire premise is
"nothing happened recently" — exclude an entity with an unexpired suppression, because that premise is
the one thing a suppression honestly asserts is false. `--blocked`, `--overdue`, and `--behind` are
untouched: a suppressed project that is also overdue still appears under `--overdue`, timer-free and
unhidden, exactly as §1.7 already promised a blown deadline would be. Suppression is a statement about
activity, not about whether something needs attention for some other reason, and folding it into every
group would let it hide the one thing this design has never let anything hide.

No return event: when `until` passes, the entity re-enters `--stale`/`--skills` through the ordinary
threshold check, the same one it would have hit regardless — not a one-time "suppression lapsed" row.
Simpler, and it treats suppression as a fact about the past rather than a standing arrangement that
needs its own closing event.

### 28.4 `state.toml` gains a materialized field and a materialized table

```toml
# projects/acme-migration/.para/state.toml
kind       = "project"
name       = "Acme migration"
status     = "in-progress"
priority   = "high"
due        = "2026-09-30"
tags       = ["kafka", "consumer"]
created    = "2026-01-01T16:15:00Z"
attention  = "2026-08-20T10:00:00Z"

[suppression]
until = "2027-03-01"
note  = "paused, resumes with Q1 relaunch"
```

This reverses §2.5 and §8.3, which name `attention` explicitly as never stored, and it is worth being
exact about what changes and what does not.

**What does not change**: `attention`'s definition (§3.6) is untouched — still the newest `note` or
`measurement` that counts, else `created`. There is still no `para set … --attention`; `state.toml` is
still not where you *set* it. What changes is where the answer to "what is it right now" lives once
computed, not how it is computed.

**Why now, having held this line through v1 and v2's stored-clock failures** (§3.6's "gone and staying
gone" list — `blocked-after`, `at-risk-after`, `sla-reset-at`, `status-changed-at`): those clocks were
stored *instead of* being derived from the journal, maintained by write-time logic separate from
whatever derived the live value, with nothing to catch the two drifting apart. That is the failure this
must not repeat, and it is avoided by a narrower rule than "cache what is convenient":

> **One function computes a derived value. It is called to write `state.toml`, and it is the only
> function `doctor` calls to check `state.toml`.** There is no second implementation of "what is
> `attention` right now" anywhere, so there is nothing for the write path and the check path to
> disagree about.

That rule is not new — it is §2.3's write-through invariant and §2.4's `rebuild` guarantee, applied to
a truth file instead of only to projections, for the first time. `state.toml` was pure input because
everything in it was something you typed; `attention` and `[suppression]` are the first fields in it
that para writes to itself, on every mutation, from the same derivation `review` and `show` already
trust — and the reason it is worth doing at all is that `review` walking every entity in the tree was
paying a full journal scan per entity, per run, to answer a question whose answer had not changed since
the last time it was asked.

The precedent already exists one layer up: `README.md`'s frontmatter is generated and doctor-checked
while its body is yours (§2.1) — one file, two regimes, decided by who writes which part rather than by
the file's name. `state.toml` now works the same way: the fields you set are truth, unconditionally;
`attention` and `[suppression]` are generated, in the same sense frontmatter is, just generated into the
truth file instead of into a projection.

**A `state.toml` written before §28 has neither key at all**, and that has to be a fact the reader
handles, not an upgrade step every existing tree is required to run first. **Absence means
"not yet cached," never "false" or "zero."** Every read path falls back exactly as every read path
always has — compute `attention` (and any suppression) live from the journal — so a pre-§28 tree
answers every question correctly from the moment the binary is upgraded, before anything has written
a byte. Two things then backfill the cache, neither of them required before reads are trusted:
**opportunistically**, the moment an entity is next mutated, write-through computes and writes the
field per the rule above, same as any other entity from then on; **in bulk**, `para rebuild` computes
and writes it for everything at once, which is not a new job for `rebuild` — "a new para version
renders a template differently" (§2.4) already names exactly this shape of problem. `doctor`'s
`stale-projection` finding (§10) reports a missing field the same way it reports a wrong one: "differs
from what would be written now" already covers "isn't there."

`§19` ("hand-editing truth is legal") gets the same caveat frontmatter already has: hand-editing
`name`, `status`, `due`, and every other input field is still unconditionally legal, because it is
still truth. Hand-editing `attention` or `[suppression]` is not an error either — nothing in
`state.toml` ever is — but it is pointless in exactly the way hand-editing `ACTIVITY.md` is: the next
mutation, or `rebuild`, overwrites it with what the journal actually says, and `doctor`'s
`stale-projection` finding (§10) reports the mismatch in the meantime, extended to cover this one part
of a truth file rather than replaced by a new finding class.

### 28.5 Naming

Considered and set aside: `shelve` — evocative, but implies the whole entity is set aside, when
`--blocked`/`--overdue` deliberately keep firing underneath it. `mute` / `snooze` — the right shape,
but this tool's existing vocabulary (`stale-after`, `attention`, `terminal`) is precise rather than
casual, and `snooze` specifically undersells a suppression that can legitimately run a quarter long.
`defer` — collides with an unrelated existing meaning ("defer this work, push the date") in the same
toolchain this project sits alongside; reusing it here would suggest it also moves `due`, which it does
not and must not (§28.1). `suppress` names the mechanism rather than implying a change to the entity
itself, which matches what actually happens: nothing about the entity changes, only whether one
specific class of signal fires.

The record it produces is `suppression`, a noun, not `suppressed`, an adjective: `[suppression]` holds
two attributes (`until`, `note`), which reads as a small record rather than a flag — the same
relationship `note`-the-verb has to `note`-the-thing it produces.

## 29. Links

Added after v4 was declared structurally settled (§11, §27, §28), the same way suppression was:
recorded as its own section, touching §1.2, §1.3, §1.4, and §15 by reference rather than folded into
them silently, because it adds an eighth noun and that is worth being able to find later.

### 29.1 The problem

A project, area, or resource has no first-class way to declare where it touches the outside world — a
project's Jira epic, a Slack channel it should watch or update, an area's output blog, an RSS feed
worth monitoring as input. Tooling built on top of `para` has had to invent ad hoc conventions per
case, which do not compose and are not visible via `show`. `link` closes that gap: a typed,
directional, labeled external touchpoint, stored the same way everything else in this tree is and
surfaced the same way everything else is read.

### 29.2 The `link` noun and its fields

A link is an entity (§1.2) — its own directory, `.para/state.toml`, README.md, ACTIVITY.md — attached
to a project, an area, or a resource. Fields (§15): `type` (an opaque string para never interprets —
`jira-epic`, `slack-channel`, `blog`, `rss-feed`, ...; same posture as `--tags`, fixed at creation the
way a key-result's `type` is, for a different reason — nothing measures it, but tooling keys its own
interpretation off it, and letting it change silently would be the same "the label lied" failure with
a different reader); `ref` (an opaque locator the type interprets — an issue key, a channel id, a
URL); `direction` — a real two-value enum, validated like `status` already is: `input` (monitor this
for relevant activity) or `output` (this needs to be kept updated) — a link is always one or the
other, never both (§29.6 explains why); and `description`, the same field and meaning as everywhere
else, optional. `name` is Optional and defaults
to the link's own id — the one field row where a link departs from every other kind, because a link's
slug is usually name enough and a link's own CLI shape never needs to type one.

A project, an area, or a resource may have zero, one, or many links, of the same type or mixed types —
the same cardinality relationship key-results already have to an objective.

### 29.3 Addressing: a link answers to three parents, so its chain names one

Every other noun with a fixed nesting shape needs no help disambiguating its own address: `key-result`
always nests under exactly one parent kind (`objective`), so its three-segment chain is unambiguous by
construction (§1.4). `link` cannot borrow that trick — it nests under *three* different parent kinds
with different arities (`project` takes exactly one id; `area`/`resource` take one or more, at any
depth), and a bare id-chain like `acme.jira-epic` cannot say on its own whether `acme` names a project
or an area, since nothing stops the two ids from coinciding as plain strings, and §1.4's own invariant
— "the arity and shape of the id-chain determine the path completely" — means resolution may not fall
back to checking which one actually exists on disk.

The resolution reuses a device §1.4 already has, at the other end of a chain: `container acme.objectives`
and `container acme.q1-growth.key-results` are disambiguated by spelling the reserved structural word
literally, because the chain's own shape has more than one legal reading otherwise. A link's chain
applies the same device at the front instead of the back — its first segment is always the parent's own
noun word, `project`, `area`, or `resource`, followed by that noun's own id-chain, followed by the
link's own id: `link project.acme.jira-epic`, `link area.health.training.blog`,
`link resource.papers.kafka.feed`. Reading it the other way: a link's chain is exactly a links
container's own chain (below) with its trailing `links` word replaced by the link's own id — the same
length, not one segment longer, the same relationship a key-result's chain has to its key-results
container's.

The `links` structural word joins `objectives`/`key-results` in §1.4's reserved list, and a `links`
container's own address takes the identical parent-selector prefix, since it faces the identical
disambiguation: `container project.acme.links`, `container area.health.training.links`.

Links are otherwise ordinary addressable entities: `--archived` reaches them exactly as it reaches
every other kind (§1.6) — archiving a project, area, or resource drags its `links/` subtree with it,
the same whole-subtree cascade every other archive already is — and a skill's `scope` (§5.2) may name
one directly, the same way it names any other address, since scope entries are addresses and nothing
about being a link excludes it from that list.

No disk lookup is introduced by a link's own chain resolution — chain to path stays
pure functions of the address's own text, exactly as §1.4 requires everywhere else.

### 29.4 The `links/` container is lazy, not eager

`objectives/` and `key-results/` are eager (§18.1): a project is born with an `objectives/` container,
empty or not, because an agent or a human looking for one should not have to know that absence means
"none" rather than "elsewhere." `links/` breaks that pattern deliberately. Making it eager would mean
every project, and every area and resource *at every nesting depth*, is born with an unused `links/`
folder on the (overwhelmingly common) case that it never gets one — clutter with no offsetting benefit,
since unlike `objectives/key-results` a link-capable parent's own §1.3 nesting shape does not otherwise
require a fixed child.

Instead, `links/` is created the first time a link is actually added beneath a parent, whichever parent
that is and at whatever depth — the same "para's own, made on the way past rather than required in
advance" posture `.agents/skills/` already has (§1.4), generalized from a bare directory to a real
container with its own `state.toml`. A parent that never gets a link never grows a `links/` folder; one
that does gets it exactly once, on the first add.

### 29.5 What deliberately stays out of `para`

- **Nothing that actually talks to Jira, Slack, a blog, or any other system.** That is purely a
  skill/agent-layer concern. `para`'s job stops at storing a typed, directional, labeled list of
  external touchpoints per entity, and surfacing it — `show <noun> <chain>` renders a parent's links
  grouped by direction, alongside what it already shows.
- **No status field, and no off switch beyond the same primitives every kind already has.** A link's
  own state has no lifecycle to report on the way a project's `planned`/`done` does — the same "no
  status" posture `area`/`resource`/`container` already have. Muting one is `suppress`/`unsuppress`
  (§28), unchanged and unextended by anything below.

### 29.6 Staleness: one clock, one key, because every link is single-direction

`link`'s first release shipped a `both` direction — a single link modeling a genuinely two-way channel,
watched for asks and posted to. Giving that link the staleness this section adds meant either two
independent attention clocks (§3.6) on one entity, `attention-input`/`attention-output`, plus two
config keys, `link.input-stale-after`/`link.output-stale-after`, so each stream could be judged
against its own threshold — or removing `both` outright.

**The two-clock design was reversed before it shipped**, in favor of removing `both` entirely:
`direction` is `input` or `output`, full stop. The reasoning: every other entity in this model has
exactly one attention clock and answers to exactly one `<kind>.stale-after` key, with zero
exceptions — a `both` link would have been the only entity anywhere that needed two of either.
Removing the value doesn't work around that, it removes the one shape where the problem could exist
at all, which is the more complete fix, not a lesser one. Once every link is single-direction, `link`
simply joins the `<kind>.stale-after` family (§15) like any other kind — **one key,
`link.stale-after`**, resolved by the link's own kind exactly the way
`project.stale-after`/`area.stale-after`/etc. already are. There is no per-direction key
family: the direction disambiguates which *stream* a link represents, but the threshold itself needs
naming only once, the same as everywhere else.

The accepted cost: a genuinely bidirectional external channel — one Slack channel both watched and
posted to — now costs two link entities sharing one duplicated `ref`, rather than one link carrying
both facts. Each half gets its own `type`, its own description, and its own id to disambiguate it from
its sibling, which is judged the better trade against the alternative: teaching one entity kind to
carry two attention clocks, the one thing nothing else in this model ever does. A future reader wanting
"input threshold different from output threshold" for links sharing one `links/` container already has
it, at the per-entity or per-container level, the same limit every other single-key kind already has —
not a gap this reversal opened.

**This was a breaking change to already-shipped behavior.** `direction: both` was accepted and stored
by the first release of `link`. A `state.toml` carried over from before this section is refused the
same way any other value outside a closed field's vocabulary already is (§15, §10's `invalid`
finding) — there is no migration path beyond hand-editing the value, which is the correct level of
machinery for a value that was never valid to begin with once the vocabulary changed under it.

---

## 30. Kind in `state.toml`

§1.3 used to be called *location is kind*. It is not any more. `state.toml` carries a `kind` key
(§8.3) and that key is the only place a directory's kind is written down. The path keeps deriving
**id**, **parent**, and **locator**; it stops deriving **kind**.

That sentence is the entire change, and the shape of it is deliberate. Three of the four facts v2 put
in the path stay there, because each is a fact *about the path* — a second copy of an id, a parent, or
a locator could contradict the bytes' own location, which is exactly the desync §8.4 refuses. A kind
is not a fact about the path. `areas/relationships/people` is a legal home for an area and for a
Managed Directory alike, so the position states nothing that a stored kind could contradict, and the
count §8.4 cares about stays at one.

### 30.1 Why

A directory whose purpose is to hold files needs to sit at a natural, user-chosen path. Under
location-is-kind it cannot. `areas/relationships/people` necessarily derives **area**, because
`areas/` nests with no depth limit and every segment there is an area id; `projects/acme/evidence`
derives nothing at all, because a project cannot nest. Neither answer is wrong under the old rule —
the rule simply has no room for a kind whose position implies nothing.

The alternative was a reserved container word, the device `links/` uses (§29.3):
`areas/relationships/directories/people`. It preserves every invariant and it was rejected on cost.
The word is mechanical, it lands in the middle of paths a human reads and an agent writes to, and
adopting an existing directory would mean moving every file inside it. A tool whose first principle is
that the tree is readable without it should not spend a path segment on its own bookkeeping.

Two things fell out of the change that were not the reason for it, and both are worth more than the
feature that prompted it.

**One kind mapping instead of two.** `kindmeta.KindOf` and `address.FromLocator` were two independent
traversals of the same path→kind mapping, kept apart on purpose so that nothing in the CLI path
inferred a kind from a shape. Each carried its own project / nesting / link / skill chain walker.
There is now one source and both are lookups.

**Adoption stops being impossible.** §1.5 gave up adoption — "an untracked directory is invisible, and
the way to make something an entity is to create the entity and move your files into it" — *because*
the kind came from position. There was nothing to write. With a stored kind, pointing para at a
directory that already exists is a coherent operation rather than a contradiction.

### 30.2 The containment table

§1.3's table did two jobs: it derived a kind, and it forbade illegal shapes. Only the second job
survives, and it becomes an explicit table rather than a property of a parser.

| Parent | May contain |
| --- | --- |
| **project** | `objectives/`, `links/` |
| **area** | area, `links/` |
| **resource** | resource, `links/` |
| **objective** | `key-results/` |
| **key-result** | — leaf |
| **link** | — leaf |
| **skill** | — leaf |
| **container** `objectives/` | objective |
| **container** `key-results/` | key-result |
| **container** `links/` | link |

And the rows above the entities. A bucket is a container at depth 1 (§1.2), so it stores
`kind = "container"` like any other and its *name* is what says which one:

| Parent | May contain |
| --- | --- |
| **root** | the four buckets: `projects/`, `areas/`, `resources/`, `archive/` |
| **container** `projects/` | project |
| **container** `areas/` | area |
| **container** `resources/` | resource |
| **container** `archive/` | the three mirrors: `archive/projects/`, `archive/areas/`, `archive/resources/`, and nothing else — there is no archive of an archive |
| **container** `archive/{projects,areas,resources}/` | the same kind the live bucket holds, dormant (§1.6) |
| `.agents/skills/` | skill, one level only — not a container, and `.agents/` holds no `.para/` of its own (§8.1) |

One table, three callers: `add` asks *may I create this kind here*, `move` asks *may this kind land
there*, and `doctor` asks *is what I found where it is allowed to be* (§10). Adding a kind becomes a
row rather than an edit to four functions in two packages.

**The three callers do not need equal amounts of it, and that is worth saying because it is the
answer to "why keep the bucket rows at all".** `add` and `move` are handed a noun, and §1.4's path
templates already fix the bucket from the noun alone — a typed `project acme` cannot produce anything
but `projects/acme`, so no bucket rule can be broken by a legal address. `doctor` gets no noun: it
finds a `state.toml` at whatever position a filesystem happens to hold, so it is the caller that
needs every row, all the way to the root. A table that stopped at the entity rows would answer `add`
completely and leave `doctor` unable to fault a project sitting in `resources/`.

The container rows are not decoration. *May a container named `key-results` sit under this parent* was
already a containment question, answered by a switch statement several layers away from the rules it
belonged with.

**The vocabulary.** A `kind` is one of the eight words §1.4 gives the nouns — `project`, `area`,
`resource`, `objective`, `key-result`, `link`, `skill`, `container` — and nothing else. That the noun
vocabulary and the stored-kind vocabulary are the same list is the point, not a coincidence: it is why
`add` needs no `--kind` flag (§0 principle 1) and why one parser serves both. `unknown` is not among
them; it is what an unclassified thing prints, never a value a file may hold.

**What the table covers, and what it says nothing about.** It governs where an *entity or container*
may sit. It is not a whitelist of directory entries: **content** is legal inside any entity or bucket
(§1.2), which is what `design.md` and `scans/` are, and a **stub** is legal in `archive/` and must
never be reported as malformed (§1.6). Neither has a `state.toml`, so neither has a kind for a
containment rule to be about. `doctor`'s existing advisory for a plain directory sitting directly in a
bucket (§10) stays an advisory and does not become `misplaced`; the table is asked only about things
that carry a stored kind.

**The table grants no new freedom beyond that.** For every kind §1.4 names, it reproduces §1.3. A
stored kind means no position is *inherently* illegal any more, which invites re-deriving the whole
table from scratch — projects nesting, objectives hanging off areas, resources under projects. That is
a redesign of what PARA means, not a consequence of where a kind is written, and each new combination
would need its own answers for rollup, archival, and projection.

**Which leaves the motivating case deliberately unbuilt.** A Managed Directory — the kind whose
position implies nothing, and the reason §30.1 exists — has no row above and no word in the vocabulary
above, because this section moves a kind into `state.toml` and stops there. Adding the kind is a
separate change with its own section, which owns its word, its row, and its chain shape (the same
posture the chain-ambiguity bullet below takes). Until then §30 enables such a kind without containing
one, and a `state.toml` claiming a kind this section does not list is `invalid` (§10) exactly as it
should be.

Two details this table does not settle, because they belong with the kind that needs them:

- **A kind whose chain is ambiguous.** Where a kind may nest inside itself *and* inside a parent that
  also nests, a flat id-chain no longer says where the parent ends — `area.relationships.people.ryan-king`
  splits two ways. The kind index answers it (§30.3), address→path is unaffected (§1.4), and the arity
  rule that used to make chains self-describing is the thing given up. The section introducing such a
  kind owns the statement of its own chain shape.
- **A stub still has no kind** (§1.6), on the reason §1.6 always gave: there is no `state.toml` to
  read. `KindAt` reports it the way it reports the tree root — no kind, no error. Nothing gives a stub
  a `.para/` to fix this, because a stub carrying one would satisfy the `state.toml`-presence test the
  walk uses to tell an entity from content.

### 30.3 The kind index

The walk already opens `.para/state.toml` for every entity it descends into — that presence test is
what tells it an entity from content (§8.5). It now reads the file it was already opening, and emits a
locator→kind index for the command's lifetime. The walk stays O(entities); a stat becomes a read.

Everything that used to derive a kind from a path consults the index instead. `KindAt` keeps its job
and changes its method. Address→path is untouched and stays free of the tree, because the noun is
spoken (§1.4).

The old path→kind walker is **retired, not deleted**. It is exactly the right code for reading a
`schema = 1` tree, where position *is* authoritative, and that is the one job left for it: §30.5's
migration step. Deleting it would mean writing the same traversal again a week later.

### 30.4 When the stored kind disagrees with its position

A `state.toml` saying `kind = "project"` in a directory under `areas/`. The stored kind wins, every
read treats it as a project, and `doctor` reports the position as illegal against §30.2's table.

The tempting alternative is that position wins and `rebuild` repairs the field, which is self-healing
and never leaves a tree the write path would refuse to create. It is also unimplementable. A rule that
rewrites a field disagreeing with its position cannot distinguish a mistake from a Managed Directory,
because the two are the same observation: a stored kind its position does not imply. The rule that
repairs the first destroys the second, and it cannot tell which it is looking at.

So the field is authoritative because it is the only copy, and reinterpreting the only copy is
inventing data. Repair stays manual, as everything in `doctor` does (§21), and it runs in whichever
direction is actually wrong: `mv` the directory back, or `para move` it, if the position was the
mistake — or edit `kind` in the `state.toml` if the position was deliberate and the kind is what is
stale. Hand-editing truth is legal (§19), and this is the one field for which it is the *only* way,
since no verb sets a kind: `add` fixes it at creation and `move` carries it unchanged (§18.3, whose
"the kind cannot change" is now a property of `move` being same-kind rather than a fact about
positions).

Note what this replaces rather than adds. The same hand-`mv` used to raise nothing at all: the project
became an area, silently, with its objectives and measurements hanging off a kind that cannot hold
them (§1.5). A finding where there was none is the improvement.

### 30.5 `schema = 2` and `para migrate`

`schema` (§8.1) goes to 2, and a binary reading a `schema = 1` tree refuses it and names the command
that fixes it.

`para migrate` is a schema-step verb rather than a one-off: it reads `schema`, applies each registered
step in order, and bumps the field. Step 1 → 2 derives every entity's kind from its position and
writes it down.

**That derivation cannot be wrong.** Position was authoritative in every tree that predates this
section, because no kind existed whose position implied nothing. The migration is not a guess about
old data; it is a transcription of what the old rule already said, performed once by the very walker
that used to say it.

Properties, in the order they matter:

- **No files move and no paths change.** A `schema = 1` tree's layout is already correct; only the
  state files gain a line.
- **`schema` is bumped last**, after every entity is written. A crash halfway leaves a tree that still
  says `schema = 1`, which is true — some entities carry a kind, the rest do not, and the tree is
  still owed a migration. Bumping first would leave a tree lying about itself.
- **Re-running completes it.** The derivation is idempotent and position-based, so a second run writes
  only what the first did not reach. This is the same property, and the same reason, as writing a
  cascade parent-first: ordering alone buys the safety, so no transaction is needed.
- **`--dry-run` reports the count**, and a tree already at 2 reports that it is and exits 0.
- **A `schema = 2` tree can still acquire a kindless `state.toml`**, and that is §10's `no-kind`: not
  from migration, which writes every one it walks, but from a hand-created directory, a half-applied
  merge, or a `.para/` copied from elsewhere. It is an error rather than an advisory because there is
  nothing to read the entity *as* — every other finding at least knows what it is looking at.
- **One journal event at the root**, which is where tree-level facts already go (§8.1). It is a
  `change` on the tree's own `schema`, not a new event kind: §3.1's vocabulary is closed and §10's
  `journal` finding reports an unknown `kind`, so inventing a `migrate` event would have `doctor`
  faulting the journal of the tree it had just repaired.

### 30.6 What it cost

Honest ledger, since all four of these are real.

**A pure function became tree-dependent.** Asking a bare path what it is now needs the index. Every
caller that asks already walked to find the path, so no caller acquired a dependency it lacked — but
the function's signature says so now, and that is a genuine loss of the kind of code that can be
reasoned about in isolation.

**Path → address is no longer total on its own** (§1.4). Address → path still is, which is the
direction scope entries, journal event ids, and `doctor` findings actually travel, so the property
§1.5 calls load-bearing survives. The reverse question is the one that changed.

**"Holds only files" stops being a path fact.** Under a container word, what may sit inside a
directory would be structurally checkable. It is now a claim the containment table makes and `doctor`
verifies, which is weaker: a hand-created directory in the wrong place is found on the next `doctor`
run rather than being impossible to express.

**Every tree needs one command run before the upgrade works.** This is the first release that refuses
to operate until the user acts. It is also the first one that could offer a migration at all — compare
§29.6, where a vocabulary change left no path but hand-editing, because the values were never valid.
A mechanical change to a mechanical fact earns mechanical repair.
