package rebuild

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mirror"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/writeset"
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

// residue is what a subject has on disk that nothing generates any more.
//
// Today there is exactly one: a CLAUDE.md at one of the eight locations with
// `emit.claude` off (§6.1). `.gitattributes` is deliberately not a second — it
// is *partly* generated, a block inside a file whose other lines are the
// repository's (§9), so turning `emit.gitattributes` off leaves a block to
// remove rather than a file, and removing the file would take lines para never
// wrote.
func (e *Env) residue(in render.In) ([]Artifact, error) {
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
// The config is resolved at the location rather than at the root, because
// `emit.claude` is chain-resolved like every other key (§7). §7's table calls
// the root where it "usefully lives", which is advice about where to put it and
// not a restriction on where it is read.
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
	for _, a := range artifacts {
		if !a.Stale() {
			continue
		}
		abs := filepath.Join(e.Root, filepath.FromSlash(a.Path))
		if !a.Wanted {
			if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
				return wrote, removed, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("removing %s", a.Path))
			}
			removed = append(removed, a.Path)
			continue
		}
		if err := writeset.WriteFile(abs, a.Derived); err != nil {
			return wrote, removed, err
		}
		wrote = append(wrote, a.Path)
	}
	return wrote, removed, nil
}

// SyncMirror brings `.claude/skills/` into line with the skills that exist, and
// reports what it did — or, under dryRun, what it would do.
//
// The config is the root's, because the mirror is one directory at the root:
// there is no per-bucket `.claude/`, so there is no location for a nearer level
// to answer for.
func (e *Env) SyncMirror(dryRun bool) ([]mirror.Change, error) {
	cfg, err := e.Resolver.RenderConfig(nil)
	if err != nil {
		return nil, err
	}
	ids, err := tree.SkillIDs(e.Root)
	if err != nil {
		return nil, err
	}
	issues, err := mirror.Inspect(e.Root, cfg, ids)
	if err != nil {
		return nil, err
	}
	if dryRun {
		return mirror.Planned(cfg, issues), nil
	}
	return mirror.Repair(e.Root, cfg, issues)
}
