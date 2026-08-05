// Package ptoml implements the byte-stable TOML codec design §0.2 requires:
// an ordered writer that renders exactly the fields it is given, in the
// order given, and a reader wrapping pelletier/go-toml/v2 for random-access
// lookup by key. Every timestamp in a truth file is stored as a TOML string
// (§8.1-§8.3's examples all quote `created`, `due`, etc.), so the value
// grammar here only needs to cover string, int64, float64, bool, and string
// array — nothing else appears anywhere in a state.toml, config.toml, or
// tree.toml.
//
// Key order is not recovered from a parsed document — a Document gives
// random access by key, not iteration order — because para only ever reads
// back files it wrote itself, and each file kind declares its own fixed
// field order (Phase 7/8). Round-trip identity (§0.2) therefore holds by
// construction: decode, look up each declared key, re-encode in the
// declared order, and the bytes match because that order is what wrote them
// in the first place.
package ptoml

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// Kind identifies which of Value's fields is populated.
type Kind int

const (
	KindString Kind = iota
	KindInt64
	KindFloat64
	KindBool
	KindStringArray
)

// Value is a single TOML scalar or string array, tagged by Kind.
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

// Field is one key = value line, keyed by its (possibly dotted) TOML key.
type Field struct {
	Key   string
	Value Value
}

// keySegmentPattern is TOML's bare-key charset. Every key this codec is
// asked to write is either a single identifier or a dotted path of them
// (§7's config families, e.g. "log.rotate-bytes") — nothing in the design
// ever needs a quoted key, so Encode rejects anything outside this charset
// rather than silently emitting invalid TOML.
var keySegmentPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func validateKey(key string) error {
	if key == "" {
		return paraerr.New(paraerr.KindValidation, "empty TOML key")
	}
	for _, seg := range strings.Split(key, ".") {
		if !keySegmentPattern.MatchString(seg) {
			return paraerr.Newf(paraerr.KindValidation, "%q is not a valid TOML key (bare keys only, dots as path separators)", key)
		}
	}
	return nil
}

// Encode renders fields in the given order, one `key = value` per line
// (§0.2): no map iteration, no reordering, no blank lines. Two calls over
// the same fields always produce identical bytes.
func Encode(fields []Field) ([]byte, error) {
	var b strings.Builder
	for _, f := range fields {
		if err := validateKey(f.Key); err != nil {
			return nil, err
		}
		lit, err := encodeValue(f.Value)
		if err != nil {
			return nil, err
		}
		b.WriteString(f.Key)
		b.WriteString(" = ")
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
		return "", paraerr.Newf(paraerr.KindInternal, "ptoml: unknown value kind %d", v.Kind)
	}
}

// formatFloat renders f as a TOML float literal that always carries a
// decimal point — "1" would reparse as a TOML integer, changing the value's
// type on round-trip, so a whole number renders as "1.0".
func formatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// quoteString renders s as a TOML basic (double-quoted) string, escaping
// exactly what TOML requires: backslash and quote literally, and the
// control characters with their short escapes. TOML's basic-string grammar
// forbids every control character other than tab unescaped — U+0000
// through U+0008, U+000A through U+001F, *and* U+007F (DEL) — so DEL gets
// the same \u escape as the C0 range, a case a fuzz run caught pelletier
// rejecting when it was left unescaped.
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
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 || r == 0x7F {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Document is a parsed TOML file, queryable by key (including dotted paths
// into nested tables produced by TOML's own dotted-key syntax). It carries
// no ordering — see the package doc for why that is never needed.
type Document struct {
	values map[string]any
}

// Decode parses data via pelletier/go-toml/v2 into a Document.
func Decode(data []byte) (Document, error) {
	var m map[string]any
	if err := toml.Unmarshal(data, &m); err != nil {
		return Document{}, paraerr.Wrap(paraerr.KindValidation, err, "invalid TOML")
	}
	return Document{values: m}, nil
}

// lookup walks a dotted key through nested tables, as produced by decoding
// TOML's dotted-key or [table] syntax into a map[string]any tree.
func (d Document) lookup(key string) (any, bool) {
	segs := strings.Split(key, ".")
	cur := any(d.values)
	for _, seg := range segs {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[seg]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// Value returns key's value in Value form, letting a caller round-trip an
// arbitrary field without knowing its kind ahead of time.
func (d Document) Value(key string) (Value, bool) {
	v, ok := d.lookup(key)
	if !ok {
		return Value{}, false
	}
	switch t := v.(type) {
	case string:
		return String(t), true
	case int64:
		return Int64(t), true
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
	v, ok := d.lookup(key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func (d Document) Int64(key string) (int64, bool) {
	v, ok := d.lookup(key)
	if !ok {
		return 0, false
	}
	i, ok := v.(int64)
	return i, ok
}

func (d Document) Float64(key string) (float64, bool) {
	v, ok := d.lookup(key)
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

func (d Document) Bool(key string) (bool, bool) {
	v, ok := d.lookup(key)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func (d Document) StringArray(key string) ([]string, bool) {
	v, ok := d.lookup(key)
	if !ok {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	return toStringSlice(arr)
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
