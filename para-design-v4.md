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

1. **Location is kind — and locator is path.** Where a directory sits says what it is, and its
   locator is its path with `/` swapped for `.`. There is no noun word, no `--kind` flag, no
   elision table, no second locator form.
2. **One source of truth per fact.** For anything with a `.para/`, that is its `.para/state.toml`.
   For history, the append-only journal. Nothing else is authoritative, ever.
3. **Generated files are the product, not a cache.** README frontmatter, `ACTIVITY.md`,
   `MEASUREMENTS.csv`, `AGENTS.md`, rule files — these exist for readers who will never run para.
   They are written through on every mutation and rebuildable from truth at any time.
4. **One home per event.** An event is logged at the entity it happened to, and nowhere else.
   Rollups are computed when asked for, never stored.
5. **One spelling per thing.** One locator form. One place scope is declared. One clock.

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
├── README.md  AGENTS.md  CLAUDE.md  ACTIVITY.md
├── projects/
│   ├── README.md  AGENTS.md  CLAUDE.md  ACTIVITY.md
│   ├── .para/{state.toml, config.toml, logs/}
│   └── acme-migration/                          projects.acme-migration
│       ├── README.md  ACTIVITY.md
│       ├── .para/{state.toml, config.toml, logs/}
│       ├── design.md                            yours. para never touches it.
│       └── objectives/                          projects.acme-migration.objectives
│           ├── README.md  ACTIVITY.md
│           ├── .para/{state.toml, config.toml, logs/}
│           └── q1-growth/                       …objectives.q1-growth
│               ├── README.md  ACTIVITY.md
│               ├── .para/{state.toml, config.toml, logs/}
│               └── key-results/                 …q1-growth.key-results
│                   ├── README.md  ACTIVITY.md
│                   ├── .para/{state.toml, config.toml, logs/}
│                   └── signups/                 …key-results.signups
│                       ├── README.md  ACTIVITY.md  MEASUREMENTS.csv
│                       └── .para/{state.toml, config.toml, logs/}
├── areas/
│   ├── README.md  AGENTS.md  CLAUDE.md  ACTIVITY.md
│   ├── .para/{state.toml, config.toml, logs/}
│   └── health/                                  areas.health
│       ├── README.md  ACTIVITY.md
│       ├── .para/{state.toml, config.toml, logs/}
│       ├── training/                            areas.health.training
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
    │   └── old-migration/                       archive.projects.old-migration
    ├── areas/
    │   ├── README.md  AGENTS.md  CLAUDE.md  ACTIVITY.md
    │   ├── .para/{state.toml, config.toml, logs/}
    │   └── health/                              ← a stub. no README, no .para/ (§1.6)
    │       └── training/                        archive.areas.health.training
    └── resources/
        └── … same shape
```

- **Every `.para/` holds the same two filenames** — `state.toml` and `config.toml`. The kind is in the
  path and appears nowhere else, so nothing can desync (§8.4).
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
| **entity** | holds `.para/state.toml` and has an id for a name | a project, area, resource, objective, key-result, skill |
| **stub** | in `archive/`, holds no `.para/` at all | `archive/areas/health/` above |
| **content** | anything else inside an entity or a bucket | `design.md`, `scans/` |

Containers and entities hold the same two filenames and are distinguished by **name**: a container's
name is always one of the reserved words (§1.4), and a reserved word can never be an id, so the test
is exact and needs no file read. The kind itself comes from the full path (§1.3), which is the only
place it is written down.

### 1.3 Location is kind

| Location | Kind | Nests? |
| --- | --- | --- |
| `projects/X` | **project** | no — depth 1 only |
| `projects/X/objectives/Y` | **objective** | no — depth 1 under the container |
| `projects/X/objectives/Y/key-results/Z` | **key-result** | no — leaf |
| `areas/X`, `areas/X/Y/…` | **area** | yes, no depth limit |
| `resources/X`, `resources/X/…` | **resource** | yes, no depth limit |
| `archive/{projects,areas,resources}/…` | the same kinds, dormant (§1.6) | as above |
| `.agents/skills/para-X` | **skill** — an entity | one level |
| `.agents/rules/para-X.md` | **derived rule** — a projection, not an entity (§5.3) | — |
| `.agents/**` without a `para-` prefix | **yours**. para does not read or write it, ever. | — |
| anything else | **content**. para does not track it, ever. | — |

No state file records its own kind, id, parent, or locator. All four come from the path. *(v2 §1.2's
load-bearing idea, kept whole — with the caveat in §1.5 about what a hand-`mv` now costs.)*

**`max-depth` remains gone**, with its two config keys, its creation-time refusal, its `--force`, and
its `doctor` finding. Depth was a proxy for a filing judgement it could not make. *(v2 §5.3.)*

### 1.4 Locator = path, always

```
projects.acme-migration
projects.acme-migration.objectives.q1-growth.key-results.signups
areas.health.training
archive.areas.health.training
skills.para-signups-report
```

Dots separate segments, hyphens separate words within a segment, charset `[a-z0-9-]`. Reserved and
unusable as an id: `projects`, `areas`, `resources`, `archive`, `objectives`, `key-results`, `skills`,
`logs`, `.para`, `.agents`.

**`skills.<id>` is the second exception to locator↔path**, mapping to `.agents/skills/para-<id>/`. It
exists because a segment cannot contain a dot, so `.agents` can never appear in a locator, and because
skills are entities with the same field vocabulary as everything else — giving them a locator means
every verb works on them and the command surface grows by nothing (§14). The `para-` prefix is part of
the directory name, not the locator: `skills.signups-report` addresses
`.agents/skills/para-signups-report/`. There is no `rules.` namespace, because rules are projections
and nothing addresses a projection (§5.3).

One form. Every command takes it, every output prints it, anything you read pastes anywhere. The
container segments are **not elided** — a key-result is six segments and that is what you type.

The alternative was eliding `objectives`/`key-results` to give `projects.acme.q1-growth.signups`.
Rejected: v2's best ergonomic win was killing the table explaining which command wanted which
locator form, and elision quietly reintroduces a translation layer that every command, every output,
every `scope` list, and every `doctor` message has to agree on. Six segments are typed once and
pasted thereafter.

A log entry's locator is its entity's locator plus the entry timestamp as a final segment:
`projects.acme-migration.20260101T081502`.

### 1.5 Entities all the way up

An entity's parent chain is entities and containers, never content. `areas/health/scans/training/`
with a `.para/` in `training/` is **illegal**, not merely discouraged: `areas.health.scans.training`
would contain a segment that is not an id, and `areas.health.training` would be a lie about where the
bytes are. `doctor` reports it as `orphan`.

The consequence is that **locators are isomorphic to disk paths**, with archive stubs the single
stated exception (§1.6). That isomorphism is what lets scope lists, `doctor` messages, log entry ids,
and README frontmatter all use one string with no resolution step.

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

### 1.6 Archive is a place

Archiving **moves bytes**. `projects/acme` becomes `archive/projects/acme`, and the locator becomes
`archive.projects.acme`.

- **Archived things are first-class.** They have locators, they are addressable, they can be shown
  and listed, and a skill's `scope` may name them.
- **Archiving drags the whole subtree.** Archiving an area takes its sub-areas and its content with
  it, in one move. There is no partial state.
- **Unarchiving cascades upward.** You cannot unarchive a child whose parent is archived; para
  refuses and names the parent. Unarchive the parent and the child comes with it.
- **Id collision on unarchive is a hard error.** If a live sibling has taken the id, para refuses and
  names it; you rename the sibling or unarchive nothing.
- **Stubs preserve ancestry.** Archive a sub-area whose parent stays live and para creates
  `archive/areas/<parent>/` as a bare directory — no `README.md`, no `.para/` — purely to record
  where the thing came from. Stubs are the one place a locator segment has no entity behind it.
  `doctor` must recognise them and never report them as malformed.
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
> `ACTIVITY.md`, `MEASUREMENTS.csv`, `CLAUDE.md`, rule files — have no human-authored body to own.

### 2.2 Every generated file

| File | Generated from | Human-owned part | Merge posture (§9) |
| --- | --- | --- | --- |
| `README.md` (entity, container, bucket, root) | frontmatter ← `state.toml` (root: `tree.toml`) | the body — everything after the frontmatter | normal; your prose is at stake |
| `ACTIVITY.md` | the entity's own journal (§3.5) | none | `merge=ours` |
| `MEASUREMENTS.csv` | measurement events in the journal (§4.4) | none | `merge=ours` |
| `AGENTS.md` (root + 4 buckets + `archive/{projects,areas,resources}`) | a delimited para-owned block | everything outside the markers (§6) | normal; your prose is at stake |
| `CLAUDE.md` | wholly — `@AGENTS.md` plus one `@` import per derived rule (§6.1) | none | `merge=ours` |
| `.claude/skills/para-X` | wholly — one mirror per skill, if `emit.claude` (§6.1) | none | `merge=ours` in `copy` mode; n/a for a symlink |
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

Derived at read time, always, because a stored copy rots with no event having occurred:

- `updated`, `attention` (§3.6), and anything else clock-dependent;
- a key-result's `current`, `progress`, `pace`, and derived status (§4);
- effective status and archival dormancy, which are ancestor-dependent (§1.6, §1.7);
- roll-ups of any kind: descendant counts, subtree activity, aggregate progress;
- the set of skills that apply to an entity, and therefore the rules that govern it (§5.2).

Note the asymmetry with §2.2 and be clear about it: `MEASUREMENTS.csv` *does* contain derived columns,
and `ACTIVITY.md` *does* contain a rendered summary. Those are projections — files whose only job is
to be read by something that will not compute. Truth files contain none of it.

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
attention = the newest `at` among events of kind `note` or `measurement`, else `created`
```

The rule is uniform across kinds, including the ones that cannot produce every event: a **skill**
takes no measurements, so its `attention` is its newest `note`, else `created` — which is what
`review --skills` measures `review.cadence` against (§20). A **container** has a journal too, but only
`child` events land in it, so its `attention` is always `created`; nothing reads it.

That is the whole rule, and it is v2's unchanged. `change` events never count — **including status
changes** — because if any entry reset the clock, `para set x --due 2027-01-01` would buy silence
from every check. `child` events do not count either: filing an objective under a project is not the
same as attending to the project, and v2 reached the same answer when adding an objective was a
`change`.

Two threshold knobs, still:

- `stale-after` — days since `attention` before something reads stale.
- `at-risk-pace` — the pace below which a key-result reads `at-risk`.

Gone and staying gone: `blocked-after`, `at-risk-after`, `log-sla`, `sla-reset-at`,
`status-changed-at`, and all three of v1's stored clocks.

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
name        = "Signups report"
description = "when asked for the weekly signups number"
scope       = [
  "projects.acme-migration.objectives.q1-growth",
  "areas.growth",
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

### 5.2 Scope is an explicit locator list, and it covers the subtree

```toml
scope = ["projects", "areas.health.training"]
```

- Entries are fully-qualified locators — the same strings every command prints. A single language, no
  second matcher.
- **Omitting `scope` entirely means the whole tree.** Breadth is the resting state; narrowing is the
  deliberate act. There is no `scope = ["*"]` spelling, because absence already says it.
- **An entry covers that locator and everything beneath it.** `["projects"]` means every project,
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
skill behind it is a standing constraint ("in `areas.finance`, never commit a number you did not
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

**`CLAUDE.md`** is wholly para's, and it is a pointer file — no prose of its own. It carries
`@AGENTS.md` plus one `@` import per derived rule file:

```markdown
@AGENTS.md
@.agents/rules/para-signups-report.md
@.agents/rules/para-commit-style.md
```

Rules reach Claude Code by import rather than by being copied or linked anywhere. The import list
regenerates from the same scope walk that produces the rules (§5.4), so adding or removing a skill
keeps it correct with no separate bookkeeping, and there is no second copy of a rule to drift.

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

---

## 7. Config

`config.toml` in any `.para/`, dotted keys, including the root's.

**Resolution walks the full ancestor chain, nearest wins.** Asking for `stale-after` on
`projects.acme.objectives.q1-growth.key-results.signups` consults, in order: the key-result, its
`key-results/` container, the objective, `objectives/`, the project, `projects/`, and the root. First
value found is the answer.

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
| `emit.gitattributes` | `true` | root |
| `<kind>.stale-after` | `project.stale-after = 14` | root, or any subtree |
| `key-result.at-risk-pace` | `0.8` | root, or one key-result |
| `review.cadence` | `90` | one skill |

`unset` removes a value and resolution continues up the chain. Unset everywhere means the check
never fires.

---

## 8. Truth files

### 8.1 The tree — `.para/tree.toml`, at the root and nowhere else

Schema version and tree identity, and nothing else. No counters, no caches, no index; everything
else is a walk.

```toml
schema       = 1
para-version = "0.4.0"          # the version that last wrote here
name         = "max's brain"
description  = "Everything I am carrying."
created      = "2026-01-01T16:00:00Z"
```

This is the root marker, and it is the root's state — the root has no `state.toml`, because the root
is not an entity, it is the tree. Its README frontmatter generates from here.

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
name        = "Objectives"
description = "What acme-migration is trying to move."
created     = "2026-01-01T16:15:00Z"
```

Identity for the generated README frontmatter, and a place for its config sibling to hang. No status,
no rollups, no child list — the child list is `ls`.

### 8.3 Entity — `.para/state.toml`

```toml
# projects/acme-migration/.para/state.toml
name     = "Acme migration"
status   = "in-progress"
priority = "high"
due      = "2026-09-30"
tags     = ["kafka", "consumer"]
created  = "2026-01-01T16:15:00Z"
```

```toml
# …/key-results/signups/.para/state.toml
name    = "Weekly signups"
type    = "ratio"
start   = "480/9000"
target  = "2000/12000"
due     = "2026-09-30"
created = "2026-01-01T16:15:00Z"
```

**Absent by construction**: `kind`, `id`, `parent`, `locator` — all in the path; `updated`,
`current`, `progress`, `pace`, derived status, `attention` — all read from the journal or computed.
An area's or resource's `archived` — that is the path too (§1.6).

### 8.4 Uniform filenames, and the kind in exactly one place

`state.toml` and `config.toml`, in every `.para/`, for every kind. Not `project.toml`,
`key-result.toml`, `archive-projects.toml`.

The alternative — naming each file for its kind — reads pleasantly in a directory listing, and v3
sketched it that way. It loses on principle 1: the filename would be a **second copy of the kind**,
which the path already states, and two copies with nothing able to adjudicate is the failure v2 named
as the load-bearing risk of the whole design. Rename a directory into a different bucket and the
filename is a lie; para can rewrite it, but a hand-`mv` or a half-applied merge cannot.

Uniform names also make one rule serve everywhere: a projection generator, the walk, `doctor`, and
`rebuild` all open the same path relative to any `.para/` without first deciding what they are looking
at. The cost is that a directory listing no longer announces the kind — which the *directory's own
position* announces instead, and which `README.md`'s frontmatter prints for anyone browsing.

The single non-uniform file is `.para/tree.toml` at the root (§8.1), which names a different kind of
fact and exists once.

### 8.5 The walk

Read a directory's immediate children; descend into those holding `.para/state.toml`; never descend
into content. O(entities).

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
**/CLAUDE.md            merge=ours linguist-generated=true
.agents/rules/**/*.md   merge=ours linguist-generated=true
```

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
`AGENTS.md`. Their human-authored bodies are at stake, and no automatic rule may choose between two
people's prose.

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
| `misplaced` | a `state.toml` at a location that derives no kind — e.g. depth 2 under `projects/`, or a key-result outside a `key-results/` container |
| `invalid` | unparseable TOML, missing required field, enum out of range, a measurement whose shape contradicts its key-result's `type`, a skill with no `description` (it would be inert) |
| `journal` | a line that is not valid JSON, or has no `at`/`kind`, or an unknown `kind`, or an `at` in the future — reported with file and line |
| `collision` | a reserved name used as an id (§1.4) |
| `scope-unresolved` | a `scope` entry naming a locator that does not exist (§5.4) |
| `orphan-rule` | a derived rule file whose `generated_from` skill is gone (§5.3) |
| `orphan-mirror` | something under `.claude/skills/` carrying the `para-` prefix whose skill is gone (§6.1) |
| `broken-link` | a `symlink`-mode mirror that does not resolve — including one materialised as a plain file by a `core.symlinks=false` checkout (§6.1) |
| `stale-projection` | a generated file differs from what would be written now → `para rebuild` (§2.4) |

`stale-projection` is the load-bearing one, because principle 3 rests on it, and it is **full
fidelity**: every generated file is re-derived in memory from truth and compared byte for byte, with
`ACTIVITY.md` re-derived from *every* journal file backing it rather than only the newest. Write-through
guarantees today; this check is what guarantees every day before it (§3.5). Its report names the file
and, for `ACTIVITY.md`, the earliest day that differs — so drift is dated rather than merely detected.

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
  root as the one marker. The kind stays in the path only (§8.4).
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
- **Skills are addressable as `skills.<id>`**, so every universal verb works on them and the command
  surface grows by nothing (§1.4, §14).
- **Para runs no matcher at write time**; it rewrites scope entries on move/rename/archive and
  `doctor` reports unresolvable ones.

**Structure**

- **`objectives/` and `key-results/` are both tracked containers**, uniform with the buckets.
- **Locator = path, always.** No elision of container segments.
- **Entities may not live beneath untracked directories**; untracked directories are invisible;
  **adoption/`import` does not survive**.
- **Archive is a place.** Locators change on archival, subtrees move whole, unarchive cascades
  upward, id collisions are hard errors, and stubs preserve ancestry.
- **Areas and resources lose their status field** — location is archival state.
- **`AGENTS.md` only at root, the four buckets, and `archive/{projects,areas,resources}`**, with a
  delimited para-owned block.
- **`emit.claude` is one flag for one concern** — Claude Code compatibility, off by default. It emits
  `CLAUDE.md` (pointing at `AGENTS.md` and `@`-importing every derived rule) and one symlink per skill
  into `.claude/skills/`. Rules travel by import, not by symlink, so the only mirrored things are the
  ones an import cannot express (§6.1).
- **`emit.claude-skills` chooses `symlink` (default) or `copy`.** Symlink cannot drift and does not
  double the bytes; copy is the escape hatch where links do not survive checkout. **Per-machine
  auto-detection was rejected** — it would make the tree's shape depend on which machine last ran
  `rebuild`. Two flat config keys, because TOML cannot hold `emit.claude = true` and an
  `[emit.claude]` table at once.

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
| `import`/adoption exists. | It does not. | The "point para at my existing repo" story. Bought: entities-all-the-way-up, and therefore locator↔path isomorphism. |
| Git is a precondition. | It is not, and para never invokes git. | Nothing. v2's precondition guarded a projection-merge problem that `merge=ours` now answers. |
| One `entity.md` per entity, fields in its frontmatter. | Two truth files — `state.toml` and `config.toml` — and a generated `README.md`. | Three files where there was one, and a `config.toml` that is usually empty. Bought: policy separable from identity at any level of the tree, and a README body that is yours. |

Kept from v2 without amendment, so nobody re-litigates them: location is kind; kind, id, parent, and
locator all derived from the path and written nowhere else; one locator form; setting a field to its current value writes nothing; pushing
a due date must not buy quiet; `missed` is not terminal; terminal status cascades by derivation;
derived key-result status does not latch; word operators rather than `&`/`|` in any filter; all three
key-result types; one clock and two knobs; no `max-depth`; union merge for append streams; generated
output declares itself in-band rather than via a manifest.

---

## 13. The verb set

Eighteen commands, twenty-two shapes counting `config`'s four. Every one of them is derived from the
model above rather than inherited: where a v2 verb survives, it survives because §1–§10 still needs
it, and where it does not, §13.1 says what deleted it.

| Command | Shape |
| --- | --- |
| `init` | `para init [path]` |
| `add` | `para add <locator> --name … [--field …]` |
| `show` | `para show <locator>` |
| `list` | `para list [<locator>] [filters]` |
| `set` | `para set <locator> --field value […]` |
| `unset` | `para unset <locator> <field> […]` |
| `move` | `para move <from> <to>` |
| `remove` | `para remove <locator> [--keep-files] [--force]` |
| `archive` | `para archive <locator>` |
| `unarchive` | `para unarchive <locator>` |
| `note` | `para note <locator> "text" [--at …]` |
| `measure` | `para measure <kr-locator> <value> [--at …] [--note …]` |
| `log` | `para log <locator> [--kind …] [--limit n] [--reverse]` |
| `activity` | `para activity [<locator>] [--recursive] [--since …]` |
| `review` | `para review [<locator>] [--stale｜--blocked｜--overdue｜--behind｜--skills]` |
| `rebuild` | `para rebuild [<locator>] [--dry-run]` |
| `path` | `para path <locator>` |
| `doctor` | `para doctor [<locator>]` |
| `config` | `set` / `unset` / `list [--prefix …]` / `show <key> [<locator>]` |

Four are new against v2, and each is demanded by a specific decision in the first half:

- **`archive` / `unarchive`** — archival moves bytes now (§1.6), so it is an operation rather than a
  field value. `move` cannot serve: two spellings for one thing violates principle 5, so `move`
  refuses to cross the archive boundary and these two verbs own it.
- **`rebuild`** — projections exist (§2.4), so something has to render them from truth.
- **`activity`** — §3.2 promised the recursive rollup would be a read-time command, and this is it.
  Nothing about it is stored.

### 13.1 What did not survive, and what deleted it

- **`emit`** — deleted by write-through (§2.3). There is no state in which projections are pending, so
  there is nothing to trigger. `rebuild` inherits the only job `emit` still had, and is honest about
  being repair rather than routine.
- **`para skill add|list|remove`** — deleted by skills becoming entities (§5.1) and getting locators
  (§1.4). v2 justified the sub-noun on the grounds that a skill shared none of the field vocabulary;
  in v4 it shares `name`, `description`, `tags`, `created`, and has a journal. `para add
  skills.signups-report --scope …` is the same verb doing the same job, so three shapes disappear
  without anything replacing them.
- **`para rule …`** — never existed, because a rule is a projection (§5.3). Nothing addresses a
  projection.
- **`import`** — deleted by §1.5. There is no adoption, so there is nothing to import.
- **`search`** — deleted by v2 already: `list --match` is cross-kind and is the same command.
- **`--body` and any `$EDITOR` invocation** — para never opens an editor and never takes prose on the
  command line. Bodies are files; you edit files with your editor. v2's principle 3 said the
  filesystem already does this better, and that principle survived the pivot even though principle 2
  did not.
- **`para rules <locator>`** — considered and folded into `show`, which already has a line for the
  skills that apply (§16.1). A verb whose whole output is one line of another verb is not a verb.

---

## 14. Addressing

Every command takes the one locator form from §1.4, fully qualified, and every command prints it that
way, so anything you read pastes into anything you type. Two conveniences, and no more:

- **`.` means the entity or container containing the working directory.** `para show .`, `para note .
  "text"`. It resolves by walking up from `$PWD` to the nearest directory with a `.para/state.toml`,
  which is exactly the walk §8.5 already defines. Cheap, and it is the difference between para being
  usable from inside a project and not.
- **`para path <locator>`** is the inverse, printing one bare line shaped for `$(…)`.

What each command accepts as its argument:

| Argument | Verbs |
| --- | --- |
| entity | `add`, `set`, `unset`, `move`, `remove`, `archive`, `unarchive`, `note`, `measure` |
| entity or container | `show`, `log`, `activity`, `path`, `rebuild`, `doctor`, `review` |
| container, bucket, or root | `list` (positionally — what to look inside) |
| a key-result only | `measure` |

Naming a container where an entity is required is an error that says so: containers hold `name`,
`description`, and `created` and nothing you would want to set (§8.2).

---

## 15. Field vocabulary

One spelling per field, shared by `add` and `set`. `unset` takes bare names.

| Field | project | area | resource | objective | key-result | skill | container |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `name` | req | req | req | req | req | req | req |
| `description` | req | req | req | req | opt | req | req |
| `status` | opt | — | — | opt | derived (`dropped` settable) | — | — |
| `priority` | opt | opt | — | opt | — | — | — |
| `due` | opt | — | — | opt | opt | — | — |
| `tags` | opt | opt | opt | opt | opt | opt | — |
| `created` | opt (now) | opt (now) | opt (now) | opt (now) | opt (now) | opt (now) | opt (now) |
| `type` | — | — | — | — | **req, fixed** | — | — |
| `start` | — | — | — | — | opt | — | — |
| `target` | — | — | — | — | req | — | — |
| `scope` | — | — | — | — | — | opt (all) | — |

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
  already logged (§4.1). Delete and recreate.
- **`scope` replaces wholly on `set`**, and `para unset skills.x scope` widens it back to the whole
  tree (§5.2). `para set skills.x --scope a,b` is the full new list.
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
$ para show projects.acme-migration
projects.acme-migration                          project
Acme migration
  Rebuild the consumer so it stops falling over under replay load.

status       in-progress
priority     high
due          2026-09-30        in 181 days
tags         consumer, kafka
created      2026-01-01
attention    2026-03-02        31 days ago
             stale (stale-after 14, from .para/config.toml)

objectives
  q1-growth  Grow signups                          in-progress
    signups  Weekly signups                        at-risk
             480/9000 → 880/11000 / 2000/12000     progress 0.24   pace 0.70

skills       signups-report (from skills.signups-report, scope projects)
```

- Derived values announce themselves by being *computed lines* — `in 181 days`, `31 days ago`, `stale`,
  `progress`, `pace`, and a key-result's status are never stored (§2.5).
- **`stale` names where its threshold came from**, because §7's chain resolution is only defensible if
  it is visible (§7). Same for any other resolved knob `show` reports.
- **The `skills` line is `para rules` folded in** (§13.1): each entry names the skill and the scope
  entry that reached it, so "why is this rule in my context" is answerable without a second command.
- `show` says when an entity is dormant under a terminal ancestor or lives under `archive/`, because
  its own fields do not explain why it stopped appearing in `list` (§1.6, §1.7).
- `show` is unaffected by terminal-status hiding. You named the thing.

### 16.2 `list`

```
para list [<container-locator>] [filters]
```

Lists **entities** beneath the given locator, at any depth, defaulting to the whole tree.

- **Containers are transparent.** `objectives/` and `key-results/` are never rows in the output and
  are always traversed through, because a row you can neither set nor act on is noise. Name one
  explicitly with `show` when you want it.
- `archive/` is not traversed unless you name it: `para list archive.projects`. Archived things are
  not hidden, they are simply somewhere else, which is the whole point of §1.6.
- Terminal-status items are hidden unless `--all`.

### 16.2.1 Timestamps in output, and `--local`

Every timestamp para *stores* is UTC and every timestamp it *generates into a file* is UTC (§3.5,
§15.1). Terminal output is the one place that is negotiable, because nothing compares it and nothing
commits it:

```
para show projects.acme-migration --local
para log projects.acme-migration --local
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
para activity projects.acme-migration --recursive --since 2026-01-01
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

**`--direct` counts containment as a reader sees it, not as the disk does.** `list projects.acme
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
without a locator would be a guess about tree size, and the cost is proportional to what you asked for.
`--match` under a locator scopes the read to that subtree, which is the answer when it hurts.

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
para add projects.acme-migration --name "Acme migration" --description "Rebuild the consumer."
```

Creates the directory, its `.para/{state.toml, config.toml, logs/}`, its `README.md` (generated
frontmatter, stub body) and `ACTIVITY.md`, **and its child container** — `objectives/` for a project,
`key-results/` for an objective, each with the same shape.

**Containers are created eagerly, not on first child.** A project always has an `objectives/`, empty
or not. The alternative saves four files per childless project and costs a predictable shape: an agent
or a human looking for objectives would have to know that absence means "none" rather than
"elsewhere", and `add` would have to decide whether to create a container as a side effect. Uniform
shape is the reason the tree looks like this at all (§0).

- `add` on an existing locator is an error, not an edit. One verb, one behaviour.
- A missing parent is an error, not an implicit creation. You create a project before its objectives.
- Reserved words (§1.4) and ids colliding with a live sibling are refused by name.
- The new entity's journal starts **empty**: `created` is a field, not an event (§3.1), so
  `attention` falls back to it. The **parent** logs `child added` (§3.3).
- No `--body`, no editor. The README body is a stub you edit yourself; a skill's `SKILL.md` body is
  the same (§13.1).

### 18.2 `set` and `unset`

```bash
para set projects.acme-migration --status blocked --note "waiting on the ingest team"
para set skills.signups-report --scope projects,areas.growth
para unset projects.acme-migration due priority
```

Any number of fields at once, one event per field changed, one write-through pass at the end.
`--note` is **required** when setting `status` to `blocked` — a blocker with no recorded reason is
worthless in six months. *(v1's one exception, kept twice.)*

A skill has no status and no enabled flag (§1.7). `unset skills.x scope` widens it back to the whole
tree; `remove skills.x` is how a skill stops applying, and it takes its derived rule with it (§5.3).

### 18.3 `move`

```bash
para move areas.health.training areas.fitness.training
```

One `rename(2)` where possible, plus the projection work that a hand-`mv` cannot do (§1.5): README
frontmatter, both parents' journals and `ACTIVITY.md`, and **every `scope` entry naming the moved
locator or anything beneath it** (§5.4).

- **Same-kind only.** `move projects.acme areas.acme` is refused: the kind would change, and with it
  which fields are legal. Create the target and move your content.
- **Refuses to cross the archive boundary** in either direction — that is `archive`/`unarchive`.
- Logs `child moved` at the old parent and the new one, and a `change` on the entity itself with
  `field = "locator"`. That last one is the single place a derived value enters a journal, and it
  earns the exception: a journal is a record of what happened, not a copy of current state, and the
  entity's own history is the one place a move must remain visible after the fact. Like every
  `change`, it does not move the clock (§3.6).

### 18.4 `remove`

```bash
para remove projects.acme-migration --keep-files
```

Confirms interactively, naming the blast radius; `--force` skips; `--dry-run` rehearses.

`--keep-files` leaves your content and deletes para's footprint throughout the subtree: every
`.para/`, every `ACTIVITY.md`, every `MEASUREMENTS.csv`, and **the frontmatter block from every
`README.md`, leaving the body**. That last clause is v4's cost for generating into files you also
write in — v2's `--keep-files` was a single `rm -rf .para/`, and it no longer can be.

The parent logs `child removed`.

### 18.5 `archive` and `unarchive`

```bash
para archive projects.acme-migration          # → archive.projects.acme-migration
para unarchive archive.areas.health.training
```

Exactly the semantics of §1.6, and the verbs exist to make them unmistakable:

- `archive` takes the **whole subtree** in one move, creating stubs for any live ancestors that stay
  behind.
- `unarchive` **cascades upward**: unarchiving a child whose parent is archived is refused, naming the
  parent.
- An id colliding with a live sibling on unarchive is refused, naming the sibling.
- Both rewrite `scope` entries, both log `child` events at the old and new parents plus a
  `field = "locator"` change on the entity, and both take `--dry-run`.
- Neither touches a single field. Status says how something ended; the archive says where it lives
  (§1.7).

### 18.6 `note` and `measure`

```bash
para note areas.health.training "swapped the tempo block for intervals"
para measure projects.acme.objectives.q1-growth.key-results.signups 880/11000 --at 2026-01-03
```

The two verbs that move the clock (§3.6), which is why they are verbs of their own rather than
`set` on a field. `measure`'s value grammar follows the key-result's `type` and a mismatch is an
error naming both. A duplicate `--at` on the same key-result is refused (§15.1).

Correcting a past event is appending a corrected one, or editing that one line of the journal by hand
and running `rebuild`. There are no log-entry verbs, no entry ids to pass, and no re-timing flag.

---

## 19. Safety

- **`--dry-run` on `move`, `remove`, `archive`, `unarchive`, and `rebuild`** — the operations that
  touch more than one entity's worth of bytes — and nowhere else. A field change writes inside one
  `.para/` and needs no rehearsal.
- **`remove` confirms interactively**, naming what will be deleted; `--force` skips it. It is the only
  interactive prompt in the surface.
- **`--note` required for `blocked`** (§18.2).
- **Hand-editing truth is legal.** `state.toml` and the journals are yours to edit; they are truth, not
  projections. What you get is stale projections, which `doctor` reports as `stale-projection` and
  `rebuild` repairs. Nothing is corrupted and nothing is lost — which is the property that makes the
  whole write-through design safe to live in.
- **Hand-editing a projection is not an error either**, just pointless: the next mutation or `rebuild`
  overwrites it. `doctor` tells you before that happens.

---

## 20. `review`

```
para review [<locator>] [--stale | --blocked | --overdue | --behind | --skills]
```

| Group | Fires when |
| --- | --- |
| `--stale` | no `note` or `measurement` within `stale-after` (§3.6) |
| `--blocked` | `status` is `blocked`. **No timer** — blocked is always listed |
| `--overdue` | open and past `due` |
| `--behind` | a key-result whose pace is below `at-risk-pace` |
| `--skills` | a skill untouched for longer than its `review.cadence` |

Grouped by reason, ordered within a group by distance past the threshold. Takes `--limit`, not
`--sort` — the ordering is the point. Terminal items and archived things are excluded unless `--all`.

**What each group can contain**, since not everything with a journal is reviewable:

- `--stale`, `--blocked`, `--overdue`, `--behind` cover **entities only**.
- **Skills are reached by `--skills` and nothing else.** A skill has an `attention` (§3.6) but no
  `status` and no `due`, so it can only ever be stale — and its threshold is `review.cadence`, not
  `stale-after`. Putting it in `--stale` would mean one group reading two different knobs.
- **Containers never appear in any group.** They carry no status, no due date, and an `attention` that
  is always `created` (§3.6). A row you can neither act on nor set is the same noise §16.2 keeps out of
  `list`.

**Always exits 0.** Having work is not a failure, and a command that fails whenever you have work is a
command you stop running. `doctor` is where the gate belongs.

`--skills` is new, and it exists because §7's config table has a `review.cadence` key: a knob nothing
reads is a knob that will be wrong, so either the key goes or something surfaces it. A skill nobody
has touched in a year is either load-bearing and worth re-reading, or dead and worth deleting.

---

## 21. `rebuild` and `doctor`

### 21.1 `rebuild`

```
para rebuild [<locator>] [--dry-run]
```

Regenerates every projection under the locator — the whole tree by default — from `state.toml`,
`tree.toml`, and the journals. Idempotent, and it never reads a projection to produce one (§2.4).
`--dry-run` lists what would change without writing.

`ACTIVITY.md` is re-derived **in full**, from every rotated journal file, which is the one thing
write-through never does (§3.5).

### 21.2 `doctor`

```
para doctor [<locator>]
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
para config set --at projects.acme project.stale-after 30
para config unset --at projects.acme project.stale-after
para config list [--prefix emit]
para config show project.stale-after projects.acme.objectives.q1-growth
```

- `set`/`unset` write the **root's** `config.toml` unless `--at <locator>` names a level to write at.
  Root is the default because that is where a knob usually belongs, and `--at` is how §7's chain gets
  built deliberately rather than by accident.
- **`config show <key> [<locator>]` prints the resolved value and the chain that produced it**, marking
  the level that won:

```
$ para config show project.stale-after projects.acme-migration
30

  projects.acme-migration      —
→ projects                     30
  <root>                       14
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
$ para measure projects.acme.objectives.q1-growth.key-results.signups 880/11000
measured signups = 880/11000   progress 0.24   at-risk

wrote  projects/…/key-results/signups/.para/logs/20260101T081502.jsonl
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
| command shapes | 72 | 22 | 22 |
| sub-nouns | `log`, `config`, 6 nouns | `skill`, `config` | `config` |
| locator forms | 2 | 1 | 1 |
| addressable kinds | 6 | 5 | 7 (skills and containers included) |
| threshold knobs | 5 | 2 | 2 |
| per-entity threshold *fields* | 5 | 2 | 0 — the config chain absorbed them (§15) |
| stored clocks | 3 | 0 | 0 |
| generated artifact kinds | `AGENTS.md` + provider dirs + skill copies | `AGENTS.md` | 9 (§2.2) — 8 files + skill symlinks |
| repair commands | `rebuild` | none | `rebuild` |
| files per entity | `entity.md` + N logs | `entity.md` + `log.jsonl` | 2 truth + 2–3 generated + N logs |
| doctor findings | 10 | 7 | 11 |
| authored things in the skills mechanism | skill + rule + wiring | skill | skill |
| interactive prompts | several | `remove` | `remove` |
| editor invocations | `skill add` | `skill add` | 0 |

The honest reading: v4 costs **files** and buys **legibility**. It spends nothing on command surface —
same twenty-two shapes as v2, with one fewer sub-noun and two more addressable kinds — and it spends
nothing on knobs. Every increase in the table is a file count, which was the trade §0 signed for.

---

## 25. Decision log — command surface

- **Verb-first with locators, not noun-verb.** v2's biggest ergonomic win, and §1.4's locator=path
  makes it stronger: the locator carries the kind, so the verb never needs to.
- **Skills use the universal verbs** via `skills.<id>`; the `skill` sub-noun is deleted (§13.1).
- **Four new verbs, each demanded by a first-half decision**: `archive`, `unarchive` (§1.6),
  `rebuild` (§2.4), `activity` (§3.2).
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
- **`config set` defaults to the root**, with `--at <locator>` to build the chain deliberately.
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
para add projects.acme-migration --name "Acme migration" \
  --description "Rebuild the consumer so it stops falling over under replay load."

# an objective — the parent must already exist; key-results/ comes with it
para add projects.acme-migration.objectives.q1-growth \
  --name "Grow signups" --description "Move the top of the funnel."

# a key-result — type is required and permanent
para add projects.acme-migration.objectives.q1-growth.key-results.signups \
  --name "Weekly signups" --type ratio --start 480/9000 --target 2000/12000 --due 2026-09-30

# areas and resources nest freely
para add areas.health --name "Health" --description "Staying in one piece."
para add areas.health.training --name "Training" --description "The weekly plan."

# a skill — scope is a locator list, and this also writes .agents/rules/para-signups-report.md
para add skills.signups-report --name "Signups report" \
  --description "when asked for the weekly signups number" \
  --scope projects.acme-migration,areas.growth

# a skill with no scope applies to the whole tree, and its rule says so
para add skills.commit-style --name "Commit style" \
  --description "when writing a commit message"
```

Refusals, each naming the problem: `para add projects.acme-migration` again (exists);
`para add projects.a.b` (a project cannot nest); `para add projects.objectives` (reserved word);
`para add projects.x.objectives.y.key-results.z --type ratio` with no `--target` (required field).

### `para show` / `list` / `path`

```
$ para list projects --tags kafka --sort attention
projects.acme-migration                 project        in-progress   31 days ago
…q1-growth                              objective      in-progress   31 days ago
…q1-growth.key-results.signups          key-result     at-risk       12 days ago
showing 3 of 3

$ para list --status blocked --all
$ para list archive.projects
$ para path areas.health.training
/Users/max/brain/areas/health/training
$ para show .            # from inside areas/health/training
```

Note what `list` does not show: `objectives`, `key-results`, or any other container (§16.2).

### `para set` / `unset` / `move`

```
$ para set projects.acme-migration --status blocked
error: --note is required when setting status to blocked

$ para set projects.acme-migration --status blocked --note "waiting on the ingest team"
projects.acme-migration  status in-progress → blocked

$ para set projects.acme-migration --status blocked --note "still waiting"
no change (status already blocked); note recorded

$ para move areas.health.training areas.fitness.training
moved  areas.health.training → areas.fitness.training
       rewrote 1 scope entry in skills.training-plan

$ para move projects.acme-migration areas.acme-migration
error: kind would change (project → area); create the target and move your content
```

### `para note` / `measure` / `log` / `activity`

```
$ para measure …key-results.signups 880/11000 --at 2026-01-03
measured signups = 880/11000   decimal 0.0800   progress 0.24   at-risk

$ para measure …key-results.signups 0.08
error: value 0.08 is not a ratio (type ratio expects <numerator>/<denominator>)

$ para measure …key-results.signups 900/11000 --at 2026-01-03
error: a measurement already exists at 2026-01-03T08:00:00Z (2026-01-03T00:00:00-08:00 local)

$ para log projects.acme-migration --kind change --limit 3
$ para activity projects.acme-migration --recursive --since 2026-01-01
2026-01-05  …objectives            added objective q1-growth
2026-01-03  …key-results.signups   measured 880/11000 (8.0%) — 24% of target
2026-01-01  projects.acme-migration created
```

### `para archive` / `unarchive`

```
$ para archive areas.health.training
archived  areas.health.training → archive.areas.health.training
          created stub archive/areas/health/ (parent areas.health is live)

$ para unarchive archive.areas.health
error: nothing to unarchive — archive.areas.health is a stub, not an entity

$ para archive areas.health
archived  areas.health → archive.areas.health   (3 descendants moved with it)
          stub archive/areas/health/ became the entity

$ para unarchive archive.areas.health.training
error: parent archive.areas.health is archived; unarchive it first

$ para unarchive archive.projects.old-migration
error: projects.old-migration exists; rename it or leave this archived
```

### `para review` / `rebuild` / `doctor`

```
$ para review --stale --behind
stale (3)
  areas.fitness.training                    61 days   stale-after 30
  …
behind (1)
  …key-results.signups                      pace 0.70   at-risk-pace 0.80

$ echo "hand-edited" >> projects/acme-migration/ACTIVITY.md
$ para doctor
error  stale-projection  projects/acme-migration/ACTIVITY.md differs from journal (from 2026-01-05)
exit 1

$ para rebuild projects.acme-migration --dry-run
would rewrite  projects/acme-migration/ACTIVITY.md

$ para rebuild projects.acme-migration
rewrote  projects/acme-migration/ACTIVITY.md

$ para doctor
clean
exit 0
```

### `para config`

```
$ para config set --at projects project.stale-after 30
$ para config show project.stale-after projects.acme-migration
30

  projects.acme-migration      —
→ projects                     30
  <root>                       14

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
