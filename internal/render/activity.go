package render

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptime"
)

// ActivityRenderer renders ACTIVITY.md, the human-readable digest of one
// entity's own journal, grouped by day, newest day first (§3.5).
//
// Days selects the mode, and the two modes are the reason this type is
// exported while every other renderer is not:
//
//   - Days empty is **full** mode. Every section is derived from In.Events,
//     which must be the entity's whole history across every rotated file. This
//     is what rebuild writes and what doctor compares against (§10, §21.1).
//
//   - Days non-empty is **incremental** mode, the write path. Only those days'
//     sections are re-derived; every other day is copied out of
//     In.Existing byte for byte, because it was already written when it was
//     today and prior days are never recomputed (§3.5). In.Events must contain
//     every event falling on those days.
//
// For the same history the two modes produce identical bytes. That equivalence
// is what makes the cheap write safe and what gives §10's drift report a day to
// name; it is asserted as a property test rather than assumed.
//
// **A key-result must use full mode**, and asking for incremental is an error
// rather than a caveat. Every measurement line ends in "N% of target", a
// function of `start` and `target` — both settable (§15) — and of the oldest
// reading, which is the default baseline (§4.1). So `set --target`, or a
// measurement backdated before every existing one, changes lines on days
// incremental mode would never revisit. Refusing costs nothing: rewriting the
// same mutation's MEASUREMENTS.csv already requires the whole measurement
// history (§2.3, §4.4), so for a key-result there is no cheap read to protect.
type ActivityRenderer struct {
	Days []string

	// Since drops every day section before this UTC day, and is `activity
	// --since`'s (§16.4). It is a narrowing of what is *shown*, applied after
	// every line is derived, so a key-result's measurement lines keep the
	// baseline the whole history gives them (§4.1) rather than acquiring a new
	// one from the oldest reading that survived the filter.
	//
	// **It is read-only, and nothing on a write path may set it.** A truncated
	// ACTIVITY.md is a projection that disagrees with its journal, which is
	// exactly what doctor exists to report (§10). Full mode only: it is refused
	// alongside Days, since a filtered incremental splice would delete prior
	// days from a file rather than hide them from a reader.
	Since string
}

// activityFile is the filename, at every level that has one (§1.1).
const activityFile = "ACTIVITY.md"

// activityTitle is the file's first line. ACTIVITY.md is wholly generated
// (§2.2), so there is no human-owned heading to preserve.
const activityTitle = "# Activity\n"

func (ActivityRenderer) Path(in In) (string, error) {
	dir, err := in.dir()
	if err != nil {
		return "", err
	}
	return join(dir, activityFile), nil
}

func (r ActivityRenderer) Render(in In) ([]byte, error) {
	derived, err := activitySections(in)
	if err != nil {
		return nil, err
	}
	if len(r.Days) == 0 {
		return renderActivity(sinceSections(derived, r.Since)), nil
	}
	if r.Since != "" {
		return nil, paraerr.New(paraerr.KindInternal,
			"render: ACTIVITY.md's Since is a read-time narrowing and cannot be combined with incremental mode")
	}
	if in.Kind == kindmeta.KindKeyResult {
		return nil, paraerr.Newf(paraerr.KindInternal,
			"render: %s must be rendered in full for a key-result, whose measurement lines depend on start, target, and the oldest reading",
			activityFile)
	}

	path, err := Activity.Path(in)
	if err != nil {
		return nil, err
	}
	prior, err := parseActivity(in.existing(path))
	if err != nil {
		return nil, err
	}
	return renderActivity(spliceSections(prior, derived, r.Days)), nil
}

// section is one day's worth of the file: its date header and the rendered
// lines beneath it, each newline-terminated.
type section struct {
	Day  string
	Body []byte
}

// renderActivity assembles sections newest-day-first. The layout is fixed —
// title, then one blank line before each day header — because parseActivity has
// to reproduce it exactly for the incremental path to be byte-equivalent to the
// full one.
func renderActivity(sections []section) []byte {
	var b bytes.Buffer
	b.WriteString(activityTitle)
	for _, s := range sections {
		b.WriteString("\n## ")
		b.WriteString(s.Day)
		b.WriteString("\n")
		b.Write(s.Body)
	}
	return b.Bytes()
}

var dayHeaderPattern = regexp.MustCompile(`^## (\d{4}-\d{2}-\d{2})$`)

// parseActivity recovers the day sections of an existing ACTIVITY.md, keeping
// each one's lines exactly as found. What it deliberately does not recover is
// the structure between sections: the title and the blank separator lines are
// re-emitted by renderActivity from the fixed layout, so a file whose
// separators drifted is normalised rather than perpetuated.
//
// CRLF is normalised to LF. ACTIVITY.md is wholly generated (§2.2), so unlike
// README.md there is no human-authored byte here to preserve — which makes
// normalising safe, and makes it necessary: a checkout with git's
// core.autocrlf on hands back a CRLF file, and a parser that failed to
// recognise its day headers would drop every prior day on the next mutation.
//
// Anything else this parser cannot account for is refused rather than dropped.
// Silently discarding a line is the one failure mode that loses history, and
// history is the only thing in this file worth anything; refusing names the
// repair instead (§2.4: doctor reports, rebuild repairs).
func parseActivity(data []byte) ([]section, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	normalised := strings.ReplaceAll(string(data), "\r\n", "\n")

	var sections []section
	var current *section
	for _, line := range strings.SplitAfter(normalised, "\n") {
		if line == "" {
			continue
		}
		text := strings.TrimSuffix(line, "\n")
		if m := dayHeaderPattern.FindStringSubmatch(text); m != nil {
			sections = append(sections, section{Day: m[1]})
			current = &sections[len(sections)-1]
			continue
		}
		if current == nil {
			// Before the first day header only the title and blank lines
			// belong. Anything else is content this parser would silently
			// lose on re-render.
			if trimmed := strings.TrimSpace(text); trimmed != "" && trimmed+"\n" != activityTitle {
				return nil, paraerr.Newf(paraerr.KindValidation,
					"%s has content before its first day heading (%q) — run `para rebuild` to regenerate it",
					activityFile, trimmed)
			}
			continue
		}
		current.Body = append(current.Body, line...)
	}

	// An empty section is dropped rather than carried: §3.5 says days with no
	// events are absent, and full mode never emits one.
	kept := sections[:0]
	for _, s := range sections {
		if s.Body = normalizeSectionBody(s.Body); len(s.Body) > 0 {
			kept = append(kept, s)
		}
	}
	sections = kept

	if err := assertDescending(sections); err != nil {
		return nil, err
	}
	return sections, nil
}

// normalizeSectionBody reduces a parsed section body to exactly its lines: the
// blank separator that precedes the next day header is dropped, a body of
// nothing but blank lines becomes empty, and a final line missing its newline
// gains one.
//
// All three matter for byte equivalence with full mode. Full mode never emits a
// blank line at the end of a section and never emits an empty section at all, so
// a body left holding either would be a permanent difference between the two
// modes on a file neither of them would have written.
func normalizeSectionBody(body []byte) []byte {
	trimmed := bytes.TrimRight(body, "\n")
	if len(trimmed) == 0 {
		return nil
	}
	out := make([]byte, len(trimmed)+1)
	copy(out, trimmed)
	out[len(trimmed)] = '\n'
	return out
}

func assertDescending(sections []section) error {
	for i := 1; i < len(sections); i++ {
		if sections[i-1].Day <= sections[i].Day {
			return paraerr.Newf(paraerr.KindValidation,
				"%s is not newest-day-first: %s precedes %s", activityFile, sections[i-1].Day, sections[i].Day)
		}
	}
	return nil
}

// spliceSections replaces the sections for days with the freshly derived ones
// and leaves every other section's bytes untouched. A day in days with nothing
// derived for it loses its section entirely, which is how §3.5's "days with no
// events are absent" stays true after a rebuild removes something.
func spliceSections(prior, derived []section, days []string) []section {
	target := make(map[string]bool, len(days))
	for _, d := range days {
		target[d] = true
	}

	// Both loops walk slices, never the map: the map is only ever asked "is
	// this day being re-derived", so no map's iteration order reaches the
	// output (§0.2). `derived` is already newest-first, and `prior` is checked
	// to be by parseActivity, so the merge below preserves that ordering
	// without needing to re-sort.
	out := make([]section, 0, len(prior)+len(derived))
	for _, s := range prior {
		if !target[s.Day] {
			out = append(out, s)
		}
	}
	for _, s := range derived {
		if target[s.Day] {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Day > out[j].Day })
	return out
}

// activitySections folds in.Events into day sections, newest day first and
// newest line first within a day — one direction throughout, matching `log`'s
// so the two never read in opposite orders (§16.3).
//
// The subject's `created` contributes a line of its own. `created` is a field
// in the truth file rather than an event (§3.1), but the digest is the answer to
// "what has happened here", and coming into existence is the first thing that
// happened: §26's own `activity` output ends with a `created` line, and §16.4
// makes ACTIVITY.md and `activity` agree by construction. Its consequence is
// worth stating, because it is a real constraint on the write path: changing
// `created` moves that line to a different day, so a mutation that changes it
// must re-derive both days' sections, not just today's.
func activitySections(in In) ([]section, error) {
	days, err := digest(in, markdown)
	if err != nil {
		return nil, err
	}
	out := make([]section, 0, len(days))
	for _, day := range days {
		var b bytes.Buffer
		for _, line := range day.Lines {
			b.WriteString("- ")
			b.WriteString(line.Text)
			b.WriteString("\n")
		}
		out = append(out, section{Day: day.Date, Body: b.Bytes()})
	}
	return out, nil
}

// Day is one day of the digest: the UTC day, and its lines newest first.
type Day struct {
	Date  string
	Lines []Line
}

// Line is one event as the digest states it.
type Line struct {
	// At is the event's instant, or the zero time for the `created` line, which
	// comes from a field rather than an event (§3.1).
	At time.Time
	// Kind is the event kind, or "" for the `created` line — what `log --kind`
	// filters on and what `--json` reports.
	Kind journal.Kind
	// Text is the line itself, in the style Digest was asked for.
	Text string
}

// Digest is the fold ACTIVITY.md contains, as data rather than as a file
// (§3.5, §16.4).
//
// It exists because `activity --recursive` merges several entities' digests
// into one chronology with a locator per line, which is not a shape any single
// file has — but the lines have to be the same lines, or §16.4's "they agree by
// construction" would be a claim rather than a fact. So the templates live in
// one place and the two callers differ only in spelling: the file is markdown,
// the terminal is not.
//
// The lines are plain text: no emphasis markers, no terminating period, and a
// lower-case opening, which is what §26's `activity --recursive` output shows.
func Digest(in In) ([]Day, error) { return digest(in, plain) }

// digest folds in.Events into days, newest day first and newest line first
// within a day — one direction throughout, matching `log`'s so the two never
// read in opposite orders (§16.3).
//
// The subject's `created` contributes a line of its own. `created` is a field
// in the truth file rather than an event (§3.1), but the digest is the answer to
// "what has happened here", and coming into existence is the first thing that
// happened: §26's own `activity` output ends with a `created` line, and §16.4
// makes ACTIVITY.md and `activity` agree by construction. Its consequence is
// worth stating, because it is a real constraint on the write path: changing
// `created` moves that line to a different day, so a mutation that changes it
// must re-derive both days' sections, not just today's.
func digest(in In, style lineStyle) ([]Day, error) {
	byDay := map[string][]Line{}
	dates := []string{}
	addLine := func(day string, line Line) {
		if _, seen := byDay[day]; !seen {
			dates = append(dates, day)
		}
		byDay[day] = append(byDay[day], line)
	}

	rs := readings(in)
	events := slices.Clone(in.Events)
	sort.SliceStable(events, func(i, j int) bool { return events[j].At.Before(events[i].At) })
	for _, e := range events {
		text, err := eventLine(in, e, rs, style)
		if err != nil {
			return nil, err
		}
		addLine(utcDay(e.At), Line{At: e.At, Kind: e.Kind, Text: text})
	}

	if created := createdDay(in); created != "" {
		addLine(created, Line{Text: style.finish(createdLine)})
	}

	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	out := make([]Day, 0, len(dates))
	for _, date := range dates {
		out = append(out, Day{Date: date, Lines: byDay[date]})
	}
	return out, nil
}

// dayLayout is the date form the day headers use.
const dayLayout = "2006-01-02"

// utcDay is the day an instant falls on, in UTC.
//
// Every day heading in a generated file is a UTC day, and the reason is
// §3.1's: "ordering comes from `at`, never from file position". Group by each
// event's own recorded offset instead and the sections stop partitioning the
// timeline — an event at 2026-03-05T23:00-08:00 (07:00Z) files under 03-05
// while an *earlier* event at 2026-03-06T09:00+09:00 (00:00Z) files under
// 03-06, so a newest-day-first file presents the earlier event as the newer
// one. UTC boundaries are the only ones every event agrees on, and this file
// is read by people in several zones from the same commit, so agreement is the
// requirement.
//
// The wall clock is not lost, only moved: journals keep each event's offset, so
// a read command can convert for display on request.
func utcDay(t time.Time) string {
	return t.UTC().Format(dayLayout)
}

// createdLine is the digest's one line that comes from a field rather than an
// event. It is written unterminated, like every other core, and the style
// finishes it.
const createdLine = "Created"

// lineStyle is the difference between the two places a digest line is printed:
// ACTIVITY.md, which is markdown and is committed, and the terminal, which is
// neither (§16.2.1's argument for `--local` applies to spelling too).
//
// It is a style and not two sets of templates, because §16.4 requires the file
// and the command to agree, and two copies of six templates would agree only
// until one of them was edited.
type lineStyle struct {
	// emphasise wraps a field or child name in markdown bold.
	emphasise bool
	// sentence gives the line a capital opening and a terminating period.
	sentence bool
}

var (
	markdown = lineStyle{emphasise: true, sentence: true}
	plain    = lineStyle{}
)

// emph marks a field or child name, or leaves it alone.
func (s lineStyle) emph(text string) string {
	if s.emphasise {
		return "**" + text + "**"
	}
	return text
}

// finish spells a completed line. The cores are written as sentences with a
// capital opening, so plain style lowers it rather than every template
// carrying two spellings of its first word.
func (s lineStyle) finish(core string) string {
	if s.sentence {
		return sentence(core)
	}
	return lowerFirst(core)
}

// lowerFirst lowers the first rune of s, leaving the rest — including a proper
// noun later in the line — alone.
func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// CreatedDay is the UTC day the subject's `created` falls on, or "" if there is
// no created field to read.
//
// It is exported because the incremental path cannot be used correctly without
// it. The `created` line lives in whichever day section `created` names, and
// incremental mode re-derives only the days it is given — so a mutation must
// include this day in Days whenever it is writing ACTIVITY.md for the first
// time (`add`, §18.1) or changing `created` itself (§15). Omit it and the file
// loses a line the full re-derivation would produce, which doctor would then
// correctly report as drift.
func CreatedDay(in In) string { return createdDay(in) }

// createdDay is the UTC day the subject's `created` falls on, or "" if there is
// no created field to read (a stub, or a truth file doctor will report as
// invalid).
//
// `created` is stored in progressive precision (§15.1), so it is parsed rather
// than sliced: only the full form carries the offset that decides which UTC day
// it lands on. A form with no offset is taken as UTC, which is the only reading
// available once no offset was recorded — and the one `add` avoids by resolving
// `--created` to a full timestamp at write time.
func createdDay(in In) string {
	created := in.State.Created
	if in.shape() == shapeRoot {
		created = in.Tree.Created
	}
	if created == "" {
		return ""
	}
	t, err := ptime.ParseAt(created, time.UTC)
	if err != nil {
		return ""
	}
	return utcDay(t)
}

// DaysOf returns the days the given events fall on, deduplicated — the argument
// a mutation passes to ActivityRenderer.Days after appending them.
func DaysOf(events []journal.Event) []string {
	seen := map[string]bool{}
	var days []string
	for _, e := range events {
		day := utcDay(e.At)
		if !seen[day] {
			seen[day] = true
			days = append(days, day)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(days)))
	return days
}

// eventLine renders one event as its ACTIVITY.md line, without the leading
// "- " or the trailing newline.
//
// A `note` is a field on every kind (§3.1), so any event can carry a reason;
// it is appended to the sentence with an em dash rather than given a line of
// its own, so one mutation stays one line.
func eventLine(in In, e journal.Event, rs []reading, style lineStyle) (string, error) {
	var core string
	switch e.Kind {
	case journal.KindChange:
		core = changeLine(e, style)
	case journal.KindMeasurement:
		core = measurementLine(e, rs)
	case journal.KindNote:
		return style.finish("Note: " + flatten(e.Note)), nil
	case journal.KindChild:
		core = childLine(in, e, style)
	default:
		return "", paraerr.Newf(paraerr.KindValidation, "unknown journal event kind %q", e.Kind)
	}
	if note := flatten(e.Note); note != "" {
		core += " — " + note
	}
	return style.finish(core), nil
}

func changeLine(e journal.Event, style lineStyle) string {
	field := style.emph(flatten(e.Field))
	switch {
	case e.From == "" && e.To != "":
		return fmt.Sprintf("Set %s to %s", field, flatten(e.To))
	case e.From != "" && e.To == "":
		return fmt.Sprintf("Unset %s (was %s)", field, flatten(e.From))
	default:
		return fmt.Sprintf("Changed %s from %s to %s", field, flatten(e.From), flatten(e.To))
	}
}

// measurementLine renders §3.5's measurement line: the reading as logged, the
// decimal in parentheses where the reading is not already one, and the progress
// it represents. A reading whose arithmetic did not work out prints alone.
func measurementLine(e journal.Event, rs []reading) string {
	core := "Measured " + flatten(e.Value)
	r, ok := readingAt(rs, e)
	if !ok || !r.HasDerived {
		return core
	}
	// A ratio's decimal is worth printing because the reading itself is not
	// one; a number already is its own decimal, and a boolean has nothing to
	// say (§4.1).
	if r.Value.Type == krvalue.TypeRatio {
		core += " (" + percent(r.Decimal, 1) + ")"
	}
	return core + " — " + percent(r.Progress, 0) + " of target"
}

// childLine renders a containment event, which is the parent's own event
// because the parent genuinely changed: it has a different set of children than
// it did (§3.3).
func childLine(in In, e journal.Event, style lineStyle) string {
	child := style.emph(flatten(e.Child))
	noun := childNoun(in, e.Child)

	verb := map[journal.ChildOp]string{
		journal.ChildOpAdded:      "Added",
		journal.ChildOpRemoved:    "Removed",
		journal.ChildOpMoved:      "Moved",
		journal.ChildOpArchived:   "Archived",
		journal.ChildOpUnarchived: "Unarchived",
	}[e.Op]
	if verb == "" {
		verb = "Changed child"
	}

	words := []string{verb}
	if noun != "" {
		words = append(words, noun)
	}
	subject := strings.Join(append(words, child), " ")
	if e.Op == journal.ChildOpMoved && e.From != "" && e.To != "" {
		return fmt.Sprintf("%s from %s to %s", subject, flatten(e.From), flatten(e.To))
	}
	return subject
}

// childNoun names the child's kind, so a line reads "Added objective
// **q1-growth**" rather than leaving the reader to infer it from the path.
// It is empty where no kind applies — a bucket under the root, or a container
// under an entity — since "Added **objectives**" is already unambiguous.
func childNoun(in In, child string) string {
	if child == "" || locator.IsReserved(child) {
		return ""
	}
	info, err := kindmeta.KindOf(append(slices.Clone(in.Locator), child))
	if err != nil || info.Kind == kindmeta.KindUnknown || info.Kind == kindmeta.KindContainer {
		return ""
	}
	return info.Kind.String()
}

// sinceSections drops the day sections before since, which is a string compare
// because the headers are ISO dates and ISO dates sort as text (§3.5's layout is
// chosen so that they do).
func sinceSections(sections []section, since string) []section {
	if since == "" {
		return sections
	}
	out := make([]section, 0, len(sections))
	for _, s := range sections {
		if s.Day >= since {
			out = append(out, s)
		}
	}
	return out
}
