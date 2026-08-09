package mdfile

import (
	"bytes"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// BeginMarker and EndMarker delimit the para-owned block in AGENTS.md (§6):
// para writes what is between them and never touches a byte outside them.
const (
	BeginMarker = "<!-- para:begin — generated, do not edit; run `para rebuild` -->"
	EndMarker   = "<!-- para:end -->"
)

// Markers is a delimited-block marker pair. AGENTS.md uses Markdown's comment
// syntax (MarkdownMarkers), but the same block mechanism serves any file whose
// human-owned part must survive a rewrite — .gitattributes, for instance,
// where a `<!-- -->` line would not be a comment (§2.2, §9). The comment
// syntax is the caller's to choose; the mechanism is not.
type Markers struct {
	Begin string
	End   string
}

// MarkdownMarkers is §6's marker pair, for AGENTS.md and any other Markdown
// file para shares with a human.
var MarkdownMarkers = Markers{Begin: BeginMarker, End: EndMarker}

// HashMarkers is the same pair spelled as `#` comments, for files whose
// comment syntax is a hash — .gitattributes (§9).
var HashMarkers = Markers{
	Begin: "# para:begin — generated, do not edit; run `para rebuild`",
	End:   "# para:end",
}

// ReplaceBlock rewrites the para-owned block in data to block, leaving
// every byte before BeginMarker and after EndMarker untouched (§6). A nil
// or empty data produces a fresh file containing only the block, for a new
// AGENTS.md that doesn't exist yet.
func ReplaceBlock(data []byte, block []byte) ([]byte, error) {
	return ReplaceDelimited(MarkdownMarkers, data, block)
}

// ReplaceDelimited is ReplaceBlock for an arbitrary marker pair.
func ReplaceDelimited(m Markers, data []byte, block []byte) ([]byte, error) {
	block = normalizeBlock(block)

	if len(data) == 0 {
		var b bytes.Buffer
		b.WriteString(m.Begin)
		b.WriteString("\n")
		b.Write(block)
		b.WriteString(m.End)
		b.WriteString("\n")
		return b.Bytes(), nil
	}

	prefix, _, suffix, err := splitBlock(m, data)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(prefix)
	b.WriteString(m.Begin)
	b.WriteString("\n")
	b.Write(block)
	b.WriteString(m.End)
	b.Write(suffix)
	return b.Bytes(), nil
}

// AppendDelimited rewrites data's para-owned block to block, or — when there is
// no block — puts one at the end and leaves every existing byte where it was.
//
// It is ReplaceDelimited for the one file §2.2 calls "append-only to an
// existing file": `.gitattributes` belongs to the repository, and para's block
// is something it gains rather than something it is (§9). ReplaceDelimited
// refuses a file with no markers, which is right for AGENTS.md — para wrote that
// file, so markers missing from it means markers edited out, and guessing where
// they were would eat the prose §6 promises to leave alone. It is wrong here:
// almost every repository para is run in already has a .gitattributes, and
// refusing to write into one would make `para init` fail on it.
//
// A file with no final newline gains one, because a marker cannot share a line
// with somebody else's rule. That is the one byte the RemoveDelimited round trip
// does not give back. (The round trip is also lossy for two shapes para cannot
// produce and does not accept: a marker sitting mid-line, and a block whose own
// content contains the end marker. Marker matching is a substring search rather
// than a line-anchored one, which is what makes both possible to write by hand
// and impossible to write with para.)
func AppendDelimited(m Markers, data []byte, block []byte) ([]byte, error) {
	if len(data) == 0 || bytes.Contains(data, []byte(m.Begin)) {
		return ReplaceDelimited(m, data, block)
	}

	var b bytes.Buffer
	b.Write(data)
	if !bytes.HasSuffix(data, []byte("\n")) {
		b.WriteString("\n")
	}
	b.WriteString(m.Begin)
	b.WriteString("\n")
	b.Write(normalizeBlock(block))
	b.WriteString(m.End)
	b.WriteString("\n")
	return b.Bytes(), nil
}

// RemoveDelimited returns data with the para-owned block taken out and every
// other byte left exactly where it was, and reports whether there was a block
// to take.
//
// It is what turning a delimited block's config key off means. The block is
// removable rather than the *file* being removable, because para owns what is
// between its markers and the rest of the lines are the repository's (§2.2,
// §9) — so a `.gitattributes` that held nothing but para's block comes back as
// an empty file, which is precisely the file para was handed.
//
// A file with no begin marker at all is not damage — para simply has no block in
// it — and comes back unchanged with found false. A file whose begin marker has
// lost its end marker *is* damage, and is an error rather than a third quiet
// answer: guessing where the block ended would take lines para never wrote, and
// reporting "no block to remove" would leave the markers and the rules sitting
// in a file para had just been asked to take them out of, while saying it had.
//
// That is the same answer ReplaceDelimited and AppendDelimited give the same
// bytes. The three must agree: they are one rule about what a damaged block
// means, and a version of this function that shrugged at damage would make
// `emit.gitattributes = false` a command that succeeds and changes nothing.
func RemoveDelimited(m Markers, data []byte) ([]byte, bool, error) {
	if !bytes.Contains(data, []byte(m.Begin)) {
		return data, false, nil
	}
	prefix, _, suffix, err := splitBlock(m, data)
	if err != nil {
		return nil, false, err
	}
	// The newline after the end marker closes the block's last line, so it goes
	// with the block. Without this, removing a block from the middle of a file
	// would leave a blank line where it had been.
	suffix = bytes.TrimPrefix(suffix, []byte("\n"))

	out := make([]byte, 0, len(prefix)+len(suffix))
	out = append(out, prefix...)
	out = append(out, suffix...)
	return out, true, nil
}

// ExtractBlock returns the current content of data's para-owned block
// (between the markers, exclusive), for a rebuild that wants to compare
// what's on disk against what it would regenerate.
func ExtractBlock(data []byte) ([]byte, error) {
	return ExtractDelimited(MarkdownMarkers, data)
}

// ExtractDelimited is ExtractBlock for an arbitrary marker pair.
func ExtractDelimited(m Markers, data []byte) ([]byte, error) {
	_, content, _, err := splitBlock(m, data)
	return content, err
}

// splitBlock locates the para-owned block in data, returning everything
// before m.Begin, the content between the markers, and everything
// after m.End — each preserved byte for byte.
func splitBlock(m Markers, data []byte) (prefix, content, suffix []byte, err error) {
	idxBegin := bytes.Index(data, []byte(m.Begin))
	if idxBegin == -1 {
		return nil, nil, nil, paraerr.New(paraerr.KindValidation, "missing para-owned block: no begin marker found")
	}
	afterBegin := data[idxBegin+len(m.Begin):]
	if !bytes.HasPrefix(afterBegin, []byte("\n")) {
		return nil, nil, nil, paraerr.New(paraerr.KindValidation, "malformed para-owned block: begin marker is not on its own line")
	}
	afterBeginContent := afterBegin[1:]

	idxEnd := bytes.Index(afterBeginContent, []byte(m.End))
	if idxEnd == -1 {
		return nil, nil, nil, paraerr.New(paraerr.KindValidation, "missing para-owned block: no end marker found")
	}

	prefix = data[:idxBegin]
	content = afterBeginContent[:idxEnd]
	suffix = afterBeginContent[idxEnd+len(m.End):]
	return prefix, content, suffix, nil
}

// normalizeBlock ensures a non-empty block ends with a newline, so callers
// never have to remember the trailing-newline convention themselves.
func normalizeBlock(block []byte) []byte {
	if len(block) == 0 || bytes.HasSuffix(block, []byte("\n")) {
		return block
	}
	out := make([]byte, len(block)+1)
	copy(out, block)
	out[len(block)] = '\n'
	return out
}
