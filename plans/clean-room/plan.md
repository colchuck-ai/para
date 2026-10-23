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
| D7 | **`para migrate` moves a tree from `schema = 1` to `schema = 2`**, adding an id and a stored `kind` to every entity in one pass. v0.8.0 reads and writes only schema 2. (Reverses the earlier "stay at schema 1".) |
| D8 | **The first clean-room release is `v0.8.0`.** |
| D9 | **Dirs and sensitivity labels are designed as part of the new spec** and ship in `v0.8.0`. Storing `kind` in `state.toml` was the first step toward them. The design is worked through together, chapter by chapter, in Phase 1. |
| D10 | **Addressing is by a generated, stored id, not by path.** The id is the key and survives moves and re-parenting; it resolves to the thing's name and path for people. Design details are settled in Phase 1 (§2.3 of `spec.md`). |
| D11 | **Commands take an id only.** Names and paths are not accepted as addresses; an id is looked up first (with `list`), so every output shows ids. |
| D12 | **Ids are a tree prefix plus a short random code**, like beads (`brain-7k2q`). The code grows longer as the number of ids grows, to keep collisions unlikely. The scaling rule follows a study of beads. |
| D13 | **Id rules.** Code is random (not a content hash) over lowercase `0-9a-z`; at least 4 characters; length chosen so the chance of any collision stays at or below 1%, counting every id ever issued in the tree, so lengths only grow (4 up to ~180 ids, 5 up to ~1,100, 6 up to ~6,600). Prefix is letters and digits, no hyphen. No child ids. Local collisions are retried at creation; `doctor` reports duplicate ids, which only a merge can produce. Studied from beads; see the Phase 1 notes. |
| D14 | **`para migrate` upgrades a tree explicitly.** It assigns ids to a v0 tree (and runs any later format change). Other commands refuse an unmigrated tree and name the command. Amends D3. |
| D15 | **Sensitivity is an inheritance tree of entities.** Each entity's `config.toml` sets its sensitivity and may name paths beneath it, including unmanaged ones, that need a stricter label. A path rule never reaches into a child entity: the child classifies everything inside itself, so two configs never claim the same file. A child entity may only match or tighten its parent's label, never lower it. Path rules are relative to their own directory and may not climb out of it: no `..` segments, no absolute paths, and no symlink whose target resolves outside the directory. |
| D16 | **Entities form an unbroken chain from the root.** Every entity's parent is itself managed by para (the root, a bucket, a container such as `objectives/`, or another entity), so an unmanaged directory can never hold an entity. Commands refuse to create a gap, and `doctor` reports one made by hand (the PoC's `orphan` finding). Adopting an existing folder therefore works from the top down. |
| D17 | **Terms.** An *entity* is any directory para manages: a project, area, objective, key result, standard, indicator, analytic, metric, term, link or dir. A *dir* is the basic kind of entity (D18). *Directory* stays an ordinary word for any folder on disk. *Component* means anything in the spec's tour (chapter 1). |
| D18 | **What a dir is.** It carries a journal, notes, links, `ACTIVITY.md` and a README with frontmatter, plus sensitivity and provenance; it has no status. It sits in the resources bucket or inside a project, area or another dir, never directly in the projects or areas bucket, and never contains a project or area. **Resource is no longer a kind:** the resources bucket is a hierarchy of dirs, and `para migrate` converts each `kind = "resource"` to `dir`. A dir can be archived on its own, the same way as anything else: it moves to the archive bucket at a path that mirrors where it came from. To tighten one subfolder's label, use a path rule; make the subfolder a dir only when it needs more than a label. |
| D19 | **Areas have standards and indicators**, mirroring projects' objectives and key results. A standard is the qualitative bar an area keeps up. Both are optional: an area may have no standards, and a standard may have no indicators. But an indicator always sits under a standard, never directly under an area. An indicator holds an analytic to a target over a rolling window (a KPI, or an SLI with its SLO) and has a computed status (D20). |
| D20 | **The measurement library.** Analytics, metrics and terms are kinds that live in one fixed place, `resources/measurement/{analytics,metrics,terms}`; para creates it, and `measurement` is a reserved name. An *analytic* is a computation over metrics, with no target of its own. A *metric* is a single measurement point; its dated values are the measurements, which belong only to metrics. A *term* is a shared definition that scopes metrics. These relationships are many-to-many, so they are references by id, not nesting. A *key result* holds an analytic to a target by a due date; an *indicator* holds an analytic to a target over a rolling window. One analytic may serve any number of key results and indicators across projects and areas. |
| D21 | **Para no longer generates `CLAUDE.md`**, and the `emit.claude` key is retired. Claude Code reads `AGENTS.md` itself since 2.1.277 (root and parents at start, subfolders on first use). `para migrate` removes para's block from existing `CLAUDE.md` files and deletes any left empty. Because any `CLAUDE.md` or `CLAUDE.local.md` in or above the tree turns off Claude Code's `AGENTS.md` reading, `doctor` reports one that does not import `@AGENTS.md`. |
| D22 | **`para` never deletes or edits content, and `remove` keeps it.** `remove` deletes only `para`'s own files for the entity and everything beneath it, deepest first, so a failed run can be rerun; the plain folders left behind hold no entity, so the chain rule holds. Its own files go with it, including the journal, recorded measurements and the generated `ACTIVITY.md` and `MEASUREMENTS.csv`, so `remove` means `para` forgets the entity completely; `archive` is how to stop work and keep history. It confirms first, naming what will be lost (for example, "deletes 214 journal entries and 1,032 measurements; use `para archive` to keep them"), with `--dry-run` and `--force`; prints the content paths it left and the `rm -r` to delete them, and points to `archive` as the normal way to retire something. It strips `para`'s blocks from a README and deletes the README only if nothing else is left. Removed ids stay issued and are never reused; `doctor` reports references to them. Chosen unanimously by three user-lens reviews (person, AI agent, scripts and maintainers). |

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
