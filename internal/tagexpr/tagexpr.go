// Package tagexpr implements the boolean expression grammar `--tags` accepts
// (design §17): word operators `not`, `and`, `or` at precedence
// not → and → or, no parentheses — every expression has a disjunctive normal
// form instead — and a bare comma list as or's short form. It is pure: a
// parsed Expr is evaluated against a tag-membership test the caller
// supplies, never against a stored entity.
package tagexpr

import (
	"regexp"
	"strings"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// ReservedWords cannot appear as a tag: they are the expression's own
// operators, and interpreting them as either would be ambiguous (§17).
var ReservedWords = []string{"and", "or", "not"}

// IsReserved reports whether s is one of the words §17 reserves and so can
// never be used as a tag.
func IsReserved(s string) bool {
	for _, r := range ReservedWords {
		if s == r {
			return true
		}
	}
	return false
}

var tagPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// ValidTag reports whether s has a tag's charset — `[a-z0-9-]`, the same
// shape as a locator segment (§17). It does not check ReservedWords; a
// reserved word never reaches here because the parser consumes it as an
// operator first, but a caller validating a tag being *set* on an entity
// must check both.
func ValidTag(s string) bool {
	return tagPattern.MatchString(s)
}

// Expr is a parsed --tags boolean expression.
type Expr interface {
	// Match reports whether the expression is satisfied, given has — a
	// membership test over whatever tag set the caller holds, so this
	// package never needs to know the tag container's shape.
	Match(has func(tag string) bool) bool
}

type tagExpr struct{ tag string }

func (e tagExpr) Match(has func(string) bool) bool { return has(e.tag) }

type notExpr struct{ inner Expr }

func (e notExpr) Match(has func(string) bool) bool { return !e.inner.Match(has) }

type andExpr struct{ left, right Expr }

func (e andExpr) Match(has func(string) bool) bool { return e.left.Match(has) && e.right.Match(has) }

type orExpr struct{ left, right Expr }

func (e orExpr) Match(has func(string) bool) bool { return e.left.Match(has) || e.right.Match(has) }

// Parse parses s per §17's grammar. A comma is accepted anywhere a literal
// "or" would be — the two are interchangeable tokens, not distinct
// operators — which is what makes a bare comma list "or"'s short form.
func Parse(s string) (Expr, error) {
	tokens := tokenize(s)
	if len(tokens) == 0 {
		return nil, paraerr.New(paraerr.KindValidation, "empty tag expression")
	}

	p := &parser{tokens: tokens, expr: s}
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.tokens) {
		return nil, paraerr.Newf(paraerr.KindValidation, "tag expression %q: unexpected %q", s, p.peek())
	}
	return expr, nil
}

// tokenize splits s into words and lone "," tokens, so a comma reads as an
// operator regardless of the whitespace around it — "rust,reference" and
// "rust , reference" tokenize identically. A malformed comma (doubled,
// leading, or trailing) surfaces as a parse error from the grammar itself —
// an operator where a tag was expected, or an expression that ends
// unexpectedly — rather than a special case here.
func tokenize(s string) []string {
	return strings.Fields(strings.ReplaceAll(s, ",", " , "))
}

type parser struct {
	tokens []string
	pos    int
	expr   string
}

func (p *parser) peek() string {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return ""
}

func (p *parser) next() string {
	t := p.peek()
	p.pos++
	return t
}

// parseOr binds loosest: `or` and `,` are interchangeable at this level.
func (p *parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek() == "or" || p.peek() == "," {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = orExpr{left, right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.peek() == "and" {
		p.next()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = andExpr{left, right}
	}
	return left, nil
}

// parseNot binds tightest, and is right-recursive so "not not x" parses.
func (p *parser) parseNot() (Expr, error) {
	if p.peek() == "not" {
		p.next()
		inner, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return notExpr{inner}, nil
	}
	return p.parseTag()
}

func (p *parser) parseTag() (Expr, error) {
	tok := p.next()
	if tok == "" {
		return nil, paraerr.Newf(paraerr.KindValidation, "tag expression %q ends unexpectedly", p.expr)
	}
	if tok == "and" || tok == "or" || tok == "not" || tok == "," {
		return nil, paraerr.Newf(paraerr.KindValidation, "tag expression %q: unexpected operator %q where a tag was expected", p.expr, tok)
	}
	if !ValidTag(tok) {
		return nil, paraerr.Newf(paraerr.KindValidation, "%q is not a valid tag (must match [a-z0-9-]+)", tok)
	}
	return tagExpr{tok}, nil
}
