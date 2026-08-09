package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// This file is the plan's Phase 14 task 2, the four properties carried forward
// from the v2 plan and re-aimed at v4:
//
//   - any legal mutation sequence followed by `rebuild` is a no-op;
//   - `rebuild; rebuild` is a no-op;
//   - `doctor` is clean after any sequence of legal commands;
//   - `add` then `remove --keep-files` returns a directory to its pre-para
//     contents, README body included.
//
// The first three are one experiment: generate a random legal sequence, run it,
// and ask the tree the three questions. They are separated below only because
// they fail for different reasons — a `rebuild` that writes says write-through
// missed a file, a second `rebuild` that writes says a renderer is not a
// function of truth alone, and a `doctor` finding says something else entirely.
//
// Phase 13 added the axis the plan asked for: the same sequence is run with the
// Claude surface off, on in symlink mode, and on in copy mode, because the write
// path's refresh and `rebuild`'s sync are two code paths that must produce
// identical trees. A fourth setting turns `emit.gitattributes` off, which is
// Phase 14's own new write path.
//
// The generator only ever emits commands it believes are legal, and every one of
// them is asserted to succeed. A refusal is a failure of the run rather than a
// skipped step: silently dropping refusals would let a bug that made a legal
// command fail hide as a generator that just does not generate it.

// surface is one configuration the whole sequence is replayed under.
type surface struct {
	name string
	// config is run through `para config set` on the fresh tree, before the
	// sequence, so the write path sees the setting on every mutation.
	config [][]string
}

var surfaces = []surface{
	{name: "surface off"},
	{name: "surface on, symlink", config: [][]string{{"config", "set", "emit.claude", "true"}}},
	{
		name: "surface on, copy",
		config: [][]string{
			{"config", "set", "emit.claude", "true"},
			{"config", "set", "emit.claude-skills", "copy"},
		},
	},
	{name: "gitattributes off", config: [][]string{{"config", "set", "emit.gitattributes", "false"}}},
}

// propertySeeds is how many independent sequences each surface is tried with.
// Each is a fixed seed rather than a random one, so a failure is reproducible
// from the test name alone.
//
// Twelve rather than the five this started with, because five was not enough to
// reach the generator's own bugs: the review that found two of them had to widen
// the matrix to hit either. A rare draw the generator gets wrong looks exactly
// like a rare draw it never makes.
var propertySeeds = []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}

// commandsPerSequence is the length of one sequence. Long enough that entities
// are created, changed, moved, archived and unarchived in the same tree — and
// that the interesting interactions between those need somewhere to happen:
// both generator bugs the review found needed four specific commands in order.
const commandsPerSequence = 40

// TestPropertiesOfALegalSequence is the first three properties.
func TestPropertiesOfALegalSequence(t *testing.T) {
	if !testHooksEnabled {
		t.Skip("the property tests need -tags para_testhooks so PARA_NOW fixes the clock; run `make test`")
	}

	for _, s := range surfaces {
		for _, seed := range propertySeeds {
			t.Run(fmt.Sprintf("%s/seed %d", s.name, seed), func(t *testing.T) {
				t.Parallel()
				root := filepath.Join(t.TempDir(), "brain")
				if out, code := runParaIn(t, filepath.Dir(root), nil, "init", "brain"); code != 0 {
					t.Fatalf("init: exit %d:\n%s", code, out)
				}
				for _, args := range s.config {
					if out, code := runPara(t, root, nil, args...); code != 0 {
						t.Fatalf("%v: exit %d:\n%s", args, code, out)
					}
				}

				run := replay(t, root, seed)

				// Property 3: doctor is clean after any sequence of legal
				// commands. It is asserted first because the other two are less
				// informative when it fails — a rebuild that rewrites a file
				// doctor has already named as stale is not news.
				assertDoctorClean(t, root, "after "+strings.Join(run, "; "))

				// Property 1: the sequence followed by a rebuild is a no-op.
				// This is §2.3's write-through claim stated as an experiment: if
				// every mutation really rewrote every projection it affected,
				// there is nothing left for a rebuild to do.
				out, code := runPara(t, root, nil, "rebuild")
				if code != 0 {
					t.Fatalf("rebuild: exit %d:\n%s", code, out)
				}
				if !strings.Contains(out, "nothing to rewrite") {
					t.Fatalf("rebuild after the sequence was not a no-op:\n%s\nsequence:\n%s",
						out, strings.Join(run, "\n"))
				}

				// Property 2: rebuild; rebuild is a no-op — and the interesting
				// case is a rebuild that had work to do, so the projections are
				// wiped first. A second pass that writes anything means a
				// renderer is reading something other than truth.
				wipeProjections(t, root)
				out, code = runPara(t, root, nil, "rebuild")
				if code != 0 {
					t.Fatalf("rebuild after wiping the projections: exit %d:\n%s", code, out)
				}
				if strings.Contains(out, "nothing to rewrite") {
					// Without this the no-op assertion below would pass on a
					// rebuild that never had anything to do, which is the way a
					// test of idempotence quietly stops testing anything.
					t.Fatalf("wiping every ACTIVITY.md left rebuild with nothing to do:\n%s", out)
				}
				out, code = runPara(t, root, nil, "rebuild")
				if code != 0 {
					t.Fatalf("second rebuild: exit %d:\n%s", code, out)
				}
				if !strings.Contains(out, "nothing to rewrite") {
					t.Fatalf("rebuild; rebuild was not a no-op:\n%s", out)
				}
				assertDoctorClean(t, root, "after wiping the projections and rebuilding")
			})
		}
	}
}

// TestRemoveKeepFilesRestoresThePreParaDirectory is the fourth property, and
// §18.4's exact promise: "--keep-files leaves your content and deletes para's
// footprint throughout the subtree: every .para/, every ACTIVITY.md, every
// MEASUREMENTS.csv, and the frontmatter block from every README.md, leaving the
// body".
//
// The directory is filled the way a user's would be — a README body, a nested
// content directory, a file that looks like something para generates but is not
// — and compared byte for byte against what was there before `remove` ran.
func TestRemoveKeepFilesRestoresThePreParaDirectory(t *testing.T) {
	if !testHooksEnabled {
		t.Skip("needs -tags para_testhooks; run `make test`")
	}

	root := filepath.Join(t.TempDir(), "brain")
	if out, code := runParaIn(t, filepath.Dir(root), nil, "init", "brain"); code != 0 {
		t.Fatalf("init: exit %d:\n%s", code, out)
	}
	for _, args := range [][]string{
		{"add", "projects.acme", "--name", "Acme", "--description", "Rebuild the consumer."},
		{"add", "projects.acme.objectives.q1", "--name", "Q1", "--description", "Move the funnel."},
		{
			"add", "projects.acme.objectives.q1.key-results.signups",
			"--name", "Signups", "--type", "ratio", "--start", "480/9000", "--target", "2000/12000",
		},
		{"measure", "projects.acme.objectives.q1.key-results.signups", "880/11000"},
		{"note", "projects.acme", "the ingest team is blocked"},
	} {
		if out, code := runPara(t, root, nil, args...); code != 0 {
			t.Fatalf("%v: exit %d:\n%s", args, code, out)
		}
	}

	// What the user owns: prose appended under para's frontmatter, and content
	// para never looks at.
	dir := filepath.Join(root, "projects", "acme")
	appendToReadme(t, filepath.Join(dir, "README.md"), "\n# Acme\n\nThe consumer falls over under replay load.\n\n- one\n- two\n")
	appendToReadme(t, filepath.Join(dir, "objectives", "q1", "README.md"), "\n## Q1\n\nThe funnel, in detail.\n")
	writeFileAt(t, filepath.Join(dir, "notes", "design.md"), "# Design\n\nProse.\n")
	// A file whose name looks generated and is not. §18.4 deletes para's
	// footprint, and this is not part of it.
	writeFileAt(t, filepath.Join(dir, "ACTIVITY.md.bak"), "not a generated file\n")

	// The comparison is against the tree as it stands, with the frontmatter
	// blocks removed — not against a hand-written expectation, because §18.4's
	// promise is about *these* bytes: "the frontmatter block from every
	// README.md, leaving the body", and everything else untouched.
	before := treeContents(t, dir)
	want := map[string]string{}
	for rel, content := range before {
		base := filepath.Base(rel)
		if strings.HasPrefix(rel, ".para/") || strings.Contains(rel, "/.para/") ||
			base == "ACTIVITY.md" || base == "MEASUREMENTS.csv" {
			continue
		}
		if base == "README.md" {
			body, ok := strings.CutPrefix(content, "---\n")
			if !ok {
				t.Fatalf("%s has no frontmatter to begin with:\n%s", rel, content)
			}
			_, body, ok = strings.Cut(body, "---\n")
			if !ok {
				t.Fatalf("%s has an unterminated frontmatter block:\n%s", rel, content)
			}
			want[rel] = body
			continue
		}
		want[rel] = content
	}
	if len(want) < 4 {
		t.Fatalf("the fixture is too thin to prove anything: %v", want)
	}

	if out, code := runPara(t, root, nil, "remove", "projects.acme", "--keep-files", "--force"); code != 0 {
		t.Fatalf("remove --keep-files: exit %d:\n%s", code, out)
	}

	// Directories as well as files: treeContents walks files, so an emptied but
	// surviving `.para/` would otherwise slip past every assertion below.
	for _, rel := range []string{".para", "objectives/.para", "objectives/q1/.para"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("remove --keep-files left the directory %s behind (%v)", rel, err)
		}
	}

	got := treeContents(t, dir)
	for rel, content := range want {
		if _, ok := got[rel]; !ok {
			t.Errorf("remove --keep-files took %s", rel)
			continue
		}
		if got[rel] != content {
			t.Errorf("%s =\n%q\nwant\n%q", rel, got[rel], content)
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			t.Errorf("remove --keep-files left %s behind", rel)
		}
	}

	// The tree it was removed from carries no error, and the one advisory is the
	// directory the user asked to keep: §21.2's "loose filing", which is exactly
	// what a kept directory in a bucket now is.
	rep := doctorReport(t, root, "after remove --keep-files")
	for _, f := range rep.Findings {
		if f.Severity != "advisory" || f.Finding != "untracked" || f.Path != "projects/acme" {
			t.Errorf("doctor reported %s %s %s (%s)", f.Severity, f.Finding, f.Path, f.Detail)
		}
	}
	if len(rep.Findings) != 1 {
		t.Errorf("doctor reported %d findings, want exactly the kept directory", len(rep.Findings))
	}
}

// replay generates and runs one legal sequence, returning the commands it ran so
// a failure can be reproduced by reading them.
func replay(t *testing.T, root string, seed int64) []string {
	t.Helper()
	w := &world{rng: rand.New(rand.NewSource(seed))} //nolint:gosec // reproducibility, not secrecy
	var run []string
	for i := 0; i < commandsPerSequence; i++ {
		// A draw can land on something the tree cannot do yet — a `measure`
		// before any key-result exists. Redrawing rather than skipping keeps the
		// sequence dense without teaching the generator an ordering, which would
		// make it produce the same shape of history every time.
		var args []string
		for attempt := 0; attempt < 6 && args == nil; attempt++ {
			args = w.next(i)
		}
		if args == nil {
			continue
		}
		run = append(run, strings.Join(args, " "))
		if out, code := runPara(t, root, nil, args...); code != 0 {
			t.Fatalf("the generator produced an illegal command (exit %d):\n  para %s\n%s\nsequence so far:\n%s",
				code, strings.Join(args, " "), out, strings.Join(run, "\n"))
		}
	}
	return run
}

// world is the generator's model of what exists, which is all it needs to emit
// only legal commands.
//
// It tracks locators and kinds and nothing else — not fields, not journals —
// because legality is a question about placement and existence (§1.5, §18.1).
type world struct {
	rng *rand.Rand

	projects   []string // "projects.<id>"
	objectives []string // "<project>.objectives.<id>"
	keyResults []string // "<objective>.key-results.<id>"
	areas      []string // "areas.<id>", possibly nested
	resources  []string
	skills     []string
	archived   []string // "archive.<…>"

	// measured counts the measurements taken, so each one gets a distinct --at:
	// §3.1 refuses two readings at the same stored instant and the clock is
	// fixed, so without this every second measurement on a key-result is a
	// legitimate refusal.
	measured int
}

// next picks one legal command, or nil when the draw lands on something this
// tree cannot do yet.
func (w *world) next(step int) []string {
	id := fmt.Sprintf("x%d", step)
	switch w.rng.Intn(14) {
	case 0:
		w.projects = append(w.projects, "projects."+id)
		return []string{"add", "projects." + id, "--name", "P" + id, "--description", "A project."}
	case 1:
		parent, ok := pick(w.rng, w.projects)
		if !ok {
			return nil
		}
		loc := parent + ".objectives." + id
		w.objectives = append(w.objectives, loc)
		return []string{"add", loc, "--name", "O" + id, "--description", "An objective."}
	case 2:
		parent, ok := pick(w.rng, w.objectives)
		if !ok {
			return nil
		}
		loc := parent + ".key-results." + id
		w.keyResults = append(w.keyResults, loc)
		return []string{"add", loc, "--name", "K" + id, "--type", "ratio", "--start", "100/1000", "--target", "900/1000"}
	case 3:
		// An area, sometimes nested inside another — §1.5's "areas and
		// resources nest freely".
		loc := "areas." + id
		if parent, ok := pick(w.rng, w.areas); ok && w.rng.Intn(2) == 0 {
			loc = parent + "." + id
		}
		w.areas = append(w.areas, loc)
		return []string{"add", loc, "--name", "A" + id, "--description", "An area."}
	case 4:
		// §1.5's "areas and resources nest freely" is about both, so this nests
		// too. Without it the generator claimed a shape it could never emit.
		loc := "resources." + id
		if parent, ok := pick(w.rng, w.resources); ok && w.rng.Intn(2) == 0 {
			loc = parent + "." + id
		}
		w.resources = append(w.resources, loc)
		return []string{"add", loc, "--name", "R" + id, "--description", "A resource."}
	case 5:
		loc := "skills." + id
		w.skills = append(w.skills, loc)
		return []string{"add", loc, "--name", "S" + id, "--description", "when " + id}
	case 6:
		// A status, on a kind that has one (§15): project and objective.
		loc, ok := pick(w.rng, append(slices.Clone(w.projects), w.objectives...))
		if !ok {
			return nil
		}
		switch w.rng.Intn(3) {
		case 0:
			return []string{"set", loc, "--status", "in-progress"}
		case 1:
			return []string{"set", loc, "--status", "blocked", "--note", "waiting on " + id}
		default:
			return []string{"set", loc, "--status", "done"}
		}
	case 7:
		loc, ok := pick(w.rng, w.everything())
		if !ok {
			return nil
		}
		return []string{"set", loc, "--tags", "alpha,beta"}
	case 8:
		loc, ok := pick(w.rng, w.everything())
		if !ok {
			return nil
		}
		return []string{"note", loc, "something happened at step " + id}
	case 9:
		loc, ok := pick(w.rng, w.keyResults)
		if !ok {
			return nil
		}
		// A distinct instant per reading, and never in the future relative to the
		// fixed clock (§15: `created` and `--at` are rejected if future). §3.1
		// refuses two readings at the same stored instant, and the clock is
		// fixed, so the counter has to be the thing that makes them distinct —
		// and it carries into the hour rather than wrapping at 28 days, so
		// lengthening a sequence cannot turn this into a refusal.
		day := 1 + w.measured%27
		hour := (w.measured / 27) % 24
		w.measured++
		at := fmt.Sprintf("2026-02-%02dT%02d:00", day, hour)
		return []string{"measure", loc, "500/1000", "--at", at}
	case 10:
		// A rename inside the same parent, which is the only move that cannot
		// change a kind (§18.3).
		loc, ok := pick(w.rng, w.renamable())
		if !ok {
			return nil
		}
		to := loc[:strings.LastIndex(loc, ".")+1] + "r" + id
		w.rename(loc, to)
		return []string{"move", loc, to}
	case 11:
		// Archive a leaf. Leaves only, so the model does not have to reproduce
		// §1.6's cascade to know what is where afterwards.
		loc, ok := pick(w.rng, w.archivable())
		if !ok {
			return nil
		}
		w.forget(loc)
		w.archived = append(w.archived, "archive."+loc)
		return []string{"archive", loc}
	case 12:
		// §1.6: "unarchiving cascades upward … and brings its own subtree with
		// it, in one operation". So the model has to bring the subtree back too —
		// tracking only the locator named would leave the generator believing in
		// archived descendants that are now live, and the next `unarchive` of one
		// is a command para correctly refuses.
		loc, ok := pick(w.rng, w.unarchivable())
		if !ok {
			return nil
		}
		for _, archived := range slices.Clone(w.archived) {
			if archived != loc && !strings.HasPrefix(archived, loc+".") {
				continue
			}
			w.archived = slices.DeleteFunc(w.archived, func(s string) bool { return s == archived })
			w.remember(strings.TrimPrefix(archived, "archive."))
		}
		return []string{"unarchive", loc}
	default:
		// A config key at the root, including the two that decide whether a file
		// exists at all — which is what makes the surface axis a moving target
		// within a sequence rather than only a setting it starts under.
		switch w.rng.Intn(3) {
		case 0:
			return []string{"config", "set", "project.stale-after", fmt.Sprint(7 + w.rng.Intn(30))}
		case 1:
			return []string{"config", "set", "review.cadence", fmt.Sprint(30 + w.rng.Intn(60))}
		default:
			return []string{"config", "set", "key-result.at-risk-pace", "0.8"}
		}
	}
}

// everything is every live entity, which is what `note` and `--tags` accept.
func (w *world) everything() []string {
	out := slices.Clone(w.projects)
	out = append(out, w.objectives...)
	out = append(out, w.keyResults...)
	out = append(out, w.areas...)
	out = append(out, w.resources...)
	out = append(out, w.skills...)
	return out
}

// leaves is every entity with nothing beneath it that the model tracks, so a
// rename or a removal does not have to rewrite a subtree's worth of locators.
func (w *world) leaves() []string {
	var out []string
	for _, loc := range w.everything() {
		if !w.hasChild(loc) {
			out = append(out, loc)
		}
	}
	return out
}

// renamable is leaves() minus anything with an archived descendant recorded
// under its current name.
//
// That exclusion is a real rough edge in the design rather than a shortcut here,
// and the generator found it: archive `resources.a.b`, then rename `resources.a`
// to `resources.c`, and `archive/resources/a/b` still records an ancestry that no
// longer exists. §18.3's relocation rewrites a moved entity's descendants and a
// skill's scope entries; it does not follow the entity's archived shadow. Para
// refuses the later `unarchive` and names the ancestor it cannot reinstate,
// which is the right answer to a state it cannot repair — but it is a refusal a
// legal sequence can reach, so the generator stays out of it. See the Phase 14
// notes in docs/implementation-plan.md.
func (w *world) renamable() []string {
	var out []string
	for _, loc := range w.leaves() {
		stranded := false
		for _, archived := range w.archived {
			if strings.HasPrefix(archived, "archive."+loc+".") {
				stranded = true
				break
			}
		}
		if !stranded {
			out = append(out, loc)
		}
	}
	return out
}

// archivable is leaves() minus the skills: `archive` is for entities in the
// three live buckets, and a skill lives in .agents/ (§1.4, §1.6).
func (w *world) archivable() []string {
	var out []string
	for _, loc := range w.leaves() {
		if !strings.HasPrefix(loc, "skills.") {
			out = append(out, loc)
		}
	}
	return out
}

// unarchivable is the archived locators with no archived ancestor — the only
// ones §1.6 lets you name, since unarchiving an ancestor takes its descendants
// with it and unarchiving a descendant reinstates the ancestors it needs.
func (w *world) unarchivable() []string {
	var out []string
	for _, loc := range w.archived {
		nested := false
		for _, other := range w.archived {
			if other != loc && strings.HasPrefix(loc, other+".") {
				nested = true
				break
			}
		}
		if !nested {
			out = append(out, loc)
		}
	}
	return out
}

func (w *world) hasChild(loc string) bool {
	for _, other := range w.everything() {
		if other != loc && strings.HasPrefix(other, loc+".") {
			return true
		}
	}
	return false
}

func (w *world) rename(from, to string) {
	for _, list := range w.lists() {
		for i, loc := range *list {
			if loc == from {
				(*list)[i] = to
			}
		}
	}
}

func (w *world) forget(loc string) {
	for _, list := range w.lists() {
		*list = slices.DeleteFunc(*list, func(s string) bool { return s == loc })
	}
}

// remember puts an unarchived locator back in the list its shape says it
// belongs to.
func (w *world) remember(loc string) {
	switch {
	case strings.Contains(loc, ".key-results."):
		w.keyResults = append(w.keyResults, loc)
	case strings.Contains(loc, ".objectives."):
		w.objectives = append(w.objectives, loc)
	case strings.HasPrefix(loc, "projects."):
		w.projects = append(w.projects, loc)
	case strings.HasPrefix(loc, "areas."):
		w.areas = append(w.areas, loc)
	default:
		w.resources = append(w.resources, loc)
	}
}

func (w *world) lists() []*[]string {
	return []*[]string{&w.projects, &w.objectives, &w.keyResults, &w.areas, &w.resources, &w.skills}
}

func pick(rng *rand.Rand, from []string) (string, bool) {
	if len(from) == 0 {
		return "", false
	}
	return from[rng.Intn(len(from))], true
}

func assertDoctorClean(t *testing.T, root, when string) {
	t.Helper()
	rep := doctorReport(t, root, when)
	if rep.Clean {
		return
	}
	var lines []string
	for _, f := range rep.Findings {
		lines = append(lines, f.Severity+" "+f.Finding+" "+f.Path+" — "+f.Detail)
	}
	t.Fatalf("doctor %s is not clean:\n%s", when, strings.Join(lines, "\n"))
}

// wipeProjections deletes every wholly generated file in the tree, which is the
// state §2.4 calls "a merge resolved truth and left the projections wrong".
func wipeProjections(t *testing.T, root string) {
	t.Helper()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		switch filepath.Base(path) {
		case "ACTIVITY.md", "MEASUREMENTS.csv", "CLAUDE.md":
			return os.Remove(path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("wiping projections: %v", err)
	}
}

func appendToReadme(t *testing.T, path, body string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, []byte(body)...), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func writeFileAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// treeContents is every file under dir, keyed by its path relative to dir.
func treeContents(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return out
}
