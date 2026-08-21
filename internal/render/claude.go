package render

import (
	"bytes"
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// claudeRenderer renders CLAUDE.md as a para-owned block delimited by
// mdfile.MarkdownMarkers, the same pair AGENTS.md uses — a pointer with no
// prose of its own — `@AGENTS.md` plus one `@` import per derived rule
// (§6.1) — living inside a file para no longer owns outright. §2.2's interop
// test decides this: some other tool may write CLAUDE.md too, so para keeps
// only its block current and leaves every other byte where it found it.
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

	path := join(dir, claudeFile)
	// Append rather than replace, for the same reason as .gitattributes (R2):
	// a CLAUDE.md that predates para — or is shared with another tool — keeps
	// every byte, and para's block goes at the end. ReplaceDelimited's refusal
	// of a marker-less file is right for AGENTS.md, which para itself wrote,
	// but wrong here, where a marker-less file is the normal first encounter.
	out, err := mdfile.AppendDelimited(mdfile.MarkdownMarkers, in.existing(path), b.Bytes())
	if err != nil {
		return nil, damagedBlock(path, err)
	}
	return out, nil
}

// WithoutClaudeBlock returns existing with para's block taken out, and
// reports whether there was one to take. It is Render's other half, for
// `emit.claude = false` (§6.1), and withoutBlock's second caller alongside
// WithoutGitAttributesBlock.
//
// CLAUDE.md has no fixed path — it lives at eight locations (agentsLocations)
// — so unlike .gitattributes this takes the path as a parameter rather than
// closing over a constant.
func WithoutClaudeBlock(path string, existing []byte) ([]byte, bool, error) {
	return withoutBlock(mdfile.MarkdownMarkers, path, existing)
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
