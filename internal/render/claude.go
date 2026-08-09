package render

import (
	"bytes"
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// claudeRenderer renders CLAUDE.md: wholly para's, and a pointer file with no
// prose of its own — `@AGENTS.md` plus one `@` import per derived rule (§6.1).
//
// Rules reach Claude Code by import rather than by being copied or linked, so
// there is no second copy of a rule to drift, and the import list regenerates
// from the same walk that produces the rules.
type claudeRenderer struct{}

const claudeFile = "CLAUDE.md"

func (claudeRenderer) Path(in In) (string, error) {
	dir, err := in.dir()
	if err != nil {
		return "", err
	}
	return join(dir, claudeFile), nil
}

func (claudeRenderer) Render(in In) ([]byte, error) {
	if !HasAgents(in.Locator) {
		return nil, paraerr.Newf(paraerr.KindInternal,
			"render: %s is not emitted at %q", claudeFile, in.Locator.String())
	}
	dir, err := in.dir()
	if err != nil {
		return nil, err
	}
	up := upToRoot(dir)

	var b bytes.Buffer
	b.WriteString("@" + agentsFile + "\n")

	// Every derived rule, at all eight locations. A bucket's CLAUDE.md could
	// in principle import only the rules whose scope reaches that bucket, but
	// scope entries are frequently deeper than a bucket (§5.2 — an entry covers
	// its own subtree), so filtering here would drop rules that apply to the
	// work being done inside it. Importing the same set everywhere is the
	// answer that cannot be wrong; narrowing it is Phase 13's to revisit
	// against a real tree.
	rules := slices.Clone(in.Rules)
	slices.Sort(rules)
	for _, rule := range slices.Compact(rules) {
		b.WriteString("@" + up + RulesDir + "/" + rule + "\n")
	}
	return b.Bytes(), nil
}

// upToRoot is the relative prefix that reaches the tree root from dir, so a
// bucket's CLAUDE.md imports `@../.agents/rules/...` while the root's imports
// `@.agents/rules/...`. Claude Code resolves an `@` import relative to the file
// that carries it, so the prefix is not optional.
func upToRoot(dir string) string {
	if dir == "" {
		return ""
	}
	return strings.Repeat("../", strings.Count(dir, "/")+1)
}
