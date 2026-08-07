package tagexpr

import (
	"slices"
	"strings"
	"testing"
)

func has(tags ...string) func(string) bool {
	set := make(map[string]bool, len(tags))
	for _, t := range tags {
		set[t] = true
	}
	return func(tag string) bool { return set[tag] }
}

// TestParse_Match covers every worked example in §17, plus the precedence
// and comma-as-or cases the grammar promises.
func TestParse_Match(t *testing.T) {
	cases := []struct {
		name string
		expr string
		yes  []string // tag sets that should match
		no   []string // comma-joined tag sets that should not match ("" for empty set)
	}{
		{
			name: "bare tag",
			expr: "rust",
			yes:  []string{"rust", "rust,reference"},
			no:   []string{"", "reference"},
		},
		{
			name: "comma is or's short form",
			expr: "rust,reference",
			yes:  []string{"rust", "reference", "rust,reference"},
			no:   []string{"", "kafka"},
		},
		{
			name: "and and not",
			expr: "kafka and not deprecated",
			yes:  []string{"kafka"},
			no:   []string{"kafka,deprecated", "deprecated", ""},
		},
		{
			name: "and binds tighter than or, no parens needed",
			expr: "rust and reference or kafka and not deprecated",
			yes:  []string{"rust,reference", "kafka", "kafka,rust"},
			no:   []string{"rust", "reference", "kafka,deprecated", ""},
		},
		{
			// Same expression as above with the comma standing in for
			// "or" (§17: a bare comma list is or’s short form) — proves
			// the substitution holds next to "and", not just in a flat
			// comma list.
			name: "comma stands in for or even next to and",
			expr: "rust and reference, kafka and not deprecated",
			yes:  []string{"rust,reference", "kafka", "kafka,rust"},
			no:   []string{"rust", "reference", "kafka,deprecated", ""},
		},
		{
			name: "explicit or keyword",
			expr: "rust or kafka",
			yes:  []string{"rust", "kafka", "rust,kafka"},
			no:   []string{"", "reference"},
		},
		{
			name: "not binds tighter than and",
			expr: "not deprecated and kafka",
			yes:  []string{"kafka"},
			no:   []string{"kafka,deprecated", "deprecated", ""},
		},
		{
			name: "double not",
			expr: "not not kafka",
			yes:  []string{"kafka"},
			no:   []string{""},
		},
		{
			name: "comma with surrounding whitespace tokenizes the same",
			expr: "rust , reference",
			yes:  []string{"rust", "reference"},
			no:   []string{""},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expr, err := Parse(c.expr)
			if err != nil {
				t.Fatalf("Parse(%q): %v", c.expr, err)
			}
			for _, set := range c.yes {
				if !expr.Match(hasFromCSV(set)) {
					t.Errorf("Parse(%q).Match(%q) = false, want true", c.expr, set)
				}
			}
			for _, set := range c.no {
				if expr.Match(hasFromCSV(set)) {
					t.Errorf("Parse(%q).Match(%q) = true, want false", c.expr, set)
				}
			}
		})
	}
}

func hasFromCSV(csv string) func(string) bool {
	if csv == "" {
		return has()
	}
	var tags []string
	start := 0
	for i := 0; i < len(csv); i++ {
		if csv[i] == ',' {
			tags = append(tags, csv[start:i])
			start = i + 1
		}
	}
	tags = append(tags, csv[start:])
	return has(tags...)
}

func TestParse_Invalid(t *testing.T) {
	cases := []string{
		"",
		"and",
		"or",
		"not",
		"rust and",
		"and rust",
		"rust,,reference",
		"rust,",
		",rust",
		"rust and and reference",
		"RUST",
		"rust_reference",
	}
	for _, expr := range cases {
		if _, err := Parse(expr); err == nil {
			t.Errorf("Parse(%q): want error, got nil", expr)
		}
	}
}

func TestIsReserved(t *testing.T) {
	for _, w := range []string{"and", "or", "not"} {
		if !IsReserved(w) {
			t.Errorf("IsReserved(%q) = false, want true", w)
		}
	}
	for _, w := range []string{"rust", "kafka", "android"} {
		if IsReserved(w) {
			t.Errorf("IsReserved(%q) = true, want false", w)
		}
	}
}

func TestValidTag(t *testing.T) {
	valid := []string{"rust", "kafka-2", "a1-b2-c3"}
	invalid := []string{"", "Rust", "kafka_deprecated", "a.b", "has space"}
	for _, s := range valid {
		if !ValidTag(s) {
			t.Errorf("ValidTag(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if ValidTag(s) {
			t.Errorf("ValidTag(%q) = true, want false", s)
		}
	}
}

// FuzzParseNeverPanics is the least a parser owes its caller. `--tags` takes a
// string straight off the command line (§17), so every byte sequence a shell can
// produce reaches Parse, and the only two acceptable outcomes are an expression
// and an error.
func FuzzParseNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"", "kafka", "kafka,consumer", "kafka|consumer", "!kafka",
		"kafka,!consumer|urgent", "(", ")", "!", ",", "|", "!!kafka",
		"a,,b", "  ", "!(kafka)", "kafka|", "|kafka",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		expr, err := Parse(s)
		if err != nil {
			if expr != nil {
				t.Errorf("Parse(%q) returned both an expression and an error", s)
			}
			return
		}
		if expr == nil {
			t.Fatalf("Parse(%q) returned neither an expression nor an error", s)
		}
		// A parsed expression must be evaluable against any tag set, which is
		// where a half-built node would surface as a nil dereference.
		for _, has := range []func(string) bool{
			func(string) bool { return true },
			func(string) bool { return false },
			func(tag string) bool { return strings.HasPrefix(tag, "k") },
		} {
			expr.Match(has)
		}
	})
}

// FuzzParseIsStableUnderRepetition: parsing is a pure function of the string, so
// two parses of the same input must agree on every tag set. A parser that
// consumed shared state would answer differently the second time.
func FuzzParseIsStableUnderRepetition(f *testing.F) {
	for _, seed := range []string{"kafka", "kafka,!consumer|urgent", "!a|!b", "a,b,c"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		first, err1 := Parse(s)
		second, err2 := Parse(s)
		if (err1 == nil) != (err2 == nil) {
			t.Fatalf("Parse(%q) succeeded once and failed once", s)
		}
		if err1 != nil {
			return
		}
		for _, tags := range [][]string{{}, {"kafka"}, {"consumer"}, {"kafka", "consumer", "urgent"}} {
			has := func(tag string) bool { return slices.Contains(tags, tag) }
			if first.Match(has) != second.Match(has) {
				t.Fatalf("Parse(%q) matched %v differently on two parses", s, tags)
			}
		}
	})
}
