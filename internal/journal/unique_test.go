package journal

import (
	"testing"

	"github.com/colchuck-ai/para/internal/paraerr"
)

func TestCheckMeasurementUnique_Collision(t *testing.T) {
	at := mustParseAt(t, "2026-01-03T00:00:00-08:00")
	events := []Event{NewMeasurement(at, "880/11000", "")}

	err := CheckMeasurementUnique(events, at)
	if err == nil {
		t.Fatal("CheckMeasurementUnique: want error, got nil")
	}
	if paraerr.ExitCode(err) != 1 {
		t.Errorf("ExitCode(err) = %d, want 1 (a collision is a command error)", paraerr.ExitCode(err))
	}
	want := "a measurement already exists at 2026-01-03T00:00:00-08:00"
	if err.Error() != want {
		t.Errorf("err.Error() = %q, want %q", err.Error(), want)
	}
}

// TestCheckMeasurementUnique_DifferentPrecisionNoCollision pins §3.1's
// example directly: a bare date and a same-day timestamp with minute
// precision are different exact instants, so no collision.
func TestCheckMeasurementUnique_DifferentPrecisionNoCollision(t *testing.T) {
	midnight := mustParseAt(t, "2026-01-03T00:00:00-08:00")
	nineOhTwo := mustParseAt(t, "2026-01-03T09:02:00-08:00")
	events := []Event{NewMeasurement(midnight, "880/11000", "")}

	if err := CheckMeasurementUnique(events, nineOhTwo); err != nil {
		t.Errorf("CheckMeasurementUnique: want nil, got %v", err)
	}
}

func TestCheckMeasurementUnique_NotesAndChangesDoNotCollide(t *testing.T) {
	at := mustParseAt(t, "2026-01-03T00:00:00-08:00")
	events := []Event{
		NewNote(at, "a note"),
		NewChange(at, "status", "planned", "in-progress", ""),
	}

	if err := CheckMeasurementUnique(events, at); err != nil {
		t.Errorf("CheckMeasurementUnique: want nil (notes/changes don't collide), got %v", err)
	}
}

func TestCheckMeasurementUnique_NoExistingMeasurements(t *testing.T) {
	at := mustParseAt(t, "2026-01-03T00:00:00-08:00")
	if err := CheckMeasurementUnique(nil, at); err != nil {
		t.Errorf("CheckMeasurementUnique(nil, ...): want nil, got %v", err)
	}
}
