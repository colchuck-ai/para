<!-- para:begin — generated, do not edit; run `para rebuild` -->
This tree is organised by the PARA method: four places, each answering a different question
about the thing you are filing.

- **projects** — work with a finish line. You will finish it, or you will explicitly stop.
- **areas** — a responsibility with no finish line. A standard you hold, ongoing.
- **resources** — reference material kept because it is useful later.
- **archive** — anything from the three above, once it is done. Archived things are neither
  deleted nor hidden; they live here so that "where did this go" is answerable by looking.

Each of those directories explains itself in its own AGENTS.md.

Inside any tracked directory: `README.md` carries generated frontmatter above a body that is
yours; `ACTIVITY.md` is a generated digest of that directory's own history; `.para/` holds the
machine-readable truth. Anything else in the directory is yours, and is left alone.

Two rules before you edit anything here:

- The frontmatter of a `README.md` is generated and the body below it is yours. Edit the body
  freely; edits to the frontmatter are overwritten.
- `ACTIVITY.md`, `MEASUREMENTS.csv`, and everything under `.agents/rules/` are wholly generated. Do
  not hand-edit them.

Resolving a merge conflict in this tree: resolve the truth files under `.para/` and the journals
under `.para/logs/`, then run `para rebuild`. Never hand-resolve a generated file.
<!-- para:end -->
