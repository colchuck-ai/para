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
| D3 | **Existing trees keep working in place.** The next revision reads a v0 tree (e.g. `~/brain`) with no migration step. Skills are the only exception (D4). |
| D4 | **Skills are removed, with no migration.** Distributing skills is a separate problem that para keeps out of scope. Skills can be layered on top of a para tree by other tools. |
| D5 | **`AGENTS.md` and `CLAUDE.md` stay** as marker-scoped generated blocks. `CLAUDE.md` loses its rule imports. |
| D6 | **`skill` and `skills` are ordinary names again.** Both leave the reserved-word list. |
| D7 | **The tree format stays at its current version, `schema = 1`** (what `~/brain` and the code use). `schema = 2` and `para migrate` were designed in §30.5 but never built. |
| D8 | **The first clean-room release is `v0.8.0`.** |
| D9 | **Managed Directories and sensitivity labels are designed as part of the new spec** and ship in `v0.8.0`. Storing `kind` in `state.toml` was the first step toward them. The design is worked through together, chapter by chapter, in Phase 1. |

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
- `emit.cursor` and `emit.claude-skills` in `config.toml` are read as **retired keys**: ignored,
  with one `doctor` note saying they can be removed.
- The first `rebuild` rewrites the `CLAUDE.md` block without the `@.agents/rules/…` imports. This
  is the only v0 file the next revision changes as a result of D4.
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
docs/
  spec/                      normative: what para does, present tense
    00-principles.md
    01-model.md              tree shapes, kinds, containment, addressing
    02-storage.md            tree.toml, state.toml, journals, config, git posture
    03-projections.md        README frontmatter, ACTIVITY.md, MEASUREMENTS.csv, AGENTS.md, CLAUDE.md
    04-cli.md                grammar, verbs, fields, filters, output, exit codes
    05-review-doctor.md      review groups, doctor findings, rebuild
    06-features.md           key results and measurements, suppression, links
    07-managed-dirs.md       Managed Directories and sensitivity labels (new)
    08-compat.md             reading a v0 tree; retired keys and files
    examples/                worked examples, which are also the acceptance suite
  glossary.md                one definition per term
  decisions/                 short ADRs, current decisions only
  release-process.md
AGENTS.md / CLAUDE.md        contributor and agent orientation for this repo
```

### 1.2 Where each existing document goes

| Today | Goes to |
| --- | --- |
| `para-design-v4.md` §0–§10, §13–§24, §26, §28–§30 | Rewritten into `spec/`, with amendments folded into their topic chapter |
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
- **One term, one meaning.** `glossary.md` is written first and the chapters use its words.

### 1.4 Questions to settle while writing the spec

1. **Feature cuts.** These are candidates to argue, not decisions: `migrate` (is there anything left
   for it to do?), `path`, the `log` / `activity` split, filter flags versus `--where`.
2. **Stored kind in a `schema = 1` tree.** Managed Directories (D9) need a stored `kind`, and
   §30.5 planned a schema bump to 2 plus `para migrate` to fill it in. D7 rules out the bump. The
   likely answer: a missing `kind` means the kind its position implies, and `add` writes `kind`
   for every new directory. Settle this in `01-model.md`.
3. **`emit.claude`.** With no rule imports, the block is `@AGENTS.md` plus a note. Is
   `emit.claude` still worth a config key, or is `CLAUDE.md` always written?
4. **Managed Directory.** Its word, where it may sit, its address form (including the ambiguous
   chain that §30.2 flags), and how it appears in projections, `review` and `doctor`.
5. **Sensitivity labels.** Which directories can carry a label, the default, how a label
   reaches files beneath it, what para does with a label (does it show it, check it, or put it in
   `AGENTS.md`?), and how a v0 tree with no labels reads.

Done when: every `spec/` chapter is reviewed, every question in 1.4 has an answer recorded in
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
  `~/brain`. The tests check that the new binary reads it, `doctor` reports only the retired-key
  note, and `rebuild` changes nothing except the `CLAUDE.md` block.
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

- **The Managed Directory kind.** `para-design-v4.md` §30 (Sep 3) moved `kind` into `state.toml`
  so that a "Managed Directory" could exist: a directory that holds files at a natural,
  user-chosen path such as `areas/relationships/people` or `projects/acme/evidence`, where the
  position implies no kind. §30.2 leaves the kind "deliberately unbuilt": it has no word, no
  containment row and no chain shape, and no code exists for it. Its follow-up issue was probably
  lost along with `para-sxt`. The next spec has to finish it or drop it, because the stored kind
  only exists to support it. The PR for `para-sxt` (#15) says the feature that needed it was
  `para-5ul.1`, at a path like `projects/entra/evidence`. That issue is lost too, and nothing
  else records what it was.
- **Sensitivity scoped by Managed Directory.** A classification set in a Managed Directory's
  `.para/` applies to every file beneath it, so no file carries its own label. Nothing in the
  repo records this pairing yet. The labelling rules from the Oct 20 brain session carry over:
  TLP 2.0 labels (CLEAR, GREEN, AMBER, AMBER+STRICT, RED); a nested directory may tighten its
  parent's label but never loosen it; only the user loosens a label.
- **`intent`'s command-resurface plan.** It is intent's own plan, not para's. It is still worth
  reading for its rules: the noun narrows lookup before an ambiguity check, the subject is
  the only bare argument, and relations are flags named after their kind. Para's v4 grammar has
  the same `verb noun` shape.

## Not in scope

- Any replacement for skill distribution.
- Converting `~/brain`'s skills into anything else.
- New features beyond D9. If spec writing turns up one, it gets a beads issue for after the
  cutover.
