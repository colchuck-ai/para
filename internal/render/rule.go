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

// ruleRenderer renders a skill's derived rule file — one generated file, and
// all a rule ever is (§5.3). Every fact a routing rule needs is already in the
// skill's state.toml, so there is nothing here to author, own, or address.
type ruleRenderer struct{}

// RulesDir is where derived rule files live, relative to the tree root. The
// para- prefix on the filename is what makes ownership checkable: anything in
// here without it is yours, and para never reads or writes it (§5.3).
const RulesDir = ".agents/rules"

// ruleKeyGeneratedFrom is the one line of provenance a rule carries. It is what
// lets removing a skill remove its rule, and what lets doctor report a rule
// that outlived its skill (`orphan-rule`, §5.3, §10).
const ruleKeyGeneratedFrom = "generated_from"

// RuleFilename is the rule file basename for a skill id — the same para-<id>
// spelling the skill's own directory carries (§1.4).
func RuleFilename(id string) string { return "para-" + id + ".md" }

// RuleFilenames is CLAUDE.md's import list for a set of skill ids: one rule per
// skill, sorted, deduplicated.
//
// It takes *skills* and not a directory listing of .agents/rules/, and that is
// the whole point of it existing. A rule is a projection of a skill and nothing
// else (§5.3), so CLAUDE.md — itself a projection — must be derived from the
// skills, or rebuild would be reading a projection to produce a projection,
// which §21.1 forbids in those words. The consequence is not theoretical: a
// rebuild of a tree whose rule files are missing writes every CLAUDE.md before
// it writes the rules, so a list read off disk is a list of what has not been
// repaired yet, and the run needs a second pass to converge. Derived from the
// skills, it is right on the first.
//
// It also settles what happens to a rule file whose skill is gone: it is
// `orphan-rule` (§10), and para does not import residue it is simultaneously
// reporting.
func RuleFilenames(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, RuleFilename(id))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func (ruleRenderer) Path(in In) (string, error) {
	if in.Kind != kindmeta.KindSkill || len(in.Locator) != 2 {
		return "", paraerr.Newf(paraerr.KindInternal, "render: %q is not a skill", in.Locator.String())
	}
	return RulesDir + "/" + RuleFilename(in.Locator[1]), nil
}

func (ruleRenderer) Render(in In) ([]byte, error) {
	if in.Kind != kindmeta.KindSkill || len(in.Locator) != 2 {
		return nil, paraerr.Newf(paraerr.KindInternal, "render: %q is not a skill", in.Locator.String())
	}
	id := in.Locator[1]

	skillPath, err := in.Locator.Path()
	if err != nil {
		return nil, err
	}

	// §5.3's two forms, and the difference between them is the whole
	// difference between a tree-wide skill and a scoped one — visible in the
	// rule file rather than inferred. Omitting scope means the whole tree
	// (§5.2), so absence renders without the location clause.
	var b bytes.Buffer
	if where := scopeClause(in.State.Scope); where != "" {
		b.WriteString("When working under ")
		b.WriteString(where)
		b.WriteString(", use the ")
	} else {
		b.WriteString("Use the ")
	}
	b.WriteString("**")
	b.WriteString(flatten(in.State.Name))
	b.WriteString("** skill (`")
	b.WriteString(skillPath + "/" + skillFile)
	b.WriteString("`)")
	if desc := flatten(in.State.Description); desc != "" {
		b.WriteString(" ")
		b.WriteString(desc)
	}
	b.WriteString(".\n")

	fields := []mdfile.Field{{
		Key:   ruleKeyGeneratedFrom,
		Value: mdfile.String("para-" + id),
	}}
	return mdfile.Render(fields, b.Bytes())
}

// scopeClause renders scope as the location phrase §5.3 puts in the rule's
// routing sentence: each entry as the directory it names, since a rule is read
// by something looking at a path rather than at a locator. An entry covers that
// location and everything beneath it (§5.2), which the trailing slash is meant
// to suggest.
func scopeClause(scope []string) string {
	paths := make([]string, 0, len(scope))
	for _, entry := range scope {
		paths = append(paths, "`"+scopePath(entry)+"`")
	}
	switch len(paths) {
	case 0:
		return ""
	case 1:
		return paths[0]
	case 2:
		return paths[0] + " or " + paths[1]
	default:
		return strings.Join(paths[:len(paths)-1], ", ") + ", or " + paths[len(paths)-1]
	}
}

// scopePath is the directory a scope entry names. An entry that does not
// parse is rendered as written: a scope entry naming nothing is doctor's
// `scope-unresolved` finding to report (§5.4, §10), and swallowing it here
// would hide it from the file where a reader could notice it.
//
// The routing sentence stays a directory path (§5.3's own worked example,
// unchanged) even though scope is now stored as a dotted address (R24): a
// rule is read by something looking at a path on disk, not at an address,
// so the entry is converted to a Locator and back to a path exactly as it
// always was — only the parsing step, entry's stored form, has moved.
func scopePath(entry string) string {
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
	return path + "/"
}
