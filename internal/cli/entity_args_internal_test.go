package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// execNote/execRemove/execArchive/execUnarchive run args against a fresh
// command in the cwd chdirToTestTree set up. None of the four dispatches on
// a subcommand — the noun is a plain runtime argument, like move's — so
// there is no HasParent()/Root() gotcha here either.
func execNote(args []string) (string, error)      { return execPlain(newNoteCmd(), args) }
func execRemove(args []string) (string, error)    { return execPlain(newRemoveCmd(), args) }
func execArchive(args []string) (string, error)   { return execPlain(newArchiveCmd(), args) }
func execUnarchive(args []string) (string, error) { return execPlain(newUnarchiveCmd(), args) }
func execShow(args []string) (string, error)      { return execPlain(newShowCmd(), args) }

func execPlain(cmd *cobra.Command, args []string) (string, error) {
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// TestNoteBareNounRefusedAsABucket is R17: note requires an entity, so a
// noun with no chain — the bucket — is refused by name. The chain is given
// as an empty string rather than omitted: noteArgs' custom Args validator
// requires exactly three args unless "." is the first, so a genuinely
// missing chain (two args, first word not ".") is caught earlier, by
// TestNoteTwoArgsWithoutDotIsRefused below, not here.
func TestNoteBareNounRefusedAsABucket(t *testing.T) {
	chdirToTestTree(t)
	_, err := execNote([]string{"project", "", "text goes here"})
	if err == nil {
		t.Fatal("note project <text>: want a refusal (missing chain), got none")
	}
	if !strings.Contains(err.Error(), "project names a bucket, not an entity") {
		t.Errorf("error = %q, want R17's named refusal", err.Error())
	}
}

// TestNoteTwoArgsWithoutDotIsRefused is a regression test for a review
// finding: <noun> <text> (a genuinely missing chain, two args, first word
// not ".") used to reach RunE, where parseAddressArgs happily took the text
// as the chain and left nothing in rest for env.Note's own text argument —
// an index-out-of-range panic on any single-word note. noteArgs now
// refuses this shape before RunE ever runs.
func TestNoteTwoArgsWithoutDotIsRefused(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	_, err := execNote([]string{"project", "done"})
	if err == nil {
		t.Fatal("note project done: want a refusal, got none")
	}
	if strings.Contains(err.Error(), "panic") {
		t.Fatalf("note project done panicked: %v", err)
	}
	if !strings.Contains(err.Error(), `takes "<noun> <chain> <text>" or ". <text>"`) {
		t.Errorf("error = %q, want the named arity refusal", err.Error())
	}
}

// TestNoteThreeArgsWithDotIsRefused is a second regression test from the
// same review round: ". <extra> <text>" (three args, first word ".") used
// to be silently accepted, resolving "." correctly but then taking <extra>
// as the note text and dropping the real <text> entirely — no error, wrong
// text recorded. There is no valid three-arg shape starting with ".": the
// two legal shapes are ". <text>" (two args) and "<noun> <chain> <text>"
// (three args, a real noun first).
func TestNoteThreeArgsWithDotIsRefused(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(cwd, "projects", "acme")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	_, err = execNote([]string{".", "extra", "the real text"})
	if err == nil {
		t.Fatal(`note . extra "the real text": want a refusal, got none`)
	}
	if !strings.Contains(err.Error(), `takes "<noun> <chain> <text>" or ". <text>"`) {
		t.Errorf("error = %q, want the named arity refusal", err.Error())
	}
}

// TestNoteNounAndChainRecordsTheNote is the entity form succeeding.
func TestNoteNounAndChainRecordsTheNote(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	out, err := execNote([]string{"project", "acme", "waiting on the ingest team"})
	if err != nil {
		t.Fatalf("note project acme ...: %v (%s)", err, out)
	}
	if !strings.Contains(out, "noted") || !strings.Contains(out, "project.acme") {
		t.Errorf("note output = %q, want it to confirm the note against project.acme", out)
	}
}

// TestNoteRefusesContainer is R17 applied to the one command whose
// mutate-layer function (Env.Note) does not itself refuse a container —
// unlike move/remove/archive/unarchive, which inherit the refusal from
// Env.relocatable, note needs its own CLI-level check.
func TestNoteRefusesContainer(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")
	mustAddArea(t, "acme") // irrelevant; just needs a tree with a project

	_, err := execNote([]string{"container", "acme.objectives", "a note"})
	if err == nil {
		t.Fatal("note container acme.objectives: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "container is refused") {
		t.Errorf("error = %q, want it to name container's own refusal", err.Error())
	}
}

// TestNoteDotStillWorks is R16 preserved for note, one of its own two named
// examples ("para note . \"text\"").
func TestNoteDotStillWorks(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(cwd, "projects", "acme")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	out, err := execNote([]string{".", "noted from inside"})
	if err != nil {
		t.Fatalf("note . ...: %v (%s)", err, out)
	}
	if !strings.Contains(out, "project.acme") {
		t.Errorf("note output = %q, want it to confirm project.acme", out)
	}
}

// TestRemoveBareNounRefusedAsABucket mirrors note's.
func TestRemoveBareNounRefusedAsABucket(t *testing.T) {
	chdirToTestTree(t)
	_, err := execRemove([]string{"project", "--force"})
	if err == nil {
		t.Fatal("remove project: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "project names a bucket, not an entity") {
		t.Errorf("error = %q, want R17's named refusal", err.Error())
	}
}

// TestRemoveNounAndChainRemoves is the entity form succeeding, exercised
// with --force to skip the interactive confirmation.
func TestRemoveNounAndChainRemoves(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	out, err := execRemove([]string{"project", "acme", "--force"})
	if err != nil {
		t.Fatalf("remove project acme --force: %v (%s)", err, out)
	}
	if !strings.Contains(out, "project.acme") {
		t.Errorf("remove output = %q, want it to confirm project.acme", out)
	}
	if _, err := os.Stat("projects/acme"); !os.IsNotExist(err) {
		t.Errorf("projects/acme still exists after remove")
	}
}

// TestRemoveRefusesContainer confirms remove inherits its container
// refusal from Env.relocatable rather than needing a CLI-level check.
func TestRemoveRefusesContainer(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	_, err := execRemove([]string{"container", "acme.objectives", "--force"})
	if err == nil {
		t.Fatal("remove container acme.objectives: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "container") {
		t.Errorf("error = %q, want it to mention container", err.Error())
	}
}

// TestArchiveNounAndChainArchives is archive's entity form.
func TestArchiveNounAndChainArchives(t *testing.T) {
	chdirToTestTree(t)
	mustAddArea(t, "health")

	out, err := execArchive([]string{"area", "health"})
	if err != nil {
		t.Fatalf("archive area health: %v (%s)", err, out)
	}
	if !strings.Contains(out, "area.health") || !strings.Contains(out, "archive.area.health") {
		t.Errorf("archive output = %q, want both the live and archived address", out)
	}
}

// TestArchiveBareNounRefusedAsABucket mirrors note's and remove's.
func TestArchiveBareNounRefusedAsABucket(t *testing.T) {
	chdirToTestTree(t)
	_, err := execArchive([]string{"area"})
	if err == nil {
		t.Fatal("archive area: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "area names a bucket, not an entity") {
		t.Errorf("error = %q, want R17's named refusal", err.Error())
	}
}

// TestUnarchiveNounAndChainUnarchives is unarchive's entity form, given the
// dotted chain of the already-archived entity.
func TestUnarchiveNounAndChainUnarchives(t *testing.T) {
	chdirToTestTree(t)
	mustAddArea(t, "health")
	archiveArea(t, "health")

	out, err := execUnarchive([]string{"area", "health"})
	if err != nil {
		t.Fatalf("unarchive area health: %v (%s)", err, out)
	}
	if !strings.Contains(out, "area.health") {
		t.Errorf("unarchive output = %q, want it to confirm area.health", out)
	}
}

// TestUnarchiveRefusesDot is R16's exception carried into the new grammar:
// "." resolves against cwd, and unarchive relocates the very thing "."
// would name — you cannot stand in it.
func TestUnarchiveRefusesDot(t *testing.T) {
	chdirToTestTree(t)
	mustAddArea(t, "health")
	archiveArea(t, "health")

	_, err := execUnarchive([]string{".", "health"})
	if err == nil {
		t.Fatal(`unarchive . health: want a refusal, got none`)
	}
}

// TestUnarchiveDotAloneGetsTheNamedRefusal is a review finding: Args
// stayed at RangeArgs(1,2) rather than ExactArgs(2) specifically so a bare
// `unarchive .` reaches parseAddressArgs' own "." is not accepted here"
// message instead of cobra's generic "accepts 2 arg(s), received 1".
func TestUnarchiveDotAloneGetsTheNamedRefusal(t *testing.T) {
	chdirToTestTree(t)

	_, err := execUnarchive([]string{"."})
	if err == nil {
		t.Fatal("unarchive .: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), `"." is not accepted here`) {
		t.Errorf(`error = %q, want %q`, err.Error(), `"." is not accepted here`)
	}
}

// TestUnarchiveRefusesArchivedFlag confirms P19.2's refusal (R8) survives
// the argument-parsing rewiring.
func TestUnarchiveRefusesArchivedFlag(t *testing.T) {
	_, err := execUnarchive([]string{"area", "health", "--archived"})
	if err == nil {
		t.Fatal("unarchive --archived: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "the source is archived by definition") {
		t.Errorf("error = %q, want the named refusal", err.Error())
	}
}
