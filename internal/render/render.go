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
var agentsLocations = []string{
	"",
	"projects",
	"areas",
	"resources",
	"archive",
	"archive.projects",
	"archive.areas",
	"archive.resources",
}

// HasAgents reports whether loc is one of the eight places AGENTS.md and
// CLAUDE.md belong (§6).
func HasAgents(loc locator.Locator) bool {
	target := loc.String()
	for _, l := range agentsLocations {
		if l == target {
			return true
		}
	}
	return false
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
		if in.Config.EmitClaude {
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
		if HasAgents(in.Locator) && in.Config.EmitClaude {
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

// Artifacts renders every file the subject owns, in For's order. It is the
// seam rebuild writes through and doctor compares against (§10, §21.1).
func Artifacts(in In) ([]Artifact, error) {
	rs := For(in)
	out := make([]Artifact, 0, len(rs))
	for _, r := range rs {
		path, err := r.Path(in)
		if err != nil {
			return nil, err
		}
		data, err := r.Render(in)
		if err != nil {
			return nil, err
		}
		out = append(out, Artifact{Path: path, Bytes: data})
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
