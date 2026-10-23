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

`para` owns everything it writes:

- its own files: `tree.toml` at the root and the `.para/` folder in each folder it manages, holding state, config, the journal, and records about content such as sensitivity rules and provenance ([storage](#3-storage));
- the files it generates whole, such as `ACTIVITY.md` and `MEASUREMENTS.csv`;
- its marked blocks inside files you also write in, such as a README's frontmatter ([para-owned blocks](#52-para-owned-blocks)).

Change these, and create, move, rename, archive or remove entities, only through `para`'s commands. The commands are designed to be the easiest way for a person or an agent to make these changes, and `para` validates each change and refuses an invalid one before writing anything.

Everything else is content: notes, documents, data, folders without a `.para/`, and your text around `para`'s blocks. Anyone may create and edit content. `para` never edits or deletes it. It moves content only along with the entity that holds it, removing an entity leaves its content in place, and if content already sits where `para` needs to put a file of its own or move an entity, `para` refuses rather than overwrite or merge it.

Hand edits to anything on `para`'s side are possible but unsupported. Text written in a file `para` generates whole, or inside one of its blocks, is replaced the next time `para` rewrites it. `para`'s [safety guarantees](#81-safety-guarantees) cover only changes made through its commands, and [`doctor`](#82-doctor-findings) reports hand edits that leave the tree inconsistent.

Every command and generated file depends on this split: commands write only `para`'s side, and generated files show it to readers who never run `para`.

### 0.4 Why deterministic structure helps agents
Why a simple, predictable framework with reliable commands lets agents act safely, while leaving them free to manage the files inside it.

### 0.5 Scope and non-goals
What `para` does not do, such as distribute skills, so features outside it are refused for a stated reason.

### 0.6 Principles
The few rules every design decision follows, so new questions can be answered without redesigning.

### 0.7 How to read this spec
The spec's structure (a tour of every component in chapter 1, then the detail in later chapters), its normative words and how its examples are tested, so a reader knows where to look and what is binding.

## 1. Components

### 1.1 The tree
The root directory and the entities in it (every directory `para` manages, each with its own id), because every other component lives inside one tree.

### 1.2 Buckets
The four top-level folders, which are the first sorting decision for anything filed.

### 1.3 Projects
Work with a finish line, the main unit people and agents act on.

#### 1.3.1 Objectives
What a project is trying to achieve, with objectives and key results (OKRs) explained from scratch here, since they exist only inside projects.

#### 1.3.2 Key results
An analytic held to a target by a due date, showing whether an objective is met, which makes progress checkable.

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

### 3.2 `tree.toml`
The file that marks a tree's root and version, so `para` can find and trust a tree.

### 3.3 `state.toml`
The file that holds each thing's facts, the one source of truth `para` reads and writes.

### 3.4 The journal
The format and rules of the event history, so history can be appended safely and read back.

### 3.5 Time and the clock
How times are recorded and compared, so dates mean the same thing everywhere.

### 3.6 Config
Settings and how they pass down the tree, so behaviour can be tuned in one place.

### 3.7 The walk
How `para` finds everything in a tree, so every command sees the same set of things.

### 3.8 Git
How `para` behaves inside a git repository, since most trees are versioned and merged.

## 4. Progress and attention

### 4.1 Analytics, metrics and terms
How analytics compute from metrics and how terms scope metrics, all referenced by id, so a measurement is defined once and reused.

### 4.2 Recording measurements
How dated values are recorded against a metric, the only place raw data enters the tree.

### 4.3 Targets: key results and indicators
How a target is set on an analytic, by a due date or over a rolling window, so goals and standards are measured the same way.

### 4.4 Derived status
How on track, behind or breached is worked out, so nobody has to set it by hand.

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
The rendered history file, so recent changes are readable without the CLI.

### 5.5 `MEASUREMENTS.csv`
The rendered measurement table, so data opens in any spreadsheet.

### 5.6 `AGENTS.md`
The file that orients an agent to the tree, which is how agents learn the framework.

### 5.7 What is never stored
Values that are always computed, so no stored copy can go stale.

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
What `para migrate` does to an older tree, such as assigning ids, so upgrading is one explicit step that never strands data.

### 9.2 Retired keys and files
What `para` now ignores, such as skills and the `emit.*` keys, so leftovers are understood and not misread.

### 9.3 Schema version
What schema 2 is and how `para` treats other versions, so the format can evolve through `para migrate` and never silently.

## Appendix A. Example tree
One complete example tree, used by every example in the spec, so readers learn one tree.

## Appendix B. Worked examples
Commands run against the example tree with their output, which double as the acceptance tests.

## Appendix C. Glossary
One definition per term, so every word in the spec means one thing.

## Appendix D. Decision index
The decisions behind the design, briefly, so the reasoning is on record without cluttering the spec.
