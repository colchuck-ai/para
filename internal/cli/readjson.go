package cli

import (
	"time"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/view"
)

// entityJSON is one entity as `--json` reports it: the stored fields, and every
// value the human output computed in order to print (§23).
//
// Carrying the derived values is the settled reading of §23, not a convenience.
// §2.5 already makes derive-at-read-time the rule rather than the exception, so
// a payload of stored fields alone would push §4.2's arithmetic, §1.7's cascade,
// and §3.6's clock onto every caller — three reimplementations of rules that
// exist precisely so there is one.
//
// Keys are kebab-case, matching every other name para prints: §15 field names,
// §7 config keys, locator segments.
//
// Timestamps are always UTC here, and `--local` does not reach them. §16.2.1
// makes terminal output negotiable "because nothing compares it and nothing
// commits it", and JSON is the one output shape that is neither — it is read by
// a program, which wants the instant, not the reader's wall clock. The day
// counts beside them are the part that was ever about a wall clock, and they
// are carried as numbers.
type entityJSON struct {
	Locator     string   `json:"locator"`
	Kind        string   `json:"kind"`
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Due         string   `json:"due,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Created     string   `json:"created,omitempty"`
	Type        string   `json:"type,omitempty"`
	Start       string   `json:"start,omitempty"`
	Target      string   `json:"target,omitempty"`
	Scope       []string `json:"scope,omitempty"`

	// Everything below is derived and therefore never in a truth file (§2.5).
	EffectiveStatus string `json:"effective-status,omitempty"`
	Terminal        bool   `json:"terminal"`
	Archived        bool   `json:"archived"`
	Dormant         bool   `json:"dormant"`
	DormantUnder    string `json:"dormant-under,omitempty"`
	Attention       string `json:"attention,omitempty"`
	AttentionDays   int    `json:"attention-days"`
	Overdue         bool   `json:"overdue"`
	DueDays         *int   `json:"due-days,omitempty"`

	KeyResult *keyResultJSON `json:"key-result,omitempty"`
}

// keyResultJSON is §4's arithmetic. Each derived number is a pointer so that
// "undefined" and "zero" stay different answers — an undefined pace is §4.2's
// named case and a pace of zero is a key-result that has not moved.
type keyResultJSON struct {
	Current  string   `json:"current,omitempty"`
	Start    string   `json:"start,omitempty"`
	Target   string   `json:"target,omitempty"`
	Progress *float64 `json:"progress"`
	Pace     *float64 `json:"pace"`
	Status   string   `json:"status,omitempty"`
}

func newEntityJSON(env *view.Env, e view.Entity) entityJSON {
	out := entityJSON{
		Locator:         entityLocatorString(e.Locator),
		Kind:            e.Kind.String(),
		ID:              e.ID(),
		Name:            e.State.Name,
		Description:     e.State.Description,
		Status:          e.State.Status,
		Priority:        e.State.Priority,
		Due:             e.State.Due,
		Tags:            e.State.Tags,
		Created:         utcOrEmpty(e.Created),
		Type:            e.State.Type,
		Start:           e.State.Start,
		Target:          e.State.Target,
		Scope:           e.State.Scope,
		EffectiveStatus: e.EffectiveStatus,
		Terminal:        e.Terminal,
		Archived:        e.Archived,
		Dormant:         e.Dormant(),
		Attention:       utcOrEmpty(e.Attention),
		AttentionDays:   env.DaysSince(e.Attention),
		Overdue:         e.Overdue(),
	}
	if len(e.DormantUnder) > 0 {
		out.DormantUnder = e.DormantUnder.String()
	}
	if e.HasDeadline {
		days := env.DaysUntil(e.Deadline)
		out.DueDays = &days
	}
	if kr := e.KeyResult; kr != nil {
		out.KeyResult = &keyResultJSON{
			Current:  raw(kr.HasCurrent, kr.Current.Raw),
			Start:    raw(kr.HasStart, kr.Start.Raw),
			Target:   raw(kr.HasTarget, kr.Target.Raw),
			Progress: optional(kr.Outlook.HasProgress, kr.Outlook.Progress),
			Pace:     optional(kr.Outlook.HasPace, kr.Outlook.Pace),
			Status:   string(kr.Outlook.Status),
		}
	}
	return out
}

// entityLocatorString is loc's dotted address (R24, R26): the form changes,
// the JSON key does not, and no separate `noun` key is added since `kind`
// already carries it and the two agree by construction. Every view.Entity a
// read command builds one of these from is a real entity the walk found, so
// address.String failing here is not a case this package exercises; the raw
// Locator string is a defensive fallback only.
func entityLocatorString(loc locator.Locator) string {
	s, err := address.String(loc)
	if err != nil {
		return loc.String()
	}
	return s
}

func utcOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func raw(has bool, value string) string {
	if !has {
		return ""
	}
	return value
}

func optional(has bool, value float64) *float64 {
	if !has {
		return nil
	}
	return &value
}

// showOutput is `show --json`: the entity, plus the three things `show` adds to
// a bare row — its children, the skills that reach it, and the staleness
// verdict with the level that supplied its threshold (§16.1).
type showOutput struct {
	entityJSON
	Stale    *staleJSON   `json:"stale"`
	Children []entityJSON `json:"children"`
	Skills   []skillJSON  `json:"skills"`
}

type staleJSON struct {
	Stale bool    `json:"stale"`
	Key   string  `json:"key"`
	Value float64 `json:"value"`
	// From is the config.toml that supplied the threshold, root-relative, or
	// empty when the built-in default did (§7, §22).
	From string `json:"from"`
}

type skillJSON struct {
	Locator string `json:"locator"`
	Name    string `json:"name,omitempty"`
	// Via is the scope entry that reached this entity, empty when the skill is
	// unscoped and therefore covers the whole tree (§5.2).
	Via       string `json:"via,omitempty"`
	WholeTree bool   `json:"whole-tree"`
}

func showOutputOf(env *view.Env, s shown) showOutput {
	out := showOutput{
		entityJSON: newEntityJSON(env, s.ent),
		Children:   make([]entityJSON, 0, len(s.children)),
		Skills:     make([]skillJSON, 0, len(s.skills)),
	}
	if s.stale.Found {
		out.Stale = &staleJSON{Stale: s.isStale, Key: s.stale.Key, Value: s.stale.Value, From: s.stale.From}
	}
	for _, c := range s.children {
		out.Children = append(out.Children, newEntityJSON(env, c))
	}
	for _, k := range s.skills {
		out.Skills = append(out.Skills, skillJSON{
			Locator:   entityLocatorString(k.Locator),
			Name:      k.Name,
			Via:       k.Via,
			WholeTree: k.WholeTree,
		})
	}
	return out
}
