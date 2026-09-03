package cli

import (
	"strings"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
)

// addressArity is R12's table read as the three axes an address argument
// varies along: whether the noun itself may be left off the command line
// (rootOK — omitting it entirely names the tree root, R18), whether a noun
// with no chain names R17's bucket rather than a refusal (bucketOK), and
// whether "." may stand in for the whole address (dotOK, R16).
type addressArity struct {
	rootOK   bool
	bucketOK bool
	dotOK    bool
}

var (
	// entityArity is note's, remove's, and archive's shape: a noun and
	// always a chain, naming an entity — never a bucket, never the root.
	// "." is accepted: all three name something that already exists.
	entityArity = addressArity{dotOK: true}
	// entityArityNoDot is unarchive's shape: the same, but "." is refused
	// because "." resolves against the working directory and unarchive
	// relocates the very thing being named — you cannot stand in it.
	entityArityNoDot = addressArity{}
	// bucketArity is show's, log's, and path's shape: the noun is required,
	// but alone it names R17's bucket.
	bucketArity = addressArity{bucketOK: true, dotOK: true}
	// scopeArity is activity's, review's, rebuild's, and doctor's shape:
	// no arguments at all names the tree root (R18), and a noun alone
	// names the bucket (R17).
	scopeArity = addressArity{rootOK: true, bucketOK: true, dotOK: true}
)

// parseAddressArgs reads the noun and chain (R1) off the front of args and
// returns the Locator the rest of the CLI runs against, plus whatever args
// are left over for the caller to interpret — note's text, unset's field
// list, and so on. It is the one function every row of R12's table that
// reads a noun from the command line funnels through: nineteen hand-rolled
// parsers become the four arity shapes above, applied here.
//
// "." is checked first, because it is not a noun at all: it occupies the
// whole address as a single token (R16), resolved by walking up from cwd to
// the nearest .para/state.toml.
func parseAddressArgs(root, cwd string, args []string, arity addressArity, archived bool) (locator.Locator, []string, error) {
	if len(args) == 0 {
		if arity.rootOK {
			return nil, args, nil
		}
		return nil, args, paraerr.Newf(paraerr.KindValidation, "a noun is required (one of: %s)", kindmeta.KindWordList())
	}
	if args[0] == "." {
		if !arity.dotOK {
			return nil, args[1:], paraerr.New(paraerr.KindValidation, `"." is not accepted here`)
		}
		loc, err := tree.ResolveDot(root, cwd)
		return loc, args[1:], err
	}

	rest := args[1:]
	chain := ""
	if len(rest) > 0 {
		chain = rest[0]
		rest = rest[1:]
	}
	loc, err := chainToLocator(args[0], chain, archived, arity.bucketOK)
	return loc, rest, err
}

// chainToLocator builds the Locator a noun word and a dot-joined chain
// name (R1), applying --archived (R7) and refusing an empty chain when
// bucketOK is false (R17's "names a bucket, not an entity"). It is
// parseAddressArgs' own foundation, and it is also add's, set's, and
// unset's entry point once their noun-dispatching subcommand has fixed the
// noun (R3) — there the noun never comes from args at all, so this is the
// layer both paths share.
func chainToLocator(nounWord, chain string, archived, bucketOK bool) (locator.Locator, error) {
	a, err := address.Parse(nounWord, chain)
	if err != nil {
		return nil, err
	}
	if len(a.Chain) == 0 && !bucketOK {
		return nil, paraerr.Newf(paraerr.KindValidation, "%s names a bucket, not an entity — a chain is required", a.Noun.String())
	}
	a.Archived = archived
	return a.ToLocator()
}

// dispatchedChainToLocator is chainToLocator's counterpart for add's,
// set's, and unset's noun-dispatching subcommands (R3): the noun is already
// fixed by which subcommand cobra dispatched to, so the only argument left
// is the chain — except that "." (R16) may stand in for it, resolved
// against cwd the way parseAddressArgs resolves it for the commands whose
// noun is a runtime argument instead.
//
// A dispatched noun can no longer be read off "." the way parseAddressArgs
// reads one off args[0], so "." is instead checked against what it actually
// resolves to — via address.FromLocator, never kindmeta.KindOf, since the
// CLI path does not derive a kind by inference (plan §0.1) — refusing a
// mismatch by name rather than silently mutating whatever "." happened to
// name under a noun the user did not mean.
//
// add never calls this: "." is deliberately refused there (it resolves to
// something that already exists, and add is for something that does not),
// so add keeps calling chainToLocator directly.
func dispatchedChainToLocator(root, cwd string, kind address.Noun, chain string, archived bool) (locator.Locator, error) {
	if chain != "." {
		return chainToLocator(kind.String(), chain, archived, false)
	}
	loc, err := tree.ResolveDot(root, cwd)
	if err != nil {
		return nil, err
	}
	addr, err := address.FromLocator(loc)
	if err != nil {
		return nil, err
	}
	if addr.Noun != kind {
		return nil, paraerr.Newf(paraerr.KindValidation,
			"the working directory names %s, not %s", withArticle(addr.Noun.String()), withArticle(kind.String()))
	}
	return loc, nil
}

// withArticle prefixes noun with "a" or "an", since "a area" and "a
// objective" read as typos in a refusal a user is already annoyed by.
func withArticle(noun string) string {
	if strings.ContainsRune("aeiou", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}
