package rebuild

import (
	"os"
	"path/filepath"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mirror"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/tree"
)

// This file is the Claude Code compatibility surface's half of rebuild (§6.1):
// the eight CLAUDE.md pointer files, and `.claude/skills/`.
//
// It is here rather than in a package of its own because of what the surface
// shares with everything else rebuild does — the answer to "what should this
// file contain", which doctor asks and rebuild acts on. Splitting it would give
// the surface a second opinion about its own bytes.
//
// The one thing that is genuinely different about it is that turning it *off*
// leaves a file behind. Every other projection has a knob that decides its
// contents; `emit.claude` decides whether para's block is present in a file it
// shares with everyone else, so this file carries the package's residue logic.

// residue is what a subject has on disk that truth no longer justifies: a
// block-scoped file para wrote part of and no longer would (§2.2, §9, §6.1).
//
// CLAUDE.md and .gitattributes are the same rule now, not two: both are files
// para owns a *block* inside, and turning their key off means taking the block
// out — the file stays, shorter, carrying every line the repository put
// there, unless the block was its only content, in which case removing it
// leaves nothing and the file goes (R6).
func (e *Env) residue(in render.In) ([]Artifact, error) {
	claude, err := e.claudeResidue(in)
	if err != nil {
		return nil, err
	}
	git, err := e.gitAttributesResidue(in)
	if err != nil {
		return nil, err
	}
	return append(claude, git...), nil
}

// claudeResidue is para's block sitting in a CLAUDE.md at one of the eight
// locations with `emit.claude` off — the file as it should be once the block
// comes out, or a removal when taking it out leaves nothing (R5-R7).
func (e *Env) claudeResidue(in render.In) ([]Artifact, error) {
	if !render.HasAgents(in.Locator) || render.HasClaude(in.Locator, in.Config) {
		// Not one of the eight locations, or the key is on — in which case
		// render.For already has this file and deriving it twice would be two
		// opinions about it.
		return nil, nil
	}
	path, err := claudePath(in.Locator)
	if err != nil {
		return nil, err
	}
	a, err := e.blockResidue(path, claudeBlockRemover(path))
	if err != nil || !a.Stale() {
		return nil, err
	}
	return []Artifact{a}, nil
}

// claudeBlockRemover closes WithoutClaudeBlock over the one path it needs and
// the two callers on the write side (residue and the constant-cost surface
// refresh) do not — CLAUDE.md's path, unlike .gitattributes's, is not a
// package constant.
func claudeBlockRemover(path string) func([]byte) ([]byte, bool, error) {
	return func(existing []byte) ([]byte, bool, error) {
		return render.WithoutClaudeBlock(path, existing)
	}
}

// gitAttributesResidue is para's block sitting in a .gitattributes with
// `emit.gitattributes` off — the file as it should be once the block comes out,
// or a removal when taking it out leaves nothing (R5-R7).
//
// The off case lives here rather than in render.For because For runs before
// anything has been read from disk, and whether there is a block to remove is
// a question about the file: with the key off and no file there, para has
// nothing to shorten and no reason to create one.
func (e *Env) gitAttributesResidue(in render.In) ([]Artifact, error) {
	if len(in.Locator) != 0 || in.Config.EmitGitattributes {
		// Not the root, or the key is on — in which case render.For already
		// has this file and deriving it twice would be two opinions about it.
		return nil, nil
	}
	path, err := render.GitAttributes.Path(render.In{})
	if err != nil {
		return nil, err
	}
	a, err := e.blockResidue(path, render.WithoutGitAttributesBlock)
	if err != nil || !a.Stale() {
		return nil, err
	}
	return []Artifact{a}, nil
}

// blockResidue is the one rule behind claudeResidue, gitAttributesResidue, and
// claudeArtifact's off branch (R5-R7): read what is at path, take para's block
// out through remove, and return the artifact apply already knows how to act
// on — a removal when taking the block out leaves nothing (R6), a shortened
// write when it leaves something (R5), or an artifact that compares equal to
// itself (Stale() false) when there is no file there or no block to take
// (R7). It never returns a nil artifact: claudeArtifact needs exactly one per
// location, stale or not, and residue's two callers filter on Stale()
// themselves rather than each repeating the no-op shape.
//
// remove is WithoutClaudeBlock or WithoutGitAttributesBlock, each already
// closed over the markers and the path its file's block lives at — the one
// thing this function does not need to know, because render owns what the
// markers are.
func (e *Env) blockResidue(path string, remove func([]byte) ([]byte, bool, error)) (Artifact, error) {
	existing, err := e.read(path)
	if err != nil {
		return Artifact{}, err
	}
	if existing == nil {
		return Artifact{Path: path}, nil
	}
	shortened, found, err := remove(existing)
	if err != nil {
		return Artifact{}, err
	}
	if !found {
		// Somebody else's file, or one para has already shortened: there is
		// nothing to remove, so the target is what is already there.
		return Artifact{Path: path, Existing: existing, Derived: existing, Present: true, Wanted: true}, nil
	}
	if len(shortened) == 0 {
		return Artifact{Path: path, Existing: existing, Present: true}, nil
	}
	return Artifact{Path: path, Derived: shortened, Existing: existing, Present: true, Wanted: true}, nil
}

// WriteGitAttributes brings the root's .gitattributes into line with
// `emit.gitattributes` and reports what it wrote and what it removed — a
// removal is reachable now that an emptied block-scoped file is deleted
// rather than left behind (R6), so unlike a plain write list this cannot drop
// the second return value the way a caller only expecting a write could.
//
// It is the write path's half of the same rule gitAttributesResidue is
// rebuild's half of, and it is one file at one known location — so a `config
// set` of the key can finish the job rather than leaving a tree that needs a
// rebuild, which is what every other config key that decides a file's contents
// already does (§6.1's precedent, §2.3's cost test).
//
// dryRun reports the same wrote/removed without touching the file, for
// `config set --dry-run` (para-ato).
func (e *Env) WriteGitAttributes(dryRun bool) (wrote, removed []string, err error) {
	cfg, err := e.Resolver.RenderConfig(nil)
	if err != nil {
		return nil, nil, err
	}
	path, err := render.GitAttributes.Path(render.In{})
	if err != nil {
		return nil, nil, err
	}
	data, err := e.read(path)
	if err != nil {
		return nil, nil, err
	}

	var artifacts []Artifact
	if cfg.EmitGitattributes {
		in := render.In{Config: cfg, Existing: map[string][]byte{path: data}}
		derived, err := render.GitAttributes.Render(in)
		if err != nil {
			return nil, nil, err
		}
		artifacts = []Artifact{{Path: path, Derived: derived, Existing: data, Present: data != nil, Wanted: true}}
	} else if artifacts, err = e.gitAttributesResidue(render.In{Config: cfg}); err != nil {
		return nil, nil, err
	}

	return e.apply(artifacts, dryRun)
}

// ClaudeSurface derives the CLAUDE.md at every one of the eight locations that
// exists in this tree, and nothing else.
//
// This is the constant-cost refresh §6.1 requires and §2.3 permits. §6.1 says a
// skill's import list "regenerates from the same scope walk that produces the
// rules … with no separate bookkeeping"; §2.3 forbids a mutation walking to
// root. The eight locations are a fixed set rather than a walk, so `add
// skills.x` can keep every CLAUDE.md correct while touching a bounded, known
// list — which is what makes this a legal thing for a mutation to call.
//
// adding and removing are passed straight through to rules: adding is only
// ever non-empty when AddDryRun is rehearsing the addition of a skill, since a
// rehearsal writes nothing and the skill it is adding is otherwise invisible
// to tree.SkillIDs; removing is only ever non-empty when a rehearsed `remove`
// is about to delete a skill that tree.SkillIDs can still see, for the
// opposite reason (para-ato).
//
// It loads no journals and no state: CLAUDE.md is a pointer file whose whole
// content is derived from which skills exist (§6.1), so a full Derive over
// eight subjects would read four files apiece to produce bytes that do not
// depend on any of them.
func (e *Env) ClaudeSurface(adding, removing []string) ([]Artifact, error) {
	rules, err := e.rules(adding, removing)
	if err != nil {
		return nil, err
	}

	var out []Artifact
	for _, loc := range render.AgentsLocations() {
		dir, err := e.locationDir(loc)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(dir); err != nil {
			// A bucket that is not there owns no files. A tree missing one is
			// broken in a way `doctor` reports and this is not the place to say
			// so twice.
			continue
		}
		a, err := e.claudeArtifact(loc, rules)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// claudeArtifact is one location's CLAUDE.md: the bytes it should hold, or —
// when the surface is off there — the file with para's block taken out of it
// (R5-R7), through the same blockResidue rule `rebuild.Run`'s residue uses.
// `add skill` and `config set emit.claude` reach this through
// WriteClaudeSurface, so a foreign CLAUDE.md must survive them exactly as it
// survives a full `rebuild` — the bug para-0o6 reports is this path taking a
// whole file that held content para never wrote.
//
// The config is still asked for per location even though the two `emit.claude`
// keys answer with the root's value wherever they are asked (see
// config.Resolver.RenderConfig). Reaching past the resolver to the root here
// would be this file deciding the rule a second time, and it is the resolver's;
// asking normally is what keeps the eight files, the mirror, and `render.For`
// unable to disagree.
func (e *Env) claudeArtifact(loc locator.Locator, rules []string) (Artifact, error) {
	cfg, err := e.Resolver.RenderConfig(loc)
	if err != nil {
		return Artifact{}, err
	}
	path, err := claudePath(loc)
	if err != nil {
		return Artifact{}, err
	}

	if !render.HasClaude(loc, cfg) {
		return e.blockResidue(path, claudeBlockRemover(path))
	}

	existing, err := e.read(path)
	if err != nil {
		return Artifact{}, err
	}
	a := Artifact{Path: path, Existing: existing, Present: existing != nil, Wanted: true}
	in := render.In{Locator: loc, Config: cfg, Rules: rules}
	if existing != nil {
		in.Existing = map[string][]byte{path: existing}
	}
	if a.Derived, err = render.Claude.Render(in); err != nil {
		return Artifact{}, err
	}
	return a, nil
}

// claudePath is where CLAUDE.md sits for a location, root-relative and
// slash-separated. It asks the renderer, so the file's location has one
// definition even where its contents are not being rendered.
func claudePath(loc locator.Locator) (string, error) {
	return render.Claude.Path(render.In{Locator: loc})
}

// locationDir is one of the eight locations as an absolute OS path.
func (e *Env) locationDir(loc locator.Locator) (string, error) {
	if len(loc) == 0 {
		return e.Root, nil
	}
	rel, err := loc.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(e.Root, filepath.FromSlash(rel)), nil
}

// WriteClaudeSurface derives the eight CLAUDE.md files and writes or removes
// the ones that are not what they should be, reporting both lists — or, under
// dryRun, reports them and writes nothing (§21.1).
//
// adding and removing are ClaudeSurface's parameters of the same names,
// threaded through so a rehearsed skill add or remove can name itself.
//
// It is the whole of what a skill mutation owes the surface's first half, and
// it is separate from Run because Run re-derives a tree and this re-derives
// eight files.
func (e *Env) WriteClaudeSurface(dryRun bool, adding, removing []string) (wrote, removed []string, err error) {
	artifacts, err := e.ClaudeSurface(adding, removing)
	if err != nil {
		return nil, nil, err
	}
	return e.apply(artifacts, dryRun)
}

// SyncMirror brings `.claude/skills/` into line with the skills that exist, and
// reports what it did — or, under dryRun, what it would do.
//
// The config is asked for at the root, which for the two `emit.claude` keys is
// where it is answered from anyway — but the mirror is one directory at the
// root, so asking anywhere else would be pretending there is a level that could
// have a different answer.
//
// pending is only ever non-empty under dryRun, and it is what makes a dry run's
// list equal to what the real run does. A real run has already rewritten the
// skills by the time it gets here, so the comparison against disk is the whole
// truth; a dry run has written nothing, so in copy mode a mirror of a skill
// whose SKILL.md is about to change still matches byte for byte and would be
// reported as fine. See mirror.Inspect.
//
// adding is a different rehearsal case: a skill AddDryRun is rehearsing the
// creation of, which is not on disk to compare against at all. Appending it to
// ids puts it in Inspect's `want` set, so a tree with emit.claude on reports it
// StateMissing — the same verdict, and the same planned Change, a real add's
// SyncMirror call would compute one line later, once the skill exists.
//
// removing is adding's mirror image: a skill a rehearsed `remove` is about to
// delete, still on disk and so still in tree.SkillIDs, dropped from the `want`
// set before Inspect sees it. Without it a skill's remove --dry-run would
// compare the mirror against a want set that still contains the skill being
// removed, and report no change at all (para-ato).
func (e *Env) SyncMirror(dryRun bool, pending, adding, removing []string) ([]mirror.Change, error) {
	cfg, err := e.Resolver.RenderConfig(nil)
	if err != nil {
		return nil, err
	}
	ids, err := tree.SkillIDs(e.Root)
	if err != nil {
		return nil, err
	}
	ids = pendingSkillIDs(ids, adding, removing)
	issues, err := mirror.Inspect(e.Root, mirror.Claude, cfg, ids, pending)
	if err != nil {
		return nil, err
	}
	if dryRun {
		return mirror.Planned(mirror.Claude, cfg, issues), nil
	}
	return mirror.Repair(e.Root, mirror.Claude, cfg, issues)
}
