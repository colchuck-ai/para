package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/rebuild"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/view"
)

// Options is one doctor run.
type Options struct {
	// Scope narrows the scan to a locator and everything under it. The empty
	// locator is the whole tree, and is the only scan that checks the
	// tree-wide artifacts — the derived rules and the .claude/ mirror, which
	// belong to no entity's subtree (§5.3, §6.1).
	Scope locator.Locator
}

// Run performs the deep scan and reports what it found (§21.2).
//
// It returns an error only when the scan itself could not be performed — an
// unreadable directory, a scope naming nothing. Everything it was able to look
// at and found wrong is a finding, because a doctor that stopped at the first
// problem would report one thing per run on the tree that most needs several.
func Run(env *view.Env, opts Options) (Report, error) {
	s, err := newScan(env, opts)
	if err != nil {
		return Report{}, err
	}
	// The order below is the order the checks *run* in, not the order they are
	// reported in — sortFindings puts the report into §10's order at the end.
	// What matters here is that the fast walk happens first, since every later
	// check asks what it found.
	if err := s.deepScan(); err != nil {
		return Report{}, err
	}
	if err := s.checkSubjects(); err != nil {
		return Report{}, err
	}
	if err := s.checkMirrors(); err != nil {
		return Report{}, err
	}

	sortFindings(s.findings)
	return Report{Findings: s.findings}, nil
}

// scan is one run's state: where it is looking, what the fast walk found, and
// the findings so far.
type scan struct {
	env   *view.Env
	root  string
	scope locator.Locator

	// rb derives what every projection should contain. doctor never renders
	// anything itself — see the package doc.
	rb *rebuild.Env

	// scanRoot is where the filesystem scan starts: the tree root, or the
	// scope's directory.
	scanRoot string
	// skillsRoot is .agents/skills/, the second root the walk scans (§1.4).
	skillsRoot string

	// subjects are the places that own truth and projections, in walk order,
	// the root first when the whole tree is being scanned.
	subjects []rebuild.Subject
	// byPath is what the fast walk reached, keyed by absolute path. A path
	// missing from it that has a state.toml is `orphan` or `misplaced`.
	byPath map[string]tree.Node

	findings []Finding
}

func newScan(env *view.Env, opts Options) (*scan, error) {
	s := &scan{
		env:        env,
		root:       env.Root,
		scope:      opts.Scope,
		rb:         &rebuild.Env{Root: env.Root, Resolver: env.Resolver},
		scanRoot:   env.Root,
		skillsRoot: filepath.Join(env.Root, filepath.FromSlash(tree.SkillsDir())),
		byPath:     map[string]tree.Node{},
	}

	nodes, err := s.walkNodes()
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		s.byPath[n.Path] = n
		if !n.Stub {
			// A stub owns no files: it is a bare ancestry placeholder with no
			// .para/ and no README.md (§1.6), so there is nothing about it to
			// check for validity or drift.
			s.subjects = append(s.subjects, rebuild.Subject{Locator: n.Locator, Kind: n.Kind, Dir: n.Path})
		}
	}
	return s, nil
}

// walkNodes runs the §8.5 walk over the scanned region and fixes where the
// filesystem scan starts.
func (s *scan) walkNodes() ([]tree.Node, error) {
	if len(s.scope) == 0 {
		var nodes []tree.Node
		err := tree.Walk(s.root, func(n tree.Node) error {
			nodes = append(nodes, n)
			return nil
		})
		if err != nil {
			return nil, paraerr.Wrap(paraerr.KindInternal, err, "walking the tree")
		}
		// The root is not an entity — it is the tree (§8.1) — so the walk does
		// not visit it, and it owns four of §2.2's generated files.
		s.subjects = append(s.subjects, rebuild.Subject{Dir: s.root})
		return nodes, nil
	}

	resolves, err := tree.Resolves(s.root, s.scope)
	if err != nil {
		return nil, err
	}
	if !resolves {
		return nil, paraerr.Newf(paraerr.KindNotFound, "%s does not exist", doctorAddr(s.scope))
	}
	if s.scanRoot, err = s.scopeDir(); err != nil {
		return nil, err
	}
	return tree.Subtree(s.root, s.scope)
}

// scopeDir is the directory the scan starts in. A bare `skills` is the one
// locator with no directory of its own: it addresses the second root (§1.4),
// which is the directory the skills sit in.
func (s *scan) scopeDir() (string, error) {
	if len(s.scope) == 1 && s.scope[0] == "skills" {
		return s.skillsRoot, nil
	}
	return tree.ResolvePath(s.root, s.scope)
}

// deepScan is §21.2's "walks every directory including inside content" — the
// half of doctor the §8.5 walk cannot do, and the reason orphan and misplaced
// are findable at all.
//
// "Every directory" is taken at its word, including the ones para owns:
// `.para/` and `.claude/` are walked like anything else, because an entity
// hand-`mv`'d into one is exactly the vanishing §8.5 names as the walk's
// weakness and doctor exists to find. An earlier version skipped both, and the
// reason given for `.claude/` did not survive contact with §6.1: a `copy`-mode
// mirror never contains a `.para/` ("**`.para/` is never mirrored**"), so it
// cannot be read as a second entity, and a `symlink`-mode one is not descended
// because WalkDir reports a symlink as a non-directory whatever it points at
// (§21.2). §6.1 already prevents the phantom; the skip only hid orphans.
//
// `.git` is the one exception, and it is a different kind of thing: a
// repository's own object store is not the tree, cannot hold an entity, and is
// thousands of directories deep.
func (s *scan) deepScan() error {
	return filepath.WalkDir(s.scanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("scanning %s", s.rel(path)))
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" {
			return fs.SkipDir
		}
		if path == s.scanRoot {
			return nil
		}

		if fileExists(filepath.Join(path, ".para", "state.toml")) {
			if _, walked := s.byPath[path]; !walked {
				s.classifyUnreached(path)
			}
			return nil
		}
		s.checkUntracked(path)
		return nil
	})
}

// classifyUnreached decides why the fast walk did not visit a directory that
// holds a state.toml. §10 gives three answers and they are not
// interchangeable — each names a different repair.
//
// Reachability is asked first, because it dominates: a locator's legality says
// nothing about a directory the walk never gets to. An entity two levels down a
// content directory may be perfectly well shaped and is still invisible to
// every read, which is §8.5's named weakness and §1.5's error.
func (s *scan) classifyUnreached(path string) {
	rel := s.rel(path)

	if !s.reachable(path) {
		s.add(Finding{
			Kind: KindOrphan, Path: rel,
			Detail: "holds a state.toml the walk cannot reach — an entity beneath content",
		})
		return
	}

	loc, err := locator.FromPath(rel)
	if err != nil {
		s.add(Finding{Kind: KindMisplaced, Path: rel, Detail: message(err)})
		return
	}
	if _, err := tree.KindAt(loc); err != nil {
		// KindOf already tells the two apart: a reserved word used as an id is
		// tagged KindConflict, a position that derives no kind KindValidation
		// (§1.3, §10). Reading the tag rather than re-deriving the answer is
		// what keeps the finding and the refusal `add` would give in agreement.
		kind := KindMisplaced
		var perr *paraerr.Error
		if errors.As(err, &perr) && perr.Kind == paraerr.KindConflict {
			kind = KindCollision
		}
		s.add(Finding{Kind: kind, Path: rel, Locator: loc, Detail: message(err)})
		return
	}
	s.add(Finding{
		Kind: KindOrphan, Path: rel, Locator: loc,
		Detail: "holds a state.toml the walk cannot reach",
	})
}

// reachable reports whether the §8.5 walk descends all the way to path's
// parent — which is what decides whether a state.toml there is unreachable
// (`orphan`) or merely ill-placed (`misplaced`).
//
// A skill is where the walk stops going down: §1.3 gives skills one level, and
// everything inside a skill's directory is the author's (§5.1). So an entity
// planted under one is beneath content exactly as an entity under a project's
// notes/ directory is.
func (s *scan) reachable(path string) bool {
	for p := filepath.Dir(path); ; p = filepath.Dir(p) {
		if p == s.scanRoot || p == s.root || p == s.skillsRoot {
			return true
		}
		node, walked := s.byPath[p]
		if !walked || node.Kind == kindmeta.KindSkill {
			return false
		}
		if parent := filepath.Dir(p); parent == p {
			return false
		}
	}
}

// checkUntracked reports §10's one advisory: a plain directory sitting
// directly in a bucket, which is the one place only entities belong.
//
// A stub is not one, and the difference is what the walk already decided: a
// bare directory under archive/ that leads to something real is an ancestry
// placeholder (§1.6), and the walk reports it as such. Content inside an entity
// is content and is never reported, which falls out of asking about the parent
// rather than about the directory.
func (s *scan) checkUntracked(path string) {
	if node, walked := s.byPath[path]; walked && node.Stub {
		return
	}
	if locator.IsReserved(filepath.Base(path)) {
		// para's own directories sit in buckets too — `projects/.para/` is one.
		// A reserved word can never be an id (§1.4), so a directory carrying one
		// is not a thing that should have been an entity; it is para's, and
		// reporting it as loose filing would be reporting the tool on itself.
		return
	}
	parentLoc, err := locator.FromPath(s.rel(filepath.Dir(path)))
	if err != nil || !address.IsBucket(parentLoc) {
		return
	}
	s.add(Finding{
		Kind: KindUntracked, Path: s.rel(path),
		Detail: "is a plain directory in a bucket, where only entities belong",
	})
}

// add records a finding.
func (s *scan) add(f Finding) { s.findings = append(s.findings, f) }

// rel is an absolute path as every finding reports it: relative to the tree
// root, with forward slashes on every platform.
func (s *scan) rel(path string) string {
	r, err := filepath.Rel(s.root, path)
	if err != nil {
		return path
	}
	if r == "." {
		return ""
	}
	return filepath.ToSlash(r)
}

// join places a filename inside a root-relative directory.
func join(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// message is a nested error's own sentence, for a finding that supplies its
// own context.
func message(err error) string {
	var perr *paraerr.Error
	if errors.As(err, &perr) {
		return perr.Msg
	}
	return err.Error()
}

// doctorAddr is loc's dotted address (R24), the same conversion every other
// package that names a Locator in an error message asks of address.String
// (internal/mutate's relocateAddr; internal/cli's entityLocatorString and
// findingLocatorString; internal/view's viewAddr; internal/rebuild's
// rebuildAddr; internal/query's queryAddr). The scope reaching this refusal
// was already built by the CLI's own parseAddressArgs, which resolves it
// through address.Parse before doctor ever sees it — so the raw Locator
// string is a defensive fallback only, for the one case a scope built some
// other way (a test, a future caller) does not round-trip.
func doctorAddr(loc locator.Locator) string {
	s, err := address.String(loc)
	if err != nil {
		return loc.String()
	}
	return s
}
