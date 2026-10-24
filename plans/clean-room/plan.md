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
| D17 | **Terms.** An *entity* is a managed directory of one of these kinds: a project, area, objective, key result, standard, indicator, analytic, metric, term, link, dir or container. The root, buckets, archive buckets and archive stubs are managed but are not entities and have no id (D28, D32). A *container* is an entity with a reserved name that groups one kind of child, such as `objectives/` (D47). A *dir* is the basic kind of entity (D18). *Directory* stays an ordinary word for any folder on disk. *Component* means anything in the spec's tour (chapter 1). |
| D18 | **What a dir is.** It carries a journal, notes, links, `ACTIVITY.md` and a README with frontmatter, plus sensitivity and provenance; it has no status. It sits in the resources bucket or inside a project, area or another dir, never directly in the projects or areas bucket, and never contains a project or area. **Resource is no longer a kind:** the resources bucket is a hierarchy of dirs, and `para migrate` converts each `kind = "resource"` to `dir`. A dir can be archived on its own, like any entity but a container (D51): it moves to the archive bucket at a path that mirrors where it came from. To tighten one subfolder's label, use a path rule; make the subfolder a dir only when it needs more than a label. |
| D19 | **Areas have standards and indicators**, mirroring projects' objectives and key results. A standard is the qualitative bar an area keeps up. Both are optional: an area may have no standards, and a standard may have no indicators. But an indicator always sits under a standard, never directly under an area. An indicator holds an analytic to a target over a rolling window (a KPI, or an SLI with its SLO) and has a computed status (D20). |
| D20 | **The measurement library.** Analytics, metrics and terms are kinds that live in one fixed place, `resources/measurement/{analytics,metrics,terms}`; para creates it on first use (D44), and `measurement` is a reserved name. An *analytic* is a computation over metrics, with no target of its own. A *metric* is a single measurement point; its dated values are the measurements, which belong only to metrics. A *term* is a shared definition that scopes metrics. These relationships are many-to-many, so they are references by id, not nesting. A *key result* holds an analytic to a target by a due date; an *indicator* holds an analytic to a target over a rolling window. One analytic may serve any number of key results and indicators across projects and areas. |
| D21 | **Para no longer generates `CLAUDE.md`**, and the `emit.claude` key is retired. Claude Code reads `AGENTS.md` itself since 2.1.277 (root and parents at start, subfolders on first use). `para migrate` removes para's block from existing `CLAUDE.md` files and deletes any left empty. Because any `CLAUDE.md` or `CLAUDE.local.md` in or above the tree turns off Claude Code's `AGENTS.md` reading, `doctor` reports one that does not import `@AGENTS.md`. |
| D22 | **`para` never deletes or edits content, and `remove` keeps it.** `remove` deletes only `para`'s own files for the entity and everything beneath it, deepest first, so a failed run can be rerun; the plain folders left behind hold no entity, so the chain rule holds. Its own files go with it, including the journal, recorded measurements and the generated `ACTIVITY.md` and `MEASUREMENTS.csv`, so `remove` means `para` forgets the entity completely; `archive` is how to stop work and keep history. It confirms first, naming what will be lost (for example, "deletes 214 journal entries and 1,032 measurements; use `para archive` to keep them"), with `--dry-run` and `--force`; prints the content paths it left and the `rm -r` to delete them, and points to `archive` as the normal way to retire something. It strips `para`'s blocks from a README and deletes the README only if nothing else is left. Removed ids stay issued and are never reused; `doctor` reports references to them. Chosen unanimously by three user-lens reviews (person, AI agent, scripts and maintainers). |
| D23 | **Generated files live in `.para/`.** `ACTIVITY.md` and `MEASUREMENTS.csv` are written inside the entity's `.para/`, and `para`'s README block links to them. The only visible files `para` writes in are a README and `AGENTS.md` (root, buckets and archive buckets), and only inside its blocks. On `remove`, the parent's README block is regenerated and the removal is recorded in the parent's journal, so the parent's history says what became of the child. `AGENTS.md` is unaffected by `remove`. |
| D24 | **`para` owns only its own keys in README frontmatter.** Keys you add (Obsidian `tags`, `aliases` and so on) are content: `para` adds, updates and removes only the keys it writes and keeps yours. |
| D25 | **Every refusal says why and what to do instead.** It names the rule, the offending id or path, and a remedy: a command to run, or, when content is in the way, the user's own files to move. This makes the right path the easy one (§0.4); §7.10 defines how the remedy appears in text and JSON output. |
| D26 | **`para` never runs git or requires a repository.** It writes files and leaves committing, merging and syncing to you, so it works the same in any tree and never commits for you. §3.7 says how its files behave in a repository (whether it reads `.git/` or writes `.gitattributes` is settled there). Carried from the PoC (v4 §9). |
| D27 | **Only a person lowers a label or raises trust.** The command that does it requires an interactive confirmation and has no `--force` or other non-interactive path, so an agent or script cannot complete it alone. It refuses when no terminal is attached, saying a person must run it (D25). Every other loosening is refused. §6.4 defines the commands. |
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
| D39 | **An area holds its standards in `standards/`, and a standard its indicators in `indicators/`.** This mirrors `objectives/` and `key-results/`. A standard holds only its indicators and your content; an indicator holds no entities (as D37). Each container name is reserved in its parent (`objectives` and `links` in a project, `key-results` in an objective, `standards` and `links` in an area, `indicators` in a standard), so no sub-area, dir or content folder may take it. §2.2 states the containment rule. |
| D40 | **Indicator statuses are computed: `no-data`, `breached`, `at-risk`, `healthy`, first match wins.** `no-data` when the analytic has no value over the rolling window, `breached` when the analytic over the window misses the target, `at-risk` when it meets the target but is within a margin of missing it, otherwise `healthy`. The margin is a config setting with a default that passes down the tree (D29, §3.5). Nothing is set by hand: to stop tracking an indicator, remove or archive it (D51). Never stored. §4.4 defines the computation. |
| D41 | **A measurement is a dated number.** Metrics have no types: the PoC's `ratio` becomes an analytic over two metrics, and a yes/no milestone is tracked as an objective's status. One grammar, no type rules. Replaces the PoC's `number`, `ratio` and `boolean` key result types (v4 §4.1). |
| D42 | **The measurement library is flat.** Each analytic, metric and term is a folder directly in `analytics/`, `metrics/` or `terms/` and holds no other entities, only its `.para/`, README and content. They are found by id, so folders add little. |
| D43 | **A term is a definition that metrics reference.** Its definition is the text of its README, which `para` never interprets. A metric names the terms it uses by id. For every reference in the library (key result or indicator to analytic, analytic to metric, metric to term), commands refuse a missing id or the wrong kind, `show` lists the reference both ways, and `doctor` reports one whose target was removed. |
| D44 | **The measurement library is created on first use.** `init` does not create `resources/measurement/`. Adding the first analytic, metric or term creates `measurement/` and that kind's container; the other containers wait for their own first entity. `measurement` is reserved in `resources/` whether or not the library exists, so `para` refuses to create or adopt anything else under that name. A tree with no library is not a `doctor` finding. Amends D20. |
| D45 | **Staleness is set per kind and is off unless set.** Every kind that can go stale has its own `<kind>.stale-after` setting (a number of days), including `dir`, `analytic`, `metric` and `term` (but not `container`, which never goes stale, D47), as the PoC had for its kinds (v4 §7). There is no built-in default, so nothing goes stale until someone opts in. A setting passes down from wherever it is set, root included, and the nearest wins (§3.5). So a resource (a dir in `resources/`) has no status or due date and review never reports it stale unless a `dir.stale-after` reaches it; set it on `resources/` to cover only resources. `metric.stale-after` flags a metric not measured lately. §4.5 defines the clock. |
| D46 | **`init` writes starter staleness settings.** A new tree's root `config.toml` gets `project.stale-after` and `area.stale-after` values, so review flags neglect from the start; you can see, change or remove them like any setting. `para` itself has no hidden staleness defaults (D45), and every other kind stays off until set. `migrate` writes none, so a migrated tree keeps what it had. §7.3 and §3.5 give the values. |
| D47 | **A container is an entity kind of its own.** Each container (`objectives/`, `key-results/`, `standards/`, `indicators/`, `links/`, and in the library `measurement/`, `analytics/`, `metrics/` and `terms/`) has `kind = "container"`, an id and a place in the chain, so it is managed like any entity and easy to tell apart from a dir. It carries a journal (which records its children arriving and leaving, D23) and a README whose `para` block lists its children; it has no links, no staleness and no label of its own, inheriting its parent's. `measurement/` is the one container that holds containers (`analytics/`, `metrics/`, `terms/`). Its name is reserved in its parent. `para` files only the expected kind in it (objectives in `objectives/`) and refuses anything else; anything else found in it, another entity or content, is a `doctor` warning. A container never goes stale and has no `stale-after` setting, so a `dir.stale-after` never reaches one. It cannot be archived on its own (D51). Stubs keep no id of their own (D32): a stub already records the id of the entity it stands for. Replaces the non-entity container; amends D16, D17, D28, D32, D42, D44 and D45. |
| D48 | **Anything can be archived whatever its status.** Archiving moves an entity and never changes its fields, so its status still says how it ended. As in the PoC (v4 §18.5). |
| D49 | **Unarchive brings back the ancestors it needs.** Unarchiving an entity whose ancestors are also archived makes each of them live again, joins any ancestor that is already live, and leaves a stub for archived siblings that stay. It restores everything beneath it, including anything that was archived on its own before its parent was; to keep that archived, archive it again. As in the PoC (v4 §1.6). |
| D50 | **The archive is read-only.** An archived entity can be shown, listed, unarchived or removed; any other command, including a note or a measurement, needs it unarchived first. Nothing is created under `archive/` except by archiving or unarchiving (which may leave stubs). |
| D51 | **Any entity but a container can be archived on its own.** It moves with everything beneath it to its mirrored path, with stubs (D32) for live ancestors, containers included. A container stays with its parent: archiving one refuses and names its entities, and its parent when that can be archived (not for `measurement/`, whose parent is a bucket),, because a new container made in its place could never rejoin it. When an entity is archived, every stub that stands for it or a descendant gives way to that entity at every depth, matched by id, an exception to §0.3's collision rule; `para` refuses only when two archived entities would share a path (D25). Resolves the open "what can be archived alone" question. |
| D52 | **Containers are created on first use.** A container appears when its first child is added, as `measurement/` does (D44), so a new project has no empty `objectives/` or `links/`. Resolves the open "when containers exist" question for creation; what happens when a container's last child goes is for §2.5. |

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
