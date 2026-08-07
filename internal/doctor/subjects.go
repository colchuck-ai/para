package doctor

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mirror"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptime"
	"github.com/colchuck-ai/para/internal/ptoml"
	"github.com/colchuck-ai/para/internal/rebuild"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
)

// checkSubjects asks the four questions that are about one entity at a time:
// is its truth readable and legal (`invalid`), is its journal (`journal`), does
// its scope name things that exist (`scope-unresolved`), and do its generated
// files say what truth says (`stale-projection`).
func (s *scan) checkSubjects() error {
	for _, sub := range s.subjects {
		state, truthOK, err := s.checkTruth(sub)
		if err != nil {
			return err
		}
		journalOK, err := s.checkJournal(sub, state)
		if err != nil {
			return err
		}
		if err := s.checkScope(sub, state); err != nil {
			return err
		}
		configOK := s.checkConfig(sub)
		if !truthOK || !journalOK || !configOK || !s.configResolves(sub) {
			// Every projection is derived from truth, the journal, and the
			// resolved config, so any of the three failing to read makes "what
			// would be written now" unanswerable. Reporting drift here would be
			// reporting a comparison that was never made — and reporting it
			// once per descendant, since a config.toml an ancestor cannot parse
			// breaks the chain for everything beneath it. The file itself is
			// reported at the level that owns it.
			continue
		}
		if err := s.checkStale(sub); err != nil {
			return err
		}
	}
	return nil
}

// configResolves reports whether the §7 chain answers at all for this subject.
//
// It is asked separately from checkConfig because the file that will not parse
// may be an *ancestor's*, and the resolver is the only thing that knows the
// chain. Nothing is reported here: the offending file is named by the check at
// the level that owns it, and a second finding per descendant is the noise this
// replaced.
func (s *scan) configResolves(sub rebuild.Subject) bool {
	_, err := s.env.Resolver.RenderConfig(sub.Locator)
	return err == nil
}

// checkTruth reads the subject's truth file and reports §10's `invalid`: TOML
// that will not parse, a missing required field, a value outside its vocabulary
// (§15), and the two things only a clock or a raw document can see — a `created`
// in the future, and a key para does not recognise.
//
// It returns the decoded state and whether truth read cleanly enough for the
// rest of the checks to mean anything.
func (s *scan) checkTruth(sub rebuild.Subject) (truth.State, bool, error) {
	if len(sub.Locator) == 0 {
		return truth.State{}, s.checkTree(sub), nil
	}

	rel := join(s.rel(sub.Dir), ".para/state.toml")
	data, err := os.ReadFile(truth.StatePath(sub.Dir))
	if err != nil {
		s.add(Finding{Kind: KindInvalid, Path: rel, Locator: sub.Locator, Detail: "cannot be read: " + message(err)})
		return truth.State{}, false, nil
	}

	state, err := truth.DecodeState(data)
	if err != nil {
		s.add(Finding{Kind: KindInvalid, Path: rel, Locator: sub.Locator, Detail: "is not readable: " + err.Error()})
		return truth.State{}, false, nil
	}

	unknown, err := truth.UnknownStateKeys(data)
	if err != nil {
		return state, false, err
	}
	for _, key := range unknown {
		s.add(Finding{
			Kind: KindInvalid, Path: rel, Locator: sub.Locator,
			Detail: fmt.Sprintf("has an unknown key %q", key),
		})
	}

	for _, p := range truth.Check(sub.Kind, state) {
		s.add(Finding{Kind: KindInvalid, Path: rel, Locator: sub.Locator, Detail: p.String()})
	}
	s.checkNotFuture(rel, sub.Locator, kindmeta.FieldCreated, state.Created)

	return state, true, nil
}

// checkConfig reports §10's `invalid` for the other truth file: .para/config.toml
// (§2.2 — "state.toml is what the thing *is*, config.toml is policy about it").
//
// It is checked for the same three things state.toml is — does it parse, are its
// keys ones para knows, do its values fit their types — because §10's `invalid`
// row opens with "unparseable TOML" and does not say which truth file it means.
// Leaving it unchecked was worse than an omission: an unparseable config.toml
// made every projection under it underivable, so it surfaced as one
// `stale-projection` per descendant, naming a directory rather than the file and
// saying nothing about what was wrong with it.
//
// It reports whether the file is sound, which decides whether the subject's
// projections can be compared: a config that will not resolve makes "what would
// be written now" unanswerable, exactly as unreadable state does.
func (s *scan) checkConfig(sub rebuild.Subject) bool {
	rel := join(s.rel(sub.Dir), ".para/config.toml")
	data, err := os.ReadFile(truth.ConfigPath(sub.Dir))
	if os.IsNotExist(err) {
		// No config.toml is the common case and sets nothing (§7).
		return true
	}
	if err != nil {
		s.add(Finding{Kind: KindInvalid, Path: rel, Locator: sub.Locator, Detail: "cannot be read: " + message(err)})
		return false
	}

	doc, err := ptoml.Decode(data)
	if err != nil {
		s.add(Finding{Kind: KindInvalid, Path: rel, Locator: sub.Locator, Detail: "is not readable: " + err.Error()})
		return false
	}
	keys, err := doc.Keys()
	if err != nil {
		s.add(Finding{Kind: KindInvalid, Path: rel, Locator: sub.Locator, Detail: "is not readable: " + err.Error()})
		return false
	}

	for _, key := range keys {
		spec, known := config.Lookup(key)
		if !known {
			s.add(Finding{
				Kind: KindInvalid, Path: rel, Locator: sub.Locator,
				Detail: fmt.Sprintf("has an unknown key %q", key),
			})
			continue
		}
		value, ok := doc.Value(key)
		if !ok {
			continue
		}
		if err := spec.Check(value); err != nil {
			s.add(Finding{
				Kind: KindInvalid, Path: rel, Locator: sub.Locator,
				Detail: fmt.Sprintf("%s: %s", key, message(err)),
			})
		}
	}
	return true
}

// checkTree is checkTruth for the root, whose identity lives in tree.toml
// rather than a state.toml (§8.1).
func (s *scan) checkTree(sub rebuild.Subject) bool {
	rel := ".para/tree.toml"
	data, err := os.ReadFile(truth.TreePath(sub.Dir))
	if err != nil {
		s.add(Finding{Kind: KindInvalid, Path: rel, Detail: "cannot be read: " + message(err)})
		return false
	}
	t, err := truth.DecodeTree(data)
	if err != nil {
		s.add(Finding{Kind: KindInvalid, Path: rel, Detail: "is not readable: " + err.Error()})
		return false
	}
	if unknown, err := truth.UnknownTreeKeys(data); err == nil {
		for _, key := range unknown {
			s.add(Finding{Kind: KindInvalid, Path: rel, Detail: fmt.Sprintf("has an unknown key %q", key)})
		}
	}
	for _, p := range truth.CheckTree(t) {
		s.add(Finding{Kind: KindInvalid, Path: rel, Detail: p.String()})
	}
	s.checkNotFuture(rel, nil, kindmeta.FieldCreated, t.Created)
	return true
}

// checkNotFuture is §15's rule that `created` "defaults to now and is rejected
// if future". It is here rather than in truth.Check because it is a question
// about the clock, and a check that read the clock could not be a pure function
// of a file — the same line §2.5 draws between what may be stored and what must
// be derived.
func (s *scan) checkNotFuture(rel string, loc locator.Locator, field kindmeta.Field, stored string) {
	if stored == "" {
		return
	}
	at, ok := ptime.StoredAt(stored)
	if !ok {
		// Unparseable is already reported against the field itself.
		return
	}
	if at.After(s.env.Now) {
		s.add(Finding{
			Kind: KindInvalid, Path: rel, Locator: loc,
			Detail: fmt.Sprintf("%s: %s is in the future", field, stored),
		})
	}
}

// checkJournal reports §10's `journal` finding — "a line that is not valid
// JSON, or has no `at`/`kind`, or an unknown `kind`, or an `at` in the future —
// reported with file and line" — and the one `invalid` that lives in a journal
// rather than in truth: a measurement whose shape contradicts its key-result's
// `type`.
//
// The two are checked together because they are the same pass over the same
// lines, and separating them would mean reading every journal twice to report
// two things about one line.
//
// What a journal line *is* stays journal.Decode's: this loop supplies the line
// number, which a codec has no way to know, and the clock, which it must not
// read.
//
// It reports whether every line read, which decides whether the subject's
// projections can be compared at all: ACTIVITY.md is a fold of the journal, so
// a journal with a line nothing can parse has no "what would be written now"
// to differ from. An `at` in the future is not that — the line reads, it is
// just wrong about when — so it does not suppress the comparison.
func (s *scan) checkJournal(sub rebuild.Subject, state truth.State) (bool, error) {
	files, err := journal.Files(truth.LogsDir(sub.Dir))
	if err != nil {
		return false, err
	}
	ok := true
	typ, hasType := keyResultType(sub.Kind, state)

	for _, path := range files {
		rel := s.rel(path)
		f, err := os.Open(path)
		if err != nil {
			return false, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", rel))
		}
		scanner := bufio.NewScanner(f)
		// A journal line is one JSON object and nothing here writes a long
		// one, but a hand-edited file can hold anything, and the default 64K
		// limit would report "token too long" as if it were the file's fault.
		scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

		line := 0
		for scanner.Scan() {
			line++
			raw := scanner.Bytes()
			if strings.TrimSpace(string(raw)) == "" {
				continue
			}
			e, err := journal.Decode(raw)
			if err != nil {
				// The wrapped cause, not just the outer sentence: "invalid
				// journal event" without the codec's own complaint leaves the
				// reader with a line number and no idea what is wrong with it.
				ok = false
				s.add(Finding{Kind: KindJournal, Path: rel, Line: line, Locator: sub.Locator, Detail: err.Error()})
				continue
			}
			if e.At.After(s.env.Now) {
				s.add(Finding{
					Kind: KindJournal, Path: rel, Line: line, Locator: sub.Locator,
					Detail: fmt.Sprintf("at %s is in the future", e.At.UTC().Format("2006-01-02T15:04:05Z")),
				})
			}
			if e.Kind == journal.KindMeasurement && hasType {
				if _, err := krvalue.Parse(typ, e.Value); err != nil {
					s.add(Finding{
						Kind: KindInvalid, Path: rel, Line: line, Locator: sub.Locator,
						Detail: fmt.Sprintf("measurement %q is not a %s: %s", e.Value, typ, message(err)),
					})
				}
			}
		}
		closeErr := f.Close()
		if err := scanner.Err(); err != nil {
			ok = false
			s.add(Finding{Kind: KindJournal, Path: rel, Line: line + 1, Locator: sub.Locator, Detail: err.Error()})
		}
		if closeErr != nil {
			return false, paraerr.Wrap(paraerr.KindInternal, closeErr, fmt.Sprintf("closing %s", rel))
		}
	}
	return ok, nil
}

// checkScope reports §10's `scope-unresolved`: a skill's scope entry naming a
// locator that does not exist.
//
// §5.4 is why this is doctor's job and not `set`'s: para runs no matcher at
// write time, and an entry may legitimately name something that has not been
// created yet — so the write path stores what you gave it and the scan is where
// a scope that has gone stale surfaces.
func (s *scan) checkScope(sub rebuild.Subject, state truth.State) error {
	if sub.Kind != kindmeta.KindSkill {
		return nil
	}
	rel := join(s.rel(sub.Dir), ".para/state.toml")
	for _, entry := range state.Scope {
		loc, err := locator.Parse(entry)
		if err != nil {
			// Not a locator at all: truth.Check already reported it as
			// invalid, and asking whether it resolves would be asking about
			// something that cannot be looked up.
			continue
		}
		resolves, err := tree.Resolves(s.root, loc)
		if err != nil {
			return err
		}
		if !resolves {
			s.add(Finding{
				Kind: KindScopeUnresolved, Path: rel, Locator: sub.Locator,
				Detail: fmt.Sprintf("scope names %s, which does not exist", entry),
			})
		}
	}
	return nil
}

// checkStale is §10's load-bearing finding, at full fidelity: every generated
// file re-derived in memory from truth and compared byte for byte.
//
// Nothing is re-derived here. rebuild is asked what it would write, so the
// finding and its repair cannot disagree — a doctor with its own renderer would
// be a second opinion about the same bytes, and the whole point of principle 3
// is that there is only one.
func (s *scan) checkStale(sub rebuild.Subject) error {
	artifacts, err := s.rb.Derive(sub)
	if err != nil {
		// A projection para cannot produce is a projection that differs from
		// what would be written now, which is what §10 asks. The usual cause is
		// a delimited block whose markers were edited out of an AGENTS.md or a
		// .gitattributes, and naming the failure is more use than naming the
		// file it would have been.
		s.add(Finding{
			Kind: KindStaleProjection, Path: s.rel(sub.Dir), Locator: sub.Locator,
			Detail: "cannot be re-derived: " + message(err),
		})
		return nil
	}

	for _, a := range artifacts {
		if !a.Stale() {
			continue
		}
		s.add(Finding{
			Kind: KindStaleProjection, Path: a.Path, Locator: sub.Locator,
			Detail: staleDetail(a),
		})
	}
	return nil
}

// staleDetail says what a generated file no longer agrees with.
//
// Naming the source is not decoration: the repair is the same for all of them
// (`para rebuild`), but the *cause* is not — a README.md that drifted was
// hand-edited, while an ACTIVITY.md that drifted may have been hand-edited or
// may be the visible end of a journal that was. §26 spells the ACTIVITY.md line
// out, and the others follow its shape.
func staleDetail(a rebuild.Artifact) string {
	if !a.Wanted {
		// Residue: the file is there and nothing generates it any more (§6.1).
		// §10 has no row for that and does not need one — "differs from what
		// would be written now" covers it, since what would be written now is
		// nothing, and `rebuild` is the same repair that row already promises.
		//
		// The sentence is mirror's, because the mirror says it about itself in
		// the same report and about the same cause. CLAUDE.md is the only file
		// that can be residue today; `emit.gitattributes` looks like a second
		// and is not, because para owns a block inside that file rather than
		// the file (§9) — so turning it off shortens a file rather than
		// removing one, and this branch stays the one case it names.
		return mirror.ResidueDetail
	}
	if !a.Present {
		return "is missing"
	}
	switch baseName(a.Path) {
	case "ACTIVITY.md":
		// §10: "the earliest day that differs — so drift is dated rather than
		// merely detected".
		if day, ok := render.ActivityDrift(a.Existing, a.Derived); ok {
			return fmt.Sprintf("differs from journal (from %s)", day)
		}
		return "differs from journal"
	case "MEASUREMENTS.csv":
		return "differs from journal"
	case "README.md", "SKILL.md":
		return "differs from state.toml"
	case "CLAUDE.md":
		return "differs from the derived rules"
	default:
		if strings.HasPrefix(a.Path, tree.RulesDir()+"/") {
			return "differs from the skill it is generated from"
		}
		return "differs from what para would write"
	}
}

func baseName(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[i+1:]
	}
	return rel
}

// keyResultType is the type a key-result's measurements must fit, and whether
// there is a readable one to check them against at all.
func keyResultType(kind kindmeta.Kind, state truth.State) (krvalue.Type, bool) {
	if kind != kindmeta.KindKeyResult {
		return "", false
	}
	switch typ := krvalue.Type(state.Type); typ {
	case krvalue.TypeNumber, krvalue.TypeRatio, krvalue.TypeBoolean:
		return typ, true
	default:
		// An unreadable type is reported against the field itself; measuring
		// every reading against it would report one broken field once per
		// reading.
		return "", false
	}
}
