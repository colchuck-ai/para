// Package mirror owns `.claude/skills/` and `.cursor/skills/`: the half of
// the Claude Code and Cursor compatibility surfaces that an import or a rule
// file cannot express, because a skill ships scripts and references and not
// just prose (design §6.1, §6.2).
//
// It is one package with two consumers and one opinion per target, which is
// the whole reason it exists. `doctor` reports what is wrong with a mirror and
// `rebuild` repairs it, and §10 gives two of those reports their own names —
// `orphan-mirror` and `broken-link` — so the classification has to be shared
// rather than written twice. Inspect is that classification: it says what is
// wrong with every entry, once, and Repair acts on exactly the list Inspect
// produced. A doctor that decided for itself which links were broken would be
// a second opinion about the same directory.
//
// Claude and Cursor are two instances of one Target, not two implementations:
// Inspect, Planned, and Repair take a Target and do not otherwise care which
// one it is. The two directories are independent surfaces (§6.2 — "neither
// implies the other"), never read together and never pruned into each other.
//
// Three rules from §6.1 (and §6.2, for the Cursor target) are enforced here
// and nowhere else:
//
//   - **`.para/` is never mirrored.** A reachable
//     `.claude/skills/para-X/.para/state.toml` (or its Cursor equivalent)
//     would satisfy §1.2's entity test, and doctor's deep scan would report a
//     phantom orphan living outside the tree. In copy mode it is excluded; in
//     symlink mode doctor does not follow para-owned links.
//
//   - **Ownership is the name, not a frontmatter line.** `generated_from`
//     cannot live on a symlink, so anything under a target's directory
//     carrying the `para-` prefix is para's and anything else is yours —
//     never read, never written, never removed.
//
//   - **Both modes are projections.** Nothing here is ever read back as truth.
//     A copy that drifts is stale, not a second source, and the repair for
//     every state below is the same one: write what the skill says.
//
// Writes here are ordinary, not atomic. Replacing a mirror means removing a
// directory and writing a new one, which no per-file rename can make atomic
// anyway, and a crash midway leaves precisely the degraded state §0.2 defines:
// truth correct, a projection stale, `doctor` naming it and `rebuild` fixing
// it.
package mirror

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/render"
)

// prefix marks para's own entries. It is the same prefix a skill's own
// directory carries, and it is what makes ownership checkable without
// frontmatter.
const prefix = "para-"

// skillsDir is where the skills themselves live (§1.4), the same for every
// target: both mirrors copy or link the one skill directory under
// .agents/skills/.
const skillsDir = ".agents/skills"

// Target names one mirror surface — Claude's or Cursor's — and the four
// things the two differ on: where mirrors live, what governs turning the
// surface on, which mode it writes in, and what a leftover entry's report
// says. Inspect, Planned, and Repair are the same three functions either way;
// only these answers change per target, which is what keeps the two surfaces
// from drifting into two implementations of one idea (§6.1, §6.2).
type Target struct {
	// Dir is where this target's mirrors live, relative to the tree root,
	// slash-separated.
	Dir string
	// parentDir is the directory directly above Dir. Para writes nothing
	// else under it, and removes it only when the sweep leaves it empty.
	parentDir string
	// residueDetail is the sentence a report prints about anything this
	// surface left behind when its flag turns off.
	residueDetail string
	// enabled reports whether cfg turns this surface on.
	enabled func(render.Config) bool
	// mode reports the configured mirror mode ("symlink" or "copy") for this
	// surface.
	mode func(render.Config) string
}

// Claude is the §6.1 target: .claude/skills/, governed by emit.claude and
// emit.claude-skills.
var Claude = Target{
	Dir:           ".claude/skills",
	parentDir:     ".claude",
	residueDetail: ResidueDetail,
	enabled:       func(c render.Config) bool { return c.EmitClaude },
	mode:          func(c render.Config) string { return c.EmitClaudeSkills },
}

// Cursor is the §6.2 target: .cursor/skills/, governed by emit.cursor and
// emit.cursor-skills.
var Cursor = Target{
	Dir:           ".cursor/skills",
	parentDir:     ".cursor",
	residueDetail: "should not exist; emit.cursor is off",
	enabled:       func(c render.Config) bool { return c.EmitCursor },
	mode:          func(c render.Config) string { return c.EmitCursorSkills },
}

// Name is the mirror's directory name for a skill id — the same for every
// target, since it is the skill's own directory name that gets mirrored.
func (t Target) Name(id string) string { return prefix + id }

// LinkTarget is the body of a symlink-mode mirror: relative, and therefore
// unchanged by cloning the tree somewhere else or moving it. Two levels up
// from t.Dir is the tree root.
func (t Target) LinkTarget(id string) string { return "../../" + skillsDir + "/" + t.Name(id) }

// Path is a mirror's location relative to the tree root, slash-separated.
func (t Target) Path(id string) string { return t.Dir + "/" + t.Name(id) }

// State is what is wrong with one mirror entry. There is no state for "fine":
// Inspect returns only entries that need something done, so an empty result is
// a clean mirror.
type State string

const (
	// StateMissing is a skill with no mirror at all.
	StateMissing State = "missing"
	// StateStale is a mirror in the wrong shape for the mode, or a copy whose
	// bytes no longer match the skill (§6.1 — "doctor reports the divergence
	// as stale-projection").
	StateStale State = "stale"
	// StateResidue is a mirror left behind by turning `emit.claude` off. It is
	// not an orphan — the skill is right there — and it is not broken. It is
	// the old shape of a surface that has been switched off, and §6.1 makes
	// removing it part of the switch.
	StateResidue State = "residue"
	// StateOrphan is §10's `orphan-mirror`: a para- prefixed entry whose skill
	// is gone.
	StateOrphan State = "orphan"
	// StateBroken is §10's `broken-link`: a symlink-mode mirror that does not
	// resolve, including one materialised as a plain file by a
	// core.symlinks=false checkout.
	StateBroken State = "broken"
)

// ResidueDetail is the sentence a report prints about anything the Claude
// surface left behind when it was turned off. It is one constant because two
// packages print it — this one about a mirror, and doctor about a CLAUDE.md —
// and they are saying the same thing about the same cause.
const ResidueDetail = "should not exist; emit.claude is off"

// Issue is one mirror entry that is not what §6.1 says it should be.
type Issue struct {
	// ID is the skill the entry names, taken from the directory name rather
	// than from anything inside it.
	ID string
	// Path is the entry, relative to the tree root and slash-separated.
	Path string
	// State is what is wrong with it.
	State State
	// Detail is the sentence a report prints after the path.
	Detail string
}

// Verb is what a repair did to one entry, and the word the CLI prints.
type Verb string

const (
	VerbLinked  Verb = "linked"
	VerbCopied  Verb = "copied"
	VerbRemoved Verb = "removed"
)

// Would is the verb in the tense `rebuild --dry-run` needs: a run that says
// "linked" when it linked nothing is describing something that did not happen.
//
// It lives beside the enum rather than in the package that prints it, so a
// fourth verb arrives with its own tense instead of falling into somebody
// else's default and printing a plausible lie.
func (v Verb) Would() string {
	switch v {
	case VerbLinked:
		return "would link"
	case VerbCopied:
		return "would copy"
	case VerbRemoved:
		return "would remove"
	default:
		return "would " + string(v)
	}
}

// Change is one repair, as §26's `linked …/para-signups-report → …` line
// reports it.
type Change struct {
	Verb Verb
	// Path is the mirror, root-relative and slash-separated.
	Path string
	// Target is a link's target, and empty for the other two verbs.
	Target string
}

// Inspect classifies every mirror entry against what cfg and skills say should
// be there, and returns only the ones needing work, ordered by path.
//
// skills is the ids of the skills that exist — the same list CLAUDE.md's import
// list is built from, so the two halves of the surface can never disagree about
// which skills there are.
//
// pending names skills whose own files the caller has not written yet but is
// about to. It exists for `rebuild --dry-run` and for nothing else: a copy-mode
// mirror of a skill whose SKILL.md is about to be rewritten is stale, but on
// disk it still matches the version it was copied from, so a dry run comparing
// bytes would report no mirror work and the run it is predicting would do some.
// Every other caller passes nil, because every other caller is looking at a
// tree that has already been written.
//
// When a target's flag is off, every para- prefixed entry is residue and
// nothing else. That is deliberate, and it answers the question Phase 12 left
// open: naming a doomed entry `orphan-mirror` would send the reader to look at
// a skill, and `broken-link` would tell them their checkout mangled a link
// that is about to be deleted. Both would be true and neither would be the
// repair. The surface is off; the entry goes.
func Inspect(root string, target Target, cfg render.Config, skills, pending []string) ([]Issue, error) {
	want := map[string]bool{}
	if target.enabled(cfg) {
		for _, id := range skills {
			want[id] = true
		}
	}
	stale := map[string]bool{}
	for _, id := range pending {
		stale[id] = true
	}

	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(target.Dir)))
	if err != nil {
		if os.IsNotExist(err) {
			// No mirror directory is the default: the surface is off unless
			// asked for (§6.1, §6.2). Every wanted skill is then missing.
			if len(want) == 0 {
				return nil, nil
			}
			entries = nil
		} else {
			return nil, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", target.Dir))
		}
	}

	var issues []Issue
	seen := map[string]bool{}
	for _, entry := range entries {
		id, ok := strings.CutPrefix(entry.Name(), prefix)
		if !ok || id == "" {
			// Not para's. §6.1 is explicit that para neither reads nor removes
			// it, and its presence is also what keeps the sweep from pruning a
			// directory somebody else is using.
			continue
		}
		seen[id] = true
		if issue, bad := classify(root, target, cfg, want, stale, id, entry); bad {
			issues = append(issues, issue)
		}
	}

	for id := range want {
		if seen[id] {
			continue
		}
		issues = append(issues, Issue{
			ID: id, Path: target.Path(id), State: StateMissing,
			Detail: fmt.Sprintf("is missing; skill.%s has no mirror", id),
		})
	}

	sort.Slice(issues, func(i, j int) bool { return issues[i].Path < issues[j].Path })
	return issues, nil
}

// classify decides what, if anything, is wrong with one existing entry.
func classify(root string, target Target, cfg render.Config, want, pending map[string]bool, id string, entry fs.DirEntry) (Issue, bool) {
	issue := Issue{ID: id, Path: target.Path(id)}
	switch {
	case !target.enabled(cfg):
		issue.State, issue.Detail = StateResidue, target.residueDetail
		return issue, true
	case !want[id]:
		issue.State, issue.Detail = StateOrphan, fmt.Sprintf("mirrors skill.%s, which is gone", id)
		return issue, true
	}

	abs := filepath.Join(root, filepath.FromSlash(target.Dir), entry.Name())
	link := entry.Type()&fs.ModeSymlink != 0

	if target.mode(cfg) == render.MirrorCopy {
		switch {
		case link:
			issue.State, issue.Detail = StateStale, "is a symlink where a copy belongs"
		case !entry.IsDir():
			issue.State, issue.Detail = StateStale, "is a plain file where a copy belongs"
		case pending[id]:
			// The bytes on disk still agree, and are both about to change. See
			// Inspect's `pending`.
			issue.State, issue.Detail = StateStale, "differs from the skill it mirrors"
		default:
			same, err := sameCopy(sourceDir(root, id), abs)
			if err != nil || !same {
				issue.State, issue.Detail = StateStale, "differs from the skill it mirrors"
			}
		}
		return issue, issue.State != ""
	}

	switch {
	case link:
		// Resolution is asked before the target is compared, because an
		// unresolvable link is the failure §6.1 gives its own name and its own
		// cause — a checkout, not an edit — whatever it happens to point at.
		if _, err := os.Stat(abs); err != nil {
			issue.State, issue.Detail = StateBroken, "is a symlink that does not resolve"
			break
		}
		// ToSlash before comparing, because LinkTarget is slash-separated by
		// contract and the link was written through filepath.FromSlash. On
		// Windows os.Readlink hands back `..\..\.agents\skills\para-x`, which
		// is the same link — so without this, doctor reported `stale-projection`
		// on a symlink para had just created, rebuild rewrote it to the
		// identical bytes, and doctor stayed red forever. The write converts
		// one way and the read has to convert back; a rule applied at one end
		// of a round trip and not the other is not a rule.
		if link, err := os.Readlink(abs); err != nil || filepath.ToSlash(link) != target.LinkTarget(id) {
			issue.State, issue.Detail = StateStale, fmt.Sprintf("points at %q, not at the skill", link)
		}
	case entry.IsDir():
		issue.State, issue.Detail = StateStale, "is a copied directory where a symlink belongs"
	default:
		issue.State, issue.Detail = StateBroken,
			"is a plain file where a mirror belongs — a checkout with core.symlinks=false"
	}
	return issue, issue.State != ""
}

// Planned is what Repair would report without touching anything — `rebuild
// --dry-run`'s half of the surface (§21.1).
//
// It shares plan with Repair rather than reproducing its reasoning, because a
// dry run whose list differs from what the real run does is worse than no dry
// run at all.
func Planned(target Target, cfg render.Config, issues []Issue) []Change {
	out := make([]Change, 0, len(issues))
	for _, issue := range issues {
		out = append(out, plan(target, cfg, issue))
	}
	return out
}

// plan is what fixing one issue amounts to: a removal, or a removal followed by
// a write in the configured mode.
func plan(target Target, cfg render.Config, issue Issue) Change {
	switch {
	case issue.State == StateResidue || issue.State == StateOrphan:
		return Change{Verb: VerbRemoved, Path: issue.Path}
	case target.mode(cfg) == render.MirrorCopy:
		return Change{Verb: VerbCopied, Path: issue.Path}
	default:
		return Change{Verb: VerbLinked, Path: issue.Path, Target: target.LinkTarget(issue.ID)}
	}
}

// Repair fixes every issue and reports what it did, in the order the issues
// were given.
//
// Every state has the same two-step repair — remove whatever is there, then
// write what the skill says — because a mirror is a projection and there is
// nothing in it worth salvaging. What differs is only whether a write follows
// the removal.
func Repair(root string, target Target, cfg render.Config, issues []Issue) ([]Change, error) {
	var changes []Change
	removed := false

	for _, issue := range issues {
		change := plan(target, cfg, issue)
		abs := filepath.Join(root, filepath.FromSlash(issue.Path))
		if err := os.RemoveAll(abs); err != nil {
			return changes, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("removing %s", issue.Path))
		}
		if change.Verb == VerbRemoved {
			removed = true
			changes = append(changes, change)
			continue
		}
		if err := write(root, target, cfg, issue.ID); err != nil {
			return changes, err
		}
		changes = append(changes, change)
	}

	if removed {
		if err := pruneEmpty(root, target); err != nil {
			return changes, err
		}
	}
	return changes, nil
}

// write puts the mirror for one skill in place, in whichever mode is
// configured. Its destination is known not to exist: Repair removes it first.
func write(root string, target Target, cfg render.Config, id string) error {
	rel := target.Path(id)
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("creating %s", target.Dir))
	}

	if target.mode(cfg) == render.MirrorCopy {
		return copyTree(sourceDir(root, id), abs)
	}
	if err := os.Symlink(filepath.FromSlash(target.LinkTarget(id)), abs); err != nil {
		// The one failure with a configuration answer rather than a filesystem
		// one: a platform that will not create links is exactly what the
		// copy mode exists for (§6.1, §6.2).
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("linking %s (switch this surface's skills mode to \"copy\" where links do not survive)", rel))
	}
	return nil
}

// pruneEmpty removes target.Dir and then its parent when the sweep has left
// them empty, so that turning the surface off returns the tree to the shape
// §26's `init` describes: "no CLAUDE.md, no .claude/" — and the Cursor
// equivalent.
//
// Neither is removed while anything is still in it. The parent directory in
// particular is one the IDE itself writes to, and §6.1/§6.2's "para never
// reads or writes anything else under it" cuts both ways.
func pruneEmpty(root string, target Target) error {
	for _, rel := range []string{target.Dir, target.parentDir} {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		entries, err := os.ReadDir(abs)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", rel))
		}
		if len(entries) > 0 {
			return nil
		}
		if err := os.Remove(abs); err != nil {
			return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("removing %s", rel))
		}
	}
	return nil
}

// sourceDir is the skill a mirror mirrors, as an absolute OS path. It is the
// §1.4 mapping `skills.<id>` ↔ `.agents/skills/para-<id>`, spelled here rather
// than taken from locator so that this package needs no locator to answer a
// question about a directory name it already has. The same for every target:
// both mirror the one skill directory.
func sourceDir(root, id string) string {
	return filepath.Join(root, filepath.FromSlash(skillsDir), prefix+id)
}

// file is one entry of a copy-mode mirror: where it sits inside the skill,
// whether it is executable, and — for a symlink — what it points at. Link is
// empty for a regular file, which is what tells the two apart.
type file struct {
	Rel  string
	Exec bool
	Link string
}

// contents lists what a copy consists of, relative to dir and slash-separated,
// sorted.
//
// Regular files and symlinks, and nothing else. A device, socket, or fifo
// inside a skill directory is not something a copy of a skill contains, and
// skipping it on both sides is what keeps the comparison from declaring a
// mirror stale on every pass over something no copy could ever hold.
//
// skipPara excludes every `.para/` directory, at any depth, which is what makes
// this the source side of the comparison; the destination is listed without the
// exclusion so that a `.para/` which somehow appeared in a mirror reads as an
// extra file rather than as agreement.
//
// One function lists both sides, because "what a copy consists of" is a single
// rule and two copies of it would let the writer and the comparer drift — the
// mirror would then be rewritten on every rebuild, or never.
func contents(dir string, skipPara bool) ([]file, error) {
	var out []file
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipPara && d.Name() == ".para" {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		f := file{Rel: filepath.ToSlash(rel)}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			// Carried as a link rather than as the bytes it resolves to, so a
			// skill author's own link keeps meaning what they wrote — and so a
			// link out of the skill cannot smuggle a .para/ into the mirror.
			if f.Link, err = os.Readlink(path); err != nil {
				return err
			}
		case d.Type().IsRegular():
			info, err := d.Info()
			if err != nil {
				return err
			}
			f.Exec = info.Mode().Perm()&0o111 != 0
		default:
			return nil
		}
		out = append(out, f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
}

// sameCopy reports whether the copy at dst is byte-for-byte what a copy of src
// would be: the same files, the same executable bits, the same contents.
//
// An error reading either side is not distinguished from a difference. The
// caller's only two answers are "leave it" and "write it again", and a mirror
// nobody can read is one that should be written again.
func sameCopy(src, dst string) (bool, error) {
	want, err := contents(src, true)
	if err != nil {
		return false, err
	}
	got, err := contents(dst, false)
	if err != nil {
		return false, err
	}
	if len(want) != len(got) {
		return false, nil
	}
	for i := range want {
		// Name, executable bit, and link target in one comparison: the struct
		// holds exactly what a copy reproduces, so equality here is equality of
		// everything but the bytes.
		if want[i] != got[i] {
			return false, nil
		}
		if want[i].Link != "" {
			continue
		}
		a, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(want[i].Rel)))
		if err != nil {
			return false, err
		}
		b, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(got[i].Rel)))
		if err != nil {
			return false, err
		}
		if !bytes.Equal(a, b) {
			return false, nil
		}
	}
	return true, nil
}

// copyTree writes a copy-mode mirror of the skill at src into dst, which does
// not exist yet.
//
// It copies exactly the files contents lists, so the thing written and the
// thing compared are the same set by construction. Empty directories are not
// carried: nothing reads a mirror as a directory listing, and git does not
// carry one either.
func copyTree(src, dst string) error {
	files, err := contents(src, true)
	if err != nil {
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", src))
	}
	for _, f := range files {
		target := filepath.Join(dst, filepath.FromSlash(f.Rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("creating the directory for %s", f.Rel))
		}
		if f.Link != "" {
			if err := os.Symlink(f.Link, target); err != nil {
				return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("linking %s", f.Rel))
			}
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(f.Rel)))
		if err != nil {
			return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", f.Rel))
		}
		mode := os.FileMode(0o644)
		if f.Exec {
			// A skill that ships a script ships it runnable; a copy that
			// dropped the bit would be a mirror the agent cannot use.
			mode = 0o755
		}
		if err := os.WriteFile(target, data, mode); err != nil {
			return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("writing %s", f.Rel))
		}
		if err := os.Chmod(target, mode); err != nil {
			return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("setting the mode of %s", f.Rel))
		}
	}
	return nil
}
