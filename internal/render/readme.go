package render

import (
	"bytes"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// readmeRenderer renders README.md: generated frontmatter over a human-owned
// body (§2.1, §2.2). Every entity, container, bucket, and the root has one.
type readmeRenderer struct{}

// readmeFile is the one filename, at every level (§1.1).
const readmeFile = "README.md"

func (readmeRenderer) Path(in In) (string, error) {
	dir, err := in.dir()
	if err != nil {
		return "", err
	}
	return join(dir, readmeFile), nil
}

// Frontmatter keys README.md carries beyond §15's stored fields. Both are
// projections of the path, which is where the kind and the locator actually
// live (§1.3, §8.3): they are here because §8.4 makes README.md the place a
// browsing reader learns what they are looking at, and they are safe to render
// because a projection may repeat what truth states — it just may not be the
// only copy.
const (
	readmeKeyKind    = "kind"
	readmeKeyLocator = "locator"
)

// kindTree is the value the root's `kind` carries. The root is not an entity,
// it is the tree (§8.1), and no kindmeta.Kind spells that.
const kindTree = "tree"

func (readmeRenderer) Render(in In) ([]byte, error) {
	fields, err := readmeFrontmatter(in)
	if err != nil {
		return nil, err
	}
	path, err := Readme.Path(in)
	if err != nil {
		return nil, err
	}
	return mdfile.Render(fields, readmeBody(in, in.existing(path)))
}

func readmeFrontmatter(in In) ([]mdfile.Field, error) {
	if in.shape() == shapeRoot {
		// The root's frontmatter generates from tree.toml, not state.toml
		// (§2.2, §8.1). schema and para-version stay out: they are machinery
		// a reader of the README has no use for.
		fields := []mdfile.Field{{Key: readmeKeyKind, Value: mdfile.String(kindTree)}}
		return append(fields, statelikeFields(in.Tree.Name, in.Tree.Description, in.Tree.Created)...), nil
	}
	if in.Kind == kindmeta.KindUnknown {
		return nil, paraerr.Newf(paraerr.KindInternal, "render: no kind for %q", in.Locator.String())
	}
	locatorValue, err := readmeLocatorValue(in.Locator)
	if err != nil {
		return nil, err
	}

	fields := []mdfile.Field{
		{Key: readmeKeyKind, Value: mdfile.String(in.Kind.String())},
		{Key: readmeKeyLocator, Value: mdfile.String(locatorValue)},
	}
	// §15's row order, filtered to the fields this kind has and this state
	// carries. Derived values — attention, progress, pace, key-result status —
	// are deliberately absent: README.md is rewritten on mutation, not on
	// every tick of the clock, so a stored copy of a clock-dependent value
	// here would rot exactly the way §2.5 forbids.
	for _, f := range kindmeta.AllFields() {
		if !kindmeta.Has(in.Kind, f) {
			continue
		}
		switch f {
		case kindmeta.FieldTags, kindmeta.FieldScope:
			if list := in.State.List(f); len(list) > 0 {
				fields = append(fields, mdfile.Field{Key: string(f), Value: mdfile.StringArray(list)})
			}
		default:
			if v := in.State.Field(f); v != "" {
				fields = append(fields, mdfile.Field{Key: string(f), Value: mdfile.String(v)})
			}
		}
	}
	return fields, nil
}

// readmeLocatorValue is the dotted form (R1, R24) of loc, with one
// exception: address.IsArchiveRoot's — the archive root is a real container
// with its own generated README (§1.1's tree diagram) but has no address of
// its own to convert to (§1.6/R7), so it keeps its bucket word verbatim; the
// word is spelled the same in both the old and new forms, so this is not a
// place R22 finds the old form surviving.
func readmeLocatorValue(loc locator.Locator) (string, error) {
	if address.IsArchiveRoot(loc) {
		return "archive", nil
	}
	addr, err := address.FromLocator(loc)
	if err != nil {
		return "", err
	}
	return addr.String(), nil
}

// statelikeFields renders the root's three identity fields in §15's order, so
// the root's frontmatter reads the same as everything else's.
func statelikeFields(name, description, created string) []mdfile.Field {
	var fields []mdfile.Field
	for _, kv := range []struct {
		key string
		val string
	}{
		{string(kindmeta.FieldName), name},
		{string(kindmeta.FieldDescription), description},
		{string(kindmeta.FieldCreated), created},
	} {
		if kv.val != "" {
			fields = append(fields, mdfile.Field{Key: kv.key, Value: mdfile.String(kv.val)})
		}
	}
	return fields
}

// readmeBody returns the body to write under the frontmatter: the existing
// body, byte for byte, whenever there is one (§2.1 — humans own bodies), and a
// stub otherwise, for the README add creates (§18.1).
//
// A file that exists but has no frontmatter is treated as body from its first
// byte. That is the shape `remove --keep-files` leaves behind (§18.4: strip the
// frontmatter block, keep the body), so adding an entity back over one of those
// directories re-adopts the prose instead of burying it.
func readmeBody(in In, existing []byte) []byte {
	if len(existing) == 0 {
		return stubBody(in)
	}
	_, body, err := mdfile.Split(existing)
	if err != nil {
		return existing
	}
	return body
}

func stubBody(in In) []byte {
	name, description := in.State.Name, in.State.Description
	switch in.shape() {
	case shapeRoot:
		name, description = in.Tree.Name, in.Tree.Description
	case shapeSkill:
		// A skill's description is a when-to-use clause — "when asked for the
		// weekly signups number" (§5.1) — which is the right thing in
		// frontmatter and reads as a fragment when set as prose. So a skill's
		// stub body is its heading alone; the instructions beneath it are the
		// author's to write.
		description = ""
	}

	var b bytes.Buffer
	b.WriteString("\n")
	if name != "" {
		b.WriteString("# ")
		b.WriteString(flatten(name))
		b.WriteString("\n")
	}
	if description != "" {
		b.WriteString("\n")
		b.WriteString(flatten(description))
		b.WriteString("\n")
	}
	return b.Bytes()
}
