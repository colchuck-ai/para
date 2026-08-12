package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestArchivedFlagRegisteredOnTheThirteen is R7: every command that reads an
// address takes --archived without cobra reporting "unknown flag" — checked
// structurally, since these commands go on to open a real tree in RunE and
// this test asserts nothing about that.
func TestArchivedFlagRegisteredOnTheThirteen(t *testing.T) {
	constructors := map[string]func() *cobra.Command{
		"show":     newShowCmd,
		"list":     newListCmd,
		"log":      newLogCmd,
		"activity": newActivityCmd,
		"path":     newPathCmd,
		"doctor":   newDoctorCmd,
		"rebuild":  newRebuildCmd,
		"review":   newReviewCmd,
		"set":      newSetCmd,
		"unset":    newUnsetCmd,
		"note":     newNoteCmd,
		"move":     newMoveCmd,
		"remove":   newRemoveCmd,
	}
	for name, ctor := range constructors {
		t.Run(name, func(t *testing.T) {
			cmd := ctor()
			if cmd.Flags().Lookup("archived") == nil {
				t.Errorf("%s: --archived is not registered", name)
			}
		})
	}
}

// TestArchivedFlagRefusedOnTheThree is R8: add, archive, and unarchive each
// refuse --archived with their own named reason rather than cobra's generic
// "unknown flag" — proven by registering it and having RunE refuse before
// ever touching a filesystem, so no tree fixture is needed here.
//
// add is not a case below: since P19.3 it dispatches on the noun, and
// --archived is registered per noun subcommand rather than on the now
// childless-of-its-own-flags top-level `add` — TestAddRefusesArchived in
// add_internal_test.go covers it against newAddNounCmd directly.
func TestArchivedFlagRefusedOnTheThree(t *testing.T) {
	cases := []struct {
		name    string
		ctor    func() *cobra.Command
		args    []string
		wantErr string
	}{
		{
			name:    "archive",
			ctor:    newArchiveCmd,
			args:    []string{"projects.acme", "--archived"},
			wantErr: "the source is live by definition",
		},
		{
			name:    "unarchive",
			ctor:    newUnarchiveCmd,
			args:    []string{"archive.projects.acme", "--archived"},
			wantErr: "the source is archived by definition",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := c.ctor()
			if cmd.Flags().Lookup("archived") == nil {
				t.Fatalf("%s: --archived is not registered (want it registered and refused)", c.name)
			}
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(c.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("%s --archived: want a refusal, got none", c.name)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("%s --archived error = %q, want it to contain %q", c.name, err.Error(), c.wantErr)
			}
			if strings.Contains(err.Error(), "unknown flag") {
				t.Errorf("%s --archived error = %q, refused by cobra rather than by name", c.name, err.Error())
			}
		})
	}
}
