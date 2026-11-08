# para — specification

Status: **outline.** One sentence per section: what it holds, and why it is here.

## 0. About para

### 0.1 What para is

`para` is a command-line tool that manages the structure and metadata of a PARA file tree: a directory with four buckets (projects, areas, resources and archive), organized by [the PARA method](#02-the-para-method).

The **structure** is which entities (the projects, areas, dirs and other things `para` manages) exist, what kind each one is and where it sits. The **metadata** is the facts `para` records about entities, such as ids, status, history, measurements, classification and lineage. Both are meant to change only through `para`'s commands ([structure is para's, content is yours](#03-structure-is-paras-content-is-yours)). Each command is deterministic: for the same tree, arguments and current time, it gives the same output and makes the same change, apart from any new ids it generates. [Safety guarantees](#81-safety-guarantees) states what `para` promises when a command fails or is interrupted.

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
- everything inside each `.para/`: state (including each recorded file's lineage), config (including classification levels), a metric's measurements and the journal ([storage](#3-storage)). Anything else put inside a `.para/` is `para`'s too;
- the `para` key in README frontmatter ([READMEs](#53-readmes)), its marked blocks in READMEs and `AGENTS.md` files, and, until `para migrate` removes them, the blocks an earlier version wrote in `CLAUDE.md` ([para-owned blocks](#52-para-owned-blocks)).

Change these only through `para`'s commands, including creating, moving, renaming, archiving and removing entities. The commands are designed to be the easiest way for a person or an agent to make these changes. `para` validates each change before writing anything and refuses an invalid one.

Everything else is content: notes, documents, data, other folders, and your own text and frontmatter keys in READMEs and `AGENTS.md`. Anyone may create and edit content. `para` never edits or deletes it, even when removing an entity, and moves it only along with the entity that holds it. Where a README or `AGENTS.md` already exists, `para` adds its part and keeps yours, and adopting a folder adds a `.para/` and keeps everything already there ([adopting existing directories](#24-adopting-existing-directories)). In every other case, if content already sits where `para` needs to create a folder or move an entity, `para` refuses rather than overwrite or merge it.

Changing `para`'s side by hand, such as moving an entity or editing a file in `.para/`, is possible but unsupported, and text written inside `para`'s files or blocks is overwritten the next time `para` rewrites them. `para`'s [safety guarantees](#81-safety-guarantees) cover only changes made through its commands, and [`doctor`](#82-doctor-findings) reports hand changes that leave the tree inconsistent, including a `.para/` that `para` did not create.

Every command depends on this split: it writes only `para`'s side.

### 0.4 Why deterministic structure helps agents

Agents, like people, are drawn to a clear, predictable framework that leaves them free to create within it. The framework keeps their attention on the task, and when following its rules is also the easiest way to work, they follow them. An agent often arrives in a tree with no memory of it and shares the tree with people, scripts and other agents, so it needs rules it can learn quickly and trust everyone else to follow, and `para` provides them.

`para`'s rules for the structure (the buckets, the kinds and which kind may sit where) are small and the same in every tree, so an agent learns them once. The tree explains itself on disk: `AGENTS.md` at the root, in each bucket and in each archive bucket describes the rules ([`AGENTS.md`](#54-agentsmd)), and each entity's README shows its main facts in frontmatter ([READMEs](#53-readmes)), so ordinary file tools are enough to find any entity and read about it.

The commands make the right path the easy one. Every change to the structure is made with a command, so the agent never needs to edit `para`'s side or know how `.para/` is laid out. A command refuses an invalid change before writing anything and says why and what to do instead, and [safety guarantees](#81-safety-guarantees) covers one that fails partway. Commands are [deterministic](#01-what-para-is), so an agent can rely on how each one behaves and confirm its result from the JSON output and exit codes ([output and exit codes](#710-output-and-exit-codes)). Each entity's journal shows the next agent or person what happened to it ([notes and the journal](#18-notes-and-the-journal)).

Within that framework, agents are free to create, edit and organize content with any tool ([structure is para's, content is yours](#03-structure-is-paras-content-is-yours)).

### 0.5 Scope and non-goals

These are outside `para`'s scope, and a feature that falls under one is declined for the reason given.

- **Skills and tool-specific files**: `para` does not distribute skills (instructions packaged for an agent) or generate files for one agent tool, such as Cursor rules or `CLAUDE.md`. Distributing skills is its own problem, and tool-specific files differ from tool to tool. Other tools can layer both on a tree, and layering skills on top is the intended way to customize how agents work with it. `para` writes only its own block in `AGENTS.md`, a file many agent tools read and that you may also use for your own instructions ([`AGENTS.md`](#54-agentsmd)). [`para migrate`](#91-migrating-a-v0-tree) removes the `CLAUDE.md` blocks an earlier version wrote.
- **Version control**: `para` never runs git or requires a repository, so it works the same in any tree and never commits for you. [Git](#37-git) covers how its files behave in a repository.
- **Interpreting content**: `para` never judges a file by what it says, and calls no AI model. A model's answer would make commands nondeterministic, so a file's lineage comes from what a person or agent records, its classification level from the tree's rules, and its trust is computed from those records, the grades a person sets and their vouches.
- **Reaching outside the tree**: `para` never fetches or sends anything over a network, which keeps every command [deterministic](#01-what-para-is). A [link](#110-links) points to something outside the tree and is flagged when it is due to be checked or updated, and outside values enter only as [recorded measurements](#42-recording-measurements).
- **Running on its own**: `para` has no background process, schedule or notifications, so nothing changes the tree unless someone runs a command. [`review`](#77-review) is how to ask what needs attention.
- **Access control**: `para` does not encrypt files or set file permissions. A [classification](#112-classification) level records how far content may travel, but `para` cannot stop a readable file being copied. [What para does with classifications](#65-what-para-does-with-classifications) covers what it does instead.

### 0.6 Principles

Every design decision in this spec follows these rules. A new question is answered by applying them, and a change that would break one needs a recorded decision ([decision index](#appendix-d-decision-index)).

1. **Structure is `para`'s, content is yours**: `para` writes only its own side of the tree and never edits or deletes content ([ownership](#03-structure-is-paras-content-is-yours)).
2. **Deterministic**: the same tree, arguments and time give the same result, apart from new ids ([what para is](#01-what-para-is)).
3. **One home per fact**: each fact is stored as truth in one place ([truth and projection](#31-truth-and-projection)). Each event is recorded once, in the journal of the folder it happened to, where a child arriving or leaving is its parent's event. Computed statuses, analytic values, overdue, stale and trust follow the clock, measurements or file contents, so they are never stored ([what is never stored](#55-what-is-never-stored)). A value that history decides, such as attention, is kept in state by the command that changes it.
4. **Generated files serve readers**: the files `para` writes for readers who never run it show facts and the values computed from them that only a command can change; they are updated with every change, can be rebuilt from the stored truth at any time and are never read back as truth ([generated files](#117-generated-files)).
5. **Ids are identity**: every entity is addressed by a stored id that survives moves and renames and is never reused. Its path only says where it is now ([addressing](#23-addressing)).
6. **The right path is the easy one**: every change to the structure is a command, and a command that cannot make a change validly and safely refuses before writing anything, saying why and what to do instead. It never guesses ([why deterministic structure helps agents](#04-why-deterministic-structure-helps-agents)).
7. **Loosening and format changes are explicit**: a classification level only tightens down the tree and along lineage, and trust only falls as content is derived from other content. Anything that lowers a level or raises trust is a person's act, confirmed unless forced, and never takes a level below its parent's ([grammar](#71-grammar), [how classifications flow](#64-how-classifications-flow)). A tree's format changes only through `para migrate` ([schema version](#93-schema-version)).
8. **Small and one way**: a feature is added only when managing a PARA tree needs it ([scope and non-goals](#05-scope-and-non-goals)), and each concept has one name, each kind of change one command and each value one format ([glossary](#appendix-c-glossary)).

### 0.7 How to read this spec

[Chapter 1](#1-components) tours the tool, with one short section for each component. Chapters 2 to 9 give the detail, and the appendices hold the [example tree](#appendix-a-example-tree), the [worked examples](#appendix-b-worked-examples), the [glossary](#appendix-c-glossary) and the [decision index](#appendix-d-decision-index).

Every statement about what `para` does is a requirement, written in the present tense: "`para` refuses" means an implementation must refuse, "never" means must not, and "may" means allowed but not required. A reason given with a rule explains it and adds no requirement, and neither does the decision index, which summarizes each decision and links its record. Each term means what the glossary says. Chapter 0 and the tour summarize the detail chapters, and examples illustrate them; where any of these disagrees with a detail chapter, the detail chapter is right.

Every example starts from the tree in Appendix A unless it names another starting point, such as an empty directory, a v0 tree or the example tree with a stated change. The worked examples in Appendix B are the acceptance tests: each states its starting point, command and any input it reads, and a test checks its output, exit code and every file's path and contents exactly. Appendix B says how tests fix the clock, new ids and the terminal.

## 1. Components

### 1.1 The tree

A tree is a root directory and everything in it. The root's `.para/state.toml` has `kind = "root"`, which marks the tree, and records the tree's schema version and the prefix of its ids ([`state.toml`](#32-statetoml)). [`init`](#73-init) creates a tree, and every command that works on a tree finds it by checking the current directory and then each parent for a `.para/state.toml` with `kind = "root"`, unless `--root` or `PARA_ROOT` names it ([the walk](#36-the-walk)). A v0 tree is marked by `tree.toml` instead, and only [`migrate`](#91-migrating-a-v0-tree) works on it.

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
| `resources/` | [dirs](#111-dirs), the [measurement library](#15-measurement) in `measurement/` and the [sources](#113-lineage) in `sources/` ([resources](#16-resources)) |
| `archive/` | the archive buckets `projects/`, `areas/` and `resources/` |

Each archive bucket holds the projects, areas or dirs archived from the matching bucket, and the [stubs](#17-archive) that stand in for an ancestor so an archived entity keeps its path.

Besides those, every bucket and archive bucket holds its `.para/`, a `README.md` and an `AGENTS.md`. `para` writes its own block in each of the last two, and anything you write outside the block is yours ([para-owned blocks](#52-para-owned-blocks)). Anything else put directly in a bucket or archive bucket, such as a file or a folder with no `.para/`, stays content that `para` never moves or deletes, but [`doctor`](#82-doctor-findings) reports it and says to move it into a project, area or dir, or to [adopt](#24-adopting-existing-directories) a folder as one.

### 1.3 Projects

A project is work with a finish line, the first of the [PARA categories](#02-the-para-method). Its objectives say what it is trying to do, and its key results and status say how far it has got.

Each live project is a folder directly in `projects/`, as sketched in [the tree](#11-the-tree). Its [status](#114-status) is one of `planned` (where a new project starts), `in-progress`, `blocked`, `done` or `dropped`, which a person or agent sets with a command as the work moves, and it may have a due date. Its `.para/state.toml` records these facts, and `para` copies them into the `para` key of the README's frontmatter and keeps its own block in the README. Any other keys and text you put in the README stay as you wrote them ([READMEs](#53-readmes), [para-owned blocks](#52-para-owned-blocks)).

A project holds its [objectives](#131-objectives) in `objectives/`, each with its [key results](#132-key-results), and its [links](#110-links) in `links/`. It can also hold [dirs](#111-dirs) and any content of your own. A project sits only in `projects/` or `archive/projects/`, and nothing else holds one at any depth, so a large effort is one project with several objectives ([kinds and containment](#22-kinds-and-containment)).

A project stays in `projects/` whatever its status until you [archive](#17-archive) it, which moves it to `archive/projects/`.

#### 1.3.1 Objectives

Objectives and key results (OKRs) are a goal-setting method. An objective says in words what you want to achieve, such as making the website easy to find. Its key results say how you will know: each is a number with a target and a due date, such as 200 weekly visitors from search by 30 June.

In `para`, objectives exist only inside projects. Besides its `.para/` and README, an objective holds only its key results, in `key-results/`, and any content of your own ([kinds and containment](#22-kinds-and-containment)). It has a [status](#114-status) from the same set as a project and may have a due date. Its key results show whether it is being met but never change its status, so a person or agent decides when it is `done`. Areas have a matching pair for ongoing upkeep, [standards](#141-standards) and [indicators](#142-indicators).

#### 1.3.2 Key results

A key result holds an [analytic](#151-analytics) (the number tracked, such as weekly visitors from search) to a target by its due date. Every key result has its own due date, required when it is created, which separates it from an [indicator](#142-indicators), which tracks a rolling analytic. The analytic computes its value from [metrics](#152-metrics), where each raw value is recorded once, so any key result or indicator can use it ([measurement](#15-measurement)). Each key result is a folder in its objective's `key-results/` and holds no other entities. Together, an objective's key results show whether it is met.

Apart from `dropped`, which you set, its status is computed from the analytic, the target, the due date and the current time, and is never stored ([derived status](#44-derived-status), [what is never stored](#55-what-is-never-stored)). The first row that applies wins:

| Status | Means |
| --- | --- |
| `dropped` | you set it for a key result you no longer pursue; clearing it returns the computed status |
| `achieved` | the analytic has reached the target |
| `missed` | the due date has passed without reaching the target |
| `at-risk` | progress is too slow to reach the target by the due date |
| `on-track` | none of the above |

Before the due date, status follows the analytic's current value, so a key result that falls back short of its target is no longer `achieved`. Once the due date has passed, status uses the analytic's value computed at the due date ([analytics, metrics and terms](#41-analytics-metrics-and-terms)), so readings claimed later do not change it.

### 1.4 Areas

An area is a part of work or life you keep up with no end date, such as health or a home, the second of the [PARA categories](#02-the-para-method). Its standards say what good upkeep looks like, and its indicators show whether the area meets them.

Each live area is a folder directly in `areas/` or directly in another area, so areas nest to any depth (health can hold fitness and sleep), and nothing else holds one. An area has no [status](#114-status) and no due date, because it never finishes; [review](#115-review) reports it when it has gone without attention for longer than its staleness setting, which [`init`](#73-init) writes into the root's config for every new tree ([staleness](#45-staleness)). As for a project, `para` keeps an area's facts in [`.para/state.toml`](#32-statetoml), copies them into the README's frontmatter and keeps its own block there ([projects](#13-projects)).

An area holds its [standards](#141-standards) in `standards/`, each with its [indicators](#142-indicators) in `indicators/`, and its [links](#110-links) in `links/`. It can also hold sub-areas, [dirs](#111-dirs) and any content of your own ([kinds and containment](#22-kinds-and-containment)). Standards are optional, and so are a standard's indicators, but every indicator sits under a standard.

An area stays live until you [archive](#17-archive) it, for example when you sell the house it was about. Archiving moves it, with everything beneath it, to the matching path in `archive/areas/`.

#### 1.4.1 Standards

A standard says in words what an area must keep up, such as keeping the house in good repair or getting enough sleep. It is the area's counterpart to an objective, with no status or due date. Besides its `.para/` and README, a standard holds only its indicators, in `indicators/`, and any content of your own ([kinds and containment](#22-kinds-and-containment)). Its indicators show whether it is held.

#### 1.4.2 Indicators

An indicator holds a rolling [analytic](#151-analytics), such as average nightly sleep over the last 28 days, to a bound, such as at least seven hours, or to a range, such as seven to nine hours. Others call an indicator a key performance indicator (KPI), or a service level indicator (SLI) with its service level objective (SLO). It is the area's counterpart to a key result, usually with a rolling analytic in place of a due date. Each indicator is a folder in its standard's `indicators/` and holds no other entities.

Its status is computed from the analytic, its bounds, the margin and the current time, and is never stored ([derived status](#44-derived-status), [what is never stored](#55-what-is-never-stored)). The first row that applies wins:

| Status | Means |
| --- | --- |
| `no-data` | the analytic has no value, such as when its range holds no measurements |
| `breached` | the analytic is outside its bounds |
| `at-risk` | the analytic is inside its bounds, but within a margin of leaving them |
| `healthy` | none of the above |

The margin is a setting with a default that passes down the tree, where a nearer setting replaces a farther one ([config](#35-config)), and [derived status](#44-derived-status) defines how it applies. You never set an indicator's status; to stop tracking one, remove or [archive](#17-archive) it.

### 1.5 Measurement

The measurement library in `resources/measurement/` defines each number that key results and indicators track, so the same number is never defined twice. `measurement/` is a container that holds three more, `analytics/`, `metrics/` and `terms/`. `para` creates the library and each container when you add the first analytic, metric or term that needs it, and the name `measurement` is reserved in `resources/` before then. Each analytic, metric and term is a folder directly in its container and holds no entities, only its `.para/`, README and any content of your own.

Each refers by id to what it uses. A [key result](#132-key-results) or [indicator](#142-indicators) names one analytic, an analytic names the metrics it computes from, and a metric names the terms and sources it uses ([analytics, metrics and terms](#41-analytics-metrics-and-terms)). So one metric can feed many analytics, and one analytic can serve many key results and indicators. Commands refuse a reference to a missing id, to the wrong kind or to an archived entity, [`show`](#74-reading) lists what each one uses and what uses it, and [`doctor`](#82-doctor-findings) reports a reference whose target has since been removed. A live entity never uses an archived one, whatever its status, so archiving an analytic, metric, term or source is refused while a live entity uses it, and unarchiving an entity is refused while one it uses is archived. Each refusal names those entities, so you can point them elsewhere or archive or unarchive them first.

#### 1.5.1 Analytics

An analytic is a computation over one or more metrics, as simple as one metric's latest value or as involved as visitors from search divided by all visitors. It has no target of its own, because the key results and indicators that use it set their own. Its value at any time is computed from its metrics' measurements and is never stored ([what is never stored](#55-what-is-never-stored)). [Analytics, metrics and terms](#41-analytics-metrics-and-terms) defines what an analytic can compute.

#### 1.5.2 Metrics

A metric is one thing you measure, such as visitors from search in a week or hours slept in a night. Its measurements are dated numbers you record with a command. Each is one row in `measurements.csv` in the metric's `.para/`, which its README links to and any spreadsheet opens ([recording measurements](#42-recording-measurements)). The computed statuses of key results and indicators come from them. A metric can name the sources its values come from, such as a bathroom scale ([lineage](#113-lineage)).

#### 1.5.3 Terms

A term defines a word that metrics share, such as what counts as a critical bug, so every metric that counts critical bugs counts the same thing. The definition is the text of the term's README, which `para` never interprets.

### 1.6 Resources

A resource, the third of the [PARA categories](#02-the-para-method), is reference material on a topic, such as cooking or a programming language. Each resource is a [dir](#111-dirs) in `resources/`, since a resource has no kind of its own, and dirs nest to any depth, so cooking can hold baking. `resources/` also holds the [measurement library](#15-measurement) once you add the first analytic, metric or term, and the [sources](#113-lineage) once you add the first source.

A resource is not a commitment, so it has no [status](#114-status) or due date, and [review](#115-review) never reports it as stale unless you set a staleness period for dirs in [config](#35-config), on it or on a folder above it ([staleness](#45-staleness)).

A resource can be [archived](#17-archive) even when it is nested in another resource that stays live. It moves, with everything beneath it, to the matching path in `archive/resources/`.

### 1.7 Archive

The archive is where finished or inactive things go, so `projects/`, `areas/` and `resources/` show only what is live. `archive/` holds an archive bucket for each of them, and an archived entity sits at the path that mirrors where it was, so `projects/launch-website/` becomes `archive/projects/launch-website/`.

You can [archive](#25-archiving) any entity except the root, a container or a stub, whatever its status, and everything beneath it goes too, unless a live entity still uses it ([measurement](#15-measurement)). Archiving changes where it is and none of its fields, so its status still says how it ended.

When something is archived from inside a live entity, `para` puts a stub for each live ancestor that nothing in the archive stands for yet. A stub is an entity that records the id and kind of the entity it stands for and holds only archived entities and other stubs. Stubs are `para`'s alone, so only `show` and `list` take a stub's id. Archiving `projects/launch-website/objectives/grow-traffic/` alone puts stubs at `archive/projects/launch-website/` and at its `objectives/`. When the project is archived later, it and its `objectives/` replace their stubs. An entity or stub arriving at a path already held gets a numbered suffix ([archiving](#25-archiving)).

The archive is read-only: an archived entity can be shown, listed, unarchived or [removed](#76-writing), and any other command, even adding a note, needs it unarchived first, except tightening a [classification](#112-classification) level, lowering a [source's grade](#113-lineage), vouching for a file or metric, or a change of the [classification scale](#61-classification-levels). Unarchiving moves it back to its live path with everything beneath it and any archived ancestors it needs, leaves stubs for anything still archived, and removes stubs left with nothing beneath them.

### 1.8 Notes and the journal

Every entity except a stub keeps a journal in its `.para/`, the history of what was done to it through `para`. Each time a command changes what the folder is or holds, `para` appends a dated event, such as a project's status changing, a child being added, moved, archived or removed, a measurement being recorded, or a note. `para` only appends to a journal, so anyone arriving later can read what happened and when without having to remember it. Only [`migrate`](#91-migrating-a-v0-tree) changes old events ([the journal](#33-the-journal)).

The journal is a log. Current values live in the entity's state, config and, for a metric, its measurements ([truth and projection](#31-truth-and-projection)).

Each event goes to the folder it happened to; a child arriving or leaving is its parent's event, so a move gives the old and new parent one each ([principles](#06-principles)). [Removing](#76-writing) an entity deletes its journal, and its parent's journal records the removal (a stub, having none, records nothing).

A note is a short dated entry in an entity's journal, such as "waiting on the design review", and every entity that keeps a journal takes them. You add one with its own command, and any command that changes an entity can carry one to say why.

An entity's attention keeps it from going [stale](#45-staleness). Its attention time is the latest of:

- its creation
- its newest note marked as attention, such as a note about work you did
- for a metric, its newest measurement

`para` keeps the attention time in [`state.toml`](#32-statetoml). A status change or a child's note does not count, so routine notes never make neglected work look tended.

### 1.9 History

An entity's history is its journal, a file of JSON lines that any agent or text viewer can read without running `para`. `para` also shows it on request, newest first ([reading](#74-reading)).

### 1.10 Links

A link points to something outside the tree, such as a URL, a search query, a Slack channel or an email address, so [review](#115-review) can flag it when it is due to be checked or updated. Only projects, areas and [dirs](#111-dirs) hold links, any number each, in their `links/` container, and a link holds no entities ([kinds and containment](#22-kinds-and-containment)).

A link records a `type` you choose, such as `slack-channel` or `rss-feed`; a `ref` the type gives meaning to, such as a channel name or a URL; and a `direction`, `input` (check it for news) or `output` (keep it updated). The type and direction are fixed when the link is created: your own scripts and agents act on the type, and the direction says whether attention on the link means a check or an update. The ref can change, as when a URL moves. `para` never interprets the type or ref and never contacts what a link points to ([scope and non-goals](#05-scope-and-non-goals)). A two-way channel, both watched and posted to, is two links, one for each direction.

Review flags a link through ordinary [staleness](#45-staleness): you record each check or update as a note marked as attention, and `link.stale-after` sets how long a link may go without one. [`init`](#73-init) writes no such setting, so links are never flagged until one is set in [config](#35-config).

### 1.11 Dirs

A dir is the basic kind of entity, a folder for material of any kind, such as a [resource](#16-resources) on cooking or a project's `designs/`. It has no status, due date or target.

Each live dir sits directly in `resources/`, a project, an area or another dir, and nowhere else; an archived one sits at its mirrored path in `archive/` ([archive](#17-archive)). A dir holds other dirs, its [links](#110-links) in its `links/` container and any content of your own, and no other entities ([kinds and containment](#22-kinds-and-containment)).

Like a project or area, a dir has a journal that takes [notes](#18-notes-and-the-journal) and a README whose `para` frontmatter key and [block](#52-para-owned-blocks) `para` keeps, and it can be [classified](#112-classification) at its own level. [Review](#115-review) reports a dir as stale only when a `dir.stale-after` setting reaches it ([staleness](#45-staleness)).

A plain folder in a project, area or dir is content and needs nothing from `para`. Make it a dir when it needs notes, links or staleness of its own; to give it only a stricter level, a path rule in its entity's config is enough ([where classifications are set](#63-where-classifications-are-set)). [Adopting](#24-adopting-existing-directories) such a folder makes it a dir and keeps everything in it; a folder inside a plain folder can be adopted only once that folder is a dir.

### 1.12 Classification

Classification says how far content may travel. Each folder can be given a level, and by default `para` uses the five levels of [TLP 2.0](https://www.first.org/tlp/), from loosest to strictest: `CLEAR` (anyone), `GREEN` (your community, but not public channels), `AMBER` (your organization and its clients, as they need it), `AMBER+STRICT` (your organization only) and `RED` (only the people it was given to). The root's [config](#35-config) can define another scale, and [classification levels](#61-classification-levels) defines each one.

A level is set in the [config](#35-config) of any entity except a stub, the root and buckets included, and covers everything beneath it, so no file has to carry one in itself. A config can also hold path rules, each giving a stricter level to a file or plain folder beneath that folder, though never to anything inside a child entity, which sets its own ([where classifications are set](#63-where-classifications-are-set)). A file recorded as made from other files also takes the strictest of their levels, until a person declassifies it ([how classifications flow](#64-how-classifications-flow)).

A level is never looser than the one above it, and `para` refuses a change that would break this, such as classifying an area `RED` when its sub-area is classified `AMBER`, until the sub-area is tightened first. A level can also be removed, so the folder takes the one above it. Loosening a level, or removing one when the one above is looser or there is none, is a person's act: it is refused unless run by a person with no one acting for them, asks for confirmation unless run with `--force`, and is journaled either way ([how classifications flow](#64-how-classifications-flow)).

A command that moves an entity, archiving and unarchiving included, refuses if the level covering it would become looser or none at all, as when an unclassified dir in a `RED` area moves to a `resources/` classified `GREEN`, or if any level it carries, its own or one set beneath it, would end up looser than the one above it. The remedy is to tighten the entity's level, or one inside it, and run the command again; a level can be tightened even in the read-only archive.

Content no level covers is unclassified, and `para` assumes nothing about it, so classification is optional; to classify everything, set a level on the root ([unclassified content](#66-unclassified-content)). An entity's README frontmatter, [`show`](#74-reading) and [`list`](#74-reading) give the level that covers it, and `AGENTS.md` tells agents what each level allows ([what para does with classifications](#65-what-para-does-with-classifications)).

### 1.13 Lineage

Lineage records where content came from, as a graph: sources at its start, then the files and measurements that come from them, then what is made from those. `para` records the graph as facts, a person grades the sources, and `para` computes trust from those two, never from what a file says ([scope and non-goals](#05-scope-and-non-goals)). Classification flows along the same graph ([how classifications flow](#64-how-classifications-flow)).

A source is an entity that stands for where material comes from, such as a sender, a website, a person or an agent, and carries a reliability grade a person sets, from `A` (reliable) to `F` (cannot be judged) by default ([config](#35-config)). Sources are kept, like terms, in `resources/sources/`, which [`init`](#73-init) creates along with your own source, since every event names one ([the journal](#33-the-journal)). A source added without a grade starts at the lowest. Setting or raising a grade is a person's act, as loosening a level is, and lowering one is allowed even in the archive.

You record a file with a command after it exists, as a primary or a derived work. A primary is a copy of something from outside the tree, such as an email or a web page; it names its source, and `para` keeps its hash, so a later change to it is flagged. A derived work, such as notes or a summary, names the files it was made from by their records' ids, and stays free to change. Each input may carry a one-line note saying what was done to it, such as "kept only the signing date". Naming a file that has no record gives it a bare one, which counts as unrecorded until you record it. Naming one where no record can be written, such as in the archive, is refused. Every record names the person accountable and, when someone acted for them, such as an agent working for a person, who did the work; both are sources. An agent's source is its model and harness, and each record it makes also notes the exact model, harness version and session, given with the command, as facts that are never graded. `para` takes these names as given.

Measurements join the same graph. A metric may name its sources, such as a bathroom scale or a payment service. An analytic is made from what its formula names, each of which may carry a note too, and a key result or indicator from its analytic ([measurement](#15-measurement)). Changing a metric's sources so its reliability rises is a person's act too, as is any change to a record that raises a file's reliability, such as naming a better-graded source or dropping an input. A source can be archived unless a live metric names it; a record or event that names it does not prevent this.

Trust has two parts, computed whenever it is shown and never stored. A file's reliability is the worst grade among its sources: for a primary, its source alone; for a derived work, the people accountable for it, anyone who acted for them, and everything it was made from, through every input. A primary that changed, a source or input that is gone, or an unrecorded input, counts as the lowest grade. A measurement's reliability is the worst grade among its metric's sources, the lowest when it names none or one is gone. The person it was recorded for, and anyone acting for them, count too. An analytic, key result or indicator takes the worst of the measurements it uses. A file is unchecked while it, or anything it was made from, holds recorded work done for a person that they have not since vouched for, and a metric while it holds such measurements. A vouch, on a file or a metric, is that person's act, with no one acting for them ([grammar](#71-grammar)), and puts the person's grade in place of every actor's for the work recorded before it. It never lifts reliability above the worst source. So a summary that an agent graded `B` writes for a person graded `A`, from an email graded `C`, reads `C, unchecked` until that person vouches for it, and `C` even then.

A file's lineage record has an id, drawn and recorded like entity ids ([addressing](#23-addressing)), and sits in the `state.toml` of the entity that holds it, which is never the root, a container or a stub. Its path within that entity is a property of the record: after a rename or move, a command updates the path, and every input still resolves because it names the record's id. Like a path rule, a record never covers a file inside a child entity, with one exception: a file in a container, or in a plain folder there waiting to be adopted, keeps its record in the nearest ancestor that can hold one. Each recording, change to a record and vouch is an event in that entity's journal. A file with no record is `unrecorded`, and `para` assumes nothing about it. [`show`](#74-reading) gives what a file or entity was made from, what was made from it, and its trust, which [`list`](#74-reading) gives too. [`doctor`](#82-doctor-findings) reports a primary that changed and a record whose file is gone; a command sets its new path ([lineage and trust](#62-lineage-and-trust)).

### 1.14 Status

Status says where a piece of work stands, so active, blocked and finished work can be told apart. Only these four kinds have one:

| Kind | Statuses | How |
| --- | --- | --- |
| [project](#13-projects), [objective](#131-objectives) | `planned` (the default), `in-progress`, `blocked`, `done`, `dropped` | set by you, in any order |
| [key result](#132-key-results) | `dropped`, `achieved`, `missed`, `at-risk`, `on-track` | computed, except `dropped`, which you set and can clear |
| [indicator](#142-indicators) | `no-data`, `breached`, `at-risk`, `healthy` | computed |

A status you set, `dropped` included, is stored in the entity's `state.toml`, and each change is an event in its journal. Setting `blocked` needs a note saying why. State keeps that reason until the status changes. A computed status is never stored: `para` works it out from the measurements and the current time whenever it is shown ([derived status](#44-derived-status)).

State also keeps `status-since`, the time the status last changed, so review and [`show`](#74-reading) can say how long work has been blocked or in progress. Setting the same status again, such as `blocked` with a new reason, keeps it. Creating or adopting a project or objective sets it to `created`, and [`migrate`](#91-migrating-a-v0-tree) works it out from the v0 history. A key result has it only while it is `dropped`.

`done` and `dropped` say how the work ended. A project is closed when its status is one of them, an objective when its own or its project's is, and a key result when it is `dropped` or its objective is closed; all other work is open. Closure is never written to the children, so reopening a project or objective reopens only the work beneath it whose own status is not `done` or `dropped`. [Review](#115-review) stops reporting closed work, and [archiving](#17-archive) it is a separate step.

An open project or objective whose due date has passed is overdue, a flag review reports rather than a status, so the status you set stays as it is.

An archived key result or indicator shows its status as of when it was last archived, alone or with an entity above it, so its status no longer follows the clock and an indicator's analytic does not run out of data. A setting changed above it, such as a margin, can still change it ([config](#35-config)). Unarchiving it computes its status at the current time again.

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
| unchecked | files holding work done for a person that they have not since vouched for, when the file, or any file made from it directly or through other inputs, is live and outside closed work; metrics holding such measurements, when a live key result or indicator outside closed work uses them ([lineage](#113-lineage)) |

A missed key result stays listed, except while [suppressed](#116-suppression), until it is closed, archived or removed. The unchecked group lists the files and metrics to vouch for, wherever they sit, and names the live files, key results and indicators that use each; a file unchecked only through its inputs is not listed itself.

Apart from the unchecked group, review reports nothing in the archive, and nothing that is closed or sits under a closed project or objective, including its dirs, links and files. Areas never close, so their indicators and staleness always count. To pause one flag for a known reason, such as an area left alone while you travel, suppress it.

Review covers attention to the work. The tree's integrity, such as a duplicated id, a primary that changed or a record whose file is gone, is [`doctor`](#82-doctor-findings)'s to report.

### 1.16 Suppression

Suppression pauses one [review](#115-review) flag, an entity's place in one review group, until a date you give, with a note saying why, so review stays useful while something is knowingly on hold. Only flags in these four groups can be suppressed, because while the work waits, the only other ways to clear them are recording attention or a measurement that did not happen, setting a status that is not true, or giving up work you still mean to do:

| Group | Example |
| --- | --- |
| blocked | a project waiting months on a legal review |
| stale | an area left alone for a season, such as a garden in winter |
| key results | a key result `at-risk` because a launch slipped, which you still mean to reach |
| indicators | an indicator with `no-data` while a broken tracker is replaced |

Overdue and unchecked flags cannot be suppressed, since each has its own remedy: set a new due date, or vouch for the file or metric.

A suppression leaves the entity's other flags, and everything beneath it, reported as usual. A stale flag can be suppressed before it appears, such as before a trip. A status can be suppressed only while it is flagged, and the suppression holds only for that status: one suppressed while `at-risk` is reported again if it becomes `missed`, and one suppressed for `no-data` is reported again if it turns `breached`.

`suppress` sets a suppression, with its end and a note, and `unsuppress` lifts it early, also with a note ([writing](#76-writing)). The suppression is stored in the entity's `state.toml`, and setting or lifting it is an event in the entity's journal. Neither counts as [attention](#18-notes-and-the-journal), so a suppression never makes neglected work look tended. When the end date passes, the suppression lapses with no event, and review checks the flag as usual ([suppression](#47-suppression)).

Review ends by saying how many flags its suppressions leave out, and lists each suppression with its note and end when asked ([`review`](#77-review)).

### 1.17 Generated files

`para` writes some files for people and tools that read the tree without running `para`, such as someone browsing it on GitHub or an agent using ordinary file tools. They are part of what `para` delivers, meant to stay in the tree for anyone to read. `para` never reads them as truth: it works only from the truth files in each `.para/`, and reads a README or `AGENTS.md` only to find its own part ([truth and projection](#31-truth-and-projection)).

| File | Where | `para`'s part | Shows |
| --- | --- | --- | --- |
| `README.md` | every entity except a stub | its `para` frontmatter key and its block | the entity's main facts, such as its id, kind, set status and classification level; its title and description, its children, a link to its history and, for a metric, a link to its measurements |
| `AGENTS.md` | the root, each bucket and each archive bucket | its block | the tree's rules and what each classification level allows, for agents |

A command that changes a fact rewrites every generated file that shows it in the same run ([write-through](#51-write-through)), and [`rebuild`](#78-rebuild) rewrites them all from the truth files, as after a git merge or a hand edit. Your own text outside `para`'s part is content and is kept, and anything written inside it is replaced at the next rewrite ([para-owned blocks](#52-para-owned-blocks)).

Generated files show only values that a command alone can change. Values that change without one are left out: whether a project or objective is overdue and whether anything is stale, which follow the clock, and trust and a derived work's level, which follow other files and their records. Computed statuses are left out too, so any status a file shows is one you set. [`show`](#74-reading), [`list`](#74-reading) and [review](#115-review) give all of these. So a generated file goes out of date only through a change made outside `para`, such as a hand edit or a git merge, and `rebuild` in the same version of `para` writes the same files on any day and on any machine.

## 2. Model

### 2.1 Directory shapes

Every directory in a tree has one of four shapes, and `para` tells them apart the same way every time:

| Shape | Recognised by | Owner |
| --- | --- | --- |
| entity | it holds a `.para/` whose `state.toml` gives a known kind | `para`, though what you put in it is [yours](#03-structure-is-paras-content-is-yours) |
| damaged folder | it holds something named `.para`, but its kind cannot be read, or its `config.toml` exists but cannot be read ([truth and projection](#31-truth-and-projection)), or `para` is denied when checking it for `.para` | `para`, though what you put in it is yours |
| `.para/` | its name | `para`, with everything in it, which is never classified further |
| plain folder | anything else | you, as content |

A folder inside your content is content too, whatever it holds, since the walk down never looks inside content ([the walk](#36-the-walk)).

An entity's kind is the `kind` in its `state.toml`, matched exactly, case included, and recorded nowhere else ([`state.toml`](#32-statetoml)). `para` never works out a kind from a folder's name or where it sits. Having read the kind, `para` checks that it may sit there and, for a container, that its name is one fixed for a container where it sits, so its name and parent say which container it is ([kinds and containment](#22-kinds-and-containment)). An archived entity keeps its kind, since being archived is a matter of where it sits.

| Kind | What it is |
| --- | --- |
| `root` | the top of the tree ([the tree](#11-the-tree)) |
| `container` | a bucket (`projects/`, `areas/`, `resources/`, `archive/`), an archive bucket (`projects/`, `areas/` or `resources/` in `archive/`), or one of `objectives/`, `key-results/`, `standards/`, `indicators/`, `links/`, `measurement/`, `analytics/`, `metrics/`, `terms/` and `sources/` |
| `stub` | a stand-in in `archive/` for an ancestor of an archived entity, one that is live or was removed ([archiving](#25-archiving)) |
| `project`, `objective`, `key-result`, `area`, `standard`, `indicator`, `analytic`, `metric`, `term`, `source`, `link`, `dir` | the other kinds toured in [chapter 1](#1-components) |

A damaged folder, such as one whose `state.toml` is missing, has no `kind` or names an unknown kind such as `Project`, or whose `.para` is a file, or whose `config.toml` cannot be read, is never guessed at. A command that would change it or anything beneath it refuses, naming the folder and pointing to [`doctor`](#82-doctor-findings), which reports it; [`show`](#74-reading), [`list`](#74-reading) and [`review`](#77-review) leave it and everything beneath it out and warn that they did. A v0 tree is not classified this way, since only [`migrate`](#91-migrating-a-v0-tree) works on it.

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

The archive follows the same table. An archive bucket holds the archived entities its matching bucket may hold, and appears the first time anything is archived into it. A stub records the id and kind of the entity it stands for, and counts as that entity. It sits where that entity would be archived, inside an archive bucket, another stub or an archived entity, and holds what that entity's row lists ([archive](#17-archive)). How stubs follow renames, moves and removals is in [archiving](#25-archiving).

A container's name comes from the row of the entity that holds it, such as `links/` in a dir. Inside that entity the name is reserved: `para` creates or [adopts](#24-adopting-existing-directories) nothing else under it, and refuses to create the container while something already has the name, saying to rename it first. Names compare as the filesystem compares them, so where case is ignored a plain `Links/` blocks `links/` ([safety guarantees](#81-safety-guarantees)). Elsewhere the name is free: `resources/cooking/measurement/` is an ordinary dir.

A command refuses any change that breaks these rules, such as filing a dir in `projects/` or moving an entity into itself or anything beneath it ([writing](#76-writing)). `doctor` reports breaks made by hand or by a git merge:

- a missing bucket
- an entity out of place, or inside a plain folder
- a container whose name its parent's row does not give
- anything else under a reserved name

### 2.3 Addressing

Commands name an entity only by its id, never by its path or name. A path says where an entity is now, and that changes whenever it is renamed, moved or archived, often by people, agents or git outside `para`, which cannot rewrite the notes, scripts and memories that mention it. An id stays the same for the entity's whole life and is never given to another, so a stale reference fails instead of reaching the wrong entity. Write the id alone when you refer to an entity, and use [`show`](#74-reading) to see what it names. The id is stored in the entity's [`state.toml`](#32-statetoml) and travels with its folder, and `para` finds an entity by reading the tree for its id ([the walk](#36-the-walk)). [Lineage](#113-lineage) records have ids too, and a derived work names its inputs by them.

An id is the tree's prefix, a hyphen and a random code, such as `brain-7k2q`. The prefix is letters and digits, set when the tree is created or migrated and never changed ([`init`](#73-init), [migrating a v0 tree](#91-migrating-a-v0-tree)). The code is lowercase letters and digits, at least four characters long. New codes get longer as the tree issues more ids, so the chance of two ids ever matching stays at or below 1%: four characters cover about 180 ids, five about 1,100 and six about 6,600. Issuing a new id never changes an existing one. The exact rule: each new code has the shortest length L, at least 4, for which n² ÷ (2 × 36^L) ≤ 0.01, where 36 is the number of characters a code may use and n counts every id issued so far plus the new one. [Worked examples](#appendix-b-worked-examples) says how tests fix the codes `para` draws.

`para` keeps a record of every id it has issued, to entities and to [lineage](#113-lineage) records alike, so it never reuses one and can count them. Each id gets an empty file of that name in the root's `.para/ids/`, which is never deleted, even when what it named is gone. A code whose file already exists is drawn again, and the number of files is the n above. Branches that issue ids add files with different names, so git merges the record without a conflict. If two branches draw the same code by chance, their files merge into one and two holders share the id, as below. An entity whose id has no file, such as one copied by hand from another tree with the same prefix, still answers to its id; [`doctor`](#82-doctor-findings) reports it, and [repair](#83-repair) adds the file.

A command takes an id exactly as `para` prints it, never shortened. Every output names entities by id, [`list`](#74-reading) finds one by name, kind or place, and each README's frontmatter carries its entity's id ([READMEs](#53-readmes)). A command refuses an id it cannot use and says why ([output and exit codes](#710-output-and-exit-codes)):

- its prefix is not this tree's
- two entities or records hold it
- it no longer exists: nothing holds it, but its file is there, because what held it was removed or, for a stub, replaced by the archived entity
- it was never issued: nothing holds it and it has no file
- the command does not take its kind, as for a stub given to anything but `show` or `list`, or a record's id where an entity's is needed, or the reverse

A git merge or a hand copy of a folder can leave two entities or records with one id, and a folder copied in from another tree brings ids with another prefix. `doctor` reports both. [Repair](#83-repair) gives a new id from this tree to all but one holder of a duplicated id, and to every entity or record with another prefix. You choose which holder keeps a duplicated id. Repair is the only time an id changes. A stub's recorded id is a reference to the entity it stands for, so lookup and duplicate checks ignore it.

### 2.4 Adopting existing directories

Adopting makes a plain folder an entity. `para` adds a `.para/` and its parts of the README, moves nothing and changes none of your content, so it can take over a tree that already exists.

`adopt` takes the parent entity's id, a kind, and the names of one or more plain folders directly inside that parent; a name is enough there, since those folders are not entities yet. For example, it can make `cooking/` and `travel/` in `resources/` into dirs. Any facts the kind needs when created, such as a key result's due date, apply to every folder named. `adopt` checks every folder first, then adopts all of them or none ([safety guarantees](#81-safety-guarantees)).

Each adopted folder gets a new id and the files a new entity of its kind gets, with `para`'s part added to a README already there ([generated files](#117-generated-files)). Its subfolders stay plain. The adoption is an event in the new entity's journal and in its parent's.

Adopt parents first: a folder inside a plain folder waits until that folder is an entity ([kinds and containment](#22-kinds-and-containment)). In a project, a plain `objectives/` can be adopted as its container, and its subfolders then as objectives. `adopt` refuses, saying why, when:

- the kind is the root or a stub, or may not sit in that parent
- the name and kind do not match: a container's name is taken only by that container, and a container only by its name
- a name is given twice or contains `/`
- a named folder is missing, a file or a symlink, or holds something named `.para` at any depth
- a README already there is not a plain file, or its frontmatter already has a `para` key or is not valid YAML ([READMEs](#53-readmes))
- the parent is in the archive, which is read-only ([archive](#17-archive))

No file's level loosens and no lineage is lost. Once the folder is an entity, the parent's path rules stop covering its files. So adoption gives the new entity the strictest level those rules gave any file in it, and journals it ([where classifications are set](#63-where-classifications-are-set)). Files those rules did not cover can end up stricter. The parent's rules stay as they are, and [`doctor`](#82-doctor-findings) warns about any rule that now matches nothing, which you can then delete. Lineage records for files in the folder, at any depth, move to the new entity from the nearest ancestor holding them, with their ids unchanged, so whatever names them as inputs still does ([lineage](#113-lineage)). A container holds no records, so when the new entity is a container they stay where they are until each file's subfolder is adopted.

To bring in a whole tree:

1. Run [`init`](#73-init) in its top folder. After you confirm, or with `--force`, `init` makes that folder the root and makes any `projects/`, `areas/`, `resources/` and `archive/` already there buckets, creating those missing. It makes `projects/`, `areas/` and `resources/` inside `archive/` archive buckets even before anything is archived, and adds `para`'s part to any README or `AGENTS.md` already in these folders. It refuses where a folder could not be adopted, such as a symlinked bucket, and in a v0 tree points to [`migrate`](#91-migrating-a-v0-tree). Loose files in a bucket stay content that `doctor` reports.
2. Move any folder under `archive/` by hand to the matching live bucket, since nothing in the archive can be adopted.
3. Adopt the folders, parents first, running `doctor` after each round to list what is still plain, such as the subfolders of a newly adopted container.
4. [Archive](#25-archiving) again what came from `archive/`, once its subfolders are adopted and its level is set, since the archive is read-only.

### 2.5 Archiving

Archiving moves an entity out of the live buckets into `archive/`, with its history, and unarchiving brings it back ([archive](#17-archive)).

`archive` takes an entity's id and moves it, with everything beneath it, into the matching archive bucket, creating that bucket the first time. The entity lands under whatever in the archive stands for its live parent, matched by id: the parent's archived self, or a stub recording the parent's id. Where nothing does, `para` adds a stub for the parent, and so on up to the archive bucket, so an entity's archived path mirrors its live one. A stub recording the id of the entity or of anything beneath it is replaced by that entity, which keeps what the stub held and takes the stub's folder name, saying so when it differs from its own. A stub for a removed descendant has nothing to replace it, so it stays where it is, now inside an archived entity. `archive` refuses, saying why, when:

- the entity is the root, a stub or already archived
- the entity is a container, naming its entities and, where it can be archived, its parent
- a live entity uses an analytic, metric or term that would move, unless that user moves too ([measurement](#15-measurement))
- the level covering it would loosen, as when `projects/` is `RED` and `archive/projects/` has none, or a level it carries would be looser than the one above it in its new place ([classification](#112-classification)); tighten the archive's level or the entity's own, then run it again

`unarchive` takes an archived entity's id and moves it back under its live parent, with everything beneath it, including descendants archived on their own before it. Archived ancestors it needs come back live too. Stubs stay for anything still archived, such as what sits beneath the stub of a removed descendant, which has no live place to return to. `unarchive` refuses, saying why, when:

- the entity is live or a stub
- a level would loosen, as for `archive`
- it, or anything coming back with it, uses an archived entity that stays archived
- an ancestor was removed, so only its stub is left, naming that ancestor; give a new parent instead

A new parent is the id of a live entity to return under, such as another project for an objective. `para` files the entity in the right container under it, such as `objectives/` for an objective, creating the container if needed. A container or bucket given directly, such as `projects/` for a project, takes the entity itself. `unarchive` refuses if the entity's kind may not sit there ([kinds and containment](#22-kinds-and-containment)). Its archived ancestors stay archived.

When the path an arriving entity or stub would take is already held, `para` appends `-1` to its folder name, or `-2` and so on if that is held too, and says so. Names compare as the filesystem compares them, as for [reserved names](#22-kinds-and-containment). The suffix is then part of the name, so it stays when the entity is unarchived. For example:

- a new `launch-website`, archived after the first was, becomes `archive/projects/launch-website-1/`
- unarchiving it while a live `launch-website-1` exists makes `projects/launch-website-1-1/`

An entity's stubs, those recording its id, follow the live tree. Renaming or moving a live entity renames or moves its stubs to match, taking a suffix only if the new path is held, and adds stubs for new ancestors. A stub exists only while something archived sits beneath it, and `para` removes one left empty by any command. Removing a live entity leaves its stubs where they are, so what is archived beneath keeps its place and history.

A container, once created, stays when its last child leaves, whether moved, archived or removed. It goes only when its parent is removed, and moves with its parent otherwise. So its id, level and journal survive, and its stubs always match it.

Archiving and unarchiving are journaled like any move ([notes and the journal](#18-notes-and-the-journal)):

- each entity moved on its own, the one named and any ancestors brought back, gets an event in its own journal, as does each descendant archived on its own that `unarchive` brings back with it
- its old and new parent each get one, containers included, unless the parent is a stub, which has no journal
- entities moved beneath it get none

`archive` also stores the archive time in the state of every live entity it moves, descendants included ([`state.toml`](#32-statetoml)):

- An entity archived earlier keeps its own time.
- `unarchive` removes the time from each entity it brings back.
- An archived key result's or indicator's [status](#114-status) is computed as of that time.

A move takes the folder as it is. Symlinks inside move unchanged and are never followed, so a relative link that points outside the entity may break.

Besides the user commands the [archive](#17-archive) allows, `para` itself changes the archive only by the moves and stub upkeep above, by regenerating [generated files](#117-generated-files), by [repair](#83-repair) and by [`migrate`](#91-migrating-a-v0-tree).

## 3. Storage

### 3.1 Truth and projection

`para` keeps its truth in a few files in its `.para/` folders. The [generated files](#117-generated-files) it writes for readers are projections of that truth, never read back as truth, so each fact has one home and a generated file that disagrees with it is out of date.

The truth files are:

| File | Where | Holds |
| --- | --- | --- |
| `state.toml` | every `.para/` | the entity's facts, such as its kind, id, set status, lineage records and suppressions; the root's also holds the schema version and id prefix ([`state.toml`](#32-statetoml)) |
| `config.toml` | any `.para/` but a stub's, once something is set | your choices, such as a classification level, path rules and settings ([config](#35-config)) |
| `measurements.csv` | a metric's `.para/`, created with it | the metric's measurements, one row each ([recording measurements](#42-recording-measurements)) |
| the journal | every `.para/` but a stub's | the entity's events, notes included, only ever appended to ([the journal](#33-the-journal)) |
| `ids/` | the root's `.para/` | an empty file for every id issued ([addressing](#23-addressing)) |

State, config and a metric's measurements hold what is true now, and the journal holds what happened. A command that changes a fact, such as a status, writes the new value to state and appends the change to the journal. The value lives in one truth file, and the change in the journal.

Commands find current values in truth files, never by reading a journal, so a long history never slows one down. Some values depend on history, such as an entity's [attention](#18-notes-and-the-journal), and the command that changes one stores the result in state. A journal is read only:

- to show history or notes on request ([reading](#74-reading))
- by [`doctor`](#82-doctor-findings), [repair](#83-repair) and [`migrate`](#91-migrating-a-v0-tree)
- to reset attention after a correction ([the journal](#33-the-journal))

Nothing checks a truth file against its journal. After a hand edit or a merge, commands go by state, config and measurements, and the journal lacks the change.

A folder whose kind cannot be read from its `state.toml` is [damaged](#21-directory-shapes). So is one whose `config.toml` exists but cannot be read, since going on without it would drop its levels: commands that would change it or anything beneath it refuse, and reads warn. A missing `config.toml` means nothing is set there. A `measurements.csv` that cannot be read makes each command that writes to it refuse, naming the folder and pointing to `doctor`, and reads leave out the statuses that need it and warn. `doctor` reports any other fault in a truth file, such as a missing journal.

An edit inside a generated file changes no fact and is replaced at the next rewrite, so a fact changes only by its command. Anything else inside a `.para/` is `para`'s but is neither truth nor generated, and `doctor` reports it.

Everything outside `para`'s side is your content ([structure is para's, content is yours](#03-structure-is-paras-content-is-yours)), your own text in a README included. `para` reads it, for example to hash a recorded file or keep your text around its own part of a README, but never treats it as truth.

There is no truth or config outside the tree, such as in your home folder. Apart from the new ids it draws, and the system time zone that [`init`](#73-init) reads to set the tree's own ([time and the clock](#34-time-and-the-clock)), a command depends only on the tree, its arguments and the clock ([what para is](#01-what-para-is)). Its arguments include the current folder, any `PARA_*` variables that stand in for flags ([fields and time input](#72-fields-and-time-input)), and whether you are there to answer a confirmation, and how. The clock is set out in [time and the clock](#34-time-and-the-clock).

A hand edit to a truth file changes the truth itself, outside `para`'s promises ([safety guarantees](#81-safety-guarantees)); `doctor` reports what it breaks.

### 3.2 `state.toml`

Every `.para/` holds a `state.toml`, the one home of its entity's facts ([truth and projection](#31-truth-and-projection)). A fact is a value whose change would make something already recorded false, such as a kind or a status. Only commands write it: the command that makes each fact true, [repair](#83-repair) and [`migrate`](#91-migrating-a-v0-tree), never [`para config`](#79-config).

These keys come first, in this order:

| Key | Holds |
| --- | --- |
| `kind` | the entity's kind, which says how to read the rest ([directory shapes](#21-directory-shapes)) |
| `id` | the entity's id ([addressing](#23-addressing)) |
| `title` | a title such as `"Launch website"`, present only when set, on any entity but a stub |
| `description` | one line saying what the entity is for, such as `"Rebuild the marketing site on the new stack."`, present only when set, on any entity but a stub |
| `created` | when the entity was created, adopted or migrated, set once and never changed ([time and the clock](#34-time-and-the-clock)) |

A title is one line with at least one non-space character and no control characters, separate from the folder name: renaming one leaves the other. Clearing a title removes the key, and output then shows the folder name.

A description follows the same form as a title. Both are set and cleared by command, and each change is journaled. Clearing a description removes the key.

The root's `state.toml` also holds `schema`, the tree's schema version ([schema version](#93-schema-version)), and `prefix`, the prefix of its ids, which never changes. Having `kind = "root"` is what marks the tree ([the tree](#11-the-tree)).

```toml
# brain/.para/state.toml
kind = "root"
id = "brain-0a1b"
title = "Max's brain"
created = "2026-01-01T16:00:00Z"
schema = 2
prefix = "brain"
```

A stub's `state.toml` holds `kind`, `id` and `created`, then `stands-for` and `stands-for-kind`, the id and kind of the entity it stands for ([archiving](#25-archiving)).

Other keys are defined with the behaviour they serve:

| Facts | Kinds | Keys defined in |
| --- | --- | --- |
| set status, its time and due date | projects, objectives, key results | [status](#114-status), [targets](#43-targets-key-results-and-indicators) |
| blocked reason | projects, objectives | [status](#114-status) |
| attention time | every kind but the root, a container and a stub | [staleness](#45-staleness) |
| archive time | archived entities | [archiving](#25-archiving) |
| analytic, target (`at-least` or `at-most`), start and bounds | key results, indicators | [targets](#43-targets-key-results-and-indicators) |
| what it computes and the metrics, terms and sources it uses | analytics, metrics | [analytics, metrics and terms](#41-analytics-metrics-and-terms) |
| type, ref and direction | links | [links](#110-links) |
| lineage records | every kind but the root, a container and a stub | [lineage and trust](#62-lineage-and-trust) |
| a declassification | analytics; a derived work's sits in its lineage record | [how classifications flow](#64-how-classifications-flow) |
| suppressions | entities that can go stale, projects, objectives, key results, indicators | [suppression](#47-suppression) |

State never holds the entity's folder name, path or parent, since those are where its folder sits, nor any value [what is never stored](#55-what-is-never-stored) lists, nor anything chosen in config ([config](#35-config)).

`para` writes the file the same way every time, so the same facts give the same bytes and a diff shows only what changed:

- top-level keys first, in the order above, then the root's or stub's keys, then the kind's keys, following the rows of the table above and, within a row, the order its section gives
- then tables, such as lineage records, in the same order
- no comments, so any written by hand are dropped

[The example tree](#appendix-a-example-tree) shows the exact format, such as spacing and quoting.

A key `para` does not know, such as one added by hand, is kept with its value, written in the same format. An unknown top-level key goes after `para`'s and before the first table, an unknown key inside one of `para`'s tables stays in that table, and an unknown table goes after `para`'s. A key on a kind that does not take it, such as a `status` on a dir, counts as unknown. [`doctor`](#82-doctor-findings) reports unknown keys.

A `state.toml` that cannot be parsed, or whose kind cannot be read, makes a [damaged folder](#21-directory-shapes). A known key with a missing or bad value, such as a `created` that is not a time, makes each command that needs it refuse, naming the folder and pointing to `doctor`, which reports it. A bad [suppression](#47-suppression) is ignored instead. An entity without a valid `id` is skipped, with a warning, when a command looks for an id, and only commands that act on it refuse.

The root's `prefix` must be ASCII letters and digits. Otherwise every command but `doctor` refuses, since every new id would carry it, and [repair](#83-repair) restores it from the names of the issued ids in `ids/`. How `para` treats a `schema` it does not read, letting `migrate` upgrade an older one, is in [schema version](#93-schema-version).

### 3.3 The journal

Each entity's journal is `.para/journal.jsonl`, the log of everything done to it through `para` ([notes and the journal](#18-notes-and-the-journal)). Each line is one event: a JSON object on one line, in UTF-8, ending in a newline. Every event holds these fields, in this order:

| Field | Holds |
| --- | --- |
| `time` | when `para` wrote it ([time and the clock](#34-time-and-the-clock)) |
| `id` | the event's id, eight random lowercase letters and digits, unique within its journal, so a correction can name it |
| `type` | what happened, such as `created`, `note` or `status` |
| `sub` | the source id of the person accountable for it, missing only on events converted from a v0 tree, which carry `"v0": true` instead |
| `act` | present only when someone acted for that person: an object with the actor's source id as its `sub`, an optional `model`, `harness` and `session`, and an optional `act` of its own for a chain ([lineage](#113-lineage)) |
| `at` | the earlier time a backdated note or measurement claims, present only when given |

The type's own fields follow, in the order given by the section that defines that behaviour, as for state keys ([`state.toml`](#32-statetoml)). [The example tree](#appendix-a-example-tree) shows the exact format.

Every event names who did it, as [sources](#113-lineage), the entities that stand for people and agents:

- A command takes the accountable person with `--sub` and anyone acting for them with `--act`, or `PARA_SUB` and `PARA_ACT` when the flags are absent ([fields and time input](#72-fields-and-time-input)).
- `--act` repeats for a chain: the first acts for the `sub`, and each later one for the one before.
- Each must name a live source, not an archived one. A command that would append an event refuses without a `sub` or with an id that is not a live source.
- [`init`](#73-init) and [`migrate`](#91-migrating-a-v0-tree) take your name and create your source, which their own events then name.
- `para` does not check that the people named are who they say.

A note or a measurement can claim an earlier time than when it was written, such as a reading taken yesterday. The claimed time goes in `at` and can never be in the future. A note's can also not be before the entity's `created` time, so a backdated note cannot make old neglect look tended. Every other event is dated when it is written, and a command refuses `at` on it.

Where order matters, such as finding the newest attention note after a correction, events go by their claimed time, `at` or else `time`, and file order breaks ties. After a merge or a clock change, a later line can carry an earlier time, and that is not a fault.

A command appends its events only after its checks pass, in the same run as its changes to state ([safety guarantees](#81-safety-guarantees)). When one command writes to several journals, such as a move's entity and its old and new parents, its events share one `time`. Rewriting a generated file is never an event.

Every change made with [`para config`](#79-config) is an event in the journal of the folder whose `config.toml` changed, with the old and new values. The root's journal also records `init` and `migrate`.

`para` changes an event only when `migrate` converts old events to the current format, giving each an id and keeping its meaning and times. Giving a duplicated entity or record id a new one ([repair](#83-repair)) leaves old events as they are and appends one naming the old and new id. A journal is never trimmed, since it is the whole history ([history](#19-history)).

A wrong note is corrected with a `correction` event in the same journal. Its `of` field names the note's `id`, and it holds one of:

- `values`: the note's text, claimed time and attention mark, as corrected.
- `"withdrawn": true`, for a note that should never have been written.

A corrected claimed time cannot be later than the note's `time` or before the entity's `created` time. The correction itself is dated when written.

A correction replaces all of the note's values. The newest correction wins, so a later one can undo a withdrawal. The journal keeps the note and every correction, and history shows each one. A correction is not attention itself.

A measurement is corrected or withdrawn in `measurements.csv` itself, and the journal logs the old and new values ([recording measurements](#42-recording-measurements)). Its corrected claimed time can be any past time, as when it was recorded. Only notes take correction events.

Attention can change when you correct or withdraw a note that is, or becomes, an attention note, and when you correct or withdraw a measurement. The command then reads the journal and `measurements.csv` and resets the attention time ([notes and the journal](#18-notes-and-the-journal)).

A blocked reason in state is its own fact. Correcting the note it came from leaves it, and setting `blocked` again with a new note replaces it.

A line is readable when it is a JSON object with a `time` and a `type`. Problems are handled like this:

- **An unreadable line**, such as a git conflict marker or half a line from an interrupted write, makes each command that reads or appends to that journal refuse, naming the folder and pointing to [`doctor`](#82-doctor-findings). `doctor` still reads it.
- **A merge conflict**, from two branches that both appended, leaves unreadable lines, as above. You resolve it by hand, usually by keeping every event from both sides ([git](#37-git)).
- **A missing journal** is created at the next append.
- **An event of a type, or with a field, `para` does not know** is kept like any other.
- **An event with no `sub`** that is not marked `v0` is read like any other.
- **Two events with the same `id`**, which only a merge can cause, make a new correction naming that id refuse, and a correction already naming it applies to neither.
- **A correction that names a missing id**, or names an event that is not a note, is kept and changes nothing.
- **An event with no `id`**, such as one written by hand, is read like any other but cannot be corrected.
- **A journal that is a symlink** makes each command that would append to it refuse, so `para` never writes outside the tree ([safety guarantees](#81-safety-guarantees)).

`doctor` reports each of these.

### 3.4 Time and the clock

Every time `para` stores is a moment in UTC, truncated to the second and written like `2026-11-05T16:30:00Z`. That includes deadlines, such as a due date or the end of a suppression. [`doctor`](#82-doctor-findings) reports a stored time in any other form.

The tree has one time zone, the `time-zone` setting in the root's `config.toml`, an IANA name such as `America/Los_Angeles` ([config](#35-config)). It decides only how commands read and show times ([fields and time input](#72-fields-and-time-input)):

- A time you type without an offset is in the tree's zone, and one with an offset is converted. A local time that a daylight saving change repeats means its earlier moment, and one the change skips is refused.
- A date you type without a time means the last second of that day in the tree's zone, the later one if it repeats. In Los Angeles, `--due 2026-11-05` stores `2026-11-06T07:59:59Z`.
- Command output shows times in the tree's zone, and a deadline at the last second of a day in that zone as just that date. After the zone changes, an older deadline may show as a full time.

Changing the zone changes no stored time. Generated files, such as README frontmatter with its due dates, show full UTC times, so they never depend on the zone.

The setting can be made only on the root. Unset, or with no root `config.toml`, the zone is UTC. A name `para` does not know makes every command but `doctor` and `para config` refuse. [`init`](#73-init) sets it from `--time-zone`, or else from your system's zone, and says which. If it finds no zone, or one that is not an IANA name, it sets UTC and says so. Each `para` release carries its own copy of the time zone database, so a newer release can read or show a time differently.

Times compare as moments:

- Work is [overdue](#114-status) once the current time is past its due date. A key result that has not reached its target becomes `missed` at the same moment.
- A key result's value at its due date is its analytic's value computed at the due date, from measurements claimed at or before it ([analytics, metrics and terms](#41-analytics-metrics-and-terms)).
- A [suppression](#116-suppression) hides its flag until its end has passed.
- An entity is [stale](#45-staleness) once more than its `<kind>.stale-after` has passed since its attention time ([`state.toml`](#32-statetoml)).

A duration, such as `stale-after`, is a positive whole number and a unit: `m` for minutes, `h` for hours, `d` for days of 24 hours or `w` for weeks, such as `90m`, `36h` or `7d`. There is no unit for months. A bare number is refused, and so is one over `5200w`, about a hundred years.

Each command reads the clock once, when it starts. That one time is the `time` of every event it writes, and the current time for every check, status and flag, such as the rule that a claimed time cannot be in the future. The one exception is an archived key result or indicator, which uses the time it was last archived, alone or with an entity above it ([status](#114-status)).

`PARA_NOW` replaces that reading for one command, such as `PARA_NOW=2026-11-05T09:00:00Z`, so tests and [worked examples](#appendix-b-worked-examples) give the same result on every run. It is an environment variable with no command-line option.

An empty `PARA_NOW` counts as unset. Any other value must be a full time with an offset, `Z` included, which `para` converts to UTC and truncates to the second; otherwise every command refuses.

Anyone may use it. It adds no weakness, since stored times were never more trustworthy than the clock, which anyone can set.

A clock behind times already stored, as on a machine set wrong, is not a fault. Statuses and flags use that time as usual, so work may look not yet overdue.

### 3.5 Config

A `.para/config.toml` holds your choices for its folder and everything beneath it, such as a classification level or how long a project may go untended. A change applies from then on and never rewrites what is already recorded.

Any entity but a stub can have one, including the root and containers such as buckets. The file appears when the first setting is made there and goes when nothing is left in it. [`para config`](#79-config) reads and changes settings, and each change is an event in that folder's journal, with the old and new values ([the journal](#33-the-journal)). In the archive, `para config` can only tighten a level, lower a source's grade or apply a scale change ([archive](#17-archive)).

These are the settings:

| Setting | Where | Default | Defined in |
| --- | --- | --- | --- |
| `time-zone` | the root only | UTC | [time and the clock](#34-time-and-the-clock) |
| `classification-scale` | the root only | TLP 2.0 | [classification levels](#61-classification-levels) |
| the reliability scale | the root only | `A` to `F` | [lineage and trust](#62-lineage-and-trust) |
| `classification` and `path-classifications` | any entity but a stub | none | [where classifications are set](#63-where-classifications-are-set) |
| a source's grade | sources only | the lowest grade | [lineage and trust](#62-lineage-and-trust) |
| `<kind>.stale-after`, for each kind that can go stale | any entity but a stub | none (`init` writes two, below) | [staleness](#45-staleness) |
| `key-result.grace` | any entity but a stub | 0.1 | [derived status](#44-derived-status) |
| `key-result.pace` | any entity but a stub | 0.8 | [derived status](#44-derived-status) |
| `indicator.margin` | any entity but a stub | 0.1 | [derived status](#44-derived-status) |

A kind can go stale unless it is the root, a container or a stub.

A setting holds at the folder that sets it and at every folder beneath it, until a nearer folder sets it again. To find a value, `para` looks at the entity, then at each ancestor up to the root, containers included, and takes the first it finds. For example, if the root sets `area.stale-after = "90d"` and `areas/garden/` sets `"180d"`, the garden and its sub-areas use 180 days and every other area uses 90.

A source's grade does not pass down. It covers only that source.

A setting with nothing beneath it to apply to, such as `project.stale-after` on an area, is allowed and changes nothing, even when its value is bad.

A folder cannot cancel a setting it inherits. To keep a setting from one subtree, remove it from the ancestor, then set it again on each folder that should still have it.

Levels pass down the same way. A level is never looser than the one above it, and loosening one is a person's act ([how classifications flow](#64-how-classifications-flow)).

A path rule covers only files and plain folders in its own entity ([where classifications are set](#63-where-classifications-are-set)).

An archived entity looks up settings along its path in the archive, through `archive/` to the root:

- Its own `config.toml` moves with it.
- Settings on its old live ancestors no longer reach it, since they are not on its archive path.
- So archiving can change an indicator's margin, and with it the status frozen at archiving.
- A later change above it, such as to the root's margin, can also change that status.
- A level cannot loosen this way, since the move refuses ([classification](#112-classification)).

To keep a setting, set it on the entity before you archive it.

A root-only setting holds for the whole tree. `para config` refuses it anywhere else. If one is added elsewhere by hand, it changes nothing and [`doctor`](#82-doctor-findings) reports it. The same goes for a grade on an entity that is not a source.

A `config.toml` on a stub is ignored. `doctor` reports it, and `para` removes it with the stub.

[`init`](#73-init) writes three settings into the root's `config.toml`: the time zone ([time and the clock](#34-time-and-the-clock)) and two staleness settings, so review flags neglect from the start. For example:

```toml
time-zone = "America/Los_Angeles"

[project]
stale-after = "14d"

[area]
stale-after = "90d"
```

You can change or remove them like any setting. [`migrate`](#91-migrating-a-v0-tree) writes none, so a migrated tree keeps what it had.

A missing `config.toml` means nothing is set there. One that cannot be read makes its folder [damaged](#21-directory-shapes) ([truth and projection](#31-truth-and-projection)).

A value `para` cannot use, such as `stale-after = "7"` with no unit or `stale-after = 14` with no quotes, makes each command that needs it refuse. The command names the folder and points to `doctor`. `para` uses the nearest value even when it is bad. It never falls back to one from further up. An unknown `time-zone` is handled in [time and the clock](#34-time-and-the-clock).

A dotted key and a table are the same setting, so `project.stale-after = "14d"` at the top level means the same as `stale-after = "14d"` under `[project]`.

`para` writes `config.toml` the same way every time, as it does [`state.toml`](#32-statetoml):

- Top-level keys come first, in the order of the settings table. Each setting's section says whether it is a key or a table.
- Each kind's settings follow in a table of their own, such as `[project]`, in the order [directory shapes](#21-directory-shapes) lists the kinds. Inside one, `stale-after` comes first, then `grace` and `pace`, or `margin`.
- Other tables, such as path rules, come last, in the order of the settings table.
- An empty table is not written, and comments are dropped.
- An unknown key is kept with its value and placed as in `state.toml`: after `para`'s keys, inside `para`'s table if it was in one, or in its own table at the end. A setting out of place, such as `time-zone` below the root, is placed the same way. `doctor` reports both.

[The example tree](#appendix-a-example-tree) shows the exact format.

### 3.6 The walk

Each command starts with two walks. The walk up finds the tree's root. The walk down then finds the entities in it. Both follow the same rules every time, so every command sees the same set of entities.

`para` recognises a name only when it matches exactly, case included, on every filesystem. A folder named `.PARA` or `Projects/` is content, never `para`'s. Creating a name still collides as the filesystem sees it ([kinds and containment](#22-kinds-and-containment)).

The walk up starts at the current directory, with symlinks resolved, and checks it and then each parent, up to the filesystem root. At each folder it asks, in this order:

1. Does `.para/state.toml` have `kind = "root"`? Then this is the root, and the walk stops.
2. Does `.para/` hold `tree.toml`? Then this is a v0 tree, and the walk stops. Every command but [`migrate`](#91-migrating-a-v0-tree) refuses there and names `para migrate`.
3. Is the folder damaged ([directory shapes](#21-directory-shapes))? Then the command refuses, naming the folder and suggesting [`doctor`](#82-doctor-findings) run with `--root`, as below.
4. Otherwise the walk goes on to the parent.

A root whose `config.toml` cannot be read is damaged too, and the command refuses as in step 3. The walk up judges each folder as it finds it, so a stray `.para` in your content above the current directory also stops it. A command therefore never lands in a tree above a broken folder. To run one from inside a damaged folder, name the root with `--root`.

With no root found, the command refuses. It says no tree was found and suggests [`init`](#73-init) or `--root`. A current directory that cannot be resolved, such as one deleted, also refuses.

`--root <dir>`, or the `PARA_ROOT` variable, names the root directly, and there is no walk up. When both are given, the flag wins. An empty `PARA_ROOT` counts as unset.

- A relative path is read from the current directory, and symlinks are resolved, as in the walk up.
- The folder must be a root or a v0 tree, and a v0 tree still refuses every command but `migrate`. Anything else refuses, a damaged folder included.

`init` makes a root, so it refuses `--root` and ignores `PARA_ROOT`. Its walk up passes damaged folders, and it refuses only where it finds a root or a v0 tree.

The walk down starts at the root. It reads each entity's `.para/state.toml` and looks at each folder directly inside that entity:

- A folder holding anything named `.para`, or one `para` is denied when checking, is `para`'s: an entity, a nested root or a damaged folder.
- Any other folder is your content, and the walk never goes into it.

The walk also never goes into:

- a damaged folder, which reads leave out with a warning
- a nested root
- a `.para/`, where it never looks for entities
- a symlink, which is never followed, so a symlinked folder is content; a `.para` that is a symlink makes its folder damaged

A nested tree is a tree inside a folder of another tree. It comes from a copy, from `init` run above a tree kept in content, or from a hand edit, such as an entity's kind changed to `root`. The walk up stops at the nearest root, so a command run inside a nested tree works on that tree alone.

A nested root directly inside an outer entity is a root in the wrong place, which `doctor` reports ([kinds and containment](#22-kinds-and-containment)). So is a folder directly inside an outer entity whose `.para/` holds `tree.toml`, a nested v0 tree.

### 3.7 Git

Most trees live in a git repository. `para` never reads or writes `.git/` or checks for a repository ([scope](#05-scope-and-non-goals)). A `.git/` inside the tree is content.

`para` writes plain text files, each the same way every time, so a commit shows only what changed. It writes LF line endings, a whole file at a time, and reads LF, CRLF or a mix. In a README or `AGENTS.md` it writes only its own lines this way and keeps yours as they are ([para-owned blocks](#52-para-owned-blocks)). Before appending to a file whose last line has no ending, it adds one.

`para` never resolves a merge conflict: you resolve each one by hand. A merge can leave these behind:

- **A conflict in a journal**, most often a bucket's, since each create writes to its parent's journal ([the journal](#33-the-journal)).
- **A conflict in `state.toml` or `config.toml`.** The markers make the file unreadable, so its folder is [damaged](#21-directory-shapes). A conflict in either of the root's files stops every command, `doctor` included, until you resolve it ([the walk](#36-the-walk)).
- **A conflict in `measurements.csv`**, handled as [recording measurements](#42-recording-measurements) describes.
- **A conflict in a README or `AGENTS.md`**, common in a bucket's for the same reason. Markers inside `para`'s block go when [`rebuild`](#78-rebuild) rewrites the block, since it is generated. Markers anywhere else, or across the edge of the block, are yours to fix; in frontmatter they make the YAML invalid, and `doctor` reports them ([para-owned blocks](#52-para-owned-blocks)).
- **A folder left without its `state.toml`**, as when one branch removes an entity and the other changes a file in its `.para/`. The folder is damaged.
- **Two entities with one id**, as a merge or a copy can leave ([addressing](#23-addressing)).
- **A structure that breaks the rules**, such as a project inside a project ([kinds and containment](#22-kinds-and-containment)).
- **Changes from both branches joined in one `state.toml`** without a conflict, which can break its rules. `doctor` reports them, or the file becomes unreadable.
- **A branch that ran [`migrate`](#91-migrating-a-v0-tree) merged with one that did not.** Migrate on one branch and merge it before other branches write.
- **Generated files that are out of date**, which `rebuild` rewrites.

After a merge, run [`doctor`](#82-doctor-findings) to see what it left, fix what it reports, then run `rebuild`.

`para` writes no `.gitattributes`. You can add these lines to one at the root of the tree:

```text
**/.para/journal.jsonl merge=union
* -text
```

- The first makes git keep both sides of a journal conflict, so you rarely resolve one by hand. It can leave repeated lines, half lines and both versions of an event that `migrate` rewrote. `doctor` reports each ([the journal](#33-the-journal)).
- The second turns off line-ending conversion for every file in the tree, yours included, since a [recorded file](#113-lineage) can be any file and its hash uses its exact bytes. Files already checked out keep their endings until you check them out again.

## 4. Progress and attention

### 4.1 Analytics, metrics and terms

References in the measurement library run one way, from key results and indicators to analytics, then to metrics, then to terms and sources, so none can loop back ([measurement](#15-measurement)). A metric's `terms` key lists the ids of the terms it uses, and its `sources` key the ids of the sources its values come from ([lineage](#113-lineage)). Each is present only when it names any.

An analytic computes one number with a `formula` over metrics, in the style of a [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) query. This one gives the share of visitors who came from search over the last 28 days:

```text
sum(brain-7k2q[28d]) / sum(brain-9p3w[28d])
```

A value is computed for a time: now, except for a key result past its due date and an archived key result or indicator ([derived status](#44-derived-status)). A function takes one metric and a range in brackets, a [duration](#34-time-and-the-clock) such as `28d`. The range covers measurements claimed after the time minus the duration and at or before the time:

| Function | Value over the range |
| --- | --- |
| `sum` | the total, added in the order of [`measurements.csv`](#42-recording-measurements) |
| `avg` | the sum divided by the count |
| `min` | the smallest |
| `max` | the largest |
| `count` | how many measurements there are |

A metric's id alone takes its newest measurement claimed at or before the time, however old. "Newest" goes by claimed time, then event id, then place in the file.

A formula combines these and numbers with operators and parentheses, as the grammar below shows, with `*` and `/` before `+` and `-`, and names at least one metric:

- An id is written as `para` prints it ([addressing](#23-addressing)), and names match exactly, case included. Text that starts with the tree's prefix and a `-` is read as an id, so with an all-digit prefix such as `5`, write `5 - 1234` with spaces.
- A number is unsigned, below 10^15, so every `-` is an operator.
- A range must be more than zero.
- Spaces and tabs may sit only between these parts.

```text
formula  = term { ("+" | "-") term }
term     = factor { ("*" | "/") factor }
factor   = "-" factor | number | id | function "(" id "[" duration "]" ")" | "(" formula ")"
function = "sum" | "avg" | "min" | "max" | "count"
number   = digits [ "." digits ]
```

`para` stores the formula in one form: one space on each side of a binary operator, none elsewhere, numbers as a [measurement's value](#42-recording-measurements) is stored, and durations without leading zeros. So `sum( brain-7k2q[028d] )/100.0` is stored as `sum(brain-7k2q[28d]) / 100`. A formula that does not parse is refused.

`count` of no measurements is 0, and every other function, or an id alone, with none has no value. A formula has no value when any part of it has none, when it divides by zero, or when any step gives a result that is not finite. It computes with 64-bit floats, rounding after every operation, never fusing a multiply and an add, so every machine gives the same result.

The analytic's one key of its own is `formula`. In this abridged example:

```toml
kind = "analytic"
id = "brain-4n8r"
title = "Share of visitors from search"
created = "2026-01-05T17:00:00Z"
formula = "sum(brain-7k2q[28d]) / sum(brain-9p3w[28d])"
```

### 4.2 Recording measurements

A metric's measurements live in `.para/measurements.csv`, the one home of its data ([truth and projection](#31-truth-and-projection)). `para` creates the file with the metric, holding only its header. With two rows it reads:

```csv
event,at,value,note
q7m2k9xa,2026-03-01T07:00:00Z,6.5,
b3n8w1pz,2026-03-02T07:00:00Z,7.25,"late night, then slept in"
```

- `event` is the id of the journal event that recorded the row, kept when the row is corrected ([the journal](#33-the-journal)).
- `at` is the measurement's claimed time, in UTC to the second ([time and the clock](#34-time-and-the-clock)).
- `value` is the number.
- `note` is an optional note, empty when there is none. It is valid UTF-8 with no control characters, such as a line break or a tab, and its spaces are kept exactly. `para` puts it in double quotes when it holds a comma or a double quote, doubling any double quote inside.

Rows are sorted by `at`, then by `event`, and the file is written the same way every time. A row that sorts last is appended to a file already sorted and in stored form. Any other change rewrites the whole file.

A value is typed as an optional `-`, digits, and optionally a `.` and more digits, such as `7.25`, `-3` or `007.50`, so `+7`, `.5`, `5.`, `1,000`, `1e3`, NaN and infinities are refused. Its magnitude, once stored, must be below 10^15. `para` stores the nearest 64-bit float, written as the shortest decimal that reads back to it, with no exponent: `7.5`, `0.1`, `1200`, and `0` for minus zero.

Recording a measurement adds its row, with a note if you give one, and logs it in the metric's journal, with who recorded it. Its claimed time is now unless you give an earlier one, which may fall before the metric was created, as when loading old readings. It is never in the future.

Correcting a measurement changes its value, its note or its claimed time, which may again be any past time. Withdrawing one deletes its row ([the journal](#33-the-journal)). After the last withdrawal, the file holds only its header. An archived metric's measurements cannot be recorded, corrected or withdrawn until you unarchive it.

When reading the file, `para` ignores a UTF-8 byte order mark and blank lines and accepts a field in double quotes. A value in another accepted form, such as `7.50`, is read and rewritten in stored form at the next write. Hand edits and merges can leave these behind:

- **A file that cannot be read**, such as one with a bad header, a row without four fields, a bad time or value, or conflict markers, makes writes to the metric refuse. Reads leave out the statuses that depend on it and warn ([truth and projection](#31-truth-and-projection)).
- **A missing file** means no measurements. [`doctor`](#82-doctor-findings) reports it, and the next recording creates it.
- **A row with an empty `event`**, such as one you typed in, counts. `doctor` reports it, and [repair](#83-repair) gives it one.
- **Rows sharing an `event`** all count. A correction or withdrawal naming that event refuses, and `doctor` reports them.
- **Rows out of order** are read as they are and sorted at the next write.
- **A merge conflict** is yours to resolve, usually by keeping both sides' rows, which `para` sorts at the next write ([git](#37-git)).

### 4.3 Targets: key results and indicators

A key result and an indicator each hold one analytic to a target or a bound, naming it by id in `analytic` ([analytics, metrics and terms](#41-analytics-metrics-and-terms)).

A key result's keys follow, in this order, among the rest ([`state.toml`](#32-statetoml)):

| Key | Holds |
| --- | --- |
| `status` | `dropped`, present only when you set it ([key results](#132-key-results)) |
| `status-since` | when it was dropped, present with `status` ([status](#114-status)) |
| `due` | its due date, required ([time and the clock](#34-time-and-the-clock)) |
| `analytic` | the analytic's id |
| `at-least` or `at-most` | the target, exactly one of the two |
| `start` | the time progress is measured from, in the form of `created`, present only when you give one; otherwise the key result's `created` |

The start value is the analytic's value at the start time. If it has none then, it is the analytic's value at the earliest later claimed time, up to the time the status is computed for ([derived status](#44-derived-status)), among the measurements of the metrics it names, at which it has one. It is computed from the metrics' history each time and never stored, so a backdated measurement can change it. The start time is never in the future and is before the due date ([time and the clock](#34-time-and-the-clock)). It may be before the key result was created, such as the first day of a quarter.

A live key result may not be due later than its objective or its project. Any command that would make it so refuses, such as setting the key result's due date, giving the objective or project a due date or an earlier one, or moving or unarchiving a key result, or an objective holding one, under an objective or project due earlier. Work with no due date sets no limit. [`doctor`](#82-doctor-findings) reports a later due date left by a hand edit.

The key gives the direction. `at-least = 200.0` is met once the analytic is 200 or more, such as weekly signups. `at-most = 2.0` is met once it is 2 or less, such as page load time in seconds. [Derived status](#44-derived-status) turns the target, the start and the due date into a status.

An indicator's keys are `analytic` and one or both of `at-least` and `at-most`, the bounds its analytic must stay within. With both, `at-least` must be below `at-most`, so together they give a range. A hand edit that breaks this is a bad value. It has no window of its own. Any analytic will do, and one whose functions take a range, such as the last 28 days, makes the indicator roll. In this abridged example:

```toml
kind = "indicator"
id = "brain-6t1m"
created = "2026-02-01T18:00:00Z"
analytic = "brain-8d5h"
at-least = 7.0
```

Targets and bounds take the form of a [measurement's value](#42-recording-measurements), written as TOML floats, so a whole number gains `.0`. Reading accepts any finite TOML integer or float whose magnitude is below 10^15.

You set each of these keys, and an analytic's, by command when you create the entity, and can change them later. Each change is journaled. An id naming a missing, archived or wrong-kind entity is refused ([measurement](#15-measurement)).

A hand edit that leaves a value in the wrong form is a bad value ([`state.toml`](#32-statetoml)). An id whose entity was later removed is reported by [`doctor`](#82-doctor-findings), and [derived status](#44-derived-status) says what the status then shows.

### 4.4 Derived status

`para` works out a key result's or an indicator's status each time it is shown, and never stores it ([status](#114-status)). It computes for one time: now, or for an archived one, the time it was last archived ([archiving](#25-archiving)). Every step uses 64-bit floats, as a formula does ([analytics, metrics and terms](#41-analytics-metrics-and-terms)).

A key result's status takes four values:

- **Current** is the analytic's value at the time. Once the due date has passed, it is the value at the due date.
- **Progress** is how much of the way from the start value to the target is covered: (current − start value) ÷ (target − start value). It is 0 at the start value and 1 at the target. If the start value already meets the target, progress is 1 while the target is met and 0 otherwise. If the analytic has no current value or no start value, progress is 0.
- **Elapsed** is how much of the time from the start time to the due date has passed: 0 at the start time, 1 at the due date.
- **Pace** is progress ÷ elapsed, or 0 when that is negative. A pace of 1 keeps to a straight line from the start value to the target. Pace has no value when elapsed is 0, or when a hand edit leaves the start time at or after the due date.

The first rule that applies gives the status:

1. `dropped`, if you set it.
2. After the due date: `achieved` if current meets the target, otherwise `missed`.
3. `achieved`, if current meets the target.
4. `at-risk`, if elapsed is at least `key-result.grace` and pace has a value below `key-result.pace`.
5. `on-track`.

For example, weekly signups start at 100 with `at-least = 200.0`, due in ten weeks. At week five, elapsed is 0.5. With 150 signups, progress is 0.5 and pace 1, so it is `on-track`. With 130, pace is 0.6, below the built-in 0.8, so it is `at-risk`.

The grace keeps a new key result from being `at-risk` before anyone could have made progress. Both settings are found like any setting ([config](#35-config)). Each is a TOML integer or float from 0 to 1, and anything else is a bad value. A pace setting of 0 means never `at-risk`.

A due date may be in the past, as when you record last quarter's results. The status then settles at once. A measurement backdated to before the due date can still change a settled status ([recording measurements](#42-recording-measurements)).

An indicator has a margin at each bound: `indicator.margin` times the size of that bound, ignoring the sign. The setting takes the same form as the key result settings. The first rule that applies gives the status:

1. `no-data`, if the analytic has no value at the time.
2. `breached`, if the value is below `at-least` or above `at-most`.
3. `at-risk`, if the value is below `at-least` + its margin, or above `at-most` − its margin.
4. `healthy`.

For example, with `at-least = 7.0` and the built-in margin of 0.1, 6.9 is `breached`, 7 up to just under 7.7 is `at-risk`, and 7.7 is `healthy`. With `at-most = 2.0`, anything above 1.8 up to 2 is `at-risk`. With both `at-least = 7.0` and `at-most = 9.0`, 7 up to just under 7.7 and anything above 8.1 up to 9 are `at-risk`.

A margin setting of 0 means never `at-risk`, and a bound of 0 has no margin. In a narrow range the two margins can overlap, leaving no `healthy` value until you lower the margin.

When the analytic, or a metric its formula names, has been removed, the analytic has no value, so a key result's progress is 0 and an indicator is `no-data`. [`show`](#74-reading) names the missing id, and `doctor` reports it.

### 4.5 Staleness

An entity is stale once more than its `<kind>.stale-after` has passed since its attention time ([time and the clock](#34-time-and-the-clock)). Review reports it in the stale group ([review](#115-review)), and `show` and `list` mark it.

Only a live entity outside closed work can be stale. Nothing in the archive is stale, and neither is closed work or anything beneath it, such as a dir in a `done` project ([status](#114-status)).

Every kind but the root, a container and a stub has its own `<kind>.stale-after`, such as `project.stale-after` or `dir.stale-after`. A kind is never stale until a setting reaches it. [`init`](#73-init) writes two ([config](#35-config)).

Like any setting, `dir.stale-after` on a project covers that project's dirs, and on `resources/` it covers only resources.

The attention time is the `attention` key in the entity's [`state.toml`](#32-statetoml), a time in UTC. Creating or adopting the entity sets it to `created`, and [`migrate`](#91-migrating-a-v0-tree) works it out from the v0 history. After that it moves only forward, to the claimed time of:

- a note on the entity marked as attention ([notes and the journal](#18-notes-and-the-journal))
- for a metric, a measurement

A note or measurement claimed before the attention time leaves it as it is. A key result or indicator counts only its own notes, since measurements belong to the metric.

The one exception is a correction or withdrawal of an attention note or a measurement, which resets it ([the journal](#33-the-journal)). It then becomes the latest of `created`, the claimed times of notes marked as attention, and for a metric the claimed times in `measurements.csv`. Each note counts as its latest correction leaves it, and a withdrawn note does not count.

Nothing else counts: a status, due date or other change, a suppression, unarchiving, or anything on another entity. An entity archived long ago therefore comes back stale. Add a note marked as attention when you pick it up again.

A sub-area's notes never keep its parent fresh. An area with no work of its own, such as `health/` holding `fitness/` and `sleep/`, needs notes of its own or a longer `area.stale-after` set on it.

### 4.6 Review groups

Review reports each flag in its group, and ranks each group so the worst comes first ([review](#115-review)). Every entry names the entity by its id and path, or for an unchecked file its record ([addressing](#23-addressing)).

| Group | Each entry shows | Order within the group |
| --- | --- | --- |
| blocked | the reason and how long it has been blocked | longest blocked first, then soonest due |
| overdue | the due date | furthest past due first |
| key results | the status, the analytic's value or none, the target and the due date | `missed`, then `at-risk`, each earliest due first |
| indicators | the status, the analytic's value or none, and the bounds | `breached`, then `no-data`, then `at-risk` |
| stale | the attention time, the setting that flagged it and the folder that sets it | longest past its `stale-after` first |
| unchecked | the file and the live files made from it, or the metric and the live key results and indicators that use it | by path |

Ties go by path. Paths compare one name at a time, by the bytes of each name in UTF-8. A path comes before any longer path that starts with it, so a folder comes before everything in it.

An entry whose analytic or metric was removed names the missing id ([derived status](#44-derived-status)).

If a stale entry looks wrong, history shows whether a note lacks its attention mark ([reading](#74-reading)).

### 4.7 Suppression

A suppression keeps one flag on one entity out of review until its end ([suppression](#116-suppression)). It is in force until its end has passed. An entity has at most two flags to suppress:

- `stale`, on an entity that can go stale ([staleness](#45-staleness))
- `status`, on a project or objective while it is `blocked`, or on a key result or an indicator, for the status it shows when suppressed ([status](#114-status))

`suppress` sets one, with an end and a note, and `unsuppress` lifts one early, with a note ([writing](#76-writing)). Both refuse an entity in the archive, closed, or beneath closed work ([status](#114-status)). `unsuppress` also refuses when no suppression is in force. `suppress` also refuses:

- an end that is not after the current time
- `status` on an entity whose status is not flagged now
- `stale` when no `<kind>.stale-after` reaches the entity

A date alone means the end of that day, so today's date works ([time and the clock](#34-time-and-the-clock)). Suppressing a flag that is already suppressed replaces its end and note. Setting, replacing and lifting are each an event in the entity's journal, and none is attention.

While a suppression is in force, review leaves out its flag:

- A `stale` suppression leaves out staleness.
- A `status` suppression leaves out its status whenever the entity shows it. One set while `at-risk` still leaves it out if the key result goes `on-track` and back, but `missed` is reported.

`show` and `list` still give the status or the stale mark and say it is suppressed.

After its end, commands treat a suppression as absent. It stays in `state.toml` until a command next rewrites the file, which drops it with no event. A rewrite keeps a table it cannot use as it is.

A suppression survives closing, reopening, archiving and unarchiving. It applies again while the entity is live and outside closed work.

Review ends with the number of flags it leaves out now. On request, it lists every suppression in force on a live entity outside closed work, with its note and end, and says whether it leaves out a flag now ([`review`](#77-review)).

Each suppression is a table in the entity's [`state.toml`](#32-statetoml), `stale` before `status`:

```toml
[suppressions.stale]
until = "2027-03-01T07:59:59Z"
note = "Garden is dormant until spring"

[suppressions.status]
status = "at-risk"
until = "2026-12-31T07:59:59Z"
note = "Launch slipped a month"
```

A table `para` cannot use, such as one with a bad `until`, or a `status` the entity cannot be suppressed for, is ignored, unlike other bad values ([`state.toml`](#32-statetoml)), so its flag shows. [`doctor`](#82-doctor-findings) reports it, and `suppress` on that flag replaces it.

Two branches that suppress the same flag leave the table twice, or conflict markers, after a merge. Either way the file cannot be parsed, so the folder is damaged until you fix it by hand ([git](#37-git)).

## 5. Generated files

### 5.1 Write-through

A command that changes a fact rewrites every generated file that shows it, in the same run. It writes them after its checks pass, along with its changes to the truth files ([safety guarantees](#81-safety-guarantees)). Nothing is deferred, and no option skips it.

A change reaches these files:

| Change | Files rewritten |
| --- | --- |
| a fact of the entity's own, such as its status or due date | its README |
| its id, title or folder name | its README and its parent's |
| its description | its README |
| an entity or stub arriving or leaving | the README of each parent it joins or leaves |
| the level that covers it | the README of the entity, of its parent and of every entity beneath it |
| the classification scale | the root's `AGENTS.md`, and every README whose level or `[classified]` lines change |

A new entity gets its README. [`init`](#73-init) also writes the `AGENTS.md` of the root and each bucket, and a new archive bucket gets its own. A file whose bytes would not change is left alone.

A missing README or `AGENTS.md` is recreated whenever `para` would rewrite it, and [`doctor`](#82-doctor-findings) reports it until then. If a symlink, a folder or a file `para` cannot read stands where it must write, the command refuses before writing anything, naming it, so `para` never writes outside the tree. Replace it with a plain file or delete it.

A level set on the root reaches every README in the tree, so one README anywhere beneath that is a symlink, unreadable or broken ([para-owned blocks](#52-para-owned-blocks)) stops the change. `doctor` lists them all.

[`rebuild`](#78-rebuild) rewrites every generated file from the truth files. For the same truth, write-through and `rebuild` write the same bytes, so `rebuild` right after a command changes nothing. Run it after a hand edit or a merge ([git](#37-git)), and after upgrading `para`, since a new release may word or lay out these files differently and write-through rewrites only the files a change reaches. `doctor` reports any generated part that differs from what `rebuild` would write.

Rewriting a generated file is never a journal event ([the journal](#33-the-journal)). Files in the archive are rewritten like any others ([archiving](#25-archiving)).

### 5.2 Para-owned blocks

`para` shares two kinds of file with you, `README.md` and `AGENTS.md`. In each it writes one block between two marker lines:

```markdown
<!-- para:begin -->
...
<!-- para:end -->
```

The block is `para`'s, marker lines included. Everything outside it is yours, apart from a README's `para` frontmatter key ([READMEs](#53-readmes)). `para` writes its own lines with LF endings and keeps your bytes exactly as they are, line endings included, so a CRLF file ends up mixed ([git](#37-git)).

A begin marker is a line that starts with `<!-- para:begin` and ends with `-->`. An end marker is the line `<!-- para:end -->`. Spaces at either end of a marker line are ignored, so a later release can reword the begin marker and still find the blocks an earlier one wrote. A marker line counts wherever it is, even inside a fenced code block of yours, so quote a marker with other text on its line.

A file with no block gets one at the top, after any frontmatter, followed by a blank line. An existing block is rewritten where it is, so a block you move stays where you put it.

`para` finds the block before it writes anything, and refuses rather than guess when:

- a marker has no partner
- there are two begin markers or two end markers
- the end marker comes before the begin marker

Only a command that must rewrite the file refuses. It names the file and points to `doctor`, which reports it. Fix the markers by hand, or delete the leftover ones and let `para` add a fresh block.

Text written inside the block is replaced at the next rewrite. A git conflict inside the block goes the same way when `rebuild` rewrites it. A conflict outside the block is yours to fix, and `doctor` reports it ([git](#37-git)).

[Removing](#76-writing) an entity takes `para`'s parts out of its README. It deletes the file only when nothing else is left but blank lines and empty frontmatter.

### 5.3 READMEs

Every entity except a stub has a `README.md`, the main place a reader without `para` sees its facts. `para`'s parts of it are the `para` key in its frontmatter and the block right after the frontmatter. Your own frontmatter keys sit beside the `para` key, and your text follows the block:

```markdown
---
tags: [web]
para:
  kind: "project"
  id: "brain-7k2q"
  title: "Launch website"
  description: "Rebuild the marketing site on the new stack."
  classification: "AMBER"
  status: "in-progress"
  status-since: "2026-10-02T16:00:00Z"
  due: "2026-12-01T07:59:59Z"
---
<!-- para:begin -->
# Launch website

Rebuild the marketing site on the new stack.

## Contents (para)

- [designs/](designs/) dir "Mockups" brain-8k1p
- [links/](links/) container brain-2c6v
- [objectives/](objectives/) container brain-3f9a

History: [.para/journal.jsonl](.para/journal.jsonl)
<!-- para:end -->

The old site is in `archive/`. Sam owns the copy.
```

The `para` key holds a map of what the entity is and where it stands. Its keys are `kind`, `id`, `title` and `description` when set, and `classification` when one covers it, then these where the entity has them, with the key names and order of [`state.toml`](#32-statetoml):

- its set status, `status-since`, blocked reason and due date ([status](#114-status))
- its archive time ([archiving](#25-archiving))
- the keys that define its kind: a key result's or indicator's analytic, target or bounds and start ([targets](#43-targets-key-results-and-indicators)), an analytic's formula and a metric's terms and sources ([analytics, metrics and terms](#41-analytics-metrics-and-terms)), and a link's type, ref and direction ([links](#110-links))

`classification` is the level that covers the entity, left out when there is none ([classification](#112-classification)). An indicator with a range shows both bounds.

The map leaves out `created`, attention, suppressions, lineage records and a source's grade, and every computed value, such as a key result's status or an analytic's value ([what is never stored](#55-what-is-never-stored)). [`show`](#74-reading) gives all of them.

Strings are in double quotes, times are in UTC as stored, and numbers are written as in `state.toml`. [The example tree](#appendix-a-example-tree) shows the exact format.

Frontmatter is the lines between a `---` on the file's first line, after any byte order mark, and the next `---` line. `para` keeps a byte order mark ahead of it. The `para` key runs from its line, which starts at the first column, to the next line that does too and is not blank. `para` rewrites only those lines, so your other keys keep their exact bytes, comments and order included. With no `para` key, `para` adds one at the end of the frontmatter, and a README with no frontmatter gets one at the top. A command that must rewrite the file refuses, as for a broken block ([para-owned blocks](#52-para-owned-blocks)), when:

- the frontmatter is not valid YAML, or has no closing `---`
- the `para` key appears twice
- its value is not a map written one key per line, or an anchor or alias touches it

The block opens with `# ` and the entity's title, or its folder name when no title is set, so it follows a rename or a new title. The description follows when set. Then comes the heading `## Contents (para)`, and a list follows with one line for each entity directly inside, stubs included, in path order ([review groups](#46-review-groups)). Each line gives the child's folder name as a relative link, its kind, its title in quotes when set, and its id. A child whose level is stricter than the entity's shows `[classified]` in place of its title ([what para does with classifications](#65-what-para-does-with-classifications)). Links and titles are escaped where a name or title would break them. Then come a link to the entity's journal and, for a metric, a link to its `measurements.csv`. An entity with nothing inside has no list. [Adopting](#24-adopting-existing-directories) puts the block above your existing text, so you may want to delete a heading of your own that repeats the title.

### 5.4 `AGENTS.md`

`para` writes a block in `AGENTS.md` at the root, in each bucket and in each archive bucket, and nowhere else. Many agent tools read these files, and together they cover every rule of the tree, so more files would only repeat them. Your own instructions outside the block are kept ([para-owned blocks](#52-para-owned-blocks)).

The root's block tells an agent:

- what the PARA method is and what each bucket holds ([the PARA method](#02-the-para-method))
- which parts of the tree are `para`'s and which are yours ([structure is para's, content is yours](#03-structure-is-paras-content-is-yours))
- to change the structure only with `para`'s commands, naming the person it acts for with `--sub` and itself with `--act` ([the journal](#33-the-journal))
- to write an entity's id alone when referring to it, and to run `para show` to see what an id names ([addressing](#23-addressing))
- to mark a note as attention only when the upkeep it describes happened ([staleness](#45-staleness))
- each level of the tree's scale, what it allows, and how to handle classified content ([what para does with classifications](#65-what-para-does-with-classifications))
- after a git merge, to run `doctor`, fix what it reports, then run `rebuild` ([git](#37-git))

A bucket's block says what the bucket holds and which kinds may sit where in it ([kinds and containment](#22-kinds-and-containment)), and points to the root's block for the rest. An archive bucket's block also says that the archive is read-only and what a stub is ([archive](#17-archive)).

The blocks hold rules, never facts about entities, so they change only when `para` is upgraded, and the root's also when the classification scale changes. [The example tree](#appendix-a-example-tree) gives their exact text.

### 5.5 What is never stored

These values follow the clock, measurements or file contents, so `para` works them out whenever they are needed and never stores them, in truth files or generated files ([principles](#06-principles)):

- key result and indicator statuses, apart from a `dropped` you set ([derived status](#44-derived-status))
- an analytic's value, and a key result's progress, pace and start value
- whether work is overdue or stale ([staleness](#45-staleness))
- a file's trust, its reliability and whether it is checked, and a computed value's reliability ([lineage and trust](#62-lineage-and-trust))
- a derived work's level and the level of a computed value ([how classifications flow](#64-how-classifications-flow))

A stored copy would go wrong with no command run, and a generated file would go out of date with no change made ([generated files](#117-generated-files)).

These are also worked out, not stored, but change only through a command, so a generated file may show them:

- whether work is closed ([status](#114-status))
- the level and settings that reach an entity, which `config.toml` holds only where they are set ([config](#35-config))
- an entity's folder name, path and parent, which are where its folder sits ([`state.toml`](#32-statetoml))

A value that history decides, such as attention, is a fact that its command stores in state ([truth and projection](#31-truth-and-projection)).

## 6. Classification and lineage

### 6.1 Classification levels

A tree has one scale of classification levels, ordered from loosest to strictest. A level later in the scale is stricter. Content no level covers is unclassified, which ranks below every level ([unclassified content](#66-unclassified-content)).

By default the scale is [TLP 2.0](https://www.first.org/tlp/), whose levels say who may receive content:

| Level | Who may receive it |
| --- | --- |
| `CLEAR` | anyone, with no limit |
| `GREEN` | your community, but not through public channels |
| `AMBER` | your organization and its clients, as they need it |
| `AMBER+STRICT` | your organization only |
| `RED` | only the people it was given to |

TLP also says that sharing more widely needs permission from whoever gave the content. It defines no inheritance and no rule for content made from other content, so `para` adds both ([how classifications flow](#64-how-classifications-flow)).

The root's [config](#35-config) can replace TLP with a scale of your own. Each level is a `[[classification-scale]]` entry, loosest first, with a `name` and what it `allows`:

```toml
[[classification-scale]]
name = "PUBLIC"
allows = "Anyone, anywhere."

[[classification-scale]]
name = "INTERNAL"
allows = "Staff only. Never in public repositories."

[[classification-scale]]
name = "SECRET"
allows = "Only the people it was given to."
```

A custom scale replaces all of TLP's levels. `para` writes its entries in scale order, `name` before `allows`. `para config` refuses a scale where:

- a level has no `name` or no `allows`
- a name holds anything but ASCII letters, digits, `+`, `-` and `_`
- two levels share a name

Names match exactly, case included. With no `[[classification-scale]]`, the tree uses TLP, and `para` writes nothing for it.

The root's `AGENTS.md` lists each level with what it allows: TLP's meanings above, or each `allows`. So a level means one thing everywhere in the tree ([`AGENTS.md`](#54-agentsmd)).

Changing the scale maps every old level to a new one. You give [`para config`](#79-config) a file holding the new scale and a `[classification-mapping]` that sends each old level to a new level and says why. The file is input to that one command, not a `config.toml`. Level names are quoted keys:

```toml
[classification-mapping]
"CLEAR" = { to = "PUBLIC", reason = "Same audience." }
"AMBER" = { to = "INTERNAL", reason = "Clients sign NDAs, so they count as contractors." }
"AMBER+STRICT" = { to = "INTERNAL", reason = "No content needed the stricter split." }
```

`para` never stores the mapping in config. Once applied, only the root's journal event keeps it. A level that keeps its name needs no `[classification-mapping]` entry unless its `allows` changes, when it needs one with a reason. A mapping key may name any level in use, even one the old scale lacks. `para` refuses the change when:

- a level in use has no mapping, where levels in use are those set anywhere, archived entities included: an entity's `classification`, a path rule, and a declassification's level and the levels its notes were bound at
- the mapping breaks the order: an old level stricter than another must map to a level at least as strict, though levels may merge
- the new scale breaks the rules above

Run it first with `--dry-run`, which writes nothing ([grammar](#71-grammar)). It lists every config and state file each mapping changes, each old and new `allows` side by side, and the README lines that would change. Old names in journals are history and are never rewritten.

Applying a scale change:

- is a [person's act](#64-how-classifications-flow) ([grammar](#71-grammar)), so an agent may draft and dry-run one but not apply it
- rewrites every level in use in one run, all or nothing, the archive included ([safety guarantees](#81-safety-guarantees))
- is an event in the root's journal with the mapping and its reasons, and an event in each folder whose config or state changes
- rewrites the root's `AGENTS.md` and every README whose level or `[classified]` lines change

A scale edited by hand gets no mapping, so any level it drops is then not in the scale. A level in config or state that is not in the scale is a value `para` cannot use. Each command that shows or compares a level covering that folder refuses, naming the folder and pointing to [`doctor`](#82-doctor-findings) ([config](#35-config)). Two changes still work: a scale change that maps it, and setting a level in its place, which is a person's act since the two cannot be compared.

### 6.2 Lineage and trust
Sources and their grades, a metric's sources, what a file's lineage record holds (primary or derived, inputs and a note on each, the accountable person and who acted for them, with the model, harness and session for an agent), how reliability and checked or unchecked are computed for files and measurements, the notes on what an analytic's formula names, what `show` gives upstream and downstream, and vouching, so origin and trust are recorded one way.

### 6.3 Where classifications are set

An entity's level is the `classification` key in its `config.toml`. Any entity but a stub can have one, the root, buckets, archive buckets and containers included. It covers the entity and everything beneath it, until a nearer entity sets its own:

```toml
classification = "AMBER"
```

Path rules give a stricter level to paths inside the entity. They sit in a `[path-classifications]` table in the same file, each mapping a path, relative to the entity's folder, to a level:

```toml
[path-classifications]
"contracts" = "RED"
"notes/salaries.md" = "RED"
```

A rule on a folder covers everything in it. A rule must name a file or plain folder that exists when the rule is set. Its path uses `/` between names, with no `./` and no trailing `/`, and matches the names on disk exactly, case included. `para` writes the rules in path order ([review groups](#46-review-groups)). `para config` refuses a rule whose path:

- lies inside a child entity, which sets its own level
- contains `..` or is absolute
- is a symlink or passes through one
- is the entity's README or `AGENTS.md`, or lies in a `.para/`

A rule is never looser than the level above it: its entity's, or a rule on a folder that holds its path. `para` refuses a change that would break this, as it does for an entity's level ([how classifications flow](#64-how-classifications-flow)).

Every level has one home. The place level of an entity or a file is the level its place gives it. An entity's is the nearest level set on it or above it. A file's comes from its entity's config alone: the nearest rule covering it, or else its entity's place level. Content made from other content can end up stricter than its place ([how classifications flow](#64-how-classifications-flow)).

A level set on an entity moves with it, since its `config.toml` does. A path rule stays put, keyed by its path. A hand rename or move can leave a rule matching nothing, which silently drops the level from what it covered. A rule matches nothing when its path is gone or now lies inside a child entity, and [`doctor`](#82-doctor-findings) warns about it, naming the rule and its level. [Adopting](#24-adopting-existing-directories) a folder sets on the new entity the strictest level its parent's rules gave anything inside it.

Levels and rules are set with [`para config`](#79-config), and each change is an event in that folder's journal ([config](#35-config)). In the archive they can only be tightened, or changed by a scale change ([archive](#17-archive)). Setting an entity's own level equal to the one covering it counts as tightening.

### 6.4 How classifications flow

Levels flow two ways: down the tree, and along [lineage](#113-lineage) to what is made from content. Both ways they only tighten, and only a person's act loosens one ([principles](#06-principles)).

Down the tree, a level is never looser than the level covering its folder from above. `para` refuses a change that would break this, and names the remedy:

- tightening a folder above a stricter level set beneath it: tighten that level first
- moving an entity, archiving and unarchiving included, so that a level it carries ends up looser than the one above it: tighten the entity first
- moving an entity so that the level covering it falls or goes: set its level on the entity first

Tightening is allowed anywhere, even in the read-only archive, so every refused move has a remedy ([archive](#17-archive)).

Loosening means lowering a level, or removing one when the level above is looser or absent. It is a person's act: it is refused unless run by a person with no one acting for them, and asks for confirmation unless run with `--force` ([grammar](#71-grammar)). `para` takes the names it is given as true, so this guards against accidents only. The confirmation names the result, such as "`brain-x9f2` and everything beneath it go from `RED` to `GREEN`", along with each derived work it loosens. `--force` skips the question. Either way the change is an event in the folder's journal, saying whether it was forced ([config](#35-config)). Removing a level that matches the one above changes nothing and asks nothing.

The confirmation guards against accidents only. A hand edit to `config.toml` skips it. `doctor` reports a level looser than the one above it. A hand edit that keeps that order cannot be seen, since nothing checks config against the journal ([truth and projection](#31-truth-and-projection)).

Along lineage, a derived work's level is the strictest of its place level ([where classifications are set](#63-where-classifications-are-set)) and the level of each file it was made from ([lineage and trust](#62-lineage-and-trust)). Each input's level is worked out the same way, so it already includes that input's own inputs and any declassification. `para` works this out whenever the level is shown, as it does trust, and visits each input once, so a loop of inputs adds nothing. An input whose record or file is gone counts as the strictest level in the scale, so losing an input never loosens what was made from it. In a tree that sets no level anywhere, it adds nothing. An unclassified input adds nothing.

For example, an agent writes `projects/blog/draft.md` in a `GREEN` project and records it as made from `areas/legal/nda.eml`, which is `RED`. Then `draft.md` is `RED`.

Measurements flow the same way. A metric's measurements have the metric's level. An analytic's value takes the strictest of the analytic's place level and the levels of the values its formula names. A key result's or indicator's progress, pace and status take the strictest of its own place level and the level of its analytic's value. The entity itself keeps its place level, which is what its README shows ([READMEs](#53-readmes)).

A person loosens a derived work or an analytic below what flows into it by declassifying it ([writing](#76-writing)). Declassifying names a new level, no looser than the place level. Every input stricter than that level, or for an analytic each stricter value its formula names, must carry a note saying what was done to its content ([lineage](#113-lineage)). Declassifying binds those notes: it records them, and changing one later voids the declassification. A record names each input once, and an analytic's notes are keyed by the id its formula names. For example, `draft.md` could be declassified to `GREEN` once its input `nda.eml` carries the note "Kept only that a deal was signed; no names, terms or dates."

Declassifying:

- is a person's act, so an agent cannot declassify ([grammar](#71-grammar))
- is refused in the archive, which is read-only
- shows each note it binds in its confirmation
- is an event in the entity's journal, with each bound note

State keeps the declassified level and, for each bound note, its input's level at that moment: in the derived work's record, or in the analytic's `state.toml`. From then on the level is the strictest of:

- the place level
- the declassified level
- each input's level, except an input whose note was bound at a level at least as strict as its level now

So an input that tightens later, or a new input, counts again in full. A gone input with a bound note is taken to be at the level its note was bound at, so its note still covers it.

A declassification covers the content as it was. For a file, state keeps its hash, and any later edit to the file voids the whole declassification. For an analytic, state keeps its formula, and a new formula voids it. A metric the formula names that is removed counts as a gone input. `show` says when a declassification is void, and why.

A change to a record or a formula that would lower a work's level, such as dropping a stricter input, re-recording a derived work as a primary or dropping a stricter metric from a formula, is a person's act like declassifying. An agent making it is refused, naming the person who must. It needs no note, since the work no longer claims to come from what was dropped, and its journal event names each dropped input. Dropping an input with a bound note voids the declassification.

A source's own level classifies only its folder. A source stands for an origin, not content, so nothing that names it takes its level.

Trust flows the opposite way: it only falls as content is derived, and every change that raises it is a person's act ([lineage and trust](#62-lineage-and-trust)).

### 6.5 What para does with classifications

`para` records levels, shows them and refuses changes that would loosen them by accident. It does not encrypt files or set permissions, and cannot stop a readable file being copied ([scope and non-goals](#05-scope-and-non-goals)).

An entity's README frontmatter gives the level covering it as `classification` ([READMEs](#53-readmes)). [`show`](#74-reading) and [`list`](#74-reading) give it too. `show` also gives a file's level, with the rule or the inputs it comes from and any declassification. `show`, `list` and `review` print each computed value with its level and reliability, as in `progress 40% (AMBER, B)` ([how classifications flow](#64-how-classifications-flow)).

A parent's README lists each child entity. When a child's level is stricter than the parent's, its line gives `[classified]` in place of its title:

```markdown
- [website/](website/) project "Launch website" brain-7k2q
- [acme-deal/](acme-deal/) project [classified] brain-9x3d
```

The folder name still shows, as it does on disk, so keep secrets out of folder names.

`list`, `show` and `review` take a ceiling, given as `--classification-at-most` with a level or in the `PARA_CLASSIFICATION_AT_MOST` variable, the flag winning. They then leave out every entity, file and computed value that is stricter than the ceiling or unclassified, and say how many they left out. Anything worked out from what they leave out, such as a count or a list of what was made from a file, is worked out without it. A ceiling naming a level not in the scale refuses ([filters, sorting, limiting](#75-filters-sorting-limiting)). A person can start an agent with the variable set, so stricter content stays out of what it reads from `para`. Whoever runs a command can drop the ceiling, so it guards against accidents only.

The root's `AGENTS.md` gives each level and what it allows, then tells agents ([`AGENTS.md`](#54-agentsmd)):

- that a README with no `classification` key is unclassified, not free to share
- to run `para show` on a file before copying its content anywhere, since a README gives only its folder's level
- never to copy content to a place with a looser level or none
- to record a file made from others as a derived work, so levels flow to it
- that only a person lowers a level or raises trust, so an agent never does either, even with `--force`

[`doctor`](#82-doctor-findings) reports a level that is not in the scale, a level looser than the one above it, and a path rule that matches nothing.

### 6.6 Unclassified content

Content no level covers is unclassified. There is no default level. `para` assumes nothing about such content and never treats it as having a level, so classification stays optional. To classify everything, set a level on the root.

Where levels are compared, unclassified ranks below every level:

- moving classified content to where no level covers it is a fall, and is refused ([how classifications flow](#64-how-classifications-flow))
- an unclassified input adds nothing to a derived work's level
- a classified child of an unclassified parent is `[classified]` in the parent's README ([what para does with classifications](#65-what-para-does-with-classifications))

A ceiling leaves unclassified content out too, since nothing says it is safe.

README frontmatter leaves out `classification` when no level covers the entity, and `show` and `list` print a dash. No level name is reserved for it.

A file with no lineage record, or a metric with no sources, counts as the lowest grade ([lineage and trust](#62-lineage-and-trust)). Its place still gives it a level, which flows to whatever is made from it.

## 7. Command line

### 7.1 Grammar
The shape every command follows, so the CLI can be learned once, including the three tiers of change (routine; destructive, confirmed unless `--force`; loosening, a person's act with no one acting, confirmed unless `--force`), what `--force` and `--dry-run` do, and how a confirmation behaves without a terminal.

### 7.2 Fields and time input
The fields a command can set and how dates are typed, so input is consistent.

### 7.3 `init`
Creating a tree, the one command that runs before a tree exists.

### 7.4 Reading
The commands that show and list things, an entity's history included, the most common way into a tree.

### 7.5 Filters, sorting, limiting
How to narrow and order results, so large trees stay usable.

### 7.6 Writing
The commands that change a tree, each a safe, checked operation.

### 7.7 `review`
The command that answers "what needs attention?", `para`'s main daily use.

### 7.8 `rebuild`
Regenerating every generated file from the truth files, which is how a tree is repaired or refreshed.

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
