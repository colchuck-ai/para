package config

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptoml"
)

// File is one .para/config.toml: a set of dotted keys and their values, held
// in lexical order.
//
// Lexical order is the whole ordering rule, for both the keys para knows and
// the ones it does not. §0.2 forbids map iteration in output, and a config
// file is one a human edits and git diffs, so one declared order — rather
// than the order keys happened to be set in — is what keeps two people who
// set the same knobs from producing different bytes.
type File struct {
	entries []entry
}

type entry struct {
	Key   string
	Value ptoml.Value
}

// Decode parses one config.toml.
//
// Every key that carries a value survives, including ones para does not
// recognise: a key written by a newer version must not be deleted by an older
// version's `config set` (§7 puts no version fence around the file), and an
// unrecognised key is doctor's `invalid` finding to report (§10). The one
// thing Decode refuses is a value it cannot carry — a TOML datetime, a mixed
// array — because accepting it would mean dropping it silently at the next
// rewrite, and a refusal the user can see beats data loss they cannot.
//
// What does not survive is a `[table]` header with nothing under it: the file
// is rewritten as flat dotted keys, and an empty table states no fact to
// carry over.
func Decode(data []byte) (File, error) {
	doc, err := ptoml.Decode(data)
	if err != nil {
		return File{}, err
	}
	keys, err := doc.Keys()
	if err != nil {
		return File{}, err
	}
	var f File
	for _, key := range keys {
		v, ok := doc.Value(key)
		if !ok {
			return File{}, paraerr.Newf(paraerr.KindValidation,
				"config key %q holds a value type para cannot carry (strings, numbers, booleans, and string arrays only)", key)
		}
		f.entries = append(f.entries, entry{Key: key, Value: v})
	}
	return f, nil
}

// Encode renders the file, one `key = value` line per entry in lexical
// order. An empty file encodes to no bytes.
func (f File) Encode() ([]byte, error) {
	fields := make([]ptoml.Field, 0, len(f.entries))
	for _, e := range f.entries {
		fields = append(fields, ptoml.Field{Key: e.Key, Value: e.Value})
	}
	return ptoml.Encode(fields)
}

// Get returns key's value and whether the file sets it.
func (f File) Get(key string) (ptoml.Value, bool) {
	if i := f.index(key); i >= 0 {
		return f.entries[i].Value, true
	}
	return ptoml.Value{}, false
}

// Set stores v under key and reports whether that changed anything. Setting
// a key to the value it already holds is a no-op — §3.1's rule, which is why
// `config set` can honestly say "no change" and write nothing.
//
// It refuses a key that cannot coexist with one the file already has. TOML
// makes `emit` a value and `emit.claude` a value inside a table named `emit`,
// and one file cannot hold both: writing them anyway produces a file that
// parses nowhere, including in para. That only arises from a hand edit — para
// itself never writes a bare `emit` — but a hand edit is exactly the case
// where the next `config set` must not make things worse.
func (f *File) Set(key string, v ptoml.Value) (bool, error) {
	if i := f.index(key); i >= 0 {
		if equal(f.entries[i].Value, v) {
			return false, nil
		}
		f.entries[i].Value = v
		return true, nil
	}
	if clash, ok := f.tableClash(key); ok {
		return false, paraerr.Newf(paraerr.KindConflict,
			"cannot set %q: this file already has %q, and TOML cannot hold both a value and a table under the same name", key, clash)
	}
	f.entries = append(f.entries, entry{Key: key, Value: v})
	slices.SortFunc(f.entries, func(a, b entry) int {
		switch {
		case a.Key < b.Key:
			return -1
		case a.Key > b.Key:
			return 1
		default:
			return 0
		}
	})
	return true, nil
}

// tableClash reports an existing key that TOML cannot hold alongside key:
// one is a strict dotted prefix of the other, so the shared segment would
// have to be a value on one line and a table on the next.
func (f File) tableClash(key string) (string, bool) {
	for _, e := range f.entries {
		if strings.HasPrefix(key, e.Key+".") || strings.HasPrefix(e.Key, key+".") {
			return e.Key, true
		}
	}
	return "", false
}

// Unset removes key and reports whether it was there to remove.
func (f *File) Unset(key string) bool {
	i := f.index(key)
	if i < 0 {
		return false
	}
	f.entries = slices.Delete(f.entries, i, i+1)
	return true
}

// Keys returns the keys the file sets, in lexical order.
func (f File) Keys() []string {
	out := make([]string, 0, len(f.entries))
	for _, e := range f.entries {
		out = append(out, e.Key)
	}
	return out
}

// Len is how many keys the file sets.
func (f File) Len() int { return len(f.entries) }

// equal compares two values including their kind, so 14 and "14" are
// different stored facts and setting one over the other is a change. A
// ptoml.Value is not comparable with == because it carries a slice.
func equal(a, b ptoml.Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == ptoml.KindStringArray {
		return slices.Equal(a.StrArray, b.StrArray)
	}
	// Floats compare by their bits, not by ==, because the question is
	// whether the file would change: -0.0 and 0.0 are equal numbers that
	// write different bytes, and two NaNs are unequal numbers that write
	// the same ones.
	if math.Float64bits(a.Float) != math.Float64bits(b.Float) {
		return false
	}
	return a.Str == b.Str && a.Int == b.Int && a.Bool == b.Bool
}

func (f File) index(key string) int {
	for i, e := range f.entries {
		if e.Key == key {
			return i
		}
	}
	return -1
}

// Read loads the config.toml at path. An absent file is an empty config
// rather than an error: most levels of a chain have no config.toml, and
// resolution walks every level whether or not one exists (§7).
func Read(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, nil
		}
		return File{}, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", path))
	}
	f, err := Decode(data)
	if err != nil {
		return File{}, paraerr.Wrap(paraerr.KindValidation, err, fmt.Sprintf("reading %s", path))
	}
	return f, nil
}
