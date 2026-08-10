// Package locator implements the locator ↔ path isomorphism (design §1.4,
// §1.5): dot-separated segments that address any entity in the tree, the
// reserved words that can never be an id, and the one exception to the
// locator↔path mapping — skills.<id> ↔ .agents/skills/para-<id>/.
package locator

import (
	"regexp"
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// Locator is a parsed locator: one string segment per dot-separated
// component, in order. It is a pure value — no filesystem, no clock.
type Locator []string

// ReservedWords cannot appear as an id anywhere in a locator (§1.4). The
// list grows from ten words to seventeen with R6: the seven singular nouns
// join the ten structural words, because without them an entity legitimately
// named e.g. "project" would make a grammar whose first token is a noun
// unable to tell the noun from an id sharing its spelling.
var ReservedWords = []string{
	"projects", "areas", "resources", "archive", "objectives",
	"key-results", "skills", "logs", ".para", ".agents",
	"project", "area", "resource", "objective", "key-result", "skill", "container",
}

// IsReserved reports whether s is one of the words §1.4 reserves and so can
// never be used as an id.
func IsReserved(s string) bool {
	return slices.Contains(ReservedWords, s)
}

var segmentPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// ValidSegment reports whether s is a legal locator segment: non-empty and
// drawn from [a-z0-9-] (§1.4).
//
// It is exported because a locator is a path (§1.4) and the implication runs
// both ways: a *directory name* that is not a legal segment names no locator,
// so kindmeta can derive no kind for it and doctor must report it as
// `misplaced` (§10). Parse enforces this on what you type; this is the same
// rule asked about what is on disk, and it is one function so that the two
// cannot come to different answers about the same name.
func ValidSegment(s string) bool { return segmentPattern.MatchString(s) }

// Parse splits s on "." into segments, validating that every segment is
// non-empty and drawn from the charset [a-z0-9-] (§1.4). It does not check
// reserved words or structural legality — a segment that happens to equal a
// reserved word may be a legitimate structural segment (e.g. "projects" in
// "projects.acme"); only kindmeta knows which positions are id positions.
func Parse(s string) (Locator, error) {
	if s == "" {
		return nil, paraerr.New(paraerr.KindValidation, "empty locator")
	}
	segs := strings.Split(s, ".")
	for _, seg := range segs {
		if seg == "" {
			return nil, paraerr.Newf(paraerr.KindValidation, "locator %q has an empty segment", s)
		}
		if !segmentPattern.MatchString(seg) {
			return nil, paraerr.Newf(paraerr.KindValidation, "locator %q has an illegal segment %q (must match [a-z0-9-]+)", s, seg)
		}
	}
	return Locator(segs), nil
}

// String renders the locator back to its canonical dotted form.
func (l Locator) String() string {
	return strings.Join(l, ".")
}

// Bucket returns the locator's first segment — the bucket or bucket-like
// prefix (projects, areas, resources, archive, skills) everything else
// hangs off of — or "" for an empty locator.
func (l Locator) Bucket() string {
	if len(l) == 0 {
		return ""
	}
	return l[0]
}

// IsArchived reports whether the locator names something under archive/
// (§1.6). Archival never changes a locator's shape otherwise — it only
// prepends this one segment.
func (l Locator) IsArchived() bool {
	return l.Bucket() == "archive"
}

// Path converts the locator to its relative filesystem path, applying the
// one exception §1.4 defines: a two-segment "skills.<id>" locator maps to
// ".agents/skills/para-<id>", the "para-" prefix belonging to the directory
// and never to the locator. Every other locator maps segment-for-segment to
// path components. The result uses "/" as the separator regardless of GOOS;
// callers join it onto an OS path with filepath.FromSlash.
func (l Locator) Path() (string, error) {
	if len(l) == 0 {
		return "", paraerr.New(paraerr.KindValidation, "empty locator has no path")
	}
	if l[0] == "skills" {
		if len(l) != 2 {
			return "", paraerr.Newf(paraerr.KindValidation, "locator %q: skills is one level only", l.String())
		}
		return ".agents/skills/para-" + l[1], nil
	}
	return strings.Join(l, "/"), nil
}

// FromPath is the inverse of Path: it converts a "/"-separated relative
// path into the locator it corresponds to, including the skills.<id>
// exception. Paths under ".agents" that are not a para-owned skill
// directory address no locator — that space is "yours" (§1.3) — and FromPath
// reports that with a KindValidation error rather than fabricating one.
func FromPath(relPath string) (Locator, error) {
	relPath = strings.Trim(relPath, "/")
	if relPath == "" {
		return nil, paraerr.New(paraerr.KindValidation, "empty path has no locator")
	}
	parts := strings.Split(relPath, "/")

	if parts[0] == ".agents" {
		if len(parts) == 3 && parts[1] == "skills" && strings.HasPrefix(parts[2], "para-") {
			id := strings.TrimPrefix(parts[2], "para-")
			if id == "" || !segmentPattern.MatchString(id) {
				return nil, paraerr.Newf(paraerr.KindValidation, "path %q has an illegal skill id %q", relPath, id)
			}
			return Locator{"skills", id}, nil
		}
		return nil, paraerr.Newf(paraerr.KindValidation, "%q is not a para-owned path under .agents", relPath)
	}

	for _, seg := range parts {
		if seg == "" {
			return nil, paraerr.Newf(paraerr.KindValidation, "path %q has an empty segment", relPath)
		}
		if !segmentPattern.MatchString(seg) {
			return nil, paraerr.Newf(paraerr.KindValidation, "path %q has an illegal segment %q", relPath, seg)
		}
	}
	return Locator(parts), nil
}
