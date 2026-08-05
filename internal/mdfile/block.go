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
