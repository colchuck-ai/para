# para — `CLAUDE.md` as a marker-scoped block (requirements)

`CLAUDE.md` stops being a file para owns and becomes a file para owns a *block inside*, exactly as
`AGENTS.md` and `.gitattributes` already are. This document is the requirement set;
`para-design-v4.md` is amended to match it, and [`implementation-plan.md`](implementation-plan.md)
breaks the work down.

Tracked as **para-0o6** (P1, bug, labels `claude` `data-loss` `emit` `interop`).

**Pre-1.0.** para has one user and no released consumers. There is no migration path, no adoption
pass for marker-less `CLAUDE.md` files, and no deprecation notice. After this change a `CLAUDE.md`
with no para markers is, by definition, somebody else's file, and para treats it as one.

---

## 1. Why

`emit.claude = false` deletes `CLAUDE.md` outright, including content para never wrote.

Observed in a tree where the beads issue tracker also writes a root `CLAUDE.md` (a marker-delimited
`<!-- BEGIN BEADS INTEGRATION v:1 -->` block, written by `bd setup claude`). With `emit.claude` at
its default of `false`, `para doctor` reported `stale-projection  CLAUDE.md should not exist`, and the
next mutation that touched the surface — `para add skill …` — printed `removed  CLAUDE.md` and took
the whole file. para had never written a byte of it. Recoverable only because beads had committed it.

The mechanism is not subtle. `claudeResidue` (`internal/rebuild/surface.go`) emits an
`Artifact{Wanted: false}` for a present-but-unwanted `CLAUDE.md`, and `apply`
(`internal/rebuild/rebuild.go:198`) turns `Wanted: false` into `os.Remove`. Every caller inherits it:
`para rebuild` through `Run`, and any skill-subject mutation or `config set` of an `emit.claude` key
through `WriteClaudeSurface` (`internal/mutate/claude.go:23-32` enumerates them).

**The `merge=ours` half is worse than the deletion.** `internal/render/gitattributes.go:44` writes
`**/CLAUDE.md merge=ours linguist-generated=true`. The deletion is loud and, under git, recoverable.
`merge=ours` on a file holding a third party's content is silent: a teammate edits the non-para
portion, you merge, git keeps your side and discards theirs, with no conflict and no output.

## 2. What this reverses

`CLAUDE.md`'s whole-file ownership is not an implementation slip. It is written into the spec in eight
places, and v4 is normative, so this is a spec amendment first and a code change second.

| Site | What it says today |
| --- | --- |
| §2.1 (line 367) | `CLAUDE.md` is named in the wholly-generated list, "no human-authored body to own" |
| §2.2 table (line 378) | generated "wholly"; human-owned part "none"; merge posture `merge=ours` |
| §6.1 (line 834) | "**`CLAUDE.md`** is wholly para's, and it is a pointer file — no prose of its own" |
| §9 block (line 1063) | `**/CLAUDE.md merge=ours linguist-generated=true` |
| §9 prose (lines 1071–1078) | argues `merge=ours` is safe "because the file is fully generated" |
| §12 principles (line 1141) | "wholly generated files have no body" |
| §25 decisions (line 1191) | `emit.claude` "emits `CLAUDE.md`" as a file it produces |
| §10 / residue | `stale-projection` covers "a file that is there and nothing generates any more" |

§26's worked transcripts are **not** in this list, and that is the change's best property — see R9.

## 3. The real defect is §2.2's membership test

§2.2 sorts every generated file by one question: *is this a pure projection of `.para/` state?*
`CLAUDE.md` answers yes — it is `@AGENTS.md` plus one `@` import per derived rule, and nothing else —
so the table put it beside `ACTIVITY.md` and gave it `merge=ours`. The test is wrong. It measures
where the bytes come *from*, and the property that decides merge posture and delete-versus-shorten is
who else writes to the *path*.

`AGENTS.md` landed on the correct side by accident: it has human prose, so nobody had to ask the
interop question. That means the principle is not recorded anywhere and will not catch the next file.

**R0.** §2.2's membership test becomes: *does any tool other than para write to this path?* If yes,
para owns a marker-delimited block and never the file. The projection question survives as a second,
independent axis — it still decides whether a body is human-owned — but it no longer decides merge
posture or removal.

Applied, R0 sorts the eight rows without exception: `AGENTS.md`, `.gitattributes`, and `CLAUDE.md`
are shared paths and get blocks; `ACTIVITY.md`, `MEASUREMENTS.csv`, `.agents/rules/**`,
`.claude/skills/para-X`, and `SKILL.md` are paths para alone writes, and keep what they have.

## 4. Requirements

### The emitted file

**R1.** With `emit.claude` on, `CLAUDE.md` is written as a para-owned block delimited by
`mdfile.MarkdownMarkers` — the same `<!-- para:begin … -->` / `<!-- para:end -->` pair `AGENTS.md`
uses. The block's content is unchanged: `@AGENTS.md`, then one `@`-import per derived rule, sorted.

**R2.** The write is **append semantics**, not replace: `mdfile.AppendDelimited`, as
`.gitattributes` uses. A `CLAUDE.md` that predates para keeps every byte, and para's block goes at
the end. `ReplaceDelimited`'s refusal of a marker-less file is correct for `AGENTS.md` — para wrote
that file, so missing markers mean edited-out markers — and wrong here, where a marker-less file is
the normal first encounter.

**R3.** Bytes outside the markers are preserved exactly, at all eight locations, across any number of
rebuilds. Block position within the file is preserved: a para block in the middle of a file stays in
the middle.

**R4.** A `CLAUDE.md` holding para's begin marker with no matching end marker is a **validation
error naming the path and the repair**, not a silent overwrite. This is the behavior
`.gitattributes` already has, and the reason is the same: fixing it means deciding where a block para
did not write ends. `damagedBlock` (`internal/render/gitattributes.go:72`) currently hardcodes
`.gitattributes` into its message and must take the path instead.

### Turning the surface off

**R5.** With `emit.claude` off, para removes **only its block** and leaves every other byte where it
was. `RemoveDelimited` already does this; the requirement is that `CLAUDE.md` reach it.

**R6.** When removing the block leaves the file empty, the file is **deleted**. This applies to
`.gitattributes` on the same code path, changing its documented behavior deliberately — see the
decision in §6.

**R7.** A `CLAUDE.md` para has never written — no begin marker — is never modified and never deleted
by any para command. `RemoveDelimited` returns it unchanged with `found` false; nothing downstream
may act on it.

### `.gitattributes`

**R8.** The `**/CLAUDE.md merge=ours linguist-generated=true` line is removed from
`gitAttributesBlock` **entirely**, not reduced to `linguist-generated=true`. git attributes are
per-file and cannot be scoped to a marker range, so no whole-file attribute is correct for a shared
file — which is precisely why `AGENTS.md` has no row in the block. `linguist-generated=true` would
collapse a teammate's hand-written note in review along with para's import churn.

The three remaining entries still satisfy the block's own header comment ("any side is as good as any
other, because `para rebuild` produces the truth") with no rewording.

### Observable behavior

**R9.** For a tree where para is the only writer of `CLAUDE.md`, **nothing observable changes.**
Turning `emit.claude` off still ends with no `CLAUDE.md` on disk (R6), and `rebuild` still prints
`removed  CLAUDE.md`. This is a requirement, not an accident: it is what keeps §26's transcripts, the
`testdata/script` fixtures, and `doctor`'s eight residue goldens valid, and it is why this change is
small.

**R10.** `emit.claude`'s *meaning* changes even though a para-only tree's behavior does not: the key
stops answering "does this file exist" and starts answering "is para's block present". The key's own
documentation (`internal/config/keys.go:146`) and §6.1 must say the new thing.

### `doctor`

**R11.** `emit.claude` off, `CLAUDE.md` present with third-party content outside para's markers, no
para block: **clean**. No finding of any kind.

**R12.** `emit.claude` off, `CLAUDE.md` present holding para's block *and* other content: one
`stale-projection` whose detail says the block is what should go, mirroring `.gitattributes`'s
existing sentence — `"still holds para's block; emit.<key> is off"`. One shared sentence with the key
as a variable, not two constants.

**R13.** `emit.claude` off, `CLAUDE.md` present holding nothing but para's block: `stale-projection`
with `mirror.ResidueDetail` — `"should not exist; emit.claude is off"` — unchanged. The sentence is
still exactly true, because under R6 the file should not exist.

**R14.** `emit.claude` on, `CLAUDE.md` present with third-party content and a stale block: one
`stale-projection` reading `"differs from the derived rules"`, unchanged
(`internal/doctor/subjects.go:415`).

### Generated prose

**R15.** The generated root `AGENTS.md` (`internal/render/agents.go:73, 92`) currently tells agents
"`CLAUDE.md` and everything under `.claude/` are wholly generated. Do not hand-edit them." Both
halves change:

- `CLAUDE.md` becomes marker-scoped, described the way the same file already describes `AGENTS.md`.
- `.claude/` narrows to `.claude/skills/`. **This half is wrong today, independent of this change** —
  `mirror.pruneEmpty` (`internal/mirror/mirror.go:392`) has always removed `.claude/skills/` and
  `.claude/` only when empty, and para has never touched `.claude/settings.json`. The docs promise a
  breadth the code does not have, and the fix is to narrow the docs.

### Tests

**R16.** `cmd/para/property_test.go`'s `wipeProjections` (line 791) stops treating `CLAUDE.md` as a
wholly generated file. It moves to the marker-preservation property.

**R17.** A new property: third-party content outside para's markers is byte-identical across
`wipe → rebuild`, at **all eight locations** and not only the root, in both `emit.claude` states.

**R18.** A `testdata/script` case reproducing para-0o6 end to end: a foreign marker-delimited block
in a root `CLAUDE.md`, `emit.claude` off, then a skill mutation — and the foreign block survives.
Run under `-tags para_testhooks`; `go test ./...` alone SKIPs the txtar suite.

## 5. Non-goals

- **No migration and no adoption pass.** A marker-less `CLAUDE.md` is somebody else's file. para
  does not detect "this is a `CLAUDE.md` I wrote under the old rules" and does not wrap it in
  markers. Pre-1.0, one user, one `git checkout`.
- **No change to how rules reach Claude Code.** §6.1's import architecture is untouched — still
  `@AGENTS.md` plus one `@` per derived rule, still no second copy of a rule on disk. Only the file's
  ownership model changes.
- **No change to the eight locations.** `render.HasClaude` and `render.HasAgents` are inputs to this
  work, not outputs of it.
- **No change to the mirror.** `.claude/skills/` keeps whole-directory ownership, correctly: para is
  the only writer of `.claude/skills/para-X`, so R0 leaves it where it is. `pruneEmpty` is already
  narrow enough and does not move.
- **No new `mdfile` primitives.** `AppendDelimited`, `RemoveDelimited`, and `ReplaceDelimited` are
  everything this needs.
- **No new `doctor` finding.** §10's finding set stays closed at eleven; R12 and R13 are both
  `stale-projection`.

## 6. Decisions

Recorded here so nobody re-litigates them.

- **A block, not a narrower delete.** The alternative — keep whole-file ownership but refuse to
  delete a file containing anything unexpected — needs para to recognize its own output to know what
  "unexpected" means, which is a fingerprinting problem with no good answer. Markers *are* the
  fingerprint, and para already has them.

- **`AppendDelimited`, not `ReplaceDelimited`.** The two differ only on a marker-less file, and that
  is exactly the case this bug is about. `.gitattributes` already made this call for the same reason
  and its doc comment already argues it.

- **Delete when the block's removal empties the file.** `WithoutGitAttributesBlock`'s doc
  (`gitattributes.go:87-93`) argues the other way — "para cannot tell a file it created from an empty
  one it was given, and the safe reading is that every line it did not write is the repository's" —
  and that argument loses here on two counts. A zero-byte file carries no information, so deleting it
  destroys nothing. And not deleting would leave eight empty `CLAUDE.md` files behind every time
  someone turns `emit.claude` off, breaking R9 and invalidating fixtures for no gain. Taking it
  uniformly means `.gitattributes` changes behavior in the emptied case too; that case is nearly
  unreachable in a real repository, and one rule beats two.

- **`Artifact.Wanted` survives.** An earlier reading of this change had residue disappearing as a
  category — if `CLAUDE.md` is only ever *shortened*, `Wanted: false` has no producers and the
  removal path in `apply` is dead. R6 is why that is wrong: the shortened bytes decide which
  artifact you get. Empty result → `Wanted: false`, a removal. Non-empty → `Wanted: true` with
  shorter bytes. Both files now produce both shapes, so `subjects.go:393`'s "CLAUDE.md is the only
  file that can be residue today" becomes false and its comment is rewritten rather than deleted.

- **The whole `.gitattributes` line goes, not just `merge=ours`.** R8. Fixing the silent-merge half
  alone would leave a whole-generated file with no merge posture, and conflicts in a file `rebuild`
  regenerates. Right direction, wrong resting place — so it lands in the same commit as the markers,
  not ahead of them.

- **One residue function, two callers.** `claudeResidue` and `gitAttributesResidue` become one
  parameterized function. They were already the same shape and are now the same rule; two copies of a
  gate that decides whether a file is shortened or removed is the kind of duplicate that drifts
  silently.

- **The `.claude/` docs narrow; the behavior does not.** R15. `pruneEmpty` is already correct, and
  the temptation to "make the code match the docs" here would introduce the exact bug this document
  exists to remove.
