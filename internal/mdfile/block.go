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

// ReplaceBlock rewrites the para-owned block in data to block, leaving
// every byte before BeginMarker and after EndMarker untouched (§6). A nil
// or empty data produces a fresh file containing only the block, for a new
// AGENTS.md that doesn't exist yet.
func ReplaceBlock(data []byte, block []byte) ([]byte, error) {
	block = normalizeBlock(block)

	if len(data) == 0 {
		var b bytes.Buffer
		b.WriteString(BeginMarker)
		b.WriteString("\n")
		b.Write(block)
		b.WriteString(EndMarker)
		b.WriteString("\n")
		return b.Bytes(), nil
	}

	prefix, _, suffix, err := splitBlock(data)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(prefix)
	b.WriteString(BeginMarker)
	b.WriteString("\n")
	b.Write(block)
	b.WriteString(EndMarker)
	b.Write(suffix)
	return b.Bytes(), nil
}

// ExtractBlock returns the current content of data's para-owned block
// (between the markers, exclusive), for a rebuild that wants to compare
// what's on disk against what it would regenerate.
func ExtractBlock(data []byte) ([]byte, error) {
	_, content, _, err := splitBlock(data)
	return content, err
}

// splitBlock locates the para-owned block in data, returning everything
// before BeginMarker, the content between the markers, and everything
// after EndMarker — each preserved byte for byte.
func splitBlock(data []byte) (prefix, content, suffix []byte, err error) {
	idxBegin := bytes.Index(data, []byte(BeginMarker))
	if idxBegin == -1 {
		return nil, nil, nil, paraerr.New(paraerr.KindValidation, "missing para-owned block: no begin marker found")
	}
	afterBegin := data[idxBegin+len(BeginMarker):]
	if !bytes.HasPrefix(afterBegin, []byte("\n")) {
		return nil, nil, nil, paraerr.New(paraerr.KindValidation, "malformed para-owned block: begin marker is not on its own line")
	}
	afterBeginContent := afterBegin[1:]

	idxEnd := bytes.Index(afterBeginContent, []byte(EndMarker))
	if idxEnd == -1 {
		return nil, nil, nil, paraerr.New(paraerr.KindValidation, "missing para-owned block: no end marker found")
	}

	prefix = data[:idxBegin]
	content = afterBeginContent[:idxEnd]
	suffix = afterBeginContent[idxEnd+len(EndMarker):]
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
