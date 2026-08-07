// Package mdfile implements the frontmatter+body codec §2.1 requires for
// every entity's README.md and every skill's SKILL.md: para owns the
// frontmatter, humans own the body, and a rewrite must preserve the body
// byte for byte (§2.2). It also implements the delimited-block codec
// AGENTS.md needs (§6), in block.go.
//
// As with ptoml, key order on decode is not recovered — a Document gives
// random access by key — because each file kind declares its own fixed
// frontmatter field order (Phase 6+), and re-encoding in that declared
// order is what reproduces the original bytes.
package mdfile

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// delim is the frontmatter fence line, exactly as it must appear at the
// start of the file and again to close the block.
const delim = "---\n"

// Split parses data into its raw frontmatter bytes (the YAML between the
// two "---" fence lines, exclusive) and its body (everything after the
// closing fence's newline, preserved exactly as found — §2.2's "the
// body — everything after the frontmatter").
func Split(data []byte) (frontmatter, body []byte, err error) {
	if !bytes.HasPrefix(data, []byte(delim)) {
		return nil, nil, paraerr.New(paraerr.KindValidation, "missing frontmatter: file must start with a \"---\" line")
	}
	rest := data[len(delim):]

	if bytes.HasPrefix(rest, []byte(delim)) {
		return []byte{}, rest[len(delim):], nil
	}

	idx := bytes.Index(rest, []byte("\n"+delim))
	if idx == -1 {
		return nil, nil, paraerr.New(paraerr.KindValidation, "missing frontmatter: no closing \"---\" line found")
	}
	frontmatter = rest[:idx+1]
	body = rest[idx+1+len(delim):]
	return frontmatter, body, nil
}

// Render reassembles a full file from ordered frontmatter fields and a
// (presumably preserved) body.
func Render(fields []Field, body []byte) ([]byte, error) {
	fm, err := EncodeFrontmatter(fields)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString(delim)
	b.Write(fm)
	b.WriteString(delim)
	b.Write(body)
	return b.Bytes(), nil
}

// Kind identifies which of Value's fields is populated.
type Kind int

const (
	KindString Kind = iota
	KindInt64
	KindFloat64
	KindBool
	KindStringArray
)

// Value is a single frontmatter scalar or string array, tagged by Kind.
type Value struct {
	Kind     Kind
	Str      string
	Int      int64
	Float    float64
	Bool     bool
	StrArray []string
}

// String constructs a KindString Value.
func String(s string) Value { return Value{Kind: KindString, Str: s} }

// Int64 constructs a KindInt64 Value.
func Int64(i int64) Value { return Value{Kind: KindInt64, Int: i} }

// Float64 constructs a KindFloat64 Value.
func Float64(f float64) Value { return Value{Kind: KindFloat64, Float: f} }

// Bool constructs a KindBool Value.
func Bool(b bool) Value { return Value{Kind: KindBool, Bool: b} }

// StringArray constructs a KindStringArray Value.
func StringArray(a []string) Value { return Value{Kind: KindStringArray, StrArray: a} }

// Field is one "key: value" line, in the order it will be rendered.
type Field struct {
	Key   string
	Value Value
}

// EncodeFrontmatter renders fields in the given order, one "key: value"
// line each (§0.2): no map iteration, no reordering. Two calls over the
// same fields always produce identical bytes. Every scalar is
// double-quoted, including strings, so a value that looks like a YAML
// keyword ("true", "null", a bare number) never silently changes type.
func EncodeFrontmatter(fields []Field) ([]byte, error) {
	var b strings.Builder
	for _, f := range fields {
		if f.Key == "" {
			return nil, paraerr.New(paraerr.KindValidation, "empty frontmatter key")
		}
		lit, err := encodeValue(f.Value)
		if err != nil {
			return nil, err
		}
		b.WriteString(f.Key)
		b.WriteString(": ")
		b.WriteString(lit)
		b.WriteString("\n")
	}
	return []byte(b.String()), nil
}

func encodeValue(v Value) (string, error) {
	switch v.Kind {
	case KindString:
		return quoteString(v.Str), nil
	case KindInt64:
		return strconv.FormatInt(v.Int, 10), nil
	case KindFloat64:
		return formatFloat(v.Float), nil
	case KindBool:
		if v.Bool {
			return "true", nil
		}
		return "false", nil
	case KindStringArray:
		if len(v.StrArray) == 0 {
			return "[]", nil
		}
		parts := make([]string, len(v.StrArray))
		for i, s := range v.StrArray {
			parts[i] = quoteString(s)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	default:
		return "", paraerr.Newf(paraerr.KindInternal, "mdfile: unknown value kind %d", v.Kind)
	}
}

// formatFloat renders f as a decimal literal that always carries a point,
// matching ptoml's rule so the same value round-trips the same way in
// either codec.
func formatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// The two non-control characters yaml.v3's scanner reads as line breaks.
const (
	lineSeparator      = '\u2028'
	paragraphSeparator = '\u2029'
)

// quoteString renders s as a YAML double-quoted scalar, escaping backslash,
// quote, the short escapes, and every other control character via \uXXXX —
// a stray control byte left unescaped would make the frontmatter invalid
// YAML on the next parse, breaking round-trip identity (§0.2). YAML
// forbids the C0 range (U+0000-U+001F), DEL (U+007F), and the C1 range
// (U+0080-U+009F) unescaped — a fuzz run caught yaml.v3 rejecting an
// unescaped C1 character before this covered it too.
//
// U+2028 and U+2029 are escaped for a different reason, and a second fuzz run
// caught it: they are not control characters and yaml.v3 accepts them, but its
// scanner treats them as **line breaks**, so a quoted scalar carrying one is
// folded on the way back in: a value of space-then-U+2028 decodes as U+2028
// alone, because folding eats the space beside the break. It survives the file
// and does not survive the parse, which is drift `doctor` would report forever
// on a tree nobody touched.
func quoteString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if plainYAML(r) {
				b.WriteRune(r)
			} else {
				fmt.Fprintf(&b, `\u%04X`, r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// plainYAML reports whether r may stand for itself inside a double-quoted
// scalar. It is YAML's printable set, minus the characters the scanner reads
// as line breaks — and it is stated as a set rather than as a list of
// exceptions because the exceptions kept arriving one fuzz seed at a time:
// first a C1 character, then U+2028, then U+FFFF, each of which yaml.v3
// refuses or folds while the encoder wrote it happily.
//
// The set is the YAML 1.2 spec's c-printable: tab, LF, CR (handled by their
// own escapes above), U+0020-U+007E, U+0085, U+00A0-U+D7FF, U+E000-U+FFFD, and
// U+10000-U+10FFFF. Everything else — the C0 and C1 ranges, DEL, the
// surrogates, and the two non-characters at the end of the BMP — is what
// yaml.v3 rejects as "control characters are not allowed".
//
// U+0085, U+2028, and U+2029 are printable by that definition and excluded
// here anyway: they are line breaks to the scanner, and a scalar that folds is
// a value that does not survive its own file.
func plainYAML(r rune) bool {
	switch {
	case r == 0x85, r == lineSeparator, r == paragraphSeparator:
		return false
	case r >= 0x20 && r <= 0x7E:
		return true
	case r >= 0xA0 && r <= 0xD7FF:
		return true
	case r >= 0xE000 && r <= 0xFFFD:
		return true
	case r >= 0x10000 && r <= 0x10FFFF:
		return true
	default:
		return false
	}
}

// Document is a parsed frontmatter block, queryable by key. It carries no
// ordering — see the package doc for why that is never needed.
type Document struct {
	values map[string]any
}

// DecodeFrontmatter parses raw (as returned by Split) via gopkg.in/yaml.v3
// into a Document.
func DecodeFrontmatter(raw []byte) (Document, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Document{values: map[string]any{}}, nil
	}
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return Document{}, paraerr.Wrap(paraerr.KindValidation, err, "invalid frontmatter YAML")
	}
	return Document{values: m}, nil
}

// Value returns key's value in Value form, letting a caller round-trip an
// arbitrary field without knowing its kind ahead of time.
func (d Document) Value(key string) (Value, bool) {
	v, ok := d.values[key]
	if !ok {
		return Value{}, false
	}
	switch t := v.(type) {
	case string:
		return String(t), true
	case int:
		return Int64(int64(t)), true
	case int64:
		return Int64(t), true
	case uint64:
		return Int64(int64(t)), true
	case float64:
		return Float64(t), true
	case bool:
		return Bool(t), true
	case []any:
		arr, ok := toStringSlice(t)
		if !ok {
			return Value{}, false
		}
		return StringArray(arr), true
	default:
		return Value{}, false
	}
}

func (d Document) String(key string) (string, bool) {
	s, ok := d.values[key].(string)
	return s, ok
}

func (d Document) Int64(key string) (int64, bool) {
	switch v := d.values[key].(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case uint64:
		return int64(v), true
	default:
		return 0, false
	}
}

func (d Document) Float64(key string) (float64, bool) {
	f, ok := d.values[key].(float64)
	return f, ok
}

func (d Document) Bool(key string) (bool, bool) {
	b, ok := d.values[key].(bool)
	return b, ok
}

func (d Document) StringArray(key string) ([]string, bool) {
	a, ok := d.values[key].([]any)
	if !ok {
		return nil, false
	}
	return toStringSlice(a)
}

func toStringSlice(a []any) ([]string, bool) {
	out := make([]string, len(a))
	for i, v := range a {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		out[i] = s
	}
	return out, true
}
