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
// leaves files behind. Every other projection has a knob that decides its
// contents; `emit.claude` decides whether eight files and a directory exist at
// all, so this file carries the only two removals in the package.

// residue is what a subject has on disk that truth no longer justifies: a file
// nothing generates any more, or a file para wrote part of and no longer would.
//
// There are two, and the difference between them is the whole of why this is
// not one rule. A CLAUDE.md with `emit.claude` off is a file para owns end to
// end, so it goes (§6.1). A .gitattributes with `emit.gitattributes` off is a
// file para owns a *block* inside (§2.2, §9), so the block goes and the file
// stays — shorter, and carrying every line the repository put there.
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

// claudeResidue is a CLAUDE.md left at one of the eight locations by turning
// `emit.claude` off: a whole file nothing generates any more (§6.1).
func (e *Env) claudeResidue(in render.In) ([]Artifact, error) {
	if !render.HasAgents(in.Locator) || render.HasClaude(in.Locator, in.Config) {
		return nil, nil
	}
	path, err := claudePath(in.Locator)
	if err != nil {
		return nil, err
	}
	data, err := e.read(path)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	return []Artifact{{Path: path, Existing: data, Present: true}}, nil
}

// gitAttributesResidue is para's block sitting in a .gitattributes with
// `emit.gitattributes` off — the file as it should be once the block comes out.
//
// It is an artifact whose Wanted is true and whose bytes are *shorter*, not a
// removal, and only render decides what those bytes are. The off case lives
// here rather than in render.For because For runs before anything has been read
// from disk, and whether there is a block to remove is a question about the
// file: with the key off and no file there, para has nothing to shorten and no
// reason to create one.
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
	data, err := e.read(path)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	shortened, found, err := render.WithoutGitAttributesBlock(data)
	if err != nil {
		return nil, err
	}
	if !found {
		// Somebody else's .gitattributes, or one para has already shortened.
		return nil, nil
	}
	return []Artifact{{Path: path, Derived: shortened, Existing: data, Present: true, Wanted: true}}, nil
}

// WriteGitAttributes brings the root's .gitattributes into line with
// `emit.gitattributes` and reports whether it wrote it.
//
// It is the write path's half of the same rule gitAttributesResidue is
// rebuild's half of, and it is one file at one known location — so a `config
// set` of the key can finish the job rather than leaving a tree that needs a
// rebuild, which is what every other config key that decides a file's contents
// already does (§6.1's precedent, §2.3's cost test).
func (e *Env) WriteGitAttributes() (wrote []string, err error) {
	cfg, err := e.Resolver.RenderConfig(nil)
	if err != nil {
		return nil, err
	}
	path, err := render.GitAttributes.Path(render.In{})
	if err != nil {
		return nil, err
	}
	data, err := e.read(path)
	if err != nil {
		return nil, err
	}

	var artifacts []Artifact
	if cfg.EmitGitattributes {
		in := render.In{Config: cfg, Existing: map[string][]byte{path: data}}
		derived, err := render.GitAttributes.Render(in)
		if err != nil {
			return nil, err
		}
		artifacts = []Artifact{{Path: path, Derived: derived, Existing: data, Present: data != nil, Wanted: true}}
	} else if artifacts, err = e.gitAttributesResidue(render.In{Config: cfg}); err != nil {
		return nil, err
	}

	wrote, _, err = e.apply(artifacts, false)
	return wrote, err
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
// It loads no journals and no state: CLAUDE.md is a pointer file whose whole
// content is derived from which skills exist (§6.1), so a full Derive over
// eight subjects would read four files apiece to produce bytes that do not
// depend on any of them.
func (e *Env) ClaudeSurface() ([]Artifact, error) {
	rules, err := e.rules()
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
// when the surface is off there — the fact that it should not exist.
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
	existing, err := e.read(path)
	if err != nil {
		return Artifact{}, err
	}
	a := Artifact{Path: path, Existing: existing, Present: existing != nil}

	if !render.HasClaude(loc, cfg) {
		return a, nil
	}
	a.Wanted = true
	if a.Derived, err = render.Claude.Render(render.In{Locator: loc, Config: cfg, Rules: rules}); err != nil {
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
// the ones that are not what they should be, reporting both lists.
//
// It is the whole of what a skill mutation owes the surface's first half, and
// it is separate from Run because Run re-derives a tree and this re-derives
// eight files.
func (e *Env) WriteClaudeSurface() (wrote, removed []string, err error) {
	artifacts, err := e.ClaudeSurface()
	if err != nil {
		return nil, nil, err
	}
	return e.apply(artifacts, false)
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
func (e *Env) SyncMirror(dryRun bool, pending []string) ([]mirror.Change, error) {
	cfg, err := e.Resolver.RenderConfig(nil)
	if err != nil {
		return nil, err
	}
	ids, err := tree.SkillIDs(e.Root)
	if err != nil {
		return nil, err
	}
	issues, err := mirror.Inspect(e.Root, cfg, ids, pending)
	if err != nil {
		return nil, err
	}
	if dryRun {
		return mirror.Planned(cfg, issues), nil
	}
	return mirror.Repair(e.Root, cfg, issues)
}
