// Package journal implements the append-only event codec design §3.1
// requires: one JSON object per line, for the four event kinds and the
// note field every kind carries. This phase covers only the codec —
// appending, rotation (§3.4), and ordered multi-file reads (§3.1's "never
// by position") land in Phase 5 on top of it.
//
// Struct field order, not a sorted-keys post-pass, is what makes encoding
// deterministic (§0.4): encoding/json marshals a struct's fields in their
// declared order, so Event's field order is chosen to match §3.1's own
// worked examples exactly, with json:",omitempty" dropping whatever a
// given kind doesn't carry.
package journal

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// Kind is one of the four event kinds §3.1 defines.
type Kind string

const (
	KindChange      Kind = "change"
	KindMeasurement Kind = "measurement"
	KindNote        Kind = "note"
	KindChild       Kind = "child"
)

// ChildOp is a child event's operation (§3.1).
type ChildOp string

const (
	ChildOpAdded      ChildOp = "added"
	ChildOpRemoved    ChildOp = "removed"
	ChildOpMoved      ChildOp = "moved"
	ChildOpArchived   ChildOp = "archived"
	ChildOpUnarchived ChildOp = "unarchived"
)

// Event is one journal line. Which fields are populated depends on Kind
// (§3.1's table); Note is available on every kind.
type Event struct {
	At    time.Time `json:"at"`
	Kind  Kind      `json:"kind"`
	Field string    `json:"field,omitempty"`
	From  string    `json:"from,omitempty"`
	To    string    `json:"to,omitempty"`
	Value string    `json:"value,omitempty"`
	Op    ChildOp   `json:"op,omitempty"`
	Child string    `json:"child,omitempty"`
	Note  string    `json:"note,omitempty"`
}

// Encode renders e as one JSONL line, including its trailing newline, so a
// caller can write the result directly to an append-only journal file.
func Encode(e Event) ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, paraerr.Wrap(paraerr.KindInternal, err, "journal: encoding event")
	}
	return append(data, '\n'), nil
}

// Decode parses one journal line into an Event, rejecting anything outside
// the four kinds §3.1 defines.
func Decode(line []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(bytes.TrimSpace(line), &e); err != nil {
		return Event{}, paraerr.Wrap(paraerr.KindValidation, err, "invalid journal event")
	}
	switch e.Kind {
	case KindChange, KindMeasurement, KindNote, KindChild:
	default:
		return Event{}, paraerr.Newf(paraerr.KindValidation, "unknown journal event kind %q", e.Kind)
	}
	return e, nil
}
