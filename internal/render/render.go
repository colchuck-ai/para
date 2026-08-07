// Package render is the projection engine: every generated file in the tree
// (design §2.2) is produced here, by a pure function from truth to bytes.
//
// Purity is not a stylistic preference, it is what makes doctor's
// stale-projection finding possible. That finding is full fidelity (§10): it
// re-derives every generated file in memory and compares it byte for byte
// against what is on disk. A renderer that read the clock, the filesystem, or
// a map in iteration order would report drift on a clean tree. So:
//
//   - no renderer takes a Clock or touches the filesystem — anything
//     time-dependent arrives in In;
//   - no renderer iterates a map to produce output (§0.2); every field order
//     is declared;
//   - the human-owned parts of the two partly-generated files arrive in
//     In.Existing and are copied through byte for byte (§2.1).
//
// ACTIVITY.md is the one renderer with two modes. On the write path it
// re-derives only the affected day's section and copies every prior day
// through unchanged (§3.5); on the rebuild and doctor paths it re-derives the
// whole file from every journal file backing it. Both must produce identical
// bytes for the same history — that equivalence is what makes §10's dated
// drift report meaningful, and it is this package's central property.
package render

import (
	"slices"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/truth"
)

// Config is the resolved configuration a renderer may read. Resolution walks
// the ancestor chain and is §7's job (Phase 7); a renderer only ever sees the
// answers, never the chain, because a renderer that resolved anything itself
// would be reading truth it was not given.
type Config struct {
	// EmitClaude turns on the Claude Code compatibility surface: CLAUDE.md
	// and the .claude/skills mirror (§6.1). Off by default.
	EmitClaude bool
	// EmitClaudeSkills is "symlink" (default) or "copy" (§6.1). The mirror
	// itself is Phase 13's; this carries the setting through.
	EmitClaudeSkills string
	// EmitGitattributes writes the .gitattributes block (§9). On by default.
	EmitGitattributes bool
}

// Mirror modes for EmitClaudeSkills (§6.1).
const (
	MirrorSymlink = "symlink"
	MirrorCopy    = "copy"
)

// DefaultConfig is §7's defaults for the keys render reads: the Claude surface
// off, .gitattributes on, and symlink as the mirror mode.
func DefaultConfig() Config {
	return Config{EmitClaude: false, EmitClaudeSkills: MirrorSymlink, EmitGitattributes: true}
}

// In is everything a renderer reads about one subject — a root, a bucket, a
// container, an entity, or a skill. It is a value: no clock, no filesystem.
type In struct {
	// Locator names the subject. The empty locator is the tree root, which
	// has no locator of its own (§8.1).
	Locator locator.Locator

	// Kind is the subject's kind, as derived from its locator (§1.3). It is
	// carried rather than re-derived so that a container — which KindOf never
	// returns, because no locator addresses a container's id — can be
	// rendered at all.
	Kind kindmeta.Kind

	// State is the subject's .para/state.toml (§8.2, §8.3). Unused for the
	// root, whose identity lives in Tree instead.
	State truth.State

	// Tree is .para/tree.toml, read only when Locator is empty (§8.1).
	Tree truth.Tree

	// Events are the subject's own journal events, ordered by At (§3.1).
	//
	// In full mode this is the entity's whole history. In ACTIVITY.md's
	// incremental mode it need only cover the days being re-derived — with
	// one exception: a key-result's measurement baseline defaults to the
	// first reading ever logged (§4.1), so a key-result's incremental render
	// needs its full measurement history. That costs nothing extra, because
	// rewriting MEASUREMENTS.csv in the same mutation already requires it
	// (§2.3).
	Events []journal.Event

	// Config is the resolved configuration (§7).
	Config Config

	// Rules are the basenames of the derived rule files in .agents/rules/,
	// for CLAUDE.md's import list (§6.1). Sorted by the renderer, so the
	// caller's order never reaches the output.
	Rules []string

	// Existing holds the current bytes of files whose human-owned parts must
	// survive a rewrite, keyed by the same root-relative path Artifacts
	// reports: a README.md's body, an AGENTS.md's prose outside the markers,
	// .gitattributes' other lines, and ACTIVITY.md's prior days (§2.1, §2.2,
	// §3.5). A missing entry means the file does not exist yet.
	Existing map[string][]byte
}

// Artifact is one generated file: where it belongs, relative to the tree root
// and slash-separated, and the bytes that belong there.
type Artifact struct {
	Path  string
	Bytes []byte
}

// Renderer renders one generated file from truth. Implementations are pure and
// deterministic — see the package doc for why that is load-bearing rather than
// tidy.
type Renderer interface {
	// Path is the artifact's location relative to the tree root,
	// slash-separated regardless of GOOS.
	Path(in In) (string, error)
	// Render is the bytes that belong at Path.
	Render(in In) ([]byte, error)
}

// The renderers, one per row of §2.2's table of generated files.
var (
	Readme        Renderer = readmeRenderer{}
	Activity      Renderer = ActivityRenderer{}
	Measurements  Renderer = measurementsRenderer{}
	Skill         Renderer = skillRenderer{}
	Rule          Renderer = ruleRenderer{}
	Agents        Renderer = agentsRenderer{}
	Claude        Renderer = claudeRenderer{}
	GitAttributes Renderer = gitAttributesRenderer{}
)

// shape is the subject's structural role, which decides which files it owns.
// It is not kindmeta.Kind: root and bucket are shapes with no kind, and every
// entity kind but skill owns the same file set.
type shape int

const (
	shapeRoot shape = iota
	shapeContainer
	shapeEntity
	shapeSkill
)

func (in In) shape() shape {
	switch {
	case len(in.Locator) == 0:
		return shapeRoot
	case in.Kind == kindmeta.KindSkill:
		return shapeSkill
	case in.Kind == kindmeta.KindContainer:
		return shapeContainer
	default:
		return shapeEntity
	}
}

// agentsLocations is the exact set of places AGENTS.md and CLAUDE.md are
// emitted: the root, the four buckets, and archive/{projects,areas,resources}
// (§6, and §26's config example, which counts eight CLAUDE.md files). Nowhere
// else — not on entities, not on objectives/, not on key-results/ — because
// AGENTS.md orients an agent to the framework, and those eight places cover
// every concept there is.
//
// It is a fixed set rather than a walk, and that is what lets a skill mutation
// keep every CLAUDE.md correct without violating §2.3: §6.1 wants the import
// list to regenerate "with no separate bookkeeping", §2.3 forbids a mutation
// walking to root, and eight known locations are neither.
var agentsLocations = []locator.Locator{
	nil,
	{"projects"},
	{"areas"},
	{"resources"},
	{"archive"},
	{"archive", "projects"},
	{"archive", "areas"},
	{"archive", "resources"},
}

// AgentsLocations returns those eight, in the order they are written and
// reported. The root is the empty locator.
//
// Each locator is cloned, not just the slice holding them: a Locator is itself a
// slice, so copying the outer one would hand every caller the same backing
// arrays and a caller that wrote through one would rewrite the package's own
// table.
func AgentsLocations() []locator.Locator {
	out := make([]locator.Locator, 0, len(agentsLocations))
	for _, l := range agentsLocations {
		out = append(out, slices.Clone(l))
	}
	return out
}

// HasAgents reports whether loc is one of the eight places AGENTS.md and
// CLAUDE.md belong (§6).
func HasAgents(loc locator.Locator) bool {
	target := loc.String()
	for _, l := range agentsLocations {
		if l.String() == target {
			return true
		}
	}
	return false
}

// HasClaude reports whether CLAUDE.md is emitted for loc: one of the eight
// AGENTS.md locations, with the Claude Code surface turned on there (§6, §6.1).
//
// It is one predicate rather than an `&&` at each site because three places ask
// it and they must not disagree: For, which renders the file; rebuild, which
// removes it when the answer turns false; and doctor, which reports either as
// drift.
func HasClaude(loc locator.Locator, cfg Config) bool {
	return cfg.EmitClaude && HasAgents(loc)
}

// For returns the renderers the subject in owns, in the order a mutation
// writes them. Order matters twice: within a write it is the projection order
// §0.2 fixes, and within doctor it is the order findings are reported in.
func For(in In) []Renderer {
	var rs []Renderer
	switch in.shape() {
	case shapeSkill:
		// A skill's identity file is SKILL.md rather than README.md — §2.2 gives
		// it its own row for that reason — but it is otherwise an entity like
		// any other, so it gets an ACTIVITY.md too.
		//
		// §5.1's file listing omits one, and that omission is not evidence: the
		// same listing is arguing why a skill cannot be generated from a TOML
		// string, and its "everything except SKILL.md's frontmatter is yours"
		// already excludes the .para/ sitting right beside it. What decides it
		// is that §5.1 calls a skill "a real entity — it has state, config, and
		// a journal", §3.6 defines its attention as its newest note, and
		// `review --skills` measures review.cadence against exactly that. A
		// skill's staleness is a feature, so the file that answers "when did I
		// last touch this" should exist. Leaving it out would also buy rebuild
		// and doctor a per-kind exception, which is the thing §8.4's uniform
		// filenames exist to avoid.
		//
		// Its third artifact lives outside its directory — the derived rule
		// (§5.3).
		rs = append(rs, Skill, Activity, Rule)
	case shapeRoot:
		rs = append(rs, Readme, Agents, Activity)
		if HasClaude(in.Locator, in.Config) {
			rs = append(rs, Claude)
		}
		if in.Config.EmitGitattributes {
			rs = append(rs, GitAttributes)
		}
	case shapeContainer:
		rs = append(rs, Readme)
		if HasAgents(in.Locator) {
			rs = append(rs, Agents)
		}
		rs = append(rs, Activity)
		if HasClaude(in.Locator, in.Config) {
			rs = append(rs, Claude)
		}
	case shapeEntity:
		rs = append(rs, Readme, Activity)
		// MEASUREMENTS.csv exists only where there is something to
		// measure: it lives at the key-result and nowhere else (§4.4), and
		// it appears on the first reading rather than at creation, which is
		// why §18.1's add file set does not list it.
		if in.Kind == kindmeta.KindKeyResult && hasMeasurement(in.Events) {
			rs = append(rs, Measurements)
		}
	}
	return rs
}

// Artifacts renders every file the subject owns, in For's order, from the
// In it is given and nothing else.
func Artifacts(in In) ([]Artifact, error) {
	return Collect(in, For(in), nil)
}

// Collect renders the given renderers in order, reading each one's current
// bytes through read before any of them renders.
//
// The two phases are the point, and they are why this is one function rather
// than a loop in each caller. Two renderers copy a human-owned part of the file
// already on disk (§2.1) and one copies every prior day of it (§3.5), so every
// path has to be read *before* rendering starts — a caller that interleaved the
// two would render the first file against an Existing map that did not yet hold
// the last one. Both the write path and rebuild need exactly that order, and it
// is the kind of ordering that goes wrong silently: the output is right until
// the day two renderers on one subject read each other's files.
//
// read is given each artifact's root-relative path and returns its current
// bytes, or nil where the file does not exist. A nil read means the subject is
// rendered from In alone — which is only correct where In.Existing is already
// populated or where none of the renderers has a human-owned part to preserve.
func Collect(in In, renderers []Renderer, read func(path string) ([]byte, error)) ([]Artifact, error) {
	// in is a copy, so filling the map here cannot reach back into the
	// caller's — but a caller that supplied one deserves to keep it.
	if in.Existing == nil {
		in.Existing = map[string][]byte{}
	}

	paths := make([]string, len(renderers))
	for i, r := range renderers {
		path, err := r.Path(in)
		if err != nil {
			return nil, err
		}
		paths[i] = path
		if read == nil {
			continue
		}
		data, err := read(path)
		if err != nil {
			return nil, err
		}
		if data != nil {
			in.Existing[path] = data
		}
	}

	out := make([]Artifact, 0, len(renderers))
	for i, r := range renderers {
		data, err := r.Render(in)
		if err != nil {
			return nil, err
		}
		out = append(out, Artifact{Path: paths[i], Bytes: data})
	}
	return out, nil
}

// dir is the subject's directory relative to the tree root, slash-separated.
// The root's is "".
func (in In) dir() (string, error) {
	if len(in.Locator) == 0 {
		return "", nil
	}
	return in.Locator.Path()
}

// join places name inside dir, using the slash-separated, root-relative form
// every Artifact.Path carries.
func join(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// existing returns the current bytes of the file at path, or nil.
func (in In) existing(path string) []byte {
	return in.Existing[path]
}

func hasMeasurement(events []journal.Event) bool {
	for _, e := range events {
		if e.Kind == journal.KindMeasurement {
			return true
		}
	}
	return false
}
