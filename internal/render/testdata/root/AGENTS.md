<!-- para:begin — generated, do not edit; run `para rebuild` -->
This tree is organised by the PARA method: four places, each answering a different question
about the thing you are filing.

- **projects** — work with a finish line. You will finish it, or you will explicitly stop.
- **areas** — a responsibility with no finish line. A standard you hold, ongoing.
- **resources** — reference material kept because it is useful later.
- **archive** — anything from the three above, once it is done. Archived things are neither
  deleted nor hidden; they live here so that "where did this go" is answerable by looking.

Each of those directories explains itself in its own AGENTS.md.

A directory with a `.para/` inside it is a thing this tree tracks. Most directories without one
are content — yours, and left alone. The exceptions are `.agents/`, described below; the
placeholders inside the archive, which the archive explains; and `.claude/` with its `CLAUDE.md`,
which para generates when the Claude Code surface is switched on and does not create otherwise.
In a tracked directory, `ACTIVITY.md` is a generated digest of that directory's own history and
`.para/` holds the machine-readable truth: what the thing is, and an append-only log of what
has happened to it. Alongside them is a `README.md` — or, for a skill, a `SKILL.md` — whose
frontmatter is generated above a body that is yours.

`.agents/` is not one of the four places and holds nothing you would file. Under
`.agents/skills/`, a skill is a directory you author — `SKILL.md`, plus any scripts and references
it needs — saying how to do something and when to do it. A skill names the parts of the tree it
applies to, or names none and applies to all of them, and that scope is rendered into a one-line
routing file under `.agents/rules/`, one per skill. Those files are generated: to change where a
skill applies, change the skill.

Two rules before you edit anything here:

- The frontmatter of a `README.md` or a `SKILL.md` is generated and the body below it is yours.
  Edit the body freely; edits to the frontmatter are overwritten.
- `ACTIVITY.md`, `MEASUREMENTS.csv`, everything under `.agents/rules/`, and — where they exist —
  `CLAUDE.md` and everything under `.claude/` are wholly generated. Do not hand-edit them.

Resolving a merge conflict in this tree: resolve the truth files under `.para/` and the journals
under `.para/logs/`, then run `para rebuild`. Never hand-resolve a generated file.
<!-- para:end -->
