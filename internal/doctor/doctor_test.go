package doctor_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/doctor"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/rebuild"
	"github.com/colchuck-ai/para/internal/view"
)

const fixedNow = "2026-03-05T17:00:00Z"

func now(t *testing.T) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func loc(t *testing.T, s string) locator.Locator {
	t.Helper()
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return l
}

// cleanTree plants a tree, adds one project with an objective and a
// key-result, and rebuilds — so that everything doctor could report is the
// thing a test then breaks on purpose.
func cleanTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".para/tree.toml": "schema = 1\nname = \"brain\"\ndescription = \"Everything.\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
	}
	for _, rel := range []string{
		"projects", "areas", "resources", "archive",
		"archive/projects", "archive/areas", "archive/resources",
	} {
		files[rel+"/.para/state.toml"] = "name = \"" + rel + "\"\ndescription = \"A bucket.\"\ncreated = \"2026-01-01T00:00:00Z\"\n"
	}
	files[".agents/skills/.keep"] = ""
	for rel, content := range files {
		write(t, root, rel, content)
	}

	e := mutate.NewEnv(root, clock.Fixed{At: now(t)})
	add(t, e, "projects.acme", "name", "Acme", "description", "Rebuild the consumer.")
	add(t, e, "projects.acme.objectives.q1", "name", "Q1", "description", "Grow signups.")
	add(t, e, "projects.acme.objectives.q1.key-results.signups",
		"name", "Signups", "type", "ratio", "start", "480/9000", "target", "2000/12000")
	if _, err := e.Measure(loc(t, "projects.acme.objectives.q1.key-results.signups"), "700/10000", "", ""); err != nil {
		t.Fatalf("Measure: %v", err)
	}
	add(t, e, "skills.report", "name", "Report", "description", "when asked for the signups number")

	if _, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	return root
}

func add(t *testing.T, e *mutate.Env, at string, pairs ...string) {
	t.Helper()
	var f mutate.Fields
	for i := 0; i+1 < len(pairs); i += 2 {
		field := kindmeta.Field(pairs[i])
		if mutate.IsListField(field) {
			f.SetList(field, strings.Split(pairs[i+1], ","))
			continue
		}
		f.Set(field, pairs[i+1])
	}
	if _, err := e.Add(loc(t, at), f); err != nil {
		t.Fatalf("Add(%s): %v", at, err)
	}
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, root string, opts doctor.Options) doctor.Report {
	t.Helper()
	env := view.NewEnv(root, clock.Fixed{At: now(t)})
	rep, err := doctor.Run(env, opts)
	if err != nil {
		t.Fatalf("doctor.Run: %v", err)
	}
	return rep
}

// findings returns the findings of one kind, as "path: detail" lines.
func findings(rep doctor.Report, kind doctor.Kind) []string {
	var out []string
	for _, f := range rep.Findings {
		if f.Kind != kind {
			continue
		}
		path := f.Path
		if f.Line > 0 {
			path = f.Path + ":" + itoa(f.Line)
		}
		out = append(out, path+": "+f.Detail)
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// assertOnly checks that the report holds exactly the findings of the kinds
// given, and nothing else — the plan's "a hand-mutated tree produces exactly
// the expected finding set".
func assertOnly(t *testing.T, rep doctor.Report, want ...doctor.Kind) {
	t.Helper()
	counts := map[doctor.Kind]int{}
	for _, f := range rep.Findings {
		counts[f.Kind]++
	}
	for _, k := range want {
		if counts[k] == 0 {
			t.Errorf("no %s finding; report was %v", k, lines(rep))
		}
		delete(counts, k)
	}
	for k := range counts {
		t.Errorf("unexpected %s finding; report was %v", k, lines(rep))
	}
}

func lines(rep doctor.Report) []string {
	var out []string
	for _, f := range rep.Findings {
		out = append(out, string(f.Kind)+"  "+f.Path+" "+f.Detail)
	}
	return out
}

// TestCleanTree is the control: a tree para built and rebuilt reports nothing
// and exits 0. Every other test in this file breaks exactly one thing about it.
func TestCleanTree(t *testing.T) {
	rep := run(t, cleanTree(t), doctor.Options{})
	if !rep.Clean() {
		t.Fatalf("a rebuilt tree is not clean: %v", lines(rep))
	}
	if rep.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0", rep.ExitCode())
	}
}

// TestOrphan is §8.5's named weakness: hand-mv an entity into a content
// directory and it vanishes from every read.
func TestOrphan(t *testing.T) {
	root := cleanTree(t)
	mkdir(t, root, "projects/acme/notes")
	if err := os.Rename(
		filepath.Join(root, "projects", "acme", "objectives", "q1"),
		filepath.Join(root, "projects", "acme", "notes", "q1"),
	); err != nil {
		t.Fatal(err)
	}

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindOrphan)
	if len(got) == 0 || !strings.HasPrefix(got[0], "projects/acme/notes/q1:") {
		t.Fatalf("orphan findings = %v, want one for projects/acme/notes/q1", got)
	}
	// Everything beneath it is unreachable too — the key-results/ container
	// and the key-result itself — and each is reported as its own orphan
	// rather than folded into its parent's. Folding would be a guess about
	// which one the repair is, and the repair is one `mv` for all three.
	if len(got) != 3 {
		t.Errorf("orphan findings = %v, want the objective, its container, and its key-result", got)
	}
}

// TestMisplaced covers §10's own example: a state.toml at a location that
// derives no kind.
func TestMisplaced(t *testing.T) {
	root := cleanTree(t)
	write(t, root, "projects/acme/nested/.para/state.toml", "name = \"Nested\"\ndescription = \"A project cannot nest.\"\n")

	rep := run(t, root, doctor.Options{})

	if got := findings(rep, doctor.KindMisplaced); len(got) != 1 {
		t.Fatalf("misplaced findings = %v, want one for projects/acme/nested", got)
	}
}

// TestCollision is §1.4's reserved words: a name that can never be an id.
func TestCollision(t *testing.T) {
	root := cleanTree(t)
	write(t, root, "projects/skills/.para/state.toml", "name = \"Skills\"\ndescription = \"A project named with a reserved word.\"\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindCollision)
	if len(got) != 1 || !strings.HasPrefix(got[0], "projects/skills:") {
		t.Fatalf("collision findings = %v, want one for projects/skills", got)
	}
}

// TestCollisionSingularNoun is R6: growing locator.ReservedWords to
// seventeen means the seven singular nouns are reserved too, so an entity
// directory named "project" — legal before R6 — is now the same §1.4
// collision TestCollision pins for the plural "skills".
func TestCollisionSingularNoun(t *testing.T) {
	root := cleanTree(t)
	write(t, root, "projects/project/.para/state.toml", "name = \"Project\"\ndescription = \"A project named with a noun.\"\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindCollision)
	if len(got) != 1 || !strings.HasPrefix(got[0], "projects/project:") {
		t.Fatalf("collision findings = %v, want one for projects/project", got)
	}
}

// TestInvalid walks the truth failures §10 lists: unparseable TOML, a missing
// required field, an enum out of range, and a measurement whose shape
// contradicts its key-result's type.
func TestInvalid(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
		want string
	}{
		{
			name: "unparseable TOML",
			path: "projects/acme/.para/state.toml",
			body: "name = \"Acme\"\nthis is not toml\n",
			want: "is not readable: invalid TOML",
		},
		{
			name: "a missing required field",
			path: "projects/acme/.para/state.toml",
			body: "description = \"No name.\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
			want: "name is required",
		},
		{
			name: "an enum out of range",
			path: "projects/acme/.para/state.toml",
			body: "name = \"Acme\"\ndescription = \"A status nothing admits.\"\nstatus = \"shipped\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
			want: "is not a project status",
		},
		{
			name: "a skill with no description",
			path: ".agents/skills/para-report/.para/state.toml",
			body: "name = \"Report\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
			want: "description is required",
		},
		{
			name: "a key para does not recognise",
			path: "projects/acme/.para/state.toml",
			body: "name = \"Acme\"\ndescription = \"A typo.\"\ndescripton = \"the same, misspelled\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
			want: "unknown key",
		},
		{
			name: "a created in the future",
			path: "projects/acme/.para/state.toml",
			body: "name = \"Acme\"\ndescription = \"Time travel.\"\ncreated = \"2027-01-01T00:00:00Z\"\n",
			want: "is in the future",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := cleanTree(t)
			write(t, root, c.path, c.body)

			rep := run(t, root, doctor.Options{})

			got := findings(rep, doctor.KindInvalid)
			if !anyContains(got, c.want) {
				t.Fatalf("invalid findings = %v, want one mentioning %q", got, c.want)
			}
		})
	}
}

// TestInvalidMeasurementShape is the one `invalid` that lives in a journal: a
// reading that contradicts its key-result's type (§10).
func TestInvalidMeasurementShape(t *testing.T) {
	root := cleanTree(t)
	logs := filepath.Join("projects", "acme", "objectives", "q1", "key-results", "signups", ".para", "logs")
	entries, err := os.ReadDir(filepath.Join(root, logs))
	if err != nil || len(entries) == 0 {
		t.Fatalf("reading the key-result's journal: %v", err)
	}
	rel := filepath.ToSlash(filepath.Join(logs, entries[0].Name()))
	appendLine(t, root, rel, `{"at":"2026-03-05T12:00:00Z","kind":"measurement","value":"yes"}`)

	rep := run(t, root, doctor.Options{})

	if got := findings(rep, doctor.KindInvalid); !anyContains(got, "is not a ratio") {
		t.Fatalf("invalid findings = %v, want one about a reading that is not a ratio", got)
	}
}

// TestJournal covers §10's four journal breakages, each reported with a file
// and a line.
func TestJournal(t *testing.T) {
	cases := map[string]string{
		"not valid JSON":      `{"at":"2026-03-05T12:00:00Z" "kind":"note"`,
		"no at":               `{"kind":"note","note":"no instant"}`,
		"an unknown kind":     `{"at":"2026-03-05T12:00:00Z","kind":"invented"}`,
		"an at in the future": `{"at":"2027-01-01T00:00:00Z","kind":"note","note":"tomorrow"}`,
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			root := cleanTree(t)
			// The bucket's journal rather than the project's: `add` logs a
			// child event at the parent, and a project's own journal is empty
			// until something happens *to it* (§3.3, §18.1).
			rel := firstJournal(t, root, "projects")
			appendLine(t, root, rel, line)

			rep := run(t, root, doctor.Options{})

			got := findings(rep, doctor.KindJournal)
			if len(got) != 1 {
				t.Fatalf("journal findings = %v, want exactly one", got)
			}
			if !strings.HasPrefix(got[0], rel+":") {
				t.Errorf("journal finding = %q, want it to name %s and a line", got[0], rel)
			}
		})
	}
}

// TestScopeUnresolved is §5.4: an entry naming a locator that does not exist.
func TestScopeUnresolved(t *testing.T) {
	root := cleanTree(t)
	e := mutate.NewEnv(root, clock.Fixed{At: now(t)})
	var f mutate.Fields
	f.SetList(kindmeta.FieldScope, []string{"project.acme", "project.gone"})
	if _, err := e.Set(loc(t, "skills.report"), f, ""); err != nil {
		t.Fatalf("Set: %v", err)
	}

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindScopeUnresolved)
	if len(got) != 1 || !strings.Contains(got[0], "project.gone") {
		t.Fatalf("scope-unresolved findings = %v, want one naming project.gone", got)
	}
	assertOnly(t, rep, doctor.KindScopeUnresolved)
}

// TestOrphanRule is §5.3: a derived rule whose generated_from skill is gone.
func TestOrphanRule(t *testing.T) {
	root := cleanTree(t)
	write(t, root, ".agents/rules/para-gone.md",
		"---\ngenerated_from: \"para-gone\"\n---\nUse the **Gone** skill.\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindOrphanRule)
	if len(got) != 1 || !strings.Contains(got[0], "skill.gone") {
		t.Fatalf("orphan-rule findings = %v, want one naming skill.gone", got)
	}
}

// claudeTree is cleanTree with the Claude Code surface turned on and rebuilt,
// so the eight CLAUDE.md files and the mirror are there to be broken. Every
// mirror finding needs one: with `emit.claude` off, an entry under
// .claude/skills/ is residue whatever else is wrong with it.
func claudeTree(t *testing.T) string {
	t.Helper()
	root := cleanTree(t)
	write(t, root, ".para/config.toml", "emit.claude = true\n")
	if _, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if rep := run(t, root, doctor.Options{}); !rep.Clean() {
		t.Fatalf("a tree with the Claude surface rebuilt is not clean: %v", lines(rep))
	}
	return root
}

// TestMirrorFindings covers the two mirror findings §10 names, including the
// plain-file case a `core.symlinks=false` checkout leaves behind (§6.1).
func TestMirrorFindings(t *testing.T) {
	t.Run("orphan-mirror", func(t *testing.T) {
		root := claudeTree(t)
		mkdir(t, root, ".claude/skills/para-gone")

		rep := run(t, root, doctor.Options{})

		got := findings(rep, doctor.KindOrphanMirror)
		if len(got) != 1 || !strings.Contains(got[0], "skill.gone") {
			t.Fatalf("orphan-mirror findings = %v, want one naming skill.gone", got)
		}
	})

	t.Run("a symlink that does not resolve", func(t *testing.T) {
		root := claudeTree(t)
		link := filepath.Join(root, ".claude", "skills", "para-report")
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../../.agents/skills/para-nowhere", link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		rep := run(t, root, doctor.Options{})

		if got := findings(rep, doctor.KindBrokenLink); len(got) != 1 {
			t.Fatalf("broken-link findings = %v, want one", got)
		}
	})

	t.Run("a link materialised as a plain file", func(t *testing.T) {
		root := claudeTree(t)
		if err := os.Remove(filepath.Join(root, ".claude", "skills", "para-report")); err != nil {
			t.Fatal(err)
		}
		write(t, root, ".claude/skills/para-report", "../../.agents/skills/para-report")

		rep := run(t, root, doctor.Options{})

		got := findings(rep, doctor.KindBrokenLink)
		if len(got) != 1 || !strings.Contains(got[0], "core.symlinks=false") {
			t.Fatalf("broken-link findings = %v, want one naming the checkout", got)
		}
	})

	t.Run("a mirror in the wrong mode is stale", func(t *testing.T) {
		root := claudeTree(t)
		write(t, root, ".para/config.toml", "emit.claude = true\nemit.claude-skills = \"copy\"\n")

		rep := run(t, root, doctor.Options{})

		got := findings(rep, doctor.KindStaleProjection)
		if len(got) != 1 || !strings.Contains(got[0], "symlink where a copy belongs") {
			t.Fatalf("stale-projection findings = %v, want one naming the mode", got)
		}
	})

	t.Run("a symlinked mirror is never walked as an entity", func(t *testing.T) {
		root := claudeTree(t)
		if _, err := os.Readlink(filepath.Join(root, ".claude", "skills", "para-report")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		rep := run(t, root, doctor.Options{})

		if !rep.Clean() {
			t.Fatalf("a healthy symlink mirror is not clean: %v", lines(rep))
		}
	})
}

// TestSurfaceResidue is the answer Phase 12 left open: with `emit.claude` off,
// everything the surface leaves behind is residue — one finding, one repair —
// rather than orphan-mirror or broken-link, which would name a cause that is
// not the cause.
func TestSurfaceResidue(t *testing.T) {
	root := claudeTree(t)
	write(t, root, ".para/config.toml", "emit.claude = false\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindStaleProjection)
	want := []string{
		".claude/skills/para-report: should not exist; emit.claude is off",
		"CLAUDE.md: should not exist; emit.claude is off",
		"archive/CLAUDE.md: should not exist; emit.claude is off",
		"archive/areas/CLAUDE.md: should not exist; emit.claude is off",
		"archive/projects/CLAUDE.md: should not exist; emit.claude is off",
		"archive/resources/CLAUDE.md: should not exist; emit.claude is off",
		"areas/CLAUDE.md: should not exist; emit.claude is off",
		"projects/CLAUDE.md: should not exist; emit.claude is off",
		"resources/CLAUDE.md: should not exist; emit.claude is off",
	}
	if !slices.Equal(got, want) {
		t.Errorf("stale-projection findings =\n%v\nwant\n%v", got, want)
	}
	assertOnly(t, rep, doctor.KindStaleProjection)

	if _, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if rep := run(t, root, doctor.Options{}); !rep.Clean() {
		t.Errorf("rebuild did not sweep the residue: %v", lines(rep))
	}
}

// TestMissingMirrorIsStale: a mirror is a projection (§6.1), so one that is not
// there differs from what would be written now exactly as a missing CLAUDE.md
// does.
func TestMissingMirrorIsStale(t *testing.T) {
	root := claudeTree(t)
	if err := os.RemoveAll(filepath.Join(root, ".claude")); err != nil {
		t.Fatal(err)
	}

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindStaleProjection)
	want := []string{".claude/skills/para-report: is missing; skill.report has no mirror"}
	if !slices.Equal(got, want) {
		t.Errorf("stale-projection findings = %v, want %v", got, want)
	}
}

// TestStaleProjection is §26's own case, and the plan's "the dated ACTIVITY.md
// drift report is tested against a hand-edited prior day".
func TestStaleProjection(t *testing.T) {
	root := cleanTree(t)
	current := readFile(t, root, "projects/acme/ACTIVITY.md")
	write(t, root, "projects/acme/ACTIVITY.md", current+"hand-edited\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindStaleProjection)
	if len(got) != 1 {
		t.Fatalf("stale-projection findings = %v, want one", got)
	}
	if !strings.HasPrefix(got[0], "projects/acme/ACTIVITY.md: differs from journal (from 2026-03-05)") {
		t.Errorf("stale-projection finding = %q, want it dated", got[0])
	}
	if rep.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1", rep.ExitCode())
	}
}

// TestStaleProjectionDatesAnOlderDay is the half of §10 write-through cannot
// check: ACTIVITY.md is compared against *every* journal file backing it, so a
// day older than today's is where the report points.
func TestStaleProjectionDatesAnOlderDay(t *testing.T) {
	root := cleanTree(t)
	e := mutate.NewEnv(root, clock.Fixed{At: now(t)})
	if _, err := e.Note(loc(t, "projects.acme"), "an older note", "2026-02-01"); err != nil {
		t.Fatalf("Note: %v", err)
	}

	current := readFile(t, root, "projects/acme/ACTIVITY.md")
	broken := strings.Replace(current, "an older note", "something else entirely", 1)
	if broken == current {
		t.Fatalf("the fixture's ACTIVITY.md does not hold the note:\n%s", current)
	}
	write(t, root, "projects/acme/ACTIVITY.md", broken)

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindStaleProjection)
	if len(got) != 1 || !strings.Contains(got[0], "(from 2026-02-01)") {
		t.Fatalf("stale-projection findings = %v, want one dated 2026-02-01", got)
	}
}

// TestStaleProjectionMissingFile: a generated file that is not there differs
// from what would be written now as surely as a wrong one does.
func TestStaleProjectionMissingFile(t *testing.T) {
	root := cleanTree(t)
	if err := os.Remove(filepath.Join(root, "projects", "acme", "README.md")); err != nil {
		t.Fatal(err)
	}

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindStaleProjection)
	if len(got) != 1 || !strings.Contains(got[0], "is missing") {
		t.Fatalf("stale-projection findings = %v, want one for the missing README.md", got)
	}
}

// TestUntracked is §10's one advisory, and its exit code: 2, so that CI can
// gate on 1 and ignore this (§21.2).
func TestUntracked(t *testing.T) {
	root := cleanTree(t)
	mkdir(t, root, "projects/notes-dump")
	// Content inside an entity is content and is never reported.
	mkdir(t, root, "projects/acme/notes")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindUntracked)
	if len(got) != 1 || !strings.HasPrefix(got[0], "projects/notes-dump:") {
		t.Fatalf("untracked findings = %v, want one for projects/notes-dump", got)
	}
	if rep.ExitCode() != 2 {
		t.Errorf("ExitCode = %d, want 2 for an advisory-only report", rep.ExitCode())
	}
	if rep.Errors() != 0 {
		t.Errorf("Errors = %d, want 0", rep.Errors())
	}
}

// TestArchiveStubIsNotUntracked: a stub is a real position in the tree with
// real things beneath it (§1.6), not loose filing.
func TestArchiveStubIsNotUntracked(t *testing.T) {
	root := cleanTree(t)
	e := mutate.NewEnv(root, clock.Fixed{At: now(t)})
	plan, err := e.PlanArchive(loc(t, "projects.acme.objectives.q1"))
	if err != nil {
		t.Fatalf("PlanArchive: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if _, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	rep := run(t, root, doctor.Options{})

	if !rep.Clean() {
		t.Fatalf("an archived subtree is not clean: %v", lines(rep))
	}
}

// TestScopeNarrowsTheScan: `doctor <locator>` reports what is under the
// locator and nothing else.
func TestScopeNarrowsTheScan(t *testing.T) {
	root := cleanTree(t)
	write(t, root, "projects/acme/ACTIVITY.md", "# Activity\n")
	write(t, root, "areas/.para/state.toml", "name = \"Areas\"\n")

	rep := run(t, root, doctor.Options{Scope: loc(t, "projects.acme")})

	assertOnly(t, rep, doctor.KindStaleProjection)
	for _, f := range rep.Findings {
		if !strings.HasPrefix(f.Path, "projects/acme") {
			t.Errorf("a scoped scan reported %s, which is outside its scope", f.Path)
		}
	}
}

// TestScopeThatNamesNothing: an empty report is a wrong answer to an explicit
// question.
func TestScopeThatNamesNothing(t *testing.T) {
	root := cleanTree(t)
	env := view.NewEnv(root, clock.Fixed{At: now(t)})
	if _, err := doctor.Run(env, doctor.Options{Scope: loc(t, "projects.nothing")}); err == nil {
		t.Fatal("doctor on a locator that names nothing: want an error, got nil")
	}
}

// TestFindingsAreOrderedByTheTable pins §10's order, so two runs over one
// broken tree read the same way (§0.2).
func TestFindingsAreOrderedByTheTable(t *testing.T) {
	root := cleanTree(t)
	mkdir(t, root, "projects/notes-dump")
	write(t, root, "projects/acme/ACTIVITY.md", "# Activity\n")
	write(t, root, "projects/acme/nested/.para/state.toml", "name = \"Nested\"\ndescription = \"Nope.\"\n")

	rep := run(t, root, doctor.Options{})

	var got []doctor.Kind
	for _, f := range rep.Findings {
		if len(got) == 0 || got[len(got)-1] != f.Kind {
			got = append(got, f.Kind)
		}
	}
	want := []doctor.Kind{doctor.KindMisplaced, doctor.KindStaleProjection, doctor.KindUntracked}
	if len(got) != len(want) {
		t.Fatalf("finding kinds in order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("finding kinds in order = %v, want %v", got, want)
		}
	}
}

func anyContains(lines []string, want string) bool {
	for _, line := range lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(data)
}

func appendLine(t *testing.T, root, rel, line string) {
	t.Helper()
	write(t, root, rel, readFile(t, root, rel)+line+"\n")
}

// firstJournal is the entity's oldest journal file, root-relative.
func firstJournal(t *testing.T, root, entity string) string {
	t.Helper()
	dir := filepath.Join(entity, ".para", "logs")
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil || len(entries) == 0 {
		t.Fatalf("reading %s: %v", dir, err)
	}
	return filepath.ToSlash(filepath.Join(dir, entries[0].Name()))
}

// TestSkillIDCollision is the Phase 12 review's second finding: the second root
// admitted any para-* directory as a skill, so a reserved word used as a skill
// id was blessed by doctor and refused by `add` — para disagreeing with itself
// about one id (§1.4, §10).
func TestSkillIDCollision(t *testing.T) {
	root := cleanTree(t)
	write(t, root, ".agents/skills/para-projects/.para/state.toml",
		"name = \"Projects\"\ndescription = \"A skill named with a reserved word.\"\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindCollision)
	if len(got) != 1 || !strings.HasPrefix(got[0], ".agents/skills/para-projects:") {
		t.Fatalf("collision findings = %v, want one for the skill", got)
	}
}

// TestIllegalIDIsMisplaced: a directory name that is not a legal locator
// segment names no locator (§1.4), so it derives no kind — §10's `misplaced`.
// It used to walk as a real entity, which had rebuild manufacturing generated
// files at a locator `show` and `list` then refused to parse.
func TestIllegalIDIsMisplaced(t *testing.T) {
	root := cleanTree(t)
	write(t, root, "projects/UPPER/.para/state.toml",
		"name = \"Upper\"\ndescription = \"An id no locator can address.\"\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindMisplaced)
	if len(got) != 1 || !strings.HasPrefix(got[0], "projects/UPPER:") {
		t.Fatalf("misplaced findings = %v, want one for projects/UPPER", got)
	}
	assertOnly(t, rep, doctor.KindMisplaced)
}

// TestInvalidConfig covers the other truth file (§2.2): §10's `invalid` opens
// with "unparseable TOML" and does not say which one it means.
func TestInvalidConfig(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"unparseable TOML", "this is not toml\n", "is not readable: invalid TOML"},
		{"an unknown key", "[project]\nstale-aftr = 30\n", "unknown key"},
		{"a value of the wrong type", "[project]\nstale-after = \"soon\"\n", "project.stale-after"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := cleanTree(t)
			write(t, root, "projects/acme/.para/config.toml", c.body)

			rep := run(t, root, doctor.Options{})

			got := findings(rep, doctor.KindInvalid)
			if !anyContains(got, c.want) {
				t.Fatalf("invalid findings = %v, want one mentioning %q", got, c.want)
			}
			// The finding names the file that is wrong, once — not one
			// stale-projection per descendant whose chain it broke.
			if stale := findings(rep, doctor.KindStaleProjection); len(stale) != 0 {
				t.Errorf("stale-projection findings = %v, want none: nothing can be compared", stale)
			}
			for _, line := range got {
				if strings.Contains(line, root) {
					t.Errorf("finding %q leaks an absolute path", line)
				}
			}
		})
	}
}

// TestOrphanInsideParaAndClaude: §21.2 says doctor "walks every directory
// including inside content", and para's own directories are directories. An
// entity hand-moved into one is exactly the vanishing §8.5 names.
func TestOrphanInsideParaAndClaude(t *testing.T) {
	for _, dir := range []string{"projects/acme/.para/stash", ".claude/stash"} {
		t.Run(dir, func(t *testing.T) {
			root := cleanTree(t)
			write(t, root, dir+"/.para/state.toml",
				"name = \"Stashed\"\ndescription = \"Somewhere no read will find it.\"\n")

			rep := run(t, root, doctor.Options{})

			got := findings(rep, doctor.KindOrphan)
			if len(got) != 1 || !strings.HasPrefix(got[0], dir+":") {
				t.Fatalf("orphan findings = %v, want one for %s", got, dir)
			}
		})
	}
}

// TestParaDirectoryIsNotUntracked: walking into para's own directories must not
// make them look like loose filing. `projects/.para/` sits directly in a
// bucket, and a reserved word can never be an id (§1.4).
func TestParaDirectoryIsNotUntracked(t *testing.T) {
	rep := run(t, cleanTree(t), doctor.Options{})
	if !rep.Clean() {
		t.Fatalf("a rebuilt tree is not clean: %v", lines(rep))
	}
}

// TestDoctorAtAnArchiveStub: scoping at a stub must not read it as an entity
// whose state.toml is missing. `list` answers correctly for the same locator,
// and para must not contradict itself about one of them.
func TestDoctorAtAnArchiveStub(t *testing.T) {
	root := cleanTree(t)
	e := mutate.NewEnv(root, clock.Fixed{At: now(t)})
	plan, err := e.PlanArchive(loc(t, "projects.acme.objectives.q1"))
	if err != nil {
		t.Fatalf("PlanArchive: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if _, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	rep := run(t, root, doctor.Options{Scope: loc(t, "archive.projects.acme")})

	if !rep.Clean() {
		t.Fatalf("doctor at a stub is not clean: %v", lines(rep))
	}
}

// TestDoctorReportsTheBlockLeftByTurningTheKeyOff is the reporting half of
// Phase 13's carried-forward debt. `emit.gitattributes = false` used to leave
// para's block in a file doctor said nothing about, so the tree was clean by
// doctor's account and wrong by §9's. R12: this is the mixed-content case —
// other lines beside para's block — so the file is shortened, not deleted
// (R6), and doctor names the block as the cause.
func TestDoctorReportsTheBlockLeftByTurningTheKeyOff(t *testing.T) {
	root := cleanTree(t)
	existing, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil {
		t.Fatalf("reading .gitattributes: %v", err)
	}
	write(t, root, ".gitattributes", "*.png binary\n"+string(existing))
	write(t, root, ".para/config.toml", "emit.gitattributes = false\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindStaleProjection)
	want := ".gitattributes: still holds para's block; emit.gitattributes is off"
	if !slices.Contains(got, want) {
		t.Errorf("stale-projection findings = %v, want one of them to be %q", got, want)
	}
}

// TestDoctorReportsTheEmptiedGitAttributesAsResidue is R13's .gitattributes
// twin: with nothing but para's block in the file, turning the key off means
// the file should not exist at all (R6), and doctor says so with the same
// shape mirror.ResidueDetail already uses for CLAUDE.md.
func TestDoctorReportsTheEmptiedGitAttributesAsResidue(t *testing.T) {
	root := cleanTree(t)
	write(t, root, ".para/config.toml", "emit.gitattributes = false\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindStaleProjection)
	want := ".gitattributes: should not exist; emit.gitattributes is off"
	if !slices.Contains(got, want) {
		t.Errorf("stale-projection findings = %v, want one of them to be %q", got, want)
	}
}

// TestDoctorIsCleanOnceTheBlockIsGone closes the loop §2.4 asks for: doctor
// reports it, rebuild repairs it, doctor is clean.
func TestDoctorIsCleanOnceTheBlockIsGone(t *testing.T) {
	root := cleanTree(t)
	write(t, root, ".para/config.toml", "emit.gitattributes = false\n")

	if _, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{}); err != nil {
		t.Fatalf("rebuild.Run: %v", err)
	}

	rep := run(t, root, doctor.Options{})
	if len(rep.Findings) != 0 {
		t.Errorf("doctor after rebuild = %v, want clean", rep.Findings)
	}
}

// TestDoctorIsCleanOnAThirdPartyOnlyClaudeFile is R11: `emit.claude` off,
// CLAUDE.md present holding only third-party content, no para block at all —
// the exact state the bug report observed a false stale-projection in.
// Nothing about this file is para's to have an opinion on.
func TestDoctorIsCleanOnAThirdPartyOnlyClaudeFile(t *testing.T) {
	root := cleanTree(t)
	foreign := "# Team notes\n\nRun `make check` before every commit.\n"
	write(t, root, "CLAUDE.md", foreign)

	rep := run(t, root, doctor.Options{})

	if !rep.Clean() {
		t.Fatalf("a third-party-only CLAUDE.md with emit.claude off is not clean: %v", lines(rep))
	}
}

// TestDoctorReportsTheClaudeBlockLeftByTurningTheKeyOff is R12's CLAUDE.md
// twin of TestDoctorReportsTheBlockLeftByTurningTheKeyOff: `emit.claude` off,
// CLAUDE.md holding para's block *and* other content, reported with the same
// shared sentence .gitattributes already uses.
func TestDoctorReportsTheClaudeBlockLeftByTurningTheKeyOff(t *testing.T) {
	root := cleanTree(t)
	foreign := "# Team notes\n\nRun `make check` before every commit.\n"
	block := mdfile.BeginMarker + "\n@AGENTS.md\n" + mdfile.EndMarker + "\n"
	write(t, root, "CLAUDE.md", foreign+block)

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindStaleProjection)
	want := "CLAUDE.md: still holds para's block; emit.claude is off"
	if !slices.Contains(got, want) {
		t.Errorf("stale-projection findings = %v, want one of them to be %q", got, want)
	}
}
