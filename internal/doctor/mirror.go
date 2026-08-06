package doctor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
)

// mirrorDir is where the Claude Code compatibility surface mirrors skills
// (§6.1), relative to the tree root.
const mirrorDir = ".claude/skills"

// mirrorPrefix marks para's own entries in that directory. Everything there
// without it belongs to whoever put it there, and para neither reads nor
// removes it (§6.1).
const mirrorPrefix = "para-"

// checkMirrors reports the three findings about files that are projections of a
// skill but do not live under it: `orphan-rule`, `orphan-mirror`, and
// `broken-link` (§5.3, §6.1, §10).
//
// They run only on an unscoped scan, and that is a statement about where they
// live rather than a shortcut. A derived rule sits in .agents/rules/ and a
// mirror in .claude/skills/, neither of which is inside any entity's subtree —
// so `doctor projects.acme` cannot reach them, and reporting them anyway would
// make a scoped scan quietly tree-wide.
func (s *scan) checkMirrors() error {
	if len(s.scope) > 0 {
		return nil
	}
	skills := s.skillIDs()
	if err := s.checkRules(skills); err != nil {
		return err
	}
	return s.checkMirrorEntries(skills)
}

// skillIDs is the set of skills that exist, taken from the walk that already
// found them rather than from a second listing of .agents/skills/.
func (s *scan) skillIDs() map[string]bool {
	out := map[string]bool{}
	for _, sub := range s.subjects {
		if sub.Kind == kindmeta.KindSkill && len(sub.Locator) == 2 {
			out[sub.Locator[1]] = true
		}
	}
	return out
}

// checkRules reports §10's `orphan-rule`: a derived rule file whose
// `generated_from` skill is gone.
//
// The provenance line is what decides it, not the filename, and §5.3 put it in
// the file for exactly this: "it is what lets removing a skill remove its rule,
// and what lets doctor report a rule that outlived its skill". A para- prefixed
// file with no provenance at all is read by its name instead, since the prefix
// already claims it as para's and a rule nobody can trace is worse than one
// traced by convention.
func (s *scan) checkRules(skills map[string]bool) error {
	names, err := tree.Rules(s.root)
	if err != nil {
		return err
	}
	for _, name := range names {
		rel := tree.RulesDir() + "/" + name
		id, ok := s.ruleSkillID(rel, name)
		if !ok {
			continue
		}
		if !skills[id] {
			s.add(Finding{
				Kind: KindOrphanRule, Path: rel,
				Detail: fmt.Sprintf("is generated from skills.%s, which is gone", id),
			})
		}
	}
	return nil
}

// ruleSkillID reads a rule file's generated_from, falling back to its filename.
func (s *scan) ruleSkillID(rel, name string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(rel)))
	if err != nil {
		return tree.RuleSkillID(name)
	}
	front, _, err := mdfile.Split(data)
	if err != nil {
		return tree.RuleSkillID(name)
	}
	doc, err := mdfile.DecodeFrontmatter(front)
	if err != nil {
		return tree.RuleSkillID(name)
	}
	from, ok := doc.String("generated_from")
	if !ok {
		return tree.RuleSkillID(name)
	}
	id, ok := strings.CutPrefix(from, mirrorPrefix)
	if !ok || id == "" {
		return tree.RuleSkillID(name)
	}
	return id, true
}

// checkMirrorEntries reports the two findings about .claude/skills/ (§6.1):
// `orphan-mirror`, a para- prefixed entry whose skill is gone, and
// `broken-link`, a symlink-mode mirror that does not resolve.
//
// A plain file where a mirror belongs is a broken link too, and §10 says so
// outright: that is what a `core.symlinks=false` checkout leaves behind — git
// writes the link's target as the file's contents, so the mirror becomes a
// one-line text file pointing nowhere. It is worth naming as its own case
// because the repair is a config change plus a rebuild rather than anything the
// user did wrong.
//
// The directory is read with Lstat throughout: asking Stat about a mirror would
// follow the very link whose health is the question.
func (s *scan) checkMirrorEntries(skills map[string]bool) error {
	dir := filepath.Join(s.root, filepath.FromSlash(mirrorDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// No mirror is the default: emit.claude is off unless asked for
			// (§6.1), and a tree that never turned it on has nothing here.
			return nil
		}
		return paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", mirrorDir))
	}

	for _, entry := range entries {
		name := entry.Name()
		id, ok := strings.CutPrefix(name, mirrorPrefix)
		if !ok || id == "" {
			continue
		}
		rel := mirrorDir + "/" + name

		if !skills[id] {
			s.add(Finding{
				Kind: KindOrphanMirror, Path: rel,
				Detail: fmt.Sprintf("mirrors skills.%s, which is gone", id),
			})
			continue
		}

		switch {
		case entry.Type()&fs.ModeSymlink != 0:
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				s.add(Finding{
					Kind: KindBrokenLink, Path: rel,
					Detail: "is a symlink that does not resolve",
				})
			}
		case !entry.IsDir():
			s.add(Finding{
				Kind: KindBrokenLink, Path: rel,
				Detail: "is a plain file where a mirror belongs — a checkout with core.symlinks=false",
			})
		}
	}
	return nil
}
