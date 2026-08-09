package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/colchuck-ai/para/internal/writeset"
)

// This file is the plan's Phase 14 task 1: "kill the process between each write
// in a writeset and assert the tree is always either clean or reporting exactly
// stale-projection, and that rebuild restores cleanliness (§0.2)".
//
// §0.2 chose an ordering rather than a transaction, on the argument that the
// tree has exactly one defined degraded state and that the ordering makes it the
// only state a crash can produce. That is an argument about every interleaving,
// and the only honest way to hold it is to try them all: for each command below
// the harness sweeps PARA_CRASH_AFTER from 1 upward, killing para after one more
// filesystem operation each time, until the command survives. Every kill is
// followed by the two questions §2.4 answers — is the tree in the defined
// degraded state, and does `rebuild` return it to clean.
//
// The subprocess is the point. An in-process simulation would exercise the error
// paths a real kill skips, and §0.2's claim is about a process that stopped, not
// one that returned.

// crashCommand is one mutating command, with the setup a tree needs before it
// can be run.
type crashCommand struct {
	name string
	// setup runs against a fresh tree, uncrashed.
	setup [][]string
	// args is the command whose every write is crashed at, in turn.
	args []string
}

// crashCommands covers every shape of write set the design has: the two-subject
// create, a field change, the two journal-only verbs, the three relocating verbs
// (rename, archive, delete), the config write, and the surface refresh.
var crashCommands = []crashCommand{
	{
		name: "add a project",
		args: []string{"add", "projects.acme", "--name", "Acme", "--description", "Rebuild the consumer."},
	},
	{
		name: "add a key-result, whose parent chain is three deep",
		setup: [][]string{
			{"add", "projects.acme", "--name", "Acme", "--description", "Rebuild the consumer."},
			{"add", "projects.acme.objectives.q1", "--name", "Q1", "--description", "Move the funnel."},
		},
		args: []string{
			"add", "projects.acme.objectives.q1.key-results.signups",
			"--name", "Signups", "--type", "ratio", "--start", "480/9000", "--target", "2000/12000",
		},
	},
	{
		name: "set a field",
		setup: [][]string{
			{"add", "projects.acme", "--name", "Acme", "--description", "Rebuild the consumer."},
		},
		args: []string{"set", "projects.acme", "--status", "blocked", "--note", "waiting on ingest"},
	},
	{
		name: "note",
		setup: [][]string{
			{"add", "projects.acme", "--name", "Acme", "--description", "Rebuild the consumer."},
		},
		args: []string{"note", "projects.acme", "the ingest team is blocked"},
	},
	{
		name: "measure",
		setup: [][]string{
			{"add", "projects.acme", "--name", "Acme", "--description", "Rebuild the consumer."},
			{"add", "projects.acme.objectives.q1", "--name", "Q1", "--description", "Move the funnel."},
			{
				"add", "projects.acme.objectives.q1.key-results.signups",
				"--name", "Signups", "--type", "ratio", "--start", "480/9000", "--target", "2000/12000",
			},
		},
		args: []string{"measure", "projects.acme.objectives.q1.key-results.signups", "880/11000"},
	},
	{
		name: "move, which renames before it writes",
		setup: [][]string{
			{"add", "areas.health", "--name", "Health", "--description", "Staying in one piece."},
			{"add", "areas.health.training", "--name", "Training", "--description", "The weekly plan."},
			{"add", "areas.fitness", "--name", "Fitness", "--description", "The other one."},
		},
		args: []string{"move", "areas.health.training", "areas.fitness.training"},
	},
	{
		name: "archive, which leaves a stub behind",
		setup: [][]string{
			{"add", "areas.health", "--name", "Health", "--description", "Staying in one piece."},
			{"add", "areas.health.training", "--name", "Training", "--description", "The weekly plan."},
		},
		args: []string{"archive", "areas.health.training"},
	},
	{
		name: "remove, which deletes before it writes",
		setup: [][]string{
			{"add", "projects.acme", "--name", "Acme", "--description", "Rebuild the consumer."},
		},
		args: []string{"remove", "projects.acme", "--force"},
	},
	{
		name: "add a skill, which also writes a rule outside the subject",
		args: []string{"add", "skills.report", "--name", "Report", "--description", "when asked for the number"},
	},
	{
		name: "config set, which writes truth and one projection",
		args: []string{"config", "set", "project.stale-after", "30"},
	},
	{
		name: "config set emit.claude, which refreshes eight files and a mirror",
		setup: [][]string{
			{"add", "skills.report", "--name", "Report", "--description", "when asked for the number"},
		},
		args: []string{"config", "set", "emit.claude", "true"},
	},
	{
		// Phase 14's own new write path, and the one that shortens a file
		// rather than writing or removing it (§9).
		name: "config set emit.gitattributes false, which shortens a file",
		args: []string{"config", "set", "emit.gitattributes", "false"},
	},
	{
		// §1.6's cascade: reinstates the ancestors it needs and leaves a stub
		// where an archived sibling stays put, so its relocation phase has more
		// moves in it than any other verb's.
		name: "unarchive, which reinstates a chain",
		setup: [][]string{
			{"add", "areas.health", "--name", "Health", "--description", "Staying in one piece."},
			{"add", "areas.health.training", "--name", "Training", "--description", "The weekly plan."},
			{"archive", "areas.health.training"},
			{"archive", "areas.health"},
		},
		args: []string{"unarchive", "archive.areas.health.training"},
	},
	{
		// §18.4's other half: deletes para's footprint throughout a subtree and
		// keeps everything else, so its prune list is long and its write list is
		// only the parent's.
		name: "remove --keep-files, which prunes throughout a subtree",
		setup: [][]string{
			{"add", "projects.acme", "--name", "Acme", "--description", "Rebuild the consumer."},
			{"add", "projects.acme.objectives.q1", "--name", "Q1", "--description", "Move the funnel."},
		},
		args: []string{"remove", "projects.acme", "--keep-files", "--force"},
	},
}

// TestInitCrashMatrix is the same sweep over the one command that has no tree to
// start from, and therefore no place in crashCommands.
//
// Its degraded states are the two `mutate.Init` argues for. Before the marker
// lands the directory is not a tree, so the question is whether `init` can be run
// again and reach a clean tree; after it lands, the question is the ordinary one.
// Both are asserted, which is what makes "the root is init's last subject" a
// tested decision rather than a recorded intention.
func TestInitCrashMatrix(t *testing.T) {
	if !testHooksEnabled {
		t.Skip("the crash matrix needs -tags para_testhooks so PARA_CRASH_AFTER is honoured; run `make test`")
	}
	t.Parallel()

	for n := 1; ; n++ {
		parent := t.TempDir()
		root := filepath.Join(parent, "brain")

		out, code := runParaIn(t, parent, []string{"PARA_CRASH_AFTER=" + strconv.Itoa(n)}, "init", "brain")
		if code != writeset.CrashExitCode {
			if code != 0 {
				t.Fatalf("crash %d: init failed rather than being killed (exit %d):\n%s", n, code, out)
			}
			return
		}

		if _, err := os.Stat(filepath.Join(root, ".para", "tree.toml")); err != nil {
			// No marker: not a tree at all, and the repair is to run init again.
			if out, code := runParaIn(t, parent, nil, "init", "brain"); code != 0 {
				t.Fatalf("crash %d left a directory init will not complete (exit %d):\n%s", n, code, out)
			}
		}
		// init's own allowance: every path is one it was in the middle of
		// creating, since the directory did not exist at all beforehand.
		assertDegradedStateIsDefined(t, root, n, everyPath(t, root))
	}
}

// everyPath is pathsIn as an allowance: for `init`, which starts from nothing,
// there is no path in the tree it did not create.
func everyPath(t *testing.T, root string) map[string]bool {
	t.Helper()
	return pathsIn(t, root)
}

// TestCrashMatrix is §0.2's claim, checked at every point a crash can happen.
func TestCrashMatrix(t *testing.T) {
	if !testHooksEnabled {
		t.Skip("the crash matrix needs -tags para_testhooks so PARA_CRASH_AFTER is honoured; run `make test`")
	}

	for _, tc := range crashCommands {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base := crashBaseTree(t, tc)
			allowed := allowedAdvisories(t, base, tc.args)

			for n := 1; ; n++ {
				root := filepath.Join(t.TempDir(), "brain")
				copyTree(t, base, root)

				out, code := runPara(t, root, []string{"PARA_CRASH_AFTER=" + strconv.Itoa(n)}, tc.args...)
				if code != writeset.CrashExitCode {
					if code != 0 {
						t.Fatalf("crash %d: the command failed rather than being killed (exit %d):\n%s", n, code, out)
					}
					// The command completed with the counter never reached, so
					// n is past its last write and the sweep is done.
					if n == 1 {
						t.Fatalf("%v performed no writes at all", tc.args)
					}
					return
				}

				assertDegradedStateIsDefined(t, root, n, allowed)
			}
		})
	}
}

// assertDegradedStateIsDefined is the pair of questions §2.4 answers: the tree a
// crash left is the defined degraded state, and `rebuild` returns it to clean.
//
// "Defined" needs one clause §0.2 does not spell out, and the harness found it
// rather than assuming it. A relocation has to create the archive stub
// directories before it can rename into them (§1.6, §18.5), so a crash between
// the mkdir and the rename leaves an empty directory in a bucket — which doctor
// reports as `untracked`, §21.2's advisory "loose filing". It is never an error,
// it is never truth lost, `rebuild` will not remove it (para does not delete a
// directory it cannot prove it created), and re-running the command completes
// it. So the exact claim is:
//
//   - no error-severity finding but `stale-projection`, ever;
//   - after a rebuild, no error-severity finding at all;
//   - the only advisory left is `untracked`, at a path the command was in the
//     middle of dealing with — which is what allowed is for. Without that last
//     clause the assertion would pass for any advisory the tree acquired.
func assertDegradedStateIsDefined(t *testing.T, root string, n int, allowed map[string]bool) {
	t.Helper()

	rep := doctorReport(t, root, fmt.Sprintf("after crash %d", n))
	assertOnlyDefinedFindings(t, rep, allowed, fmt.Sprintf("crash %d", n), "stale-projection")

	if out, code := runPara(t, root, nil, "rebuild"); code != 0 {
		t.Fatalf("crash %d: rebuild failed (exit %d):\n%s", n, code, out)
	}

	rep = doctorReport(t, root, fmt.Sprintf("after crash %d and a rebuild", n))
	assertOnlyDefinedFindings(t, rep, allowed, fmt.Sprintf("crash %d, after a rebuild", n))
}

// assertOnlyDefinedFindings holds the two halves of the claim: errors are
// confined to allowedErrors, and advisories to an `untracked` path the crashed
// command was in the middle of dealing with.
func assertOnlyDefinedFindings(t *testing.T, rep report, allowed map[string]bool, when string, allowedErrors ...string) {
	t.Helper()
	for _, f := range rep.Findings {
		switch f.Severity {
		case "advisory":
			if f.Finding != "untracked" {
				t.Fatalf("%s left the advisory %s %s (%s), which is not one a crash may produce", when, f.Finding, f.Path, f.Detail)
			}
			if !allowed[f.Path] {
				t.Fatalf("%s reports %s as untracked, and the command has no business with that path", when, f.Path)
			}
		default:
			if !slices.Contains(allowedErrors, f.Finding) {
				t.Fatalf("%s left %s %s (%s); §0.2 promises only %v", when, f.Finding, f.Path, f.Detail, allowedErrors)
			}
		}
	}
}

// allowedAdvisories is the set of paths a crash of this command may legitimately
// leave `untracked`, worked out by running the command to completion once.
//
// The obvious control — "any path that was not in the tree before" — is wrong,
// and the harness proved it twice. `remove --keep-files` leaves a plain
// directory where an entity was, on purpose (§18.4), and it was there before.
// `unarchive` turns an archived entity into a stub and then removes the stub,
// and it was there before too. What both have in common is that the *completed*
// command accounts for the path: it either leaves it untracked itself, or
// removes it. Anything else is a path the command has no business with, and the
// base tree is asserted clean so nothing arrives pre-untracked.
func allowedAdvisories(t *testing.T, base string, args []string) map[string]bool {
	t.Helper()

	if rep := doctorReport(t, base, "over the base tree"); !rep.Clean {
		t.Fatalf("the base tree is not clean before the command runs: %+v", rep.Findings)
	}
	before := pathsIn(t, base)

	done := filepath.Join(t.TempDir(), "brain")
	copyTree(t, base, done)
	if out, code := runPara(t, done, nil, args...); code != 0 {
		t.Fatalf("the command failed on an uncrashed run (exit %d):\n%s", code, out)
	}
	after := pathsIn(t, done)

	allowed := map[string]bool{}
	for path := range after {
		if !before[path] {
			allowed[path] = true // created by the command
		}
	}
	for path := range before {
		if !after[path] {
			allowed[path] = true // removed by the command
		}
	}
	for _, f := range doctorReport(t, done, "after an uncrashed run").Findings {
		if f.Finding == "untracked" {
			allowed[f.Path] = true // left untracked by the command, on purpose
		}
	}
	return allowed
}

// pathsIn is every path in a tree, root-relative and slash-separated — the form
// doctor reports.
func pathsIn(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return out
}

// crashBaseTree builds the tree each iteration of the sweep starts from: an
// `init` plus the command's setup, all of it uncrashed.
func crashBaseTree(t *testing.T, tc crashCommand) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "brain")
	if out, code := runParaIn(t, filepath.Dir(root), nil, "init", "brain"); code != 0 {
		t.Fatalf("init: exit %d:\n%s", code, out)
	}
	for _, args := range tc.setup {
		if out, code := runPara(t, root, nil, args...); code != 0 {
			t.Fatalf("setup %v: exit %d:\n%s", args, code, out)
		}
	}
	return root
}

// report is doctor's --json output (§23), which carries the exit code it is
// about to use and every finding as data rather than prose.
type report struct {
	Clean    bool `json:"clean"`
	Findings []struct {
		Severity string `json:"severity"`
		Finding  string `json:"finding"`
		Path     string `json:"path"`
		Detail   string `json:"detail"`
	} `json:"findings"`
}

func doctorReport(t *testing.T, root, when string) report {
	t.Helper()
	out, code := runPara(t, root, nil, "doctor", "--json")
	if code != 0 && code != 1 && code != 2 {
		t.Fatalf("doctor %s: exit %d:\n%s", when, code, out)
	}
	var rep report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("doctor %s: unreadable --json (%v):\n%s", when, err, out)
	}
	return rep
}

// runPara runs the real binary in root. testscript.Main has already copied this
// test binary onto $PATH as `para`, so this is the same program the script tests
// drive — no separate build, and no in-process shortcut.
func runPara(t *testing.T, root string, env []string, args ...string) (string, int) {
	t.Helper()
	return runParaIn(t, root, env, args...)
}

func runParaIn(t *testing.T, dir string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("para", args...)
	cmd.Dir = dir
	// A fixed clock, for the same reason every script test fixes one (§0.2).
	cmd.Env = append(os.Environ(), "PARA_NOW=2026-03-05T17:00:00Z", "PARA_TZ=UTC")
	cmd.Env = append(cmd.Env, env...)

	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode()
	}
	t.Fatalf("running para %v: %v", args, err)
	return "", 0
}

// copyTree reproduces a tree byte for byte, so every iteration of the sweep
// starts from the same bytes rather than from a tree an earlier iteration
// half-wrote.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case info.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			return copyFile(path, target, info.Mode())
		}
	})
	if err != nil {
		t.Fatalf("copying %s: %v", src, err)
	}
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
