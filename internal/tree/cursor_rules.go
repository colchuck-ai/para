package tree

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// cursorRulesDir is where Cursor's derived rule files live, relative to the
// tree root (§6.2). Spelled here rather than imported from render for the same
// layering reason rulesDir is: this package reports what is on disk, render is
// the package that writes it.
const cursorRulesDir = ".cursor/rules"

// cursorRuleSuffix is Cursor's own rule extension, not Claude's — the file
// format Cursor reads is `.mdc`, not `.md` (§6.2).
const cursorRuleSuffix = ".mdc"

// CursorRules lists the derived rule files that are *on disk* in
// .cursor/rules/, as basenames, sorted.
//
// Its one caller is doctor's `orphan-rule` finding (§6.2, §10), the same
// restriction Rules carries and for the same reason: a rule is a projection of
// a skill, so anything that *generates* one derives the list from the skills
// via render.CursorRuleFilenames, not from a directory listing of its own
// output (§21.1).
//
// A tree with no .cursor/rules/ has no rules and is not an error: the
// directory appears only once emit.cursor is on and a skill exists.
func CursorRules(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(cursorRulesDir)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", cursorRulesDir))
	}

	var out []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(name, rulePrefix) && strings.HasSuffix(name, cursorRuleSuffix) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// CursorRuleSkillID is the skill id a Cursor rule file's basename names, and
// whether the basename is a rule file at all — the inverse of
// render.CursorRuleFilename, and what doctor's `orphan-rule` asks of every
// file it finds under .cursor/rules/ (§6.2, §10).
func CursorRuleSkillID(basename string) (string, bool) {
	id, ok := strings.CutPrefix(basename, rulePrefix)
	if !ok {
		return "", false
	}
	id, ok = strings.CutSuffix(id, cursorRuleSuffix)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

// CursorRulesDir is .cursor/rules/ relative to the tree root, slash-separated
// — the form every reported path takes.
func CursorRulesDir() string { return cursorRulesDir }
