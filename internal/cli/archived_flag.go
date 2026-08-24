package cli

import (
	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// archivedFlag is R7's --archived: a locator qualifier, not a field,
// registered on every command that reads an address, so that "the archived
// place, not the live one" is spelled once rather than nineteen times.
type archivedFlag struct {
	value bool
}

// register attaches --archived, unqualified, to a command that accepts it
// (R7's thirteen: show, list, log, activity, path, doctor, rebuild, review,
// set, unset, note, move, remove — plus suppress and unsuppress, §28.2,
// added on the same footing as note once R7 was otherwise settled).
func (f *archivedFlag) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.value, "archived", false, "the archived place, not the live one")
}

// registerRefused attaches --archived too, so cobra reports why below
// instead of its own generic "unknown flag" — but the command refuses it
// the moment it is given. why names the reason this command's side of the
// archive boundary is already implied (R8): "nothing is created under
// archive/" (add), "the source is live by definition" (archive), "the
// source is archived by definition" (unarchive).
func (f *archivedFlag) registerRefused(cmd *cobra.Command, why string) {
	f.register(cmd)
	cmd.Flags().Lookup("archived").Usage = "refused: " + why
}

// check is registerRefused's other half, called first thing in RunE — before
// any filesystem read — so the refusal names the reason without a tree even
// being found.
func (f archivedFlag) check(why string) error {
	if f.value {
		return paraerr.Newf(paraerr.KindUsage, "--archived is refused here: %s", why)
	}
	return nil
}
