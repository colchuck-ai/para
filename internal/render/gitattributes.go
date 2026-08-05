package render

import (
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
**/CLAUDE.md            merge=ours linguist-generated=true
.agents/rules/**/*.md   merge=ours linguist-generated=true
`

func (gitAttributesRenderer) Render(in In) ([]byte, error) {
	path, err := GitAttributes.Path(in)
	if err != nil {
		return nil, err
	}
	return mdfile.ReplaceDelimited(mdfile.HashMarkers, in.existing(path), []byte(gitAttributesBlock))
}
