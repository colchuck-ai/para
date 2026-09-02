package query

import (
	"slices"
	"strings"
	"time"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/view"
)

// SortKey is a `--sort` key (§17).
//
// The set is §17's list and not kindmeta's field matrix, because two of the
// keys are not fields at all: `progress` and `pace` are derived (§2.5 forbids
// storing them), and `locator` is the path. The matrix decides which *kinds*
// have a key, which is a different question and the one KindsWith answers.
type SortKey string

const (
	SortLocator   SortKey = "locator"
	SortName      SortKey = "name"
	SortCreated   SortKey = "created"
	SortAttention SortKey = "attention"
	SortDue       SortKey = "due"
	SortPriority  SortKey = "priority"
	SortStatus    SortKey = "status"
	SortProgress  SortKey = "progress"
	SortPace      SortKey = "pace"
)

// sortKeys is every legal key, in the order §17 lists them — the order the
// error message enumerates and `--help` prints.
var sortKeys = []SortKey{
	SortName, SortCreated, SortAttention, SortLocator,
	SortDue, SortPriority, SortStatus, SortProgress, SortPace,
}

// SortKeys returns every legal `--sort` key.
func SortKeys() []SortKey { return slices.Clone(sortKeys) }

// ParseSortKey validates a `--sort` argument.
func ParseSortKey(s string) (SortKey, error) {
	if slices.Contains(sortKeys, SortKey(s)) {
		return SortKey(s), nil
	}
	names := make([]string, len(sortKeys))
	for i, k := range sortKeys {
		names[i] = string(k)
	}
	return "", paraerr.Newf(paraerr.KindValidation,
		"%q is not a sort key (one of %s)", s, strings.Join(names, ", "))
}

// universal are the keys every kind has: §17's "`name`, `created`, `attention`,
// `locator` everywhere". A sort on one of these never groups by kind.
var universal = []SortKey{SortName, SortCreated, SortAttention, SortLocator}

// Universal reports whether every kind has the key.
func (k SortKey) Universal() bool { return slices.Contains(universal, k) }

// AppliesTo reports whether kind has the key — §17's "where the field exists".
//
// `progress` and `pace` are key-result-only by §17's own sentence rather than by
// the field matrix, since neither is a stored field. Everything else is the
// matrix, so there is one place a kind's fields are declared (§0.3).
func (k SortKey) AppliesTo(kind kindmeta.Kind) bool {
	switch k {
	case SortProgress, SortPace:
		return kind == kindmeta.KindKeyResult
	case SortLocator:
		return true
	default:
		return kindmeta.Has(kind, kindmeta.Field(k))
	}
}

// kindOrder is the order kinds are grouped in when a sort key is not universal.
// It is kindmeta's own declared order (§1.3's reading order: project, area,
// resource, then the two nested kinds, then skill), so the grouping is stable
// and is not a second opinion about what order kinds come in.
var kindOrder = []kindmeta.Kind{
	kindmeta.KindProject,
	kindmeta.KindArea,
	kindmeta.KindResource,
	kindmeta.KindObjective,
	kindmeta.KindKeyResult,
	kindmeta.KindLink,
	kindmeta.KindSkill,
}

// KindsWith lists the kinds that have the key, in kindOrder — what the
// no-matched-kind-has-it error names (§17).
func KindsWith(k SortKey) []kindmeta.Kind {
	var out []kindmeta.Kind
	for _, kind := range kindOrder {
		if k.AppliesTo(kind) {
			out = append(out, kind)
		}
	}
	return out
}

// sortEntities orders ents by key, ascending unless reverse (§17: "ascending;
// `--reverse` flips. Direction is never folded into a key").
//
// A universal key sorts the whole list. Anything else follows ADR 0002 as §17
// amends it: error only when *no* matched kind has the field, otherwise group
// by kind, sort each group independently, and let a group whose kind lacks the
// field fall back to locator order after the groups that have it. §17's reason
// is that `list` is cross-kind by default, so a hard error would make `--sort
// progress` almost unusable — it spans projects and key-results, and only one
// of those has it.
//
// `--reverse` flips the comparison *within* a group and leaves the grouping
// alone. Reversing the whole list instead would put the fallback group first,
// which is the one thing §17 says about where it goes.
func sortEntities(ents []view.Entity, key SortKey, reverse bool) ([]view.Entity, error) {
	if key == "" {
		key = SortLocator
	}
	if key.Universal() {
		sortGroup(ents, key, reverse)
		return ents, nil
	}

	var withKey, without []view.Entity
	for _, e := range ents {
		if key.AppliesTo(e.Kind) {
			withKey = append(withKey, e)
			continue
		}
		without = append(without, e)
	}
	if len(withKey) == 0 && len(ents) > 0 {
		return nil, noKindHasIt(key)
	}

	// Within `withKey`, group by kind so each kind sorts independently: two
	// kinds' `status` vocabularies are different sets (§1.7), so comparing
	// across them would be ranking words rather than states.
	byKind := map[kindmeta.Kind][]view.Entity{}
	for _, e := range withKey {
		byKind[e.Kind] = append(byKind[e.Kind], e)
	}
	out := make([]view.Entity, 0, len(ents))
	// kindOrder, never the map: no map's iteration order may reach output
	// (§0.2), and this one decides the order of the printed rows.
	for _, kind := range kindOrder {
		group := byKind[kind]
		if len(group) == 0 {
			continue
		}
		sortGroup(group, key, reverse)
		out = append(out, group...)
	}
	// The fallback group, last and in locator order (§17).
	sortGroup(without, SortLocator, false)
	out = append(out, without...)

	// A kind missing from kindOrder would silently lose its whole group, and a
	// dropped row is the one failure mode a sort must not have.
	if len(out) != len(ents) {
		return nil, paraerr.Newf(paraerr.KindInternal,
			"sorting by %s lost %d of %d rows — a kind is missing from the grouping order",
			key, len(ents)-len(out), len(ents))
	}
	return out, nil
}

// noKindHasIt is §17's hard error: the key, and the kinds it does apply to.
func noKindHasIt(key SortKey) error {
	applies := KindsWith(key)
	names := make([]string, len(applies))
	for i, k := range applies {
		names[i] = k.String() + "s"
	}
	return paraerr.Newf(paraerr.KindValidation,
		"nothing matched has a %s — it applies to %s", key, strings.Join(names, ", "))
}

// sortGroup sorts one kind's entities by key.
//
// Absence is settled before direction, and never flipped: §17 fixes that an
// undefined `pace` "sorts last", and last means last in either direction —
// `--sort pace --reverse` asks for the worst pace first, not for a column of
// dashes. What `--reverse` flips is the order of the values that exist.
//
// The sort is stable and tie-breaks on locator, so a key every row shares
// still produces the §8.5 walk's own order rather than an arbitrary one.
func sortGroup(ents []view.Entity, key SortKey, reverse bool) {
	slices.SortStableFunc(ents, func(a, b view.Entity) int {
		va, hasA := sortValue(key, a)
		vb, hasB := sortValue(key, b)
		switch {
		case !hasA && !hasB:
			return strings.Compare(a.Locator.String(), b.Locator.String())
		case !hasA:
			return 1
		case !hasB:
			return -1
		}
		c := va.compare(vb)
		if reverse {
			c = -c
		}
		if c != 0 {
			return c
		}
		return strings.Compare(a.Locator.String(), b.Locator.String())
	})
}

// value is one entity's value for one sort key, reduced to the two orderings
// any key needs: text, or a number. Every ranked key — `priority` by §1.7's
// order, `status` by its kind's vocabulary, a timestamp by its instant — is a
// number, so there is exactly one comparison for each and no key-specific
// comparator to keep in step.
type value struct {
	text   string
	num    float64
	isText bool
}

func (v value) compare(o value) int {
	if v.isText {
		return strings.Compare(v.text, o.text)
	}
	switch {
	case v.num < o.num:
		return -1
	case v.num > o.num:
		return 1
	default:
		return 0
	}
}

func text(s string) value     { return value{text: s, isText: true} }
func num(f float64) value     { return value{num: f} }
func rank(i int) value        { return value{num: float64(i)} }
func stamp(t time.Time) value { return value{num: float64(t.Unix())} }

// sortValue is an entity's value for key, and whether it has one at all. A
// value that is absent is treated exactly like a field the kind does not have
// (§17), so the two cases never need telling apart downstream.
func sortValue(key SortKey, e view.Entity) (value, bool) {
	switch key {
	case SortLocator:
		return text(e.Locator.String()), true
	case SortName:
		return text(strings.ToLower(e.Name())), true
	case SortCreated:
		return stamp(e.Created), !e.Created.IsZero()
	case SortAttention:
		return stamp(e.Attention), !e.Attention.IsZero()
	case SortDue:
		// By the instant the stored `due` resolves to, not by its string: a
		// bare date and a full timestamp are both legal spellings (§15.1), and
		// comparing them as text would interleave them wrongly.
		return stamp(e.Deadline), e.HasDeadline
	case SortPriority:
		// §1.7's ranking rather than alphabetical, which is why `priority` is a
		// closed set at all: lexical order would rank high, low, medium.
		r, ok := kindmeta.PriorityRank(e.State.Priority)
		return rank(r), ok
	case SortStatus:
		r, ok := statusRank(e)
		return rank(r), ok
	case SortProgress:
		if e.KeyResult == nil {
			return value{}, false
		}
		return num(e.KeyResult.Outlook.Progress), e.KeyResult.Outlook.HasProgress
	case SortPace:
		if e.KeyResult == nil {
			return value{}, false
		}
		return num(e.KeyResult.Outlook.Pace), e.KeyResult.Outlook.HasPace
	default:
		return value{}, false
	}
}

// statusRank orders a status within its own kind's vocabulary, in the order
// §1.7 writes it — the progression, so `planned` sorts before `done` rather
// than after it, which alphabetical order would get backwards.
func statusRank(e view.Entity) (int, bool) {
	if e.EffectiveStatus == "" {
		return 0, false
	}
	if i := slices.Index(kindmeta.SettableStatuses(e.Kind), e.EffectiveStatus); i >= 0 {
		return i, true
	}
	// A key-result's derived statuses are not settable, so they are not in that
	// vocabulary at all (§4.3). krStatusOrder is their own progression.
	if i := slices.Index(krStatusOrder, e.EffectiveStatus); i >= 0 {
		return i, true
	}
	// A status doctor would report as invalid: ranking it would be inventing a
	// position for it, so it sorts with the absent.
	return 0, false
}

// krStatusOrder is §4.3's derived statuses, worst first, so `--sort status` on
// key-results surfaces what needs attention rather than what is finished. The
// two ends are the terminal ones (§1.7), and `missed` is deliberately at the
// front rather than among them.
var krStatusOrder = []string{"missed", "at-risk", "on-track", "achieved", "dropped"}
