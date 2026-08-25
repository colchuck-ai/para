package config

import (
	"path/filepath"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptoml"
	"github.com/colchuck-ai/para/internal/render"
)

// RootLabel is how the tree root prints in a chain. The root has no locator
// of its own — it is not an entity, it is the tree (§8.1) — so it needs a
// name that cannot collide with one.
const RootLabel = "<root>"

// Level is one link of a resolution chain: a place that could have supplied
// the value, whether it did, and the file that would say so.
type Level struct {
	// Locator names the level. The empty locator is the tree root.
	Locator locator.Locator
	// File is the level's config.toml, relative to the tree root and
	// slash-separated — the form §16.1 prints when it says where a
	// threshold came from.
	File string
	// Value is what this level sets the key to, valid only when Set.
	Value ptoml.Value
	// Set reports whether this level supplies the key at all.
	Set bool
}

// Label is how the level prints in §22's chain: its dotted address (R24), or
// <root>.
func (l Level) Label() string {
	if len(l.Locator) == 0 {
		return RootLabel
	}
	s, err := address.String(l.Locator)
	if err != nil {
		return l.Locator.String()
	}
	return s
}

// Resolution is a resolved key: the answer, and every level consulted to
// reach it.
//
// The chain is returned rather than discarded because §7 makes it the
// condition of chained resolution being defensible — "the ability to print
// the chain and the winning value is not optional". `config show` prints it
// whole (§22); `show` prints just the winner's file (§16.1).
type Resolution struct {
	// Key is the key asked for.
	Key string
	// Spec is its row of §7's table.
	Spec Spec
	// Value is the resolved value, valid only when Found.
	Value ptoml.Value
	// Found reports whether anything supplied a value — a level, or the
	// built-in default. An unset key with no default finds nothing, which
	// §7 defines as the check never firing.
	Found bool
	// FromDefault reports that the value came from Spec's built-in default
	// rather than from any level.
	FromDefault bool
	// Winner indexes Levels at the level that supplied the value, or -1.
	Winner int
	// Levels is the whole chain, nearest first.
	Levels []Level
}

// Source returns the level that supplied the value, and false when the
// built-in default did or nothing did.
func (r Resolution) Source() (Level, bool) {
	if r.Winner < 0 || r.Winner >= len(r.Levels) {
		return Level{}, false
	}
	return r.Levels[r.Winner], true
}

// Int returns the resolved value as a whole number.
func (r Resolution) Int() (int64, bool) {
	if !r.Found || r.Value.Kind != ptoml.KindInt64 {
		return 0, false
	}
	return r.Value.Int, true
}

// Float returns the resolved value as a number, widening a whole one.
func (r Resolution) Float() (float64, bool) {
	if !r.Found {
		return 0, false
	}
	return Float(r.Value)
}

// Bool returns the resolved value as a boolean.
func (r Resolution) Bool() (bool, bool) {
	if !r.Found || r.Value.Kind != ptoml.KindBool {
		return false, false
	}
	return r.Value.Bool, true
}

// Str returns the resolved value as a string.
func (r Resolution) Str() (string, bool) {
	if !r.Found || r.Value.Kind != ptoml.KindString {
		return "", false
	}
	return r.Value.Str, true
}

// Chain is the full ancestor chain for loc, nearest first and ending at the
// root (§7). It is a pure function of the locator: which levels exist on
// disk is a separate question, and a level with no config.toml simply sets
// nothing.
//
// An individual skill's chain is its own config.toml and then the root, with
// nothing in between, because .agents/ is not in the PARA tree (§7, §8.1) —
// there is no intermediate `skills` level to consult even though the
// locator has a `skills` segment. The skill bucket itself, named directly,
// is a level in its own right now (para-a3p): `.agents/skills/` is a
// container with its own state.toml, the same way projects/areas/resources
// are, even though it carries no config.toml of its own — a level with no
// file simply sets nothing, the same as any other unset level.
func Chain(loc locator.Locator) ([]Level, error) {
	var levels []Level

	if loc.Bucket() == "skills" && len(loc) > 1 {
		rel, err := loc.Path()
		if err != nil {
			return nil, err
		}
		levels = append(levels, Level{Locator: loc, File: configFile(rel)})
		return append(levels, Level{File: configFile("")}), nil
	}

	for i := len(loc); i > 0; i-- {
		cur := loc[:i]
		rel, err := cur.Path()
		if err != nil {
			return nil, err
		}
		levels = append(levels, Level{Locator: cur, File: configFile(rel)})
	}
	return append(levels, Level{File: configFile("")}), nil
}

// configFile is the root-relative, slash-separated path of the config.toml
// belonging to the directory at rel. The root's own rel is "".
func configFile(rel string) string {
	if rel == "" {
		return ".para/config.toml"
	}
	return rel + "/.para/config.toml"
}

// Resolver resolves keys against one tree, caching each config.toml it
// reads. One resolver per command: a read command resolves the same
// thresholds for every entity it lists, and every one of those chains ends
// at the same root.
type Resolver struct {
	root  string
	cache map[string]File
}

// NewResolver returns a resolver reading from the tree rooted at root.
func NewResolver(root string) *Resolver {
	return &Resolver{root: root, cache: make(map[string]File)}
}

// Resolve walks loc's ancestor chain for key, nearest first, and returns the
// first value found along with every level consulted (§7).
func (r *Resolver) Resolve(loc locator.Locator, key string) (Resolution, error) {
	spec, ok := Lookup(key)
	if !ok {
		return Resolution{}, paraerr.Newf(paraerr.KindValidation, "unknown config key %q", key)
	}

	levels, err := Chain(loc)
	if err != nil {
		return Resolution{}, err
	}

	res := Resolution{Key: key, Spec: spec, Winner: -1, Levels: levels}
	for i := range res.Levels {
		l := &res.Levels[i]
		f, err := r.file(l.File)
		if err != nil {
			return Resolution{}, err
		}
		v, ok := f.Get(key)
		if !ok {
			continue
		}
		if err := spec.Check(v); err != nil {
			return Resolution{}, paraerr.Wrap(paraerr.KindValidation, err,
				"in "+filepath.Join(r.root, filepath.FromSlash(l.File)))
		}
		l.Value, l.Set = v, true
		if !res.Found {
			res.Value, res.Found, res.Winner = v, true, i
		}
	}

	if !res.Found {
		if v, ok := spec.DefaultValue(); ok {
			res.Value, res.Found, res.FromDefault = v, true, true
		}
	}
	return res, nil
}

// RenderConfig resolves the keys a renderer reads (§6.1, §9) into the value
// render takes. Renderers never resolve anything themselves — a renderer
// that did would be reading truth it was not given — so this is the seam
// between §7's chain and the projection engine.
//
// The five keys config.RootOnly names are resolved **at the root** whatever loc
// is, and they are the only keys in the design that are. §6.1 opens by saying the whole
// subsection is "off by default and turns on together, because it is one
// concern", and §7's table gives both keys `root` as where they live. The reason
// it has to be enforced rather than merely advised is that the surface has two
// halves in different places: eight `CLAUDE.md` files, one per location, and one
// `.claude/skills/` at the root. Resolved per location, `config set --at
// projects emit.claude true` would write `projects/CLAUDE.md` and no mirror —
// half a surface, which `doctor` would then call clean because it would be
// asking the same split question.
func (r *Resolver) RenderConfig(loc locator.Locator) (render.Config, error) {
	out := render.DefaultConfig()

	claude, err := r.Resolve(nil, KeyEmitClaude)
	if err != nil {
		return render.Config{}, err
	}
	if v, ok := claude.Bool(); ok {
		out.EmitClaude = v
	}

	skills, err := r.Resolve(nil, KeyEmitClaudeSkills)
	if err != nil {
		return render.Config{}, err
	}
	if v, ok := skills.Str(); ok {
		out.EmitClaudeSkills = v
	}

	cursor, err := r.Resolve(nil, KeyEmitCursor)
	if err != nil {
		return render.Config{}, err
	}
	if v, ok := cursor.Bool(); ok {
		out.EmitCursor = v
	}

	cursorSkills, err := r.Resolve(nil, KeyEmitCursorSkills)
	if err != nil {
		return render.Config{}, err
	}
	if v, ok := cursorSkills.Str(); ok {
		out.EmitCursorSkills = v
	}

	// At the root, whatever loc is, for the reason RootOnly gives: .gitattributes
	// sits at the root and nowhere else (§9), so a value set at any other level
	// is read by nothing. Resolving it chained would let `config list projects`
	// report a value that decides no byte of any file.
	gitattributes, err := r.Resolve(nil, KeyEmitGitattributes)
	if err != nil {
		return render.Config{}, err
	}
	if v, ok := gitattributes.Bool(); ok {
		out.EmitGitattributes = v
	}

	return out, nil
}

// RotateBytes resolves log.rotate-bytes for loc (§3.4), the one config key
// the write path reads on every mutation.
func (r *Resolver) RotateBytes(loc locator.Locator) (int64, error) {
	res, err := r.Resolve(loc, "log.rotate-bytes")
	if err != nil {
		return 0, err
	}
	n, _ := res.Int()
	return n, nil
}

// file loads the config.toml at the root-relative path rel, through the
// cache.
func (r *Resolver) file(rel string) (File, error) {
	if f, ok := r.cache[rel]; ok {
		return f, nil
	}
	f, err := Read(filepath.Join(r.root, filepath.FromSlash(rel)))
	if err != nil {
		return File{}, err
	}
	r.cache[rel] = f
	return f, nil
}

// Override seeds the cache for rel with data, so every subsequent Resolve or
// RenderConfig against rel answers as if it already held data, without
// reading disk.
//
// It exists for a mutation that has not written rel's new bytes yet and needs
// an answer as if it had — `config set`'s own dry run, whose rehearsal must
// ask the surface refresh the identical question a real run's post-write
// resolver would (para-ato). A real run could call it too — the bytes are the
// same ones about to land on disk — but it is written for the case where
// nothing has landed at all.
func (r *Resolver) Override(rel string, data []byte) error {
	f, err := Decode(data)
	if err != nil {
		return err
	}
	r.cache[rel] = f
	return nil
}
