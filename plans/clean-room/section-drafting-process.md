# Section drafting process

How each section of `spec.md` is written, and where the work stands. Read this first when resuming in a new session.

## Where we are (handoff)

- **Branch:** `clean-room-impl`, local only (not pushed). `main` holds the PoC; `poc-final` and `v0.7.0` tag its last commit.
- **Beads:** epic `para-a1b`; Phase 1 is `para-a1b.2` (claimed), and its notes log progress per section.
- **Done and committed:** §0.1 What para is, §0.2 The PARA method, §0.3 Structure is para's, content is yours, §0.4 Why deterministic structure helps agents, §0.5 Scope and non-goals, §0.6 Principles, §0.7 How to read this spec (chapter 0 complete).
- **Next:** §1.1 The tree. Then continue in outline order.
- **Decisions:** D1–D27 in `plans/clean-room/plan.md`. The ones written during drafting:
  - D22: `para` never deletes or edits content; `remove` deletes only `para`'s side and names what history it loses.
  - D23: generated files (`ACTIVITY.md`, `MEASUREMENTS.csv`) live in `.para/`, linked from the README; on `remove`, the parent's README block is regenerated and the removal is journaled at the parent.
  - D24: `para` owns only its own keys in README frontmatter.
  - D25: every refusal says why and what to do instead. D13 amended: a hand copy (`cp -r`) can also duplicate ids.
  - D26: `para` never runs git or requires a repository.
  - D27: only a person lowers a label or raises trust, through an interactive confirmation with no non-interactive path.

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
- `plans/clean-room/plan.md`: decisions D1–D27, open questions, and design inputs.
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

- **Every managed folder has a `.para/`**: the root (holding `tree.toml`), buckets, archive buckets, containers and entities (§0.3). Storage (§3) and the model (§2.1, §2.2) must keep this true.
- **Provenance records live in `.para/`** (§0.3). Chapter 6 may refine how, not where.
- **Collision rule** (§0.3): refuse rather than overwrite or merge, except that an existing README or `AGENTS.md` gets `para`'s part added and adoption adds a `.para/` to an existing folder. Adoption (§2.4) and migration (§9.1) must follow it; files a v0 `para` wrote count as `para`'s side.
- **Damaged block markers**: `para` must refuse to rewrite rather than guess, so it never overwrites the user's text. Belongs in §5.2.
- **Case-insensitive filesystems** (`README.md` vs `readme.md`): collisions resolve as the filesystem sees them. Belongs in §8.1.
- **Partial failure**: validation happens before any write; what happens if a write fails partway (including `remove` reruns, D22) belongs in §8.1.
- **Concurrent commands**: §0.4 says agents, people and scripts share a tree; what happens when two `para` commands run at once belongs in §8.1.
- **Mid-write conditions** (a destination appearing mid-run, a full disk) are partial failures under §8.1, not refusals; §0.4 promises refusal only before anything is written.
- **Ids in README frontmatter**: §0.4 says file tools can find any entity and its main facts, and commands take only ids (D11), so §5.3 must make the id a frontmatter key.
- **Journaling outside entities**: §0.4 says each entity's journal shows what happened to it; §1.8 or §3.4 must say whether changes at the root and buckets (such as `AGENTS.md` blocks, D21's `CLAUDE.md` cleanup) are journaled, for example in the root's `.para/`.
- **A command for every structural change** (§0.4): every change §0.3 lists (create, move, rename, re-parent, archive, unarchive, remove, adopt) has a command, ideally one per change; check chapter 7's charters against this list.
- **Refusals and remedies** (D25): every refusal gives the rule, the offending id or path, and what to do instead, which may be the user's own files to move; §7.10 defines the output.
- **Hand changes `doctor` must catch** (§8.2): `mv` of an entity (possibly to a place its kind may not sit), `rm -r` of an entity, a hand edit to a `para` frontmatter key, and `cp -r` of an entity, which duplicates its id (D13). Decide whether a plain folder directly in `projects/` or `areas/` is a finding, since §0.3 calls it content.
- **Git** (§3.8): §0.5 says `para` never runs git or requires a repository. §3.8 must settle whether `para` reads `.git/` and whether it writes `.gitattributes` (the PoC did; D23 allows only README and `AGENTS.md` as visible files).
- **Time-dependent values in generated files** (§5.3): derived status depends on the current time, and generated files change only when a command runs (§0.5, §0.6 principle 4), so §5.3 must show such values with an as-of date or leave them out.
- **Who may loosen a label or raise trust** (§6.4, D27): an interactive confirmation with no non-interactive path. This is the one exception to the non-interactive-confirmation note below.
- **Commands given a duplicated id** (§8.2): after a merge or `cp -r` (D13), what a command does with an id held by two entities.
- **Test hooks for the clock and ids** (§3.5, §2.3, Appendix B): §0.7 says Appendix B tells how tests fix the clock and new ids, so each must define a documented mechanism (such as an environment variable) and say whether users may rely on it.
- **Test output conditions** (§7.10, Appendix B): tests check output exactly (§0.7), so §7.10 must fix colour, width, locale, prompt text, JSON key order and `/` as the path separator, both with and without a terminal. A D27 confirmation needs a terminal, so the Phase 3 harness needs pseudo-terminal support; §6.4 should say whether an agent allocating its own pseudo-terminal is an accepted limit of D27.
- **Example starting points** (Appendix A, B): Appendix A or each example must define the v0 tree and the "example tree with a stated change" form that `migrate` and `doctor` examples start from.
- **Tour coverage** (chapter 1): §0.7 says chapter 1 tours the whole tool, but no tour section yet leads to the command line, `doctor`, `migrate` or config (chapters 7 to 9). Add one or link them from existing sections.
- **Non-interactive confirmation**: agents confirm results from JSON output and exit codes (§0.4), so every confirmation prompt, such as `remove`'s (D22), needs a non-interactive path with JSON output (§7.10).
- **`doctor`** reports a `.para/` that `para` did not create, and references to removed ids (§8.2).
- **Determinism** (§0.1): output and changes depend only on the tree, arguments and current time, apart from new ids. Config must therefore come from inside the tree (§3.6), or §0.1 needs revisiting.
- **README frontmatter**: `para` writes only its own keys (D24); §5.3 defines them.
