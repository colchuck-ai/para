package cli

import (
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
)

// usageOf is a field flag's help text on kind's add subcommand.
func usageOf(t *testing.T, kind kindmeta.Kind, field string) string {
	t.Helper()
	cmd := newAddNounCmd(kind)
	fl := cmd.Flags().Lookup(field)
	if fl == nil {
		t.Fatalf("add %s has no --%s flag", kind, field)
	}
	return fl.Usage
}

// assertNoKindName fails if usage names the kind it is already the flag of
// — task P19.4's whole point: once a subcommand is specific to one kind,
// its own flags' help text does not have to repeat which kind it is.
func assertNoKindName(t *testing.T, kind kindmeta.Kind, field string) {
	t.Helper()
	usage := usageOf(t, kind, field)
	named := "a " + kind.String() + "'s"
	if strings.Contains(usage, named) {
		t.Errorf("add %s --%s usage = %q, still names its own kind (%q)", kind, field, usage, named)
	}
	if strings.Contains(usage, "a "+kind.String()) {
		t.Errorf("add %s --%s usage = %q, still names its own kind", kind, field, usage)
	}
}

// TestAddFieldHelpDoesNotNameItsOwnKind covers every field whose original
// text named the one kind that has it — type, start, and target (all
// key-result's) and scope (skill's).
func TestAddFieldHelpDoesNotNameItsOwnKind(t *testing.T) {
	assertNoKindName(t, kindmeta.KindKeyResult, "type")
	assertNoKindName(t, kindmeta.KindKeyResult, "start")
	assertNoKindName(t, kindmeta.KindKeyResult, "target")
	assertNoKindName(t, kindmeta.KindSkill, "scope")
}

// TestAddKeyResultStatusOffersOnlyItsSettableValue is the task's other
// example: the "a key-result takes only dropped" footnote disappears
// entirely on add's key-result subcommand, because that subcommand simply
// does not register the four values it cannot take — it registers dropped
// alone.
func TestAddKeyResultStatusOffersOnlyItsSettableValue(t *testing.T) {
	usage := usageOf(t, kindmeta.KindKeyResult, "status")
	if usage != "one of dropped" {
		t.Errorf("add key-result --status usage = %q, want %q", usage, "one of dropped")
	}
}

// TestAddProjectStatusOffersAllFiveWithNoKeyResultFootnote is the same
// property from the other side: a kind that genuinely has all five values
// should not carry a footnote about a different kind's restriction.
func TestAddProjectStatusOffersAllFiveWithNoKeyResultFootnote(t *testing.T) {
	usage := usageOf(t, kindmeta.KindProject, "status")
	want := "one of " + strings.Join(kindmeta.AllStatuses(), ", ")
	if usage != want {
		t.Errorf("add project --status usage = %q, want %q", usage, want)
	}
	if strings.Contains(usage, "key-result") {
		t.Errorf("add project --status usage = %q, mentions key-result", usage)
	}
}
