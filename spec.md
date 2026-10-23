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

- every folder `para` manages (the folder itself, not what you put in it): the root, the buckets and archive buckets, containers such as `objectives/`, archive stubs, and every entity. Each has a `.para/` folder, and together they form an unbroken chain from the root ([kinds and containment](#22-kinds-and-containment));
- everything inside each `.para/`: state, config (including sensitivity rules), the journal, provenance records and the files `para` generates, such as `ACTIVITY.md` ([storage](#3-storage)). Anything else put inside a `.para/` is `para`'s too;
- the keys `para` writes in README frontmatter, its marked blocks in READMEs and `AGENTS.md` files, and, until `para migrate` removes them, the blocks an earlier version wrote in `CLAUDE.md` ([para-owned blocks](#52-para-owned-blocks)).

Change these only through `para`'s commands, including creating, moving, renaming, archiving and removing entities. The commands are designed to be the easiest way for a person or an agent to make these changes. `para` validates each change before writing anything and refuses an invalid one.

Everything else is content: notes, documents, data, other folders, and your own text and frontmatter keys in READMEs and `AGENTS.md`. Anyone may create and edit content. `para` never edits or deletes it, even when removing an entity, and moves it only along with the entity that holds it. Where a README or `AGENTS.md` already exists, `para` adds its part and keeps yours, and adopting a folder adds a `.para/` and keeps everything already there ([adopting existing directories](#24-adopting-existing-directories)). In every other case, if content already sits where `para` needs to create a folder or move an entity, `para` refuses rather than overwrite or merge it.

Changing `para`'s side by hand, such as moving a managed folder or editing a file in `.para/`, is possible but unsupported, and text written inside `para`'s files or blocks is overwritten the next time `para` rewrites them. `para`'s [safety guarantees](#81-safety-guarantees) cover only changes made through its commands, and [`doctor`](#82-doctor-findings) reports hand changes that leave the tree inconsistent, including a `.para/` that `para` did not create.

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
- **Interpreting content**: `para` never judges a file by what it says, and calls no AI model. A model's answer would make commands nondeterministic, so a file's label and provenance come from what a person or agent records and from the tree's rules.
- **Reaching outside the tree**: `para` never fetches or sends anything over a network, which keeps every command [deterministic](#01-what-para-is). A [link](#110-links) points to something outside the tree and is flagged when it is due to be checked or updated, and outside values enter only as [recorded measurements](#42-recording-measurements).
- **Running on its own**: `para` has no background process, schedule or notifications, so nothing changes the tree unless someone runs a command. [`review`](#77-review) is how to ask what needs attention.
- **Access control**: `para` does not encrypt files or set file permissions. A [sensitivity label](#112-sensitivity-labels) records how far content may travel, but `para` cannot stop a readable file being copied. [What para does with labels](#65-what-para-does-with-labels) covers what it does instead.

### 0.6 Principles

Every design decision in this spec follows these rules. A new question is answered by applying them, and a change that would break one needs a recorded decision ([decision index](#appendix-d-decision-index)).

1. **Structure is `para`'s, content is yours**: `para` writes only its own side of the tree and never edits or deletes content ([ownership](#03-structure-is-paras-content-is-yours)).
2. **Deterministic**: the same tree, arguments and time give the same result, apart from new ids ([what para is](#01-what-para-is)).
3. **One home per fact**: each fact is stored as truth in exactly one place ([truth and projection](#31-truth-and-projection)), and each event is recorded once, in the journal of the entity it happened to (or its parent's, when the entity is removed). A value that can be computed, such as a key result's status, is never stored as truth ([what is never stored](#57-what-is-never-stored)).
4. **Generated files serve readers**: the files `para` writes for readers who never run it show facts and computed values, are updated with every change, can be rebuilt from the stored truth at any time and are never read back as truth ([generated files](#117-generated-files)).
5. **Ids are identity**: every entity is addressed by a stored id that survives moves and renames and is never reused. Its path only says where it is now ([addressing](#23-addressing)).
6. **The right path is the easy one**: every change to the structure is a command, and a command that cannot make a change validly and safely refuses before writing anything, saying why and what to do instead. It never guesses ([why deterministic structure helps agents](#04-why-deterministic-structure-helps-agents)).
7. **Loosening and format changes are explicit**: a sensitivity label only tightens down the tree, and trust only falls as content is derived from other content. Only a person, with an explicit command, lowers a label or raises trust, and never a label below its parent's ([how labels flow](#64-how-labels-flow)). A tree's format changes only through `para migrate` ([schema version](#93-schema-version)).
8. **Small and one way**: a feature is added only when managing a PARA tree needs it ([scope and non-goals](#05-scope-and-non-goals)), and each concept has one name, each kind of change one command and each value one format ([glossary](#appendix-c-glossary)).

### 0.7 How to read this spec

[Chapter 1](#1-components) tours the tool, with one short section for each component. Chapters 2 to 9 give the detail, and the appendices hold the [example tree](#appendix-a-example-tree), the [worked examples](#appendix-b-worked-examples), the [glossary](#appendix-c-glossary) and the [decision index](#appendix-d-decision-index).

Every statement about what `para` does is a requirement, written in the present tense: "`para` refuses" means an implementation must refuse, "never" means must not, and "may" means allowed but not required. A reason given with a rule explains it and adds no requirement, and neither does the decision index, which summarizes each decision and links its record. Each term means what the glossary says. Chapter 0 and the tour summarize the detail chapters, and examples illustrate them; where any of these disagrees with a detail chapter, the detail chapter is right.

Every example starts from the tree in Appendix A unless it names another starting point, such as an empty directory, a v0 tree or the example tree with a stated change. The worked examples in Appendix B are the acceptance tests: each states its starting point, command and any input it reads, and a test checks its output, exit code and every file's path and contents exactly. Appendix B says how tests fix the clock, new ids and the terminal.

## 1. Components

### 1.1 The tree

A tree is a root directory and everything in it. The root's `.para/state.toml` has `kind = "root"`, which marks the tree, and records the tree's schema version and the prefix of its ids ([`state.toml`](#32-statetoml)). [`init`](#73-init) creates a tree, and every command that works on a tree finds it by checking the current directory and then each parent for a `.para/state.toml` with `kind = "root"` ([the walk](#36-the-walk)). A v0 tree is marked by `tree.toml` instead, and only [`migrate`](#91-migrating-a-v0-tree) works on it.

Under the root are the four [buckets](#12-buckets), and `archive/` holds archive buckets that mirror the other three ([archive](#17-archive)). Within the buckets are the entities, directories of the kinds `para` tracks by id, such as projects, areas and dirs ([kinds and containment](#22-kinds-and-containment)). Some entities group their children in a container, such as a project's `objectives/`. Every managed folder, entity or not, has a `.para/` whose `state.toml` records its kind, and each except the root sits directly inside another. The root, buckets, archive buckets, containers and archive [stubs](#17-archive) have no id and are not entities.

```text
brain/                     the root
├── .para/state.toml       holds kind = "root"
├── AGENTS.md
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

Every entity has an id made of the tree's prefix and a short random code, such as `brain-7k2q`, and commands name entities only by id ([addressing](#23-addressing)). `para` never reuses an id. A git merge or a hand copy of an entity's folder can still duplicate one; [`doctor`](#82-doctor-findings) reports it, and [repair](#83-repair) gives all but one holder a new id, the only time an id changes.

### 1.2 Buckets

The four buckets are the top-level folders of a tree, one for each PARA category: `projects/`, `areas/`, `resources/` and `archive/`. [`init`](#73-init) creates them, with the archive buckets and the measurement library, and their names are fixed. Filing anything starts with choosing its bucket, the most actionable one that fits ([the PARA method](#02-the-para-method)).

Each bucket directly holds:

| Bucket | Holds |
| --- | --- |
| `projects/` | [projects](#13-projects) |
| `areas/` | [areas](#14-areas) |
| `resources/` | [dirs](#111-dirs), and the [measurement library](#15-measurement) in `measurement/` ([resources](#16-resources)) |
| `archive/` | the archive buckets `projects/`, `areas/` and `resources/` |

Each archive bucket holds the projects, areas or dirs archived from the matching bucket, and the [stubs](#17-archive) that stand in for a live ancestor so an archived entity keeps its path.

Besides those, every bucket and archive bucket holds its `.para/`, a `README.md` and an `AGENTS.md`. `para` writes its own block in each of the last two, and anything you write outside the block is yours ([para-owned blocks](#52-para-owned-blocks)). Anything else put directly in a bucket or archive bucket, such as a file or a folder with no `.para/`, stays content that `para` never moves or deletes, but [`doctor`](#82-doctor-findings) reports it and says to move it into an entity, or to [adopt](#24-adopting-existing-directories) a folder as one.

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
Ongoing responsibilities with no end date, which hold standards rather than deadlines.

#### 1.4.1 Standards
What an area must keep up, the qualitative bar its indicators are measured against.

#### 1.4.2 Indicators
An analytic held to a target over a rolling window (what others call a KPI, or an SLI with its SLO), showing whether a standard is held, which makes upkeep checkable.

### 1.5 Measurement
The shared library in `resources/measurement/`, defined once and used by any key result or indicator, so the same number is never defined twice.

#### 1.5.1 Analytics
A computation over metrics, such as a ratio, with no target of its own, so one analytic can serve many goals and standards.

#### 1.5.2 Metrics
A single thing measured, whose dated values (measurements) are the raw data everything else is computed from.

#### 1.5.3 Terms
A shared definition that scopes metrics, such as what counts as "critical", so metrics that use a word mean the same thing.

### 1.6 Resources
The resources bucket, a hierarchy of dirs holding reference material on a topic, kept apart from commitments so it never looks like work.

### 1.7 Archive
Where finished or inactive things go, so active buckets show only what is live.

### 1.8 Notes and the journal
The append-only history of what happened to each thing, so nothing has to be remembered.

### 1.9 Activity
A readable summary of recent history, so a reader sees what changed without running `para`.

### 1.10 Links
A loose, directional relationship to anything outside the tree (a URL, a search query, a Slack channel, an email address), so outside things stay connected and are flagged when due to be checked (an input) or updated (an output).

### 1.11 Dirs
The basic kind of entity: a folder with its own journal, links and README but no status, living in the resources bucket or inside a project, area or another dir, so any folder can be managed.

### 1.12 Sensitivity labels
How far content may travel, set on entities and inherited by everything beneath them, only ever tightening, so files never have to carry their own labels.

### 1.13 Provenance
Where each file came from and how it was made (captured, edited by a person, or inferred by AI), from which `para` derives how far to trust it alongside your judgment of the source, so content can be traced and weighed by its reliability.

### 1.14 Status
Where a project, objective, key result or indicator stands in its life (other kinds have no status, and key results' and indicators' are computed), so active, blocked and done work can be told apart.

### 1.15 Review
The question "what needs attention?", answered from the tree, which is `para`'s main daily use.

### 1.16 Suppression
A dated, explained pause on an attention flag, so review stays useful when something is knowingly on hold.

### 1.17 Generated files
Files `para` writes for readers who never run it, which are the product rather than a cache.

## 2. Model

### 2.1 Directory shapes
The kinds of directory `para` recognises, so any folder can be classified the same way every time.

### 2.2 Kinds and containment
Which kind may sit inside which, and that entities form an unbroken chain from the root, so the tree has one legal shape that can be checked.

### 2.3 Addressing
How a thing is named by a generated id rather than its path, the only address commands accept, so a reference survives moves and re-parenting.

### 2.4 Adopting existing directories
How a folder that already exists becomes managed, so `para` can be introduced to a real tree.

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
The rendered history file in `.para/`, linked from the README, so recent changes are readable without the CLI.

### 5.5 `MEASUREMENTS.csv`
The rendered measurement table in a metric's `.para/`, linked from its README, so data opens in any spreadsheet.

### 5.6 `AGENTS.md`
The file that orients an agent to the tree, which is how agents learn the framework.

### 5.7 What is never stored
Values that are always computed and never stored as truth, so no stored copy is trusted over a fresh computation.

## 6. Sensitivity and provenance

### 6.1 Sensitivity labels
The label set and what each label allows, so a label has one meaning.

### 6.2 Provenance and trust
What a file's source record holds and the trust levels it can have, so origin and reliability are recorded one way.

### 6.3 Where labels are set
How each entity's `config.toml` sets its own sensitivity and tightens it for paths beneath it, without reaching into a child entity, and where provenance is recorded, so every fact has exactly one home.

### 6.4 How labels flow
How labels reach the files beneath and the content derived from them (sensitivity only tightens, trust only falls, and only a person reverses either), so nothing is loosened or trusted by accident.

### 6.5 What para does with labels
How labels appear in outputs and checks, so a label has visible effect.

### 6.6 Unlabelled content
How content with no labels behaves, so labels stay optional.

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
