package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/query"
	"github.com/colchuck-ai/para/internal/tagexpr"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/view"
)

// openRead discovers the tree and builds the environment one read command runs
// in, plus the working directory "." resolves against (§14).
func openRead(cmd *cobra.Command) (*view.Env, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", paraerr.Wrap(paraerr.KindInternal, err, "determining working directory")
	}
	root, err := tree.Find(cwd)
	if err != nil {
		return nil, "", err
	}
	return view.NewEnv(root, clock.FromContext(cmd.Context())), cwd, nil
}

// readFlags are the two flags every read command carries: §16.2.1's `--local`
// and §23's `--json`.
type readFlags struct {
	local bool
	json  bool
}

func (f *readFlags) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.local, "local", false, "render timestamps in the reader's zone rather than UTC")
	cmd.Flags().BoolVar(&f.json, "json", false, "print the result as JSON")
}

// zone is the location timestamps render in.
//
// UTC unless asked otherwise, which is §16.2.1's rule and the same one §3.5
// applies to every generated file — with the difference that terminal output is
// "the one place that is negotiable, because nothing compares it and nothing
// commits it". `$PARA_TZ` reaches this through the clock, which is where the
// reader's zone already arrives.
func (f readFlags) zone(env *view.Env) *time.Location {
	if f.local {
		return env.Local
	}
	return time.UTC
}

// filterFlags are §17's table as flags, shared by every command that takes any
// of them so that `--tags` means one thing everywhere.
type filterFlags struct {
	tags     string
	match    string
	status   string
	priority string
	overdue  bool
	direct   bool
	all      bool

	sort    string
	reverse bool
	limit   int
}

func (f *filterFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.tags, "tags", "", "a boolean tag expression: `not` → `and` → `or`, comma for or")
	cmd.Flags().StringVar(&f.match, "match", "", "text to find in a name, description, tags, or journal notes")
	cmd.Flags().StringVar(&f.status, "status", "", "filter on effective status")
	cmd.Flags().StringVar(&f.priority, "priority", "", "one of "+strings.Join(kindmeta.Priorities(), ", "))
	cmd.Flags().BoolVar(&f.overdue, "overdue", false, "only what is open and past its due date")
	cmd.Flags().BoolVar(&f.direct, "direct", false, "only immediate children, counting containers as transparent")
	cmd.Flags().BoolVar(&f.all, "all", false, "include items whose status is terminal")
	cmd.Flags().StringVar(&f.sort, "sort", "", "sort key: "+sortKeyList())
	cmd.Flags().BoolVar(&f.reverse, "reverse", false, "reverse the sort")
	cmd.Flags().IntVar(&f.limit, "limit", 0, "print at most this many")
}

func sortKeyList() string {
	keys := query.SortKeys()
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = string(k)
	}
	return strings.Join(names, ", ")
}

// options turns the flags into a query, validating the two that have grammars
// of their own.
func (f filterFlags) options(scope locator.Locator) (query.Options, error) {
	opts := query.Options{
		Scope: scope,
		Filter: query.Filter{
			Match:    f.match,
			Status:   f.status,
			Priority: f.priority,
			Overdue:  f.overdue,
			Direct:   f.direct,
			All:      f.all,
		},
		Reverse: f.reverse,
		Limit:   f.limit,
	}
	if f.tags != "" {
		expr, err := tagexpr.Parse(f.tags)
		if err != nil {
			return query.Options{}, err
		}
		opts.Filter.Tags = expr
	}
	if f.sort != "" {
		key, err := query.ParseSortKey(f.sort)
		if err != nil {
			return query.Options{}, err
		}
		opts.Sort = key
	}
	if f.limit < 0 {
		return query.Options{}, paraerr.Newf(paraerr.KindValidation, "--limit cannot be negative")
	}
	return opts, nil
}

// dash is what an absent value prints as. §17 fixes it for an undefined pace —
// "prints `—` and sorts last" — and the same reading covers every other value
// a row does not have, since *field absent on this kind* and *field present
// with no value* are treated alike.
const dash = "—"

// table lays out rows in columns sized to the content of the rows printed.
//
// Output never adapts to the terminal: no width probing, no truncation, no
// colour. The same bytes piped as interactive, which is the only way §0.2's
// determinism argument reaches output and the only way §26's examples can be
// golden files. The cost, stated plainly: a deep locator on an 80-column
// terminal wraps, and para will not shorten it.
type table struct {
	rows [][]string
	// indent prefixes every row, for the nested blocks `show` prints.
	indent string
	// notes are continuation lines, keyed by the row they follow. A note hangs
	// under the second column and takes no part in sizing any of them, which is
	// what keeps §16.1's long key-result numbers from widening the name column
	// of every row above it.
	notes map[int]string
}

func (t *table) add(cells ...string) { t.rows = append(t.rows, cells) }

// note attaches a continuation line to the row just added.
func (t *table) note(text string) {
	if t.notes == nil {
		t.notes = map[int]string{}
	}
	t.notes[len(t.rows)-1] = text
}

// write prints the table with two spaces between columns. A trailing empty
// cell contributes no padding, so a row that stops early leaves no trailing
// whitespace — which matters because these are compared byte for byte.
func (t table) write(out io.Writer) {
	widths := make([]int, 0, 8)
	for _, row := range t.rows {
		for i, cell := range row {
			for len(widths) <= i {
				widths = append(widths, 0)
			}
			if w := len([]rune(cell)); w > widths[i] {
				widths[i] = w
			}
		}
	}

	for n, row := range t.rows {
		last := lastNonEmpty(row)
		var b strings.Builder
		b.WriteString(t.indent)
		for i := 0; i <= last; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			b.WriteString(cell)
			if i < last {
				b.WriteString(strings.Repeat(" ", widths[i]-len([]rune(cell))+2))
			}
		}
		fmt.Fprintln(out, strings.TrimRight(b.String(), " "))

		if note, ok := t.notes[n]; ok {
			hang := len(widths)
			if hang > 0 {
				hang = widths[0] + 2
			}
			fmt.Fprintln(out, strings.TrimRight(t.indent+strings.Repeat(" ", hang)+note, " "))
		}
	}
}

func lastNonEmpty(row []string) int {
	last := -1
	for i, cell := range row {
		if cell != "" {
			last = i
		}
	}
	return last
}

// day renders an instant as the calendar day it falls on, in loc — what §16.1
// prints for `created`, `attention`, and `due`.
func day(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return dash
	}
	return t.In(loc).Format("2006-01-02")
}

// instant renders an instant in full, which is what `log` prints: a journal
// line is the record of a moment, and the time of day is the part of it a
// digest throws away.
func instant(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return dash
	}
	return t.In(loc).Format(time.RFC3339)
}

// ago spells a day count as §16.1 does: "31 days ago", and "today" for zero,
// because "0 days ago" is a sentence nobody writes.
func ago(days int) string {
	switch {
	case days < 0:
		return "in the future"
	case days == 0:
		return "today"
	case days == 1:
		return "1 day ago"
	default:
		return strconv.Itoa(days) + " days ago"
	}
}

// until spells the other direction: §16.1's "in 181 days", and the overdue case
// it turns into once the deadline passes.
func until(days int) string {
	switch {
	case days < -1:
		return strconv.Itoa(-days) + " days ago"
	case days == -1:
		return "1 day ago"
	case days == 0:
		return "today"
	case days == 1:
		return "in 1 day"
	default:
		return "in " + strconv.Itoa(days) + " days"
	}
}

// number renders a derived ratio to two places — enough to tell 0.24 from 0.70
// and no more, since §4.2's quantities are estimates of a trajectory.
func number(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) }

// exact renders a configured value as it was written rather than to a fixed
// precision. A `stale-after` of 14 days is a whole number of days and printing
// it as 14.00 would suggest a precision the knob does not have — and §16.1
// prints it back in the spelling a `config set` would take.
func exact(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
