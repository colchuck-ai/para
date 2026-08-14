package render

import (
	"fmt"

	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// gitAttributesRenderer renders the one thing para writes for git's benefit
// (§9). Para never invokes git; this is a text file like any other.
//
// It is written as a delimited block rather than as the whole file, which is how
// §2.2's "append-only to an existing file" is honoured: a repository that
// already has a .gitattributes keeps every line of it, and para owns only what
// is between its markers. The markers are `#` comments, because a `<!-- -->`
// line would not be a comment here.
type gitAttributesRenderer struct{}

const gitAttributesFile = ".gitattributes"

func (gitAttributesRenderer) Path(in In) (string, error) {
	if len(in.Locator) != 0 {
		return "", paraerr.Newf(paraerr.KindInternal,
			"render: %s is emitted at the root only, not at %q", gitAttributesFile, in.Locator.String())
	}
	return gitAttributesFile, nil
}

// gitAttributesBlock is §9's block, verbatim. Both of its rules are definitions
// rather than heuristics, which is what makes them safe to write on a user's
// behalf: union merge is what merging two append streams whose order lives in
// the data *means*, and `merge=ours` on a wholly generated file is correct
// because whichever side wins, a rebuild produces the right bytes.
//
// Symlinks get no treatment here, because there is nothing useful to say about
// merging one — a wrongly-materialised link is repaired by a rebuild (§6.1).
const gitAttributesBlock = `# journals: order lives in the data, so keeping both sides is the definition of the merge
**/logs/*.jsonl merge=union

# wholly generated: any side is as good as any other, because ` + "`para rebuild`" + ` produces the truth
**/ACTIVITY.md          merge=ours linguist-generated=true
**/MEASUREMENTS.csv     merge=ours linguist-generated=true
.agents/rules/**/*.md   merge=ours linguist-generated=true
`

func (gitAttributesRenderer) Render(in In) ([]byte, error) {
	path, err := GitAttributes.Path(in)
	if err != nil {
		return nil, err
	}
	// Append rather than replace, because §2.2 calls this file's human-owned
	// part "append-only to an existing file": the repository owns it and para's
	// block is something it gains. A .gitattributes that predates para is the
	// normal case, not a damaged one.
	out, err := mdfile.AppendDelimited(mdfile.HashMarkers, in.existing(path), []byte(gitAttributesBlock))
	if err != nil {
		return nil, damagedBlock(path, err)
	}
	return out, nil
}

// damagedBlock names the file and the repair.
//
// mdfile knows the block is malformed and nothing else; it is a codec and has no
// idea which file it was handed. Without this the user gets "missing para-owned
// block: no end marker found" with no path — from `para init`, which they ran in
// a directory, and from `para rebuild`, which touches dozens of files. The repair
// is named because it is not obvious: para cannot fix this itself, precisely
// because fixing it means deciding where a block it did not write ends.
//
// Both .gitattributes and CLAUDE.md can hold a damaged block, so the path is a
// parameter rather than the constant it used to be.
func damagedBlock(path string, err error) error {
	return paraerr.Wrap(paraerr.KindValidation, err, fmt.Sprintf(
		"%s holds para's begin marker with no matching end marker — restore the marker, or delete every line from the begin marker down and let para write the block again",
		path))
}

// WithoutGitAttributesBlock returns existing with para's block taken out, and
// reports whether there was one to take.
//
// It is Render's other half, for `emit.gitattributes = false` (§9), and it is a
// function rather than a mode of Render because the two answer different
// questions and only one of them is a projection: Render says what para writes,
// this says what is left when para writes nothing. Both read the same markers
// from the same constant, which is the part that must not drift.
//
// Turning the key off shortens the file rather than deleting it, and that
// asymmetry with `emit.claude` — which removes eight whole files (§6.1) — is
// exactly the difference between owning a file and owning a block inside one
// (§2.2). A .gitattributes that held nothing else comes back as an empty file,
// which is the file para was handed: para cannot tell a file it created from
// an empty one it was given, and the safe reading is that every line it did not
// write is the repository's.
func WithoutGitAttributesBlock(existing []byte) ([]byte, bool, error) {
	return withoutBlock(mdfile.HashMarkers, gitAttributesFile, existing)
}

// withoutBlock is WithoutGitAttributesBlock's and WithoutClaudeBlock's shared
// remover: RemoveDelimited, damagedBlock on error, and the same (bytes, found,
// error) shape either file returns. One rule about what a damaged block means,
// so the two files cannot drift into disagreeing about it.
func withoutBlock(m mdfile.Markers, path string, existing []byte) ([]byte, bool, error) {
	out, found, err := mdfile.RemoveDelimited(m, existing)
	if err != nil {
		return nil, false, damagedBlock(path, err)
	}
	return out, found, nil
}
