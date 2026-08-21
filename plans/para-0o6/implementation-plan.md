# para — `CLAUDE.md` as a marker-scoped block (implementation plan)

Continues the numbering in [`docs/implementation-plan.md`](../../docs/implementation-plan.md)
(Phases 0–15) and [`../fixes/implementation-plan.md`](../fixes/implementation-plan.md) (Phases 16–22).
[`requirements.md`](requirements.md) is the requirement set; `para-design-v4.md` becomes normative
again once Phase 23 lands.

Fixes **para-0o6**. Four phases, ordered so every phase's dependencies are already merged.

**Pre-1.0.** No migration, no adoption pass for marker-less `CLAUDE.md` files, no deprecation window.

---

## 0. Ground rules for this work

Phase 0's rules in the parent plan still hold — test-first per package, `testscript` for the surface,
golden files with `-update`, renderers as pure functions of `In`. Four additions specific to this fix.

### 0.1 R9 is the safety net, and it is what makes this small

For a tree where para is the only writer of `CLAUDE.md`, this change is observationally inert: with
the key on, the file gains two marker lines; with the key off, the file still ends up gone, and
`rebuild` still prints `removed  CLAUDE.md`. Every existing `testdata/script` fixture builds its tree
with para commands, so every one of them is such a tree.

**Consequence:** §26's transcripts do not move, `cmd/para/conformance_test.go` stays green throughout,
and `doctor`'s eight residue goldens (`internal/doctor/doctor_test.go:505-512`) stay green. The only
goldens that change are the ones holding `CLAUDE.md`'s bytes (two marker lines) and the
`.gitattributes` block (one line removed). If a phase turns a fixture red for any other reason, that
is a signal the change has grown past its requirements — stop and reread R9 rather than updating the
fixture.

### 0.2 The bug stays open until Phase 25

Phase 24 makes para *write* a block. Phase 25 makes para *remove only* its block. Between them, para
emits markers and still deletes whole files when the key is off, so para-0o6 is not fixed at the
Phase 24 boundary even though the tree is green. This is the one place in the plan where "green" and
"fixed" come apart, and it is stated here so it is not rediscovered as a surprise.

Do not close para-0o6 before Phase 26's regression script (R18) passes.

### 0.3 The gate is `AppendDelimited`, and it is already written

`internal/mdfile` needs no new primitives. `AppendDelimited` (`block.go:92`), `RemoveDelimited`
(`block.go:131`), and `MarkdownMarkers` (`block.go:28`) are the whole mechanism, and `.gitattributes`
is a working reference implementation of every step — emit, shorten, report, repair. When a task below
is ambiguous, read what `gitAttributesRenderer` and `gitAttributesResidue` do and do that.

### 0.4 `go test ./...` is not the test suite

The txtar integration suite sits behind a build tag and is silently **SKIPped** without it. Use
`make test`, or `go test ./... -tags para_testhooks -race -count=1`. A phase that "passes" without the
tag has not run the fixtures that R9 depends on.

---

## Phase 23 — Amend `para-design-v4.md`

**Status: done, not yet committed.**

Spec first. The document is normative and doubles as the acceptance suite, so nothing downstream can
be reviewed against anything until it says the new thing. Eight sites, one of which is the actual
point.

**Tasks**

1. **§2.2 — replace the membership test (R0).** This is the task; the rest are consequences. The
   table currently sorts files by "is this a pure projection of `.para/` state?". Write in the second
   question — "does any tool other than para write to this path?" — state that it, and not the
   projection question, decides merge posture and delete-versus-shorten, and show the eight rows
   sorted by it. Say outright that `AGENTS.md` was on the right side by accident of having prose, and
   that this is why the principle is being recorded.
2. **§2.2 — the `CLAUDE.md` row** (line 378): generated-from becomes "a delimited para-owned block";
   human-owned part becomes "everything outside the markers (§6)"; merge posture becomes "normal".
   The row now reads identically to `AGENTS.md`'s.
3. **§2.1 — drop `CLAUDE.md`** from the wholly-generated list (line 367). Three names remain, and the
   sentence needs no other change.
4. **§6.1 — rewrite the `CLAUDE.md` paragraph** (line 834). "Wholly para's, and a pointer file" becomes
   marker-scoped; show the marker shape as §6 already does for `AGENTS.md`; keep the import list and
   the argument for it verbatim, because R-non-goals leave that architecture untouched. Add R10: the
   flag answers "is para's block present", not "does this file exist".
5. **§9 — delete the `**/CLAUDE.md` line** (line 1063) and check the surrounding prose (lines
   1071–1078), which argues `merge=ours` is safe "because the file is fully generated". That argument
   is still true of the three files that remain; make sure it does not read as covering a fourth.
6. **§9 — the emptied-file rule (R6).** §9 currently promises that turning `emit.gitattributes` off
   "shortens the file rather than deleting it" and cites the asymmetry with `emit.claude` as
   *justified*. Both halves change: the asymmetry is gone, and an emptied file is deleted. Record why
   (requirements §6) so the next reader does not restore the old sentence as an obvious fix.
7. **§12 — the principles list** (line 1141): "wholly generated files have no body" keeps its wording
   but no longer covers `CLAUDE.md`. Add a reversal row: "`CLAUDE.md` wholly generated" → "`CLAUDE.md`
   marker-scoped", with what it costs (a marker pair in a pointer file; `emit.claude` no longer
   implies the file's absence) and what it buys (composability with every other tool that writes
   there, and no silent `merge=ours` loss).
8. **§25 — amend the `emit.claude` decision** (line 1191). It reads as though the flag produces
   `CLAUDE.md`; it produces a block in it. One clause.
9. **§10 — residue** stays a `stale-projection` and the finding set stays closed at eleven. Confirm
   the section does not name `CLAUDE.md` as the only file that can be residue; after R6 both
   block-scoped files can be.
10. **§26 — verify, do not rewrite.** Read every transcript that mentions `CLAUDE.md` (lines 2009,
    2177–2178) and confirm R9 holds: `init`'s "no CLAUDE.md, no .claude/" is still true of a fresh
    tree, and `config set emit.claude true`'s "wrote … (8 files)" is still true. If a transcript does
    have to change, that is a finding about the requirements, not a licence to edit §26 — raise it.

**Done when** no section describes `CLAUDE.md` as a file para owns, §2.2 states the interop test as a
rule rather than as this file's special case, and §26 is byte-unchanged.

---

## Phase 24 — `internal/render`: emit a block

**Status: not started.**

**Tasks**

1. **Golden test first (R1, R3).** `claudeRenderer.Render` gains cases: no existing file → a fresh
   file containing only the marked block; an existing file with foreign content → that content
   preserved byte for byte with the block appended; an existing file with a para block in the *middle*
   → block refreshed in place, prefix and suffix untouched; an existing file with a foreign
   marker-delimited block of its own (the beads `<!-- BEGIN BEADS INTEGRATION -->` shape) → untouched.
2. **`claudeRenderer.Render` writes through `mdfile.AppendDelimited`** with `mdfile.MarkdownMarkers`
   and `in.existing(path)`. No new plumbing is needed in `render.Collect` — its two-pass design
   (`render.go:301`: resolve and read every path, *then* render) already guarantees `In.Existing` holds
   this location's bytes before `Render` runs, and its doc comment says so.
3. **`damagedBlock` takes a path (R4).** It currently closes over `gitAttributesFile`
   (`gitattributes.go:72`), and two files can now hold a damaged block. Signature becomes
   `damagedBlock(path string, err error)`; the message keeps its wording, including the repair
   sentence, which is the part that took thought and should not be rewritten.
4. **`WithoutClaudeBlock`** beside `WithoutGitAttributesBlock` (`gitattributes.go:94`) — or better,
   one `withoutBlock(markers, path, existing)` with two thin callers. Same shape: `RemoveDelimited`,
   `damagedBlock` on error, `(bytes, found, error)` out. Phase 25 is its only consumer; write it here
   because it belongs to `render`, which owns what the markers are.
5. **Delete line 44 of `gitattributes.go` (R8).** The whole line. Update the golden.
6. **Update `claude.go`'s doc comment** (line 11), which opens "wholly para's, and a pointer file with
   no prose of its own". The second half stays true — para's *block* has no prose. The first half is
   what changed, and the comment should say what it is now and cite §2.2's interop test rather than
   leaving the reader to diff it against the spec.

**Done when** the renderer's goldens cover all four existence shapes, `.gitattributes`'s golden has
three entries in its wholly-generated stanza, and `go test ./... -tags para_testhooks -race` is green.
para-0o6 is **not** fixed yet (§0.2).

---

## Phase 25 — `internal/rebuild` and `internal/doctor`: remove only the block

**Status: not started.** This is the phase that fixes the bug.

**Tasks**

1. **Test first (R5, R6, R7), at the `rebuild` altitude.** With `emit.claude` off: a `CLAUDE.md`
   holding a foreign block and para's → para's goes, the foreign one is byte-identical, the file
   survives; a `CLAUDE.md` holding only para's block → the file is deleted; a `CLAUDE.md` with no para
   marker at all → untouched, and *not* reported as a write; a `CLAUDE.md` with a begin marker and no
   end marker → a validation error naming the path.
2. **One residue function, two callers.** `claudeResidue` and `gitAttributesResidue`
   (`surface.go:75`) collapse into `blockResidue`, parameterized by the marker pair, the path, and the
   predicate that says where the file is emitted — root-only for `.gitattributes`, the eight
   `HasAgents` locations for `CLAUDE.md`. `residue` keeps its two calls and its shape.
3. **The emptiness branch is the whole of R6.** `blockResidue` reads the file, calls `withoutBlock`,
   and returns one of three things: nothing (no block to take, or the key is on and `render.For`
   already owns the file); an `Artifact{Wanted: true}` carrying the shortened bytes; or an
   `Artifact{Wanted: false}` when the shortened bytes are empty. `apply`
   (`rebuild.go:198`) needs no change at all — it already does both, which is the point.
4. **Rewrite `Artifact.Wanted`'s doc comment** (`rebuild.go:332-341`). It currently defines residue as
   "a `CLAUDE.md` left behind by turning `emit.claude` off". Both block-scoped files now produce both
   shapes, and the *condition* is what changed: residue is a block-scoped file whose block was its
   only content.
5. **`doctor`'s two sentences (R12, R13).** `staleDetail` (`subjects.go:385`) gains the `emit.claude`
   twin of `.gitattributes`'s "still holds para's block; emit.gitattributes is off". One shared
   sentence with the key name as a variable, not two constants — the `.gitattributes` branch
   (line 424) is rewritten to call it too. `mirror.ResidueDetail` stays exactly as it is (R13): under
   R6 a para-only `CLAUDE.md` genuinely should not exist.
6. **Rewrite `subjects.go:393`'s comment.** It asserts "CLAUDE.md is the only file that can be residue
   today; `emit.gitattributes` looks like a second and is not". After this phase both are, and the
   comment argues for the behavior being removed.
7. **R11 as a `doctor` test:** key off, `CLAUDE.md` present holding *only* third-party content →
   `doctor` reports clean. This is the exact state the bug report observed a false
   `stale-projection` in, so it belongs in `internal/doctor`'s own tests as well as in Phase 26's
   script.
8. **Check `internal/cli/output.go:63`.** Its comment reasons that "'wrote' beside a deleted
   `CLAUDE.md` would be a lie". Still true, and now *both* verbs are reachable for this file — a
   shortened one is a write, an emptied one is a removal. Extend the comment; do not delete it.

**Done when** the four cases in task 1 pass, `emit.claude` off against a tree with a foreign
`CLAUDE.md` reports clean, and no comment in `rebuild` or `doctor` still claims `CLAUDE.md` is the
only file that can be residue.

---

## Phase 26 — Generated prose, properties, and the regression

**Status: not started.**

**Tasks**

1. **`internal/render/agents.go` (R15), two independent fixes.** Line 92's "`CLAUDE.md` and everything
   under `.claude/` are wholly generated. Do not hand-edit them" becomes: `CLAUDE.md` is marker-scoped,
   described the way this same file already describes `AGENTS.md`; and `.claude/` narrows to
   `.claude/skills/`. The second is a **pre-existing** docs bug — `mirror.pruneEmpty`
   (`mirror.go:392`) has always been empty-only and para has never written `.claude/settings.json` — so
   fix the docs and change no behavior. Line 73's prose about `.claude/` needs the same narrowing.
   Golden update.
2. **`internal/config/keys.go:146` (R10).** "emit the Claude Code compatibility surface: CLAUDE.md and
   the .claude/skills mirror" — still accurate, but `--help` is where a user learns what the key does
   to a file they share. Say that para owns a block in `CLAUDE.md`.
3. **`wipeProjections` (R16).** `cmd/para/property_test.go:791` lists `CLAUDE.md` beside `ACTIVITY.md`
   and `MEASUREMENTS.csv`. Remove it. Its doc comment describes "the state §2.4 calls 'a merge
   resolved truth and left the projections wrong'", which for a marker-scoped file means a wiped
   *block*, not a wiped file.
4. **The eight-location preservation property (R17).** Foreign content outside para's markers is
   byte-identical across `wipe → rebuild`, at the root and all seven bucket locations, in both
   `emit.claude` states. Eight locations and not just the root, because `upToRoot` (`claude.go:62`)
   makes the bucket files' block content differ from the root's, and a block-splice bug that only
   shows up where the prefix is non-empty is exactly the kind this property exists to catch.
5. **The para-0o6 regression script (R18).** `testdata/script`: `para init` with `emit.claude` at its
   default, write a root `CLAUDE.md` holding a foreign marker-delimited block, `para doctor` → clean,
   `para add skill …` → the skill lands and the foreign block is byte-identical afterwards. Then
   `para config set emit.claude true` → the file gains para's block and keeps the foreign one; `false`
   again → para's block goes and the foreign one remains. Under `-tags para_testhooks`.
6. **Judge the result as a committed artifact.** Read a post-`rebuild` `CLAUDE.md` from the position of
   a teammate who pulled it and owns the non-para half: are the markers self-explanatory about what is
   safe to edit, and does the block's placement leave their content where they left it? The
   marker text already says "generated, do not edit; run `para rebuild`" — confirm that reads correctly
   in a file whose *other* half is genuinely theirs to edit.
7. **`bd close para-0o6`** with the regression script named as the evidence.

**Done when** the regression script passes, the eight-location property passes in both `emit.claude`
states, no generated file tells an agent that `CLAUDE.md` or `.claude/` is wholly para's, and
`make test` is green.

---

## Out of scope

Carried from requirements §5, restated because these are the four things most likely to be picked up
mid-phase:

- **No adoption pass** for marker-less `CLAUDE.md` files. After Phase 24 a file with no markers is
  somebody else's, full stop.
- **No change to `.claude/skills/`.** Whole-directory ownership is correct there under R0, and
  `pruneEmpty` does not move.
- **No change to the import architecture.** The block's *content* is byte-identical to what
  `claudeRenderer` renders today.
- **No new `doctor` finding and no new `mdfile` primitive.**
