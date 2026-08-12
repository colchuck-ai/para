package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// This file is the plan's Phase 14 task 3: "a test enumerates §26's fenced
// blocks and fails if any line is not covered by a script test".
//
// §26 is the acceptance suite — the design document says so outright — and the
// ground rules say the scripts in testdata/script are its executable form. What
// was missing until now was anything that noticed when the two drifted apart:
// a command added to §26 was covered only if somebody remembered, and a command
// deleted from §26 left a script asserting behaviour the spec no longer claims.
//
// So the coverage is a table rather than a heuristic. A heuristic — "some script
// mentions this verb" — passes for a `list` line nobody transcribed as long as
// some other `list` is covered, which is exactly the case worth catching. The
// table names, for every command §26 runs, the script that runs it and a line of
// that script proving it. Both halves are checked: a §26 line with no entry
// fails, an entry for a line §26 no longer has fails, and an entry whose proof
// is not in the script it names fails.

// The sweep is over §26's *fenced* blocks, which is what the plan asked for and
// is also a real limit worth naming. §26 states four refusals in prose rather
// than in a fence — `para add project acme-migration` again (exists), `para add
// project acme.b` (wrong arity — a project is one segment, it cannot nest),
// `para add project objectives` (reserved word), and a key-result with no
// `--target` — and this test never sees them. All four are transcribed in
// write.txtar; none of them is held there by anything but the fact that
// somebody wrote them.
//
// The other limit: the table pins the command lines, not §26's expected
// *output*. `showing 3 of 3`, `moved …`, and the dated `stale-projection` line
// are asserted by the scripts, and several of the proofs below carry the
// assertion line for exactly that reason — but a §26 output that changed with no
// command change would not fail here.

// spec26 is the design document, read as text.
const spec26File = "../../para-design-v4.md"

// scriptDir holds the executable form of §26.
const scriptDir = "../../testdata/script"

// covered is where one §26 command is run, and the script lines that run it.
//
// Proof is matched **between newlines**, so it is one or more whole lines of the
// script rather than a substring of one. That is not fussiness: the review that
// found this table's first version showed what a substring proof lets through —
// `exec para review --stale --behind` is a substring of `… --stale --behind
// --skills`, `exec para archive areas.health` of `… areas.health.training`, and
// every `exec para X` is a substring of `! exec para X`, so a script that flipped
// a success to a refusal would still have been accepted. A whole line cannot be
// any of those.
//
// Where §26 states an outcome, the proof carries the assertion line too, so the
// table pins what the command *did* and not only that it ran.
//
// The lines are the script's spelling rather than §26's, and the two differ on
// purpose in a few places, each a decision recorded in the script's own header:
//
//   - §26 quotes with `"` and testscript quotes with `'`.
//   - §26's `para show .` carries a trailing comment saying where it is run
//     from, which is a note to the reader rather than part of the command.
//   - §26's `--archived` boundary refusal renames `project.acme-migration` to
//     `acme-renamed`; the script asks for the same name on both ends (a
//     no-op rename) since only the archive-boundary mismatch is under test,
//     not the destination name.
type covered struct {
	script string
	proof  string
}

var spec26Coverage = map[string]covered{
	`para init brain`: {"init.txtar", "exec para init brain"},

	`para add project acme-migration --name "Acme migration" --description "Rebuild the consumer so it stops falling over under replay load."`: {
		"write.txtar", "exec para add project acme-migration --name 'Acme migration' --description 'Rebuild the consumer so it stops falling over under replay load.'",
	},
	`para add objective acme-migration.q1-growth --name "Grow signups" --description "Move the top of the funnel."`: {
		"write.txtar", "exec para add objective acme-migration.q1-growth --name 'Grow signups' --description 'Move the top of the funnel.'",
	},
	`para add key-result acme-migration.q1-growth.signups --name "Weekly signups" --type ratio --start 480/9000 --target 2000/12000 --due 2026-09-30`: {
		"write.txtar", "exec para add key-result acme-migration.q1-growth.signups --name 'Weekly signups' --type ratio --start 480/9000 --target 2000/12000 --due 2026-09-30",
	},
	`para add area health --name "Health" --description "Staying in one piece."`: {
		"write.txtar", "exec para add area health --name Health --description 'Staying in one piece.'",
	},
	`para add area health.training --name "Training" --description "The weekly plan."`: {
		"write.txtar", "exec para add area health.training --name Training --description 'The weekly plan.'",
	},
	`para add skill signups-report --name "Signups report" --description "when asked for the weekly signups number" --scope project.acme-migration,area.growth`: {
		"write.txtar", "exec para add skill signups-report --name 'Signups report' --description 'when asked for the weekly signups number' --scope project.acme-migration,area.growth",
	},
	`para add skill commit-style --name "Commit style" --description "when writing a commit message"`: {
		"write.txtar", "exec para add skill commit-style --name 'Commit style' --description 'when writing a commit message'",
	},

	`para list project --tags kafka --sort attention`: {
		"read.txtar", "exec para list project --tags kafka --sort attention\ncmp stdout $WORK/want-list-kafka.txt",
	},
	`para list --status blocked --all`: {"read.txtar", "exec para list --status blocked --all\nstdout '^showing '"},
	`para list project --archived`:     {"read.txtar", "exec para list project --archived\nstdout 'website'"},
	`para path area health.training`: {
		"tree.txtar", "exec para path area health.training\nstdout '^\\S*brain[/\\\\]areas[/\\\\]health[/\\\\]training\\n$'",
	},
	`para show . # from inside areas/health/training`: {"read.txtar", "exec para show .\nstdout '^area  health\\.training$'"},

	`para set project acme-migration --status blocked`: {
		"write.txtar", "! exec para set project acme-migration --status blocked\nstderr '^error: --note is required when setting status to blocked$'",
	},
	`para set project acme-migration --status blocked --note "waiting on the ingest team"`: {
		"write.txtar", "exec para set project acme-migration --status blocked --note 'waiting on the ingest team'\ncmp stdout $WORK/want-set-blocked.txt",
	},
	`para set project acme-migration --status blocked --note "still waiting"`: {
		"write.txtar", "exec para set project acme-migration --status blocked --note 'still waiting'\nstdout '^no change \\(status already blocked\\); note recorded\\n'",
	},
	`para move area health.training fitness.training`: {
		"relocate.txtar", "exec para move area health.training fitness.training\ncmp stdout $WORK/want-move.txt",
	},
	// The rename target differs from §26's `acme-renamed` — see the header note
	// on covered above — but the proof line is the script's actual spelling,
	// which is what has to appear verbatim.
	`para move project acme-migration acme-renamed --archived`: {
		"relocate.txtar", "! exec para move project acme-migration acme-migration --archived\nstderr '^error: --archived means both ends are archived; project\\.acme-migration is live$'",
	},

	`para measure acme-migration.q1-growth.signups 880/11000 --at 2026-01-03`: {
		"write.txtar", "exec para measure acme-migration.q1-growth.signups 880/11000 --at 2026-01-03\ncmp stdout $WORK/want-measured.txt",
	},
	`para measure acme-migration.q1-growth.signups 0.08`: {
		"write.txtar", "! exec para measure acme-migration.q1-growth.signups 0.08\nstderr 'value 0.08 is not a ratio \\(type ratio expects <numerator>/<denominator>\\)'",
	},
	`para measure acme-migration.q1-growth.signups 900/11000 --at 2026-01-03`: {
		"write.txtar", "! exec para measure acme-migration.q1-growth.signups 900/11000 --at 2026-01-03\nstderr 'a measurement already exists at 2026-01-03T08:00:00Z \\(2026-01-03T00:00:00-08:00 local\\)'",
	},
	`para log project acme-migration --kind change --limit 3`: {
		"write.txtar", "exec para log project acme-migration --kind change --limit 3",
	},
	`para activity project acme-migration --recursive --since 2026-01-01`: {
		"read.txtar", "exec para activity project acme-migration --recursive --since 2026-01-01\ncmp stdout $WORK/want-activity-recursive.txt",
	},

	`para archive area health.training`: {
		"relocate.txtar", "exec para archive area health.training\ncmp stdout $WORK/want-archive-training.txt",
	},
	`para unarchive area health`: {
		"relocate.txtar", "! exec para unarchive area health\nstderr '^error: nothing to unarchive — archive\\.area\\.health is a stub, not an entity$'",
	},
	`para archive area health`: {
		"relocate.txtar", "exec para archive area health\ncmp stdout $WORK/want-archive-health.txt",
	},
	`para unarchive area health.training`: {
		"relocate.txtar", "exec para unarchive area health.training\ncmp stdout $WORK/want-unarchive.txt",
	},
	`para unarchive project old-migration`: {
		"relocate.txtar", "! exec para unarchive project old-migration\nstderr '^error: projects\\.old-migration exists; rename it or leave this archived$'",
	},

	`para review --stale --behind`: {
		"review.txtar", "exec para review --stale --behind\ncmp stdout $WORK/want-review-stale-behind.txt",
	},

	// §26 runs `para doctor` twice with two different outcomes — the failing scan
	// that opens the block and the clean one that closes it — so the two are
	// separate entries and each proof pins its own outcome. Collapsing them, as
	// the first version of this table did, left §26's clean-doctor line with no
	// coverage requirement at all.
	`para doctor#1`: {"repair.txtar", "! exec para doctor\ncmp stdout $WORK/want-doctor-stale.txt"},
	`para doctor#2`: {"repair.txtar", "exec para doctor\nstdout '^clean$'"},

	`para rebuild project acme-migration --dry-run`: {
		"repair.txtar", "exec para rebuild project acme-migration --dry-run\nstdout '^would rewrite  projects/acme-migration/ACTIVITY\\.md$'",
	},
	`para rebuild project acme-migration`: {
		"repair.txtar", "exec para rebuild project acme-migration\nstdout '^rewrote  projects/acme-migration/ACTIVITY\\.md$'",
	},

	`para config set --at project project.stale-after 30`: {
		"config.txtar", "exec para config set --at project project.stale-after 30\ncmp stdout $WORK/want-set-projects.txt",
	},
	`para config show project.stale-after project.acme-migration`: {
		"config.txtar", "exec para config show project.stale-after project.acme-migration\ncmp stdout $WORK/want-chain.txt",
	},
	`para config set emit.claude true`: {"claude.txtar", "exec para config set emit.claude true"},
}

// TestSpec26IsCovered is the conformance sweep itself.
func TestSpec26IsCovered(t *testing.T) {
	commands, err := spec26Commands(spec26File)
	if err != nil {
		t.Fatalf("reading §26: %v", err)
	}
	if len(commands) == 0 {
		t.Fatal("§26 yielded no commands — the section headings or fences must have moved")
	}

	scripts := map[string]string{}
	seen := map[string]bool{}
	occurrences := map[string]int{}
	for _, cmd := range commands {
		// §26 runs some commands more than once, and not always to the same
		// end — `para doctor` appears as the failing scan and again as the clean
		// one. Keying on the command text alone made the second occurrence
		// coverable by the first's entry, so a key gains a `#n` suffix from its
		// second appearance on.
		occurrences[cmd]++
		key := cmd
		if n := occurrences[cmd]; n > 1 || spec26Coverage[cmd+"#1"].script != "" {
			key = fmt.Sprintf("%s#%d", cmd, n)
		}
		seen[key] = true

		cov, ok := spec26Coverage[key]
		if !ok {
			t.Errorf("§26 runs %q (occurrence %d) and no script test covers it — transcribe it and record the script here",
				cmd, occurrences[cmd])
			continue
		}
		body, ok := scripts[cov.script]
		if !ok {
			data, err := os.ReadFile(filepath.Join(scriptDir, cov.script))
			if err != nil {
				t.Errorf("§26's %q claims coverage in %s, which will not read: %v", cmd, cov.script, err)
				continue
			}
			// The newline fences below need one at each end of the file too, so
			// a proof matching the first or last line is not a special case.
			body = "\n" + string(data) + "\n"
			scripts[cov.script] = body
		}
		// Between newlines: the proof is whole lines of the script, never a
		// substring of one. See covered.
		if !strings.Contains(body, "\n"+cov.proof+"\n") {
			t.Errorf("§26's %q claims coverage in %s, which has no such line(s):\n%s", cmd, cov.script, cov.proof)
		}
	}

	// The other direction: an entry for a command §26 no longer runs is a script
	// asserting behaviour the spec has stopped claiming, which is worth knowing
	// about even though it is the less dangerous of the two.
	var stale []string
	for cmd := range spec26Coverage {
		if !seen[cmd] {
			stale = append(stale, cmd)
		}
	}
	sort.Strings(stale)
	for _, cmd := range stale {
		t.Errorf("this table covers %q, which §26 no longer runs", cmd)
	}
}

// spec26Commands returns every `para …` invocation in §26's fenced blocks, in
// document order, whitespace collapsed and shell continuations joined.
//
// It reads the document rather than a copy of it, which is the point: a §26 that
// grows a command fails this test on the next run instead of on the next reader.
func spec26Commands(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var (
		out        []string
		inSection  bool
		inFence    bool
		spaces     = regexp.MustCompile(`\s+`)
		joined     string
		continuing bool
	)
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "## 26."):
			inSection = true
			continue
		case inSection && strings.HasPrefix(line, "## "):
			inSection = false
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			continue
		}

		trimmed := strings.TrimSpace(line)
		if continuing {
			joined += " " + strings.TrimSpace(strings.TrimSuffix(trimmed, `\`))
			if !strings.HasSuffix(trimmed, `\`) {
				out = append(out, spaces.ReplaceAllString(joined, " "))
				continuing = false
			}
			continue
		}

		// A `$ ` prompt marks an interactive transcript; the `bash` blocks have
		// none. Either way what follows is the command.
		cmd := strings.TrimPrefix(trimmed, "$ ")
		if !strings.HasPrefix(cmd, "para ") {
			continue
		}
		if strings.HasSuffix(cmd, `\`) {
			joined = strings.TrimSpace(strings.TrimSuffix(cmd, `\`))
			continuing = true
			continue
		}
		out = append(out, spaces.ReplaceAllString(cmd, " "))
	}
	return out, nil
}
