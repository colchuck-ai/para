package render

import (
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// skillRenderer renders SKILL.md's frontmatter over its human-owned body
// (§2.2, §5.1). A skill is the one entity whose identity file is not
// README.md, because SKILL.md is what an agent harness opens.
type skillRenderer struct{}

// skillFile is the name the .agents/ convention fixes.
const skillFile = "SKILL.md"

func (skillRenderer) Path(in In) (string, error) {
	dir, err := in.dir()
	if err != nil {
		return "", err
	}
	return join(dir, skillFile), nil
}

// SKILL.md's frontmatter is deliberately just these two keys, and not the
// locator, kind, tags, or scope that README.md would carry. This file is read
// by agent harnesses that have their own frontmatter schema, so para keeps to
// the two fields §5.1 gives it a reason to write: `name`, and `description` —
// "the when-to-use hook and the only part that ever enters an agent's context
// automatically". Everything else about the skill is in its state.toml, which
// is where para reads it from anyway; a skill needs no locator in its
// frontmatter to be identifiable, because the .para/ inside its directory
// already identifies it.
func (skillRenderer) Render(in In) ([]byte, error) {
	if in.Kind != kindmeta.KindSkill {
		return nil, paraerr.Newf(paraerr.KindInternal, "render: %q is not a skill", in.Locator.String())
	}

	var fields []mdfile.Field
	for _, f := range []kindmeta.Field{kindmeta.FieldName, kindmeta.FieldDescription} {
		if v := in.State.Field(f); v != "" {
			fields = append(fields, mdfile.Field{Key: string(f), Value: mdfile.String(flatten(v))})
		}
	}

	path, err := Skill.Path(in)
	if err != nil {
		return nil, err
	}
	return mdfile.Render(fields, readmeBody(in, in.existing(path)))
}
