package render

import (
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// agentsRenderer renders AGENTS.md's para-owned block, and never a byte
// outside it (§6). That is what lets a new para version ship a better
// explanation to an existing tree without eating the house rules written
// underneath.
//
// The prose describes the *place*. It prints no locator and, with one
// exception, names no command: §9 puts the merge-resolution procedure in the
// root's block specifically so an agent resolving a conflict knows not to
// hand-resolve a projection, and that procedure cannot be stated without
// naming `para rebuild`.
type agentsRenderer struct{}

const agentsFile = "AGENTS.md"

func (agentsRenderer) Path(in In) (string, error) {
	dir, err := in.dir()
	if err != nil {
		return "", err
	}
	return join(dir, agentsFile), nil
}

func (agentsRenderer) Render(in In) ([]byte, error) {
	block, ok := agentsBlocks[in.Locator.String()]
	if !ok {
		return nil, paraerr.Newf(paraerr.KindInternal,
			"render: %s is not emitted at %q", agentsFile, in.Locator.String())
	}
	path, err := Agents.Path(in)
	if err != nil {
		return nil, err
	}
	return mdfile.ReplaceBlock(in.existing(path), []byte(block))
}

// agentsBlocks is the generated prose, keyed by the locator of the place it
// describes — the eight of them agentsLocations lists. §27 defers the exact
// wording out of the spec because it wants drafting against a real tree, and
// these were drafted and then revised against one: a tree `para init` made,
// read as a reader of the committed tree sees it rather than as the author of
// the sentence does.
//
// Two things that revision changed, both of them gaps rather than wording:
//
//   - The root said nothing about skills, and §6's claim that "the root plus
//     the buckets cover every concept there is" is only true if it does —
//     nothing else in the tree emits an AGENTS.md, so a reader who found
//     `.agents/rules/` had nowhere to learn what it was.
//   - Two of the three archive mirrors omitted the stub sentence the third
//     carried. A stub happens under all three, and a reader standing in one
//     directory does not read another's block.
var agentsBlocks = map[string]string{
	"": `This tree is organised by the PARA method: four places, each answering a different question
about the thing you are filing.

- **projects** — work with a finish line. You will finish it, or you will explicitly stop.
- **areas** — a responsibility with no finish line. A standard you hold, ongoing.
- **resources** — reference material kept because it is useful later.
- **archive** — anything from the three above, once it is done. Archived things are neither
  deleted nor hidden; they live here so that "where did this go" is answerable by looking.

Each of those directories explains itself in its own AGENTS.md.

A directory with a ` + "`.para/`" + ` inside it is a thing this tree tracks. Most directories without one
are content — yours, and left alone. The exceptions are ` + "`.agents/`" + `, described below; the
placeholders inside the archive, which the archive explains; and ` + "`.claude/skills/`" + `, para's mirror
of ` + "`.agents/skills/`" + ` when the Claude Code surface is switched on. The rest of ` + "`.claude/`" + ` is not
para's — nor is ` + "`CLAUDE.md`" + `, described below.
In a tracked directory, ` + "`ACTIVITY.md`" + ` is a generated digest of that directory's own history and
` + "`.para/`" + ` holds the machine-readable truth: what the thing is, and an append-only log of what
has happened to it. Alongside them is a ` + "`README.md`" + ` — or, for a skill, a ` + "`SKILL.md`" + ` — whose
frontmatter is generated above a body that is yours.

` + "`.agents/`" + ` is not one of the four places and holds nothing you would file. Under
` + "`.agents/skills/`" + `, a skill is a directory you author — ` + "`SKILL.md`" + `, plus any scripts and references
it needs — saying how to do something and when to do it. A skill names the parts of the tree it
applies to, or names none and applies to all of them, and that scope is rendered into a one-line
routing file under ` + "`.agents/rules/`" + `, one per skill. Those files are generated: to change where a
skill applies, change the skill.

A skill's own directory is named ` + "`para-<id>`" + ` — ` + "`.agents/skills/para-<id>/`" + ` — so the
` + "`.claude/skills/`" + ` mirror can never collide with a skill some other tool installed there. The
skill's address is unprefixed (` + "`skill.<id>`" + `); ` + "`para add skill`" + ` prints the directory's real
path so the two are never guessed.

Three rules before you edit anything here:

- The frontmatter of a ` + "`README.md`" + ` or a ` + "`SKILL.md`" + ` is generated and the body below it is yours.
  Edit the body freely; edits to the frontmatter are overwritten.
- ` + "`ACTIVITY.md`" + `, ` + "`MEASUREMENTS.csv`" + `, and everything under ` + "`.agents/rules/`" + ` are wholly
  generated. Do not hand-edit them.
- Where it exists, ` + "`CLAUDE.md`" + ` holds a block para owns, delimited by a ` + "`para:begin`" + `/` + "`para:end`" + `
  marker pair, and nothing outside it. Edit anywhere outside the markers freely — that part is yours,
  same as any other tool that shares the file; edits inside them are overwritten on the next
  ` + "`para rebuild`" + `.

Resolving a merge conflict in this tree: resolve the truth files under ` + "`.para/`" + ` and the journals
under ` + "`.para/logs/`" + `, then run ` + "`para rebuild`" + `. Never hand-resolve a generated file or the block
inside a ` + "`CLAUDE.md`" + `.
`,

	"projects": `This directory holds PARA **projects**: work with a finish line. A project has a defined end —
you will finish it, or you will explicitly stop.

Each project is a directory with its own generated ` + "`README.md`" + ` and ` + "`ACTIVITY.md`" + `, plus whatever
content the work needs. A project may carry objectives under ` + "`objectives/`" + `, and an objective may
carry key results under ` + "`key-results/`" + `: measurable readings with a baseline, a target, and a
history, whose readings are also written out as a ` + "`MEASUREMENTS.csv`" + ` beside them.

A project has a status, and two of its values — done and dropped — mean it has finished. A finished
project is still here and still readable; it is simply no longer work in hand.

If a thing here has no finish line, it belongs in the areas directory instead. Once it is done, it
belongs in the archive.
`,

	"areas": `This directory holds PARA **areas**: responsibilities with no finish line. An area is a standard
you hold rather than a task you complete — your health, your finances, a service you own.

Areas nest as deeply as the responsibilities do. Each is a directory with its own generated
` + "`README.md`" + ` and ` + "`ACTIVITY.md`" + `, plus whatever content belongs to it.

An area has no status, because it never ends. If a thing here has a finish line, it belongs in the
projects directory. If you have stopped holding the standard, it belongs in the archive.
`,

	"resources": `This directory holds PARA **resources**: reference material kept because it is useful later —
notes, specifications, collected reading.

Resources nest freely, and each is a directory with its own generated ` + "`README.md`" + ` and
` + "`ACTIVITY.md`" + `. A resource is something you consult; it carries no status and no deadline.

If you are actively working on it, it belongs in the projects directory. If you are responsible for
it on an ongoing basis, it belongs in the areas directory.
`,

	"archive": `This directory holds everything that has left the live tree: finished projects, areas you no
longer hold, resources you no longer consult. It mirrors the three live buckets, so a thing keeps
its shape when it moves here.

Archived things are neither deleted nor hidden. They are addressable and readable exactly as they
were — they simply live somewhere else, so that "where did this go" is answerable by looking.

A directory here with no ` + "`.para/`" + ` inside it is a stub: a bare placeholder recording where an
archived descendant came from, kept because its live counterpart is still in use.
`,

	"archive.projects": `This directory holds **projects** that have left the live tree — finished, dropped, or otherwise
done. Each keeps the shape it had while it was live, including its objectives and key results.

Nothing here is deleted and nothing here is hidden. Bringing a project back means moving it, whole,
to the live projects directory.

A directory here with no ` + "`.para/`" + ` inside it is a stub: a bare placeholder recording where an
archived objective or key result came from, kept because the project above it is still live.
`,

	"archive.areas": `This directory holds **areas** you no longer hold — responsibilities that have ended, been handed
over, or stopped mattering. Each keeps the shape it had while it was live, sub-areas included.

A directory here with no ` + "`.para/`" + ` inside it is a stub: a placeholder recording the ancestry of an
archived descendant whose own parent is still live.
`,

	"archive.resources": `This directory holds **resources** you no longer consult. Reference material is cheap to keep and
expensive to re-find, so it is archived rather than deleted.

Nothing here is deleted and nothing here is hidden. Bringing a resource back means moving it,
whole, to the live resources directory.

A directory here with no ` + "`.para/`" + ` inside it is a stub: a bare placeholder recording the ancestry
of an archived resource whose own parent is still live.
`,
}
