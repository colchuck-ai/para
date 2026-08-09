package config

import (
	"errors"
	"slices"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptoml"
)

// Every row of §7's config table, and nothing else.
func TestSpecsIsExactlySection7sTable(t *testing.T) {
	want := []string{
		"area.stale-after",
		"emit.claude",
		"emit.claude-skills",
		"emit.gitattributes",
		"key-result.at-risk-pace",
		"key-result.stale-after",
		"log.rotate-bytes",
		"objective.stale-after",
		"project.stale-after",
		"resource.stale-after",
		"review.cadence",
	}

	var got []string
	for _, s := range Specs() {
		got = append(got, s.Key)
	}
	if !slices.Equal(got, want) {
		t.Errorf("Specs() keys =\n%v\nwant\n%v", got, want)
	}
}

func TestSpecDefaults(t *testing.T) {
	tests := []struct {
		key        string
		hasDefault bool
		want       string // the display form of the default
	}{
		// §7's four defaults, and the only four keys that have one.
		{key: "emit.claude", hasDefault: true, want: "false"},
		{key: "emit.claude-skills", hasDefault: true, want: "symlink"},
		{key: "emit.gitattributes", hasDefault: true, want: "true"},
		{key: "log.rotate-bytes", hasDefault: true, want: "4194304"},
		// §7: "Unset everywhere means the check never fires" — so no
		// threshold carries a default, or the check would fire on a tree
		// that never asked for it.
		{key: "project.stale-after", hasDefault: false},
		{key: "area.stale-after", hasDefault: false},
		{key: "resource.stale-after", hasDefault: false},
		{key: "objective.stale-after", hasDefault: false},
		{key: "key-result.stale-after", hasDefault: false},
		{key: "key-result.at-risk-pace", hasDefault: false},
		{key: "review.cadence", hasDefault: false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			spec, ok := Lookup(tt.key)
			if !ok {
				t.Fatalf("Lookup(%q) not found", tt.key)
			}
			v, ok := spec.DefaultValue()
			if ok != tt.hasDefault {
				t.Fatalf("DefaultValue() ok = %v, want %v", ok, tt.hasDefault)
			}
			if ok && Format(v) != tt.want {
				t.Errorf("default = %q, want %q", Format(v), tt.want)
			}
		})
	}
}

func TestLookupRejectsAnUnknownKey(t *testing.T) {
	for _, key := range []string{
		"project.stale-aftr",   // a typo
		"emit.claude_skills",   // the wrong separator
		"projects.stale-after", // the bucket, not the kind
		"skill.stale-after",    // a skill's threshold is review.cadence (§20)
		"container.stale-after",
		"",
	} {
		if _, ok := Lookup(key); ok {
			t.Errorf("Lookup(%q) = found, want not found", key)
		}
	}
}

func TestParseValueByDeclaredType(t *testing.T) {
	tests := []struct {
		key     string
		raw     string
		want    string // display form after a round trip through Parse
		wantErr bool
	}{
		{key: "project.stale-after", raw: "30", want: "30"},
		{key: "project.stale-after", raw: "0", want: "0"},
		{key: "project.stale-after", raw: "30.5", wantErr: true},
		{key: "project.stale-after", raw: "soon", wantErr: true},
		{key: "project.stale-after", raw: "-1", wantErr: true}, // a negative window is not a window
		{key: "log.rotate-bytes", raw: "4194304", want: "4194304"},

		{key: "key-result.at-risk-pace", raw: "0.8", want: "0.8"},
		{key: "key-result.at-risk-pace", raw: "1", want: "1.0"}, // a float stays a float (§0.2)
		{key: "key-result.at-risk-pace", raw: "fast", wantErr: true},

		{key: "emit.claude", raw: "true", want: "true"},
		{key: "emit.claude", raw: "false", want: "false"},
		{key: "emit.claude", raw: "yes", wantErr: true}, // TOML's spelling, not shell's
		{key: "emit.claude", raw: "1", wantErr: true},
		{key: "emit.claude", raw: "True", wantErr: true},

		{key: "emit.claude-skills", raw: "symlink", want: "symlink"},
		{key: "emit.claude-skills", raw: "copy", want: "copy"},
		{key: "emit.claude-skills", raw: "hardlink", wantErr: true},
		{key: "emit.claude-skills", raw: `"copy"`, wantErr: true}, // the shell's quotes are not the value
	}

	for _, tt := range tests {
		t.Run(tt.key+"="+tt.raw, func(t *testing.T) {
			spec, ok := Lookup(tt.key)
			if !ok {
				t.Fatalf("Lookup(%q) not found", tt.key)
			}
			v, err := spec.Parse(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %v, want an error", tt.raw, Format(v))
				}
				var perr *paraerr.Error
				if !errors.As(err, &perr) || perr.Kind != paraerr.KindValidation {
					t.Errorf("Parse(%q) error = %v, want a %v error", tt.raw, err, paraerr.KindValidation)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.raw, err)
			}
			if got := Format(v); got != tt.want {
				t.Errorf("Parse(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// A value read back from a file is not one Parse produced — it may have been
// hand-edited to the wrong TOML type — so Check is what stands between a
// mistyped knob and code that reads it as if it were right.
func TestCheckRejectsAWrongTypedStoredValue(t *testing.T) {
	tests := []struct {
		key     string
		value   ptoml.Value
		wantErr bool
	}{
		{key: "project.stale-after", value: ptoml.Int64(30)},
		{key: "project.stale-after", value: ptoml.String("30"), wantErr: true},
		{key: "project.stale-after", value: ptoml.Float64(30), wantErr: true},
		{key: "emit.claude", value: ptoml.Bool(true)},
		{key: "emit.claude", value: ptoml.String("true"), wantErr: true},
		{key: "emit.claude-skills", value: ptoml.String("copy")},
		{key: "emit.claude-skills", value: ptoml.String("hardlink"), wantErr: true},
		{key: "emit.claude-skills", value: ptoml.Bool(true), wantErr: true},
		{key: "key-result.at-risk-pace", value: ptoml.Float64(0.8)},
		// An integer where a float belongs is the one coercion worth
		// allowing: TOML reads `at-risk-pace = 1` as an integer, and
		// refusing it would make a legal-looking file unreadable.
		{key: "key-result.at-risk-pace", value: ptoml.Int64(1)},
		{key: "key-result.at-risk-pace", value: ptoml.String("0.8"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.key+"/"+Format(tt.value), func(t *testing.T) {
			spec, ok := Lookup(tt.key)
			if !ok {
				t.Fatalf("Lookup(%q) not found", tt.key)
			}
			err := spec.Check(tt.value)
			if tt.wantErr && err == nil {
				t.Errorf("Check(%v) = nil, want an error", Format(tt.value))
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Check(%v) = %v, want nil", Format(tt.value), err)
			}
		})
	}
}

// §20 is explicit that a skill's staleness threshold is review.cadence and not
// stale-after, and that containers have no threshold at all. Deriving the key
// from the kind keeps that in one place rather than in review's five groups.
func TestStaleKeyPerKind(t *testing.T) {
	tests := []struct {
		kind kindmeta.Kind
		want string
	}{
		{kindmeta.KindProject, "project.stale-after"},
		{kindmeta.KindArea, "area.stale-after"},
		{kindmeta.KindResource, "resource.stale-after"},
		{kindmeta.KindObjective, "objective.stale-after"},
		{kindmeta.KindKeyResult, "key-result.stale-after"},
		{kindmeta.KindSkill, "review.cadence"},
		{kindmeta.KindContainer, ""},
		{kindmeta.KindUnknown, ""},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			got, ok := StaleKey(tt.kind)
			if got != tt.want || ok != (tt.want != "") {
				t.Errorf("StaleKey(%v) = %q, %v; want %q, %v", tt.kind, got, ok, tt.want, tt.want != "")
			}
			if got != "" {
				if _, found := Lookup(got); !found {
					t.Errorf("StaleKey(%v) = %q, which is not a registered key", tt.kind, got)
				}
			}
		})
	}
}

// Format is what every output shape prints — §22's chain, §16.1's provenance
// note, and --json's fallback — so it must never carry TOML's quoting into a
// terminal.
func TestFormatIsTheUnquotedDisplayForm(t *testing.T) {
	tests := []struct {
		value ptoml.Value
		want  string
	}{
		{ptoml.Int64(30), "30"},
		{ptoml.Int64(4194304), "4194304"},
		{ptoml.Float64(0.8), "0.8"},
		{ptoml.Float64(1), "1.0"},
		{ptoml.Bool(true), "true"},
		{ptoml.Bool(false), "false"},
		{ptoml.String("symlink"), "symlink"},
	}
	for _, tt := range tests {
		if got := Format(tt.value); got != tt.want {
			t.Errorf("Format(%v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}
