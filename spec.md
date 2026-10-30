# para — specification

Status: **outline.** One sentence per section: what it holds, and why it is here.

## 0. About para

### 0.1 What para is

`para` is a command-line tool that manages the structure and metadata of a PARA file tree: a directory with four buckets (projects, areas, resources and archive), organized by [the PARA method](#02-the-para-method).

The **structure** is which entities (the projects, areas, dirs and other things `para` manages) exist, what kind each one is and where it sits. The **metadata** is the facts `para` records about entities, such as ids, status, history and measurements. Both are meant to change only through `para`'s commands ([structure is para's, content is yours](#03-structure-is-paras-content-is-yours)). Each command is deterministic: for the same tree, arguments and current time, it gives the same output and makes the same change, apart from any new ids it generates. [Safety guarantees](#81-safety-guarantees) states what `para` promises when a command fails or is interrupted.

Everything `para` doesn't own is content, which people, AI agents and scripts edit freely inside a structure they can rely on ([structure is para's, content is yours](#03-structure-is-paras-content-is-yours)).

### 0.2 The PARA method

PARA, created by Tiago Forte, organizes information by how actionable it is rather than by subject. His [The PARA Method](https://fortelabs.com/blog/para/) is the authoritative description. Everything goes into one of four categories:

- **Projects**: efforts taken on with a goal in mind, which end when the goal is met or dropped, such as launching a website.
- **Areas**: parts of work and life that need ongoing attention and never finish, such as health, finances or filing taxes each year.
- **Resources**: topics you are interested in or learning about, such as a programming language.
- **Archive** (Forte's "Archives"): anything from the other three that is finished or inactive but worth keeping, such as a completed project.

Following the method's focus on actionability, something that fits more than one category goes in the most actionable one (projects first, then areas, then resources), so material gathered for a project goes with that project.

In `para`, each category is a top-level [bucket](#12-buckets) of the tree.

### 0.3 Structure is para's, content is yours

The tree's structure and metadata belong to `para`. They are:

- every entity, which is every folder `para` manages (the folder itself, not what you put in it): the root, the containers (the buckets, archive buckets and ones such as `objectives/`), archive stubs, and every project, area, dir and entity of another kind. Each has a `.para/` folder, and together they form an unbroken chain from the root ([kinds and containment](#22-kinds-and-containment));
- everything inside each `.para/`: state (including each recorded file's provenance), config (including sensitivity rules), the journal and the files `para` generates, such as `ACTIVITY.md` ([storage](#3-storage)). Anything else put inside a `.para/` is `para`'s too;
- the keys `para` writes in README frontmatter, its marked blocks in READMEs and `AGENTS.md` files, and, until `para migrate` removes them, the blocks an earlier version wrote in `CLAUDE.md` ([para-owned blocks](#52-para-owned-blocks)).

Change these only through `para`'s commands, including creating, moving, renaming, archiving and removing entities. The commands are designed to be the easiest way for a person or an agent to make these changes. `para` validates each change before writing anything and refuses an invalid one.

Everything else is content: notes, documents, data, other folders, and your own text and frontmatter keys in READMEs and `AGENTS.md`. Anyone may create and edit content. `para` never edits or deletes it, even when removing an entity, and moves it only along with the entity that holds it. Where a README or `AGENTS.md` already exists, `para` adds its part and keeps yours, and adopting a folder adds a `.para/` and keeps everything already there ([adopting existing directories](#24-adopting-existing-directories)). In every other case, if content already sits where `para` needs to create a folder or move an entity, `para` refuses rather than overwrite or merge it.

Changing `para`'s side by hand, such as moving an entity or editing a file in `.para/`, is possible but unsupported, and text written inside `para`'s files or blocks is overwritten the next time `para` rewrites them. `para`'s [safety guarantees](#81-safety-guarantees) cover only changes made through its commands, and [`doctor`](#82-doctor-findings) reports hand changes that leave the tree inconsistent, including a `.para/` that `para` did not create.

Every command depends on this split: it writes only `para`'s side.

### 0.4 Why deterministic structure helps agents

Agents, like people, are drawn to a clear, predictable framework that leaves them free to create within it. The framework keeps their attention on the task, and when following its rules is also the easiest way to work, they follow them. An agent often arrives in a tree with no memory of it and shares the tree with people, scripts and other agents, so it needs rules it can learn quickly and trust everyone else to follow, and `para` provides them.

`para`'s rules for the structure (the buckets, the kinds and which kind may sit where) are small and the same in every tree, so an agent learns them once. The tree explains itself on disk: `AGENTS.md` at the root, in each bucket and in each archive bucket describes the rules ([`AGENTS.md`](#56-agentsmd)), and each entity's README shows its main facts in frontmatter ([README frontmatter](#53-readme-frontmatter)), so ordinary file tools are enough to find any entity and read about it.

The commands make the right path the easy one. Every change to the structure is made with a command, so the agent never needs to edit `para`'s side or know how `.para/` is laid out. A command refuses an invalid change before writing anything and says why and what to do instead, and [safety guarantees](#81-safety-guarantees) covers one that fails partway. Commands are [deterministic](#01-what-para-is), so an agent can rely on how each one behaves and confirm its result from the JSON output and exit codes ([output and exit codes](#710-output-and-exit-codes)). Each entity's journal shows the next agent or person what happened to it ([notes and the journal](#18-notes-and-the-journal)).

Within that framework, agents are free to create, edit and organize content with any tool ([structure is para's, content is yours](#03-structure-is-paras-content-is-yours)).

### 0.5 Scope and non-goals

These are outside `para`'s scope, and a feature that falls under one is declined for the reason given.

- **Skills and tool-specific files**: `para` does not distribute skills (instructions packaged for an agent) or generate files for one agent tool, such as Cursor rules or `CLAUDE.md`. Distributing skills is its own problem, and tool-specific files differ from tool to tool. Other tools can layer both on a tree, and layering skills on top is the intended way to customize how agents work with it. `para` writes only its own block in `AGENTS.md`, a file many agent tools read and that you may also use for your own instructions ([`AGENTS.md`](#56-agentsmd)). [`para migrate`](#91-migrating-a-v0-tree) removes the `CLAUDE.md` blocks an earlier version wrote.
- **Version control**: `para` never runs git or requires a repository, so it works the same in any tree and never commits for you. [Git](#37-git) covers how its files behave in a repository.
- **Interpreting content**: `para` never judges a file by what it says, and calls no AI model. A model's answer would make commands nondeterministic, so a file's provenance comes from what a person or agent records, its label from the tree's rules, and its trust is computed from those records, the grades a person sets and their vouches.
- **Reaching outside the tree**: `para` never fetches or sends anything over a network, which keeps every command [deterministic](#01-what-para-is). A [link](#110-links) points to something outside the tree and is flagged when it is due to be checked or updated, and outside values enter only as [recorded measurements](#42-recording-measurements).
- **Running on its own**: `para` has no background process, schedule or notifications, so nothing changes the tree unless someone runs a command. [`review`](#77-review) is how to ask what needs attention.
- **Access control**: `para` does not encrypt files or set file permissions. A [sensitivity label](#112-sensitivity-labels) records how far content may travel, but `para` cannot stop a readable file being copied. [What para does with labels](#65-what-para-does-with-labels) covers what it does instead.

### 0.6 Principles

Every design decision in this spec follows these rules. A new question is answered by applying them, and a change that would break one needs a recorded decision ([decision index](#appendix-d-decision-index)).

1. **Structure is `para`'s, content is yours**: `para` writes only its own side of the tree and never edits or deletes content ([ownership](#03-structure-is-paras-content-is-yours)).
2. **Deterministic**: the same tree, arguments and time give the same result, apart from new ids ([what para is](#01-what-para-is)).
3. **One home per fact**: each fact is stored as truth in exactly one place ([truth and projection](#31-truth-and-projection)), and each event is recorded once, in the journal of the folder it happened to, where a child arriving or leaving is its parent's event. A value that can be computed, such as a key result's status, is never stored as truth ([what is never stored](#57-what-is-never-stored)).
4. **Generated files serve readers**: the files `para` writes for readers who never run it show facts and the values computed from them that only a command can change; they are updated with every change, can be rebuilt from the stored truth at any time and are never read back as truth ([generated files](#117-generated-files)).
5. **Ids are identity**: every entity is addressed by a stored id that survives moves and renames and is never reused. Its path only says where it is now ([addressing](#23-addressing)).
6. **The right path is the easy one**: every change to the structure is a command, and a command that cannot make a change validly and safely refuses before writing anything, saying why and what to do instead. It never guesses ([why deterministic structure helps agents](#04-why-deterministic-structure-helps-agents)).
7. **Loosening and format changes are explicit**: a sensitivity label only tightens down the tree, and trust only falls as content is derived from other content. Only an explicit command lowers a label or raises trust, asking for confirmation unless forced, and never a label below its parent's ([how labels flow](#64-how-labels-flow)). A tree's format changes only through `para migrate` ([schema version](#93-schema-version)).
8. **Small and one way**: a feature is added only when managing a PARA tree needs it ([scope and non-goals](#05-scope-and-non-goals)), and each concept has one name, each kind of change one command and each value one format ([glossary](#appendix-c-glossary)).

### 0.7 How to read this spec

[Chapter 1](#1-components) tours the tool, with one short section for each component. Chapters 2 to 9 give the detail, and the appendices hold the [example tree](#appendix-a-example-tree), the [worked examples](#appendix-b-worked-examples), the [glossary](#appendix-c-glossary) and the [decision index](#appendix-d-decision-index).

Every statement about what `para` does is a requirement, written in the present tense: "`para` refuses" means an implementation must refuse, "never" means must not, and "may" means allowed but not required. A reason given with a rule explains it and adds no requirement, and neither does the decision index, which summarizes each decision and links its record. Each term means what the glossary says. Chapter 0 and the tour summarize the detail chapters, and examples illustrate them; where any of these disagrees with a detail chapter, the detail chapter is right.

Every example starts from the tree in Appendix A unless it names another starting point, such as an empty directory, a v0 tree or the example tree with a stated change. The worked examples in Appendix B are the acceptance tests: each states its starting point, command and any input it reads, and a test checks its output, exit code and every file's path and contents exactly. Appendix B says how tests fix the clock, new ids and the terminal.

## 1. Components

### 1.1 The tree

A tree is a root directory and everything in it. The root's `.para/state.toml` has `kind = "root"`, which marks the tree, and records the tree's schema version and the prefix of its ids ([`state.toml`](#32-statetoml)). [`init`](#73-init) creates a tree, and every command that works on a tree finds it by checking the current directory and then each parent for a `.para/state.toml` with `kind = "root"` ([the walk](#36-the-walk)). A v0 tree is marked by `tree.toml` instead, and only [`migrate`](#91-migrating-a-v0-tree) works on it.

Under the root are the four [buckets](#12-buckets), and `archive/` holds archive buckets that mirror the other three ([archive](#17-archive)). Every folder `para` manages is an entity, a directory it tracks by id: the root, the buckets and archive buckets, the projects, areas, dirs and other kinds within them, and the archive's [stubs](#17-archive) ([kinds and containment](#22-kinds-and-containment)). A container is an entity with a fixed name that groups children: each bucket is one, created by `init`, and so is each archive bucket and a project's `objectives/`, created when its first child arrives. Every entity has a `.para/` whose `state.toml` records its kind, and each except the root sits directly inside another.

```text
brain/                     the root
├── .para/state.toml       holds kind = "root"
├── AGENTS.md
├── README.md
├── projects/              a bucket
│   ├── .para/
│   └── launch-website/    a project, id brain-7k2q
│       ├── .para/
│       ├── README.md
│       ├── notes.md       content
│       └── objectives/    a container
│           └── .para/
├── areas/
├── resources/
└── archive/               a bucket
    ├── projects/          an archive bucket
    ├── areas/
    └── resources/
```

The sketch is abridged; [the example tree](#appendix-a-example-tree) is complete.

Every entity has an id made of the tree's prefix and a short random code, such as `brain-7k2q`, and commands name entities only by id ([addressing](#23-addressing)). `para` never reuses an id. A git merge or a hand copy of an entity's folder can still duplicate one; [`doctor`](#82-doctor-findings) reports it, and [repair](#83-repair) gives all but one holder a new id, the only time an id changes ([addressing](#23-addressing)).

### 1.2 Buckets

The four buckets are the top-level folders of a tree, one for each PARA category: `projects/`, `areas/`, `resources/` and `archive/`. [`init`](#73-init) creates them, each archive bucket appears the first time something is archived into it, and their names are fixed. Each bucket and archive bucket is a container ([the tree](#11-the-tree)), and none is ever moved, renamed, archived or removed. Filing anything starts with choosing its bucket, the most actionable one that fits ([the PARA method](#02-the-para-method)).

Each bucket directly holds:

| Bucket | Holds |
| --- | --- |
| `projects/` | [projects](#13-projects) |
| `areas/` | [areas](#14-areas) |
| `resources/` | [dirs](#111-dirs), the [measurement library](#15-measurement) in `measurement/` and the [sources](#113-provenance) in `sources/` ([resources](#16-resources)) |
| `archive/` | the archive buckets `projects/`, `areas/` and `resources/` |

Each archive bucket holds the projects, areas or dirs archived from the matching bucket, and the [stubs](#17-archive) that stand in for an ancestor so an archived entity keeps its path.

Besides those, every bucket and archive bucket holds its `.para/`, a `README.md` and an `AGENTS.md`. `para` writes its own block in each of the last two, and anything you write outside the block is yours ([para-owned blocks](#52-para-owned-blocks)). Anything else put directly in a bucket or archive bucket, such as a file or a folder with no `.para/`, stays content that `para` never moves or deletes, but [`doctor`](#82-doctor-findings) reports it and says to move it into a project, area or dir, or to [adopt](#24-adopting-existing-directories) a folder as one.

### 1.3 Projects

A project is work with a finish line, the first of the [PARA categories](#02-the-para-method). Its objectives say what it is trying to do, and its key results and status say how far it has got.

Each live project is a folder directly in `projects/`, as sketched in [the tree](#11-the-tree). Its [status](#114-status) is one of `planned` (where a new project starts), `in-progress`, `blocked`, `done` or `dropped`, which a person or agent sets with a command as the work moves, and it may have a due date. Its `.para/state.toml` records these facts, and `para` copies them into keys in the README's frontmatter and keeps its own block in the README. Any other keys and text you put in the README stay as you wrote them ([README frontmatter](#53-readme-frontmatter), [para-owned blocks](#52-para-owned-blocks)).

A project holds its [objectives](#131-objectives) in `objectives/`, each with its [key results](#132-key-results), and its [links](#110-links) in `links/`. It can also hold [dirs](#111-dirs) and any content of your own. A project sits only in `projects/` or `archive/projects/`, and nothing else holds one at any depth, so a large effort is one project with several objectives ([kinds and containment](#22-kinds-and-containment)).

A project stays in `projects/` whatever its status until you [archive](#17-archive) it, which moves it to `archive/projects/`.

#### 1.3.1 Objectives

Objectives and key results (OKRs) are a goal-setting method. An objective says in words what you want to achieve, such as making the website easy to find. Its key results say how you will know: each is a number with a target and a due date, such as 200 weekly visitors from search by 30 June.

In `para`, objectives exist only inside projects. Besides its `.para/` and README, an objective holds only its key results, in `key-results/`, and any content of your own ([kinds and containment](#22-kinds-and-containment)). It has a [status](#114-status) from the same set as a project and may have a due date. Its key results show whether it is being met but never change its status, so a person or agent decides when it is `done`. Areas have a matching pair for ongoing upkeep, [standards](#141-standards) and [indicators](#142-indicators).

#### 1.3.2 Key results

A key result holds an [analytic](#151-analytics) (the number tracked, such as weekly visitors from search) to a target by its due date. Every key result has its own due date, required when it is created, which separates it from an [indicator](#142-indicators) and its rolling window. The analytic computes its value from [metrics](#152-metrics), where each raw value is recorded once, so any key result or indicator can use it ([measurement](#15-measurement)). Each key result is a folder in its objective's `key-results/` and holds no other entities. Together, an objective's key results show whether it is met.

Apart from `dropped`, which you set, its status is computed from the analytic, the target, the due date and the current time, and is never stored ([derived status](#44-derived-status), [what is never stored](#57-what-is-never-stored)). The first row that applies wins:

| Status | Means |
| --- | --- |
| `dropped` | you set it for a key result you no longer pursue; clearing it returns the computed status |
| `achieved` | the analytic has reached the target |
| `missed` | the due date has passed without reaching the target |
| `at-risk` | progress is too slow to reach the target by the due date |
| `on-track` | none of the above |

Before the due date, status follows the latest value, so a key result that falls back short of its target is no longer `achieved`. Once the due date has passed, status uses the latest value dated on or before it, so later readings do not change it.

### 1.4 Areas

An area is a part of work or life you keep up with no end date, such as health or a home, the second of the [PARA categories](#02-the-para-method). Its standards say what good upkeep looks like, and its indicators show whether the area meets them.

Each live area is a folder directly in `areas/` or directly in another area, so areas nest to any depth (health can hold fitness and sleep), and nothing else holds one. An area has no [status](#114-status) and no due date, because it never finishes; [review](#115-review) reports it when it has gone without attention for longer than its staleness setting, which [`init`](#73-init) writes into the root's config for every new tree ([staleness](#45-staleness)). As for a project, `para` keeps an area's facts in [`.para/state.toml`](#32-statetoml), copies them into the README's frontmatter and keeps its own block there ([projects](#13-projects)).

An area holds its [standards](#141-standards) in `standards/`, each with its [indicators](#142-indicators) in `indicators/`, and its [links](#110-links) in `links/`. It can also hold sub-areas, [dirs](#111-dirs) and any content of your own ([kinds and containment](#22-kinds-and-containment)). Standards are optional, and so are a standard's indicators, but every indicator sits under a standard.

An area stays live until you [archive](#17-archive) it, for example when you sell the house it was about. Archiving moves it, with everything beneath it, to the matching path in `archive/areas/`.

#### 1.4.1 Standards

A standard says in words what an area must keep up, such as keeping the house in good repair or getting enough sleep. It is the area's counterpart to an objective, with no status or due date. Besides its `.para/` and README, a standard holds only its indicators, in `indicators/`, and any content of your own ([kinds and containment](#22-kinds-and-containment)). Its indicators show whether it is held.

#### 1.4.2 Indicators

An indicator holds an [analytic](#151-analytics) to a target over a rolling window, such as average nightly sleep over the last 28 days staying at or above seven hours. Others call an indicator a key performance indicator (KPI), or a service level indicator (SLI) with its service level objective (SLO). It is the area's counterpart to a key result, with a window in place of a due date. Each indicator is a folder in its standard's `indicators/` and holds no other entities.

Its status is computed from the analytic over the window, the target, the margin and the current time, and is never stored ([derived status](#44-derived-status), [what is never stored](#57-what-is-never-stored)). The first row that applies wins:

| Status | Means |
| --- | --- |
| `no-data` | the analytic has no value over the window, such as when a metric it uses has no measurements inside it |
| `breached` | the analytic over the window misses the target |
| `at-risk` | the analytic meets the target but is within a margin of missing it |
| `healthy` | none of the above |

The margin is a setting with a default that passes down the tree, where a nearer setting replaces a farther one ([config](#35-config)), and [derived status](#44-derived-status) defines how it applies. You never set an indicator's status; to stop tracking one, remove or [archive](#17-archive) it.

### 1.5 Measurement

The measurement library in `resources/measurement/` defines each number that key results and indicators track, so the same number is never defined twice. `measurement/` is a container that holds three more, `analytics/`, `metrics/` and `terms/`. `para` creates the library and each container when you add the first analytic, metric or term that needs it, and the name `measurement` is reserved in `resources/` before then. Each analytic, metric and term is a folder directly in its container and holds no entities, only its `.para/`, README and any content of your own.

Each refers by id to what it uses. A [key result](#132-key-results) or [indicator](#142-indicators) names one analytic, an analytic names the metrics it computes from, and a metric names the terms it uses ([analytics, metrics and terms](#41-analytics-metrics-and-terms)). So one metric can feed many analytics, and one analytic can serve many key results and indicators. Commands refuse a reference to a missing id, to the wrong kind or to an archived entity, [`show`](#74-reading) lists what each one uses and what uses it, and [`doctor`](#82-doctor-findings) reports a reference whose target has since been removed. A live entity never uses an archived one, whatever its status, so archiving an analytic, metric or term is refused while a live entity uses it, and unarchiving an entity is refused while one it uses is archived. Each refusal names those entities, so you can point them elsewhere or archive or unarchive them first.

#### 1.5.1 Analytics

An analytic is a computation over one or more metrics, as simple as one metric's latest value or as involved as visitors from search divided by all visitors. It has no target of its own, because the key results and indicators that use it set their own. Its value at any time is computed from its metrics' measurements and is never stored ([what is never stored](#57-what-is-never-stored)). [Analytics, metrics and terms](#41-analytics-metrics-and-terms) defines what an analytic can compute.

#### 1.5.2 Metrics

A metric is one thing you measure, such as visitors from search in a week or hours slept in a night. Its measurements are dated numbers, recorded with a command and kept in the metric's `.para/` ([recording measurements](#42-recording-measurements)). The computed statuses of key results and indicators come from them. `para` also writes them to `MEASUREMENTS.csv` in the same `.para/`, linked from the metric's README, so they open in any spreadsheet ([`MEASUREMENTS.csv`](#55-measurementscsv)).

#### 1.5.3 Terms

A term defines a word that metrics share, such as what counts as a critical bug, so every metric that counts critical bugs counts the same thing. The definition is the text of the term's README, which `para` never interprets.

### 1.6 Resources

A resource, the third of the [PARA categories](#02-the-para-method), is reference material on a topic, such as cooking or a programming language. Each resource is a [dir](#111-dirs) in `resources/`, since a resource has no kind of its own, and dirs nest to any depth, so cooking can hold baking. `resources/` also holds the [measurement library](#15-measurement) once you add the first analytic, metric or term, and the [sources](#113-provenance) once you add the first source.

A resource is not a commitment, so it has no [status](#114-status) or due date, and [review](#115-review) never reports it as stale unless you set a staleness period for dirs in [config](#35-config), on it or on a folder above it ([staleness](#45-staleness)).

A resource can be [archived](#17-archive) even when it is nested in another resource that stays live. It moves, with everything beneath it, to the matching path in `archive/resources/`.

### 1.7 Archive

The archive is where finished or inactive things go, so `projects/`, `areas/` and `resources/` show only what is live. `archive/` holds an archive bucket for each of them, and an archived entity sits at the path that mirrors where it was, so `projects/launch-website/` becomes `archive/projects/launch-website/`.

You can [archive](#25-archiving) any entity except the root, a container or a stub, whatever its status, and everything beneath it goes too, unless a live entity still uses it ([measurement](#15-measurement)). Archiving changes where it is and none of its fields, so its status still says how it ended.

When something is archived from inside a live entity, `para` puts a stub at each mirrored ancestor path the archive does not already hold. A stub is an entity that records the id and kind of the entity it stands for and holds only archived entities and other stubs. Stubs are `para`'s alone, so only `show` and `list` take a stub's id. Archiving `projects/launch-website/objectives/grow-traffic/` alone puts stubs at `archive/projects/launch-website/` and at its `objectives/`. When the project is archived later, it and its `objectives/` replace their stubs; `para` refuses only if two archived entities would share a path.

The archive is read-only: an archived entity can be shown, listed, unarchived or [removed](#76-writing), and any other command, even adding a note, needs it unarchived first, except tightening a [label](#112-sensitivity-labels), lowering a [source's grade](#113-provenance) or vouching for a file. Unarchiving moves it back to its live path with everything beneath it and any archived ancestors it needs, leaves stubs for anything still archived, and removes stubs left with nothing beneath them.

### 1.8 Notes and the journal

Every entity except a stub keeps a journal in its `.para/`, the history of what happened to it. Each time a command changes what the folder is or holds, `para` appends a dated event, such as a project's status changing, a child being added, moved, archived or removed, or a note. `para` only ever appends to a journal and never edits an event in it, so anyone arriving later can read what happened and when without having to remember it ([the journal](#33-the-journal)).

Each event goes to the folder it happened to; a child arriving or leaving is its parent's event, so a move gives the old and new parent one each ([principles](#06-principles)). [Removing](#76-writing) an entity deletes its journal, and its parent's journal records the removal (a stub, having none, records nothing).

A note is a short dated entry in an entity's journal, such as "waiting on the design review", not a notes file of your own, and every entity that keeps a journal takes them. You add one with its own command, and any command that changes an entity can carry one to say why. An entity's attention, which keeps it from going [stale](#45-staleness), is its newest note that you marked as attention, as when you record work you did, or else its creation; for a metric, a new measurement counts too. Nothing else counts, not a status change or a child's note, so working notes and reasons never make neglected work look tended.

### 1.9 Activity

Every entity except a stub has an `ACTIVITY.md` in its `.para/`, linked from its README, so its history can be read in any Markdown viewer, such as GitHub, without running `para`. It lists every event in that folder's own journal in plain sentences, grouped by day with the newest day first. So the `projects/` bucket's file shows projects arriving, moving and being archived, and a project's file shows its own status changes and notes, not what happens inside its objectives.

`para` rewrites it from its journal whenever the journal changes ([write-through](#51-write-through)), so an edit to it is lost, and [`rebuild`](#78-rebuild) regenerates it ([`ACTIVITY.md`](#54-activitymd)).

### 1.10 Links

A link points to something outside the tree, such as a URL, a search query, a Slack channel or an email address, so [review](#115-review) can flag it when it is due to be checked or updated. Only projects, areas and [dirs](#111-dirs) hold links, any number each, in their `links/` container, and a link holds no entities ([kinds and containment](#22-kinds-and-containment)).

A link records a `type` you choose, such as `slack-channel` or `rss-feed`; a `ref` the type gives meaning to, such as a channel name or a URL; and a `direction`, `input` (check it for news) or `output` (keep it updated). The type and direction are fixed when the link is created: your own scripts and agents act on the type, and the direction says whether attention on the link means a check or an update. The ref can change, as when a URL moves. `para` never interprets the type or ref and never contacts what a link points to ([scope and non-goals](#05-scope-and-non-goals)). A two-way channel, both watched and posted to, is two links, one for each direction.

Review flags a link through ordinary [staleness](#45-staleness): you record each check or update as a note marked as attention, and `link.stale-after` sets how long a link may go without one. [`init`](#73-init) writes no such setting, so links are never flagged until one is set in [config](#35-config).

### 1.11 Dirs

A dir is the basic kind of entity, a folder for material of any kind, such as a [resource](#16-resources) on cooking or a project's `designs/`. It has no status, due date or target.

Each live dir sits directly in `resources/`, a project, an area or another dir, and nowhere else; an archived one sits at its mirrored path in `archive/` ([archive](#17-archive)). A dir holds other dirs, its [links](#110-links) in its `links/` container and any content of your own, and no other entities ([kinds and containment](#22-kinds-and-containment)).

Like a project or area, a dir has a journal that takes [notes](#18-notes-and-the-journal), an [`ACTIVITY.md`](#19-activity) and a README whose frontmatter and [block](#52-para-owned-blocks) `para` keeps, and it can carry its own [sensitivity label](#112-sensitivity-labels). [Review](#115-review) reports a dir as stale only when a `dir.stale-after` setting reaches it ([staleness](#45-staleness)).

A plain folder in a project, area or dir is content and needs nothing from `para`. Make it a dir when it needs notes, links or staleness of its own; to give it only a stricter label, a path rule in its entity's config is enough ([where labels are set](#63-where-labels-are-set)). [Adopting](#24-adopting-existing-directories) such a folder makes it a dir and keeps everything in it; a folder inside a plain folder can be adopted only once that folder is a dir.

### 1.12 Sensitivity labels

A sensitivity label says how far content may travel. By default `para` uses the five labels of [TLP 2.0](https://www.first.org/tlp/), from loosest to strictest: `CLEAR` (anyone), `GREEN` (your community, but not public channels), `AMBER` (your organization and its clients, as they need it), `AMBER+STRICT` (your organization only) and `RED` (only the people it was given to). The root's [config](#35-config) can define another scale, and [sensitivity labels](#61-sensitivity-labels) defines each one.

A label is set in the [config](#35-config) of any entity except a stub, the root and buckets included, and covers everything beneath it, so no file has to carry a label in itself. A config can also hold path rules, each giving a stricter label to a file or plain folder beneath that folder, though never to anything inside a child entity, which sets its own ([where labels are set](#63-where-labels-are-set)).

A label is never looser than the one above it, and `para` refuses a change that would break this, such as setting `RED` on an area whose sub-area is labelled `AMBER`, until the sub-area is tightened first. A label can also be removed, so the folder takes the one above it. Loosening a label, or removing one when the label above is looser or there is none, asks for confirmation unless run with `--force`, and is journaled either way ([how labels flow](#64-how-labels-flow)).

A command that moves an entity, archiving and unarchiving included, refuses if the label covering it would become looser or none at all, as when an unlabelled dir in a `RED` area moves to a `resources/` labelled `GREEN`, or if any label it carries, its own or one set beneath it, would end up looser than the one above it. The remedy is to tighten the entity's label, or one inside it, and run the command again; a label can be tightened even in the read-only archive.

Content no label covers is shown as `unlabelled`, and `para` assumes nothing about it, so labels are optional; to label everything, set one on the root ([unlabelled content](#66-unlabelled-content)). An entity's README frontmatter, [`show`](#74-reading) and [`list`](#74-reading) give the label that covers it, and `AGENTS.md` tells agents what each label allows ([what para does with labels](#65-what-para-does-with-labels)).

### 1.13 Provenance

Provenance answers two questions about a file: where it came from, and how far to trust it. `para` records the first as facts, a person grades the sources, and `para` computes trust from those two, never from what the file says ([scope and non-goals](#05-scope-and-non-goals)).

A source is an entity that stands for where material comes from, such as a sender, a website, a person or an agent, and carries a reliability grade a person sets, from `A` (reliable) to `F` (cannot be judged) by default ([config](#35-config)). Sources are kept, like terms, in `resources/sources/`, created when you add the first one, and the name `sources` is reserved in `resources/` before then. A source added without a grade starts at the lowest. Setting or raising a grade asks for confirmation unless forced, as loosening a label does, and lowering one is allowed even in the archive.

You record a file with a command after it exists, as a primary or a derived work. A primary is a copy of something from outside the tree, such as an email or a web page; it names its source, and `para` keeps its hash, so a later change to it is flagged. A derived work, such as notes or a summary, names the files it was made from, each by its entity's id and its path, and stays free to change. Every record names the person accountable and, when someone acted for them, such as an agent working for a person, who did the work; both are sources. An agent's source is its model and harness, and each record it makes also notes the exact model, harness version and session, given with the command, as facts that are never graded. `para` takes these names as given.

Trust has two parts, computed whenever it is shown and never stored. A file's reliability is the worst grade among its sources: for a primary, its source alone; for a derived work, the people accountable for it, anyone who acted for them, and everything it was made from, through every input. A primary that changed, a source or input that is gone, or an unrecorded input, counts as the lowest grade. A file is unchecked while it, or anything it was made from, holds recorded work done for a person that they have not since vouched for. A vouch counts only when it names that person with no one acting for them, asks for confirmation unless forced, and puts the person's grade in place of every actor's for the work recorded before it. So a summary that an agent graded `B` writes for a person graded `A`, from an email graded `C`, reads `C, unchecked` until that person vouches for it, and `C` even then.

A file's record sits in the `state.toml` of the entity that holds it, which is never the root, a container or a stub, keyed by its path, and like a label's path rule it never covers a file inside a child entity. Each recording, change to a record and vouch is an event in that entity's journal. A file with no record is `unrecorded`, and `para` assumes nothing about it. [`show`](#74-reading) and [`list`](#74-reading) give a file's provenance and trust, and [`doctor`](#82-doctor-findings) reports a primary that changed and a record whose file is gone ([provenance and trust](#62-provenance-and-trust)).

### 1.14 Status

Status says where a piece of work stands, so active, blocked and finished work can be told apart. Only these four kinds have one:

| Kind | Statuses | How |
| --- | --- | --- |
| [project](#13-projects), [objective](#131-objectives) | `planned` (the default), `in-progress`, `blocked`, `done`, `dropped` | set by you, in any order |
| [key result](#132-key-results) | `dropped`, `achieved`, `missed`, `at-risk`, `on-track` | computed, except `dropped`, which you set and can clear |
| [indicator](#142-indicators) | `no-data`, `breached`, `at-risk`, `healthy` | computed |

A status you set, `dropped` included, is stored in the entity's `state.toml`, and each change is an event in its journal. Setting `blocked` needs a note saying why, so every blocker keeps its reason. A computed status is never stored: `para` works it out from the measurements and the current time whenever it is shown ([derived status](#44-derived-status)).

`done` and `dropped` say how the work ended. A project is closed when its status is one of them, an objective when its own or its project's is, and a key result when it is `dropped` or its objective is closed; all other work is open. Closure is never written to the children, so reopening a project or objective reopens only the work beneath it whose own status is not `done` or `dropped`. [Review](#115-review) stops reporting closed work, and [archiving](#17-archive) it is a separate step.

An open project or objective whose due date has passed is overdue, a flag review reports rather than a status, so the status you set stays as it is.

An archived key result or indicator shows its status as of when it was last archived, alone or with an entity above it, so its status stops changing and an indicator's window does not run out of data. Unarchiving it computes its status at the current time again.

### 1.15 Review

Review answers "what needs attention?", and is `para`'s main daily use ([`review`](#77-review)). It reads only the tree and the current time, and changes nothing.

It reports in six groups, and anything that needs attention appears in every group that applies to it ([review groups](#46-review-groups)):

| Group | Reports |
| --- | --- |
| blocked | projects and objectives set to `blocked`, with the note saying why |
| overdue | open projects and objectives past their due date ([status](#114-status)) |
| key results | open key results that are `at-risk` or `missed` |
| indicators | indicators that are `breached`, `at-risk` or `no-data` |
| stale | entities that have gone without [attention](#18-notes-and-the-journal) for longer than their `<kind>.stale-after` setting ([staleness](#45-staleness)) |
| unchecked | files holding work done for a person that they have not since vouched for, when the file, or any file made from it directly or through other inputs, is live and outside closed work ([provenance](#113-provenance)) |

A missed key result stays listed, except while [suppressed](#116-suppression), until it is closed, archived or removed. The unchecked group lists the files to vouch for, wherever they sit, and names the live files made from each; a file unchecked only through its inputs is not listed itself.

Apart from the unchecked group, review reports nothing in the archive, and nothing that is closed or sits under a closed project or objective, including its dirs, links and files. Areas never close, so their indicators and staleness always count. To pause one flag for a known reason, such as an area left alone while you travel, suppress it.

Review covers attention to the work. The tree's integrity, such as a duplicated id, a primary that changed or a record whose file is gone, is [`doctor`](#82-doctor-findings)'s to report.

### 1.16 Suppression

Suppression pauses one [review](#115-review) flag, an entity's place in one review group, until a date you give, with a note saying why, so review stays useful while something is knowingly on hold. Only flags in these three groups can be suppressed, because while the work waits, the only other ways to clear them are recording attention or a measurement that did not happen, or giving up work you still mean to do:

| Group | Example |
| --- | --- |
| stale | an area left alone for a season, such as a garden in winter |
| key results | a key result `at-risk` because a launch slipped, which you still mean to reach |
| indicators | an indicator with `no-data` while a broken tracker is replaced |

Blocked, overdue and unchecked flags cannot be suppressed, since each has its own remedy: change the status, set a new due date, or vouch for the file.

A suppression leaves the entity's other flags, and everything beneath it, reported as usual. A stale flag can be suppressed before it appears, such as before a trip. A key result or indicator can be suppressed only while it is flagged, and the suppression holds only for that status: one suppressed while `at-risk` is reported again if it becomes `missed`, and one suppressed for `no-data` is reported again if it turns `breached`.

`suppress` sets a suppression, with the end date and a note, and `unsuppress` lifts it early, also with a note ([writing](#76-writing)). The suppression is stored in the entity's `state.toml`, and setting or lifting it is an event in the entity's journal. Neither counts as [attention](#18-notes-and-the-journal), so a suppression never makes neglected work look tended. When the end date passes, the suppression lapses with no event, and review checks the flag as usual ([suppression](#47-suppression)).

Review ends by saying how many flags are suppressed, and lists each with its note and end date when asked ([`review`](#77-review)).

### 1.17 Generated files

`para` writes some files for people and tools that read the tree without running `para`, such as someone browsing it on GitHub or an agent using ordinary file tools. They are part of what `para` delivers, meant to stay in the tree for anyone to read. `para` never reads them as truth: it works only from the state, config and journals in each `.para/`, and reads a README or `AGENTS.md` only to find its own part ([truth and projection](#31-truth-and-projection)).

| File | Where | `para`'s part | Shows |
| --- | --- | --- | --- |
| `README.md` | every entity except a stub | its frontmatter keys and its block | the entity's main facts, such as its id, kind, set status and label; its children and links to its other generated files |
| `AGENTS.md` | the root, each bucket and each archive bucket | its block | the tree's rules and what each label allows, for agents |
| `ACTIVITY.md` | every `.para/` except a stub's | the whole file | the folder's journal in plain sentences ([activity](#19-activity)) |
| `MEASUREMENTS.csv` | a metric's `.para/` | the whole file | the metric's measurements as a table ([metrics](#152-metrics)) |

A command that changes a fact rewrites every generated file that shows it in the same run ([write-through](#51-write-through)), and [`rebuild`](#78-rebuild) rewrites them all from the state, config and journals, as after a git merge or a hand edit. Your own text outside `para`'s part is content and is kept, and anything written inside it is replaced at the next rewrite ([para-owned blocks](#52-para-owned-blocks)).

Generated files show only values that a command alone can change. Values that change without one are left out: whether a project or objective is overdue and whether anything is stale, which follow the clock, and trust, which follows the contents of files. Computed statuses are left out too, so any status a file shows is one you set. [`show`](#74-reading), [`list`](#74-reading) and [review](#115-review) give all of these. So a generated file goes out of date only through a change made outside `para`, such as a hand edit or a git merge, and `rebuild` in the same version of `para` writes the same files on any day and on any machine.

## 2. Model

### 2.1 Directory shapes

Every directory in a tree has one of four shapes, and `para` tells them apart the same way every time:

| Shape | Recognised by | Owner |
| --- | --- | --- |
| entity | it holds a `.para/` whose `state.toml` gives a known kind | `para`, though what you put in it is [yours](#03-structure-is-paras-content-is-yours) |
| damaged folder | it holds something named `.para`, but its kind cannot be read | `para`, though what you put in it is yours |
| `.para/` | its name | `para`, with everything in it, which is never classified further |
| plain folder | anything else | you, as content |

An entity's kind is the `kind` in its `state.toml`, matched exactly, case included, and recorded nowhere else ([`state.toml`](#32-statetoml)). `para` never works out a kind from a folder's name or where it sits. Having read the kind, `para` checks that it may sit there and, for a container, that its name is one fixed for a container where it sits, so its name and parent say which container it is ([kinds and containment](#22-kinds-and-containment)). An archived entity keeps its kind, since being archived is a matter of where it sits.

| Kind | What it is |
| --- | --- |
| `root` | the top of the tree ([the tree](#11-the-tree)) |
| `container` | a bucket (`projects/`, `areas/`, `resources/`, `archive/`), an archive bucket (`projects/`, `areas/` or `resources/` in `archive/`), or one of `objectives/`, `key-results/`, `standards/`, `indicators/`, `links/`, `measurement/`, `analytics/`, `metrics/`, `terms/` and `sources/` |
| `stub` | a stand-in, at the mirrored path in `archive/`, for a live ancestor of an archived entity ([archive](#17-archive)) |
| `project`, `objective`, `key-result`, `area`, `standard`, `indicator`, `analytic`, `metric`, `term`, `source`, `link`, `dir` | the other kinds toured in [chapter 1](#1-components) |

A damaged folder, such as one whose `state.toml` is missing, has no `kind` or names an unknown kind such as `Project`, or whose `.para` is a file, is never guessed at. A command that would change it or anything beneath it refuses, naming the folder and pointing to [`doctor`](#82-doctor-findings), which reports it; [`show`](#74-reading), [`list`](#74-reading) and [`review`](#77-review) leave it and everything beneath it out and warn that they did. A v0 tree is not classified this way, since only [`migrate`](#91-migrating-a-v0-tree) works on it.

### 2.2 Kinds and containment

Every entity except the root sits directly inside another entity, so entities form an unbroken chain from the root. An entity may directly hold only the entities its row lists:

| Entity | May directly hold |
| --- | --- |
| the root | the buckets `projects/`, `areas/`, `resources/` and `archive/` |
| `projects/` | projects |
| `areas/` | areas |
| `resources/` | dirs, and the containers `measurement/` and `sources/` |
| `archive/` | the archive buckets `projects/`, `areas/` and `resources/` |
| a project | the containers `objectives/` and `links/`, and dirs |
| `objectives/` | objectives |
| an objective | the container `key-results/` |
| `key-results/` | key results |
| an area | the containers `standards/` and `links/`, areas and dirs |
| `standards/` | standards |
| a standard | the container `indicators/` |
| `indicators/` | indicators |
| a dir | the container `links/`, and dirs |
| `links/` | links |
| `measurement/` | the containers `analytics/`, `metrics/` and `terms/` |
| `analytics/`, `metrics/`, `terms/`, `sources/` | analytics, metrics, terms or sources, as its name says |
| a key result, indicator, analytic, metric, term, source or link | no entities |

The four buckets always exist. Everything else in the table is optional, so a project may have no objectives and an objective no key results, just as an area may have no standards.

Besides its entities and `para`'s own files ([generated files](#117-generated-files)), an entity may hold content of your own, unless it is a container or a stub. [`doctor`](#82-doctor-findings) reports content found directly in a container or stub, and leaves it in place.

The archive follows the same table. An archive bucket holds the archived entities its matching bucket may hold, and appears the first time anything is archived into it. A stub records the id and kind of the entity it stands for, and counts as that entity. It sits at that entity's mirrored path, inside an archive bucket or another stub, and holds what that entity's row lists ([archive](#17-archive)). How stubs follow renames, moves and removals is in [archiving](#25-archiving).

A container's name comes from the row of the entity that holds it, such as `links/` in a dir. Inside that entity the name is reserved: `para` creates or [adopts](#24-adopting-existing-directories) nothing else under it, and refuses to create the container while something already has the name, saying to rename it first. Names compare as the filesystem compares them, so where case is ignored a plain `Links/` blocks `links/` ([safety guarantees](#81-safety-guarantees)). Elsewhere the name is free: `resources/cooking/measurement/` is an ordinary dir.

A command refuses any change that breaks these rules, such as filing a dir in `projects/` or moving an entity into itself or anything beneath it ([writing](#76-writing)). `doctor` reports breaks made by hand or by a git merge:

- a missing bucket
- an entity out of place, or inside a plain folder
- a container whose name its parent's row does not give
- anything else under a reserved name

### 2.3 Addressing

Commands name an entity only by its id, never by its path or name, so a reference stays good when the entity is renamed, moved or archived. The id is stored in the entity's [`state.toml`](#32-statetoml) and travels with its folder, and `para` finds an entity by reading the tree for its id ([the walk](#36-the-walk)).

An id is the tree's prefix, a hyphen and a random code, such as `brain-7k2q`. The prefix is letters and digits, set when the tree is created or migrated and never changed ([`init`](#73-init), [migrating a v0 tree](#91-migrating-a-v0-tree)). The code is lowercase letters and digits, at least four characters long. New codes get longer as the tree issues more ids, so the chance of two ids ever matching stays at or below 1%: four characters cover about 180 ids, five about 1,100 and six about 6,600. Issuing a new id never changes an existing one. The exact rule: each new code has the shortest length L, at least 4, for which n² ÷ (2 × 36^L) ≤ 0.01, where 36 is the number of characters a code may use and n counts every id issued so far plus the new one. [Worked examples](#appendix-b-worked-examples) says how tests fix the codes `para` draws.

`para` keeps a record of every id it has issued, so it never reuses one and can count them. Each id gets an empty file of that name in the root's `.para/ids/`, which is never deleted, even when the entity is gone. A code whose file already exists is drawn again, and the number of files is the n above. Branches that create entities add files with different names, so git merges the record without a conflict. If two branches draw the same code by chance, their files merge into one and two entities share the id, as below. An entity whose id has no file, such as one copied by hand from another tree with the same prefix, still answers to its id; [`doctor`](#82-doctor-findings) reports it, and [repair](#83-repair) adds the file.

A command takes an id exactly as `para` prints it, never shortened. Every output names entities by id, [`list`](#74-reading) finds one by name, kind or place, and each README's frontmatter carries its entity's id ([README frontmatter](#53-readme-frontmatter)). A command refuses an id it cannot use and says why ([output and exit codes](#710-output-and-exit-codes)):

- its prefix is not this tree's
- two entities hold it
- it no longer exists: no entity holds it, but its file is there, because its entity was removed or, for a stub, replaced by the archived entity
- it was never issued: no entity holds it and it has no file
- the command does not take its kind, as for a stub given to anything but `show` or `list`

A git merge or a hand copy of a folder can leave two entities with one id, and a folder copied in from another tree brings ids with another prefix. `doctor` reports both, and repair gives all but one holder of a duplicated id, and every entity with another prefix, a new id from this tree. You choose which holder keeps a duplicated id. Repair is the only time an id changes. A stub's recorded id is a reference to the entity it stands for, so lookup and duplicate checks ignore it.

### 2.4 Adopting existing directories
How a folder that already exists becomes an entity, so `para` can be introduced to a real tree.

### 2.5 Archiving
How things move into and out of the archive, so finishing work never loses history.

## 3. Storage

### 3.1 Truth and projection
Which files are authoritative and which are derived, so every fact has exactly one home.

### 3.2 `state.toml`
The file in every `.para/` that is the one home of that folder's facts, including its kind, written only by the commands that make each fact true, never by `para config`; the root's (`kind = "root"`) also marks the tree and records its schema version and id prefix.

### 3.3 The journal
The format and rules of the event history, so history can be appended safely and read back.

### 3.4 Time and the clock
How times are recorded and compared, so dates mean the same thing everywhere.

### 3.5 Config
The settings a person chooses with `para config`, stored in each `.para/config.toml`, which change only what `para` does from then on (within the limits of [how labels flow](#64-how-labels-flow)), and how they pass down the tree, so behaviour is set once and inherited below.

### 3.6 The walk
How `para` finds a tree's root and everything in it, and what it does outside any tree, inside a nested tree and in an unmigrated v0 tree, which every command but `migrate` refuses, so every command sees the same set of things.

### 3.7 Git
How `para` behaves inside a git repository, since most trees are versioned and merged.

## 4. Progress and attention

### 4.1 Analytics, metrics and terms
How analytics compute from metrics and how terms scope metrics, all referenced by id, so a measurement is defined once and reused.

### 4.2 Recording measurements
How dated values are recorded against a metric, the only place raw data enters the tree.

### 4.3 Targets: key results and indicators
How a target is set on an analytic, by a due date or over a rolling window, so goals and standards are measured the same way.

### 4.4 Derived status
How key result and indicator statuses are worked out from an analytic, a target and the time, so nobody has to set them by hand.

### 4.5 Staleness
When something counts as neglected, which is the main signal review gives.

### 4.6 Review groups
The categories review sorts things into, so attention is ranked rather than one long list.

### 4.7 Suppression
How a pause changes review, so a known hold never hides a real problem.

## 5. Generated files

### 5.1 Write-through
When generated files are rewritten, so they are never out of date.

### 5.2 Para-owned blocks
The markers that separate `para`'s text from yours, so `para` never overwrites your writing.

### 5.3 README frontmatter
The generated header on each thing's README, the main place a reader sees its facts.

### 5.4 `ACTIVITY.md`
The rendered history file in `.para/`, linked from the README, so a folder's history is readable without the CLI.

### 5.5 `MEASUREMENTS.csv`
The rendered measurement table in a metric's `.para/`, linked from its README, so data opens in any spreadsheet.

### 5.6 `AGENTS.md`
The file that orients an agent to the tree, which is how agents learn the framework.

### 5.7 What is never stored
Values that are always computed and never stored as truth, so no stored copy is trusted over a fresh computation.

## 6. Sensitivity and provenance

### 6.1 Sensitivity labels
The default label set, how the root's config defines another scale, and what each label allows, so a label has one meaning in a tree.

### 6.2 Provenance and trust
Sources and their grades, what a file's record holds (primary or derived, inputs, the accountable person and who acted for them, with the model, harness and session for an agent), how reliability and checked or unchecked are computed, and vouching, so origin and trust are recorded one way.

### 6.3 Where labels are set
How the `config.toml` of any entity but a stub sets its sensitivity and tightens it for paths beneath it, without reaching into a child entity, so every label has exactly one home.

### 6.4 How labels flow
How labels reach the files beneath and the content derived from them (sensitivity only tightens, trust only falls, and only an explicit command, confirmed unless forced, reverses either), so nothing is loosened or trusted by accident.

### 6.5 What para does with labels
How labels appear in outputs and checks, so a label has visible effect.

### 6.6 Unlabelled content
How content with no label, or no provenance record, behaves, so labels and provenance stay optional.

## 7. Command line

### 7.1 Grammar
The shape every command follows, so the CLI can be learned once.

### 7.2 Fields and time input
The fields a command can set and how dates are typed, so input is consistent.

### 7.3 `init`
Creating a tree, the one command that runs before a tree exists.

### 7.4 Reading
The commands that show and list things, the most common way into a tree.

### 7.5 Filters, sorting, limiting
How to narrow and order results, so large trees stay usable.

### 7.6 Writing
The commands that change a tree, each a safe, checked operation.

### 7.7 `review`
The command that answers "what needs attention?", `para`'s main daily use.

### 7.8 `rebuild`
Regenerating every derived file, which is how a tree is repaired or refreshed.

### 7.9 `config`
Reading and changing settings, so config is never hand-edited by guesswork.

### 7.10 Output and exit codes
Text and JSON output and what exit codes mean, so scripts and agents can rely on them.

## 8. Integrity

### 8.1 Safety guarantees
What `para` promises for changes made through its commands, such as no half-finished writes and no deleted user files, so it is safe to run; hand edits fall outside these promises.

### 8.2 `doctor` findings
Every problem `doctor` can report, the safety net for hand edits and other mistakes, including a `CLAUDE.md` that stops Claude Code reading `AGENTS.md`, since nothing stops a file being edited by hand.

### 8.3 Repair
How each finding is fixed, so diagnosis always leads to a remedy.

## 9. Compatibility

### 9.1 Migrating a v0 tree
What `para migrate` does to a v0 (schema 1) tree, such as assigning ids and moving `tree.toml` into the root's `state.toml`, so upgrading is one explicit step that never strands data.

### 9.2 Retired keys and files
What `para` now ignores, such as skills, the `emit.*` keys and a leftover `tree.toml`, so leftovers are understood and not misread.

### 9.3 Schema version
What schema 2 is and how `para` treats other versions, so the format can evolve through `para migrate` and never silently.

## Appendix A. Example tree
One complete example tree, used by every example in the spec, so readers learn one tree.

## Appendix B. Worked examples
Commands run from a stated starting point, usually the example tree, with their output, exit code and resulting files, which double as the acceptance tests, and how those tests fix the clock, new ids and the terminal.

## Appendix C. Glossary
One definition per term, so every word in the spec means one thing.

## Appendix D. Decision index
The decisions behind the design, briefly, so the reasoning is on record without cluttering the spec.
