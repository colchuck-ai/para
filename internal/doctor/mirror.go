package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/mirror"
	"github.com/colchuck-ai/para/internal/tree"
)

// mirrorPrefix marks para's own entries in .claude/skills/ and .agents/rules/.
// Everything there without it belongs to whoever put it there, and para neither
// reads nor removes it (§5.3, §6.1).
const mirrorPrefix = "para-"

// checkMirrors reports the findings about files that are projections of a skill
// but do not live under it: `orphan-rule`, `orphan-mirror`, and `broken-link`,
// plus the `stale-projection` a mirror can be in (§5.3, §6.1, §10).
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
	return s.checkMirrorEntries()
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

// checkMirrorEntries reports what is wrong with `.claude/skills/`.
//
// Nothing here decides what is wrong with a mirror; `mirror.Inspect` does, and
// `rebuild` acts on the same list. That split is the same one `stale-projection`
// already rests on: a doctor that classified links itself would be a second
// opinion about a directory it does not repair, and §10 gives two of these
// answers their own names — so the two consumers have to agree about which
// answer an entry gets.
//
// The mapping from a mirror state to a §10 finding is the whole of this
// function, and only `residue` is not obvious. A mirror left behind by turning
// `emit.claude` off is a generated thing that nothing generates any more, which
// is what `stale-projection` means, and `rebuild` is the repair that row
// promises. It is deliberately *not* reported as `orphan-mirror` or
// `broken-link` even when the skill is also gone or the link also dangles:
// those two sentences would send a reader to look at a skill or at their git
// config, when the answer is that the surface is switched off.
func (s *scan) checkMirrorEntries() error {
	cfg, err := s.env.Resolver.RenderConfig(nil)
	if err != nil {
		return err
	}
	ids, err := tree.SkillIDs(s.root)
	if err != nil {
		return err
	}
	issues, err := mirror.Inspect(s.root, cfg, ids)
	if err != nil {
		return err
	}
	for _, issue := range issues {
		s.add(Finding{Kind: mirrorFinding(issue.State), Path: issue.Path, Detail: issue.Detail})
	}
	return nil
}

// mirrorFinding is §10's name for one mirror state.
func mirrorFinding(state mirror.State) Kind {
	switch state {
	case mirror.StateOrphan:
		return KindOrphanMirror
	case mirror.StateBroken:
		return KindBrokenLink
	default:
		return KindStaleProjection
	}
}
