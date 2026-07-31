# Conventions

Tree

- `para init` creates the four buckets in the current directory.
- Every other command finds its tree by walking up from the working directory, the way git finds `.git`.
- `$PARA_HOME` overrides that search when set.

Listing

- `list` takes a container path positionally (see Containers) and filters with
  `--tags`, `--match <text>`, `--status`, `--priority`, `--overdue`, `--direct`, and `--all`.
  `--status`, `--priority`, and `--overdue` apply only to nouns that have those fields.
- `--match` searches that noun's own text — name, summary, tags, and log bodies.
  `para search` runs the same query across every noun at once.
- The noun is always the command word, never the value of a flag. There is no `--noun`.
- See Sorting and limiting for ordering, and Search for the cross-noun form.

Sorting and limiting

- `--sort <key>` on any `list`, ascending; `--reverse` flips it. Direction is never folded into the key.
- Keys on every noun: `id`, `name`, `created`, `updated`. `updated` reads the newest log timestamp.
- Keys where the field exists: `due`, `priority`, `status`. Key-results add `progress` and `pace`.
- Log entries sort by `at` and default to newest-first — the one descending default, since a journal reads backward.
  Everything else defaults to `id`.
- `--limit <n>` truncates, and the count line says so when it does: `showing 20 of 143`.
- `review` is grouped by reason and ordered within each group by how far past the threshold an item is.
  It takes `--limit` but not `--sort`; the ordering is the point of the command.

Search

- `para search <query>` matches names, summaries, tags, and log bodies across every noun.
- `--tags` searches tags alone. `--in <container-path>` scopes to a subtree.
- Results group by noun and then follow the same `--sort` and `--limit` rules as `list`.

Output

- `--json` on every read command: `list`, `show`, `review`, `search`, `log list`, `log show`, `config list`, `config show`.
- `path` prints one bare line, already the right shape for `$(...)`.

Required flags

- `add` requires `--name` and `--summary` on every noun — the name so compiled headings read well,
  the summary as a nudge to record context while it is still in your head.
- `skill add` requires `--name`, `--description`, and `--body`. The description is the activation trigger
  and the body is the skill itself; without either the skill is inert.
- `key-result add` also requires `--type` and `--target`. No target means no progress (see Key results).
- `objective add` and `key-result add` also require `--parent` (see Containers).
- `dir import` requires `--name` and `--summary`, since a plain directory carries no metadata of its own.
  `skill import` does not: a skill directory has its own frontmatter, and the flags are overrides.
- `--id` is optional on `import`, defaulting to a slugified basename of the imported path.

Locators

- A locator is the full path to an existing thing — never a bare id when the thing is nested.
- Nouns that live in exactly one bucket (project, area, objective, key-result) carry no bucket prefix: `my-area.sub-area`, `my-project.obj-1.kr-1`
- Nouns that can live in several buckets (dir, skill) are prefixed: `root.`, `projects.`, `areas.`, `resources.`
- Every locator also has a fully-qualified form, prefixed with its bucket: `projects.my-project`, `areas.my-area.sub-area`, `projects.my-project.obj-1.kr-1`.
  The noun-agnostic commands `path` and `review` take only that form, since `my-project` alone cannot say
  whether it means a project or a dir. `search` takes a query rather than a locator, though `--in` takes one, and its output prints them.
  All output prints the fully-qualified form, so any locator you read can be pasted into any command.
- Noun commands accept either form. The bucket prefix is redundant there but never wrong.
- Dots separate path segments; hyphens separate words within a segment.
- `root`, `projects`, `areas`, and `resources` are reserved and cannot be used as an id.
- New ids — passed to `add`, to `import --id`, and to `set <locator> id` — are bare single segments. Placement never comes from the id.

Containers

- Placement is always `--parent <container-path>`, on both `add` and `import`. There are no `--root` / `--projects` / `--project` / `--area` / `--dir` / `--objective` scope flags.
- Omitting `--parent` places the thing at the top of its bucket: top-level for project and area, `root` for dir and skill.
- `--parent` is required for objective and key-result. An objective belongs to a project and a key-result to an
  objective; neither has a top-of-bucket to fall back to, so `unset <locator> parent` is an error on both.
- Re-parenting later is `set <locator> parent <container-path>`, the same word as at creation.
- `unset <locator> parent` promotes to the top of the bucket — top-level for a single-bucket noun, `root` for dir and skill.
- `list` takes a container path positionally: `para skill list projects.my-project`. Symmetric with `show`, which takes the thing's own path.
- A container path matches at any depth. `--direct` restricts to immediate containment.

Status

- project, objective: `planned | in-progress | blocked | done | dropped` — default `planned`, terminal `done` and `dropped`
- area: `active | archived` — default `active`, terminal `archived`
- key-result: derived from its measurements — see Key results below. Terminal is `achieved` and `dropped` only; `missed` stays visible, because a blown deadline is the last thing that should be hidden.
- `list` hides terminal-status items by default; `--all` shows everything, `--status <status>` filters.
- Archiving never moves anything. Status is the only record of it, and locators are stable for life.

Dates

- `--due <YYYY-MM-DD>` is optional on project, objective, and key-result.
- `list --overdue` shows open things past their due date.
- `--due` is date-only by design: a deadline is a day, while a log entry is a moment. See Logs.

Key results

- `--type number | ratio | boolean`, fixed at `add` and never settable afterward — changing it would invalidate every measurement already logged. Delete and recreate instead.
- One measurement flag, `--value`, whose literal grammar follows the type: `42`, `90/11000`, `true`.
  `--start` and `--target` take the same grammar, so a ratio's baseline keeps its reading rather than collapsing to a decimal.
- A ratio's denominator belongs to each reading, not to the key-result — it legitimately varies between measurements.
- `--target` is required. `--start` is optional and defaults to the first logged measurement.
- `progress = (current − start) / (target − start)`. Direction falls out of the arithmetic; there is no up/down flag.
- `pace = progress / elapsed`, where `elapsed = (today − start-date) / (due − start-date)`.
- Derived state, never set by hand — `dropped` is the only settable key-result status:
  - `achieved` — `progress >= 1`
  - `missed` — past `due` with `progress < 1`
  - `at-risk` — `pace < at-risk-pace`, but only once one `log-sla` window has elapsed since the start date
  - `on-track` — otherwise
- Without `--due` there is no pace, so a key-result only ever reads `achieved` or `in-progress`.
- `at-risk-pace` defaults from config and overrides per entity, exactly like `log-sla`.

Priority

- `high | medium | low`, default `medium`. On project, area, and objective only.
- A key-result inherits the importance of its objective; dirs and skills have none.

Tags

- Every noun is taggable.

SLA

- `log-sla` is an integer number of days since the last log entry. No unit suffix.
- Set per entity, defaulted per noun in config: `para config set <noun>.log-sla <days>`
- `unset` removes the explicit value: the field falls back to its config default if it has one, otherwise to empty.

Notes

- `--note` is optional on every mutation — `add`, `import`, `set`, `unset` — and writes a log entry.
  So `log list` shows manual notes and field changes interleaved, and a field change resets the SLA clock.
- `remove` takes no `--note`; the entity it would annotate is gone.

Logs

- A log entry's id *is* its timestamp: `2026-01-01T081502` — hyphens in the date, `T`, colon-free time.
  One path segment, filename-safe on every platform, and lexically sortable.
- Re-timing an entry is therefore just `log set <locator> id <timestamp>`. No separate date or ordinal field.
- `--at` accepts progressive precision and fills the rest with zeros:
  `2026-01-01`, `2026-01-01T08`, `2026-01-01T0815`, `2026-01-01T081502`. Omit it entirely and the entry lands at now.
- Several entries may share a timestamp. The second and later carry a stable `-N` suffix assigned at
  creation — `2026-01-01T081502-1` — never renumbered, never shifted by an insert or a removal.
  The suffix is a tiebreaker, not a rank: to order two entries, give them different times.
  Suffixes are assigned as max-existing + 1, so one can be reused after a deletion.
- Key-result measurements are the exception: they must be unique in time. A measurement is a reading of a
  single quantity, so two values at one instant is a contradiction and would make "current value" arbitrary.
  A duplicate timestamp is an error — supply a time, or edit the existing reading.
- Ordering is derived from the timestamp, never asserted. Same-second entries order by suffix, which is creation order.
- Timestamps are local wall-clock time; the entry records the UTC offset in force when it was written.
  `log list` sorts by the recorded instant, so ids are not strictly monotonic across travel or a DST shift.
- Backdating is allowed and is the point of `--at`. Future timestamps are rejected — that is what `--due` is for.
- The SLA clock reads the newest log timestamp and reports whole days. A backdated entry never resets it;
  a field change from `--note` lands at now, so it does.

Removal and dry run

- `--dry-run` on anything that moves files: `remove`, `import`, `set <locator> id`, `set <locator> parent`.
- Plain field changes don't need it.
- `remove` confirms interactively before deleting anything. `--force` skips the prompt for scripts.

## para init

para init
para init ./my-brain
para init --dry-run

## para review

para review
para review --stale
para review --overdue
para review --at-risk
para review --json
para review --limit 20
para review areas.my-area
para review projects.my-project

## para search

para search "kafka"
para search --tags rust
para search "kafka" --in resources
para search "kafka" --in projects.my-project
para search --tags rust --in resources
para search "kafka" --sort updated --reverse
para search "kafka" --limit 10
para search "kafka" --json

## para path

para path projects.my-project
para path areas.my-area.sub-area
para path resources.rust-reference

## para project add|remove|show|list|set|unset|import|log

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

### para project log add|set|remove|show|list

para project log add my-project --note "made some progress"
para project log add my-project --at 2026-01-01 --note "backdating some progress"
para project log add my-project --at 2026-01-01T0815 --note "backdating more precisely, so the order is unambiguous"
para project log add my-project --at 2026-01-01 --note "a second entry that lands at the same second, so it gets a -1 suffix"
para project log list my-project
para project log list my-project --limit 20
para project log list my-project --sort at --reverse
para project log show my-project.2026-01-01T000000
para project log show my-project.2026-01-01T000000-1
para project log set my-project.2026-01-01T000000 note "a better description of what happened"
para project log set my-project.2026-01-01T000000 id 2026-01-02T0900 --note "off by a day"
para project log remove my-project.2026-01-01T000000-1

## para area add|remove|show|list|set|unset|import|log

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

### para area log add|set|remove|show|list

para area log add my-area.sub-area --note "this is still a relevant area for me"
para area log add my-area.sub-area --at 2026-01-01 --note "this is a backdated review"
para area log list my-area.sub-area
para area log show my-area.sub-area.2026-01-01T000000
para area log set my-area.sub-area.2026-01-01T000000 note "a sharper account of the review"
para area log remove my-area.sub-area.2026-01-01T000000

## para objective add|remove|show|list|set|unset|log

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

### para objective log add|set|remove|show|list

para objective log add my-project.obj-1 --note "made progress toward the objective"
para objective log add my-project.obj-1 --at 2026-01-01 --note "backdating an update"
para objective log list my-project.obj-1
para objective log show my-project.obj-1.2026-01-01T000000
para objective log set my-project.obj-1.2026-01-01T000000 note "a better account of the progress"
para objective log remove my-project.obj-1.2026-01-01T000000

## para key-result add|remove|show|list|set|unset|log

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

### para key-result log add|set|remove|show|list

para key-result log add my-project.obj-1.kr-1 --value 90/11000 --note "some observation"
para key-result log add my-project.obj-1.kr-1 --value 10/10000 --at 2026-01-01 --note "some reason for why it was backdated, and maybe an observation"
para key-result log add my-project.obj-1.kr-1 --value 12/10000 --at 2026-01-01T1430 --note "a second reading that day needs its own time, since measurements must be unique"
para key-result log add my-project.obj-1.signups --value 4200 --note "halfway there"
para key-result log add my-project.obj-1.shipped --value true --note "it's out"
para key-result log list my-project.obj-1.kr-1
para key-result log show my-project.obj-1.kr-1.2026-01-01T000000
para key-result log set my-project.obj-1.kr-1.2026-01-01T000000 value 10/10001 --note "goofed the reading"
para key-result log remove my-project.obj-1.kr-1.2026-01-01T000000

## para config set|unset|list|show

para config set project.log-sla 14
para config set area.log-sla 90
para config set objective.log-sla 90
para config set key-result.log-sla 30
para config set key-result.at-risk-pace 0.8
para config show project.log-sla
para config unset area.log-sla
para config list
para config list --prefix project

## para skill add|remove|show|list|set|unset|import

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

## para dir add|remove|show|list|set|unset|import

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
