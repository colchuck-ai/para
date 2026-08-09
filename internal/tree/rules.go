package tree

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// rulesDir is where derived rule files live, relative to the tree root (§5.3).
// It is spelled here rather than imported from render because render is the
// package that *writes* files and this one is the package that reports what is
// on disk; a dependency from the second to the first would invert the layering
// for the sake of one constant.
const rulesDir = ".agents/rules"

// rulePrefix is what makes ownership checkable: a file in .agents/rules/
// without it is yours, and para never reads or writes it (§5.3).
const rulePrefix = "para-"

// Rules lists the derived rule files that are *on disk* in .agents/rules/, as
// basenames, sorted.
//
// It has exactly one caller, and the restriction is deliberate: doctor's
// `orphan-rule` finding, which asks which rule files exist so that it can name
// the ones whose skill is gone (§5.3, §10). Nothing that *generates* a file may
// use it. CLAUDE.md's import list looks like a second caller and is not — a
// rule is a projection of a skill (§5.3), so the list comes from the skills
// through render.RuleFilenames, because deriving a projection from a directory
// of projections is what §21.1 forbids and what left rebuild needing two passes
// to converge.
//
// A tree with no .agents/rules/ has no rules and is not an error: the
// directory appears with the first skill.
func Rules(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(rulesDir)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", rulesDir))
	}

	var out []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(name, rulePrefix) && strings.HasSuffix(name, ".md") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// RuleSkillID is the skill id a rule file's basename names, and whether the
// basename is a rule file at all — the inverse of render.RuleFilename, and
// what doctor's `orphan-rule` asks of every file it finds (§5.3, §10).
func RuleSkillID(basename string) (string, bool) {
	id, ok := strings.CutPrefix(basename, rulePrefix)
	if !ok {
		return "", false
	}
	id, ok = strings.CutSuffix(id, ".md")
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

// RulesDir is .agents/rules/ relative to the tree root, slash-separated — the
// form every reported path takes.
func RulesDir() string { return rulesDir }

// skillsDir is the other half of .agents/: where a skill's directory lives,
// relative to the tree root (§1.4, §5.1).
const skillsDir = ".agents/skills"

// SkillsDir is .agents/skills/ in the same form, for the callers that need the
// directory itself rather than a skill inside it: the walk that scans it, the
// deep scan that starts from it, and `init`, which creates it empty so a fresh
// tree has the shape §5 describes.
func SkillsDir() string { return skillsDir }
