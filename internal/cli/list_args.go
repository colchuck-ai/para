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

	first, err := address.ParseNoun(args[0])
	if err != nil {
		return listArgs{}, err
	}

	if len(args) == 1 {
		if first == address.Container {
			return listArgs{}, errContainerFilter()
		}
		out := listArgs{Kind: first}
		if archived {
			out.Scope = archiveRoot
		}
		return out, nil
	}

	if second, err := address.ParseNoun(args[1]); err == nil {
		if first == address.Container {
			return listArgs{}, errContainerFilter()
		}
		chain := ""
		if len(args) == 3 {
			chain = args[2]
		}
		scope, err := chainToLocator(second.String(), chain, archived, true)
		if err != nil {
			return listArgs{}, err
		}
		return listArgs{Kind: first, Scope: scope}, nil
	}

	if len(args) == 3 {
		return listArgs{}, paraerr.Newf(paraerr.KindValidation,
			"%q is not a noun (want one of: %s) — a noun and a chain is `list`'s whole scope, with nothing after it",
			args[1], nounWords())
	}

	scope, err := chainToLocator(first.String(), args[1], archived, true)
	if err != nil {
		return listArgs{}, err
	}
	return listArgs{Scope: scope}, nil
}

// errContainerFilter is R20's refusal: container is a legal noun everywhere
// else an address is read, but never in list's filter position, because
// containers are transparent to list and are never rows (§25).
func errContainerFilter() error {
	return paraerr.New(paraerr.KindValidation,
		"container is not a legal filter — containers are transparent to list and are never rows")
}
