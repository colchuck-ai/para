package render

import (
	"bytes"
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// cursorRuleRenderer renders a skill's derived rule file for Cursor (§6.2):
// wholly generated, one per skill, and — unlike §5.3's Claude rule — the only
// input it needs is already in the skill's own state.toml.
type cursorRuleRenderer struct{}

// CursorRulesDir is where Cursor's derived rule files live, relative to the
// tree root. Its own para- prefix is what makes ownership checkable, the same
// as RulesDir (§6.2).
const CursorRulesDir = ".cursor/rules"

// cursorSkillsDir is where skills are mirrored for Cursor (§6.2). Spelled here
// rather than imported from a mirror package, for the same layering reason
// RulesDir is spelled independently of tree's rulesDir: mirror will depend on
// render, so render cannot depend on mirror. The string is the one this
// package's rule body needs and the one mirror's Cursor target writes to; both
// must agree, and neither reads the other's constant to do it.
const cursorSkillsDir = ".cursor/skills"

// cursorRuleKeyGeneratedFrom is the same provenance line §5.3's rule carries.
const cursorRuleKeyGeneratedFrom = "generated_from"

// cursorRuleKeyAlwaysApply is Cursor's own frontmatter key for a rule that
// applies everywhere.
const cursorRuleKeyAlwaysApply = "alwaysApply"

// cursorRuleKeyGlobs is Cursor's own frontmatter key for a rule scoped to
// specific paths.
const cursorRuleKeyGlobs = "globs"

// CursorRuleFilename is the Cursor rule file basename for a skill id — the
// same para-<id> spelling every other generated name for the skill carries,
// with Cursor's own `.mdc` extension rather than Claude's `.md` (§6.2).
func CursorRuleFilename(id string) string { return "para-" + id + ".mdc" }

// CursorRuleFilenames is CursorRules' generation-side counterpart, for
// whatever eventually needs "one entry per skill, sorted, deduplicated" the
// way RuleFilenames already does for CLAUDE.md's import list. Unlike
// RuleFilenames, nothing reads this list back into a generated file today —
// Cursor has no shared-path pointer file (§6.2) — but it is derived from
// skills rather than from a directory listing for the identical reason: a
// projection must not be derived from a directory of other projections
// (§21.1).
func CursorRuleFilenames(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, CursorRuleFilename(id))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func (cursorRuleRenderer) Path(in In) (string, error) {
	if in.Kind != kindmeta.KindSkill || len(in.Locator) != 2 {
		return "", paraerr.Newf(paraerr.KindInternal, "render: %q is not a skill", in.Locator.String())
	}
	return CursorRulesDir + "/" + CursorRuleFilename(in.Locator[1]), nil
}

func (cursorRuleRenderer) Render(in In) ([]byte, error) {
	if in.Kind != kindmeta.KindSkill || len(in.Locator) != 2 {
		return nil, paraerr.Newf(paraerr.KindInternal, "render: %q is not a skill", in.Locator.String())
	}
	id := in.Locator[1]

	var b bytes.Buffer
	b.WriteString("Use the **")
	b.WriteString(flatten(in.State.Name))
	b.WriteString("** skill (`")
	b.WriteString(cursorSkillsDir)
	b.WriteString("/para-")
	b.WriteString(id)
	b.WriteString("/")
	b.WriteString(skillFile)
	b.WriteString("`)")
	if desc := flatten(in.State.Description); desc != "" {
		b.WriteString(" ")
		b.WriteString(desc)
	}
	b.WriteString(".\n")

	var fields []mdfile.Field
	if globs := cursorScopeGlobs(in.State.Scope); globs != "" {
		fields = append(fields, mdfile.Field{Key: cursorRuleKeyGlobs, Value: mdfile.String(globs)})
	} else {
		fields = append(fields, mdfile.Field{Key: cursorRuleKeyAlwaysApply, Value: mdfile.Bool(true)})
	}
	fields = append(fields, mdfile.Field{
		Key:   cursorRuleKeyGeneratedFrom,
		Value: mdfile.String("para-" + id),
	})
	return mdfile.Render(fields, b.Bytes())
}

// cursorScopeGlobs renders scope as Cursor's own frontmatter shape: one
// `path/**` glob per resolved entry, comma-separated, in the order scope is
// stored (§6.2). Omitting scope means the whole tree (§5.2), so an empty
// scope renders no globs at all — the caller reads that as "use
// alwaysApply instead".
func cursorScopeGlobs(scope []string) string {
	globs := make([]string, 0, len(scope))
	for _, entry := range scope {
		globs = append(globs, cursorScopeGlob(entry))
	}
	return strings.Join(globs, ",")
}

// cursorScopeGlob is the glob one scope entry contributes: the directory it
// names (§5.2), with the trailing slash rule.go's scopePath adds swapped for
// Cursor's own `**` suffix. An entry that does not parse is rendered as
// written, the same refusal-to-swallow rule.go's scopePath uses — doctor's
// `scope-unresolved` finding is what should catch it (§5.4, §10), not a
// renderer that hides it.
func cursorScopeGlob(entry string) string {
	addr, err := address.ParseDotted(entry)
	if err != nil {
		return entry
	}
	loc, err := addr.ToLocator()
	if err != nil {
		return entry
	}
	path, err := loc.Path()
	if err != nil {
		return entry
	}
	return path + "/**"
}
