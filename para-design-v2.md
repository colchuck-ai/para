# para — design v2

Status: **complete draft**. Every structural decision is settled. Supersedes `command-surface.md`,
which is kept for reference and for its rationale — several fences in it are real and are carried
forward here with credit (§12).

The one job, stated once so every later decision can be checked against it:

> **para is a filing system.** A disciplined PARA tree on disk with stable locators, so that
> you, `grep`, and an agent all have one predictable place to look. Everything else — OKRs,
> the journal, review, emit — is a layer *on top* of the tree, and no layer is allowed to bend
> the tree's shape.

---

## 0. Principles

Four rules. Every section below is downstream of one of them.

1. **Location is kind.** Where a directory sits says what it is. There is no noun word, no
   `--kind` flag, no second copy of the answer.
2. **Store what you cannot derive. Derive nothing you would have to repair.** No caches, no
   projections, no `rebuild`.
3. **The filesystem already does this.** `mkdir`, `mv`, `$EDITOR`, and `git` are better at
   moving, editing, and versioning bytes than para will ever be. para does not re-implement them.
4. **One spelling per thing.** One locator form. One field vocabulary shared by `add` and `set`.
   One clock. One emit driver.

---

## 1. Model

### 1.1 The tree

```
brain/
├── .para/config.toml                    ← root marker, defaults
├── .para/skills/…                       tree-wide skills
├── AGENTS.md                            ← emitted
├── projects/
│   ├── .para/skills/…                   skills for any project
│   ├── AGENTS.md                        ← emitted
│   └── my-project/                      projects.my-project
│       ├── .para/entity.md              fields + objectives + key-results
│       ├── .para/log.jsonl              the journal, an append-only event stream
│       ├── .para/skills/deploy/SKILL.md linked from AGENTS.md, never inlined
│       ├── AGENTS.md                    ← emitted; the region is wholly para's
│       └── design.md                    your content. para never touches it.
├── areas/
│   └── my-area/
│       ├── .para/{entity.md, log.jsonl}
│       └── sub-area/.para/{…}           areas.my-area.sub-area
└── resources/
    └── kafka-notes/.para/{…}            adopted; its own files left alone
```

A **scope** is the tree root, one of the three buckets, or any entity directory. Scopes are what
hold skills and what receive an emitted `AGENTS.md`. The three buckets get a `.para/skills/` when
they need one and never an `entity.md` — they are not entities, they are the buckets.

- `para init` creates the three buckets, `.para/config.toml`, and a `.gitattributes` line making
  journal merges automatic (§6.4).
- Tree root = the directory holding `.para/config.toml`. Every command walks up to find it,
  the way git finds `.git`. `$PARA_HOME` overrides. `init` refuses inside an existing tree.
- **Three buckets, not four.** v1's `root` namespace existed only to host `dir` and `skill`
  entities. Both are gone, so `root` is gone. A directory at the tree root is just a directory.
- No Archive bucket. Status does that job, and archiving never moves anything (§4).

### 1.2 Kind is derived from location

| Location | Kind | Nests? |
| --- | --- | --- |
| `projects/X` | **project** | no — depth 1 only |
| `areas/X`, `areas/X/Y/…` | **area** | yes |
| `resources/X`, `resources/X/…` | **resource** | yes |
| a row inside a project's `entity.md` | **objective** → **key-result** | 2 levels, fixed |
| `<scope>/.para/skills/X/SKILL.md` | **skill** — *not an entity* | one level, under any scope |
| anything else inside an entity | **content**. para does not track it, ever. | — |

An entity is a directory containing `.para/entity.md`. That file never records its own kind,
id, parent, or locator — all four come from the path, so none can desync and `mv` to a legal
position is simply correct. *(v1 §10.2's best idea, kept whole.)*

### 1.3 Objectives and key-results are rows, not directories

They live in the project's own `entity.md`. Reasons:

- v1 §1.2 already conceded they cannot be `import`ed because "nothing lying around is ever one."
  A key-result was a directory containing nothing but `.para/`. That is a row wearing a folder.
- The tree now contains only places you would `cd` into.
- The OKR layer cannot bend the tree: no `misplaced` objectives, no depth rules under `projects/`,
  no `--parent`, no empty folders.
- One journal per directory means one clock per directory (§5), which is also the honest answer:
  you check in on a *project*, not on a key-result in isolation.

Locators still read as paths — `projects.my-project.obj-1.kr-1` — they just resolve inside a
file past the project segment.

### 1.4 Skills live under the scope they apply to

A skill is a directory at `<scope>/.para/skills/<id>/` holding a `SKILL.md`.

```markdown
---
name: Consumer Lag
description: when consumer lag spikes on the ingest topic
enabled: true          ← the off switch. no status field, no archiving verb.
---          ← and no tags. nothing reads them (§14.2).

Check the rebalance log first, then …
```

- **Not entities.** No `.para/entity.md`, no log, no locator of their own beyond
  `<scope-locator>` + id. Nothing to register and nothing to keep in sync.
- `SKILL.md` frontmatter is authoritative for all of it. v1 §10.5 already conceded this while
  keeping a sibling `entity.md` that held almost nothing; v2 drops the sibling.
- **They live under `.para/` so your content stays yours.** v1 put a skill in the content tree as
  an entity, which meant `SKILL.md`, a sibling `.para/`, *and* emitted provider directories all
  landed in a folder you created. v2 writes into `.para/` and into `AGENTS.md`, and nowhere else.
- **Placement is scope, and nothing else expresses scope.** A skill under
  `areas/kafka/.para/skills/` applies when working in `areas/kafka/`. No globs, no `applies-to`
  list of locators (v1 §12 rejected those correctly — a stored second copy of a locator that
  every rename breaks), no tag selectors.
- `description` is the when-to-use hook, and it is the only part that ever enters an agent's
  context automatically (§9). The body is read on demand.

---

## 2. Locators

- **One form, always fully qualified**: `projects.my-project`, `areas.my-area.sub-area`,
  `projects.my-project.obj-1.kr-1`. Every command takes it; every output prints it; anything
  you read can be pasted anywhere. v1's bucket-optional second form (and §2's table explaining
  which command wanted which) is gone.
- Dots separate segments. Hyphens separate words within a segment. Segment charset `[a-z0-9-]`.
- Reserved and unusable as an id: `projects`, `areas`, `resources`, `.para`.
- An id is a bare segment. Placement is the rest of the locator — there is no `--parent`.

---

## 3. Commands

Twenty-two, and fifteen of them are the whole entity surface. v1 had 72 shapes across 6 nouns ×
8 verbs plus a `log` sub-noun.

| Command | Shape |
| --- | --- |
| `init` | `para init [path]` |
| `add` | `para add <locator> --name … [--field …]` — creates the dir, or **adopts** it if present |
| `show` | `para show <locator>` |
| `list` | `para list [<container-locator>] [filters]` — cross-kind; this is also v1's `search` |
| `set` | `para set <locator> [--field value …]` — any number of fields at once |
| `unset` | `para unset <locator> <field> [<field> …]` |
| `move` | `para move <from-locator> <to-locator>` — v1's `set id` **and** `set parent`; one rename(2) |
| `remove` | `para remove <locator> [--keep-files]` |
| `note` | `para note <locator> "text" [--at …]` |
| `measure` | `para measure <kr-locator> <value> [--at …] [--note …]` |
| `log` | `para log <locator>` — print the journal |
| `review` | `para review [<locator>] [--stale\|--blocked\|--overdue\|--behind]` |
| `emit` | `para emit [<locator>] [--dry-run]` |
| `path` | `para path <locator>` — one bare line, `$(…)`-shaped |
| `doctor` | `para doctor [<locator>]` |
| `skill add` | `para skill add <scope-locator> <id> --description … [--name …] [--body …]` |
| `skill list` | `para skill list [<scope-locator>]` |
| `skill remove` | `para skill remove <scope-locator>.<id>` |
| `config` | `set` / `unset` / `list [--prefix …]` / `show` over dotted keys (§10) |

Gone, with reasons:

- **`import`** — `add` adopts an existing directory in place. To bring content in from
  elsewhere you `mv` it first. This deletes `--id`, the move-vs-copy logic, cross-device
  partial failure, the "destination exists" refusal, and import's `--dry-run`.
- **`search`** — with no noun word, `list --match` is already cross-kind. Same command.
- **`rebuild`** — nothing is projected, so nothing needs recomputing (§6).
- **the `log` sub-noun** — `note`, `measure`, and `log` are plain verbs. Correcting a past event is
  appending a corrected one, or editing that one line of `log.jsonl`. This deletes log entry ids,
  the timestamp-id grammar, `-N` collision suffixes, `log set … id` re-timing, and the
  create-entry-must-be-earliest invariant.

### 3.1 Field vocabulary

One spelling per field, shared by `add` and `set`. `unset` takes the bare name.

| Field | project | area | resource | objective | key-result |
| --- | --- | --- | --- | --- | --- |
| `name` | req | req | req | req | req |
| `summary` | req | req | req | req | req |
| `status` | opt | opt | opt | opt | derived (`dropped` settable) |
| `priority` | opt | opt | — | opt | — |
| `due` | opt | — | — | opt | opt |
| `tags` | opt | opt | opt | opt | opt |
| `created` | opt (now) | opt (now) | opt (now) | opt (now) | opt (now) |
| `stale-after` | opt (config) | opt (config) | opt (config) | — | — |
| `type` | — | — | — | — | req, fixed |
| `start` | — | — | — | — | opt |
| `target` | — | — | — | — | req |
| `at-risk-pace` | — | — | — | — | opt (config) |

`created` defaults to now and is settable, taking the same progressive precision as `--at` (§5.1).
Backdate it when you adopt a directory that has been alive for years, or when a key-result's clock
should start at the top of the quarter rather than the day you wrote it down (§7.2). Two bounds, and
no others: **not in the future**, and **not after `due`** — a `created` past `due` would make §7.2's
elapsed fraction negative. `unset created` is an error; everything has a creation time.

Naming a field a kind does not have is an error. Setting a field to the value it already holds
writes nothing and exits 0 — *v1 §3.3's rule, kept: without it a shell loop buys permanent
silence from `review`.*

### 3.1a `skill` is the one sub-noun, and it earns it

`para skill` has three verbs and no more. This is the shape v2 deleted everywhere else, so the
reason it survives here is written down:

- A skill is **not an entity**. It shares none of the field set in §3.1 — no status, no due, no
  priority, no journal, no log. Folding it into `para add` would mean one verb whose legal flags
  depend on whether the locator's last segment happens to be under `.para/skills/`.
- Three verbs, not eight. There is no `skill set`, because §9.3 already establishes that
  `SKILL.md` is **hand-authored** — editing a skill is editing its file, the same way retiming a
  journal event is appending a correction to `log.jsonl`.
- `--body` omitted means `$EDITOR` opens on a stub. Passed inline means no editor, so an agent can
  create one non-interactively.
- Turning a skill off is `enabled: false` in its own frontmatter. `para skill remove` deletes the
  directory. Both are un-wired by the next `para emit`; neither needs a status field.

`para skill add` on an existing id is an error, not an edit. One verb, one behaviour.

### 3.2 Filters, on `list` and where they apply elsewhere

`list` takes a container locator positionally and narrows with:

| Flag | Meaning |
| --- | --- |
| `--tags <expr>` | boolean expression over tags — see below |
| `--match <text>` | that thing's own text: name, summary, tags, journal bodies |
| `--status <s>` | filters on **effective** status (§4) |
| `--priority <p>` | only on kinds that have one |
| `--overdue` | open and past `due` |
| `--direct` | immediate containment only; default matches at any depth |
| `--all` | include terminal-status items |
| `--sort <key>` | ascending; `--reverse` flips. Direction is never folded into a key |
| `--limit <n>` | truncates, and the count line says so: `showing 20 of 143` |
| `--json` | on every read command; carries `total` and `shown` when truncated |

Sort keys: `name`, `created`, `attention`, `locator` everywhere; `due`, `priority`, `status`
where the field exists; `progress` and `pace` on key-results. Naming a key a kind does not have
is an error — same table as §3.1.

**`--tags` is a boolean expression.** Word operators, precedence `not` → `and` → `or`, and **no
parentheses**, because every expression has a disjunctive normal form: `a and (b or c)` is
written `a and b or a and c`. A bare comma list is the short form of the common case and means
`or`. *(v1 §4.4, kept whole.)*

```bash
para list --tags rust
para list --tags rust,reference                        # or
para list --tags "rust and reference"
para list --tags "kafka and not deprecated"
para list --tags "rust and reference or kafka and not deprecated"
```

Operators are words, not `&`/`|`/`!`, because those are shell metacharacters and forgetting to
quote them does not fail — it backgrounds the command and returns a confident wrong answer. With
keywords the same mistake becomes a stray positional argument, which every command taking
`--tags` rejects loudly. `and`, `or`, and `not` are therefore reserved and cannot be tags.

A tag is `[a-z0-9-]`, the same shape as a locator segment. If filters keep accreting, the
direction this grows is one `--where` over all fields with the same vocabulary — not more
operators on more flags.

### 3.3 Safety

- `--dry-run` on `move`, `remove`, and `emit`, and nowhere else. *(v1 §3.4 had this right: a plain
  field change touches one file inside `.para/` and needs no rehearsal.)*
- `remove` confirms interactively, naming the blast radius; `--force` skips.
- `remove --keep-files` deletes only `.para/` throughout the subtree. Now trivial, since `.para/`
  is para's entire footprint inside an entity.
- `--note` is required when setting `status` to `blocked`. *(v1's one exception, kept — a blocker
  with no recorded reason is worthless in six months.)*

### 3.4 Output

`para show` prints the thing itself: stored fields, what is derived at read time, its OKR rows, and
the ids of its skills. It does **not** print the journal (`para log`) or its siblings (`para list`).

```
$ para show projects.consumer-rebuild
projects.consumer-rebuild                       project
My Project
  Rebuild the consumer so it stops falling over under replay load.

status     in-progress
priority   high
due        2026-09-30        in 58 days
tags       consumer, kafka
created    2026-01-01
attention  2026-03-02        31 days ago
           stale (stale-after 14)

objectives
  obj-1  Objective 1              in-progress
    kr-1  p99 under 200ms         at-risk
          480 -> 312 / 200        progress 0.60   pace 0.83

skills     deploy, rollback
```

- Derived values are marked as such by being *computed lines* — `in 58 days`, `31 days ago`,
  `stale`, `progress`, `pace`, and a key-result's status are never stored (§6.2).
- `show` also says when an entity is dormant under a terminal ancestor (§4), because its own status
  does not explain why it stopped appearing in `list`.
- `show` is unaffected by terminal-status hiding. You named the thing.
- `--json` on every read command: `show`, `list`, `log`, `review`, `doctor`, `skill list`,
  `config list`, `config show`. When `--limit` truncates, the JSON carries both `total` and `shown`.
- `path` prints one bare line, already the right shape for `$(…)`.

---

## 4. Status

| Kind | Values | Default | Terminal |
| --- | --- | --- | --- |
| project, objective | `planned` `in-progress` `blocked` `done` `dropped` | `planned` | `done` `dropped` |
| area, resource | `active` `archived` | `active` | `archived` |
| key-result | derived: `on-track` `at-risk` `missed` `achieved`, plus settable `dropped` | — | `achieved` `dropped` |

- One rule for every browsing read: `list` and `review` hide terminal items and accept `--all`;
  `--status` filters. `show` and `doctor` are unaffected.
- `missed` is **not** terminal. A blown deadline is the one thing that should not be hideable.
- **Effective status cascades**, computed from the path — a terminal ancestor makes everything
  beneath it dormant without touching a single descendant's own status, so un-archiving gives
  back the exact previous picture. Nothing stored. *(v1's cleanest derivation. Kept.)*
- Archiving never moves anything. Locators are stable for life.
- Derived key-result status does not latch: hit the target and regress and you read `on-track`
  again, because for "hold p99 under 200ms" that is the only true answer.

---

## 5. The journal, and the one clock

### 5.1 `log.jsonl` — an append-only event stream

One file per entity, `.para/log.jsonl`. One JSON object per line, appended. Nothing is ever
rewritten to add an entry.

```jsonl
{"at":"2026-01-01T08:15:00-08:00","kind":"note","note":"why I'm taking this on"}
{"at":"2026-01-15T09:00:00-08:00","kind":"measurement","kr":"obj-1.kr-1","value":"4200","note":"halfway there"}
{"at":"2026-03-02T14:39:12-08:00","kind":"change","field":"status","from":"planned","to":"in-progress","note":"got unblocked"}
{"at":"2026-03-02T14:40:00-08:00","kind":"note","note":"made some progress"}
```

- Every event carries `at` (RFC 3339, local wall clock with the offset in force when written) and
  `kind`. Every event may carry `note`.
- **Three kinds**: `note`, `change` (`field`/`from`/`to`), `measurement` (`kr`/`value`).
- **There is no `create` kind.** `created` is a field in `entity.md` and lives nowhere else (§6.1),
  so a create event would be a second copy of it — the thing §0's second principle forbids and the
  thing v1 needed a whole invariant to police. `add --note` writes a plain `note` at `created`;
  `add` without a note writes nothing, and an empty `log.jsonl` is a normal state. That deletes the
  kind, "exactly one create per entity", "the create must be earliest", and two `doctor` checks.
- **`note` is a field, not a kind.** A field change carries its reason; a measurement carries its
  observation; a bare `kind: note` is an event with nothing but a note. v1 needed four kinds *plus*
  an optional body *plus* a rule about which bodies mattered; here the reason lives on the event
  that caused it.
- **Ordering is by `at`, never by position in the file.** Ties are simultaneous and stay
  unordered — nothing depends on which came first, so no `-N` suffix, no ids, no tiebreak rule.
- **Measurements of one key-result must be unique in time.** A measurement is a reading of a single
  quantity, so two values at one instant is a contradiction and would make "current value" arbitrary
  — and answering it by file position would break the line above. `para measure` refuses and names
  the colliding time. *(v1 §5.3's exception, kept, and now the only thing that reads the log before
  appending to it.)* Notes and changes are free to collide; nothing computes from their order.
- `--at` backdates, progressive precision (`2026-01-01`, `…T08`, `…T0815`). Future rejected — that
  is what `due` is for.
- Nothing addresses an individual event. Correcting one is editing the line; appending a corrected
  one is usually better and is what the stream is for.
- An event older than the entity's `created` is legal. It looks odd and computes correctly, and
  refusing it would mean an invariant spanning two files to buy nothing.

**Why jsonl rather than one markdown file or one file per event.** No grammar to write and none a
hand edit can silently break — one `Unmarshal` per line, and a bad line is a `doctor` finding that
names the line number. Appending is an append rather than a read-modify-write. And the merge
property v1 bought with distinct filenames comes free from the data instead: two branches both
appending conflict only at the tail, and the resolution is *keep both lines, in any order*, because
order comes from `at`. The cost, stated plainly: a paragraph-long note becomes escaped newlines on
one long line, which reads badly raw. That is what `para log` is for — it renders the stream, which
is also why `para log` is a real command rather than a spelling of `cat`.

### 5.2 One clock

```
attention = the newest `at` among events of kind `note` or `measurement`, else `created`
```

That is the whole rule. Field changes are logged but never count as attention — **including
status changes**. Note that a `change` event carrying a `note` still does not count: attention is
decided by `kind`, never by whether a `note` is present. *(v1 §3.3 reached the same conclusion by a
longer route, and for the right reason: keying on the note would let tuning `stale-after` reset the
clock it just retimed.)*

This is v1's fence rebuilt with one plank instead of twelve. The fence is real: if any entry
resets the clock, `para set x --due 2027-01-01` buys silence from `review`. v1 answered with a
five-bullet eligibility table (v1 §3.3), three stored clocks (v1 §5.4), and a `blocked-after` timer to
plug the hole that status-changes-reset-the-clock opened. Excluding status changes closes that
hole at the source, so `blocked-after`, `sla-reset-at`, and `status-changed-at` all become
unnecessary.

### 5.3 Two knobs, total

- `stale-after` — days since `attention` before something is stale. Per entity, defaulted per
  kind in config.
- `at-risk-pace` — the pace below which a key-result reads `at-risk`. Per key-result, defaulted
  in config. Exists so you can hear about a target *before* it is strictly behind.

Gone: `blocked-after` (nothing resets it now), `at-risk-after` (a key-result with no measurements is
supposed to shout), `log-sla` (renamed to say what it is), and **`max-depth`**.

`max-depth` is worth its own sentence, because v1 built real machinery for it: two config keys, a
creation-time refusal, a `--force` escape, and an advisory `doctor` finding. What it was guarding
against — reference material that belongs in `resources/`, and a sub-area that is really a project —
are filing mistakes, not depth mistakes. A four-deep area can be exactly right and a two-deep one
can be exactly wrong, so depth was a proxy for a judgement it could not make. Areas and resources
nest as deep as you nest them.

---

## 6. Storage

Two rules generate the layout:

- A directory is an **entity** iff it contains `.para/entity.md`.
- The **tree root** is the directory containing `.para/config.toml`.

### 6.1 `entity.md`

Frontmatter for fields, body for the summary so it has room to breathe.

```markdown
---
name: My Project
status: in-progress
priority: high
due: 2026-09-30
tags: [consumer, kafka]
created: 2026-01-01T08:15:00-08:00
stale-after: 14
objectives:
  - id: obj-1
    name: Objective 1
    summary: What I want to achieve
    status: in-progress
    priority: high
    due: 2026-09-30
    key-results:
      - id: kr-1
        name: p99 under 200ms
        type: number
        start: "480"
        target: "200"
        due: 2026-09-30
---
Rebuild the consumer so it stops falling over under replay load.
```

Note what is **absent**: `kind`, `id`, `parent`, `locator` (all in the path); `updated`,
`sla-reset-at`, `status-changed-at`, `current`, `measured-at` (all read from `log.jsonl`).

### 6.2 Nothing is projected

v1 §10.3 copied 4–6 scalars out of the log into `entity.md` so reads would be one file open.
That cache cost: a `rebuild` command to repair it, a `doctor` finding to notice it broke, an
admitted regression in git merge behavior, and an ADR explaining how to fix it by hand. Four
pieces of machinery guarding one cache.

v2 reads `log.jsonl` when it needs a timestamp or a current value. On a tree of a few hundred entities that is a
second small file open per entity — a rounding error, and the profile that would justify the
cache does not exist yet. So: no projection, no `rebuild`, no `projection` doctor finding, and
ADR 0001's manual-resolve-then-rebuild path has nothing to resolve.

Nothing clock-dependent or ancestor-dependent is stored either: `pace`, derived key-result
status, and effective status are computed at read time, because a stored copy rots with no
event having occurred.

### 6.3 The walk

Read every entity's immediate subdirectories; descend only into those holding `.para/entity.md`;
never recurse into adopted content. O(entities). Its one weakness is unchanged from v1: `mv` an
entity into a plain content subdirectory by hand and it vanishes silently from every read.
`doctor` is the deep scan that finds it.

### 6.4 Git

Git is a precondition, as ADR 0001 already states. Two files per entity, and they conflict very
differently:

- `entity.md` conflicts like any config file — your fields, visibly, and no derived copy that could
  be silently wrong after you resolve it. ADR 0001's whole subject (resolve, then `rebuild`) is
  gone with the projection.
- `log.jsonl` conflicts only at the tail, and the resolution is always **keep both lines**, because
  ordering comes from `at` and not from position. So `para init` writes it down for git:

  ```
  # .gitattributes
  **/log.jsonl merge=union
  ```

  Union merge is not a heuristic here, it is the *definition* of merging two append streams whose
  order is carried in the data. ADR 0001 declined to ship a merge driver, and was right to: it was
  weighing a driver that would have had to *choose* between conflicting `entity.md` projections,
  which is a judgement call a heuristic cannot make. There is no judgement here. para appends to
  `.gitattributes` if it exists and creates it if not, and never touches anything else git reads.

---

## 7. Key results

### 7.1 Three types

`--type number | ratio | boolean`, required at creation and **never settable** — changing it would
invalidate every measurement already logged. Delete and recreate instead.

| Type | `--start` / `--target` / `--value` grammar | `start` | Pace |
| --- | --- | --- | --- |
| `number` | `42`, `0.024`, `480` | optional | yes, with `due` |
| `ratio` | `880/11000` | optional | yes, with `due` |
| `boolean` | `true` / `false` | **rejected** — `false` is the only baseline | **no** |

- One measurement flag, `--value`, whose grammar follows the type. `--start` and `--target` take
  the same grammar, so a ratio's baseline keeps its reading rather than collapsing to a decimal.
- **A ratio's denominator belongs to each reading**, not to the key-result — it legitimately varies
  between measurements. So values are stored as strings: `"880/11000"`, never `0.08`. Six months
  later the denominator is the thing you want.
- `--target` is required. `--start` defaults to the first logged measurement. `target == start` is
  rejected: it is not a target, and it would make §7.2's denominator zero.
- `boolean` requires `target: true` — a target of `false` is achieved at birth.

### 7.2 Derived quantities

- `progress = (current − start) / (target − start)`. Direction falls out of the arithmetic; there
  is no up/down flag. **Not clamped**: overshoot reads above 1 and a regression below the baseline
  reads negative, because both are true and both are worth seeing.
- **No measurements yet → progress 0.** No progress has been demonstrated, and saying so plainly is
  what lets an untouched key-result go `at-risk` instead of sitting quiet.
- `pace = progress / elapsed`, where `elapsed = (today − created) / (due − created)`.
- The start *date* is the key-result's `created` — the day you committed to the target, not the day
  you got around to baselining it. So the start *value* and the start *date* can come from different
  moments, deliberately: baseline four weeks late and you have genuinely burned four weeks.
- Pace is **undefined** when `elapsed ≤ 0`, when there is no `due`, or when the type is `boolean`.
  Undefined pace never reads `at-risk`, and `--sort pace` puts it last and prints `—`.

**Why `boolean` skips pace.** Its progress is 0 until done and 1 after, so pace would read
`at-risk` for the key-result's entire life and then flip straight to `achieved` — noise, not signal.
This is also the reason `boolean` survives rather than being spelled `--start 0 --target 1`: the
spelling is available either way, but only a declared type can carry the exemption.

### 7.3 Derived status

Never set by hand. `dropped` is the only settable key-result status.

| Status | Condition |
| --- | --- |
| `achieved` | `progress >= 1` |
| `missed` | past `due` with `progress < 1` |
| `at-risk` | `pace < at-risk-pace`, where pace is defined |
| `on-track` | otherwise |

- **Status does not latch.** A key-result that hits its target and then regresses reads `on-track`
  again. For "hold p99 under 200ms" that is the only honest answer, and latching would mean storing
  an achievement the numbers no longer support.
- Without `--due` there is no pace and no deadline, so it reads only `achieved` or `on-track`.
- A `boolean` has no pace but keeps its deadline, so it reads `achieved`, `missed`, or `on-track`.

---

## 8. Review

```
para review [<locator>] [--stale | --blocked | --overdue | --behind]
```

| Group | Fires when |
| --- | --- |
| `--stale` | no `note` or `reading` within `stale-after` |
| `--blocked` | `status` is `blocked`. **No timer** — blocked is always listed |
| `--overdue` | open and past `due` |
| `--behind` | a key-result whose pace is below `at-risk-pace` |

Grouped by reason, ordered within a group by distance past the threshold. Takes `--limit`, not
`--sort` — the ordering is the point. Terminal items excluded unless `--all`. **Always exits 0**:
having work is not a failure, and a command that fails whenever you have work is a command you
stop running. `doctor` is where the gate belongs.

---

## 9. Emit

`para emit` writes exactly one kind of file — `AGENTS.md` — at the tree root, at each bucket, and
at every entity directory. Nothing else is written. **Nothing is ever copied.**

### 9.1 AGENTS.md is not about para

It never names the CLI, never explains the tree, never prints a locator. It tells an agent how to
operate in the directory it is standing in. An agent working in `areas/kafka/` has no reason to
know a filing tool wrote the file, and para is not the subject.

Three things, and only three:

1. **What this directory is** — the entity's `summary`, verbatim.
2. **The skills at this scope** — each as its `description` (the when-to-use hook) plus a relative
   path to its `SKILL.md`. Hook in context, body one read away.
3. **One line saying directories above have their own**, with skills that may also apply.

```markdown
<!-- para:begin -->
This directory: **Kafka** — keeping the consumer fleet healthy and the lag graphs boring.

Skills for this directory. Read one when its trigger matches; ignore the rest.

- when consumer lag spikes on the ingest topic → `.para/skills/consumer-lag/SKILL.md`
- when a broker is being replaced → `.para/skills/broker-swap/SKILL.md`

Directories above this one have their own `AGENTS.md`, with skills that may also apply.
<!-- para:end -->
```

Bodies are never inlined. `AGENTS.md` is always-loaded context and it repeats at every level, so
anything put in it is paid for on every turn. Linking is what keeps a 400-line skill out of the
context of every agent that never needs it — which is the whole reason a skill has a `description`
separate from its body.

### 9.2 No union of ancestors

v1 §8.2 reasoned: `AGENTS.md` resolution is nearest-only, so a file at the root is read by nothing
standing deeper, **therefore** every file must carry the union of every skill at-or-above it, with
nearer ids winning. The premise is right and the conclusion is expensive.

The cheaper conclusion: parent directories are *knowable*. An agent can walk up. So each file
describes its own scope and says the ancestors exist.

That deletes the union computation, the nearer-id-wins tiebreak, the repetition of every skill at
every depth, and any growth of `AGENTS.md` with tree depth. What survives from v1 §8.2 is the half
that was load-bearing: a file is emitted at **every** scope, not only where a skill exists, because
a directory with no file of its own falls back to an ancestor's — which describes the wrong place.

### 9.3 Ownership

- para writes the whole `<!-- para:begin -->` … `<!-- para:end -->` region and rewrites it on every
  emit. Anything outside the region is untouchable. A file para created whole also carries
  `generated-by: para` in frontmatter.
- Emit prunes marked files it did not produce this run, so deleting a skill — or setting
  `enabled: false` — un-wires it. Without pruning, a skill you deleted keeps firing.
- **No manifest.** A manifest of emitted paths is a second copy of derived state, and losing it
  would orphan output nothing could ever find again. Self-describing files cannot desync.
- Output is **committed** and emit is **deterministic** — the reader may be an agent in a fresh
  clone with no binary installed, and same tree means same bytes, so diff churn is exactly the
  change you made. `doctor` reports staleness as a real defect.
- Emit is **explicit**, never a side effect of a mutation: `SKILL.md` is hand-authored, so output
  can rot with no para command in the loop and a verb is needed regardless.

Gone from v1 §8: the provider enum, the driver interface, `claude-code` and `cursor`, the
`scope: nested|root` policy and its per-provider legality table, the `emit.<provider>.*` config
family, the per-target `tags` filter, the `target` doctor finding, skill copying, symlink-vs-copy
as a question at all, and the union rule.

---

## 10. Config

`.para/config.toml` at the tree root. Dotted keys. `para config` has `set`, `unset`, `list`, `show`.

| Family | Example |
| --- | --- |
| `<kind>.stale-after` | `project.stale-after 14`, `area.stale-after 90` |
| `key-result.at-risk-pace` | `key-result.at-risk-pace 0.8` |
| `emit.enabled` | `emit.enabled true` |

Two families and one flag, against v1's three families — one of them a per-provider namespace whose
targets sprang into existence key by key and so could only be validated after the fact by `doctor`.
Every other field is per-entity or has a fixed default (§3.1).

`config unset` removes a default; entities falling back to it fall back to empty, and empty means
the check never fires.

---

## 11. doctor

Read-only, no `--fix`. Walks every directory including inside adopted content.

**Errors** — a read would be wrong or incomplete. Exit `1`.

| Kind | Means |
| --- | --- |
| `orphan` | an `entity.md` the fast walk cannot reach — buried inside content |
| `misplaced` | an `entity.md` at a location that derives no kind (e.g. depth 2 under `projects/`) |
| `invalid` | unparseable, required field missing, enum out of range, a measurement whose shape does not match its key-result's `type`, a `SKILL.md` with no `description` (it would be inert) |
| `log` | a `log.jsonl` line that is not valid JSON, or has no `at` / no `kind`, or an unknown `kind`, or a future `at`, or a `measurement` naming a key-result that does not exist, or a `change` naming a field the kind does not have. Reported with the line number |
| `collision` | a reserved name used as an id |
| `stale-emit` | emitted output differs from what `emit` would write now → `para emit` |

**Advisory** — your filing is loose. Exit `2` when nothing worse is present.

| Kind | Means |
| --- | --- |
| `untracked` | a plain directory sitting **directly** in a bucket — the one place only entities belong. Content inside an entity is content, and is never reported |

Exit `0` clean, so CI gates on `1` and ignores `2`, and an agent learns from the code alone
whether judgment is required.

Gone from v1's ten: `projection` (nothing is projected), `target` (no emit targets to misconfigure),
and `depth` (§5.3). `log` survives, narrowed to "this line is not a valid event, and here is the
line number." `misplaced` survives but with almost nothing left to catch — under location-is-kind
the only illegal position is an `entity.md` somewhere no kind can be derived from.

---

## 12. What v1 was right about

Carried forward with credit, so nobody re-litigates them:

- **Derive `id`, `parent`, and locator from the path.** Two copies would be equally plausible
  with nothing able to adjudicate. This is the load-bearing idea of the whole design.
- **Setting a field to its current value writes nothing.** Otherwise a loop buys silence.
- **Pushing a due date out must not buy quiet from `review`.** v2 answers it more cheaply, but v1
  found the hole.
- **`blocked` requires a `--note`.**
- **`missed` is not terminal.**
- **Terminal status cascades by derivation, not by writing down the subtree.**
- **`review` exits 0; `doctor` is the gate.**
- **Emitted output declares itself in-band rather than via a manifest.**
- **`AGENTS.md` must be emitted at every scope, not only where a skill exists.** Nearest-only
  resolution means a directory with no file of its own falls back to an ancestor's, which describes
  the wrong place. v1 §8.2 got that premise exactly right. Its *conclusion* — duplicate the union
  of every ancestor's skills into every file — is the one v2 replaces, with "say the ancestors
  exist" (§9.2).
- **Derived key-result status does not latch.**
- **Word operators, not `&`/`|`, for any filter expression** — the symbol version fails by
  backgrounding the command and returning a confident wrong answer.
- **Depth gates bind at creation, not on adoption day.** Failing a whole tree the day you adopt
  a real repo is hostile at the wrong moment.

## 13. Scale, before and after

| | v1 | v2 |
| --- | --- | --- |
| command shapes | 72 | 22 (15 + `skill` × 3 + `config` × 4) |
| nouns in the grammar | 6 + `config` + `log` sub-noun | `skill` + `config`, 3 and 4 verbs |
| locator forms | 2 | 1 |
| threshold knobs | 5 | 2 |
| depth limits | 2 config keys + refusal + `--force` + finding | none |
| stored clocks | 3 | 0 |
| emit drivers | 3 | 0 — one output shape, no driver concept |
| emitted file kinds | `AGENTS.md` + provider dirs + skill copies | `AGENTS.md` |
| `AGENTS.md` size | grows with (skills × depth) | flat: summary + hooks |
| config families | 3 (one per-provider) | 2 |
| doctor findings | 10 | 7 |
| config keys | `<noun>.<field>` × 4, `max-depth.<noun>`, `emit.<provider>.<field>` × 4 | 3 |
| event kinds | 4, plus a body, plus a which-bodies-count rule | 3, with `note` a field on each |
| files per entity | `entity.md` + N log files | `entity.md` + `log.jsonl` |
| repair commands | `rebuild` | none |
| spec length | 1065 lines | ~1000, and ~200 of that is worked examples |

---

## 14. Decision log

Nothing structural is open. §15's worked examples are written and double as the acceptance suite, the
way v1 §11 did for the implementation plan.

### 14.1 Settled after the first draft of this document

- **Tag filters** are v1 §4.4's full boolean expression — word operators, precedence
  `not`/`and`/`or`, no parentheses, comma means `or`. Written up in §3.2.
- **Skills live at `<scope>/.para/skills/<id>/`**, are not entities, and are linked from
  `AGENTS.md` by their `description` hook, never inlined (§1.4, §9.1).
- **`AGENTS.md` never mentions para.** It carries the scope's `summary`, the skill hooks, and one
  line pointing at ancestors (§9.1).
- **No union of ancestors.** Parent directories are knowable; the agent walks up (§9.2).
- **The `claude-code` and `cursor` drivers are gone**, and so is the driver concept. One output
  shape, written entirely by para.
- **`log.jsonl` is an append-only jsonl event stream**, `note` is a field on every event kind, and
  ordering comes from `at` rather than file position (§5.1).
- **`para init` writes `**/log.jsonl merge=union` to `.gitattributes`** — union merge is the
  definition of merging append streams whose order lives in the data, not a heuristic (§6.4).
- **`para show` prints fields, derived values, OKR rows, and skill ids** — not the journal, not
  siblings (§3.4).
- **Four small ones decided rather than asked:** `unset` stays its own verb; a `resource` has no
  `priority`; `--dry-run` is on `move`/`remove`/`emit` only; a skill has no `tags`, since with no
  per-target filter and no skill status nothing would read them.
- **Measurements must be unique in time**; `para measure` refuses a collision and names it. Notes
  and changes may collide freely (§5.1).
- **`created` is a field in `entity.md` and nowhere else**, and the `create` event kind is deleted —
  three kinds, no invariant (§5.1). Bounds: not future, not after `due`.
- **`max-depth` is gone**, with its refusal, its `--force`, its two config keys, and the `depth`
  doctor finding. Depth was a proxy for a filing judgement it could not make (§5.3).
- **All three key-result types survive** — `number`, `ratio`, `boolean` — because only a declared
  type can carry `boolean`'s exemption from pace (§7.1, §7.2).
- **`para skill add | list | remove`** — three verbs, the one surviving sub-noun, and §3.1a records
  why. No `skill set`: `SKILL.md` is hand-authored.

---

## 15. Worked examples

Every command, in the shape a conformance test would run it. v1 §11 had 316 invocations across 72
shapes; this is the same job at 22.

### para init

```bash
para init
para init ./my-brain
para init --dry-run
```

Creates `projects/`, `areas/`, `resources/`, `.para/config.toml`, and appends
`**/log.jsonl merge=union` to `.gitattributes` (§6.4). Refuses inside an existing tree.

### para add

```bash
# projects — always depth 1 under projects/
para add projects.consumer-rebuild --name "Consumer Rebuild" \
    --summary "Rebuild the consumer so it stops falling over under replay load" \
    --tags consumer,kafka --note "why I'm taking this on"
para add projects.q3-launch --name "Q3 Launch" --summary "Ship the thing" \
    --priority high --status in-progress --due 2026-09-30

# areas and resources — nest freely, no depth limit (§5.3)
para add areas.platform --name "Platform" --summary "Keeping the fleet boring"
para add areas.platform.kafka --name "Kafka" --summary "Consumer fleet health"
para add resources.rust --name "Rust Reference" --summary "Things I keep coming back to" \
    --tags rust,reference

# objectives and key-results — rows in the project's entity.md (§1.3)
para add projects.consumer-rebuild.obj-1 --name "Objective 1" \
    --summary "What I want to achieve" --priority high --due 2026-09-30 --tags okr-2026
para add projects.consumer-rebuild.obj-1.latency --name "p99 under 200ms" \
    --summary "Tail latency at the edge" --type number --start 480 --target 200 \
    --due 2026-09-30 --created 2026-07-01 --note "clock starts at the top of the quarter"
para add projects.consumer-rebuild.obj-1.churn --name "Churn under 2%" \
    --summary "Monthly logo churn" --type ratio --target 20/1000 \
    --note "no baseline yet, the first measurement becomes the start"
para add projects.consumer-rebuild.obj-1.shipped --name "Shipped to production" \
    --summary "Live for all customers" --type boolean --target true --due 2026-09-30

# adoption — the directory already exists, so add writes .para/ and nothing else (§3)
mkdir -p resources/kafka-notes && mv ~/notes/kafka/* resources/kafka-notes/
para add resources.kafka-notes --name "Kafka Notes" \
    --summary "Notes I already had lying around" --created 2026-03-14 \
    --tags kafka,reference --note "finally tracking this properly"
```

Errors worth a test: `projects.a.b.c.d` (no kind derives), an id colliding with a sibling, an id of
`projects` / `areas` / `resources` / `.para`, `--priority` on a resource, `--due` on an area,
`--type` on anything but a key-result, `--created` in the future, `--created` after `--due`,
`--target` equal to `--start`, `--start` on a boolean, `--target false` on a boolean.

### para show / list / path

```bash
para show projects.consumer-rebuild
para show projects.consumer-rebuild.obj-1.latency
para show areas.platform.kafka
para show --json projects.consumer-rebuild

para list                                   # whole tree, cross-kind
para list projects
para list areas.platform                    # any depth below
para list areas.platform --direct
para list --status blocked
para list --status at-risk                  # key-results
para list --priority high
para list --overdue
para list --all                             # include done / dropped / archived
para list --match "consumer"                # v1's `search`
para list --tags rust
para list --tags rust,reference             # or
para list --tags "rust and reference"
para list --tags "kafka and not deprecated"
para list --tags "rust and reference or kafka and not deprecated"
para list --sort due
para list --sort attention --reverse
para list --sort pace --limit 5
para list --limit 10 --json

para path projects.consumer-rebuild
cd "$(para path areas.platform.kafka)"
```

### para set / unset / move / remove

```bash
para set projects.consumer-rebuild --status blocked --note "waiting on the platform team"
para set projects.consumer-rebuild --status in-progress --note "got unblocked"
para set projects.consumer-rebuild --due 2026-12-31 --priority high --note "slipped a quarter"
para set projects.consumer-rebuild --created 2026-01-04 --note "it really started the week before"
para set projects.consumer-rebuild.obj-1.latency --target 180 --note "raising the bar"
para set projects.consumer-rebuild.obj-1.latency --at-risk-pace 0.9 --note "hear about it early"
para set areas.platform.kafka --stale-after 180 --note "90 days was not long enough"
para set areas.platform.kafka --status archived --note "no longer my responsibility"

para unset projects.consumer-rebuild due --note "no longer time-boxed"
para unset areas.platform.kafka stale-after --note "back to the config default"
para unset projects.consumer-rebuild tags

para move projects.consumer-rebuild projects.consumer-rewrite          # rename
para move areas.platform.kafka areas.kafka                             # reparent
para move areas.kafka resources.kafka                                  # reclassify buckets
para move projects.p.obj-1.kr-1 projects.p.obj-2.kr-1                  # row to another objective
para move areas.platform.kafka areas.kafka --dry-run

para remove projects.silly --dry-run
para remove projects.silly
para remove projects.silly --force
para remove resources.kafka-notes --keep-files --dry-run
para remove resources.kafka-notes --keep-files
```

Errors worth a test: `set --status blocked` with no `--note` (§3.3); setting a field to the value it
already holds (writes nothing, exit 0, says so); `unset created`; `set --type`; setting a
key-result's `status` to anything but `dropped`; `move` onto an existing locator; `move` a project
to `projects.a.b`.

### para note / measure / log

```bash
para note projects.consumer-rebuild "made some progress"
para note projects.consumer-rebuild "backdating" --at 2026-01-01
para note projects.consumer-rebuild "more precisely" --at 2026-01-01T0815
para note areas.platform.kafka "reviewed the lag dashboards, all boring"

para measure projects.consumer-rebuild.obj-1.latency 312 --note "halfway there"
para measure projects.consumer-rebuild.obj-1.churn 241/9950 --at 2026-08-01
para measure projects.consumer-rebuild.obj-1.shipped true --note "it's out"

para log projects.consumer-rebuild
para log projects.consumer-rebuild --limit 20
para log projects.consumer-rebuild --json
```

Errors worth a test: two measurements of one key-result at the same instant (§5.1); `--at` in the
future; `measure` on something that is not a key-result; a value whose shape does not match the
key-result's `type`.

### para review

```bash
para review
para review --stale
para review --blocked
para review --overdue
para review --behind
para review --all
para review --limit 20
para review --json
para review projects.consumer-rebuild
para review areas.platform
```

Always exits 0 (§8).

### para skill

```bash
para skill add areas.platform.kafka consumer-lag \
    --name "Consumer Lag" \
    --description "when consumer lag spikes on the ingest topic" \
    --body "Check the rebalance log first, then …"
para skill add projects.consumer-rebuild deploy \
    --description "when deploying the consumer"          # no --body: $EDITOR opens on a stub
para skill add projects shared-review \
    --description "when reviewing any project's plan"    # bucket scope
para skill add . house-style \
    --description "when writing anything in this tree"   # tree-root scope

para skill list
para skill list areas.platform.kafka
para skill remove areas.platform.kafka.consumer-lag
```

Turning one off is `enabled: false` in its own `SKILL.md` (§1.4). There is no `skill set`.

### para emit

```bash
para emit
para emit --dry-run
para emit projects.consumer-rebuild
para emit areas.platform
```

Writes `AGENTS.md` at the tree root, at each bucket, and at every entity directory. Nothing else,
nothing copied (§9).

### para config

```bash
para config set project.stale-after 14
para config set area.stale-after 90
para config set resource.stale-after 365
para config set objective.stale-after 30
para config set key-result.at-risk-pace 0.8
para config set emit.enabled true

para config show project.stale-after
para config unset area.stale-after
para config list
para config list --prefix project
para config list --json
```

### para doctor

```bash
para doctor
para doctor --json
para doctor projects.consumer-rebuild
para doctor areas.platform
```

Exit `0` clean, `1` on any error finding, `2` when only advisories are present (§11).
