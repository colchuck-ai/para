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
	// NoAttention is meaningful only on a note event (§3.6, §18.6): it records
	// that this note does not claim the entity was tended, so Attention skips
	// it. omitempty keeps every event ever written before this field existed
	// decoding as false — a note that predates the flag is a note that counted,
	// which is the only backward-compatible reading.
	NoAttention bool `json:"no_attention,omitempty"`
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
// the four kinds §3.1 defines and any line with no instant.
//
// The two required keys are `at` and `kind`, and they are required here rather
// than checked by each reader because §3.1 makes the first of them load
// bearing: "ordering comes from `at`, never from file position". A line with no
// `at` would sort as the year 1 and take its place at the head of every
// digest — a silently wrong answer where refusing is a reportable one. It is
// also what lets doctor name the file and the line (§10's `journal` finding),
// since the only thing that can point at a line is whatever refused it.
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
	if e.At.IsZero() {
		return Event{}, paraerr.New(paraerr.KindValidation, "journal event has no at")
	}
	return e, nil
}
