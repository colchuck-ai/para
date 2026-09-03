package cli

import (
	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
)

// listArgs is R19's lookahead rule, resolved: an optional kind filter and
// the scope to list beneath. A zero Kind (kindmeta.KindUnknown) is "no
// filter" — it is never itself a legal noun, so it cannot be confused with
// one.
type listArgs struct {
	Kind  kindmeta.Kind
	Scope locator.Locator
}

// archiveRoot is R7's "the whole archive" (`para list --archived`, no other
// argument): the one locator IsArchiveRoot admits that no Address converts
// to (address/bucket.go's own comment), used here as the scope that redirects
// an otherwise tree-wide row of R19's table into archive/ instead.
var archiveRoot = locator.Locator{"archive"}

// parseListArgs reads `list`'s 0-3 positional args and returns R19's table,
// scanned left to right with one token of lookahead: whether the second
// argument itself parses as a noun decides whether it is the filter's
// bucket scope or the first noun's own chain. R6 is what makes this
// unambiguous — a noun word can never legally be an id, so a second
// argument that parses as a noun is never anything else.
//
// `archived` (R7) only has work to do here in the two rows below that never
// reach `chainToLocator` — a noun and a chain thread it through on their own.
// Without it, `para list --archived` and `para list project --archived`
// would silently list the live tree, the one thing `--archived` promises not
// to do.
//
// `root` and `cwd` exist for exactly one row, checked before any of R19's
// five: "." (R16) stands in for a whole `<noun> <chain>` scope as a single
// token, the same convenience `show`/`log`/etc. get from `parseAddressArgs`.
// `list` never had a second slot for a kind filter beside it — nothing in
// R16, R19, or the old single-locator grammar this replaces gives "." a
// second meaning — so it is refused with anything after it rather than
// guessed at. Like `parseAddressArgs`'s own dot branch, `archived` is not
// applied here: cwd already names an archived or live place, not an
// independent qualifier on top of one.
func parseListArgs(root, cwd string, args []string, archived bool) (listArgs, error) {
	if len(args) > 3 {
		return listArgs{}, paraerr.Newf(paraerr.KindValidation,
			"list takes at most a kind filter, a noun, and a chain — got %d arguments", len(args))
	}
	if len(args) == 0 {
		if archived {
			return listArgs{Scope: archiveRoot}, nil
		}
		return listArgs{}, nil
	}
	if args[0] == "." {
		if len(args) > 1 {
			return listArgs{}, paraerr.New(paraerr.KindValidation,
				`"." stands for the whole scope and takes no further argument`)
		}
		scope, err := tree.ResolveDot(root, cwd)
		if err != nil {
			return listArgs{}, err
		}
		return listArgs{Scope: scope}, nil
	}

	kind, scope, err := kindFilterLookahead(args, archived)
	if err != nil {
		return listArgs{}, err
	}
	return listArgs{Kind: kind, Scope: scope}, nil
}

// kindFilterLookahead is R19's own disambiguation, factored out so review
// (para-nd3, internal/cli/review.go's parseReviewArgs) can reuse the
// identical rule rather than reimplementing it: whether the second argument
// itself parses as a noun decides whether it is the filter's scope or the
// first noun's own chain. args must be non-empty and args[0] must not be
// "." — both callers check those cases themselves first, since what a bare
// zero-argument or dot call means is specific to each command, not part of
// this lookahead.
//
// Neither refusal below names the calling command — matching every other
// shared refusal in this file (chainToLocator's, parseAddressArgs') — since
// both are true of the rule itself, not of whichever command asked: a bad
// second noun or a `container` filter is illegal the same way regardless of
// which caller's grammar reached this lookahead.
func kindFilterLookahead(args []string, archived bool) (kindmeta.Kind, locator.Locator, error) {
	first, err := address.ParseNoun(args[0])
	if err != nil {
		return kindmeta.KindUnknown, nil, err
	}

	if len(args) == 1 {
		if first == address.Container {
			return kindmeta.KindUnknown, nil, errContainerFilter()
		}
		var scope locator.Locator
		if archived {
			scope = archiveRoot
		}
		return first, scope, nil
	}

	if second, err := address.ParseNoun(args[1]); err == nil {
		if first == address.Container {
			return kindmeta.KindUnknown, nil, errContainerFilter()
		}
		chain := ""
		if len(args) == 3 {
			chain = args[2]
		}
		scope, err := chainToLocator(second.String(), chain, archived, true)
		if err != nil {
			return kindmeta.KindUnknown, nil, err
		}
		return first, scope, nil
	}

	if len(args) == 3 {
		return kindmeta.KindUnknown, nil, paraerr.Newf(paraerr.KindValidation,
			"%q is not a noun (want one of: %s) — a noun and a chain is the whole scope, with nothing after it",
			args[1], kindmeta.KindWordList())
	}

	scope, err := chainToLocator(first.String(), args[1], archived, true)
	if err != nil {
		return kindmeta.KindUnknown, nil, err
	}
	return kindmeta.KindUnknown, scope, nil
}

// errContainerFilter is R20's refusal: container is a legal noun everywhere
// else an address is read, but never in a kind filter position, because
// containers are transparent to a kind-filtered walk and are never rows
// (§25) — true of `list` and `review` alike, so the message names neither.
func errContainerFilter() error {
	return paraerr.New(paraerr.KindValidation,
		"container is not a legal filter — containers are transparent and are never rows")
}
