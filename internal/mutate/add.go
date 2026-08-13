package mutate

import (
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
)

// Add creates the entity loc names (§18.1): its directory, its
// .para/{state.toml, config.toml, logs/}, its README.md and ACTIVITY.md, and —
// for a project or an objective — its eager child container, with the same
// shape.
//
// The new entity's journal starts empty. `created` is a field in the truth file
// rather than an event (§3.1), so there is nothing to log about coming into
// existence; the **parent** logs `child added`, because the parent is the thing
// that genuinely changed (§3.3).
func (e *Env) Add(loc locator.Locator, f Fields) (Result, error) {
	info, err := kindmeta.KindOf(loc)
	if err != nil {
		return Result{}, err
	}
	if info.Archived {
		// The archive is a place you move things to, not a place you create
		// them in (§1.6, §18.5). Creating there would also make `add` decide
		// whether a live counterpart exists and whether a stub is owed, which
		// is machinery `archive` owns.
		return Result{}, paraerr.Newf(paraerr.KindValidation,
			"%s is in the archive — create it live and `para archive` it", relocateAddr(loc))
	}

	exists, err := tree.Exists(e.Root, loc)
	if err != nil {
		return Result{}, err
	}
	if exists {
		return Result{}, paraerr.Newf(paraerr.KindConflict, "%s already exists", relocateAddr(loc))
	}
	parentExists, err := tree.ParentExists(e.Root, loc)
	if err != nil {
		return Result{}, err
	}
	if !parentExists {
		missing, err := e.shallowestMissing(loc)
		if err != nil {
			return Result{}, err
		}
		return Result{}, paraerr.Newf(paraerr.KindNotFound, "%s does not exist — create it first", missing)
	}

	state, err := e.newState(info.Kind, f)
	if err != nil {
		return Result{}, err
	}

	subj, err := e.subjectAt(loc, info.Kind)
	if err != nil {
		return Result{}, err
	}
	subj.state = state
	plans := []*plan{{subj: subj, writeState: true, config: []byte{}, creating: true}}

	childPlan, err := e.containerPlan(loc, info.Kind, state)
	if err != nil {
		return Result{}, err
	}
	if childPlan != nil {
		plans = append(plans, childPlan)
	}

	parent, err := e.parentPlan(loc, []journal.Event{
		journal.NewChild(e.Now.UTC(), journal.ChildOpAdded, loc[len(loc)-1], "", "", ""),
	})
	if err != nil {
		return Result{}, err
	}

	wrote, err := apply(e, plans, parents(parent))
	return e.syncSurface(Result{Locator: loc, Kind: info.Kind, Wrote: wrote}, err)
}

// newState builds the entity's state.toml from the fields given, refusing a
// field the kind does not have and a required field left out (§15).
func (e *Env) newState(kind kindmeta.Kind, f Fields) (truth.State, error) {
	// Only requiredness is judged here; whether the kind has a field at all is
	// resolve's to answer, and it answers it for every field it is given.
	var missing []string
	for _, field := range kindmeta.AllFields() {
		switch kindmeta.Requirement(kind, field) {
		case kindmeta.Required, kindmeta.RequiredFixed:
			if !f.Has(field) {
				missing = append(missing, "--"+string(field))
			}
		}
	}
	if len(missing) > 0 {
		return truth.State{}, paraerr.Newf(paraerr.KindValidation,
			"%s needs %s", kind, strings.Join(missing, " and "))
	}

	base := truth.State{
		Created: stamp(e.Now),
		Status:  kindmeta.DefaultStatus(kind),
	}
	return e.resolve(kind, base, f)
}

// eagerContainers is §18.1's whole table: which kinds are born with a child
// container, and the identity that container is given.
//
// One table rather than three switches on the same two rows, because the
// alternative has a default arm — and a default arm here means a typo'd id
// silently gets the other container's prose.
var eagerContainers = map[kindmeta.Kind]struct {
	id   string
	name string
	// description takes the parent's name, since a container has no identity
	// of its own to ask about — it is the parent's child list given a
	// directory (§8.2), and §15 requires name and description of every kind.
	// Asking for them would put two more required flags on every `add` for a
	// fact the tree already states.
	description func(parentName string) string
}{
	kindmeta.KindProject: {
		id:          "objectives",
		name:        "Objectives",
		description: func(parent string) string { return "What " + parent + " is trying to move." },
	},
	kindmeta.KindObjective: {
		id:          "key-results",
		name:        "Key results",
		description: func(parent string) string { return "How " + parent + " is measured." },
	},
}

// containerPlan builds the eager child container a kind is born with, or nil
// for a kind that holds no children of its own (§18.1).
//
// Containers are created eagerly, not on first child: a project always has an
// objectives/, empty or not, because an agent or a human looking for one should
// not have to know that absence means "none" rather than "elsewhere".
func (e *Env) containerPlan(parentLoc locator.Locator, kind kindmeta.Kind, parent truth.State) (*plan, error) {
	spec, ok := eagerContainers[kind]
	if !ok {
		return nil, nil
	}
	subj, err := e.subjectAt(append(slices.Clone(parentLoc), spec.id), kindmeta.KindContainer)
	if err != nil {
		return nil, err
	}
	subj.state = truth.State{
		Name:        spec.name,
		Description: spec.description(parent.Name),
		Created:     stamp(e.Now),
	}
	return &plan{subj: subj, writeState: true, config: []byte{}, creating: true}, nil
}

// shallowestMissing names the highest ancestor of loc that is not there, which
// is the one to create first.
//
// The immediate parent is usually the wrong thing to name: `add
// projects.missing.objectives.q1` fails at `projects.missing.objectives`, but
// that container does not exist because the *project* does not, and it is never
// created by hand anyway (§18.1). Naming the shallowest gap names the thing the
// user has to type next.
func (e *Env) shallowestMissing(loc locator.Locator) (string, error) {
	if loc.Bucket() == "skills" {
		// A skill's parent is .agents/skills/, which is not a locator at all
		// (§1.4) and is created by `init` rather than by anything here.
		return ".agents/skills/", nil
	}
	for i := 1; i < len(loc); i++ {
		exists, err := tree.Exists(e.Root, loc[:i])
		if err != nil {
			return "", err
		}
		if !exists {
			return relocateAddr(loc[:i]), nil
		}
	}
	return relocateAddr(loc[:len(loc)-1]), nil
}
