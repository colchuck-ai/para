# para — design

**Status:** Proposed
**Date:** 2026-07-29
**Supersedes:** the all-prose `brain/SKILL.md`

---

## What para is

**A build system for a PARA knowledge base.**

`para.yml` is the source of truth. The directory tree, every `AGENTS.md`, and the agent skill are compiled output. Your content — notes, documents, READMEs — is yours, and para never reads or writes it.

The governing principle: **para owns form, you own content.**

| Layer | What | Rule |
|---|---|---|
| **Source** | `para.yml` | the only thing you edit that para reads |
| **Compiled** | directories, `AGENTS.md` files, `.claude/skills/para/SKILL.md` | never hand-edit; regenerated freely |
| **Content** | everything inside the directories, all `README.md` files | para never touches it |

`AGENTS.md` being *output* rather than *source* is the keystone. It dissolves three problems a prose-only skill cannot solve:

- **Duplication** — one source, N compiled views. Repetition in generated output is free.
- **Clobbering** — build artifacts are meant to be overwritten. There is no hand-authored content to destroy.
- **Drift** — becomes a staleness check, trivially decidable, rather than a semantic judgment.

## Why this exists

The predecessor was a 95-line skill that passed every conformance check and still felt complicated. Four problems were tangled together:

1. **Two unrelated domains in one skill.** Structure-and-lifecycle and note-taking method are independent concerns with independent triggers. The zettelkasten half already exists as separate skills in the same org.
2. **Deterministic work expressed as prose.** Resolving guidance for a path, scaffolding a directory, checking that a procedure has a trigger and enumerated steps — all pure functions of the manifest, all written as instructions an agent might or might not follow.
3. **The skill and its template were two copies of one contract.** The procedure-schema sentence appeared eight times across the repo.
4. **A prose rubric cannot enforce itself.** The skill's own audit procedure told agents to verify links resolve and filenames are conventional. Run it against the repo's own template and it failed — see [Appendix: the case bug](#appendix-the-case-bug).

---

## `para.yml`

```yaml
version: 1

config:
  max-depth: { area: 3, directory: 3 }
  stale-after: 90
  max-high-priority: 3

library:                                # reusable hook definitions; attach with `use:`
  log-a-decision:
    name: Log a decision
    when: A material choice is made about direction, scope, or tooling.
    steps:
      - Append an entry to <scope>/decisions.md.
      - Record the date, the choice, and the alternatives rejected.
  record-progress:
    name: Record progress
    when: The user reports having worked a project, including work done elsewhere.
    steps:
      - Run `para progress <project-id>`.
      - If the work changed what "done" means, update the objectives.
  cite-the-source:
    name: Cite the source
    when: Writing down something learned from a person or a page.
    steps:
      - Add a *Source:* line as the first line of the note.

root:                                   # guidance for the repo root
  use: [log-a-decision]
  hooks:
    start-of-week:
      name: Start of week
      when: The user asks what to focus on.
      steps:
        - List in-progress projects by priority with their objectives.
        - Name anything blocked and what it is blocked on.

projects:                               # guidance for projects/ as a whole
  use: [record-progress]
  hooks:
    triage-an-idea:
      name: Triage an idea
      when: Someone proposes work that may or may not be a project.
      steps:
        - Decide time-bound (project) vs ongoing (area).
        - If a project, run `para new project` and write at least one objective.
  items:
    redesign-api:
      name: Redesign the API
      summary: Replace v1 endpoints with a versioned contract.
      status: blocked
      blocked-by: Vendor contract unsigned; cannot publish the sunset date.
      priority: high
      check-progress-every: 14
      created-on: 2026-03-14
      status-updated-on: 2026-07-12
      progress-on: 2026-07-24
      objectives:
        versioned-contract:
          name: Ship a versioned API contract customers can rely on.
          key-results:
            v2-adoption:
              name: v2 endpoints serve 100% of new integrations.
            dated-sunset:
              name: v1 deprecation notice live with a dated sunset.
        support-burden:
          name: Reduce the integration support burden.
          key-results:
            ticket-volume:
              name: Tickets tagged api-v1 down 50% quarter over quarter.
      use: [log-a-decision]
      hooks:
        ship-checklist:
          name: Ship checklist
          when: About to cut a release.
          steps:
            - Verify every v1 deprecation notice carries a date.
            - Regenerate the OpenAPI document and commit it.
      items:
        research:                       # no kind — projects hold only directories
          name: Research
          summary: Prior art feeding the contract design.
          items:
            interviews:
              name: Customer interviews
              use: [cite-the-source]

areas:                                  # guidance for areas/ as a whole
  use: [cite-the-source]
  hooks:
    is-this-still-yours:
      name: Check an area is still yours
      when: An area has gone quiet for a quarter.
      steps:
        - Ask whether the responsibility still exists.
        - Prefer archiving to removing; the record stays useful.
  items:
    finances:
      name: Finances
      summary: Money, taxes, and household accounts.
      status: active
      review-every: 30
      created-on: 2026-01-08
      status-updated-on: 2026-01-08
      reviewed-on: 2026-07-02
      items:
        receipts:
          kind: directory               # required: an area's items are ambiguous
          name: Receipts
          summary: Scanned receipts.
          hooks:
            name-a-receipt:
              name: Name a receipt
              when: Adding a scanned receipt.
              steps:
                - Name the file <scope>/<YYYY-MM-DD>-<vendor>.pdf
        taxes:
          kind: area
          name: Taxes
          summary: Annual filing and supporting records.
          status: active
          review-every: 90
          created-on: 2026-01-08
          status-updated-on: 2026-01-08
          reviewed-on: 2026-04-15
          items:
            filings:
              kind: directory
              name: Filings
              summary: Submitted returns by year.

resources:
  items:
    prompt-library:
      name: Prompt library
      summary: Prompts and scaffolds worth keeping.
      status: active
      created-on: 2026-02-20
      status-updated-on: 2026-02-20
      items:
        hooks:                          # a directory named "hooks" — no collision
          name: Hook prose drafts
```

Compiles to:

```
projects/redesign-api/research/interviews/
areas/finances/receipts/
areas/finances/taxes/filings/
resources/prompt-library/hooks/
archive/
```

### Why `items:`

Structural keys and user-chosen IDs must never share a YAML mapping. Without `items:`, a container reads `projects: { hooks: {…}, redesign-api: {…} }` — so a project named `hooks` is unrepresentable and a reader cannot tell structure from data.

Nesting user keys one level down fixes it. Inside an entity every key is already structural (`name`, `summary`, `status`, `priority`, dates, `objectives`, `use`, `hooks`, `items`), and user keys appear only inside the homogeneous `items:`, `hooks:`, `objectives:`, and `key-results:` maps.

**One `items:` per node** is what makes duplicate compiled paths structurally impossible — a name can appear at most once beneath a node — and it makes path elision a theorem rather than a rule needing enforcement.

### Why `kind:`

An area's `items:` may hold sub-areas *or* directories, and `receipts` versus `taxes` is indistinguishable from position, field set, or path. So `kind:` is required there, with values `area` and `directory`.

It appears **only under an area**, because nowhere else is ambiguous: a child of `projects.items` is a project, and a child of a project, a resource, or a directory can only be a directory. Written elsewhere it is accepted and validated; para never writes what it can infer.

**Required, not defaulted.** There is no default to forget, so omitting `kind` is a loud error rather than a silent demotion of a sub-area into a bucket — which is the one failure mode where the wrong answer would have been quiet.

### Other schema notes

**Keys are IDs and directory names.** Every collection is a keyed map, so duplicates are impossible and every node has a stable handle. The prose display string is always `name`. Uniqueness is per-sibling, not global — nesting gives that for free.

**Naming is emergent.** Whatever case you write in a key appears on disk. para has no style opinion. Two rules survive as *legality*: a key must be a usable path segment, and sibling keys differing only by case are rejected because they collide on case-insensitive filesystems.

**Paths are opinionated.** `projects/`, `areas/`, `resources/`, `archive/`. Changing these is a breaking change to the binary, not a config edit.

**Objectives are projects-only**, keyed, and each owns its own key results — never shared. Key results stay prose strings rather than tracked booleans, which preserves the division of labour: **the manifest declares intent; the agent evaluates reality against it.**

**`when` is prose, not an event enum.** "Hook" nudges toward `on: create`, but the useful triggers are user intent, not para's lifecycle.

**`<scope>` substitutes to the emitting directory** at compile time. That is what makes one library hook correct in several places.

---

## Status

| Kind | States | Lives in |
|---|---|---|
| project | `to-do` · `in-progress` · `blocked` | `projects/` |
| | `done` · `dropped` | `archive/projects/` |
| area | `active` | `areas/` |
| | `archived` | `archive/areas/` |
| resource | `active` | `resources/` |
| | `archived` | `archive/resources/` |
| directory | none | moves with its owner |

**Required on projects, defaulted on areas and resources.** None of the five project states is the boring case, so requiring it is honest; `para new project` writes `to-do`. For areas and resources `active` is almost always true, so it is omitted unless archived.

**`blocked-by` is required when `status: blocked`**, and is a `node.field-not-allowed` error otherwise. A blocker with no recorded reason is worthless in six months, and it is the difference between `project.long-blocked` being a nag and being actionable.

**`done` versus `dropped`** records the one fact you cannot reconstruct. Whether a project shipped or was abandoned is not recoverable from the tree or from git history, and it is exactly what you want to know later — especially next to its objectives, which say what "done" was supposed to mean.

**Areas get no middle state.** A responsibility you are not tending is either still yours (`active`, possibly neglected) or no longer yours (`archived`). "Paused responsibility" is usually a project in disguise.

**Archived cascades by containment.** Effective status is *own status, unless any ancestor is archived*. `para archive areas.finances` writes status on `finances` only and leaves descendants untouched, so a sub-area archived independently last year stays archived when its parent is restored. A child's `status: active` under an archived ancestor is dormant, not a finding.

**Deliberately excluded:** user-defined states. That is a workflow engine, and para is not one.

## Priority

```yaml
priority: high | normal | low            # default: normal, omitted
```

Projects and areas only. `high` surfaces first in the compiled index; `low` suppresses staleness findings, because a consciously deprioritised thing going quiet is not news.

Every priority field ever built inflates until everything is high. **The top tier is capped and the cap is checkable** — `priority.inflation` fires when more than `max-high-priority` entries sit at the top tier: *"you have 9 high-priority projects; that is not a priority list."* That turns a field that always inflates into one that self-corrects.

## Dates

| Field | Project | Area | Resource | Written on |
|---|:--:|:--:|:--:|---|
| `created-on` | ✓ | ✓ | ✓ | creation |
| `status-updated-on` | ✓ | ✓ | ✓ | status transition |
| `progress-on` | ✓ | — | — | `para progress` |
| `check-progress-every` | ✓ | — | — | set by hand; static |
| `reviewed-on` | — | ✓ | — | `para review` |
| `review-every` | — | ✓ | — | set by hand; static |

ISO dates, day granularity — no times, no timezones. The CLI writes them because it has a clock; agents get dates wrong. Directories carry no dates.

**`progress-on` and `reviewed-on` are asserted, not measured.** This is the important distinction. mtime is the wrong signal for two reasons: it is moved by trivial edits, and **most meaningful work on a project happens outside its directory** — a PR in a code repo, a vendor call, a decision in a meeting. An mtime-only staleness check is therefore systematically wrong for exactly the projects with the most real activity.

So a project `in-progress` for eight months with `progress-on` last week is **not stale**. Time-in-state and time-since-progress are different questions.

**`check-progress-every` and `review-every`** are per-entity overrides of `stale-after`. One global number cannot say that a slow-burn infrastructure project legitimately goes 90 days between progress while a launch project going 14 days silent is a problem — or that finances wants monthly attention and a hobby wants yearly.

**Rejected dates, and why:**

- **`updated-on`** — either stamped on every manifest edit (churn in every diff, conflicts on every concurrent edit) or a duplicate of mtime guaranteed to drift.
- **`archived-on`** — redundant. When status becomes `done`, `dropped`, or `archived`, `status-updated-on` *is* the archive date.

---

## Compilation

`para build` emits:

- **Directories** for every manifest entry
- **`AGENTS.md` at every para-managed directory** — every container, project, area, sub-area, and directory, whether or not it has hooks of its own
- **Container indexes** in `projects/AGENTS.md`, `areas/AGENTS.md`, `resources/AGENTS.md`
- **`.claude/skills/para/SKILL.md`**

### Why every managed directory gets a file

The [AGENTS.md convention](https://agents.md/) specifies **nearest-only** resolution:

> "Place another AGENTS.md inside each package. Agents automatically read the nearest file in the directory tree, so the closest one takes precedence and every subproject can ship tailored instructions."

The agent reads **one file** — a closer file supersedes a further one rather than merging with it. Two consequences drive the whole model:

1. **Emitting only where hooks exist is broken.** A project with no `AGENTS.md` falls back to `projects/AGENTS.md`, whose `<scope>` substitutions resolved to `projects/` and whose heading describes the container. Every path in it is wrong for the directory the agent is in.
2. **Inheritance cannot be left to the harness.** A hook at `projects/` is invisible to an agent in `projects/redesign-api/` once that directory has its own file.

This also explains the predecessor skill: its central instruction — *"read every AGENTS.md from repo root down and apply them as an overlay"* — asked agents to do the opposite of what the convention specifies. It was a workaround for a spec that deliberately chose nearest-wins.

**Unmanaged content directories work for free.** `projects/redesign-api/notes/` has no `AGENTS.md`, so an agent there reads `projects/redesign-api/AGENTS.md` — the nearest managed ancestor, whose `<scope>` paths are correct for that subtree.

### Inheritance renders as a digest

Own hooks render in full. Inherited hooks render as **name + when + a link to the steps** where they are defined.

Full materialisation repeats entire procedures at every level. Omitting inherited hooks makes them unreachable under nearest-only resolution. The digest repeats one line per hook, so an agent knows a hook exists and what triggers it without a second read, and follows the link only when the hook fires.

**Deliberate exception to DRY:** an entity's `summary` appears both in its container's index and in its own file header. Strict DRY would strip one and both get worse — an index without summaries cannot orient you, and a document without a header does not say what it is. Objectives and hooks live only in the entity's own file.

## Compiled output

**A project** — `projects/redesign-api/AGENTS.md`:

```markdown
<!-- Generated by para v1.2.0 from para.yml. Do not edit — run `para build`. -->

# Redesign the API

Replace v1 endpoints with a versioned contract.

**Status** — blocked since 2026-07-12: vendor contract unsigned; cannot publish
the sunset date. Last progress 2026-07-24.

## Objectives

**Ship a versioned API contract customers can rely on.**
- v2 endpoints serve 100% of new integrations.
- v1 deprecation notice live with a dated sunset.

**Reduce the integration support burden.**
- Tickets tagged api-v1 down 50% quarter over quarter.

## Hooks

Follow these when the described situation arises.

### Ship checklist

**When** — About to cut a release.

1. Verify every v1 deprecation notice carries a date.
2. Regenerate the OpenAPI document and commit it.

## Inherited hooks

These also apply here. Steps are in the linked file.

| Hook | When | |
|---|---|---|
| Record progress | The user reports having worked a project. | [steps](../AGENTS.md#record-progress) |
| Triage an idea | Someone proposes work that may or may not be a project. | [steps](../AGENTS.md#triage-an-idea) |
| Log a decision | A material choice is made about direction, scope, or tooling. | [steps](../../AGENTS.md#log-a-decision) |
```

**A container** — `projects/AGENTS.md`:

```markdown
<!-- Generated by para v1.2.0 from para.yml. Do not edit — run `para build`. -->

# Projects

Time-bound work with a defined end state.

## Hooks

### Record progress

**When** — The user reports having worked a project, including work done elsewhere.

1. Run `para progress <project-id>`.
2. If the work changed what "done" means, update the objectives.

### Triage an idea

**When** — Someone proposes work that may or may not be a project.

1. Decide time-bound (project) vs ongoing (area).
2. If a project, run `para new project` and write at least one objective.

## Inherited hooks

| Hook | When | |
|---|---|---|
| Log a decision | A material choice is made about direction, scope, or tooling. | [steps](../AGENTS.md#log-a-decision) |

## Blocked

| | Name | Blocked on | Since |
|---|---|---|---|
| [redesign-api/](redesign-api/) | Redesign the API | Vendor contract unsigned | 2026-07-12 |

## In progress

*(none)*

## To do

*(none)*

## Archived

| | Name | Outcome | Since |
|---|---|---|---|
| [../archive/projects/legacy-auth/](../archive/projects/legacy-auth/) | Legacy auth migration | done | 2026-02-11 |
```

Grouping by status is what makes a *"what should I work on?"* hook answerable from one file read. Anchors are deterministic — `### Triage an idea` slugs to `#triage-an-idea` — so digest links are generated, not guessed.

---

## Commands

| Command | Job |
|---|---|
| `para init [dir]` | scaffold `para.yml`, the four directories, READMEs, and the skill |
| `para new project\|area\|resource <path> --name --summary` | add to manifest, then build |
| `para import [path]` | adopt untracked directories; directory name becomes the key |
| `para objective add\|edit\|remove <path> <key> --name` | manage objectives |
| `para kr add\|edit\|remove <path> <objective-key> <kr-key> --name` | manage key results |
| `para hook add\|edit\|remove <key> [--path <p>] --name --when --step --step` | library when `--path` omitted, inline when given |
| `para hook use\|unuse <path> <key>` | attach or detach a library hook |
| `para progress <path>` | set `progress-on` to today |
| `para review <path>` | set `reviewed-on` to today |
| `para status <path> <state> [--blocked-by …]` | set status and `status-updated-on` |
| `para move <path> <destination>` | reclassify between scopes |
| `para archive <path>` / `para restore <path>` | flip status and relocate |
| `para remove <path>` | delete a subtree |
| `para build` | compile manifest → tree, `AGENTS.md`, skill |
| `para check` | schema, tree/manifest agreement, build freshness, links, health |
| `para tree` | print the reconstructed tree to stdout |

**Path addressing** is dotted with `items:` elided: `projects.redesign-api`, `areas.finances.taxes.filings`. Resolution walks exactly one `items:` map per hop, so there is nothing to disambiguate. `/` is accepted on input so you can paste from `ls`; output and errors always use `.`.

**Steps are passed as repeated `--step` flags.** The interface's weakness is aligned with the design's goal: long multi-line steps are painful to pass, which pushes detail toward linked files and keeps procedures to followable one-liners. A JSON interface would make 40-line steps *comfortable*, which is the wrong incentive.

Two accommodations for the one place flags fight the shell — markdown code spans and English contractions collide in quoting, and double-quoting content containing backticks is command substitution:

- `--steps-stdin` for content that fights the shell
- `para hook show <key> --as-command` emits the invocation that recreates a hook, recovering round-tripping within the flags paradigm

**Dropped:** `search`, `context`, `graph`. Agents read files with their own tools, and compile-time resolution removed any need for `context`.

## Invariants

1. **`build` creates and compiles. It never deletes and never relocates.** This is the command you will run constantly and wire into hooks, so it must be safe to run anywhere, anytime.
2. **`archive`, `move`, and `restore` relocate. `remove` deletes.** Only explicit verbs touch what already exists.
3. **`remove` is deliberately blunt** — it takes the whole subtree, tracked and untracked children alike, and is unrecoverable. `--dry-run` is available; there is no confirmation prompt and no git precondition. `SKILL.md` states this plainly.
4. **para never touches content or READMEs.** READMEs are created once at `init` and are yours permanently after.
5. **Untracked is not ignored.** A directory on disk but absent from the manifest is a `check` finding, resolved by `para import` or by you.

## `para check`

**Errors — mechanically decidable**

| Rule | Checks |
|---|---|
| `schema.invalid` | manifest fails the schema |
| `node.kind-missing` / `node.kind-not-allowed` | missing `kind` under an area; illegal kind for the position |
| `node.field-not-allowed` | `objectives` on an area, `status` on a directory, `blocked-by` without `status: blocked` |
| `node.invalid-key` / `node.key-collision` | unusable path segment; siblings differing only by case |
| `tree.untracked` / `tree.missing` | disk and manifest disagree |
| `build.stale` | compiled output older than the manifest |
| `link.unresolved` / `link.case-mismatch` | broken links; case differing from **git's index** |
| `hook.unresolved-use` | `use:` names a missing library hook |

**Needs review — judgment required**

| Rule | Reads | Suppressed by |
|---|---|---|
| `project.no-progress` | `progress-on`, else `status-updated-on`, vs `check-progress-every` ?? `stale-after` | `status: blocked`, `priority: low` |
| `project.never-started` | `status-updated-on` while `to-do` | `priority: low` |
| `project.long-blocked` | `status-updated-on` while `blocked` | — |
| `area.unreviewed` | `reviewed-on`, else `status-updated-on`, vs `review-every` ?? `stale-after` | `priority: low` |
| `priority.inflation` | count at `high` vs `max-high-priority` | — |
| `hook.unused` | library hook nothing references | — |
| `depth.area-exceeded` / `depth.directory-exceeded` | over `config.max-depth` | — |
| `scope.misfiled` | area README reads as a deliverable; project has no end state | — |
| `readme.contains-procedures` | agent-facing prose in a README | — |

`project.long-blocked` has no suppressor on purpose. A blocker you have stopped questioning is the most expensive silence in the system, and `blocked-by` being required means the finding can always say what you are waiting on.

The `status-updated-on` fallback matters: without it, a project nobody annotates can never be flagged — which is exactly the neglected one. *"In-progress for eight months with no progress ever recorded"* is a legitimate finding. It also keeps the rule manifest-only.

**Every core rule is derivable from the manifest plus file existence.** No mtimes, no file counts. So `para check` has a one-line description: *does the tree match the manifest, and is the manifest coherent?*

### Why depth is `needs-review` and not an `error`

`error` is reserved for *the output would be wrong* — an unparseable manifest, a broken link, a stale build. A four-level tree compiles perfectly and nothing downstream breaks. It is **unwise, not wrong**, and putting PARA hygiene advice in the same severity bucket as a broken manifest devalues the bucket.

**Enforcement lives at creation instead, which is where it actually binds.** A managed directory can only appear via `para new` or `para import` — you cannot `mkdir` your way into one, since an unregistered directory is `tree.untracked`, not a depth violation. So the creation-time gate catches essentially everything:

```
$ para new directory areas.finances.taxes.filings.2026
  This would sit 4 levels below areas/ (max-depth.area = 3).
  Deep material usually means one of two things:
    1. It is reference material   -> resources/
    2. `taxes` is really a project
  Refusing. Pass --force to create anyway.
```

That leaves the check rule firing on only two populations: **hand-edited manifests**, where you typed the nesting deliberately, and **imported trees**, where the depth predates para. Making either a hard failure means `para check` fails on day one of adopting an existing repo, before you have learned the tool — hostile at exactly the wrong moment.

`max-depth` stays the config name. It is a real maximum for creation; check simply does not escalate deviation to a failure.

The two severities make the seam visible in tool output: an agent fixes every `error` and reasons about every `needs-review` without having to remember which half of the design it is in.

**Exit codes:** `0` clean, `1` error findings, `2` only `needs-review` findings, `64` usage error, `65` not a para repo. The 1-versus-2 split lets CI gate on mechanical failures while an agent learns from the exit code alone whether judgment is required.

**`link.case-mismatch` must compare against `git ls-files`**, not directory entries. A filesystem check finds the working-tree name and passes, reproducing exactly the blind spot in the appendix. It needs a test on both case-sensitive and case-insensitive filesystems.

**Self-check in CI.** para ships an embedded template; if that template violates para's own rules, every new repo inherits the bug.

```bash
para init /tmp/fixture
para check /tmp/fixture      # must exit 0
para build /tmp/fixture      # must produce no diff — idempotent
```

## Tracked and untracked

`para init` on an existing tree leaves your directories in place and untracked. `para check` reports them. `para import` adopts them with a plain directory-name-to-key conversion; you then add `name` and `summary`. Only after import does para consider a directory its business.

## The skill

Roughly 25 lines, written into the repo by `para init` at `.claude/skills/para/SKILL.md`.

**Why repo-local rather than globally installed:** one install channel, so having the skill implies having the binary. Version skew is structurally impossible because the binary emits the skill it was built with. And it cannot misfire in unrelated code repositories, because it is not installed there — the activation guard is placement, not prompt engineering.

Three judgments remain that no CLI can make:

1. **Placement** — project, area, or resource?
2. **Lifecycle** — is this actually done? Is this still a responsibility?
3. **Durability** — does this convention deserve to become a hook?

On the third: the agent **proposes hooks in chat**, showing the exact `para hook add …` command it would run, and writes only on approval. Nothing about proposals lives in the manifest. That keeps workflow state out of source and avoids both failure modes — silent durable edits to your knowledge base, and conventions that stay tacit because nobody remembered para exists.

The skill also carries the contract that used to be duplicated eight times: the `README.md`-versus-`AGENTS.md` audience rule and the hook schema. It can safely hold them precisely *because* it is generated and version-locked.

Gotchas it states explicitly:

- `error` findings are yours to fix; `needs-review` findings are yours to **judge** — never auto-fix them.
- `para remove` is unrecoverable. Run `--dry-run` first.
- `para init` refuses a non-empty target. Do not pass `--force` without asking.
- Dates come from the CLI, never from you.
- A hook at `--path projects` applies to the `projects/` directory only — **not** to each project inside it. Inheritance flows down from it, but the hook is anchored there.
- If `para --version` fails, tell the user how to install and stop. There is no prose fallback.

## Packaging

Go, with `go:embed` for the template. That eliminates a defect in the predecessor, which told agents the template lived at `.cursor/skills/brain/assets/brain-template/` — harness-specific, wrong under Claude Code, and unknowable from inside a subprocess.

- **Install:** `brew install para`, with `go install github.com/colchuck-ai/para/cmd/para@latest` for Go users and raw binaries on the GitHub release.
- **Version pinning:** mise (`.mise.toml`) pins the version per directory. This is *correctness*, not hygiene — `AGENTS.md` files are compiled by a specific binary and `check` verifies freshness against it.
- **Build:** GoReleaser on a per-platform matrix, triggered by `release: published` so semantic-release keeps ownership of tagging.

### Repo layout

```
cmd/para/main.go
internal/
  manifest/          # para.yml parse, schema, addressing
  compile/           # build: tree, AGENTS.md, skill
  check/             # rules engine; one file per rule family
  scaffold/          # init, new, import, move/archive/restore/remove
  template/
    para-template/   # go:embed source — the only copy
docs/
  design/para.md
  release-notes.md
```

### Implementation notes

- **Surgical writes are mandatory.** `yaml.v3` re-encoding is not format-preserving: it renormalises indentation and drops blank lines. The CLI must splice byte ranges, not decode-mutate-re-encode.
- **The `yaml.Node` duplicate-key gap.** Typed decode rejects duplicate keys; decoding into `yaml.Node` silently keeps both. Since writing requires the Node path, "duplicates impossible by construction" is false exactly where the CLI writes unless key uniqueness is asserted explicitly on that path.
- **Canonical field order**, with `items:` last: `kind, name, summary, status, blocked-by, priority, dates, cadence, objectives, use, hooks, items`. Adding a child becomes a pure append that never splits an existing block.
- **Parse with `yaml.Node`** and never round-trip through `map[string]`, so document order is preserved and hooks appear in compiled output in the order you wrote them.

## Migration from `brain`

The repo is `colchuck-ai/brain`; the local git remote still points at the redirected `jomadu/brain-skill`. It becomes `colchuck-ai/para`; GitHub redirects the old URL and preserves releases and tags.

Nothing has shipped: `v1.0.0-rc.1` is the only published pre-release, and rc.2 and rc.3 are stuck as **drafts** — live evidence that `.releaserc.json`'s `{"path": "brain"}` asset declaration does not do what it looks like, since `@semantic-release/github` globs its assets and a bare directory path is not a glob. There are no external users: 0 forks, 0 watchers, 0 issues, every PR self-authored.

So the only real migration target is `jomadu/brain` (private), the one repo built from the old template:

1. `brew install para`
2. Remove any old `brain` skill directory. Two skills with overlapping descriptions cause ambiguous activation.
3. `para init --config-only` in the existing repo — infers the manifest from the tree. Review it; it is a guess.
4. Decide what happens to `zettelkasten/` and `scripts/`. Neither is a PARA scope. Leave them untracked, move them under `resources/`, or split them out and use the `wiki` / `zettlekasten` skills.
5. Flatten any existing `archive/<name>/` to `archive/projects/<name>/`.
6. `para check`, then `para build`.
7. Work the `error` findings, then reason about `needs-review` with the skill loaded.

### Scope removals

**The zettelkasten is dropped entirely.** `colchuck-ai/wiki` and `colchuck-ai/zettlekasten` already exist, and `zk-skill/zk/assets/zk-template/zettelkasten.md` is byte-identical to the copy here — so this repo was holding a redundant second copy of a separable domain. It was also the part carrying the most semantic judgment and the part most replaceable by other tools.

Removing it makes para **composable**: the contract governs placement and movement and says nothing about what lives inside a directory, so a wiki, an Obsidian vault, or a zettelkasten can own file contents while para owns the structure around them.

**`scripts/` is dropped.** Never part of PARA, and its entire contract was "name script files in kebab-case."

---

## Open questions

**Does an agent reliably follow a digest link?** The one place correctness rests on model behaviour rather than the compiler, and the only remaining architectural unknown. Test it: put a hook at `root:`, work in a deep subarea, see whether it fires. If unreliable, the fallback is full materialisation — more repetition, guaranteed visibility.

**`scope.misfiled` noise.** The most valuable `needs-review` rule and the most likely to produce false positives, since detecting "this reads like a deadline" is a language heuristic. Ship it behind a flag so a user drowning in noise can disable it without losing the rest of `check`.

## Deferred

**Health checks (`para check --health`).** Depth is only a *proxy* for the failure modes that actually matter, and those are directly measurable:

| Rule | Fires on | Catches |
|---|---|---|
| `dir.sparse` | ≤2 files after 90 days | premature categorisation |
| `dir.scaffolding-only` | only subdirectories, no files | pure ceremony |
| `dir.stale` | untouched for 180 days | orphaned branches |
| `area.reads-as-project` | date or version in the name; hooks phrased as deliverables | areas drifting into projects |

*"This folder has one file and has not been touched in six months"* is not something you argue with, which makes these strictly better signals than a depth number. Deferred for two reasons: they are the only rules that would make `check` read mtimes and count files, breaking the manifest-only boundary; and every threshold above is invented. Shipping four made-up numbers that fire on a real archive is a reliable way to get `check` switched off. Calibrate against `jomadu/brain` after it has run under para for a while, then ship as an opt-in tier.

**`exclude:` for opting a child out of an inherited hook.** The obvious next request, and the obvious first place complexity creeps in. Wait for a real need.

## Rejected alternatives

### Schema shape

Three architectures were explored in parallel before the `items:` form was chosen.

**Flat path-keyed map** — every node one entry in a single mapping keyed by its filesystem path. Buys 1:1 source-to-output correspondence, orphan detection as a hash-map miss, and constant splice indentation. Rejected because renaming a parent rewrites every descendant key (and a human reaching for `sed s|areas/health|areas/wellbeing|` also hits `areas/health-insurance`), subtree moves are N edits, and the visual tree is gone — eighty left-aligned quoted strings with depth legible only by counting slashes.

**Uniform recursive node** — one node shape everywhere, children under a single `children:` key, kind as data. Rejected on its own strongest self-criticism: `sub-areas:` and `directories:` told a reader what kind of thing came next at zero cost, and `children:` tells them nothing. It also costs a constant +3 indent levels across the whole document.

**Trailing-slash mark** — mark the *variable* side, so `redesign-api/:` is a directory and `hooks:` is schema vocabulary, making collision lexically impossible. Genuinely elegant, and it marks the minority of lines rather than taxing the vocabulary. Rejected because it makes the file hostile to every other YAML tool (`yq '."./"."projects/"."hooks/".kind'`), and it converts a collision class into a typo class — a stray slash on `summary/:` is a legal directory named `summary`.

**Flat ID registry with `parent:` references** — entities as top-level keys with hierarchy by reference. Buys stable terse addresses that survive reparenting, one-line reparenting, and one-field reclassification. Rejected because `parent:` is an adjacency list, so cycles and dangling parents become expressible and need three new check rules that nesting makes impossible; entity IDs must be globally unique; and the area tree becomes invisible. The wins mostly accrue to hand-editors and the CLI absorbs them — you run `para move`, and whether that is a field edit or a block move is the tool's problem.

**Typed nesting keys (`sub-areas:` + `directories:`) without `kind:`** — the minimal fix, and it preserves the free type annotation. Rejected because `sub-areas: {notes: …}` and `directories: {notes: …}` under one area are both legal and compile to the same path. `kind:` plus a single `items:` makes that impossible rather than merely detectable.

**A separate top-level `scopes:` section for container guidance.** The first attempt at the collision, and it produced two independent trees describing one hierarchy.

**Sigil-prefixed structural keys (`$hooks`, `.hooks`).** They tax the majority of lines to fix the rare case, and carry no meaning beyond "para owns this word." Worth recording the parse traps found along the way: `@` is a parse error, `!hooks:` is silently consumed as a **tag** and the key vanishes, `#hooks:` becomes a comment and vanishes, `&`/`*` become anchor and alias.

**YAML anchors and aliases for hook reuse.** Free in the format, but the CLI has to *write* this file and round-tripping anchors correctly through `yaml.Node` is a tar pit.

### Earlier decisions

**Keep everything in prose; just tighten the skill.** Cannot fix problem 4. A prose rubric depends on an agent choosing to check carefully, and the evidence that it does not is in this repo's own template.

**Hand-authored `AGENTS.md` as the source of truth.** The predecessor's position, with one real virtue: guidance survives with no tool installed, in any harness. Rejected because it makes every other problem unsolvable — duplication has to be policed rather than prevented, generation and authoring fight over the same file, and inheritance cannot work at all under nearest-only resolution. Compiled output preserves the zero-tool fallback for *reading*; only *authoring* now requires para.

**Guidance in a hidden `.para/guidance/` mirror tree.** `AGENTS.md` is the ecosystem's established location, and a hidden tree is invisible to every harness.

**Procedures inline in TOML.** Multi-line markdown steps in TOML mean escaping pain, no markdown tooling, and awkward diffs. YAML block scalars handle prose properly.

**`scope:` expressions with globs or lists on hooks.** Position in the manifest determines scope instead — no expressions to parse, no precedence model, no way to point at a directory that does not exist.

**Overrides on `use:`, and library hooks composing other library hooks.** Merge semantics, recursion, cycle detection. If you need a variant, define a second hook.

**A `status: proposed` field on hooks with `approve`/`reject` commands.** Proposals belong in chat. Keeping them in the manifest puts workflow state in source and leaves half-committed hooks lying around.

**Tracking key-result completion in the manifest.** Drags manifest maintenance into every work session. The manifest declares intent; the agent evaluates reality.

**A git-clean precondition on `remove`.** Would make blunt deletion recoverable, at the cost of refusing to run in a dirty worktree. Destructive actions stay dumb and predictable, with `--dry-run` as the guard.

**Loose directory sniffing to identify a para repo.** `para.yml` is required, which makes detection exact. Sniffing for any of `projects/`, `areas/`, `resources/`, or `archive/` was rejected: plenty of code repositories have a `resources/` directory, and misdetection combined with `remove` is a bad pairing.

**Python single file with `uv run`.** Zero install, and the convention the Agent Skills docs recommend for bundled scripts. Rejected for Go: instant startup on commands that run constantly, and `go:embed` gives a single-source template a bundled script cannot.

**Node package via `npx`.** Reuses the existing npm and semantic-release toolchain, but pays cold-start latency on every invocation and splits versioning between the package and the skill.

**Flat `archive/<name>/` instead of mirroring the source path.** Matches PARA's canonical single archive, but discards what kind of thing was archived, making `restore` guesswork and leaving `check` no rule to enforce.

**User-defined status states, and `planned`/`blocked` as content rather than state.** The first is a workflow engine. The second was reconsidered and accepted — projects are often not tracked in any other system, so `to-do` and `blocked` earn their place, and `blocked` in particular enables the `project.long-blocked` finding.

---

## Appendix: the case bug

Worth recording precisely, because it is the empirical case for mechanising the contract and it dictates how `link.case-mismatch` must be implemented.

Four files in the old template referenced the zettelkasten method doc as `zettelkasten.md`. The working tree contains `zettelkasten.md`. **Git's index contains `Zettelkasten.md`.** `git status` reports clean.

The file was renamed locally; APFS is case-insensitive, so git saw no change and the uppercase name stayed committed. The links have therefore been broken on github.com and on any Linux checkout for months, while looking correct on the author's machine — and the skill's own prose audit procedure never caught it. The `zk-skill` copy is correctly lowercase, so this was a brain-only artifact.

The design consequence: **`link.case-mismatch` must compare against `git ls-files`.** A check that reads the filesystem finds the lowercase name and passes, reproducing exactly the blind spot that let this ship.

`node.key-collision` catches the same class of bug one layer earlier, before it ever reaches disk.
