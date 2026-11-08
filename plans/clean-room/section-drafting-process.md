# Section drafting process

How each section of `spec.md` is written, and where the work stands. Read this first when resuming in a new session.

## Where we are (handoff)

- **Branch:** `clean-room-impl`, local and on `origin` (tracks `origin/clean-room-impl`); push with `git push` only when the human asks. `main` holds the PoC; `poc-final` and `v0.7.0` tag its last commit.
- **Beads:** epic `para-a1b`; Phase 1 is `para-a1b.2` (claimed), and its notes log progress per section.
- **Done and committed:** §0.1 What para is, §0.2 The PARA method, §0.3 Structure is para's, content is yours, §0.4 Why deterministic structure helps agents, §0.5 Scope and non-goals, §0.6 Principles, §0.7 How to read this spec (chapter 0 complete), §1.1 The tree, §1.2 Buckets, §1.3 Projects, §1.3.1 Objectives, §1.3.2 Key results, §1.4 Areas, §1.4.1 Standards, §1.4.2 Indicators, §1.5 Measurement, §1.5.1 Analytics, §1.5.2 Metrics, §1.5.3 Terms, §1.6 Resources, §1.7 Archive, §1.8 Notes and the journal, §1.9 History (was Activity), §1.10 Links, §1.11 Dirs, §1.12 Sensitivity labels, §1.13 Provenance, §1.14 Status, §1.15 Review, §1.16 Suppression, §1.17 Generated files (chapter 1 complete), §2.1 Directory shapes, §2.2 Kinds and containment, §2.3 Addressing, §2.4 Adopting existing directories, §2.5 Archiving (chapter 2 complete), §3.1 Truth and projection, §3.2 `state.toml`, §3.3 The journal, §3.4 Time and the clock, §3.5 Config, §3.6 The walk, §3.7 Git (chapter 3 complete), §4.1–§4.3 (chunk 1), §4.4–§4.5 (chunk 2), §4.6–§4.7 (chunk 3, which also amended §1.4.2, §1.14, §1.16, §3.2, §4.3 and §4.4), §5.1–§5.5 (chunk 4, which renamed §5.3 to READMEs and also amended §0.3, §1.3, §1.11, §1.17, §2.4, §3.2 and §3.7; chapter 5 complete). Also committed: the journal-as-audit-log restructure (D156–D161), which amended §0.3, §0.6, §1.5.2, §1.8, §1.9, §1.14, §1.17, §2.5, §3.1–§3.4 and removed §5.4 `ACTIVITY.md` and §5.5 `MEASUREMENTS.csv` (old §5.6 and §5.7 are now §5.4 and §5.5).
- **Next:** chunk 5, §6.1 and §6.3–§6.6 Sensitivity (see the loop for the chunk plan). Then continue in outline order. Reviewer agents don't survive a session, so start fresh reviewers.
- **For chunk 5:** read the chapter 6 charters, §1.12, §3.5, §5.3 and §5.4, `plans/clean-room/research/labeling.md`, the carried notes naming §6.x (Config follow-ups, Top of the label chain, Who may loosen a label, Labels on computed values, Hand-made loosening, Labels through derivation, Path rules and adoption, Generated file details), and D58–D69, D121, D126, D168, D169, D226. Carried from chunk 4: README frontmatter shows the covering label as `label`, or `unlabelled` when none (D221), so §6.1 must stop a custom scale from naming a label `unlabelled`; only the root's `AGENTS.md` lists the labels and what each allows (D226), so §6.1 defines where a custom scale's meanings come from.
- **Working style the human has asked for:** short sentences and one idea per paragraph (bulleted lists and numbered steps are fine); ask design questions before drafting when no decision covers them, recommending one option; record every answer as the next D-number and tag the decisions it amends; reviews may run a third round or over the whole change set when the human asks. In questions about file layouts, show concrete before and after examples of the files (the human asked for a second example before choosing in chunk 4). The human may revise a choice after reviewing the full text, as with the README layout in chunk 4 (D228), so keep each change small and record it as a new D-number that replaces the old one.
- **Git:** the branch is pushed and level with `origin` after chunk 4; push only when asked.
- **Decisions:** D1–D229 in `plans/clean-room/plan.md`. The ones written during drafting:
  - D22: `para` never deletes or edits content; `remove` deletes only `para`'s side and names what history it loses.
  - D23: generated files (`ACTIVITY.md`, `MEASUREMENTS.csv`) live in `.para/`, linked from the README; on `remove`, the parent's README block is regenerated and the removal is journaled at the parent.
  - D24: `para` owns only its own keys in README frontmatter.
  - D25: every refusal says why and what to do instead. D13 amended: a hand copy (`cp -r`) can also duplicate ids.
  - D26: `para` never runs git or requires a repository.
  - D27: only a person lowers a label or raises trust, through an interactive confirmation (amended by D69: `--force` skips it).
  - D28: every `.para/` holds a `state.toml` with its kind; the root's has `kind = "root"`, schema version and id prefix, replacing `tree.toml`. The `tree.toml` section was removed, and §3.3–§3.8 became §3.2–§3.7.
  - D29: state holds facts (changing one would make a record false), config holds choices; the id prefix is state, fixed when the tree is created or migrated, and ids are never re-indexed.
  - D30: repair resolves a duplicate id by giving one entity a new id, the person choosing which keeps it; the only time an id changes.
  - D31: a bucket holds only entities of its kind and `para`'s own files (`.para/`, README, `AGENTS.md`); anything else directly in it is a `doctor` finding.
  - D32: archiving from inside a live entity leaves a managed stub (`kind = "stub"`, the id it stands for, no id of its own) at each mirrored ancestor; §0.3 and §1.1 lists amended to include stubs. Amended by D112 (stubs and buckets are entities with ids; containers may carry labels).
  - D33: projects never nest; a project sits only directly in `projects/` or `archive/projects/`.
  - D34: project and objective statuses are `planned` (default), `in-progress`, `blocked`, `done`, `dropped`, set by command in any order.
  - D35: key result statuses `achieved`, `missed`, `at-risk`, `on-track` are computed; only `dropped` is set by hand.
  - D35 (amended): after the due date only measurements dated on or before it count; `dropped` overrides and can be cleared.
  - D36: every key result has its own due date.
  - D37: an objective holds only `key-results/` and your content; a key result holds no entities.
  - D38: areas nest at any depth.
  - D39: `standards/` and `indicators/` containers; a standard holds only indicators and content, an indicator no entities.
  - D40: indicator statuses `no-data`, `breached`, `at-risk`, `healthy` are computed; the at-risk margin is a config setting.
  - D41: a measurement is a dated number; metrics have no types.
  - D42: the measurement library is flat; analytics, metrics and terms hold no entities.
  - D43: a term is a README definition that metrics reference by id; every library reference is checked by kind, listed both ways by `show`, and reported by `doctor` once dangling.
  - D44: the measurement library and each of its containers are created on first use, not by `init`; `measurement` stays reserved.
  - D45: staleness is a per-kind `<kind>.stale-after` (dir, analytic, metric and term included), off unless set, inherited from anywhere, root included.
  - D46: `init` writes starter `project.stale-after` and `area.stale-after` into the root's config; no hidden defaults.
  - D47: container is an entity kind with an id and a reserved name; it holds only its expected kind (anything else is a `doctor` warning), never goes stale and is never archived alone; it has a journal and a README listing its children; stubs keep no id. Amended by D112 (stubs and buckets are entities with ids; containers may carry labels).
  - D48: anything can be archived whatever its status; fields never change.
  - D49: unarchive brings back archived ancestors as live and restores everything beneath, even what was archived alone first.
  - D50: the archive is read-only (show, list, unarchive, remove).
  - D51: any entity but a container can be archived alone; stubs give way to their entities at every depth, refusing only when two archived entities share a path.
  - D52: containers are created on first use.
  - D53: every managed folder but a stub keeps a journal; one home per event.
  - D54: a note counts as attention only when marked (opt in), for notes and reasons alike; `migrate` marks v0 notes that counted.
  - D55: attention is the entity's own newest marked note, or its creation; a metric's measurements count; nothing on a child counts.
  - D56: `ACTIVITY.md` is the full history, for every folder with a journal.
  - D57: links sit in `links/` of projects, areas and dirs; opaque type and ref, input or output; type and direction fixed; flagged by `link.stale-after`.
  - D58: labels are TLP 2.0 (`CLEAR`, `GREEN`, `AMBER`, `AMBER+STRICT`, `RED`), loosest to strictest.
  - D59: a label can be set on the root, a bucket, an archive bucket or any entity but a container; amends D15.
  - D60: unlabelled means no rule; no default label.
  - D61, D62: a move (archive and unarchive included) refuses when a covering label would fall.
  - D63: the covering label shows in README frontmatter, `show`, `list`; `AGENTS.md` says what each allows.
  - D64: a label is never looser than the one above it; refuse what would cause it (chosen over strictest-wins).
  - D65: losing a label is a fall.
  - D66: path rules may sit in any config that can hold a label.
  - D67: tightening is allowed anywhere, the archive included (amends D50; §1.7 amended).
  - D68: a label can be removed; it is a loosening when the label above is looser or absent.
  - D69: loosening confirms by default, `--force` skips it, journaled either way (reverses D27's no-non-interactive rule; principle 7 amended).
  - D70: no folder defaults for provenance; files are recorded explicitly.
  - D71, D78: sources are a library kind in `resources/sources/` (reserved), grade in config.
  - D72: trust is computed when read; removed, changed or unrecorded inputs and removed sources count as the lowest grade.
  - D73, D79: provenance is state in the holding entity's `state.toml` (files inside entities only); recording, contributing and vouching are journal events.
  - D74: sensitivity and reliability scales are configurable in root config (TLP 2.0 and A–F by default); the creation method is not.
  - D75: `para` never writes content; recording is its own command per file.
  - D76: a recorded file is a primary (fixed, hashed, names its source) or a derived work (living, names its inputs).
  - D77: RFC 8693 `sub` and nested `act`; identities taken as given.
  - D80, D89: a source starts at the lowest grade; setting or raising a grade confirms unless forced, even at creation.
  - D81: inputs are named by entity id and path.
  - D82, D85, D86: people and agents are sources; an agent's source is its model and harness; a derived work counts its people, actors and inputs.
  - D83, D88: a vouch confirms unless forced, counts only with no `act`, and replaces every actor's grade for work recorded before it.
  - D84: a source's grade can be lowered in the archive.
  - D87: each act records model id, harness version and session, ungraded.
  - D90: a primary's reliability is its source's grade alone.
  - D91: attribution by flags or `PARA_*` variables, flag wins; no `sub` refuses.
  - D92: a passed due date makes open work overdue, a review flag, not a status.
  - D93: setting `blocked` requires a note.
  - D94: `done` and `dropped` close the objectives and key results beneath, derived from the path, never written.
  - D95: an archived key result or indicator computes its status as of when it was last archived.
  - D96: a live entity never uses an archived one; archive, unarchive and new references refuse otherwise (amends D51; §1.5, §1.7 amended).
  - D97, D100: review reports blocked, overdue, key results, indicators, stale and unchecked; unchecked lists the files to vouch for, wherever they sit, when a live file outside closed work depends on them.
  - D98: review is quiet about the archive and everything closed or under closed work; areas never close.
  - D99: review and `doctor` stay separate.
  - D101: vouching is allowed in the archive (amends D50, D84; §1.7 amended).
  - D102: only stale, key result and indicator flags can be suppressed; blocked, overdue and unchecked have their own remedies (blocked amended by D219).
  - D103: a suppression covers one flag on one entity; on a key result or indicator it holds only for the status it was set under.
  - D104: a suppression needs an end date and a note, as does lifting it early; neither is attention; expiry writes no event.
  - D105: review ends with the count of suppressed flags, and an option lists them.
  - D106: a suppression is state, set by `suppress` and `unsuppress`, journaled, not inherited.
  - D107: staleness can be suppressed ahead of time; a key result or indicator only while flagged (§1.15 amended).
  - D108: generated files show only values a command alone can change; overdue, stale, trust and computed statuses only in `show`, `list` and `review` (principle 4 amended).
  - D109: buckets `bucket`, archive buckets `archive-bucket` (reversed by D112).
  - D110: a folder's kind is read from `state.toml`, never inferred from name or place.
  - D111: a folder whose kind cannot be read is damaged: writes in or beneath it refuse, reads skip it with a warning, `doctor` reports it.
  - D112: every managed folder is an entity with an id; kinds `root`, `container` (buckets included), `stub` and the twelve; root and buckets fixed, stubs `para`'s alone (only `show`/`list`); labels on any entity but a stub; root, containers and stubs never stale; provenance only inside the twelve kinds (§0.3, §1.1, §1.2, §1.7, §1.8, §1.9, §1.11, §1.12, §1.13, §1.17 amended).
  - D113: objectives and key results are optional, like standards and indicators.
  - D114: the root may hold your own files; only containers (buckets included) and stubs report stray content.
  - D115: archive buckets are created on first use, then fixed; `init` issues five ids (§1.1, §1.2, §2.2 amended).
  - D116: a stub outlives its removed entity, recording its kind as well as its id; `doctor` reports only a never-issued id; unarchiving beneath it refuses (§1.7, §2.2 amended).
  - D117: stubs follow renames and moves of their live entities; the move refuses when a stub's new path is taken; archiving onto a removed entity's stub refuses (amends D50).
  - D118: every issued id gets an empty file in the root's `.para/ids/`, never deleted; its count is n for id length (amends D13).
  - D119: an entity with another tree's prefix is reported by `doctor` and re-id'd by repair; commands refuse duplicated or foreign ids until then (amends D30).
  - D120: why ids, not paths (paths change and get reused; most references are out of `para`'s reach); D11 kept with cheap lookup; stored references are the id alone; D112 kept.
  - D121: adoption carries the strictest parent path-rule label onto the new entity and moves provenance records into it.
  - D122: `init` adopts existing bucket folders after confirmation or an acknowledging flag.
  - D123: `adopt` takes a parent id, a kind and several folder names, all or none; no recursion.
  - D124: a plain folder with a container's name can be adopted as that container.
  - D125: provenance records get ids from the entity issue; paths are properties; inputs name record ids (§1.13, §2.3 amended).
  - D126: a path rule matching nothing is a `doctor` warning of a possible loosening; path rules stay keyed by path.
  - D127: a container stays until its parent goes, so its id, label and journal survive (amends D52).
  - D128: a path clash in archiving, unarchiving or stub upkeep takes a suffix (`-1`, `-2`, …) that stays part of the name, instead of refusing (amends D51, D116, D117; §1.7 amended).
  - D129: `unarchive` may take a new live parent; the only way back under a removed ancestor (settles D116's remedy).
  - §2.5 also placed archived entities by id rather than path, and allowed a removed descendant's stub inside an archived entity (§1.7, §2.1, §2.2 amended).
  - D130: measurements are events in the metric's journal; `MEASUREMENTS.csv` projects them.
  - D131: nothing outside the tree is truth or config; no user-level config.
  - D132: when state and the journal disagree, commands go by state; `doctor` reports it and repair offers either side.
  - D133: an unreadable `config.toml` makes a damaged folder (§2.1 amended).
  - D134: unknown files in a `.para/` are `para`'s and a `doctor` finding.
  - D135: an optional `title` in state on any entity but a stub, separate from the folder name, set by command.
  - D136: a `created` time in state, set once at create, adopt or migrate.
  - D137: §3.2 defines only the file's rules and common keys; each kind's keys live with their behaviour.
  - D138: unknown state keys are kept in place on rewrite and reported by `doctor`.
  - D139: a journal is one JSON Lines file, `.para/journal.jsonl`; repair resolves merge conflicts by keeping both sides.
  - D140: only `migrate` rewrites events; re-iding appends an event (§1.8 qualified).
  - D141: every event names its `sub` and optional `act` chain as source ids; v0 events carry `"v0": true` instead.
  - D142: `init` and `migrate` create your source (amends D115; §1.13 amended).
  - D143: the who comes from `--sub`/`--act` or `PARA_SUB`/`PARA_ACT`; refuse without a sub.
  - D144: notes and measurements may be backdated in `at`, never into the future; both times kept.
  - D145: every `para config` change is journaled at its folder; rewriting a generated file is never an event (amends D53).
  - D146: a measurement may claim any past time; a note not before `created` (amends D144).
  - D147: only a live source can be named.
  - D148: notes and measurements are corrected by a `correction` event naming the original's id, holding every corrected value in `values` or `"withdrawn": true`; the newest wins; archived work stays read-only (amends D130).
  - D149: every event has an id, eight random characters, unique within its journal, never in `ids/`.
  - D150: stored times are UTC to the second (amended by D153, D154).
  - D151: dates cover their whole day (amended by D154, D155).
  - D152: each command reads the clock once; `PARA_NOW` sets it, documented and open to anyone.
  - D153: one tree-wide `time-zone` setting on the root, set by `init` from `--time-zone` or the system zone (qualifies D131; §3.1 amended).
  - D154: deadlines are UTC moments; the zone only decides how commands read and show times; a bare date means that day's last second; DST repeats take the earlier moment, gaps refuse; generated files use UTC (§1.14, §1.16 amended).
  - D155: `stale-after` is a duration with `m`, `h`, `d` or `w`, positive, at most `5200w` (amends D45).
  - D156: the journal is an audit log; no normal command reads it for a current value; nothing checks truth files against it (amends D53, D73, D132).
  - D157: no `ACTIVITY.md`; history is the journal, shown on request (reverses D56).
  - D158: a metric's measurements live in `.para/measurements.csv`, the truth, sorted by claimed time then id; corrected by rewriting rows; only notes take correction events (replaces D130; amends D148).
  - D159: state keeps the attention time; a correction that can change it resets it by reading the journal and `measurements.csv` (amends D55).
  - D160: state keeps the blocked reason as its own fact (amends D93).
  - D161: state keeps the archive time of every entity `archive` moves; §3.3's walk is gone (amends D95).
  - D162: settings pass down, nearest wins, no off value.
  - D163: an archived entity reads config where it sits.
  - D164: a bad config value makes commands that need it refuse.
  - D165: `init` writes `project.stale-after = "14d"` and `area.stale-after = "90d"`.
  - D166: `config.toml` is written the same way every time.
  - D167: archived entities inherit settings up to the root, so a later change above can change a frozen status (amends D95, D163).
  - D168: an unset scale means the default; `init` writes only `time-zone` and two staleness settings (amends D74).
  - D169: config details: `indicator.margin`; a grade covers only its source; settings with nothing to apply to are allowed; stub configs ignored; nearest value used even when bad; dotted keys equal tables.
  - D170: `--root <dir>` or `PARA_ROOT` names the root from anywhere, with no walk; replaces `$PARA_HOME`.
  - D171: nested trees: the nearest root wins; the outer walk never enters one; one directly in an entity is a `doctor` finding.
  - D172: the walk up climbs past damaged folders (reversed by D174).
  - D173: the walk down looks only at folders directly inside entities; content is never entered; symlinks never followed.
  - D174: the walk up refuses at the first damaged folder; `--root` gets around it.
  - D175: `para` recognises names exactly, case included; collisions when creating follow the filesystem.
  - D176: a folder `para` is denied when checking for `.para` is damaged; an unresolvable cwd refuses.
  - D177: walk details: up to the filesystem root; `--root` resolved; `init` refuses `--root`, ignores `PARA_ROOT`, passes damaged folders, refuses at a root or v0 tree; nested v0 trees; a root with an unreadable config is damaged.
  - D178: `para` never reads or writes `.git/` or checks for a repository.
  - D179: `para` writes no `.gitattributes`; §3.7 documents `merge=union` for journals and `* -text`.
  - D180: a conflict in `state.toml` or `config.toml` leaves a damaged folder, fixed by hand.
  - D181: `para` writes LF a whole file at a time, reads LF, CRLF or a mix, ends an unterminated last line before appending; recorded files hash raw bytes.
  - D182: conflict markers inside `para`'s part of a README or `AGENTS.md` are replaced by `rebuild`; elsewhere, or across its edge, fixed by hand and reported by `doctor`.
  - D183: analytic as aggregate or ratio (reversed by D191).
  - D184: ranges on analytics, counted back from the computed time (amended by D192, D193).
  - D185: an indicator has no window of its own; it rolls through its analytic.
  - D186: a key result has a target and a start; progress (current − start) ÷ (target − start) (start amended by D197).
  - D187: an indicator's bound is `at-least` or `at-most`.
  - D188: `measurements.csv` columns `event`, `at`, `value` (and `note`, D195).
  - D189: values are finite decimals below 10^15, stored as the nearest float64 in shortest form, TOML floats in state.
  - D190: measurement details (CSV created with the metric, reading rules, empty and shared event ids, key order across sections).
  - D191–D194: an analytic is one `formula`, PromQL style: `sum`, `avg`, `min`, `max`, `count` over `id[duration]`, a bare id is the newest measurement; no count ranges; each operation rounds to float64.
  - D195: a measurement may carry a one-line `note`.
  - D196: `para` never resolves a merge conflict; every conflict is fixed by hand (repair no longer cleans up after merges).
  - D197: a key result's `start` is a time (default `created`); the start value is computed from history.
  - D198: key result pace = progress ÷ elapsed; never at-risk while elapsed < `key-result.grace` (0.1), then at-risk when pace < `key-result.pace` (0.8) (§3.5 amended).
  - D199: `indicator.margin` is a fraction of the bound's size, built-in 0.1.
  - D200: no current or start value (removed analytic or metric included) means progress 0; an indicator is `no-data`.
  - D201: a key result's target is `at-least` or `at-most`, replacing `target` (§4.3 amended).
  - D202: a due date may be in the past; a status settles at once.
  - D203: unarchiving is not attention.
  - D204: closed and archived entities are never stale.
  - D205: a live key result is due no later than its objective and project; any command breaking that refuses.
  - D206: status details: negative pace counts as 0, no pace is never at-risk, settings are integers or floats from 0 to 1, the exact indicator comparison, the start value fallback bounded by the computed time.
  - D207: review ranks each group most urgent first; ties by path.
  - D208: a stale entry shows its attention time and the setting; review never reads the journal.
  - D209: suppressing a suppressed flag replaces it.
  - D210: a suppression's end must be after now.
  - D211: a status suppression covers its status until its end, through flapping.
  - D212: an expired suppression is ignored and dropped at the next rewrite.
  - D213: no new suppressions on closed or archived work; existing ones survive.
  - D214: a stale suppression needs a setting that reaches the entity.
  - D215: paths sort one name at a time by UTF-8 bytes, a folder before its contents.
  - D216: a suppression `para` cannot use is ignored; `doctor` reports it.
  - D217: an indicator may have both bounds (a range), margin at each edge.
  - D218: state keeps `status-since`; blocked ranks longest blocked first.
  - D219: `blocked` can be suppressed through the `status` flag.
  - D220: README frontmatter holds one `para` key, a map under state's key names; `para` rewrites only that key's lines (amends D24).
  - D221: the map shows kind, id, title, covering label (`unlabelled` when none), set status, `status-since`, blocked reason, due, archive time and the kind's defining keys; no `created`, attention, suppressions, provenance, grade or computed values.
  - D222: the README block lists each direct child (link, kind, title, id) in path order, then links the journal and a metric's `measurements.csv`.
  - D223: title heading on top, block at the bottom (replaced by D228).
  - D224: a damaged block or frontmatter refuses commands that must rewrite it; a missing block or `para` key is added back (narrows D182).
  - D225: a missing README or `AGENTS.md` is recreated by any rewrite; a symlink, folder or unreadable one refuses.
  - D226: only the root's `AGENTS.md` lists the labels and what each allows.
  - D227: a marked title heading (replaced by D228).
  - D228: a README is frontmatter, then `para`'s block (`# ` title, description, `## Contents (para)`), then your text; a new block goes right after any frontmatter, `AGENTS.md` included.
  - D229: entities may have a one-line `description` in state, shown in the frontmatter map and the README block (amends D135).
  - §3.3 also gave descendants that `unarchive` brings back their own event, so the archive-date walk works (§2.5 amended).

## Priorities

**Reader clarity, and less is more.** Every section is written for a reader who has only the spec: a person meeting `para` for the first time, or an AI agent dropped into a tree. Keep it simple. The shortest section that is complete wins.

## Writing conventions

- **Cross-references are Markdown links** to the heading anchor, such as `[the PARA method](#02-the-para-method)`. Never use the § sign. Anchors follow GitHub: lowercase, punctuation dropped, spaces to hyphens (`### 8.2 \`doctor\` findings` → `#82-doctor-findings`).
- **One line per paragraph.** Never hard-wrap prose.
- **The tool is always `` `para` `` in backticks** in prose. Headings stay plain. "PARA" in capitals is the method.
- **Each drafted section replaces its one-sentence charter.** Keep the charter in the reviewer prompt.
- **Check links after every edit** with this snippet:

```python
import re
s=open('spec.md').read()
slug=lambda h: re.sub(r'[^\w\- ]','',h.lower()).replace(' ','-')
anchors={slug(m.group(1)) for m in re.finditer(r'(?m)^#{1,4} (.+)$',s)}
print('broken:',[l for l in re.findall(r'\]\(#([^)]+)\)',s) if l not in anchors])
```

## Context to load when resuming

- `spec.md`: the outline and drafted sections. Each undrafted section's one sentence is its charter.
- `plans/clean-room/plan.md`: decisions D1–D57, open questions, and design inputs.
- `plans/clean-room/research/labeling.md`: research behind chapter 6.
- `para-design-v4.md` (on `main` until Phase 2, always at `poc-final`): PoC design, background only. Decisions win where they disagree.
- `bd show para-a1b.2`: progress notes.

## The loop

Work goes in chunks: a run of neighbouring sections that share decisions, drafted, reviewed and committed together. A chunk holds at most about 1,500 drafted words and the questions it raises fit in one or two rounds of up to four. A section with many open questions, or one that could force a rethink, is a chunk of its own; if a chunk's questions turn up a structural problem, split it there.

Planned chunks: (1) §4.1–§4.3, (2) §4.4–§4.5, (3) §4.6–§4.7, (4) §5.1–§5.5, (5) §6.1 and §6.3–§6.6, (6) §6.2, (7) §7.1–§7.3, (8) §7.4–§7.6, (9) §7.7–§7.10, (10) §8.1–§8.3 (may split), (11) §9.1–§9.3.

1. **Go.** The human says go, naming a chunk or taking the next one.
2. **Questions.** Read every charter, carried note and decision the chunk touches, then ask all its design questions at once (one or two rounds), recommending one option each. Record each answer as the next D-number and tag what it amends.
3. **Draft.** Write each section in `spec.md` under its header, from its charter and the decisions. Check facts against sources before writing.
4. **Humanize.** Apply the humanizer skill's rules: no dashes, no "not X but Y", no one-line closers, no padding.
5. **Review, round 1.** Three `general-purpose` subagents in parallel, in the background, one lens each (template below), over the whole chunk. They report only. Reviewers from an earlier chunk in the same session can be reused with `SendMessage`.
6. **Fix.** Wait for all three. Dismiss findings that do not hold up, saying why. Fix every reject, blocker and major; fix minors that are clean and small.
7. **Review, round 2,** only when round 1 raised a reject, blocker or major, and only by the reviewers that raised one, with `SendMessage` listing what changed. A reject, blocker or major still left: apply a clean fix if there is one, and escalate to the human, saying so.
8. **Human review.** Show the full chunk text, led by a short list of any decisions added during review. Iterate until approved.
9. **Commit.** One commit per approved chunk (`docs(spec): write X.Y–X.Z Topic`), then update the handoff and `bd update para-a1b.2 --append-notes "..."`.

At most two review rounds per chunk, unless the human asks for another.

**When a review raises a design question** (as `remove` did in §0.3), stop and ask the human. If the choice is hard, run three subagents through user lenses (a person using the CLI, an AI agent in the tree, scripts and maintainers) to compare the options, then recommend. Record the answer as the next D-number in the plan.

**If a reviewer's report does not arrive** after its completion notice, ask it to resend with `SendMessage`.

## Severities

| Severity | Meaning | Action |
| --- | --- | --- |
| Reject | The approach is wrong; the section needs rethinking, not editing | Must fix |
| Blocker | Contradicts a decision, states something false, or omits what the charter requires | Must fix |
| Major | A gap or ambiguity that would mislead a reader or an implementer | Must fix |
| Minor | Wording, ordering or small clarity issues | Fix if clean and small |

## Review lenses

The reader-clarity lens carries the most weight.

1. **Reader clarity and less is more** (primary). Could a newcomer, human or agent, understand it with only the spec? Flags undefined terms, hidden assumptions, two readings, padding, repetition of earlier sections, anything that could be shorter.
2. **Consistency and scope.** Agrees with every decision and every other section's charter; stays inside its own charter and covers all of it; uses terms as defined (D17); carries nothing the decisions reversed (paths as addresses, skills, `CLAUDE.md` generation, `resource` as a kind); link anchors resolve; for sections about outside sources, fidelity to the source.
3. **Implementation and edge cases.** Read as the engineer who must build and test it. Flags promises that cannot be kept or tested, missing edge cases (empty tree, merge collision, hand edit, symlink, case-insensitive filesystem, partial failure), and security rules. Does not demand detail that belongs to later chapters.

### Reviewer prompt template

> You are reviewing one drafted section of a software specification. Report findings only; do not edit any file.
>
> **Project.** `para` is a Go CLI that deterministically manages the structure and metadata of a PARA file tree. The spec is written section by section in `/home/jomadu/repos/colchuck-ai/para/spec.md`; undrafted sections are one-sentence charters. Conventions: cross-references are Markdown links to heading anchors (no § sign); one line per paragraph; the tool is `para` in backticks.
>
> **Section under review:** §X.Y "Title" (from `### X.Y` to the next header). Charter: "…". Earlier sections are approved; don't re-review them, but flag repetition or mismatch.
>
> **Context:** decisions in `plans/clean-room/plan.md`; severities in `plans/clean-room/section-drafting-process.md`; (lens-specific background files).
>
> **Your lens: …** (lens description, plus the section's riskiest claims or edge cases to probe).
>
> **Severities:** Reject, Blocker, Major, Minor. **Output:** numbered list; each: location (quote), severity, problem (1–2 sentences), concrete fix. Say so if nothing is above Minor. Don't invent problems. Under 350 words.

## Notes carried forward to later sections

Commitments made while drafting chapter 0 that later sections must honour:

- **Every entity has a `.para/`** (D112: every managed folder is an entity): the root (its `state.toml` has `kind = "root"`, D28), buckets, archive buckets, containers, stubs (D32) and entities (§0.3). Storage (§3) and the model (§2.1, §2.2) must keep this true.
- **Provenance records live in `.para/`** (§0.3). Chapter 6 may refine how, not where.
- **Collision rule** (§0.3): refuse rather than overwrite or merge, except that an existing README or `AGENTS.md` gets `para`'s part added and adoption adds a `.para/` to an existing folder. Adoption (§2.4) and migration (§9.1) must follow it; files a v0 `para` wrote count as `para`'s side.
- **Damaged block markers**: `para` must refuse to rewrite rather than guess, so it never overwrites the user's text. Belongs in §5.2.
- **Case-insensitive filesystems** (`README.md` vs `readme.md`): collisions resolve as the filesystem sees them. Belongs in §8.1.
- **Partial failure**: validation happens before any write; what happens if a write fails partway (including `remove` reruns, D22) belongs in §8.1.
- **Concurrent commands**: §0.4 says agents, people and scripts share a tree; what happens when two `para` commands run at once belongs in §8.1. Root and bucket journals (D53) are written by nearly every create, so concurrent appends and git merge conflicts concentrate there (§3.7).
- **Mid-write conditions** (a destination appearing mid-run, a full disk) are partial failures under §8.1, not refusals; §0.4 promises refusal only before anything is written.
- **Ids in README frontmatter**: §0.4 says file tools can find any entity and its main facts, and commands take only ids (D11), so §5.3 must make the id a frontmatter key.
- **Cheap id lookup and id-only references** (§5.6, §7.4, §7.10, D120): `list` filters by place (such as the current folder); shell completion shows names and fills in ids; every output prints each id with its current path; `AGENTS.md` tells agents to write only the id in notes and handoffs and to run `show` to see what it names.
- **Journaling outside entities** (§3.3, D53): settled; the root and buckets keep journals. §3.3 must list which root events exist (`init`, `migrate`, `AGENTS.md` blocks, D21's `CLAUDE.md` cleanup) and whether `config` changes are journaled anywhere.
- **A command for every structural change** (§0.4): every change §0.3 lists (create, move, rename, re-parent, archive, unarchive, remove, adopt) has a command, ideally one per change; check chapter 7's charters against this list.
- **Refusals and remedies** (D25): every refusal gives the rule, the offending id or path, and what to do instead, which may be the user's own files to move; §7.10 defines the output.
- **Hand changes `doctor` must catch** (§8.2): `mv` of an entity (possibly to a place its kind may not sit), `rm -r` of an entity, a hand edit to a `para` frontmatter key, and `cp -r` of an entity, which duplicates its id (D13). Anything directly in a bucket other than its entities and `para`'s files is a finding (D31), though §0.3 still calls it content that `para` never moves; decide whether OS and tool files (`.DS_Store`, `.gitkeep`, `Thumbs.db`) are exempt.
- **Git** (§3.7): §0.5 says `para` never runs git or requires a repository. §3.7 must settle whether `para` reads `.git/` and whether it writes `.gitattributes` (the PoC did; D23 allows only README and `AGENTS.md` as visible files).
- **Time-dependent values in generated files** (§5.3): derived status depends on the current time, and generated files change only when a command runs (§0.5, §0.6 principle 4), so §5.3 must show such values with an as-of date or leave them out.
- **Who may loosen a label or raise trust** (§6.4, D27, D69): a confirmation by default, skipped by `--force`; journaled either way, saying whether forced. §6.4 should say the prompt is a nudge, not a security boundary, and the confirmation should name the result ("`brain-x9f2` and everything beneath it go from `RED` to `GREEN`").
- **Commands given a duplicated id** (§2.3, §8.2, D119): settled; a command given a duplicated or foreign id refuses until repair.
- **Test hooks for the clock and ids** (§3.4, §2.3, Appendix B): §0.7 says Appendix B tells how tests fix the clock and new ids, so each must define a documented mechanism (such as an environment variable) and say whether users may rely on it.
- **Test output conditions** (§7.10, Appendix B): tests check output exactly (§0.7), so §7.10 must fix colour, width, locale, prompt text, JSON key order and `/` as the path separator, both with and without a terminal. Interactive confirmations need a terminal, so the Phase 3 harness needs pseudo-terminal support.
- **Example starting points** (Appendix A, B): Appendix A or each example must define the v0 tree and the "example tree with a stated change" form that `migrate` and `doctor` examples start from.
- **Tour coverage** (chapter 1): §0.7 says chapter 1 tours the whole tool, but no tour section yet leads to the command line, `doctor`, `migrate` or config (chapters 7 to 9). Add one or link them from existing sections.
- **Kinds of non-entities** (§2.1, D112): settled; every managed folder is an entity, with kinds `root`, `container` (buckets and archive buckets included) and `stub` beside the rest.
- **Entities moved between trees** (§2.3, D119): settled; `doctor` reports an entity with another tree's prefix and repair re-ids it; commands refuse its id until then. §8.2/§8.3 define the finding and the repair, including a whole copied subtree.
- **Top of the label chain** (§6.3, D15, D28): any `.para/` may hold a `config.toml`. Decide whether the root's or a bucket's config can set sensitivity, or whether inheritance starts at entities.
- **Prefix validation** (§3.2, §8.3, D13): settled in §3.2: ASCII letters and digits, otherwise every command but `doctor` refuses. Still open: §8.3 repair restores it from the names in `ids/`; whether the root id must carry the prefix; prefix case and length.
- **Journal follow-ups** (§5.4, §7.2, §7.3, §7.6, §8.1, §8.2, §8.3, Appendix A, D139–D147): §7.3 settles `init` adopting a `resources/` that already holds a plain `sources/` folder, and what name `init` and `migrate` ask for; §7.2 lists `--sub`, `--act`, `PARA_SUB`, `PARA_ACT` and how a backdated `at` is typed, and whether a note carried by another command can be backdated; §7.6 says whether a command that changes nothing records the note it carries; §5.4 shows backdated events as such and orders them; §8.1 covers a failure partway through writing several journals; §8.2 lists the journal findings (unreadable line, missing journal, unknown type or field, missing `sub`, symlink, backdated attention); §8.3 defines the merge cleanup; Appendix A shows exact event lines; every command section must name the event types it writes and their fields; §7.6 defines the correct and withdraw command, which fills unchanged values in for you; §5.4 renders corrections; whether correcting a live metric's data should change an archived key result's frozen status (D95).
- **Time follow-ups** (§3.5, §4.3, §4.4, §4.7, §5.3, §5.4, §7.2, §7.3, §7.6, Appendix B, D150–D155): §3.5's charter must make `time-zone` the one root-only setting and say `doctor` reports it below the root; §4.3 says whether an indicator's window is a duration like `stale-after`; §4.4 covers backdating a measurement onto a passed due date and a due date set in the past; §4.7 and §7.6 say whether a suppression end in the past is refused; §5.4 orders events within a UTC day; §5.3 shows due dates in README frontmatter as UTC moments; §7.2 defines the typed time and date forms and `--time-zone`; §7.3 says how `init` reports the zone it chose; Appendix B fixes new ids as `PARA_NOW` fixes the clock, and pins the zone with `--time-zone`.
- **Restructure follow-ups** (§3.7, §4.2, §4.5, §7.4, §7.8, §8.2, §8.3, §9.1, D156–D161): §3.7 and §8.3 cover merge conflicts in `state.toml` (attention takes the later, or is reset as after a correction) and in `measurements.csv` (keep both sides' rows, drop repeats, re-sort; the same id with different values needs the person), and repair re-iding a duplicated event id must change the matching row's id; §8.2 decides whether `doctor` reports an attention time before `created`, in the future or missing, an `archived` key on a live entity or a missing one on an archived key result or indicator, and how D99's "hand changes" are detected now that nothing cross-checks the journal; §7.8 says `rebuild` leaves attention, blocked reasons and archive times alone; §4.2 defines the CSV format (header, columns, whether a row carries `time`, `sub` and `act`), atomic rewrites, an empty file after the last withdrawal, and whether a measurement may claim a time before the metric's `created`; §4.5 says resetting attention skips withdrawn notes and applies each note's latest correction, names the `attention` key and the kinds that have it; §7.4 defines how history is shown on request; §9.1 has `migrate` compute attention, the blocked reason, the archive time (the old walk, kept only here) and `measurements.csv` once from a v0 tree.
- **Config follow-ups** (§4.4, §6.1–§6.3, §7.3, §7.9, §8.2, §9.1, D162–D169): §4.4 gives the built-in margin's value and format and whether an indicator keeps a margin in state; §6.1–§6.3 say whether each scale, label and path rules is a top-level key or a table, and their key names; §6.2 defines the lowest grade under a custom scale; §7.3 lists what `init` writes (`time-zone` and two staleness settings); §7.9 checks values when set, refuses unknown keys, root-only keys off the root and anything but tightening a label or lowering a grade in the archive, and links back to §3.5 rather than restating it; §8.2 lists unknown keys, out-of-place settings, a stub's `config.toml` and bad values; §9.1 says whether `migrate` renames a v0 `resource.stale-after` to `dir.stale-after` and converts day counts to durations.
- **Walk follow-ups** (§7.1, §7.3, §7.10, §8.2, §8.3, D170–D177): §7.1 lists `--root`/`PARA_ROOT` as a global option; §7.3 says `init` refuses inside a tree and refuses `--root`; §7.10 gives the refusal texts (no tree, damaged folder on the way up, v0 tree); §8.2 reports a nested root or nested v0 tree in an entity, a near-miss name such as `.PARA` or `Projects/`, and folders `para` is denied when checking; §8.3 must say how `doctor` and repair reach a damaged root, since `--root` refuses one (perhaps `doctor --root` accepts it). The "Finding the tree" note is settled by §3.6.
- **Git follow-ups** (§4.2, §4.7, §5.2, §8.2, §8.3, §9.1, D178–D182): §5.2 keeps D182 (markers inside `para`'s part replaced by `rebuild`, across its edge fixed by hand); §4.2 covers `measurements.csv` conflicts; §4.7 picks a suppression format and says what two branches' suppressions of one flag become (a duplicate TOML key makes the file unreadable); §8.2 reports what a union merge leaves in a journal (repeated lines, half lines, both versions of a rewritten event), conflict markers in READMEs and `AGENTS.md`, and merged `state.toml` that breaks its rules; §8.3 must let `doctor` and repair reach a tree whose root `state.toml` or `config.toml` holds conflict markers, since every command stops there today; §9.1 says to migrate on one branch and merge it before other branches write. The "Git" note is settled by §3.7.
- **Measurement follow-ups** (§1.14, §4.4, §7.x, §8.2, §8.3, §9.1, D183–D197): §4.4 defines progress when the start value equals the target or is missing, pace measured from the start time, the status when an analytic or metric named by id was removed, and a formula with no value; §1.14 or §7.6 names the `status` and `due` keys for projects and objectives; §7.x gives the commands to record, correct and withdraw measurements (with `--note`), to set a formula, target, start and bound; §8.2 reports empty or shared `event` ids, a missing `measurements.csv`, removed ids in formulas and conflict markers anywhere; §8.3 gives a row with an empty `event` an id (and whether that is journaled); §9.1 builds `measurements.csv` with the `note` column from v0 events. D196 supersedes the repair-on-merge items in the Restructure and Git follow-ups and the "Concurrent commands" note: conflicts are hand fixes.
- **`state.toml` format** (Appendix A, §9.3, D135–D138): Appendix A must show the exact bytes (spacing, quoting, table layout, trailing newline, string escapes, ints and floats), since §3.2 points there; §9.3 must let `migrate` run on an older `schema` while `doctor` only names an unreadable one; each kind's section lists its keys in write order; how a title is set and cleared belongs in chapter 7.
- **Repairing a duplicate id** (§8.3, D30, D11): repair must name one holder of a shared id (a path, as a stated exception to D11, or similar), give the choice a non-interactive form for agents, handle three or more holders, and re-id a copied subtree together so the keeper is chosen per subtree.
- **Missing or altered buckets** (§8.2, §8.3, §3.6): a bucket or archive bucket removed or renamed by hand, `measurement/` or one of its containers removed or renamed while it still holds entities (absent is fine, D44), what happens when its last entity is removed, a loose file or plain folder directly in `analytics/`, `metrics/` or `terms/`, a bucket that is a symlink, `Projects/` on a case-insensitive filesystem, and a bucket's `README.md` or `AGENTS.md` deleted (does `para` restore it?).
- **Archived measurement** (§1.7, §2.5, D20, D32): analytics, metrics and terms can be archived (D51), to `archive/resources/measurement/…`; see Archived references.
- **Stubs** (§1.7, §2.5, §2.3, §8.2, D32): a stub for a stub (a dir inside a dir inside an area); a stub left when its entity is removed, and a stub whose id no longer exists; the id a stub records is a reference, so duplicate checks and lookup ignore it; stubs are matched by id and follow their entity's renames; `remove` of an archived entity whose parent is a stub (no README to regenerate; does a stub have a journal, D23?); unarchiving after the live parent has itself been archived. Settled by D51: stubs give way to their entities at every depth, refusing only when two archived entities share a path, and any entity but a container can be archived alone, since containers have ids (D47).
- **When containers exist** (§2.1, §2.2, §2.5, Appendix B, D52): settled by D127 (containers stay until their parent goes); created on first use; say whether a container is removed when its last child goes (and a recreated one gets a new id, which breaks any stub that recorded the old one). Buckets are containers that `init` creates (D112); archive buckets are created on first use but never removed once made (D115).
- **What a due date does** (§1.14, §4.5, §4.6): §1.3 says a project may have a due date but not what happens when it passes; decide whether `review` flags an overdue open project.
- **Archiving open work** (§1.15, §2.5): settled by D48 and D94; any status can be archived, and review says nothing about a done project left unarchived, since closed work is not reported.
- **How many objectives and key results** (§2.2, §7.6): whether a project needs any objectives and an objective any key results (D19 makes areas' standards and indicators optional).
- **Simple key results** (§4.1, §4.3): a key result must hold an analytic (D20), so counting one metric needs a metric and an analytic; an analytic can be one metric's latest value (§1.5.1); decide whether `para` offers a shortcut that creates a metric and that analytic in one command.
- **Dropped and archived key results and indicators** (§1.14, §2.5): whether a dropped key result still counts toward its objective, and what a key result's or indicator's status is once it is archived, and whether `review` still reports it.
- **Key result computation** (§4.3, §4.4): define the start value and start date, the at-risk threshold, a target below the start (such as latency, where reaching means crossing downward), a yes/no target with no pace, the status with no measurements yet, a due date already past at creation, and a key result due after its objective. Also when a due date counts as passed (through the end of that day, in which time zone, §3.4), and whether a measurement may be backdated to on or before a passed due date, which would change a settled result.
- **Reserved container names** (§2.2, §2.4, §8.2, D20, D39): every container name is reserved in its parent, like `measurement`. Define the rule once: `para` refuses to create or adopt anything under a reserved name, adopting refuses when a plain folder already has one (D25), and `doctor` reports content found there. The tour (§1.3, §1.4) leaves this to §2.2. Say the depth: `measurement` is reserved only directly in `resources/` (D44), so `resources/cooking/measurement/` is allowed.
- **Staleness of nested areas** (§4.5, D55): settled; nothing on a descendant keeps an entity fresh. §4.5 should say how review keeps a parent with no work of its own (such as `health/`) from being flagged forever, for example by not setting `area.stale-after` on it.
- **Moving into a descendant** (§2.2, §7.6): moving an area (or dir) under its own descendant must be refused (D25).
- **Dangling measurement references** (§4.4, §7.6, §8.2, D22, D43): the status of a key result or indicator whose analytic or metric was removed; whether `remove` names what depends on the entity in its confirmation; `doctor` findings for a reference to the wrong kind, to an id never issued, or to another tree's prefix after a copy. References point downward by kind, so §4.1 should say no cycle is possible.
- **Measurement values** (§4.1, §4.2, D41): the number domain (NaN, infinities, huge values), date precision and time zone, two measurements at the same time (which decides "latest"), correcting or deleting a wrong measurement, division by zero, and an analytic whose metric has no measurements.
- **Labels on computed values** (§6.4, §6.5): a metric under a confidential path can feed a status and value shown in a public project's README; say whether a computed value is content derived from its metrics.
- **`MEASUREMENTS.csv` edits** (§5.5): it is a projection, so say whether hand edits are overwritten or reported.
- **Staleness per kind** (§3.5, §4.5, D45): list every `<kind>.stale-after` key; say what the clock is for kinds with no measurements (analytic, term, dir), and whether a `dir.stale-after` on a project or area is the expected way to cover its dirs. Also: the starter values `init` writes (D46); a value meaning "never" so a nearer setting can switch off a farther one (a root `dir.stale-after` reaching `resources/`); whether settings pass into `archive/`, or review simply ignores archived entities, and what happens to a `config.toml` that moves with an archived entity; links use `link.stale-after` like any kind (D57); and whether activity in a nested dir keeps its parent fresh, as for nested areas.
- **v0 resources** (§9.1, D18): how `migrate` converts `kind = "resource"` folders outside `resources/`, nested ones, and ones inside projects.
- **Containers as entities** (§2.2, §5, §8.2, Appendix A, D47): say whether a container has `ACTIVITY.md` and frontmatter, and the exact README block (D47 says it lists children); whether `remove` may target a container; `doctor`'s warning for other entities or content in a container; ids now grow faster (D13), since each project and area adds container ids; update the example tree. D112 settles commands for the root, buckets, archive buckets and stubs; still open for other containers: whether one can be renamed or moved apart from its parent, and whether `remove` may target one. A label on a container created on first use is lost if the container is removed when empty (§2.5, §6.3).
- **Read-only archive exceptions** (§5, §8.3, §9.1, D50): `doctor` repair (D30 re-iding), regenerated files, `migrate` of v0 archived entities, and stubs that follow renames all change things under `archive/`; say that read-only covers user commands only. Also unarchiving when a live entity now has the name, or when the stub's live entity was removed, including a container removed when empty and recreated with a new id, so its stub matches nothing (see When containers exist) (§2.5). Also the one refusal left in archiving: two archived entities at one path (a key result `a` archived, then a new live `a` archived); the remedy is renaming the live one, since the archive is read-only.
- **Symlinks in archived entities** (§2.5, §8.1): settled in §2.5 (moved unchanged, never followed); partial failure stays with §8.1; what archiving does with a symlink inside an entity, and moving many folders at once (partial failure).
- **Archived references** (§1.5, §1.7, §2.5, §7.6, D96): settled by D96; a live entity never uses an archived one. Still open: whether `remove` of an analytic, metric or term that live entities use is refused or only named in its confirmation (D22, D43), and what an archived user of a since-removed entity shows.
- **Journal rules** (§3.3, §3.4, §8.2): how rotation, `migrate` and D30 re-iding treat existing events, and whether `doctor` reports hand edits to a journal; for move, archive and unarchive, which journals record what (old parent, new parent, the entity itself, which is read-only once archived, D50), and where the event goes when the parent is a stub (no journal, D53); whether measurements are journal events (the PoC's design; §1.5.2 and D22 leave it open) and whether `config` changes are journaled; whether a command that changes nothing but carries a note records the note; notes are added to entities (§1.8), so say whether a container takes one; §3.3 must keep "`para` never edits an event" true under `migrate` and D30 re-iding, or state the exceptions; a wrong note is corrected by appending, as for measurements; ordering by timestamp under clock skew and equal times.
- **Attention marks** (§3.3, §4.5, §5.6, §9.1, D54): the flag's name; whether measurements still count as attention for a metric; that `AGENTS.md` tells agents to mark a note as attention only when the person says the upkeep happened; `migrate` turning v0 notes without `--no-attention` into marked ones; whether `review` shows the latest unmarked note, so a false stale flag is quick to clear. D55 names what resets the clock (a marked note, a metric's measurement, else creation); §4.5 must say whether unarchive restarts it. A mark cannot be added later (the journal is append-only), so the remedy is a new marked note; §3.3/§3.4 must say whether a note's date can be set, since a backdated mark claims past upkeep.
- **`ACTIVITY.md` details** (settled by D157: there is no `ACTIVITY.md`; §3.4, §3.7, §5.4, §6.4, §7.8, D56): the day boundary (the PoC used UTC days) and order within a day; the exact sentence for every event kind, including a removed child rendered from the event alone; the cost of rewriting full history for busy root and bucket files, and whether `doctor` keeps the PoC's full-fidelity check; git merges of newest-first files (resolve the journal, then `rebuild`); whether a bucket's or root's file may show a more sensitive child's title or a reason under a lower label; `rebuild` rewriting files under `archive/` (see Read-only archive exceptions).
- **Link details** (§2.1, §6.4, §7.2, §7.6, §8.2, §9.1, D57): a link's folder name on disk (the PoC defaulted it to the id); the command that changes a `ref` (D57 allows it); allowed characters and length of `type` and `ref`; a `ref` is not a secret store (it is in `state.toml`, may appear in its container's README and is committed), and its label is inherited from its holder; whether two links with the same type, ref and direction are refused or warned; `migrate` of v0 links with `direction = "both"` (split into two, or refuse naming the link) and of links held by v0 resources.
- **Indicator computation** (§4.3, §4.4, §3.5): how the window is set and measured (days, calendar periods, the time zone), how the margin is expressed (a fraction of the target, an absolute amount) and its default, targets that go down (latency), whether `no-data` also counts toward staleness (§4.5), whether the analytic combines every measurement in the window or uses the latest, a partial window (an indicator created today), measurements dated in the future, the boundaries of `at-risk` (exactly at the target, the edge of the margin), a target as a value plus a comparison (at or above, at or below), and that the window and target are state (defining facts set by command) while the margin is config.
- **Finding the tree** (§3.6): §1.1 says every command but `init` looks upward from the current directory for the nearest root (D28). §3.6 must settle a start outside any tree, inside a `.para/`, in a tree nested in another tree's content, under a symlinked root, and whether the PoC's `$PARA_HOME` override survives. Also: an ancestor `state.toml` that cannot be read (skip or refuse), an entity's `state.toml` hand-edited to `kind = "root"` (silently a nested tree; `doctor`?), the downward walk stopping at a nested root, and a v0 `tree.toml` recognised so every command but `migrate` refuses with the `para migrate` remedy (D14).
- **Targeting folders without ids** (§2.3, D112): settled; every entity has an id, the root and buckets included, so commands name parents by id.
- **Issued-id record** (§2.3, §3.2, §8.2, §8.3, D118): settled; one empty file per id in the root's `.para/ids/`; an entity with no file still resolves, `doctor` reports it and repair adds the file. Still open: a stray file there; repair updating stubs that record an id it changes, and which holder of a duplicated id a stub follows; §3.1 lists `.para/ids/` as state; Appendix A shows it.
- **`init` where something exists** (§7.3): `init` inside an existing tree or in a folder that already holds content follows the collision rule (§0.3).
- **Non-interactive confirmation**: agents confirm results from JSON output and exit codes (§0.4), so every confirmation prompt, such as `remove`'s (D22), needs a non-interactive path with JSON output (§7.10).
- **`doctor`** reports a `.para/` that `para` did not create, and references to removed ids (§8.2).
- **Determinism** (§0.1): output and changes depend only on the tree, arguments and current time, apart from new ids. Config must therefore come from inside the tree (§3.5), or §0.1 needs revisiting.
- **README frontmatter**: `para` writes only its own keys (D24); §5.3 defines them.
- **Path rules and adoption** (§2.4, §6.3, §8.2, D121, D126): settled; adoption sets the strictest covering path-rule label on the new entity and moves provenance records. §8.2 reports a path rule that matches nothing (after adoption, a hand rename or move) as a warning of a possible loosening, naming the rule and its label; remedy: fix or delete the rule, or adopt the folder as a dir where a dir may sit. Define "matches nothing": a rule whose files are now inside a child entity, an existing but empty folder, and globs. §6.3 should say labels on entities travel with them and path rules do not.
- **Hand-made loosening** (§6.4, §8.2, D64, D69): a hand edit to `config.toml` that loosens a label breaks no ordering; decide whether `doctor` reports a label looser than its journaled history.
- **Provenance details** (§6.2, §8.2, D76–D84): recording when the accountable person is not yet a source (refuse, or count as lowest); cycles in `from`; whether a vouch stores a hash so later edits show as "edited since vouch"; an input in a removed entity; hand renames and files moved by hand between entities; inputs outside the tree refused.
- **Record ids** (§6.2, §7.6, §8.2, §8.3, D125): repair remaps the inputs that name a record it re-ids (foreign or duplicated); the command that repoints a record at a file's new path (and moves it between entities), refusing when the new path already has a record or lies in the archive, a bucket, a container or outside the tree; which commands take a record id (`show` at least); bare records for unrecorded inputs and how `show` presents them; `doctor` findings for a missing file and for two records naming one path; ids now issued for records grow code length faster (D13).
- **Provenance storage** (§3.7, §8.1, D79): many per-file tables in one `state.toml` conflict when two branches record in the same entity; a raw-byte hash flags every primary after a git CRLF conversion.
- **Labels through derivation** (§6.4): whether a derived work takes the strictest label of its inputs (the `from` chain makes it possible); the §6.4 charter names it, no decision yet.
- **Attribution input** (§7.1, §7.2, D91): define `--sub`, `--act`, `--model`, `--harness`, `--session` and their `PARA_*` variables, flag over variable, and say the variables stand in for flags so §0.1's "arguments" covers them; tests must fix them like the clock.
- **Status details** (§3.3, §3.4, §4.4, §4.6, §8.2, D92–D95): the archive date an archived key result or indicator uses (D95) comes from the journal, so the journal rules must let one archived with an ancestor find the archiving that last put it there; when a due date counts as passed for overdue (day boundary, time zone), as for key results; a due date set in the past; `doctor` findings for a hand-edited `status` with an unknown value or on a kind whose status is computed; whether review reports dropped key results and archived work at all.
- **Review details** (§4.4, §4.6, §7.7, §8.1, D97–D101): the status of a key result or indicator whose analytic or metric was removed (dangling, D43), and whether review shows it; whether review takes the tree lock or waits while a write holds it; output with nothing to report; review run from inside the archive or a closed project; D100's walk over input chains visits each file once, survives a cycle in `from` (or recording refuses one, §6.2) and stops at an input whose record or entity is gone.
- **Suppression details** (§3.4, §3.7, §4.7, §7.6, §7.7, §8.2, D102–D107): whether the end date is inclusive, and the day boundary and time zone that decide it; whether an end date today or in the past is refused; one suppression per flag per entity, and whether suppressing again replaces it (journaled) or refuses; the status a key result or indicator suppression keeps is stored, and since it is computed from the clock, tests fix the clock (Appendix B); whether `at-risk` → `on-track` → `at-risk` before the end date stays suppressed; what review's count includes (only active suppressions hiding a flag now, not expired ones or those on closed or archived work); whether an expired suppression stays in `state.toml` or is cleared, and by what, since expiry writes no event (D104); a suppression on closed or archived work surviving reopen or unarchive; suppress is refused in the archive (not a §1.7 exception); a git merge leaving two suppressions or a conflicted `state.toml`; an early stale suppression on an entity no `<kind>.stale-after` reaches (refuse, or accept and hide nothing), and whether it starts hiding the flag once a setting arrives.
- **Generated file details** (mostly settled by chunk 4, D220–D228; still open: §8.2 lists a missing or odd README or `AGENTS.md`, broken markers, invalid frontmatter, a generated part that differs from what `rebuild` would write and conflict markers outside a block; §7.8 says to run `rebuild` after an upgrade; Appendix A gives the exact `AGENTS.md` text, README bytes, YAML quoting and how links and titles are escaped. Original note: §3.4, §5.1, §5.3, §5.4, §7.8, §8.1, §8.2, D108): whether a README shows an analytic's value (one that follows a metric's latest measurement only changes by command; one over a moving window does not); the write-through fan-out (a label change rewrites every README beneath it, D63; a rename rewrites the parent's block; a change to the label scale rewrites every `AGENTS.md`); whether write-through or `rebuild` recreates a deleted README or `AGENTS.md` anywhere, not only in buckets; a README or `AGENTS.md` that is a symlink, a directory or unreadable; `ACTIVITY.md` day boundaries come from the tree or UTC, never the machine's time zone or locale, so `rebuild` matches across machines (§1.17 promises it); whether `doctor` reports text written inside `para`'s part before the next rewrite; §1.17 gives every entity's README `para`'s frontmatter keys, the root, buckets and containers included, so §5.3 says which keys each kind gets; whether a `para` upgrade that renders files differently needs `rebuild` (§7.8, §9.3).
- **Classification details** (§2.2, §3.6, §8.1, §8.2, §8.3, D110, D111): a symlinked `.para` or managed folder (followed, or damaged); `.PARA` or `.Para` on case-insensitive and case-sensitive filesystems; a second `kind = "root"` below the root (nested tree, §3.6); a bucket or container kind on a folder whose name is not fixed for it, or a fixed name at the wrong depth (§2.2 finding); a damaged folder's ancestors: moving, archiving or removing an ancestor changes what is beneath it, so it refuses under D111, and the refusal names the damaged folder; how repair restores a damaged folder's kind (§8.3), and what the read warning looks like in text and JSON (§7.10). A damaged root or bucket: the walk must say "damaged" and point to `doctor`, never "outside any tree" or climb into an outer tree, and a damaged bucket hides the whole bucket from review behind one warning (§3.6).
- **New ids from D112** (§2.3, §2.5, §7.3, §9.1, Appendix B): `init` issues five ids at once (the root and four buckets, D115) in a fixed order tests can predict; an archive bucket gets its id when first created; `migrate` issues ids for the root, buckets, archive buckets and stubs, and creates the `.para/` v0 stubs lacked (D32); a stub has its own id and records another, so lookup by its own id returns the stub, and stubs are matched to their entity by the recorded id; a stub that gives way (D51) or is removed on unarchive retires its id, which is never reissued, and a later command given it refuses saying it no longer exists.
- **Stubs following their entities** (§2.5, §7.6, §8.2, D116, D117): settled in §2.5 (D117, D128, D129); §2.2 now only says where a stub sits and links to §2.5, which must state: stubs follow renames and moves of their live entity, adding stubs for new ancestors and removing emptied ones (D117); the move refuses when a stub's new path is taken; removing an entity leaves its stubs, keeping their last path, so archived entities beneath keep their path and history (D116); archiving a new entity onto a path held by a removed entity's stub refuses (D117); unarchiving beneath such a stub refuses, naming the removed ancestor, and §2.5 says what remedy it offers; whether a stub left with nothing beneath after `remove` of an archived entity is removed; how a container emptied, removed and recreated with a new id relates to stubs recording the old id. §8.2 lists `doctor`'s stub findings: a recorded id never issued in the tree (a stub of a removed entity is normal, D116).
- **Reserved names across filesystems** (§2.2, §8.1, §8.2): names collide as the filesystem sees them (§2.2). On a case-sensitive filesystem a `Links/` dir can sit beside `links/`, and the pair merges when the tree is cloned onto one that ignores case; §8.1 and §8.2 must say what `para` and `doctor` do about it.
- **Truth and projection follow-ups** (§3.5, §3.7, §6.2, §7.10, §8.1, §8.2, §8.3, D130–D134): §3.7 decides whether `para` reads a `.git/` above the tree (D131 allows reading, but not as truth or config); §6.2 says whether hashing a recorded file that is a symlink follows it, possibly out of the tree; §8.2 defines the state-versus-journal disagreement finding (D132) and whether the `CLAUDE.md` check looks above the tree, as Claude Code does; §8.2 and §8.1 square D134's stray-file finding with leftover temp files from an interrupted write and with stray files in `ids/`; §3.5 says whether `doctor` notices a deleted root `config.toml`, which silently drops `init`'s starter staleness settings (D46); §7.10's charter should cover how output adapts to a terminal; §8.3 says how repair restores an unreadable `config.toml` (D133).
- **Chunk 4 follow-ups** (§7.6, §8.2, §9.1, Appendix A, D220–D229): §7.6 defines the commands that set and clear a title and a description (D229); §9.1 says whether `migrate` keeps a v0 `description`, and converts v0 README frontmatter (plain keys that the v0 `para` owned outright, which count as `para`'s side) into the `para` map and moves v0 `AGENTS.md` blocks to the top; §8.2 reports what chunk 4 leaves to `doctor` (see Generated file details); Appendix A shows a README with no title, one with a description, and an adopted README with your old heading below the block.
