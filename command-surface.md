# para — command surface

> **Superseded by `para-design-v4.md`.** This is the v1/v2 surface, built on `para <noun> <verb>
> <locator>` — the noun leading as a sub-command. v4's Phase 16 amendment arrives at a different
> noun-verb grammar by a different route: `para <verb> <noun> <chain>`, the noun trailing as an
> ordinary argument that partitions help, flags, and completions for a reader, not one that dispatches
> the verb (see §12 and §25 of `para-design-v4.md`). Nothing below this notice is current.

A PARA tree on disk, driven by a CLI whose grammar is `para <noun> <verb> <locator>`. This document is the normative surface: the model first, then the grammar, then a worked example of every command.

---

# 1. Model

## 1.1 The tree

- `para init` creates `projects/`, `areas/`, and `resources/` in the current directory, plus a `.para/` directory holding config and marking the tree root.
- Those three, plus `root` — the tree directory itself — are the four locator namespaces. A dir or skill at `root` is a sibling of the three buckets on disk, which is exactly why all four names are reserved and cannot be used as an id.
- `.para/` is reserved for the same reason: it is not addressable as `root.para`, and it never appears in `list`.
- Every other command finds its tree by walking up from the working directory looking for `.para/`, the way git finds `.git`.
- `$PARA_HOME` overrides that search when set.
- `init` refuses inside an existing tree. Nesting one tree in another would make the walk-up search find the inner root and silently shadow the outer one, so every command's answer would depend on which directory you happened to be standing in.

## 1.2 Nouns

| Noun | Bucket(s) | `--parent` | Contains | `import` | `log` |
| --- | --- | --- | --- | --- | --- |
| `project` | `projects` | **none** — projects are flat | objective, dir, skill | yes | yes |
| `area` | `areas` | optional — an area | area, dir, skill | yes | yes |
| `objective` | `projects` | **required** — a project | key-result | no | yes |
| `key-result` | `projects` | **required** — an objective | — | no | yes (measurements) |
| `dir` | any of the four | optional — any container | dir, skill | yes | yes |
| `skill` | any of the four | optional — any container | — | yes | yes |

The `Contains` column is authoritative in both directions. An objective holds key-results and nothing else, and a key-result holds nothing, so `dir add` and `skill add` under either is an error — `--parent` accepts "any container," and neither of those is one.

Projects are flat because they already have a decomposition mechanism, and it is objectives. A sub-project would compete with an objective for the same job, so `project add --parent` and `project unset parent` are both errors.

`import` adopts a directory that already exists on disk, which is why it is offered only for the nouns that can plausibly predate para. A project, an area, a dir, and a skill are all things you might already have lying around; an objective and a key-result are metadata plus a log, and nothing lying around is ever one.

`config` is not a noun in this sense: it has `set`, `unset`, `list`, and `show` only, and addresses dotted keys rather than locators.

## 1.3 Fields

`req` = required at `add`. `opt` = optional, default in parentheses. `—` = the field does not exist on that noun, and naming it is an error.

| Field | project | area | objective | key-result | dir | skill |
| --- | --- | --- | --- | --- | --- | --- |
| `created` | opt (now) | opt (now) | opt (now) | opt (now) | opt (now) | opt (now) |
| `name` | req | req | req | req | req | req |
| `summary` | req | req | req | req | req | — |
| `description` | — | — | — | — | — | req |
| `body` | — | — | — | — | — | req |
| `tags` | opt | opt | opt | opt | opt | opt |
| `status` | opt (`planned`) | opt (`active`) | opt (`planned`) | derived | opt (`active`) | opt (`active`) |
| `priority` | opt (`medium`) | opt (`medium`) | opt (`medium`) | — | — | — |
| `due` | opt | — | opt | opt | — | — |
| `log-sla` | opt (config) | opt (config) | opt (config) | opt (config) | — | — |
| `blocked-after` | opt (config) | — | opt (config) | — | — | — |
| `at-risk-pace` | — | — | — | opt (config) | — | — |
| `at-risk-after` | — | — | — | opt (config) | — | — |
| `type` | — | — | — | req, fixed | — | — |
| `start` | — | — | — | opt | — | — |
| `target` | — | — | — | req | — | — |

`id` and `parent` exist on every noun but are not `add` flags — see Grammar. `updated`, `sla-reset-at`, `status-changed-at`, and a key-result's `current` and `measured-at` are not fields you set at all: they are projections of the entity's own log, and §10.3 says where they live and how they are repaired.

`created` defaults to now and is settable, taking the same progressive precision as a log entry's `--at` (§5.1). Backdate it when you `import` a directory that has been alive for years, or when a key-result's clock should start at the top of the quarter rather than the day you got around to writing it down (§6.2). Future values are rejected, a value past `due` is rejected, and `unset <locator> created` is an error — everything has a creation time. It may only move within the window that keeps its create entry first in the log (§5.2): backwards is always available, forwards only into the gap before you first touched the thing.

Rationale for the required ones: `add` requires `--name` and `--summary` on every noun — the name so emitted headings read well, the summary as a nudge to record context while it is still in your head. `skill add` requires `--name`, `--description`, and `--body` instead: the description is the activation trigger and the body is the skill itself; without either the skill is inert. `key-result add` also requires `--type` and `--target` — no target means no progress. `dir import` requires `--name` and `--summary`, since a plain directory carries no metadata of its own; `skill import` does not, because a skill directory has its own frontmatter and the flags are overrides.

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

Noun-agnostic commands: `init`, `emit`, `rebuild`, `review`, `search`, `path`, `doctor`.

---

# 2. Naming and locators

- A locator is the full path to an existing thing — never a bare id when the thing is nested.
- Dots separate path segments; hyphens separate words within a segment.
- Nouns that live in exactly one bucket (project, area, objective, key-result) carry no bucket prefix: `my-area.sub-area`, `my-project.obj-1.kr-1`
- Nouns that can live in several buckets (dir, skill) are prefixed: `root.`, `projects.`, `areas.`, `resources.`
- Every locator also has a fully-qualified form, prefixed with its bucket: `projects.my-project`, `areas.my-area.sub-area`, `projects.my-project.obj-1.kr-1`.
- A log entry's locator is its entity's locator plus the entry id as a final segment: `my-project.2026-01-01T081502`.

Which form each command takes:

| Command | Form |
| --- | --- |
| Noun commands (`project show`, `area set`, …) | either; the bucket prefix is redundant but never wrong |
| `path`, `review`, `doctor`, `emit` | fully-qualified only — `my-project` alone cannot say whether it means a project or a dir |
| `search --in` | fully-qualified only |
| `search` itself | a query, not a locator |

All output prints the fully-qualified form, so any locator you read can be pasted into any command.

New ids — passed to `add`, to `import --id`, and to `set <locator> id` — are bare single segments. Placement never comes from the id. `--id` is optional on `import`, defaulting to a slugified basename of the imported path. An id that would collide with an existing sibling is an error, reported by `--dry-run` before anything moves.

---

# 3. Grammar

## 3.1 Shape

- The noun is always the command word, never the value of a flag. There is no `--noun`.
- `add` and `import` take **flags**: `--name`, `--summary`, `--tags`, `--due`, …
- `set` and `unset` take **bare field names**: `set my-project due 2026-09-30`, `unset my-project due`. Every settable field is the flag name without the dashes.
- `list` takes a container path **positionally**; `show` takes the thing's own path positionally. `para skill list projects.my-project` and `para skill show projects.my-project.my-skill` are the symmetric pair.

## 3.2 Containment

- Placement is always `--parent <container-path>`, on both `add` and `import`. There are no `--root` / `--projects` / `--project` / `--area` / `--dir` / `--objective` scope flags.
- Omitting `--parent` places the thing at the top of its bucket: top-level for area, `root` for dir and skill.
- `--parent` is an error on project. Projects are flat, so a project is always at the top of `projects`, and `unset <locator> parent` is an error there too.
- `--parent` is required for objective and key-result. An objective belongs to a project and a key-result to an objective; neither has a top-of-bucket to fall back to, so `unset <locator> parent` is an error on both — the opposite reason, the same shape.
- Re-parenting later is `set <locator> parent <container-path>`, the same word as at creation.
- `unset <locator> parent` promotes to the top of the bucket — top-level for a single-bucket noun, `root` for dir and skill. Note the asymmetry that follows: for an area this stays inside `areas`, while for a dir or skill already sitting directly in a bucket there is nothing above it but `root`, so unsetting placement moves it out of the bucket entirely. That is a reclassification, and `--dry-run` will show it as one.
- A container path matches at any depth. `--direct` restricts to immediate containment.
- `add` refuses to place an area or a dir deeper than `max-depth.area` or `max-depth.dir`, naming the two mistakes that produce depth — reference material that belongs in `resources`, and a sub-area that is really a project — and offering `--force`. Enforcement lives at creation because that is where it binds: you cannot `mkdir` your way into an entity, so the gate catches essentially everything that is not an import or a hand edit. Those two populations get an advisory `doctor` finding instead of a refusal, so adopting an already-deep repo works on day one (§10.8).

## 3.3 Notes on every mutation

- **Creation writes one entry.** `add` and `import` write a single `kind: create` entry whose timestamp is the entity's `created`. `--note` supplies its body. Initial field values are not logged as changes — they are the entity's starting state, and they live in `entity.md`.
- **Every field change writes a log entry**, recording the field, the old value, and the new one. `--note` is optional on `add`, `import`, `set`, and `unset`, and supplies that entry's body. So `log list` shows manual notes and field changes interleaved, always — not only when you remembered to explain yourself.
- **Setting a field to the value it already has does nothing.** No entry, no clock reset, exit 0 with `already in-progress; nothing written`. The same holds for `unset` on an unset field and for `log set` to identical text. A change entry from a value to itself would carry no information, and — because status changes reset the SLA clock — writing one would let a shell loop buy permanent silence from `review`. If you want to record that you re-confirmed something, `log add --note` says that honestly and resets the clock for the right reason. Re-measuring a key-result is *not* covered by this rule: logging `42` again at a new time is a fresh reading, a real claim about the world.
- **`--note` is required when setting `status` to `blocked`.** It is the one exception in this section, and it exists because a blocker with no recorded reason is worthless in six months. See also `review --blocked` (§7.4), which is what keeps a blocked thing from going quiet.
- **Only some entries reset the SLA clock**: a note you write by hand with `log add`, a key-result measurement, and a change to `status`. Every other field change is recorded but does not count as attention, with or without a `--note`. A `kind: create` entry does not reset it either — it *is* the clock's starting point (§5.4).
- The line is between reporting and planning. `blocked`, `in-progress`, and `done` are claims about the world, and so is a measurement. A new due date, a re-tag, a sharper summary, a tuned `log-sla` — those are edits to your own plan, and editing the plan is not the same as looking at the thing.
- So pushing a deadline out never buys quiet from `review`, and tuning `log-sla` never resets the clock it just retimed. See §5.4 for how the SLA clock and `updated` differ.
- `remove` takes no `--note`; the entity it would annotate is gone.

## 3.4 Safety

- `--dry-run` on anything that moves or writes files outside the entity: `remove`, `import`, `set <locator> id`, `set <locator> parent`, `unset <locator> parent`, `emit`, `rebuild`. Plain field changes don't need it.
- `remove` confirms interactively before deleting anything. `--force` skips the prompt for scripts.
- `import` **moves**. Inside a tree that is one `rename(2)`; across devices it is a copy followed by an unlink. The source may be any readable path, inside the tree or out of it. An import whose destination already exists is an error — para never merges two directories.

## 3.5 Removing a container

`remove` takes the whole subtree — the entity, every entity nested inside it, and every other file in its directory. It is the one command that can destroy content para never created, so:

- The confirmation names the blast radius rather than asking a bare "are you sure": the count of descendant entities by noun, and the count of other files that will go with them.
- `--dry-run` prints the full list instead of the counts.
- `--keep-files` removes only para's `.para/` directories throughout the subtree, leaving every other byte where it is. The tree stops tracking the thing; your notes survive. This is the inverse of `import`, and it is the safe option when the content predates para. It is legal on an objective or a key-result too, where it simply leaves an empty directory behind.
- Removing anything also removes what `emit` wrote for it, on the next `emit` (§8.3).

Nothing here is the way to get something out of sight — `dropped` and `archived` already do that, and §4.1 keeps locators stable for life. `remove` means delete.

## 3.6 Output

- `--json` on every read command: `list`, `show`, `review`, `search`, `doctor`, `log list`, `log show`, `config list`, `config show`. When `--limit` truncates, the JSON carries both `total` and `shown`.
- `path` prints one bare line, already the right shape for `$(...)`.

---

# 4. Fields in detail

## 4.1 Status

| Noun | Values | Default | Terminal |
| --- | --- | --- | --- |
| project, objective | `planned` `in-progress` `blocked` `done` `dropped` | `planned` | `done` `dropped` |
| area, dir, skill | `active` `archived` | `active` | `archived` |
| key-result | derived — see §6 | — | `achieved` `dropped` |

**One rule covers every browsing read.** `list`, `search`, and `review` hide terminal-status items by default and all accept `--all`; `--status <status>` filters. `show` is unaffected, because you named the thing. `doctor` is unaffected, because it validates whatever exists.

- A key-result's `missed` is not terminal, so a blown deadline stays visible and stays in `review`. It demands a decision — drop it, re-target it, or move the date — and nothing silences it but making one.
- Archiving never moves anything. Status is the only record of it, and locators are stable for life.

**Effective status cascades.** A thing's effective status is its own, unless any ancestor's is terminal. Setting a project `done` therefore quiets its objectives and key-results, and archiving an area quiets everything beneath it, without touching a single descendant's own status — so restoring an area restores the previous picture exactly, including a sub-area you archived independently last year. Nothing is stored: "is any ancestor terminal" is computed from the path, the same way the locator is (§10.2). `show` prints an entity's own status and says when it is dormant under a terminal ancestor; `--status` filters on effective status.

A dir and a skill have a status for the same reason an area does — superseded reference material and a misfiring skill both need a way to be put aside without being deleted. Two consequences: an archived dir stays findable with `search --all`, which untracking it would not (`remove --keep-files` makes content invisible to para entirely); and `emit` skips archived skills, so archiving is how you turn a skill off. Combined with the cascade, archiving any container disables every skill beneath it in one move.

Status and `log-sla` remain separate concerns. A dir has a status and still has no SLA — §4.5 explains why.

## 4.2 Priority

- `high | medium | low`, default `medium`. On project, area, and objective only.
- A key-result inherits the importance of its objective, but only as prose. It has no `priority` field, so `--priority` and `--sort priority` do not apply to `key-result list` — reach for `objective list --priority high` instead. Dirs and skills have no priority at all.
- Priority ranks and filters. It does not suppress: `low` buys no quiet from `review`. See §12.

## 4.3 Dates

- `--due <YYYY-MM-DD>` is optional on project, objective, and key-result.
- `list --overdue` shows open things past their due date.
- `--due` is date-only by design: a deadline is a day, while a log entry is a moment. See §5.
- `created` later than `due` is rejected — it would make §6.2's elapsed fraction negative. It is also bounded by its own log (§5.2).

## 4.4 Tags

- Every noun is taggable. A tag is `[a-z0-9-]`, the same shape as an id segment. `and`, `or`, and `not` are reserved words and cannot be tags.
- On `add`, `import`, and `set`, `--tags` takes a plain comma-separated list, because it is assigning a set: `--tags rust,reference`.
- As a **filter**, `--tags` takes a boolean expression: `--tags "rust and reference"`, `--tags "kafka or postgres"`, `--tags "kafka and not deprecated"`.
- Precedence is `not`, then `and`, then `or`. **There are no parentheses, and none are needed**: every boolean expression has a disjunctive normal form, so `a and (b or c)` is written `a and b or a and c`. Dropping grouping costs nothing in expressiveness and removes the only characters that would have forced a parser with error positions.
- A bare comma list is still legal as a filter and means `or`: `--tags rust,reference` is exactly `--tags "rust or reference"`. It is the short form of the common case, which is why no example in §11 has to change.

The operators are words rather than `&`, `|`, and `!` because those are shell metacharacters, and the failure they produce is the dangerous kind: `para search --tags a & b` backgrounds the search and returns a wrong answer that looks right. With keywords, the same mistake hands para a stray positional argument — and every command taking `--tags` has a positional slot, a query or a container path — so it fails loudly instead. The price is quoting any expression containing spaces, which is the reflex `find -name "*.md"` already trains.

## 4.5 SLA and thresholds

- `log-sla` is an integer number of days since the last resetting log entry. No unit suffix.
- Only project, area, objective, and key-result have one. An SLA is about attention, and those are the things you are on the hook to keep current. A dir or a skill is inert: it still has a log, but that log is a changelog rather than a check-in cadence, so it never goes stale.
- `blocked-after` is an integer number of days a project or objective may sit `blocked` before `review --blocked` reports it. Unlike the SLA clock, notes and status churn do not reset it — only leaving `blocked` does.
- `at-risk-pace` is the pace below which a key-result reads `at-risk`.
- `at-risk-after` is how many days must pass after a key-result's `created` before its pace is judged at all. Early in a window, progress is lumpy and pace is meaningless, so this is the grace period. It is its own knob rather than a reuse of `log-sla` because check-in cadence and time-to-first-judgement are unrelated things: tightening your cadence from 30 days to 7 should not make every key-result start shouting three weeks earlier.
- All four are set per entity and defaulted per noun in config: `para config set <noun>.<field> <value>`
- `unset` removes the explicit value: the field falls back to its config default if it has one, otherwise to empty. Empty means the check never fires.

---

# 5. Logs

## 5.1 Ids are timestamps

- A log entry's id *is* its timestamp: `2026-01-01T081502` — hyphens in the date, `T`, colon-free time. One path segment, filename-safe on every platform, and lexically sortable.
- Re-timing an entry is therefore just `log set <locator> id <timestamp>`. No separate date or ordinal field.
- `--at` accepts progressive precision and fills the rest with zeros: `2026-01-01`, `2026-01-01T08`, `2026-01-01T0815`, `2026-01-01T081502`. Omit it entirely and the entry lands at now. `log set <locator> id` accepts the same progressive precision.
- An entry id that would collide with an existing entry is an error.

## 5.2 Kinds

Four kinds share one stream: `create`, `note`, `change`, and `measurement`.

- Exactly one `create` entry exists per entity, written by `add` or `import`, and its timestamp is the entity's `created`. `log remove` on it is an error, and `set <locator> created` re-times it — which is why `created` cannot drift from it (§10.3).
- **The create entry must remain the earliest entry in the log.** `set <locator> created` is rejected unless the new timestamp falls strictly before the entity's oldest other entry. One rule does three jobs: the history can never show a field change that predates creation, the create entry can never collide with an existing entry — nothing exists earlier than it, so any legal target time is free — and `created` can only move backwards, or forwards into the gap before the thing was first touched. Because a collision is impossible, `set created` renames a file and still needs no `--dry-run`.
- `note` is what you write by hand. `change` is written by every field mutation. `measurement` is a key-result reading.

## 5.3 Collisions

- Several entries may share a timestamp. The second and later carry a stable `-N` suffix assigned at creation — `2026-01-01T081502-1` — never renumbered, never shifted by an insert or a removal. The suffix is a tiebreaker, not a rank: to order two entries, give them different times. Suffixes are assigned as max-existing + 1, so one can be reused after a deletion.
- Key-result measurements are the exception: they must be unique in time. A measurement is a reading of a single quantity, so two values at one instant is a contradiction and would make "current value" arbitrary. A duplicate timestamp is an error — supply a time, or edit the existing reading.

## 5.4 Ordering and clocks

- Ordering is derived from the timestamp, never asserted. Same-second entries order by suffix, which is creation order.
- Timestamps are local wall-clock time; the entry records the UTC offset in force when it was written. `log list` sorts by the recorded instant, so ids are not strictly monotonic across travel or a DST shift.
- Backdating is allowed and is the point of `--at`. Future timestamps are rejected — that is what `--due` is for.
- Three clocks come off this stream, and §10.3 stores all three so no read has to walk the log to find them:
  - **`updated`** — the newest entry of any kind. A bare field change bumps it, because the entity genuinely was touched. It is the default meaning of `--sort updated`.
  - **`sla-reset-at`** — the newest entry that *resets* the SLA clock: a hand-written note, a measurement, or a `status` change (§3.3). When an entity has no such entry it is `created`, so a brand-new thing starts its window at birth.
  - **`status-changed-at`** — the newest `status` change. `review --blocked` reads it against `blocked-after`.
- A backdated entry never resets a clock, since it is not the newest; a resetting change made now lands at now, so it does.
- In practice a key-result is kept fresh by measuring it, and an area by reviewing it — flipping an area between `active` and `archived` is a real status change and does reset the clock, but it is not something you do as a check-in.

---

# 6. Key results

## 6.1 Type and measurement

- `--type number | ratio | boolean`, fixed at `add` and never settable afterward — changing it would invalidate every measurement already logged. Delete and recreate instead.
- One measurement flag, `--value`, whose literal grammar follows the type: `42`, `90/11000`, `true`. `--start` and `--target` take the same grammar, so a ratio's baseline keeps its reading rather than collapsing to a decimal.
- A ratio's denominator belongs to each reading, not to the key-result — it legitimately varies between measurements.
- `--target` is required. `--start` is optional and defaults to the first logged measurement. A `target` equal to `start` is rejected: it is not a target, and it would make §6.2's denominator zero.
- For `--type boolean`, `--target` must be `true` — a target of `false` would be achieved at birth — and `--start` is rejected, since `false` is the only sensible baseline.

## 6.2 Derived quantities

- `progress = (current − start) / (target − start)`. Direction falls out of the arithmetic; there is no up/down flag.
- Progress is not clamped. Overshoot reads above 1 and a regression below the baseline reads negative, because both are true and both are worth seeing.
- With no measurements logged yet, `progress` is 0. No progress has been demonstrated, and saying so plainly is what lets an untouched key-result go `at-risk` instead of sitting quiet.
- `pace = progress / elapsed`, where `elapsed = (today − start-date) / (due − start-date)`.
- Pace is undefined when `elapsed ≤ 0` — on the start date itself the divisor is zero. Undefined pace never reads `at-risk`, and `--sort pace` puts it last and prints `—`.
- The start date is the key-result's `created` — not the date of the first measurement, and settable (§1.3), so the clock can be moved to the day the commitment really began. The clock starts when you commit to the target, not when you get around to baselining it. So the start *value* and the start *date* can come from different moments, deliberately: baseline four weeks late and you have genuinely burned four weeks of the window.
- A `--type boolean` key-result has no pace, exactly like one with no `--due`. Its progress is 0 until done and 1 after, so pace would read `at-risk` for its whole life and then flip to `achieved` — noise, not signal.

## 6.3 Derived status

Never set by hand — `dropped` is the only settable key-result status.

| Status | Condition |
| --- | --- |
| `achieved` | `progress >= 1` |
| `missed` | past `due` with `progress < 1` |
| `at-risk` | `pace < at-risk-pace`, once `at-risk-after` days have passed since the start date |
| `on-track` | otherwise |

Status is a pure function of the current reading, so it does not latch: a key-result that hits its target and then regresses returns to `on-track`. For a "keep p99 under 200ms" target that is the only honest answer, and latching would mean storing an achievement that the numbers no longer support.

Without `--due` there is no pace and no deadline, so a key-result only ever reads `achieved` or `on-track`. A `--type boolean` key-result has no pace either but keeps its deadline, so it reads `achieved`, `missed`, or `on-track`. With `at-risk-after` empty there is no grace period, and pace is judged as soon as it is defined.

---

# 7. Reading

## 7.1 list

- `list` takes a container path positionally (see §3.2) and filters with `--tags`, `--match <text>`, `--status`, `--priority`, `--overdue`, `--direct`, and `--all`.
- `--status`, `--priority`, and `--overdue` apply only to nouns that have those fields. `--status` filters on effective status (§4.1).
- `--match` searches that noun's own text — name, summary, tags, and log bodies. On a skill, `description` and `body` stand in for `summary`; log bodies count there too.
- `--tags` takes a boolean expression over tags, with a bare comma list meaning `or` (§4.4).

## 7.2 Sorting and limiting

- `--sort <key>` on any `list`, ascending; `--reverse` flips it. Direction is never folded into the key.
- Keys on every noun: `id`, `name`, `created`, `updated`. Both timestamps are read from `entity.md` (§10.3).
- Keys where the field exists: `due`, `priority`, `status`. Key-results add `progress` and `pace`.
- `log list` defaults to `--sort at --reverse` — the one descending default, since a journal reads backward. Naming `--sort at` on its own drops the `--reverse` and gives oldest-first, because the rule above wins: a key never carries a direction, not even this one.
- Every other `list` defaults to `--sort id`, ascending.
- `--limit <n>` truncates, and the count line says so when it does: `showing 20 of 143`.

## 7.3 search

`search` is exactly the cross-noun form of `list --match`. Same engine, same flag vocabulary:

- `para search <query>` matches names, summaries, tags, and log bodies across every noun.
- It takes every `list` filter — `--tags`, `--status`, `--priority`, `--overdue`, `--direct`, `--all` — plus `--in <container-path>` to scope to a subtree, and it follows the same `--sort` and `--limit` rules.
- Results group by noun.

Text search is the half that grep could also do. The structured half is the half it cannot: effective status and terminal-hiding are computed, not written anywhere, so "areas tagged `rust` that aren't archived" is a question only para can answer.

## 7.4 review

`para review` surfaces everything wanting attention, for four reasons:

| Reason | Fires when |
| --- | --- |
| `--stale` | no resetting log entry within `log-sla` (§5.4) |
| `--overdue` | open and past `due` |
| `--at-risk` | a key-result whose pace has fallen below `at-risk-pace` |
| `--blocked` | `status` has been `blocked` for longer than `blocked-after` |

- Naming a reason restricts to it. Terminal-status items are excluded by default (§4.1), so a finished project stops nagging without your having to keep writing notes to it.
- `--blocked` is the one reason nothing can quiet but the fix. Notes and status churn reset the SLA clock, so a blocked project you keep annotating looks healthy to `--stale` forever; a blocker you have stopped questioning is the most expensive silence in the system.
- It takes an optional fully-qualified locator to scope to one subtree.
- Output is grouped by reason and ordered within each group by how far past the threshold an item is. It takes `--limit` but not `--sort`; the ordering is the point of the command.
- `review` always exits 0. Having work is not a failure — `doctor` is the command that gates.

## 7.5 path

- `para path <fully-qualified-locator>` prints the filesystem path as one bare line.

---

# 8. Emit

A tree nobody reads is a filing cabinet. `para emit` is what makes the tree reach an agent: it installs skills where each provider looks for them, and writes an `AGENTS.md` at every managed directory. Emitted output is the interface; the tree is the source.

## 8.1 Targets

- Targets are configured, not created: `para config set emit.<provider>.<field> <value>` (§9).
- `<provider>` comes from a fixed enum para ships a driver for. The on-disk shape of a provider's files is code, not configuration — that is what makes it testable, and what keeps a typo in config from producing subtly wrong output. A provider name para ships no driver for is a `doctor` finding.
- Fields per target: `enabled`, `path`, `scope`, and `tags` — a filter, taking the same boolean expression as `--tags` (§4.4), so a target can emit only the skills that are both `automation` and `public`.
- `scope` is `nested` or `root`. **Nested** emits into the directory the thing lives in, so position carries scope. **Root** emits everything at the tree root, where the driver has to express scope some other way — a glob per skill, or not at all. Not every provider can do both; §8.2 says which each accepts.
- `init` configures no targets. `para emit` with none enabled writes nothing and tells you how to add one.
- `para emit` takes an optional fully-qualified locator to emit one subtree, and `--dry-run`.

## 8.2 What each driver writes

Placement in the tree is a skill's scope, and each driver translates it differently — which is the whole reason scope policy is per-provider:

- **`claude-code`** — `nested` or `root`. Nested installs each skill once at its own position: a skill at `projects.my-project` becomes `projects/my-project/.claude/skills/<id>/SKILL.md`. Claude Code discovers directory-scoped skills and resolves most-specific-wins itself, so the driver hands it the tree's shape and lets it do the resolution. Root installs every skill into the tree root's `.claude/skills/`, which flattens scope — placement stops meaning anything, and the emitted body names the origin locator so an agent can at least see where the skill came from.
- **`agents-md`** — `nested` only. There is no root form, because `AGENTS.md` resolution is nearest-only: an agent reads the closest file and no other, so one file at the root would be read by nothing standing deeper. Each file instead carries the **union** of every active skill at or above its directory, and when two share an id the nearer one wins. Repetition in generated output is free; a project silently missing its bucket's skills is not.
- **`cursor`** — `root` only. Rule files express scope as globs rather than by position, so every skill lands in one directory at the tree root, each with a glob covering the subtree it came from. Scope survives; it just moves from the path into the frontmatter.

An `agents-md` file is emitted at **every entity directory, plus the three buckets, plus the tree root** — not only where a skill exists. Emitting only where there is something to say is broken under nearest-only resolution: a project with no file of its own falls back to `projects/AGENTS.md`, whose identity block describes the bucket and whose paths are wrong for the directory the agent is standing in. Unmanaged content directories then work for free, since the nearest managed ancestor is the right answer for them.

Each file contains the inlined skills plus a stable identity block: locator, name, summary, noun, and the names of its children. Nothing volatile goes in. Status, progress, pace, and what is overdue change without any file being touched, so the file points at `para show <locator>` and `para review <locator>` instead of carrying a snapshot that would be wrong by the afternoon.

Skills are **copied, not symlinked**. A symlink cannot carry its own generated-by marker without putting the marker in the tree's authoritative `SKILL.md`, where it would be a lie, and drivers like `cursor` need different frontmatter anyway. So §10.5's "nothing is duplicated" is a claim about *source*, not about output.

## 8.3 Ownership and pruning

Emit writes into directories that are not exclusively para's — a real repo's `.claude/skills/` holds hand-written skills, and `AGENTS.md` is a file people write prose in. So output declares itself:

- A file emitted whole carries a `generated-by: para` marker in its frontmatter.
- A region inside a shared file is delimited: `<!-- para:begin -->` … `<!-- para:end -->`.
- Emit rewrites its own regions, and prunes marked files it did not produce this run. Anything unmarked is untouchable.

That is what makes removal correct: delete a skill, or archive it, and the next `emit` un-installs it. Without pruning, a skill you deleted keeps firing.

The marker is deliberately in the output rather than in a manifest under `.para/`. A manifest would be a second copy of derived state, which §10.2 forbids, and losing it would leak output that nothing could ever find again. Self-describing files cannot desync.

## 8.4 Freshness

Emitted output is **committed**. The consumer is an agent that may be running anywhere, so a fresh clone has to be legible without a para binary installed, and that is worth the diff churn. Emit is deterministic — same tree, same bytes — so the churn is exactly the change you made.

Because it is committed, staleness is a real defect, and `doctor` reports it as one: rendering what emit *would* write and comparing is read-only, which keeps doctor read-only, and the finding prints `para emit` as the fix (§10.8).

Emit is explicit. It never runs as a side effect of a mutation, for two reasons: a skill's `SKILL.md` is hand-authored and authoritative (§10.5), so output can go stale with no para command in the loop and an explicit verb is needed regardless; and keeping mutations' blast radius inside the entity's own `.para/` is what lets `set` stay a small, safe, un-dry-runnable operation.

---

# 9. Configuration

Config lives at the tree root, in `.para/config.toml`. Keys are dotted, and `config list --prefix <text>` filters by prefix matched at whole dot-segments — so `--prefix project` matches `project.log-sla`, and `--prefix key` matches nothing rather than sloppily catching `key-result.*`.

Three families of key:

| Family | Example | What it does |
| --- | --- | --- |
| `<noun>.<field>` | `project.log-sla 14` | per-noun default for a field that takes one |
| `max-depth.<noun>` | `max-depth.area 3` | creation-time depth gate (§3.2) |
| `emit.<provider>.<field>` | `emit.claude-code.enabled true` | an emit target (§8.1) |

- Only `log-sla`, `blocked-after`, `at-risk-pace`, and `at-risk-after` take per-noun defaults. Every other field is per-entity or has a fixed default (§1.3).
- `config unset <key>` removes a default. Entities that were falling back to it fall back to empty, exactly as `unset` on the entity itself does.
- `config unset emit.<provider>` removes a whole target — the one place `unset` takes a namespace rather than a single key, because a target is several keys that only mean something together.
- A target springs into existence the first time you set a key on it, so there is no moment at which para can validate it as a whole. `doctor` covers that gap: a target naming an unknown provider, or missing a field its driver requires, is a finding.

---

# 10. Storage

Two rules generate the entire layout:

- A directory is an **entity** iff it contains `.para/entity.md`.
- The **tree root** is the directory containing `.para/config.toml` — that is what the walk-up search in §1.1 looks for.

## 10.1 Layout

```
brain/
├── .para/
│   └── config.toml                    ← root marker, defaults, emit targets
├── AGENTS.md                          ← emitted
├── my-root-dir/                       root.my-root-dir
│   ├── .para/{entity.md, log/}
│   ├── AGENTS.md                      ← emitted
│   └── my-dir-dir/.para/…             root.my-root-dir.my-dir-dir
├── projects/
│   ├── AGENTS.md                      ← emitted
│   └── my-project/                    projects.my-project
│       ├── .para/
│       │   ├── entity.md
│       │   └── log/
│       │       ├── 2026-01-01T081500.md        ← kind: create
│       │       ├── 2026-01-01T081502.md
│       │       └── 2026-01-01T081502-1.md      ← the suffix is part of the filename
│       ├── AGENTS.md                  ← emitted
│       ├── .claude/skills/…           ← emitted
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

## 10.2 What is deliberately never stored

**Derive what you cannot repair; store what you can.**

No file records its own `id`, `parent`, or locator. Two copies of a locator would be equally plausible, with nothing able to adjudicate between them, so there must only ever be one. The id is the directory name, the parent is where the directory sits, and the locator is computed from the path. Three consequences:

- `set <locator> id` and `set <locator> parent` are a single `rename(2)`. A project with three objectives and a dozen key-results moves in one operation, because they *are* subtrees and move as subtrees. No descendant file is rewritten.
- A locator cannot desync, because there is no second copy of it to drift from the first.
- Moving an entity by hand with `mv` to a **legal** position is simply correct — there is nothing for para to update. Drift is only possible into an *illegal* position, which is the narrow job §10.8 exists to catch.

Timestamps are the other category. They *are* stored (§10.3), because the log is an authoritative event stream that can always adjudicate a disagreement — so a divergent stored timestamp is detectable and recomputable, not an unresolvable ambiguity. Nothing clock-dependent is stored, and nothing ancestor-dependent: `pace`, a key-result's derived status, and effective status are all computed at read time, because a stored copy would rot with the passage of time or with a change to a different entity, and `rebuild` could not tell that it had.

## 10.3 entity.md

Frontmatter, with the summary as the body so it has room to breathe. Absent keys mean unset, and fall back to config where a default exists.

```markdown
---
noun: project
name: My Project
status: in-progress
priority: high
due: 2026-09-30
tags: [consumer, kafka]
log-sla: 14
created: 2026-01-01T081500-08:00
updated: 2026-03-02T144000-08:00
sla-reset-at: 2026-03-02T144000-08:00
status-changed-at: 2026-02-11T091200-08:00
---
Rebuild the consumer so it stops falling over under replay load.
```

The last four keys are a **projection** of the entity's own log, and a key-result carries two more — `current` and `measured-at`. The rule that bounds the set: *store exactly what would otherwise require opening a log file; compute everything else at read time.*

The log wins. `entity.md` is a materialized view of it, para refreshes the view on every mutation, and `para rebuild` recomputes it from the log at any time. So divergence — a hand-added entry, a `log remove`, a merge that went sideways — is a state doctor detects and one command repairs, rather than a silent lie.

Two things this buys, and one it costs:

- Every read is one file open. `review --stale`, `review --blocked`, and `--sort pace` used to mean finding and opening log entries per entity; now they are answered by the file the walk already reads for `name` and `status`.
- A human can read the state. Opening `entity.md` tells you when the thing was created, when it was last touched, when it last got real attention, and how long it has been blocked, without re-deriving any of it from filenames.
- Concurrency gets worse, and this is the honest cost. Log files still merge cleanly — different filenames, no conflict (§10.4). But two machines each adding a note now both rewrite these scalars, and that conflicts. The resolution is mechanical: take the later timestamp, then run `para rebuild`.

A key-result carries `type`, `start`, `target`, and `current` as strings, so `880/11000` keeps its denominator rather than collapsing to `0.08`.

## 10.4 Log entries

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

`kind: create` and `kind: note` drop `field`, `from`, and `to`. `kind: measurement` carries `value: "90/11000"` instead. Having all four kinds in one stream is what makes §3.3's promise real: `log list` interleaves creation, manual notes, field changes, and measurements because they are the same kind of file.

A `kind: change` entry written without `--note` simply has an empty body; the entry still exists, so the history stays complete either way. Whether an entry resets the SLA clock is decided by its `kind` and its `field` — never by whether it has a body (§3.3).

`offset` is the only part of the instant a filename cannot carry, so the recorded instant is filename + offset (§5.4). Ids are therefore not strictly monotonic across a DST shift or a flight, which used to matter and no longer does: the only thing that depended on lexical order being chronological was reading the newest timestamp off the greatest filename, and §10.3 stores the timestamps instead.

One file per entry buys four things at once:

- `log set <locator> id` is a rename, which is why §5.1 can say re-timing is *just* setting the id.
- The `-N` suffix lives in the filename, so "never renumbered" is enforced by the filesystem.
- Measurement uniqueness in time (§5.3) is filename uniqueness — enforced for free.
- Two machines appending logs never conflict in git; different filenames merge cleanly. This is the property `entity.md`'s projection does *not* have, which is why the log stays authoritative and the projection stays rebuildable.

## 10.5 Skills and adopted directories

A skill's `SKILL.md` stays authoritative for `name`, `description`, and `body`; the sibling `.para/entity.md` carries only `noun`, `status`, `tags`, and the projection. Nothing is duplicated in source — §8.2 explains why emitted output is a copy rather than a link. This is not a new decision: §1.3 already says `skill import` does not require `--name` or `--description` because "the flags are overrides." Storage just obeys it.

An imported `dir` gets a `.para/` written into it and nothing else. That is a real cost: para does modify the directory it adopts. The alternative — keeping all state in a mirror tree and linking back by path — makes the link between state and content unverifiable, since a fresh `git clone` resets every inode. A visible dotfile beats invisible drift.

`emit` raises that price on the same argument. An adopted `resources/kafka-notes/` will grow an `AGENTS.md`, and a subtree with skills will grow provider directories, so para writes more than one dotfile into content it did not create. The `tags` filter on a target and a skill's placement are how you keep that footprint where you want it.

## 10.6 The walk

Placement is always `--parent`, so an entity is always a *direct* child of its container. The walk therefore reads each entity's immediate subdirectories, descends only into those that hold `.para/entity.md`, and never recurses into adopted content. It is O(entities), not O(files).

Every read path is now one `entity.md` open per entity. Sorting by `updated`, `review --stale`, `review --blocked`, and `--sort pace` all read fields the walk already has in hand, so the log directory is never listed and never opened on a read. The only command that reads logs is `log list`, which is scoped to one entity, and `rebuild`.

An earlier draft of this design derived both timestamps instead of storing them, and proposed a `.para/cache/` keyed on the log directory's filename set to make `--stale` cheap. That is gone. The projection does the same job with a simpler invariant — one authoritative source, one materialized view, one command to rebuild it — and it has the property the cache never could: a human can read the answer.

The walk's one weakness is unchanged: `mv` an entity down into a plain content subdirectory by hand and it silently vanishes from every `list`, `search`, and `review` — no error, just absence. `para doctor` is the deep scan that finds it.

## 10.7 para rebuild

`para rebuild` recomputes every projection in `entity.md` from the entity's log. It writes nothing else, changes no content, and is idempotent.

- `para rebuild` — the whole tree
- `para rebuild <fully-qualified-locator>` — one subtree
- `para rebuild --dry-run` — print what would change

Run it after hand-editing a log, after resolving a merge conflict in an `entity.md`, or whenever `doctor` tells you to. It is the repair half of the bargain in §10.3: the projection is allowed to be stored precisely because this command exists.

## 10.8 para doctor

`para doctor` walks the tree ignoring the fast path's rules — every directory, including inside adopted content — and reports what it finds. It is **read-only**. There is no `--fix`: doctor prints the command that would fix each finding and leaves the running of it to you, which for the two mechanical findings means `para emit` or `para rebuild`.

- `para doctor` — the whole tree
- `para doctor <fully-qualified-locator>` — one subtree
- `para doctor --json`

Findings come in two severities, because "this `entity.md` is unparseable" and "you left a folder unregistered" are not the same news, and ranking them equally devalues the bucket.

**Errors** — a read would be wrong or incomplete. Exit status `1`.

| Kind | What it means |
| --- | --- |
| `orphan` | An `entity.md` the fast walk cannot reach — buried inside adopted content |
| `misplaced` | A legal entity in an illegal position: an objective outside a project, a key-result outside an objective, a dir or skill under an objective or key-result, a project below the top of `projects`, an area outside `areas` |
| `invalid` | `entity.md` unparseable, `noun` missing or unknown, a required field absent, an enum value not in range, a key-result `type` that does not match its `start`/`target` grammar |
| `log` | A filename that is not a legal timestamp id, unparseable frontmatter, a future timestamp, a missing or duplicated `create` entry, a `create` entry that is not the earliest, a `value` on a non-key-result, or two measurements at one instant |
| `collision` | A reserved name — `root`, `projects`, `areas`, `resources`, `.para` — used as an id |
| `projection` | `entity.md`'s stored timestamps or `current` disagree with the log → `para rebuild` |
| `stale` | Emitted output differs from what `emit` would write now → `para emit` |
| `target` | An `emit.<provider>` target names a provider para ships no driver for, omits a field its driver requires, names a `scope` the driver does not accept, or carries a malformed `tags` expression |

**Advisory** — your filing is loose. Exit status `2` when nothing worse is present.

| Kind | What it means |
| --- | --- |
| `untracked` | A directory with no `.para/entity.md` sitting **directly** in `projects/`, `areas/`, or `resources/` — the one place where only entities belong. Content inside a project is content, and is never reported |
| `depth` | Nesting past `max-depth.area` or `max-depth.dir` that arrived by `import` or by hand. A refusal at creation (§3.2) is where this binds; failing a whole tree on the day you adopt it is hostile at the wrong moment |

Exit status is `0` when clean, so CI can gate on `1` and ignore `2`, and an agent learns from the exit code alone whether judgment is required.

---

# 11. Command reference

## para init

```bash
para init
para init ./my-brain
para init --dry-run
```

## para emit

```bash
para emit
para emit --dry-run
para emit projects.my-project
para emit areas.my-area.sub-area
```

## para rebuild

```bash
para rebuild
para rebuild --dry-run
para rebuild projects.my-project
```

## para review

```bash
para review
para review --stale
para review --overdue
para review --at-risk
para review --blocked
para review --all
para review --json
para review --limit 20
para review areas.my-area
para review projects.my-project
```

## para search

```bash
para search "kafka"
para search --tags rust
para search --tags rust,reference
para search --tags "rust and reference"
para search --tags "kafka and not deprecated"
para search --tags "rust and reference or kafka and not deprecated"
para search "kafka" --in resources
para search "kafka" --in projects.my-project
para search "kafka" --status blocked
para search "kafka" --all
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
para project import ./projects/long-runner --id long-runner --name "Long Runner" --summary "This has been going since spring" --created 2026-03-14

para project list
para project list --all
para project list --status blocked
para project list --priority high
para project list --tags some-tag
para project list --tags some-tag,other-tag
para project list --tags "some-tag and not other-tag"
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
para project set my-project blocked-after 7 --note "I want to hear about this one fast"
para project unset my-project blocked-after
para project set my-project name "My Project!"
para project set my-project summary "This is a new summary of the exciting project" --note "the old one had drifted"
para project set my-project tags some-tag,other-tag,best-tag
para project unset my-project tags --note "the tags stopped earning their keep"
para project set my-project created 2026-01-04 --note "it really started the week before I wrote it down"
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
para objective set my-project.obj-1 status blocked --note "waiting on the platform team's migration"
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
para key-result add latency --name "p99 under 200ms" --parent my-project.obj-1 --summary "Tail latency at the edge" --type number --start 480 --target 200 --due 2026-09-30 --created 2026-07-01 --note "the clock starts at the top of the quarter, not today"
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
para key-result set my-project.obj-1.kr-1 at-risk-after 45 --note "a quarterly target needs a few weeks before pace means anything"
para key-result unset my-project.obj-1.kr-1 at-risk-after
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
para config set project.blocked-after 21
para config set objective.blocked-after 21
para config set key-result.at-risk-pace 0.8
para config set key-result.at-risk-after 30
para config set max-depth.area 3
para config set max-depth.dir 3

para config set emit.claude-code.enabled true
para config set emit.claude-code.scope nested
para config set emit.claude-code.path .claude/skills
para config set emit.agents-md.enabled true
para config set emit.agents-md.scope nested
para config set emit.cursor.enabled true
para config set emit.cursor.scope root
para config set emit.cursor.path .cursor/rules
para config set emit.cursor.tags automation
para config set emit.claude-code.tags "automation and not draft"

para config show project.log-sla
para config show emit.claude-code.scope
para config unset area.log-sla
para config unset emit.cursor
para config list
para config list --prefix project
para config list --prefix emit
para config list --prefix emit.claude-code
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
para skill list --status archived
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
para skill set projects.my-project.my-cool-project-skill status archived --note "it misfired twice, parking it while I rewrite the body"
para skill set projects.my-project.my-cool-project-skill status active --note "the rewrite holds up"
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
para dir list --status archived
para dir list --all
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
para dir set resources.my-resources-dir status archived --note "superseded by the 2026 edition, keeping it findable"
para dir set resources.my-resources-dir id rust-reference
para dir set root.my-root-dir parent resources --note "it belongs in resources after all" --dry-run
para dir set resources.kafka-notes parent resources.rust-reference
para dir unset resources.rust-reference parent --note "back out to the root bucket"

para dir remove root.my-root-dir --dry-run
para dir remove root.my-root-dir
para dir remove root.rust-reference --force
para dir remove resources.kafka-notes --keep-files --dry-run
para dir remove resources.kafka-notes --keep-files
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

# 12. Design notes

Why the non-obvious calls went the way they did, recorded so they don't get re-litigated.

- **Emitted output is the interface; the tree is the source.** A `SKILL.md` sitting at `projects/my-project/my-skill/` is discovered by nothing — providers look in their own directories, and `AGENTS.md` resolution is nearest-only. Without `emit`, a skill's `description` is called an activation trigger while nothing can activate it, and placement is decoration. Emit is what makes both real.
- **Emit is explicit, and its output is committed.** Explicit because `SKILL.md` is hand-authored (§10.5), so output can rot with no para command in the loop and a verb is needed regardless. Committed because the reader is an agent that may be running in a fresh clone with no binary installed. The price is diff churn, paid down by emit being deterministic.
- **Output declares itself rather than being tracked in a manifest.** A manifest of emitted paths is a second copy of derived state, and losing it would orphan output forever. Markers and delimited regions cannot desync, and they make it safe to emit into directories that also hold things you wrote by hand.
- **`entity.md` stores a projection of the log.** An earlier draft derived every timestamp on read and refused to store any, on the grounds that concurrent appends to a log directory merge cleanly while scalar fields conflict. That cost is real and is kept in §10.3 rather than hidden. It loses to two things: every review reason becomes one file open instead of a log walk, and a human can read the state without re-deriving it from filenames. The invariant that makes it safe is that the log stays authoritative, so divergence is detectable by `doctor` and repairable by `rebuild` — which is the difference between a projection and a duplicate.
- **Derive what you cannot repair; store what you can.** `id`, `parent`, and `locator` are never stored because two copies would be equally plausible with nothing able to adjudicate. Timestamps are storable because the log adjudicates. Anything clock-dependent (`pace`, derived status) or ancestor-dependent (effective status) is never stored, because those go wrong with no event having occurred and `rebuild` could not tell.
- **Every noun has a log; only four have an SLA.** A `--note` on a dir or skill mutation has to land somewhere readable, and para mutations are not git commits, so git would not capture the reasoning. The asymmetry belongs on `log-sla` instead, where it has a reason of its own: attention, not history.
- **Projects are flat.** Objectives are already the decomposition mechanism. A sub-project would compete with them for the same job, and `projects.big-thing.phase-1` is better said as an objective.
- **Terminal status quiets every browsing read, and cascades.** Otherwise a project marked `done` in January keeps appearing in `review --stale` forever, quietable only by writing log entries to a finished project — and its objectives keep nagging with no visible parent to explain why. Cascading by derivation rather than by writing down the subtree is what makes restoring an area give back the exact previous picture.
- **`missed` is not terminal.** A blown deadline is the one thing that should not be hideable. It stays in `list` and in `review` until you drop it, re-target it, or move the date, because each of those is a decision and none of them is forgetting.
- **A dir and a skill have a status.** Superseded reference material needs to be put aside while staying searchable, which `remove --keep-files` cannot do — untracked content is invisible to para. And a skill needs an off switch that is not deletion, which matters much more once emit exists.
- **`low` priority buys no silence.** The predecessor design suppressed staleness findings on low-priority items. Per-entity `log-sla` does that job better: it says *how* quiet rather than just quiet, it composes cleanly with the config default, and it keeps one intent to one mechanism. Two knobs would also make `unset log-sla` ambiguous, since priority would keep modifying the result.
- **`blocked` gets a required note and its own review reason.** Status changes reset the SLA clock, so blocking is self-silencing and each subsequent note buys another window — a project can sit blocked for a year looking healthy. `--blocked` keys on time-in-status instead, which nothing but unblocking resets. The required `--note` is the one exception in §3.3 and exists because a blocker with no recorded reason is worthless in six months.
- **Setting a field to its current value does nothing.** Without this rule, `set status in-progress` on something already in-progress writes a from-X-to-X entry and resets the SLA clock, so a shell loop buys permanent silence from `review` — the same hole §3.3 rejects the `--note`-keyed design for.
- **`at-risk-after` is its own knob, not a reuse of `log-sla`.** The grace period before pace is judged and the cadence at which you check in are unrelated. Keying one to the other means tightening your cadence retimes every at-risk judgement as a side effect.
- **`created` is mutable, bounded by its own log.** It has to move: an imported directory alive for years would otherwise date from the day you adopted it, and §6.2 needs a key-result's clock to start at the commitment rather than the baselining. The single constraint — the create entry stays earliest — is what makes that safe, and it was chosen over "earlier only" and over splitting the field into an immutable `created` plus a mutable start date. Both alternatives cost a concept; this costs a comparison. It leaves one narrow consequence, accepted knowingly: on an entity nobody has touched, the bound is *now*, so `created` can be moved forward and will carry the staleness and pace clocks with it. That is indistinguishable from re-committing to an untouched thing today, and it is only ever visible to you.
- **A key-result's start date is its `created`.** Deriving it from the first measurement is tidier — the start becomes a single (value, date) point — but it means an untouched key-result has no pace and so can never go `at-risk`. An abandoned key-result is exactly what `review` exists to surface, so the clock has to start at commitment rather than at first reading.
- **Boolean key-results skip pace.** Their progress is 0 until done, so pace would report `at-risk` for their entire life and then flip straight to `achieved`.
- **Derived status does not latch.** A key-result that hits its target and then regresses reads `on-track` again. For a "hold p99 under 200ms" target that is the only true answer, and latching would require storing an achievement the numbers no longer support.
- **`import` is only on the four nouns that can predate para.** See §1.2.
- **Key-results have no priority.** Inheritance from the objective is a fact about how to read them, not a field. Making it a real sort key would mean deriving through a parent, which no other derived field does.
- **Notes, measurements, and status changes reset the SLA; no other field change does.** Everything is still logged, so the history is complete either way — but attention is measured only by entries that report on the world. Keying on the presence of `--note` instead was the obvious alternative and is worse in two specific ways: it lets tuning `log-sla` reset the very clock being tuned, and it lets pushing a deadline out buy quiet from `review`.
- **No mechanism for one skill across scattered entities.** The predecessor had a hook library with `use`/`unuse`, which a skill-as-entity cannot express: three-of-ten projects has no home but `root` or three drifting copies. Both candidate fixes lose. An explicit `applies-to` list of locators is a stored second copy of a locator, which §10.2 forbids and which every rename would silently break. A tag selector avoids that but makes a skill's real scope invisible from the tree and turns a tag typo into silent non-emission. The reason it is tolerable to do nothing: a skill's `description` is the actual activation gate, so placement is only a coarse filter. A skill at `root` does not fire everywhere — it fires when its description matches.
- **`--tags` takes a boolean expression, with word operators and no parentheses.** Symbols were the obvious choice and are wrong here: `&` and `|` are shell metacharacters, and forgetting to quote `--tags "a & b"` does not fail — it backgrounds the command and returns a confident wrong answer. Word operators turn the same mistake into a stray positional argument, which every command taking `--tags` rejects. Parentheses are absent because DNF is universal: `a and (b or c)` is `a and b or a and c`, so grouping buys no expressiveness and would cost the only characters that force a real parser. A bare comma list still means `or`, which keeps the common case short and every existing example valid. If filters keep accreting, the direction this grows is one `--where` over all fields with the same vocabulary, not more operators on more flags.
- **Depth is refused at creation and only advised afterward.** You cannot `mkdir` your way into an entity, so the creation gate catches nearly everything, and it binds while you are still making the decision. The two populations it misses — imports and hand edits — are exactly the ones where failing hard would make `doctor` red on the first day of adopting a real repo, which is hostile at the wrong moment.
- **`untracked` is only reported directly inside a bucket.** Under filesystem-as-truth, an unregistered folder inside a project is content, and reporting it would mean nagging about your own notes. Directly inside a bucket is different: only entities belong there, so a plain directory is a `mkdir` you forgot to register, and it is invisible to every read command forever.
- **`doctor` has two severities and no `--fix`.** Ranking "this `entity.md` is unparseable" with "you left a folder unregistered" devalues the bucket, and the exit-code split lets CI gate on one and an agent learn from the code alone whether judgment is needed. No `--fix`, because the findings that need no human choice already have exact commands to print — `para emit` and `para rebuild` — and doctor staying read-only is worth more than saving a keystroke.
- **`review` exits 0 even when it finds things.** Having work is the normal state, and a command that fails whenever you have work is a command people stop running. `doctor` is where the gate belongs.
- **`search` stays, as the cross-noun form of `list --match`.** The predecessor dropped it on the grounds that agents read files with their own tools, which is right about the text half. It is wrong about the structured half: effective status and terminal-hiding are computed and written nowhere, so grep cannot ask those questions at any price.
- **`remove` deletes the subtree, and `--keep-files` is the escape hatch.** The design already has non-destructive ways to put something aside — `dropped`, `archived` — so `remove` is free to mean what it says. `--keep-files` exists because para's whole footprint inside an entity is its `.para/` directory, which makes "stop tracking this but leave my notes" a clean operation rather than a special case.
- **`--sort at` is ascending like every other key.** Folding a direction into one key would cost more than the redundancy of spelling the `log list` default as `--sort at --reverse`.
- **Local wall-clock ids, with the offset in frontmatter.** Ids are therefore not monotonic across a DST shift or a flight. That used to be a real cost, because reading the newest timestamp meant taking the lexically greatest filename; storing the projection (§10.3) retired that trick, and with it the objection. Filenames that match the clock you were looking at when you wrote the entry are worth more than an ordering property nothing depends on.
