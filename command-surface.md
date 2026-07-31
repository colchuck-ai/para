# para — command surface

A PARA tree on disk, driven by a CLI whose grammar is `para <noun> <verb> <locator>`.
This document is the normative surface: the model first, then the grammar, then a worked
example of every command.

---

# 1. Model

## 1.1 The tree

- `para init` creates `projects/`, `areas/`, and `resources/` in the current directory, plus a
  `.para/` directory holding config and marking the tree root.
- Those three, plus `root` — the tree directory itself — are the four locator namespaces. A dir
  or skill at `root` is a sibling of the three buckets on disk, which is exactly why all four
  names are reserved and cannot be used as an id.
- `.para/` is reserved for the same reason: it is not addressable as `root.para`, and it never
  appears in `list`.
- Every other command finds its tree by walking up from the working directory looking for
  `.para/`, the way git finds `.git`.
- `$PARA_HOME` overrides that search when set.

## 1.2 Nouns

| Noun | Bucket(s) | `--parent` | Contains | `import` | `log` |
| --- | --- | --- | --- | --- | --- |
| `project` | `projects` | **none** — projects are flat | objective, dir, skill | yes | yes |
| `area` | `areas` | optional — an area | area, dir, skill | yes | yes |
| `objective` | `projects` | **required** — a project | key-result | no | yes |
| `key-result` | `projects` | **required** — an objective | — | no | yes (measurements) |
| `dir` | any of the four | optional — any container | dir, skill | yes | yes |
| `skill` | any of the four | optional — any container | — | yes | yes |

Projects are flat because they already have a decomposition mechanism, and it is objectives. A
sub-project would compete with an objective for the same job, so `project add --parent` and
`project unset parent` are both errors.

`import` adopts a directory that already exists on disk, which is why it is offered only for the
nouns that can plausibly predate para. A project, an area, a dir, and a skill are all things you
might already have lying around; an objective and a key-result are metadata plus a log, and
nothing lying around is ever one.

`config` is not a noun in this sense: it has `set`, `unset`, `list`, and `show` only, and
addresses `<noun>.<field>` keys rather than locators.

## 1.3 Fields

`req` = required at `add`. `opt` = optional, default in parentheses. `—` = the field does
not exist on that noun, and naming it is an error.

| Field | project | area | objective | key-result | dir | skill |
| --- | --- | --- | --- | --- | --- | --- |
| `name` | req | req | req | req | req | req |
| `summary` | req | req | req | req | req | — |
| `description` | — | — | — | — | — | req |
| `body` | — | — | — | — | — | req |
| `tags` | opt | opt | opt | opt | opt | opt |
| `status` | opt (`planned`) | opt (`active`) | opt (`planned`) | derived | — | — |
| `priority` | opt (`medium`) | opt (`medium`) | opt (`medium`) | — | — | — |
| `due` | opt | — | opt | opt | — | — |
| `log-sla` | opt (config) | opt (config) | opt (config) | opt (config) | — | — |
| `at-risk-pace` | — | — | — | opt (config) | — | — |
| `type` | — | — | — | req, fixed | — | — |
| `start` | — | — | — | opt | — | — |
| `target` | — | — | — | req | — | — |

`id` and `parent` exist on every noun but are not `add` flags — see Grammar.

Rationale for the required ones: `add` requires `--name` and `--summary` on every noun — the
name so compiled headings read well, the summary as a nudge to record context while it is
still in your head. `skill add` requires `--name`, `--description`, and `--body` instead: the
description is the activation trigger and the body is the skill itself; without either the
skill is inert. `key-result add` also requires `--type` and `--target` — no target means no
progress. `dir import` requires `--name` and `--summary`, since a plain directory carries no
metadata of its own; `skill import` does not, because a skill directory has its own
frontmatter and the flags are overrides.

## 1.4 Verbs

| Noun | `add` | `import` | `show` | `list` | `set` | `unset` | `remove` | `log` |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `project` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `area` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `objective` | ✓ | — | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `key-result` | ✓ | — | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `dir` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `skill` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `config` | — | — | ✓ | ✓ | ✓ | ✓ | — | — |

`log` is itself a sub-noun with `add`, `set`, `remove`, `show`, and `list`.

Noun-agnostic commands: `init`, `review`, `search`, `path`, `doctor`.

---

# 2. Naming and locators

- A locator is the full path to an existing thing — never a bare id when the thing is nested.
- Dots separate path segments; hyphens separate words within a segment.
- Nouns that live in exactly one bucket (project, area, objective, key-result) carry no bucket
  prefix: `my-area.sub-area`, `my-project.obj-1.kr-1`
- Nouns that can live in several buckets (dir, skill) are prefixed: `root.`, `projects.`,
  `areas.`, `resources.`
- Every locator also has a fully-qualified form, prefixed with its bucket: `projects.my-project`,
  `areas.my-area.sub-area`, `projects.my-project.obj-1.kr-1`.
- A log entry's locator is its entity's locator plus the entry id as a final segment:
  `my-project.2026-01-01T081502`.

Which form each command takes:

| Command | Form |
| --- | --- |
| Noun commands (`project show`, `area set`, …) | either; the bucket prefix is redundant but never wrong |
| `path`, `review`, `doctor` | fully-qualified only — `my-project` alone cannot say whether it means a project or a dir |
| `search --in` | fully-qualified only |
| `search` itself | a query, not a locator |

All output prints the fully-qualified form, so any locator you read can be pasted into any command.

New ids — passed to `add`, to `import --id`, and to `set <locator> id` — are bare single
segments. Placement never comes from the id. `--id` is optional on `import`, defaulting to a
slugified basename of the imported path.

---

# 3. Grammar

## 3.1 Shape

- The noun is always the command word, never the value of a flag. There is no `--noun`.
- `add` and `import` take **flags**: `--name`, `--summary`, `--tags`, `--due`, …
- `set` and `unset` take **bare field names**: `set my-project due 2026-09-30`,
  `unset my-project due`. Every settable field is the flag name without the dashes.
- `list` takes a container path **positionally**; `show` takes the thing's own path
  positionally. `para skill list projects.my-project` and `para skill show
  projects.my-project.my-skill` are the symmetric pair.

## 3.2 Containment

- Placement is always `--parent <container-path>`, on both `add` and `import`. There are no
  `--root` / `--projects` / `--project` / `--area` / `--dir` / `--objective` scope flags.
- Omitting `--parent` places the thing at the top of its bucket: top-level for area, `root` for
  dir and skill.
- `--parent` is an error on project. Projects are flat, so a project is always at the top of
  `projects`, and `unset <locator> parent` is an error there too.
- `--parent` is required for objective and key-result. An objective belongs to a project and a
  key-result to an objective; neither has a top-of-bucket to fall back to, so
  `unset <locator> parent` is an error on both — the opposite reason, the same shape.
- Re-parenting later is `set <locator> parent <container-path>`, the same word as at creation.
- `unset <locator> parent` promotes to the top of the bucket — top-level for a single-bucket
  noun, `root` for dir and skill.
- A container path matches at any depth. `--direct` restricts to immediate containment.

## 3.3 Notes on every mutation

- `--note` is optional on every mutation — `add`, `import`, `set`, `unset` — and writes a log
  entry. So `log list` shows manual notes and field changes interleaved, and a field change
  resets the SLA clock.
- `remove` takes no `--note`; the entity it would annotate is gone.

## 3.4 Safety

- `--dry-run` on anything that moves files: `remove`, `import`, `set <locator> id`,
  `set <locator> parent`. Plain field changes don't need it.
- `remove` confirms interactively before deleting anything. `--force` skips the prompt for scripts.

## 3.5 Output

- `--json` on every read command: `list`, `show`, `review`, `search`, `doctor`, `log list`,
  `log show`, `config list`, `config show`.
- `path` prints one bare line, already the right shape for `$(...)`.

---

# 4. Fields in detail

## 4.1 Status

| Noun | Values | Default | Terminal |
| --- | --- | --- | --- |
| project, objective | `planned` `in-progress` `blocked` `done` `dropped` | `planned` | `done` `dropped` |
| area | `active` `archived` | `active` | `archived` |
| key-result | derived — see §6 | — | `achieved` `dropped` |

- `list` hides terminal-status items by default; `--all` shows everything, `--status <status>` filters.
- A key-result's `missed` stays visible, because a blown deadline is the last thing that should
  be hidden.
- Archiving never moves anything. Status is the only record of it, and locators are stable for life.

## 4.2 Priority

- `high | medium | low`, default `medium`. On project, area, and objective only.
- A key-result inherits the importance of its objective, but only as prose. It has no `priority`
  field, so `--priority` and `--sort priority` do not apply to `key-result list` — reach for
  `objective list --priority high` instead. Dirs and skills have no priority at all.

## 4.3 Dates

- `--due <YYYY-MM-DD>` is optional on project, objective, and key-result.
- `list --overdue` shows open things past their due date.
- `--due` is date-only by design: a deadline is a day, while a log entry is a moment. See §5.

## 4.4 Tags

- Every noun is taggable. Tags are a comma-separated list: `--tags rust,reference`.

## 4.5 SLA

- `log-sla` is an integer number of days since the last log entry. No unit suffix.
- Only project, area, objective, and key-result have one. An SLA is about attention, and those
  are the things you are on the hook to keep current. A dir or a skill is inert: it still has a
  log, but that log is a changelog rather than a check-in cadence, so it never goes stale.
- Set per entity, defaulted per noun in config: `para config set <noun>.log-sla <days>`
- `unset` removes the explicit value: the field falls back to its config default if it has one,
  otherwise to empty.
- `at-risk-pace` defaults from config and overrides per entity in exactly the same way.

---

# 5. Logs

## 5.1 Ids are timestamps

- A log entry's id *is* its timestamp: `2026-01-01T081502` — hyphens in the date, `T`,
  colon-free time. One path segment, filename-safe on every platform, and lexically sortable.
- Re-timing an entry is therefore just `log set <locator> id <timestamp>`. No separate date or
  ordinal field.
- `--at` accepts progressive precision and fills the rest with zeros: `2026-01-01`,
  `2026-01-01T08`, `2026-01-01T0815`, `2026-01-01T081502`. Omit it entirely and the entry lands
  at now. `log set <locator> id` accepts the same progressive precision.

## 5.2 Collisions

- Several entries may share a timestamp. The second and later carry a stable `-N` suffix
  assigned at creation — `2026-01-01T081502-1` — never renumbered, never shifted by an insert
  or a removal. The suffix is a tiebreaker, not a rank: to order two entries, give them
  different times. Suffixes are assigned as max-existing + 1, so one can be reused after a deletion.
- Key-result measurements are the exception: they must be unique in time. A measurement is a
  reading of a single quantity, so two values at one instant is a contradiction and would make
  "current value" arbitrary. A duplicate timestamp is an error — supply a time, or edit the
  existing reading.

## 5.3 Ordering and clocks

- Ordering is derived from the timestamp, never asserted. Same-second entries order by suffix,
  which is creation order.
- Timestamps are local wall-clock time; the entry records the UTC offset in force when it was
  written. `log list` sorts by the recorded instant, so ids are not strictly monotonic across
  travel or a DST shift.
- Backdating is allowed and is the point of `--at`. Future timestamps are rejected — that is
  what `--due` is for.
- The SLA clock reads the newest log timestamp and reports whole days. A backdated entry never
  resets it; a field change from `--note` lands at now, so it does.

---

# 6. Key results

## 6.1 Type and measurement

- `--type number | ratio | boolean`, fixed at `add` and never settable afterward — changing it
  would invalidate every measurement already logged. Delete and recreate instead.
- One measurement flag, `--value`, whose literal grammar follows the type: `42`, `90/11000`,
  `true`. `--start` and `--target` take the same grammar, so a ratio's baseline keeps its
  reading rather than collapsing to a decimal.
- A ratio's denominator belongs to each reading, not to the key-result — it legitimately varies
  between measurements.
- `--target` is required. `--start` is optional and defaults to the first logged measurement.
- For `--type boolean`, `--target` must be `true` — a target of `false` would be achieved at
  birth — and `--start` is rejected, since `false` is the only sensible baseline.

## 6.2 Derived quantities

- `progress = (current − start) / (target − start)`. Direction falls out of the arithmetic;
  there is no up/down flag.
- With no measurements logged yet, `progress` is 0. No progress has been demonstrated, and
  saying so plainly is what lets an untouched key-result go `at-risk` instead of sitting quiet.
- `pace = progress / elapsed`, where `elapsed = (today − start-date) / (due − start-date)`.
- The start date is the key-result's `created` — not the date of the first measurement. The
  clock starts when you commit to the target, not when you get around to baselining it. So the
  start *value* and the start *date* can come from different moments, deliberately: baseline
  four weeks late and you have genuinely burned four weeks of the window.
- A `--type boolean` key-result has no pace, exactly like one with no `--due`. Its progress is 0
  until done and 1 after, so pace would read `at-risk` for its whole life and then flip to
  `achieved` — noise, not signal.

## 6.3 Derived status

Never set by hand — `dropped` is the only settable key-result status.

| Status | Condition |
| --- | --- |
| `achieved` | `progress >= 1` |
| `missed` | past `due` with `progress < 1` |
| `at-risk` | `pace < at-risk-pace`, but only once one `log-sla` window has elapsed since the start date |
| `on-track` | otherwise |

Without `--due` there is no pace and no deadline, so a key-result only ever reads `achieved` or
`on-track`. A `--type boolean` key-result has no pace either but keeps its deadline, so it reads
`achieved`, `missed`, or `on-track`.

---

# 7. Reading

## 7.1 list

- `list` takes a container path positionally (see §3.2) and filters with `--tags`,
  `--match <text>`, `--status`, `--priority`, `--overdue`, `--direct`, and `--all`.
- `--status`, `--priority`, and `--overdue` apply only to nouns that have those fields.
- `--match` searches that noun's own text — name, summary, tags, and log bodies. On a skill it
  searches name, description, body, and tags.

## 7.2 Sorting and limiting

- `--sort <key>` on any `list`, ascending; `--reverse` flips it. Direction is never folded into
  the key.
- Keys on every noun: `id`, `name`, `created`, `updated`. `updated` reads the newest log timestamp.
- Keys where the field exists: `due`, `priority`, `status`. Key-results add `progress` and `pace`.
- `log list` defaults to `--sort at --reverse` — the one descending default, since a journal
  reads backward. Naming `--sort at` on its own drops the `--reverse` and gives oldest-first,
  because the rule above wins: a key never carries a direction, not even this one.
- Every other `list` defaults to `--sort id`, ascending.
- `--limit <n>` truncates, and the count line says so when it does: `showing 20 of 143`.

## 7.3 search

- `para search <query>` matches names, summaries, tags, and log bodies across every noun — the
  same query `list --match` runs against one.
- `--tags` searches tags alone. `--in <container-path>` scopes to a subtree.
- Results group by noun and then follow the same `--sort` and `--limit` rules as `list`.

## 7.4 review

- `para review` surfaces everything wanting attention, for three reasons: `--stale` (no log
  entry within `log-sla`), `--overdue` (open and past `due`), and `--at-risk` (a key-result
  whose pace has fallen below `at-risk-pace`). Naming a reason restricts to it.
- It takes an optional fully-qualified locator to scope to one subtree.
- Output is grouped by reason and ordered within each group by how far past the threshold an
  item is. It takes `--limit` but not `--sort`; the ordering is the point of the command.

## 7.5 path

- `para path <fully-qualified-locator>` prints the filesystem path as one bare line.

---

# 8. Configuration

- Config lives at the tree root, in `.para/`, and holds per-noun defaults for the fields that
  have them. Keys are `<noun>.<field>`: `para config set project.log-sla 14`.
- Only `log-sla` and `at-risk-pace` take config defaults. Every other field is per-entity or has
  a fixed default (see §1.3).
- `config unset <key>` removes a default. Entities that were falling back to it fall back to
  empty, exactly as `unset` on the entity itself does.
- `config list --prefix <text>` filters by key prefix, matched at whole dot-segments — so
  `--prefix project` matches `project.log-sla`, and `--prefix key` matches nothing rather than
  sloppily catching `key-result.*`.

---

# 9. Storage

Two rules generate the entire layout:

- A directory is an **entity** iff it contains `.para/entity.md`.
- The **tree root** is the directory containing `.para/config.toml` — that is what the walk-up
  search in §1.1 looks for.

## 9.1 Layout

```
brain/
├── .para/
│   └── config.toml                    ← root marker and per-noun defaults
├── my-root-dir/                       root.my-root-dir
│   ├── .para/{entity.md, log/}
│   └── my-dir-dir/.para/…             root.my-root-dir.my-dir-dir
├── projects/
│   └── my-project/                    projects.my-project
│       ├── .para/
│       │   ├── entity.md
│       │   └── log/
│       │       ├── 2026-01-01T081502.md
│       │       └── 2026-01-01T081502-1.md    ← the suffix is part of the filename
│       ├── design.md                  ← user content. para never touches it.
│       ├── obj-1/                     projects.my-project.obj-1
│       │   ├── .para/{entity.md, log/}
│       │   └── kr-1/.para/{entity.md, log/}
│       └── my-project-skill/
│           ├── SKILL.md               ← authoritative for name, description, body
│           └── .para/{entity.md, log/}
├── areas/
│   └── my-area/
│       ├── .para/{entity.md, log/}
│       └── sub-area/.para/{entity.md, log/}
└── resources/
    └── kafka-notes/                   ← adopted; its own files are left alone
        └── .para/{entity.md, log/}
```

## 9.2 What is deliberately never stored

No file records its own `id`, `parent`, or locator. The id is the directory name, the parent is
where the directory sits, and the locator is computed from the path. Three consequences:

- `set <locator> id` and `set <locator> parent` are a single `rename(2)`. A project with three
  objectives and a dozen key-results moves in one operation, because they *are* subtrees and
  move as subtrees. No descendant file is rewritten.
- A locator cannot desync, because there is no second copy of it to drift from the first.
- Moving an entity by hand with `mv` to a **legal** position is simply correct — there is
  nothing for para to update. Drift is only possible into an *illegal* position, which is the
  narrow job §9.7 exists to catch.

## 9.3 entity.md

Frontmatter, with the summary as the body so it has room to breathe. Absent keys mean unset, and
fall back to config where a default exists.

```markdown
---
noun: project
created: 2026-01-01T081500-08:00
name: My Project
status: in-progress
priority: high
due: 2026-09-30
tags: [consumer, kafka]
log-sla: 14
---
Rebuild the consumer so it stops falling over under replay load.
```

`created` is here because §6.2 needs it as the key-result start date, and because `--sort created`
needs it on every noun. A key-result carries `type`, `start`, and `target` as strings, so
`880/11000` keeps its denominator rather than collapsing to `0.08`.

## 9.4 Log entries

One file per entry, under the entity's `.para/log/`. **The filename is the id.**

```markdown
---
offset: "-08:00"
kind: change
field: status
from: planned
to: in-progress
---
got unblocked
```

`kind: note` drops `field`, `from`, and `to`. `kind: measurement` carries `value: "90/11000"`
instead. Having all three kinds in one stream is what makes §3.3's promise real: `log list`
interleaves manual notes with field changes because they are the same kind of file.

`offset` is the only part of the instant a filename cannot carry, so the recorded instant is
filename + offset (§5.3).

One file per entry buys four things at once:

- `log set <locator> id` is a rename, which is why §5.1 can say re-timing is *just* setting the id.
- The `-N` suffix lives in the filename, so "never renumbered" is enforced by the filesystem.
- Measurement uniqueness in time (§5.2) is filename uniqueness — enforced for free.
- Two machines appending logs never conflict in git; different filenames merge cleanly.

And one more, which §9.6 leans on: the newest log timestamp is the **lexically greatest
filename**, so `updated` costs a `readdir` and never opens a file.

## 9.5 Skills and adopted directories

A skill's `SKILL.md` stays authoritative for `name`, `description`, and `body`; the sibling
`.para/entity.md` carries only `noun`, `created`, and `tags`. Nothing is duplicated. This is not
a new decision — §1.3 already says `skill import` does not require `--name` or `--description`
because "the flags are overrides." Storage just obeys it.

An imported `dir` gets a `.para/` written into it and nothing else. That is a real cost: para
does modify the directory it adopts. The alternative — keeping all state in a mirror tree and
linking back by path — makes the link between state and content unverifiable, since a fresh
`git clone` resets every inode. A visible dotfile beats invisible drift.

## 9.6 The walk

Placement is always `--parent`, so an entity is always a *direct* child of its container. The
walk therefore reads each entity's immediate subdirectories, descends only into those that hold
`.para/entity.md`, and never recurses into adopted content. It is O(entities), not O(files).

That is also its one weakness: `mv` an entity down into a plain content subdirectory by hand and
it silently vanishes from every `list`, `search`, and `review` — no error, just absence. `para
doctor` is the deep scan that finds it.

## 9.7 para doctor

`para doctor` walks the tree ignoring the fast path's rules — every directory, including inside
adopted content — and reports what it finds. It is **read-only**. There is no `--fix`: nearly
every problem it can find needs a human choice (where should this orphan live? which of these
two same-instant measurements is right?), and the ones that don't are already legal. So doctor
prints the command that would fix each finding and leaves the running of it to you.

- `para doctor` — the whole tree
- `para doctor <fully-qualified-locator>` — one subtree
- `para doctor --json`
- Exit status is 0 when clean and non-zero when anything is reported, so it scripts.

Findings are grouped by kind:

| Kind | What it means |
| --- | --- |
| `orphan` | An `entity.md` the fast walk cannot reach — buried inside adopted content |
| `misplaced` | A legal entity in an illegal position: an objective outside a project, a key-result outside an objective, a project below the top of `projects`, an area outside `areas` |
| `invalid` | `entity.md` unparseable, `noun` missing or unknown, a required field absent, an enum value not in range, a key-result `type` that does not match its `start`/`target` grammar |
| `log` | A filename that is not a legal timestamp id, unparseable frontmatter, a future timestamp, a `value` on a non-key-result, or two measurements at one instant |
| `collision` | A reserved name — `root`, `projects`, `areas`, `resources`, `.para` — used as an id |

---

# 10. Command reference

## para init

```bash
para init
para init ./my-brain
para init --dry-run
```

## para review

```bash
para review
para review --stale
para review --overdue
para review --at-risk
para review --json
para review --limit 20
para review areas.my-area
para review projects.my-project
```

## para search

```bash
para search "kafka"
para search --tags rust
para search "kafka" --in resources
para search "kafka" --in projects.my-project
para search --tags rust --in resources
para search "kafka" --sort updated --reverse
para search "kafka" --limit 10
para search "kafka" --json
```

## para path

```bash
para path projects.my-project
para path areas.my-area.sub-area
para path resources.rust-reference
```

## para doctor

```bash
para doctor
para doctor --json
para doctor projects.my-project
para doctor areas.my-area.sub-area
```

## para project add|remove|show|list|set|unset|import|log

```bash
para project add my-project --name "My Project" --summary "This is a project" --tags some-tag,other-tag --note "why I'm taking this on"
para project add my-important-project --name "My Important Project" --summary "This is an important project" --priority high
para project add my-started-project --name "My Started Project" --summary "Already underway, skipping the default planned status" --status in-progress
para project add my-deadline-project --name "My Deadline Project" --summary "This one has to land by the end of the quarter" --due 2026-09-30
para project import ./projects/my-existing-project --id my-existing-project --name "My Existing Project" --summary "This is a project that already existed, but that I want to start tracking"
para project import ./projects/finished-thing --id finished-thing --name "Finished Thing" --summary "Already wrapped up before I started tracking" --status done --dry-run

para project list
para project list --all
para project list --status blocked
para project list --priority high
para project list --tags some-tag
para project list --match "consumer"
para project list --overdue
para project list --json
para project list --sort due
para project list --sort updated --reverse
para project list --limit 10
para project show my-important-project

para project set my-project status blocked --note "things outside my control"
para project set my-project status in-progress --note "got unblocked"
para project set my-project status done --note "all finished"
para project set my-important-project status dropped --note "whoops this wasn't important at all"
para project set my-project priority low --note "not as big a deal as the default medium priority"
para project set my-project due 2026-09-30 --note "committing to a date"
para project unset my-project due --note "no longer time-boxed"
para project set my-project log-sla 14 --note "a good cadence"
para project unset my-project log-sla --note "back to the config default"
para project set my-project name "My Project!"
para project set my-project summary "This is a new summary of the exciting project" --note "the old one had drifted"
para project set my-project tags some-tag,other-tag,best-tag
para project unset my-project tags --note "the tags stopped earning their keep"
para project set my-project id my-whoopsie-id-project --note "fat-fingered the rename"
para project set my-whoopsie-id-project id my-project

para project add my-silly-project --name "My Silly Project" --summary "This is a project that will get removed"
para project remove my-silly-project --dry-run
para project remove my-silly-project
```

### para project log add|set|remove|show|list

```bash
para project log add my-project --note "made some progress"
para project log add my-project --at 2026-01-01 --note "backdating some progress"
para project log add my-project --at 2026-01-01T0815 --note "backdating more precisely, so the order is unambiguous"
para project log add my-project --at 2026-01-01 --note "a second entry that lands at the same second, so it gets a -1 suffix"
para project log list my-project
para project log list my-project --limit 20
para project log list my-project --sort at
para project log show my-project.2026-01-01T000000
para project log show my-project.2026-01-01T000000-1
para project log set my-project.2026-01-01T000000 note "a better description of what happened"
para project log set my-project.2026-01-01T000000 id 2026-01-02T0900 --note "off by a day"
para project log remove my-project.2026-01-01T000000-1
```

## para area add|remove|show|list|set|unset|import|log

```bash
para area add my-area --name "My Area" --summary "This is my area" --tags a-tag,another-tag
para area add sub-area --name "My Sub-Area" --summary "This is a sub-area of my-area" --parent my-area
para area add important-area --name "My Important Sub-Area" --summary "This is an important sub-area of my-area" --parent my-area --priority high
para area add silly-area --name "Silly Area" --summary "This area will be removed"
para area import ./areas/my-area/my-existing-area --id my-existing-area --name "My Existing Area" --summary "This is an existing area that I imported" --parent my-area --note "finally tracking this properly"

para area list
para area list --all
para area list --status archived
para area list --tags a-tag
para area list --match "sub"
para area list my-area
para area list my-area --direct
para area list --sort updated
para area show my-area
para area show my-area.sub-area

para area set my-area.sub-area status archived --note "no longer my responsibility"
para area set my-area.sub-area status active --note "whoops it still is, resurrecting"
para area set my-area.sub-area priority low --note "this area is less than the default medium priority"
para area set my-area.sub-area log-sla 180 --note "the default of 90 days was not long enough"
para area unset my-area.sub-area log-sla --note "back to the default"
para area set my-area.sub-area id sub-silly-id-area
para area set my-area.sub-silly-id-area id sub-area
para area set my-area.sub-area parent important-area --note "this belongs under the important area now"
para area unset important-area.sub-area parent --note "promoting it to a top-level area"

para area remove silly-area --dry-run
para area remove silly-area
```

### para area log add|set|remove|show|list

```bash
para area log add my-area.sub-area --note "this is still a relevant area for me"
para area log add my-area.sub-area --at 2026-01-01 --note "this is a backdated review"
para area log list my-area.sub-area
para area log show my-area.sub-area.2026-01-01T000000
para area log set my-area.sub-area.2026-01-01T000000 note "a sharper account of the review"
para area log remove my-area.sub-area.2026-01-01T000000
```

## para objective add|remove|show|list|set|unset|log

```bash
para objective add obj-1 --name "Objective 1" --parent my-project --summary "What I want to achieve" --priority high --due 2026-09-30 --tags okr-2026

para objective list
para objective list --all
para objective list --status blocked
para objective list --tags okr-2026
para objective list --match "achieve"
para objective list --overdue
para objective list my-project
para objective show my-project.obj-1

para objective set my-project.obj-1 status in-progress --note "starting on this"
para objective set my-project.obj-1 status done --note "achieved it"
para objective set my-project.obj-1 status dropped --note "no longer worth pursuing"
para objective set my-project.obj-1 priority medium --note "not as urgent as I thought"
para objective set my-project.obj-1 due 2026-12-31 --note "slipped a quarter"
para objective set my-project.obj-1 log-sla 180 --note "gotta be longer than the default"
para objective unset my-project.obj-1 log-sla --note "back to the config default"
para objective set my-project.obj-1 name "Objective One"
para objective set my-project.obj-1 summary "A sharper statement of what I want to achieve"
para objective set my-project.obj-1 tags okr-2026,stretch
para objective set my-project.obj-1 id obj-one
para objective set my-project.obj-one parent my-other-project --note "this objective belongs to the other project"

para objective remove my-project.obj-1 --dry-run
para objective remove my-project.obj-1
```

### para objective log add|set|remove|show|list

```bash
para objective log add my-project.obj-1 --note "made progress toward the objective"
para objective log add my-project.obj-1 --at 2026-01-01 --note "backdating an update"
para objective log list my-project.obj-1
para objective log show my-project.obj-1.2026-01-01T000000
para objective log set my-project.obj-1.2026-01-01T000000 note "a better account of the progress"
para objective log remove my-project.obj-1.2026-01-01T000000
```

## para key-result add|remove|show|list|set|unset|log

```bash
para key-result add kr-1 --name "Key Result 1" --parent my-project.obj-1 --summary "How we know we've achieved the objective" --type ratio --start 880/11000 --target 110/11000 --due 2026-09-30 --tags okr-2026
para key-result add signups --name "10k signups" --parent my-project.obj-1 --summary "Total self-serve signups" --type number --start 0 --target 10000 --due 2026-09-30
para key-result add shipped --name "Shipped to production" --parent my-project.obj-1 --summary "Whether it is live for all customers" --type boolean --target true
para key-result add churn --name "Churn under 2%" --parent my-project.obj-1 --summary "Monthly logo churn" --type ratio --target 20/1000 --note "no baseline yet, the first measurement becomes the start"

para key-result list
para key-result list --tags okr-2026
para key-result list --match "error rate"
para key-result list --status at-risk
para key-result list --overdue
para key-result list my-project
para key-result list my-project.obj-1
para key-result list --sort progress
para key-result list --sort pace --limit 5
para key-result show my-project.obj-1.kr-1

para key-result set my-project.obj-1.kr-1 log-sla 14 --note "gotta be quicker"
para key-result unset my-project.obj-1.kr-1 log-sla --note "back to the config default"
para key-result set my-project.obj-1.kr-1 at-risk-pace 0.9 --note "this one I want to know about early"
para key-result unset my-project.obj-1.kr-1 at-risk-pace
para key-result set my-project.obj-1.kr-1 target 55/11000 --note "raising the bar"
para key-result set my-project.obj-1.kr-1 start 900/11000 --note "found the real baseline"
para key-result set my-project.obj-1.kr-1 due 2026-12-31 --note "slipped a quarter"
para key-result set my-project.obj-1.kr-1 status dropped --note "we stopped caring about this one"
para key-result set my-project.obj-1.kr-1 name "Key Result One"
para key-result set my-project.obj-1.kr-1 summary "A sharper statement of how we know"
para key-result set my-project.obj-1.kr-1 tags okr-2026,leading-indicator
para key-result set my-project.obj-1.kr-1 id kr-one
para key-result set my-project.obj-1.kr-one parent my-project.obj-2 --note "it measures the other objective better"

para key-result remove my-project.obj-1.kr-1 --dry-run
para key-result remove my-project.obj-1.kr-1
```

### para key-result log add|set|remove|show|list

```bash
para key-result log add my-project.obj-1.kr-1 --value 90/11000 --note "some observation"
para key-result log add my-project.obj-1.kr-1 --value 10/10000 --at 2026-01-01 --note "some reason for why it was backdated, and maybe an observation"
para key-result log add my-project.obj-1.kr-1 --value 12/10000 --at 2026-01-01T1430 --note "a second reading that day needs its own time, since measurements must be unique"
para key-result log add my-project.obj-1.signups --value 4200 --note "halfway there"
para key-result log add my-project.obj-1.shipped --value true --note "it's out"
para key-result log list my-project.obj-1.kr-1
para key-result log show my-project.obj-1.kr-1.2026-01-01T000000
para key-result log set my-project.obj-1.kr-1.2026-01-01T000000 value 10/10001 --note "goofed the reading"
para key-result log remove my-project.obj-1.kr-1.2026-01-01T000000
```

## para config set|unset|list|show

```bash
para config set project.log-sla 14
para config set area.log-sla 90
para config set objective.log-sla 90
para config set key-result.log-sla 30
para config set key-result.at-risk-pace 0.8
para config show project.log-sla
para config unset area.log-sla
para config list
para config list --prefix project
```

## para skill add|remove|show|list|set|unset|import|log

```bash
para skill add my-root-skill --name "My Root Skill" --description "When a particular thing happens anywhere in para" --body "Do this thing"
para skill add my-projects-skill --parent projects --name "My Projects Skill" --description "When a particular thing happens in any para project" --body "Do this other thing"
para skill add my-areas-skill --parent areas --name "My Areas Skill" --description "When a particular thing happens in any para area" --body "Do this other thing"
para skill add my-project-skill --parent projects.my-project --name "My Project Skill" --description "When a particular thing happens in my project" --body "Do this other thing" --tags automation
para skill import ./skills/my-area-skill --parent areas.my-area --id my-area-skill
para skill import ./skills/some-skill --parent root.my-dir --id my-dir-based-skill
para skill import ./skills/another-skill --parent projects.my-project --name "An Overridden Name"

para skill list
para skill list root
para skill list projects
para skill list projects --direct
para skill list areas
para skill list resources
para skill list projects.my-project
para skill list areas.my-area.sub-area
para skill list resources.my-resources-dir
para skill list --tags automation
para skill list --match "lag"
para skill show root.my-root-skill
para skill show areas.my-areas-skill
para skill show projects.my-projects-skill
para skill show resources.my-resources-skill
para skill show areas.my-area.my-area-skill
para skill show projects.my-project.my-project-skill

para skill set projects.my-project.my-project-skill id my-cool-project-skill
para skill set projects.my-project.my-cool-project-skill name "My Cool Project Skill"
para skill set projects.my-project.my-cool-project-skill description "When a particularly cool thing happens in my project"
para skill set projects.my-project.my-cool-project-skill body "Do this cool thing instead"
para skill set projects.my-project.my-cool-project-skill tags automation,favorite
para skill unset projects.my-project.my-cool-project-skill tags
para skill set projects.my-project.my-cool-project-skill parent projects --note "this is useful in every project, not just one"
para skill unset projects.my-cool-project-skill parent --note "actually it is useful everywhere"

para skill remove root.my-cool-project-skill --dry-run
para skill remove root.my-cool-project-skill
```

### para skill log add|set|remove|show|list

```bash
para skill log add projects.my-project.my-project-skill --note "rewrote the body after it misfired twice"
para skill log list projects.my-project.my-project-skill
para skill log show projects.my-project.my-project-skill.2026-01-01T000000
para skill log set projects.my-project.my-project-skill.2026-01-01T000000 note "a clearer account of the rewrite"
para skill log remove projects.my-project.my-project-skill.2026-01-01T000000
```

## para dir add|remove|show|list|set|unset|import|log

```bash
para dir add my-root-dir --name "My Root Directory" --summary "A directory of things" --tags some-tag
para dir add my-projects-dir --parent projects --name "My Directory in Projects" --summary "I keep things here for some reason"
para dir add my-areas-dir --parent areas --name "My Directory in areas" --summary "I keep something here too that isn't part of an area"
para dir add my-resources-dir --parent resources --name "My Dir in resources" --summary "a directory under resources makes sense" --tags rust,reference
para dir add my-area-dir --parent areas.my-area --name "My Area Directory" --summary "A directory under an area"
para dir add my-project-dir --parent projects.my-project --name "My Project Dir" --summary "A project directory that will get special skills"
para dir add my-dir-dir --parent root.my-root-dir --name "My Dir's Dir" --summary "A directory within a directory - neat"
para dir import ./notes/kafka --parent resources --id kafka-notes --name "Kafka Notes" --summary "Notes I already had lying around" --tags kafka,reference
para dir import ./notes/postgres --parent resources --name "Postgres Notes" --summary "No --id, so it lands at resources.postgres"

para dir list
para dir list root
para dir list projects
para dir list areas
para dir list resources
para dir list resources --direct
para dir list projects.my-project
para dir list areas.my-area.sub-area
para dir list --tags rust
para dir list --match "kafka"
para dir list resources --tags rust,reference
para dir list --sort name
para dir show root.my-root-dir
para dir show root.my-root-dir.my-dir-dir
para dir show projects.my-projects-dir
para dir show areas.my-areas-dir
para dir show resources.my-resources-dir
para dir show projects.my-project.my-project-dir

para dir set resources.my-resources-dir name "My Rust Reference Dir"
para dir set resources.my-resources-dir summary "Everything I keep coming back to for Rust"
para dir set resources.my-resources-dir tags rust,reference,favorite
para dir unset resources.my-resources-dir tags
para dir set resources.my-resources-dir id rust-reference
para dir set root.my-root-dir parent resources --note "it belongs in resources after all" --dry-run
para dir set resources.kafka-notes parent resources.rust-reference
para dir unset resources.rust-reference parent --note "back out to the root bucket"

para dir remove root.my-root-dir --dry-run
para dir remove root.my-root-dir
para dir remove root.rust-reference --force
```

### para dir log add|set|remove|show|list

```bash
para dir log add resources.rust-reference --note "pruned the dead links"
para dir log add resources.rust-reference --at 2026-01-01 --note "backdating the big cleanup"
para dir log list resources.rust-reference
para dir log show resources.rust-reference.2026-01-01T000000
para dir log set resources.rust-reference.2026-01-01T000000 note "a fuller account of the cleanup"
para dir log remove resources.rust-reference.2026-01-01T000000
```

---

# 11. Design notes

Why the non-obvious calls went the way they did, recorded so they don't get re-litigated.

- **Every noun has a log; only four have an SLA.** A `--note` on a dir or skill mutation has to
  land somewhere readable, and para mutations are not git commits, so git would not capture the
  reasoning. The asymmetry belongs on `log-sla` instead, where it has a reason of its own:
  attention, not history.
- **Projects are flat.** Objectives are already the decomposition mechanism. A sub-project would
  compete with them for the same job, and `projects.big-thing.phase-1` is better said as an
  objective.
- **A key-result's start date is its `created`.** Deriving it from the first measurement is
  tidier — the start becomes a single (value, date) point — but it means an untouched key-result
  has no pace and so can never go `at-risk`. An abandoned key-result is exactly what `review`
  exists to surface, so the clock has to start at commitment rather than at first reading.
- **Boolean key-results skip pace.** Their progress is 0 until done, so pace would report
  `at-risk` for their entire life and then flip straight to `achieved`.
- **`import` is only on the four nouns that can predate para.** See §1.2.
- **Key-results have no priority.** Inheritance from the objective is a fact about how to read
  them, not a field. Making it a real sort key would mean deriving through a parent, which no
  other derived field does.
- **`--sort at` is ascending like every other key.** Folding a direction into one key would cost
  more than the redundancy of spelling the `log list` default as `--sort at --reverse`.
