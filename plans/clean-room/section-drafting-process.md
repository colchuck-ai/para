# Section drafting process

How each section of `spec.md` is written, and where the work stands. Read this first when resuming in a new session.

## Where we are (handoff)

- **Branch:** `clean-room-impl`, local only (not pushed). `main` holds the PoC; `poc-final` and `v0.7.0` tag its last commit.
- **Beads:** epic `para-a1b`; Phase 1 is `para-a1b.2` (claimed), and its notes log progress per section.
- **Done and committed:** §0.1 What para is, §0.2 The PARA method, §0.3 Structure is para's, content is yours, §0.4 Why deterministic structure helps agents, §0.5 Scope and non-goals, §0.6 Principles, §0.7 How to read this spec (chapter 0 complete), §1.1 The tree, §1.2 Buckets, §1.3 Projects, §1.3.1 Objectives, §1.3.2 Key results, §1.4 Areas, §1.4.1 Standards, §1.4.2 Indicators, §1.5 Measurement, §1.5.1 Analytics, §1.5.2 Metrics, §1.5.3 Terms, §1.6 Resources, §1.7 Archive, §1.8 Notes and the journal.
- **Next:** §1.9 Activity. Then continue in outline order.
- **Decisions:** D1–D55 in `plans/clean-room/plan.md`. The ones written during drafting:
  - D22: `para` never deletes or edits content; `remove` deletes only `para`'s side and names what history it loses.
  - D23: generated files (`ACTIVITY.md`, `MEASUREMENTS.csv`) live in `.para/`, linked from the README; on `remove`, the parent's README block is regenerated and the removal is journaled at the parent.
  - D24: `para` owns only its own keys in README frontmatter.
  - D25: every refusal says why and what to do instead. D13 amended: a hand copy (`cp -r`) can also duplicate ids.
  - D26: `para` never runs git or requires a repository.
  - D27: only a person lowers a label or raises trust, through an interactive confirmation with no non-interactive path.
  - D28: every `.para/` holds a `state.toml` with its kind; the root's has `kind = "root"`, schema version and id prefix, replacing `tree.toml`. The `tree.toml` section was removed, and §3.3–§3.8 became §3.2–§3.7.
  - D29: state holds facts (changing one would make a record false), config holds choices; the id prefix is state, fixed when the tree is created or migrated, and ids are never re-indexed.
  - D30: repair resolves a duplicate id by giving one entity a new id, the person choosing which keeps it; the only time an id changes.
  - D31: a bucket holds only entities of its kind and `para`'s own files (`.para/`, README, `AGENTS.md`); anything else directly in it is a `doctor` finding.
  - D32: archiving from inside a live entity leaves a managed stub (`kind = "stub"`, the id it stands for, no id of its own) at each mirrored ancestor; §0.3 and §1.1 lists amended to include stubs.
  - D33: projects never nest; a project sits only directly in `projects/` or `archive/projects/`.
  - D34: project and objective statuses are `planned` (default), `in-progress`, `blocked`, `done`, `dropped`, set by command in any order.
  - D35: key result statuses `achieved`, `missed`, `at-risk`, `on-track` are computed; only `dropped` is set by hand.
  - D35 (amended): after the due date only measurements dated on or before it count; `dropped` overrides and can be cleared.
  - D36: every key result has its own due date.
  - D37: an objective holds only `key-results/` and your content; a key result holds no entities.
  - D38: areas nest at any depth.
  - D39: `standards/` and `indicators/` containers; a standard holds only indicators and content, an indicator no entities.
  - D40: indicator statuses `no-data`, `breached`, `at-risk`, `healthy` are computed; the at-risk margin is a config setting.
  - D41: a measurement is a dated number; metrics have no types.
  - D42: the measurement library is flat; analytics, metrics and terms hold no entities.
  - D43: a term is a README definition that metrics reference by id; every library reference is checked by kind, listed both ways by `show`, and reported by `doctor` once dangling.
  - D44: the measurement library and each of its containers are created on first use, not by `init`; `measurement` stays reserved.
  - D45: staleness is a per-kind `<kind>.stale-after` (dir, analytic, metric and term included), off unless set, inherited from anywhere, root included.
  - D46: `init` writes starter `project.stale-after` and `area.stale-after` into the root's config; no hidden defaults.
  - D47: container is an entity kind with an id and a reserved name; it holds only its expected kind (anything else is a `doctor` warning), never goes stale and is never archived alone; it has a journal and a README listing its children; stubs keep no id.
  - D48: anything can be archived whatever its status; fields never change.
  - D49: unarchive brings back archived ancestors as live and restores everything beneath, even what was archived alone first.
  - D50: the archive is read-only (show, list, unarchive, remove).
  - D51: any entity but a container can be archived alone; stubs give way to their entities at every depth, refusing only when two archived entities share a path.
  - D52: containers are created on first use.
  - D53: every managed folder but a stub keeps a journal; one home per event.
  - D54: a note counts as attention only when marked (opt in), for notes and reasons alike; `migrate` marks v0 notes that counted.
  - D55: attention is the entity's own newest marked note, or its creation; a metric's measurements count; nothing on a child counts.

## Priorities

**Reader clarity, and less is more.** Every section is written for a reader who has only the spec: a person meeting `para` for the first time, or an AI agent dropped into a tree. Keep it simple. The shortest section that is complete wins.

## Writing conventions

- **Cross-references are Markdown links** to the heading anchor, such as `[the PARA method](#02-the-para-method)`. Never use the § sign. Anchors follow GitHub: lowercase, punctuation dropped, spaces to hyphens (`### 8.2 \`doctor\` findings` → `#82-doctor-findings`).
- **One line per paragraph.** Never hard-wrap prose.
- **The tool is always `` `para` `` in backticks** in prose. Headings stay plain. "PARA" in capitals is the method.
- **Each drafted section replaces its one-sentence charter.** Keep the charter in the reviewer prompt.
- **Check links after every edit** with this snippet:

```python
import re
s=open('spec.md').read()
slug=lambda h: re.sub(r'[^\w\- ]','',h.lower()).replace(' ','-')
anchors={slug(m.group(1)) for m in re.finditer(r'(?m)^#{1,4} (.+)$',s)}
print('broken:',[l for l in re.findall(r'\]\(#([^)]+)\)',s) if l not in anchors])
```

## Context to load when resuming

- `spec.md`: the outline and drafted sections. Each undrafted section's one sentence is its charter.
- `plans/clean-room/plan.md`: decisions D1–D55, open questions, and design inputs.
- `plans/clean-room/research/labeling.md`: research behind chapter 6.
- `para-design-v4.md` (on `main` until Phase 2, always at `poc-final`): PoC design, background only. Decisions win where they disagree.
- `bd show para-a1b.2`: progress notes.

## The loop

1. **Go.** The human says go, naming a section or taking the next one in outline order.
2. **Draft.** Write the section in `spec.md` under its header, from its charter and the decisions. Check facts against sources before writing (as §0.2 did with Forte's article).
3. **Humanize.** Apply the humanizer skill's rules (load it with the Skill tool in a fresh session): no dashes, no "not X but Y", no one-line closers, no padding.
4. **Review, round 1.** Three fresh `general-purpose` subagents in parallel, in the background, one lens each (template below). They report only.
5. **Fix.** Wait for all three, then show the human a table of findings and actions. Dismiss findings that do not hold up, saying why. Fix every reject, blocker and major; fix minors that are clean and small.
6. **Review, round 2.** Send the revised draft to the same three agents with `SendMessage` (they keep their context), listing what changed.
   - Only minors left: fix the clean ones, then go to human review.
   - Any reject, blocker or major left: apply a clean fix if there is one, and escalate to the human, saying so.
7. **Human review.** Show the full section text and what the reviews changed. Iterate until approved.
8. **Commit.** One commit per approved section (`docs(spec): write X.Y Title`), then `bd update para-a1b.2 --append-notes "..."` and `bd dolt push`.

At most two review rounds per section, unless the human asks for another.

**When a review raises a design question** (as `remove` did in §0.3), stop and ask the human. If the choice is hard, run three subagents through user lenses (a person using the CLI, an AI agent in the tree, scripts and maintainers) to compare the options, then recommend. Record the answer as the next D-number in the plan.

**If a reviewer's report does not arrive** after its completion notice, ask it to resend with `SendMessage`.

## Severities

| Severity | Meaning | Action |
| --- | --- | --- |
| Reject | The approach is wrong; the section needs rethinking, not editing | Must fix |
| Blocker | Contradicts a decision, states something false, or omits what the charter requires | Must fix |
| Major | A gap or ambiguity that would mislead a reader or an implementer | Must fix |
| Minor | Wording, ordering or small clarity issues | Fix if clean and small |

## Review lenses

The reader-clarity lens carries the most weight.

1. **Reader clarity and less is more** (primary). Could a newcomer, human or agent, understand it with only the spec? Flags undefined terms, hidden assumptions, two readings, padding, repetition of earlier sections, anything that could be shorter.
2. **Consistency and scope.** Agrees with every decision and every other section's charter; stays inside its own charter and covers all of it; uses terms as defined (D17); carries nothing the decisions reversed (paths as addresses, skills, `CLAUDE.md` generation, `resource` as a kind); link anchors resolve; for sections about outside sources, fidelity to the source.
3. **Implementation and edge cases.** Read as the engineer who must build and test it. Flags promises that cannot be kept or tested, missing edge cases (empty tree, merge collision, hand edit, symlink, case-insensitive filesystem, partial failure), and security rules. Does not demand detail that belongs to later chapters.

### Reviewer prompt template

> You are reviewing one drafted section of a software specification. Report findings only; do not edit any file.
>
> **Project.** `para` is a Go CLI that deterministically manages the structure and metadata of a PARA file tree. The spec is written section by section in `/Users/maxdunn/dev/colchuck-ai/para/spec.md`; undrafted sections are one-sentence charters. Conventions: cross-references are Markdown links to heading anchors (no § sign); one line per paragraph; the tool is `para` in backticks.
>
> **Section under review:** §X.Y "Title" (from `### X.Y` to the next header). Charter: "…". Earlier sections are approved; don't re-review them, but flag repetition or mismatch.
>
> **Context:** decisions in `plans/clean-room/plan.md`; severities in `plans/clean-room/section-drafting-process.md`; (lens-specific background files).
>
> **Your lens: …** (lens description, plus the section's riskiest claims or edge cases to probe).
>
> **Severities:** Reject, Blocker, Major, Minor. **Output:** numbered list; each: location (quote), severity, problem (1–2 sentences), concrete fix. Say so if nothing is above Minor. Don't invent problems. Under 350 words.

## Notes carried forward to later sections

Commitments made while drafting chapter 0 that later sections must honour:

- **Every managed folder has a `.para/`**: the root (its `state.toml` has `kind = "root"`, D28), buckets, archive buckets, containers, stubs (D32) and entities (§0.3). Storage (§3) and the model (§2.1, §2.2) must keep this true.
- **Provenance records live in `.para/`** (§0.3). Chapter 6 may refine how, not where.
- **Collision rule** (§0.3): refuse rather than overwrite or merge, except that an existing README or `AGENTS.md` gets `para`'s part added and adoption adds a `.para/` to an existing folder. Adoption (§2.4) and migration (§9.1) must follow it; files a v0 `para` wrote count as `para`'s side.
- **Damaged block markers**: `para` must refuse to rewrite rather than guess, so it never overwrites the user's text. Belongs in §5.2.
- **Case-insensitive filesystems** (`README.md` vs `readme.md`): collisions resolve as the filesystem sees them. Belongs in §8.1.
- **Partial failure**: validation happens before any write; what happens if a write fails partway (including `remove` reruns, D22) belongs in §8.1.
- **Concurrent commands**: §0.4 says agents, people and scripts share a tree; what happens when two `para` commands run at once belongs in §8.1. Root and bucket journals (D53) are written by nearly every create, so concurrent appends and git merge conflicts concentrate there (§3.7).
- **Mid-write conditions** (a destination appearing mid-run, a full disk) are partial failures under §8.1, not refusals; §0.4 promises refusal only before anything is written.
- **Ids in README frontmatter**: §0.4 says file tools can find any entity and its main facts, and commands take only ids (D11), so §5.3 must make the id a frontmatter key.
- **Journaling outside entities** (§3.3, D53): settled; the root and buckets keep journals. §3.3 must list which root events exist (`init`, `migrate`, `AGENTS.md` blocks, D21's `CLAUDE.md` cleanup) and whether `config` changes are journaled anywhere.
- **A command for every structural change** (§0.4): every change §0.3 lists (create, move, rename, re-parent, archive, unarchive, remove, adopt) has a command, ideally one per change; check chapter 7's charters against this list.
- **Refusals and remedies** (D25): every refusal gives the rule, the offending id or path, and what to do instead, which may be the user's own files to move; §7.10 defines the output.
- **Hand changes `doctor` must catch** (§8.2): `mv` of an entity (possibly to a place its kind may not sit), `rm -r` of an entity, a hand edit to a `para` frontmatter key, and `cp -r` of an entity, which duplicates its id (D13). Anything directly in a bucket other than its entities and `para`'s files is a finding (D31), though §0.3 still calls it content that `para` never moves; decide whether OS and tool files (`.DS_Store`, `.gitkeep`, `Thumbs.db`) are exempt.
- **Git** (§3.7): §0.5 says `para` never runs git or requires a repository. §3.7 must settle whether `para` reads `.git/` and whether it writes `.gitattributes` (the PoC did; D23 allows only README and `AGENTS.md` as visible files).
- **Time-dependent values in generated files** (§5.3): derived status depends on the current time, and generated files change only when a command runs (§0.5, §0.6 principle 4), so §5.3 must show such values with an as-of date or leave them out.
- **Who may loosen a label or raise trust** (§6.4, D27): an interactive confirmation with no non-interactive path. This is the one exception to the non-interactive-confirmation note below.
- **Commands given a duplicated id** (§8.2): after a merge or `cp -r` (D13), what a command does with an id held by two entities.
- **Test hooks for the clock and ids** (§3.4, §2.3, Appendix B): §0.7 says Appendix B tells how tests fix the clock and new ids, so each must define a documented mechanism (such as an environment variable) and say whether users may rely on it.
- **Test output conditions** (§7.10, Appendix B): tests check output exactly (§0.7), so §7.10 must fix colour, width, locale, prompt text, JSON key order and `/` as the path separator, both with and without a terminal. A D27 confirmation needs a terminal, so the Phase 3 harness needs pseudo-terminal support; §6.4 should say whether an agent allocating its own pseudo-terminal is an accepted limit of D27.
- **Example starting points** (Appendix A, B): Appendix A or each example must define the v0 tree and the "example tree with a stated change" form that `migrate` and `doctor` examples start from.
- **Tour coverage** (chapter 1): §0.7 says chapter 1 tours the whole tool, but no tour section yet leads to the command line, `doctor`, `migrate` or config (chapters 7 to 9). Add one or link them from existing sections.
- **Kinds of non-entities** (§2.1, §2.2, D28): the root, buckets, archive buckets and stubs each have a kind in `state.toml` (`root`, `stub` and others to be named), so §2.1 or §2.2 must list them beside the entity kinds.
- **Entities moved between trees** (§2.3, §2.4, D29): an entity brought in from another tree carries that tree's prefix. Decide whether it keeps its id or gets a new one, and how a wrong-tree id is refused (D25). A new id here would need §1.1 and D30 amended, since repair is otherwise the only time an id changes.
- **Top of the label chain** (§6.3, D15, D28): any `.para/` may hold a `config.toml`. Decide whether the root's or a bucket's config can set sensitivity, or whether inheritance starts at entities.
- **Prefix validation** (§3.2, D13): a hand-edited prefix ends up in every new id, so `state.toml` reading must validate it.
- **Repairing a duplicate id** (§8.3, D30, D11): repair must name one holder of a shared id (a path, as a stated exception to D11, or similar), give the choice a non-interactive form for agents, handle three or more holders, and re-id a copied subtree together so the keeper is chosen per subtree.
- **Missing or altered buckets** (§8.2, §8.3, §3.6): a bucket or archive bucket removed or renamed by hand, `measurement/` or one of its containers removed or renamed while it still holds entities (absent is fine, D44), what happens when its last entity is removed, a loose file or plain folder directly in `analytics/`, `metrics/` or `terms/`, a bucket that is a symlink, `Projects/` on a case-insensitive filesystem, and a bucket's `README.md` or `AGENTS.md` deleted (does `para` restore it?).
- **Archived measurement** (§1.7, §2.5, D20, D32): analytics, metrics and terms can be archived (D51), to `archive/resources/measurement/…`; see Archived references.
- **Stubs** (§1.7, §2.5, §2.3, §8.2, D32): a stub for a stub (a dir inside a dir inside an area); a stub left when its entity is removed, and a stub whose id no longer exists; the id a stub records is a reference, so duplicate checks and lookup ignore it; stubs are matched by id and follow their entity's renames; `remove` of an archived entity whose parent is a stub (no README to regenerate; does a stub have a journal, D23?); unarchiving after the live parent has itself been archived. Settled by D51: stubs give way to their entities at every depth, refusing only when two archived entities share a path, and any entity but a container can be archived alone, since containers have ids (D47).
- **When containers exist** (§2.1, §2.2, §2.5, Appendix B, D52): created on first use; say whether a container is removed when its last child goes (and a recreated one gets a new id, which breaks any stub that recorded the old one).
- **What a due date does** (§1.14, §4.5, §4.6): §1.3 says a project may have a due date but not what happens when it passes; decide whether `review` flags an overdue open project.
- **Archiving open work** (§1.15, §2.5): any status can be archived (D48); say what `review` says about a done project left unarchived.
- **How many objectives and key results** (§2.2, §7.6): whether a project needs any objectives and an objective any key results (D19 makes areas' standards and indicators optional).
- **Simple key results** (§4.1, §4.3): a key result must hold an analytic (D20), so counting one metric needs a metric and an analytic; an analytic can be one metric's latest value (§1.5.1); decide whether `para` offers a shortcut that creates a metric and that analytic in one command.
- **Dropped and archived key results and indicators** (§1.14, §2.5): whether a dropped key result still counts toward its objective, and what a key result's or indicator's status is once it is archived, and whether `review` still reports it.
- **Key result computation** (§4.3, §4.4): define the start value and start date, the at-risk threshold, a target below the start (such as latency, where reaching means crossing downward), a yes/no target with no pace, the status with no measurements yet, a due date already past at creation, and a key result due after its objective. Also when a due date counts as passed (through the end of that day, in which time zone, §3.4), and whether a measurement may be backdated to on or before a passed due date, which would change a settled result.
- **Reserved container names** (§2.2, §2.4, §8.2, D20, D39): every container name is reserved in its parent, like `measurement`. Define the rule once: `para` refuses to create or adopt anything under a reserved name, adopting refuses when a plain folder already has one (D25), and `doctor` reports content found there. The tour (§1.3, §1.4) leaves this to §2.2. Say the depth: `measurement` is reserved only directly in `resources/` (D44), so `resources/cooking/measurement/` is allowed.
- **Staleness of nested areas** (§4.5, D55): settled; nothing on a descendant keeps an entity fresh. §4.5 should say how review keeps a parent with no work of its own (such as `health/`) from being flagged forever, for example by not setting `area.stale-after` on it.
- **Moving into a descendant** (§2.2, §7.6): moving an area (or dir) under its own descendant must be refused (D25).
- **Dangling measurement references** (§4.4, §7.6, §8.2, D22, D43): the status of a key result or indicator whose analytic or metric was removed; whether `remove` names what depends on the entity in its confirmation; `doctor` findings for a reference to the wrong kind, to an id never issued, or to another tree's prefix after a copy. References point downward by kind, so §4.1 should say no cycle is possible.
- **Measurement values** (§4.1, §4.2, D41): the number domain (NaN, infinities, huge values), date precision and time zone, two measurements at the same time (which decides "latest"), correcting or deleting a wrong measurement, division by zero, and an analytic whose metric has no measurements.
- **Labels on computed values** (§6.4, §6.5): a metric under a confidential path can feed a status and value shown in a public project's README; say whether a computed value is content derived from its metrics.
- **`MEASUREMENTS.csv` edits** (§5.5): it is a projection, so say whether hand edits are overwritten or reported.
- **Staleness per kind** (§3.5, §4.5, D45): list every `<kind>.stale-after` key; say what the clock is for kinds with no measurements (analytic, term, dir), and whether a `dir.stale-after` on a project or area is the expected way to cover its dirs. Also: the starter values `init` writes (D46); a value meaning "never" so a nearer setting can switch off a farther one (a root `dir.stale-after` reaching `resources/`); whether settings pass into `archive/`, or review simply ignores archived entities, and what happens to a `config.toml` that moves with an archived entity; links use their own check and update dates (§1.10) and have no `stale-after`; and whether activity in a nested dir keeps its parent fresh, as for nested areas.
- **v0 resources** (§9.1, D18): how `migrate` converts `kind = "resource"` folders outside `resources/`, nested ones, and ones inside projects.
- **Containers as entities** (§2.1, §2.2, §5, §8.2, Appendix A, D47): say whether a container has `ACTIVITY.md` and frontmatter, and the exact README block (D47 says it lists children); whether `remove` may target a container; `doctor`'s warning for other entities or content in a container; ids now grow faster (D13), since each project and area adds container ids; update the example tree.
- **Read-only archive exceptions** (§5, §8.3, §9.1, D50): `doctor` repair (D30 re-iding), regenerated files, `migrate` of v0 archived entities, and stubs that follow renames all change things under `archive/`; say that read-only covers user commands only. Also unarchiving when a live entity now has the name, or when the stub's live entity was removed, including a container removed when empty and recreated with a new id, so its stub matches nothing (see When containers exist) (§2.5). Also the one refusal left in archiving: two archived entities at one path (a key result `a` archived, then a new live `a` archived); the remedy is renaming the live one, since the archive is read-only.
- **Symlinks in archived entities** (§2.5, §8.1): what archiving does with a symlink inside an entity, and moving many folders at once (partial failure).
- **Archived references** (§1.5, §4.4, D50, D51): a live key result or analytic that uses an archived analytic or metric, which can take no new measurements; whether its status still computes from the archived data, and whether a new reference may name an archived entity.
- **Journal rules** (§3.3, §3.4, §8.2): how rotation, `migrate` and D30 re-iding treat existing events, and whether `doctor` reports hand edits to a journal; for move, archive and unarchive, which journals record what (old parent, new parent, the entity itself, which is read-only once archived, D50), and where the event goes when the parent is a stub (no journal, D53); whether measurements are journal events (the PoC's design; §1.5.2 and D22 leave it open) and whether `config` changes are journaled; whether a command that changes nothing but carries a note records the note; notes are added to entities (§1.8), so say whether a container takes one; §3.3 must keep "`para` never edits an event" true under `migrate` and D30 re-iding, or state the exceptions; a wrong note is corrected by appending, as for measurements; ordering by timestamp under clock skew and equal times.
- **Attention marks** (§3.3, §4.5, §5.6, §9.1, D54): the flag's name; whether measurements still count as attention for a metric; that `AGENTS.md` tells agents to mark a note as attention only when the person says the upkeep happened; `migrate` turning v0 notes without `--no-attention` into marked ones; whether `review` shows the latest unmarked note, so a false stale flag is quick to clear. D55 names what resets the clock (a marked note, a metric's measurement, else creation); §4.5 must say whether unarchive restarts it. A mark cannot be added later (the journal is append-only), so the remedy is a new marked note; §3.3/§3.4 must say whether a note's date can be set, since a backdated mark claims past upkeep.
- **Indicator computation** (§4.3, §4.4, §3.5): how the window is set and measured (days, calendar periods, the time zone), how the margin is expressed (a fraction of the target, an absolute amount) and its default, targets that go down (latency), whether `no-data` also counts toward staleness (§4.5), whether the analytic combines every measurement in the window or uses the latest, a partial window (an indicator created today), measurements dated in the future, the boundaries of `at-risk` (exactly at the target, the edge of the margin), a target as a value plus a comparison (at or above, at or below), and that the window and target are state (defining facts set by command) while the margin is config.
- **Finding the tree** (§3.6): §1.1 says every command but `init` looks upward from the current directory for the nearest root (D28). §3.6 must settle a start outside any tree, inside a `.para/`, in a tree nested in another tree's content, under a symlinked root, and whether the PoC's `$PARA_HOME` override survives. Also: an ancestor `state.toml` that cannot be read (skip or refuse), an entity's `state.toml` hand-edited to `kind = "root"` (silently a nested tree; `doctor`?), the downward walk stopping at a nested root, and a v0 `tree.toml` recognised so every command but `migrate` refuses with the `para migrate` remedy (D14).
- **Targeting folders without ids** (§2.3): the root, buckets and containers have no id (§1.1), so §2.3 must say how a command names one as a parent, such as creating a dir directly in `resources/` (D18).
- **Issued-id record** (§2.3 or §3.2): §1.1 says `para` never reuses an id (and D30 repair issues new ids into the same record), so a per-tree record of issued ids must outlive `remove` (D22) and merge cleanly (plan §1.4 item 4).
- **`init` where something exists** (§7.3): `init` inside an existing tree or in a folder that already holds content follows the collision rule (§0.3).
- **Non-interactive confirmation**: agents confirm results from JSON output and exit codes (§0.4), so every confirmation prompt, such as `remove`'s (D22), needs a non-interactive path with JSON output (§7.10).
- **`doctor`** reports a `.para/` that `para` did not create, and references to removed ids (§8.2).
- **Determinism** (§0.1): output and changes depend only on the tree, arguments and current time, apart from new ids. Config must therefore come from inside the tree (§3.5), or §0.1 needs revisiting.
- **README frontmatter**: `para` writes only its own keys (D24); §5.3 defines them.
