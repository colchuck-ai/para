# Clean room: the next revision of para

Status: **draft for review.** Nothing here has been started.

## Why

The proof of concept proved the idea, and the repo shows how it got there. The design lives in
four generations of documents. `para-design-v4.md` "supersedes" v2 and `command-surface.md` but
argues against them throughout, and §28–§30 were bolted on as amendments. A 2,000-line
implementation plan, three ADRs and two finished `plans/` folders sit beside it. No single place
says what para *is* today.

The next revision starts from one unified spec, and new code is written against that spec.

## Decisions already made

| # | Decision |
| --- | --- |
| D1 | **The code is rewritten from the new spec, test first.** The PoC is reference material. A package may be ported, but only as a deliberate, reviewed step that brings its tests along. Nothing is bulk-copied. |
| D2 | **Same repo, fresh start.** Tag the PoC, then replace `main`'s tree. History and the module path (`github.com/colchuck-ai/para`) are kept. |
| D3 | **Existing trees keep their data.** A v0 tree (e.g. `~/brain`) is upgraded by one explicit `para migrate` (D14), never by hand. Skills are the only exception (D4). |
| D4 | **Skills are removed, with no migration.** Distributing skills is a separate problem that para keeps out of scope. Skills can be layered on top of a para tree by other tools. |
| D5 | **`AGENTS.md` stays** as marker-scoped generated blocks. `CLAUDE.md` is no longer generated (D21). |
| D6 | **`skill` and `skills` are ordinary names again.** Both leave the reserved-word list. |
| D7 | **`para migrate` moves a tree from `schema = 1` to `schema = 2`**, adding a stored `kind` to every managed folder and an id to every entity in one pass (D28). v0.8.0 reads and writes only schema 2. (Reverses the earlier "stay at schema 1".) |
| D8 | **The first clean-room release is `v0.8.0`.** |
| D9 | **Dirs and sensitivity labels are designed as part of the new spec** and ship in `v0.8.0`. Storing `kind` in `state.toml` was the first step toward them. The design is worked through together, chapter by chapter, in Phase 1. |
| D10 | **Addressing is by a generated, stored id, not by path.** The id is the key and survives moves and re-parenting; it resolves to the thing's name and path for people. Design details are settled in Phase 1 (§2.3 of `spec.md`). |
| D11 | **Commands take an id only.** Names and paths are not accepted as addresses; an id is looked up first (with `list`), so every output shows ids. |
| D12 | **Ids are a tree prefix plus a short random code**, like beads (`brain-7k2q`). The code grows longer as the number of ids grows, to keep collisions unlikely. The scaling rule follows a study of beads. |
| D13 | **Id rules.** Code is random (not a content hash) over lowercase `0-9a-z`; at least 4 characters; length chosen so the chance of any collision stays at or below 1%, counting every id ever issued in the tree, so lengths only grow (4 up to ~180 ids, 5 up to ~1,100, 6 up to ~6,600). Prefix is letters and digits, no hyphen. No child ids. Local collisions are retried at creation; `doctor` reports duplicate ids, which only a merge or a hand copy of an entity's folder (such as `cp -r`, which copies its `.para/`) can produce. Studied from beads; see the Phase 1 notes. |
| D14 | **`para migrate` upgrades a tree explicitly.** It assigns ids to a v0 tree (and runs any later format change). Other commands refuse an unmigrated tree and name the command. Amends D3. |
| D15 | **Sensitivity is an inheritance tree of entities.** Each entity's `config.toml` sets its sensitivity and may name paths beneath it, including unmanaged ones, that need a stricter label. A path rule never reaches into a child entity: the child classifies everything inside itself, so two configs never claim the same file. A child entity may only match or tighten its parent's label, never lower it. Path rules are relative to their own directory and may not climb out of it: no `..` segments, no absolute paths, and no symlink whose target resolves outside the directory. |
| D16 | **Entities form an unbroken chain from the root.** Every entity's parent is itself managed by para (the root, a bucket, an archive stub (D32), or another entity, containers such as `objectives/` included (D47)), so an unmanaged directory can never hold an entity. Commands refuse to create a gap, and `doctor` reports one made by hand (the PoC's `orphan` finding). Adopting an existing folder therefore works from the top down. |
| D17 | **Terms.** An *entity* is a managed directory of one of these kinds: a project, area, objective, key result, standard, indicator, analytic, metric, term, source (D71), link, dir or container. The root, buckets, archive buckets and archive stubs are managed but are not entities and have no id (D28, D32). A *container* is an entity with a reserved name that groups one kind of child, such as `objectives/` (D47). A *dir* is the basic kind of entity (D18). *Directory* stays an ordinary word for any folder on disk. *Component* means anything in the spec's tour (chapter 1). |
| D18 | **What a dir is.** It carries a journal, notes, links, `ACTIVITY.md` and a README with frontmatter, plus sensitivity (provenance is recorded per file, D79); it has no status. It sits in the resources bucket or inside a project, area or another dir, never directly in the projects or areas bucket, and never contains a project or area. **Resource is no longer a kind:** the resources bucket is a hierarchy of dirs, and `para migrate` converts each `kind = "resource"` to `dir`. A dir can be archived on its own, like any entity but a container (D51): it moves to the archive bucket at a path that mirrors where it came from. To tighten one subfolder's label, use a path rule; make the subfolder a dir only when it needs more than a label. |
| D19 | **Areas have standards and indicators**, mirroring projects' objectives and key results. A standard is the qualitative bar an area keeps up. Both are optional: an area may have no standards, and a standard may have no indicators. But an indicator always sits under a standard, never directly under an area. An indicator holds an analytic to a target over a rolling window (a KPI, or an SLI with its SLO) and has a computed status (D20). |
| D20 | **The measurement library.** Analytics, metrics and terms are kinds that live in one fixed place, `resources/measurement/{analytics,metrics,terms}`; para creates it on first use (D44), and `measurement` is a reserved name. An *analytic* is a computation over metrics, with no target of its own. A *metric* is a single measurement point; its dated values are the measurements, which belong only to metrics. A *term* is a shared definition that scopes metrics. These relationships are many-to-many, so they are references by id, not nesting. A *key result* holds an analytic to a target by a due date; an *indicator* holds an analytic to a target over a rolling window. One analytic may serve any number of key results and indicators across projects and areas. |
| D21 | **Para no longer generates `CLAUDE.md`**, and the `emit.claude` key is retired. Claude Code reads `AGENTS.md` itself since 2.1.277 (root and parents at start, subfolders on first use). `para migrate` removes para's block from existing `CLAUDE.md` files and deletes any left empty. Because any `CLAUDE.md` or `CLAUDE.local.md` in or above the tree turns off Claude Code's `AGENTS.md` reading, `doctor` reports one that does not import `@AGENTS.md`. |
| D22 | **`para` never deletes or edits content, and `remove` keeps it.** `remove` deletes only `para`'s own files for the entity and everything beneath it, deepest first, so a failed run can be rerun; the plain folders left behind hold no entity, so the chain rule holds. Its own files go with it, including the journal, recorded measurements and the generated `ACTIVITY.md` and `MEASUREMENTS.csv`, so `remove` means `para` forgets the entity completely; `archive` is how to stop work and keep history. It confirms first, naming what will be lost (for example, "deletes 214 journal entries and 1,032 measurements; use `para archive` to keep them"), with `--dry-run` and `--force`; prints the content paths it left and the `rm -r` to delete them, and points to `archive` as the normal way to retire something. It strips `para`'s blocks from a README and deletes the README only if nothing else is left. Removed ids stay issued and are never reused; `doctor` reports references to them. Chosen unanimously by three user-lens reviews (person, AI agent, scripts and maintainers). |
| D23 | **Generated files live in `.para/`.** `ACTIVITY.md` and `MEASUREMENTS.csv` are written inside the entity's `.para/`, and `para`'s README block links to them. The only visible files `para` writes in are a README and `AGENTS.md` (root, buckets and archive buckets), and only inside its blocks. On `remove`, the parent's README block is regenerated and the removal is recorded in the parent's journal, so the parent's history says what became of the child. `AGENTS.md` is unaffected by `remove`. |
| D24 | **`para` owns only its own keys in README frontmatter.** Keys you add (Obsidian `tags`, `aliases` and so on) are content: `para` adds, updates and removes only the keys it writes and keeps yours. |
| D25 | **Every refusal says why and what to do instead.** It names the rule, the offending id or path, and a remedy: a command to run, or, when content is in the way, the user's own files to move. This makes the right path the easy one (§0.4); §7.10 defines how the remedy appears in text and JSON output. |
| D26 | **`para` never runs git or requires a repository.** It writes files and leaves committing, merging and syncing to you, so it works the same in any tree and never commits for you. §3.7 says how its files behave in a repository (whether it reads `.git/` or writes `.gitattributes` is settled there). Carried from the PoC (v4 §9). |
| D27 | **Only a person lowers a label or raises trust.** The command that does it requires an interactive confirmation and has no `--force` or other non-interactive path, so an agent or script cannot complete it alone. It refuses when no terminal is attached, saying a person must run it (D25). Every other loosening is refused. §6.4 defines the commands. Amended by D69: `--force` skips the confirmation. |
| D28 | **Every managed folder holds `state.toml`, and the root's marks the tree.** Each `.para/` (root, bucket, archive bucket, stub, entity) holds a `state.toml` recording its kind and may hold a `config.toml`. The root's has `kind = "root"` and records the schema version and id prefix; there is no `tree.toml`. The root has a kind but no id and is not an entity. Commands find a tree by looking upward for it (§3.6). |
| D29 | **State holds facts; config holds choices.** `state.toml` records what a folder is and what has happened to it (kind, id, status, dates, schema version, id prefix); commands set these by doing the thing, and `para config` cannot. `config.toml` records how a person wants `para` to treat a folder and what is beneath it (such as sensitivity); `para config` sets these at any time, except that only a person loosens a label (D27). The test: if the value changed, would something already recorded become false? If so, it is state. **The id prefix is state and never changes**, so every id issued in a tree carries its prefix and an id from a tree with another prefix can be recognised and refused (D25). Two trees can share a prefix (a copied tree, or the same choice twice), so a prefix shows lineage, not a unique tree. Ids are never re-indexed. |
| D30 | **Repair resolves a duplicate id by giving one entity a new id.** When a merge or hand copy leaves two entities with the same id, `doctor` reports it and repair gives one of them a new id; the person chooses which keeps the original. This is the only time an id changes. References to the old id in content keep pointing at the entity that kept it. §8.3 defines the command. Amends D13. |
| D31 | **A bucket holds only entities of its kind and `para`'s own files.** Directly inside `projects/`, `areas/` and `resources/` (and their archive buckets) sit only the entities of that bucket's kind (with `resources/measurement/`, D20, and in archive buckets the stubs of D32), plus the bucket's `.para/`, README and `AGENTS.md`, in which your own text outside `para`'s block stays content. `archive/` holds only the three archive buckets and the same files. Anything else directly in a bucket, a loose file or a plain folder, is a `doctor` finding whose remedy is to move it into an entity, or to adopt a folder as one (D25). Replaces the PoC's advisory `untracked`, which allowed loose files. |
| D32 | **Archiving keeps a managed stub for each live ancestor.** When something is archived from inside an entity that stays live (such as a dir inside an area, D18), `para` creates a stub at each mirrored ancestor path that has no archived entity, such as `archive/areas/health/`. A stub is a managed folder: its `.para/state.toml` has `kind = "stub"` and the id of the entity it stands for, it has no id of its own, no README and no `AGENTS.md`, and it is not an entity. Stubs hold only archived entities and other stubs. `para` removes a stub when the last thing beneath it is unarchived, and replaces it with the entity when that entity is itself archived. Replaces the PoC's stub, which had no `.para/` and so broke D16. Amends D16, D17, D28 and D31. |
| D33 | **Projects never nest.** A project sits only directly in `projects/` or `archive/projects/`, and no entity (project, area, dir or any other) contains a project, directly or deeper. A large effort is one project with several objectives. Carried from the PoC (v4 §1.3). §2.2 states the containment rule. |
| D34 | **Project and objective statuses are `planned`, `in-progress`, `blocked`, `done` and `dropped`.** The default is `planned`. A person or agent sets status with a command, in any order, so a done project can be reopened; `done` and `dropped` say how the work ended. A project or objective may have a due date; what a passed due date does is settled in §1.14 and chapter 4. Archiving is separate and never automatic. Carried from the PoC (v4 §1.7). §1.14 owns the details. |
| D35 | **Key result statuses are computed: `achieved`, `missed`, `at-risk`, `on-track`; `dropped` is the only one set by hand.** `achieved` when the analytic reaches the target, `missed` when the due date has passed without it, `at-risk` when progress is too slow to reach it in time, otherwise `on-track`. Status is never stored. Before the due date it follows the latest value and does not latch; once the due date has passed, only measurements dated on or before it count, so the result settles as `achieved` or `missed`. `dropped` overrides every computed status and can be cleared, which returns the computed one. Indicators get their own statuses (§1.4.2). Carried from the PoC (v4 §4.3). §4.4 defines the computation. |
| D36 | **Every key result has its own due date.** It is required at creation, and it is what separates a key result from an indicator's rolling window (D20) and makes pace and `missed` computable. |
| D37 | **An objective holds only its key results and your content; a key result holds no entities.** It holds its key results in `key-results/` and any content of your own; links and dirs belong to its project (D18). A key result holds no entities, only its own `.para/`, README and your content. §2.2 states the containment rule. |
| D38 | **Areas nest at any depth.** An area sits directly in `areas/` or `archive/areas/`, or inside another area, so health can hold fitness and sleep. No other entity holds an area (D18). Carried from the PoC (v4 §1.3). |
| D39 | **An area holds its standards in `standards/`, and a standard its indicators in `indicators/`.** This mirrors `objectives/` and `key-results/`. A standard holds only its indicators and your content; an indicator holds no entities (as D37). Each container name is reserved in its parent (`objectives` and `links` in a project, `key-results` in an objective, `standards` and `links` in an area, `indicators` in a standard, and `links` in a dir, D57), so no sub-area, dir or content folder may take it. §2.2 states the containment rule. |
| D40 | **Indicator statuses are computed: `no-data`, `breached`, `at-risk`, `healthy`, first match wins.** `no-data` when the analytic has no value over the rolling window, `breached` when the analytic over the window misses the target, `at-risk` when it meets the target but is within a margin of missing it, otherwise `healthy`. The margin is a config setting with a default that passes down the tree (D29, §3.5). Nothing is set by hand: to stop tracking an indicator, remove or archive it (D51). Never stored. §4.4 defines the computation. |
| D41 | **A measurement is a dated number.** Metrics have no types: the PoC's `ratio` becomes an analytic over two metrics, and a yes/no milestone is tracked as an objective's status. One grammar, no type rules. Replaces the PoC's `number`, `ratio` and `boolean` key result types (v4 §4.1). |
| D42 | **The measurement library is flat.** Each analytic, metric and term is a folder directly in `analytics/`, `metrics/` or `terms/` and holds no other entities, only its `.para/`, README and content. They are found by id, so folders add little. |
| D43 | **A term is a definition that metrics reference.** Its definition is the text of its README, which `para` never interprets. A metric names the terms it uses by id. For every reference in the library (key result or indicator to analytic, analytic to metric, metric to term), commands refuse a missing id or the wrong kind, `show` lists the reference both ways, and `doctor` reports one whose target was removed. |
| D44 | **The measurement library is created on first use.** `init` does not create `resources/measurement/`. Adding the first analytic, metric or term creates `measurement/` and that kind's container; the other containers wait for their own first entity. `measurement` is reserved in `resources/` whether or not the library exists, so `para` refuses to create or adopt anything else under that name. A tree with no library is not a `doctor` finding. Amends D20. |
| D45 | **Staleness is set per kind and is off unless set.** Every kind that can go stale has its own `<kind>.stale-after` setting (a number of days), including `dir`, `analytic`, `metric` and `term` (but not `container`, which never goes stale, D47), as the PoC had for its kinds (v4 §7). There is no built-in default, so nothing goes stale until someone opts in. A setting passes down from wherever it is set, root included, and the nearest wins (§3.5). So a resource (a dir in `resources/`) has no status or due date and review never reports it stale unless a `dir.stale-after` reaches it; set it on `resources/` to cover only resources. `metric.stale-after` flags a metric not measured lately. §4.5 defines the clock. |
| D46 | **`init` writes starter staleness settings.** A new tree's root `config.toml` gets `project.stale-after` and `area.stale-after` values, so review flags neglect from the start; you can see, change or remove them like any setting. `para` itself has no hidden staleness defaults (D45), and every other kind stays off until set. `migrate` writes none, so a migrated tree keeps what it had. §7.3 and §3.5 give the values. |
| D47 | **A container is an entity kind of its own.** Each container (`objectives/`, `key-results/`, `standards/`, `indicators/`, `links/`, and in the libraries `measurement/`, `analytics/`, `metrics/`, `terms/` and `sources/`, D71) has `kind = "container"`, an id and a place in the chain, so it is managed like any entity and easy to tell apart from a dir. It carries a journal (which records its children arriving and leaving, D23) and a README whose `para` block lists its children; it has no links, no staleness and no label of its own, inheriting its parent's. `measurement/` is the one container that holds containers (`analytics/`, `metrics/`, `terms/`). Its name is reserved in its parent. `para` files only the expected kind in it (objectives in `objectives/`) and refuses anything else; anything else found in it, another entity or content, is a `doctor` warning. A container never goes stale and has no `stale-after` setting, so a `dir.stale-after` never reaches one. It cannot be archived on its own (D51). Stubs keep no id of their own (D32): a stub already records the id of the entity it stands for. Replaces the non-entity container; amends D16, D17, D28, D32, D42, D44 and D45. |
| D48 | **Anything can be archived whatever its status.** Archiving moves an entity and never changes its fields, so its status still says how it ended. As in the PoC (v4 §18.5). |
| D49 | **Unarchive brings back the ancestors it needs.** Unarchiving an entity whose ancestors are also archived makes each of them live again, joins any ancestor that is already live, and leaves a stub for archived siblings that stay. It restores everything beneath it, including anything that was archived on its own before its parent was; to keep that archived, archive it again. As in the PoC (v4 §1.6). |
| D50 | **The archive is read-only.** An archived entity can be shown, listed, unarchived or removed; any other command, including a note or a measurement, needs it unarchived first. Nothing is created under `archive/` except by archiving or unarchiving (which may leave stubs). Amended by D67: a label may be tightened in the archive. Amended by D84: a source's grade may be lowered in the archive. |
| D51 | **Any entity but a container can be archived on its own.** It moves with everything beneath it to its mirrored path, with stubs (D32) for live ancestors, containers included. A container stays with its parent: archiving one refuses and names its entities, and its parent when that can be archived (not for `measurement/`, whose parent is a bucket),, because a new container made in its place could never rejoin it. When an entity is archived, every stub that stands for it or a descendant gives way to that entity at every depth, matched by id, an exception to §0.3's collision rule; `para` refuses only when two archived entities would share a path (D25). Resolves the open "what can be archived alone" question. |
| D52 | **Containers are created on first use.** A container appears when its first child is added, as `measurement/` does (D44), so a new project has no empty `objectives/` or `links/`. Resolves the open "when containers exist" question for creation; what happens when a container's last child goes is for §2.5. |
| D53 | **Every managed folder but a stub keeps a journal.** Entities, containers, buckets, archive buckets and the root each have one in `.para/`. A bucket's records entities arriving, moving, being archived or removed; the root's records `init`, `migrate` and changes to its own files and blocks. An event goes only to the journal of the folder it happened to (one home, as in the PoC, v4 §3.2). Stubs have none. Resolves the "journaling outside entities" note. |
| D54 | **A note counts as attention only when marked.** No note resets the staleness clock unless it is marked as attention, whether added with its own command or carried as a reason by another command. A forgotten mark costs a visible stale flag, cleared with one marked note, while counting every note would let working notes hide neglect; and agents, which write many notes, claim upkeep only when the person says it happened. Reverses the PoC's default (`--no-attention`, v4 §18.6), chosen after three user-lens reviews (person and scripts preferred counting `para note` by default; the agent lens preferred opt-in). `migrate` marks v0 notes without `--no-attention` as attention, so they keep their meaning. |
| D55 | **Attention is the entity's own.** An entity's attention is its newest note marked as attention (D54), or else its creation; for a metric, a new measurement counts too. Nothing else counts: not status or other field changes (as in the PoC, v4 §3.6, so a change cannot buy silence), and not anything on a child, so a sub-area's notes do not keep its parent fresh and each level is tended on its own. Resolves the "staleness of nested areas" note. |
| D56 | **`ACTIVITY.md` shows a folder's full history, for every folder with a journal.** Entities, containers, buckets and the root each have one in `.para/`, linked from their README, listing every event in that folder's own journal by day, newest first, as in the PoC (v4 §3.5). A recent window would make the file change with no command run, breaking write-through and making `rebuild` depend on the day. |
| D57 | **Links.** A link is an entity in its holder's `links/` container; projects, areas and dirs can hold any number, and nothing else holds them. It records a `type` you choose (opaque, such as `slack-channel`), fixed at creation because tools key off it; a `ref` the type gives meaning to (opaque, such as a channel or URL); and a `direction`, `input` (check it) or `output` (keep it updated), never both, so a two-way channel is two links. Type and direction are both fixed at creation (flipping direction would change what the link's attention means); the `ref` can be changed, as when a URL moves, and the change is journaled. It holds no entities, and `links` is a reserved name in projects, areas and dirs (D39). `para` never interprets the type or ref and never contacts the target (§0.5). A link is flagged through ordinary staleness, `link.stale-after` (D45): checking or updating it is recorded as a note marked as attention (D54). As in the PoC (v4 §29), with dirs in place of resources. |
| D58 | **Labels are TLP 2.0** (by default; amended by D74). The five labels are `CLEAR`, `GREEN`, `AMBER`, `AMBER+STRICT` and `RED`, with FIRST's meanings, ordered from loosest to strictest. An outside standard, so a label means the same in a tree as anywhere else. Carried from the Oct 20 brain session. §6.1 defines them. |
| D59 | **A label can be set on the root, a bucket, an archive bucket or any entity but a container.** It covers everything beneath it, and anything below may only match or tighten it. A label on the root is how a whole tree gets one. Containers inherit their parent's (D47); stubs carry none. Amends D15, which allowed labels only on entities, and resolves the "top of the label chain" note. |
| D60 | **Unlabelled means no rule.** Content no label covers is shown as `unlabelled`, and `para` assumes nothing about it, so labels stay optional. There is no default label and no `label.default` setting. §6.6 owns the details. |
| D61 | **A move never lowers a label.** An entity with no label of its own takes the one above it, so a command that moves it refuses when its label would fall, and the remedy is to set its label on the entity first (D25). Loosening stays a person's explicit act (D27, principle 7). |
| D62 | **Archiving and unarchiving follow the move rule.** They refuse when an entity's inherited label would fall at its new path, with the same remedy. An entity with its own label moves it along, but can still be refused under D64 when its new parent is stricter. Amended by D64 and D67. |
| D63 | **The effective label is visible.** Each entity's README frontmatter carries the label that covers it, `show` and `list` print it, and `AGENTS.md` says what each label allows. Changing a label rewrites the READMEs beneath it (write-through). §6.5 owns the details. |
| D64 | **A label is never looser than the one above it.** A command refuses any change that would leave a set label looser than the label covering its folder, naming what it would break (D25): tightening a folder above a child's own label (remedy: tighten the child first), or moving a labelled entity under a stricter parent (remedy: tighten the entity first). So the label covering anything is the nearest one set. `doctor` reports a looser label made by hand. Chosen over "strictest along the chain wins". Amends D62. |
| D65 | **Losing a label is a fall.** For the move rule (D61), unlabelled ranks below `CLEAR`, so moving labelled content where no label covers it is refused like any other loosening. |
| D66 | **Path rules may sit in any config that can hold a label**: the root's, a bucket's, an archive bucket's or an entity's other than a container's. A path rule never reaches into an entity (D15), so a stray file in a bucket can be labelled. Amends D15. |
| D67 | **Tightening is allowed anywhere, the archive included.** Setting a stricter label on an archived entity or anything in it is an exception to D50, since tightening can lose or loosen nothing. So every refused move, `unarchive` included, has one remedy: tighten a label, then run the command again. Chosen over an `unarchive --label` flag, which would give labels a second command (principle 8). |
| D68 | **A label can be removed.** The folder then takes the label above it. Removing a label is a loosening when the label above is looser or there is none (D65), so it needs a person's confirmation (D27); otherwise it changes nothing and needs none. `unlabelled` is how `para` shows the absence of a label, never a value to set. |
| D69 | **Loosening confirms by default, and `--force` skips it.** Lowering or removing a label (D68) and raising trust ask for confirmation at a terminal, and `--force` runs them without asking, like `remove` (D22). The prompt is a nudge against an accidental loosening, not a security boundary: a hand edit to `config.toml` or a pseudo-terminal gets round any prompt, and real enforcement (file permissions, a sandbox) is outside `para` (§0.5). Every loosening is journaled, saying whether it was forced. Reverses D27's "no non-interactive path"; amends principle 7. |
| D70 | **Provenance has no folder defaults.** How a file was made is a fact (state, D29), so no config rule claims it for files not yet there. Files are recorded explicitly, in bulk if needed (each at its current hash); files added later stay `unrecorded`. Chosen unanimously by three design lenses (agent, maintainer, standards). |
| D71 | **Sources are a library kind.** Each source is an entity in a `sources/` library container, like terms (D43), with an id, a README describing it and a journal; its reliability grade (Admiralty A–F by default, D74; a person's judgment) is config. Chosen over a root registry for its history and merge behaviour. Amends D17, D44 and D47; D78 settles where it sits. |
| D72 | **Derived trust is computed when read.** Derived content takes the worst of its inputs, looked up live, so regrading a source flows into everything derived from it; a removed, changed or unrecorded input, or a removed source, counts as the lowest grade on the scale (`F` by default, D74). Never stored (principle 3). |
| D73 | **Provenance records are journal events.** One append-only log per folder holds everything that happened, provenance included; views such as `ACTIVITY.md` may filter or summarize records. No separate provenance log. Amended by D79: the current facts are state. |
| D74 | **Sensitivity and reliability scales are configurable; the creation method is not.** The root config defines each as an ordered scale, and `init` writes the defaults (TLP 2.0 for sensitivity, Admiralty A–F for reliability). The creation method is a fixed part of the design. A label or grade not in its scale is a `doctor` finding. Amends D58. |
| D75 | **`para` never writes content, so recording is its own step.** Provenance is recorded with an explicit command naming each file, after the file exists; there is no capture command and no sweep of unrecorded files. Keeps §0.3 and D23 intact. |
| D76 | **Provenance is built on primary versus derived.** A primary is a copy of something from outside the tree (an email, a transcript, a web page): fixed, hashed, naming its source, and flagged when it changes. A derived work is made from other material or from its author's own thought: living, never hashed as a whole, with an append-only history of contributions and their inputs. Distance from the source (primary, secondary, tertiary) is computed from the input chain, never recorded. Replaces the `captured`/`written`/`inferred` methods, which mixed distance, who made it and mutability. |
| D77 | **Delegation follows RFC 8693.** Every provenance record (and every confirmed loosening, D69) carries a `sub`, the person accountable, and an optional `act`, who acted on their behalf, which nests for chains (a subagent for an agent for a person). No `act` means the subject acted directly. Trust keys on delegation, not on whether the actor is an AI: only a review by the subject acting directly counts. Identities are self-declared; `para` authenticates no one. Amended by D85 and D88. |
| D78 | **The sources library is `resources/sources/`.** Created on first use beside `resources/measurement/`; `sources` is reserved in `resources/`, as `measurement` is (D44). Settles D71's open location. |
| D79 | **A file's provenance is state, shaped like a label path rule.** The current facts about each recorded file (primary or derived, source or inputs, hash, `sub` and `act`, the last vouch and delegated work since) sit in the `state.toml` of the entity holding it, keyed by its path relative to that entity, never reaching into a child entity. Only files inside entities are recorded, so every input can be named by entity id (D81); a file directly in a bucket is already a `doctor` finding (D31). Recording, contributing and vouching are journal events. As with status, state is truth and the journal is history. Unlike a label, a record names files, never a folder (D70). Trust is computed, never stored (D72). Amends D73. |
| D80 | **A source's first grade needs no confirmation.** `para add source … --reliability C` sets it directly, since nothing depends on a new source yet (reversed by D89: it confirms unless forced); a source added without one starts at the lowest grade (`F` by default, D74). Raising a grade later confirms by default (D69); lowering one never does. |
| D81 | **A derived work names its inputs by entity id and path.** Each input is the id of the entity holding it plus the path within it, so `para` moves, archiving and unarchiving never break a reference; an input outside the tree is refused. A hand rename or a file moved by hand between entities is a `doctor` finding (§6.2, §8.2). |
| D82 | **People are sources.** A record's accountable person (`sub`, D77) is a source in `resources/sources/`, graded like any other. A derived work's reliability is the worst of its author's grade and its inputs' reliability, so a person's own thinking with no inputs reads as that person is graded. Generalized by D85. |
| D83 | **Vouching confirms unless forced.** A vouch raises trust (it clears unchecked and replaces the actors' grades, D88), so it asks for confirmation by default and `--force` skips it (D69). It counts only when it names the accountable person with no `act`; identities are taken as given (D77). |
| D84 | **A grade can be lowered in the archive.** As with tightening a label (D67), lowering an archived source's grade is allowed, since it can only lower trust. Recording and vouching wait for unarchive. Amends D50. |
| D85 | **Actors are sources.** A record's `act` (D77) names a source, as its `sub` does, so every identity is registered and unique by id; the same holds for the `sub` and `act` of a confirmed loosening (D69). Whoever made something is a source: a derived work's reliability is the worst of its accountable people's, its actors' and its inputs' grades. Delegation (unchecked until vouched, D83) still applies on top. Generalizes D82. Amended by D90 for primaries. |
| D86 | **An agent's source is its model and harness**, such as "Claude Code with Opus 5.5", so a setup can be graded as a whole. |
| D87 | **Each act records ungraded attribution**, given with the command and never detected: the exact model id (`claude-opus-5-5`), the harness and its version (`claude-code/3.2.1`) and the session or run id, so work can be traced to what produced it. These are facts, never graded, and taken as given (D77). |
| D88 | **A vouch takes over from the actors.** For the work recorded before it, in record order, the vouching person's grade counts in place of the grades of every actor in its `act` chains, person or agent, so a careful review lifts work by a low-graded agent and nothing drifts down for good. Inputs' grades still count. Amends D85. |
| D89 | **A source's first grade confirms unless forced.** Grading a source above the lowest, even when adding it, raises trust, so it asks for confirmation and `--force` skips it (D69); otherwise an agent could add itself, or a person, at the top grade unconfirmed. Reverses D80's no-confirmation; a source added without a grade still starts at the lowest. |
| D90 | **A primary's reliability is its source's grade alone.** A copy is as reliable as where it came from; who saved it shows only through unchecked (D83) when someone acted for the accountable person. A derived work counts its people, its actors and its inputs (D85). |
| D91 | **Attribution comes from flags or environment variables, and a flag wins.** `--sub`, `--act`, `--model`, `--harness` and `--session`, or `PARA_SUB` and the rest, which a harness can set once per session. With no `sub`, commands that record or vouch refuse and say how to set one (D25); `sub` has no default from config, so an agent's work is never silently attributed to a person acting directly. Chapter 7 says the `PARA_*` variables stand in for flags, so a command's input stays its arguments (§0.1). |

## What leaves with skills

D4 removes more than one noun. Everything below exists only because skills did:

- **Model:** the `skill` noun and kind, `scope` and its walk (§5.2, §5.4), derived rules (§5.3).
- **Outputs:** `.agents/rules/para-*.md`, `.claude/skills/` mirrors and `emit.claude-skills`, and
  all of `emit.cursor` (§6.2), because Cursor output was only ever rules and skill mirrors.
- **CLI:** `add/show/list/set … skill`, `review --skills`.
- **Doctor findings:** `scope-unresolved`, `orphan-rule`, `orphan-mirror`, `broken-link`, and the
  skill clause of `invalid`.
- **PoC code with no successor:** `internal/mirror`, the cursor and rule parts of `internal/render`,
  `internal/rebuild`, `internal/doctor`, `internal/mutate`, and `internal/view/skills.go`.

### What the next revision does with a v0 tree's skill leftovers

`~/brain` has four para-managed skills, generated rules, `.claude/skills/` mirrors and
`emit.claude = true`. The proposed rule: **para stops owning these files and never deletes them.**

- It never walks `.agents/`, `.claude/` or `.cursor/`. A `kind = "skill"` `state.toml` in there is
  invisible to it.
- `emit.cursor`, `emit.claude-skills` and `emit.claude` in `config.toml` are read as **retired keys**: ignored,
  with one `doctor` note saying they can be removed.
- `para migrate` removes para's block from every `CLAUDE.md`, rule imports included, and deletes a
  file left empty (D21).
- Old journal events about skills are still valid lines, but the next revision does not render them.
- Cleaning up the leftover files by hand is the user's job. The compat chapter (below) lists them.

## Phase 0: Freeze the PoC

1. Commit the pending repo changes: the beads repair in `.beads/` and `docs/release-process.md`.
2. Release the unreleased kind-in-state work as the last PoC version (`v0.7.0`), so `~/brain`'s
   installed binary matches the end of the PoC line.
3. Tag that commit `poc-final`. Every deleted document stays reachable from there.
4. Delete the stale `impl`, `rc` and `fixes` branches, local and remote, once they are confirmed
   merged or abandoned.
5. In beads: recreate the lost issues as records. `para-nd3` and `para-sxt.1`/`.2` were finished.
   The rest of `para-sxt` (at least `.4`, `.7` and `.10`) and `para-5ul.1` were not, and the
   `clean-room` work supersedes them. Then open a `clean-room` epic with one child per phase below.

Done when: `poc-final` exists on GitHub, `v0.7.0` is released, and only `main` remains.

## Phase 1: One spec (the documentation consolidation)

This is the bulk of the thinking. The spec is written **before** any new code exists.

### 1.1 Target layout

```
README.md                    what para is, install, a five-minute walkthrough
spec.md                      the whole specification: tour, detail, examples, glossary
docs/
  decisions/                 short ADRs, current decisions only
  release-process.md
AGENTS.md / CLAUDE.md        contributor and agent orientation for this repo
```

The spec is one file so it can be read and written as a whole. Its chapters are listed in
`spec.md` itself. The glossary is its Appendix C, and the worked examples (Appendix B) are also
the acceptance suite.

### 1.2 Where each existing document goes

| Today | Goes to |
| --- | --- |
| `para-design-v4.md` §0–§10, §13–§24, §26, §28–§30 | Rewritten into `spec.md`, with amendments folded into their topic chapter |
| `para-design-v4.md` §5, §6.1 skill parts, §6.2 | Dropped (D4) |
| `para-design-v4.md` §11, §25 (decision logs) | Harvested into `decisions/` for decisions still in force |
| `para-design-v4.md` §12, §24 (reversals, scale across drafts) | Dropped. Kept in history at `poc-final` |
| `para-design-v2.md`, `para-design-v3.md`, `command-surface.md` | Deleted. Kept in history at `poc-final` |
| `docs/adr/0001–0003` | 0003 rewritten into `decisions/`; 0001 (superseded) dropped; 0002 rewritten for `list` |
| `docs/implementation-plan.md` | Replaced by a new plan in Phase 3 |
| `plans/fixes/`, `plans/para-0o6/` | Deleted (finished) |
| `docs/release-process.md` | Kept, corrected: no `impl` integration branch |
| `README.md` | Rewritten last, from the spec |

### 1.3 Writing rules for the spec

- **Normative and present tense.** It says what para does. It does not say what an earlier draft
  said, or why the author changed their mind. Reasons go in `decisions/`.
- **No cross-version narration.** No "v2 said", no "reversal", no "amendment".
- **Stable anchors, not line numbers.** Tests and ADRs cite spec sections by anchor.
- **Every feature re-earns its place.** Rewriting each chapter is the moment to cut, not just to
  copy. Section 1.4 lists the candidates.
- **One term, one meaning.** The glossary (Appendix C) is written first and the chapters use its words.

### 1.4 Questions to settle while writing the spec

1. **Feature cuts.** These are candidates to argue, not decisions: `path`, the `log` / `activity` split, filter flags versus `--where`.
2. **Dir.** Where it may sit, and how it appears in projections, `review` and `doctor`.
3. **Sensitivity labels.** Which directories can carry a label, the default, how a label
   reaches files beneath it, what para does with a label (does it show it, check it, or put it in
   `AGENTS.md`?), and how a tree with no labels behaves.
4. **Ids in practice.** Where the "every id ever issued" count lives without causing merge
   conflicts, and how `list` makes looking up an id easy (D11).

Done when: every `spec.md` chapter is reviewed, every question in 1.4 has an answer recorded in
`decisions/`, and nothing in `docs/` contradicts anything else.

## Phase 2: Fresh start on main

One PR, after Phase 1 merges:

- Remove `internal/`, `cmd/`, `testdata/` and the old design documents.
- **Kept on purpose:** `LICENSE`, `go.mod` (module path unchanged), `.goreleaser.yaml`,
  `.github/workflows/`, `scripts/install.sh` and its test, `Makefile`, `.gitattributes`, `.beads/`.
  This is release and CI machinery, not product logic, so carrying it does not break D1.
- Add an empty `cmd/para` that builds and prints its version, so CI and the installer stay green
  from the first commit.

Done when: `main` holds the spec, the release machinery and a stub binary, and CI passes.

## Phase 3: Build against the spec

- **A new implementation plan**, written from the spec, replaces `docs/implementation-plan.md`.
  It keeps the PoC plan's best rules: test first; three test layers (unit, golden renderer, script);
  the spec's worked examples transcribed into `testscript` before the code that runs them.
- **The compat fixture is written early.** It is a synthetic v0 tree that has every v0 artifact,
  skills, rules and mirrors included, built from the PoC's own output. No personal data from
  `~/brain`. The tests check that `para migrate` upgrades it (ids, stored kinds, resources to dirs,
  `CLAUDE.md` blocks removed), and that `doctor` afterwards reports only retired-key notes.
- **Port policy.** The low-level PoC packages (`ptoml`, `mdfile`, `journal`, `ptime`, `csvfile`,
  `tagexpr`) are the likely candidates. Each port is its own beads issue. It cites the spec section
  it satisfies and brings or rewrites its tests. Anything that does not fit the spec's model is
  written fresh.

Done when: every spec example passes as a script test, and the compat fixture passes.

## Phase 4: Cutover

1. Run the new binary against a **copy** of `~/brain`: `doctor`, `rebuild --dry-run`, then
   `rebuild`, and diff the result.
2. Remove the skill leftovers in `~/brain` by hand (or leave them; para no longer cares).
3. Release `v0.8.0` through the process in
   `docs/release-process.md`.

## Design inputs

- **The dir kind** (v4 called it a "Managed Directory"). `para-design-v4.md` §30 (Sep 3)
  moved `kind` into `state.toml` so that this kind could exist: a directory that holds files at a natural,
  user-chosen path such as `areas/relationships/people` or `projects/acme/evidence`, where the
  position implies no kind. §30.2 leaves the kind "deliberately unbuilt": it has no word, no
  containment row and no chain shape, and no code exists for it. Its follow-up issue was probably
  lost along with `para-sxt`. The next spec has to finish it or drop it, because the stored kind
  only exists to support it. The PR for `para-sxt` (#15) says the feature that needed it was
  `para-5ul.1`, at a path like `projects/entra/evidence`. That issue is lost too, and nothing
  else records what it was.
- **Sensitivity scoped by entity.** A classification set in an entity's
  `.para/` applies to every file beneath it, so no file carries its own label. Nothing in the
  repo records this pairing yet. The labelling rules from the Oct 20 brain session carry over:
  TLP 2.0 labels (CLEAR, GREEN, AMBER, AMBER+STRICT, RED); a nested directory may tighten its
  parent's label but never loosen it; only the user loosens a label.
- **The collision rule (spec §0.3).** Where content already sits at a path `para` needs for a file of its own or an entity's destination, `para` refuses rather than overwrite or merge. Adoption (§2.4) and migration (§9.1) must honour it; files a v0 `para` wrote count as `para`'s side.
- **`intent`'s command-resurface plan.** It is intent's own plan, not para's. It is still worth
  reading for its rules: the noun narrows lookup before an ambiguity check, the subject is
  the only bare argument, and relations are flags named after their kind. Para's v4 grammar has
  the same `verb noun` shape.

- **Provenance per file.** Where each file came from, tracked at the file level, as metadata para manages. Builds on the Oct 20 brain session's record / trusted / untrusted model. Design in Phase 1.

## Not in scope

- Any replacement for skill distribution.
- Converting `~/brain`'s skills into anything else.
- New features beyond D9. If spec writing turns up one, it gets a beads issue for after the
  cutover.
