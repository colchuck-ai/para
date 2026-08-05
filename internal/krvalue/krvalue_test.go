package krvalue

import (
	"testing"
	"time"
)

func TestParse_Number(t *testing.T) {
	for _, raw := range []string{"42", "0.024", "480", "-3.5"} {
		v, err := Parse(TypeNumber, raw)
		if err != nil {
			t.Fatalf("Parse(number, %q): %v", raw, err)
		}
		if v.Raw != raw {
			t.Errorf("Parse(number, %q).Raw = %q, want %q", raw, v.Raw, raw)
		}
	}
	for _, raw := range []string{
		"", "abc", "1/2",
		"NaN", "Inf", "+Inf", "-Inf", "infinity",
		"1e10", "1E10", "0x1p10",
		"1_000", "1.", ".5", "1.2.3",
	} {
		if _, err := Parse(TypeNumber, raw); err == nil {
			t.Errorf("Parse(number, %q): want error, got nil", raw)
		}
	}
}

// TestParse_Ratio proves §4.1's central rule: a ratio's denominator
// survives round-trip exactly as written, as a string — never reduced to
// its decimal.
func TestParse_Ratio(t *testing.T) {
	cases := []struct {
		raw     string
		decimal float64
	}{
		{"880/11000", 0.08},
		{"1320/12400", 1320.0 / 12400.0},
		{"0/100", 0},
	}
	for _, c := range cases {
		v, err := Parse(TypeRatio, c.raw)
		if err != nil {
			t.Fatalf("Parse(ratio, %q): %v", c.raw, err)
		}
		if v.Raw != c.raw {
			t.Errorf("Parse(ratio, %q).Raw = %q, want %q (denominator must survive as written)", c.raw, v.Raw, c.raw)
		}
		if v.Decimal != c.decimal {
			t.Errorf("Parse(ratio, %q).Decimal = %v, want %v", c.raw, v.Decimal, c.decimal)
		}
	}

	invalid := []string{"", "880", "880/", "/11000", "880/0", "-1/2", "880/11000.5", "1.5/2"}
	for _, raw := range invalid {
		if _, err := Parse(TypeRatio, raw); err == nil {
			t.Errorf("Parse(ratio, %q): want error, got nil", raw)
		}
	}
}

func TestParse_Boolean(t *testing.T) {
	v, err := Parse(TypeBoolean, "true")
	if err != nil || v.Decimal != 1 {
		t.Errorf("Parse(boolean, true) = %+v, %v; want Decimal 1, nil", v, err)
	}
	v, err = Parse(TypeBoolean, "false")
	if err != nil || v.Decimal != 0 {
		t.Errorf("Parse(boolean, false) = %+v, %v; want Decimal 0, nil", v, err)
	}
	for _, raw := range []string{"", "True", "1", "0", "yes"} {
		if _, err := Parse(TypeBoolean, raw); err == nil {
			t.Errorf("Parse(boolean, %q): want error, got nil", raw)
		}
	}
}

func TestValidateBounds_Boolean(t *testing.T) {
	trueVal, _ := Parse(TypeBoolean, "true")
	falseVal, _ := Parse(TypeBoolean, "false")

	if err := ValidateBounds(TypeBoolean, nil, trueVal); err != nil {
		t.Errorf("boolean target=true, no start: want nil, got %v", err)
	}
	if err := ValidateBounds(TypeBoolean, nil, falseVal); err == nil {
		t.Error("boolean target=false: want error (achieved at birth), got nil")
	}
	if err := ValidateBounds(TypeBoolean, &falseVal, trueVal); err == nil {
		t.Error("boolean with explicit start: want error, got nil")
	}
}

func TestValidateBounds_NumberRatio(t *testing.T) {
	forty, _ := Parse(TypeNumber, "40")
	fifty, _ := Parse(TypeNumber, "50")

	if err := ValidateBounds(TypeNumber, nil, fifty); err != nil {
		t.Errorf("no explicit start: want nil (deferred to first measurement), got %v", err)
	}
	if err := ValidateBounds(TypeNumber, &forty, fifty); err != nil {
		t.Errorf("start != target: want nil, got %v", err)
	}
	if err := ValidateBounds(TypeNumber, &fifty, fifty); err == nil {
		t.Error("start == target: want error, got nil")
	}
}

func TestProgress(t *testing.T) {
	cases := []struct {
		name                   string
		start, target, current float64
		hasMeasurement         bool
		want                   float64
		wantErr                bool
	}{
		{"no measurement yet reads exactly 0", 0, 100, 999, false, 0, false},
		{"midpoint", 480, 2000, 1240, true, 0.5, false},
		{"regression below baseline reads negative", 480, 2000, 400, true, -80.0 / 1520.0, false},
		{"overshoot reads above 1", 0, 100, 150, true, 1.5, false},
		{"target equals start is an error", 50, 50, 60, true, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Progress(c.start, c.target, c.current, c.hasMeasurement)
			if c.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Progress: %v", err)
			}
			if got != c.want {
				t.Errorf("Progress(%v,%v,%v,%v) = %v, want %v", c.start, c.target, c.current, c.hasMeasurement, got, c.want)
			}
		})
	}
}

func TestElapsed(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 1, 11, 0, 0, 0, 0, time.UTC)  // 10 days
	today := time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC) // 5 days in

	e, ok := Elapsed(created, due, today, true)
	if !ok || e != 0.5 {
		t.Errorf("Elapsed = %v, %v; want 0.5, true", e, ok)
	}

	if _, ok := Elapsed(created, due, today, false); ok {
		t.Error("no due: want undefined, got defined")
	}
	if _, ok := Elapsed(created, created, today, true); ok {
		t.Error("due == created (zero window): want undefined, got defined")
	}
	if _, ok := Elapsed(created, created.Add(-time.Hour), today, true); ok {
		t.Error("due before created: want undefined, got defined")
	}
}

func TestPace(t *testing.T) {
	if _, ok := Pace(0.5, 0.5, true, TypeBoolean); ok {
		t.Error("boolean: want pace undefined, got defined")
	}
	if _, ok := Pace(0.5, 0, false, TypeNumber); ok {
		t.Error("elapsed undefined (no due): want pace undefined, got defined")
	}
	if _, ok := Pace(0.5, 0, true, TypeNumber); ok {
		t.Error("elapsed == 0: want pace undefined, got defined")
	}
	if _, ok := Pace(0.5, -0.1, true, TypeNumber); ok {
		t.Error("elapsed < 0: want pace undefined, got defined")
	}
	progress, want := 0.47, 0.68
	pace, ok := Pace(progress, want, true, TypeNumber)
	if !ok {
		t.Fatal("want pace defined")
	}
	if pace != progress/want {
		t.Errorf("Pace = %v, want %v", pace, progress/want)
	}
}

func TestDerivedStatus(t *testing.T) {
	const atRiskPace = 0.8

	cases := []struct {
		name        string
		dropped     bool
		progress    float64
		paceDefined bool
		pace        float64
		pastDue     bool
		want        Status
	}{
		{"dropped overrides everything", true, -5, true, 0, true, StatusDropped},
		{"achieved at exactly 1", false, 1, true, 0.5, false, StatusAchieved},
		{"achieved beats past-due", false, 1, false, 0, true, StatusAchieved},
		{"missed: past due, progress < 1", false, 0.9, false, 0, true, StatusMissed},
		{"at-risk: pace below threshold", false, 0.4, true, 0.5, false, StatusAtRisk},
		{"on-track: pace at or above threshold", false, 0.4, true, 0.8, false, StatusOnTrack},
		{"on-track: no due, no pace, progress < 1", false, 0.4, false, 0, false, StatusOnTrack},
		{"undefined pace never reads at-risk", false, 0.1, false, 0, false, StatusOnTrack},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DerivedStatus(c.dropped, c.progress, c.paceDefined, c.pace, atRiskPace, c.pastDue)
			if got != c.want {
				t.Errorf("DerivedStatus(...) = %v, want %v", got, c.want)
			}
		})
	}
}

// TestDerivedStatus_NonLatching proves §1.7's rule directly: hit the target
// and regress, and the same key-result reads on-track again — nothing about
// a prior "achieved" reading survives, because DerivedStatus is a pure
// function of the current inputs, never of history.
func TestDerivedStatus_NonLatching(t *testing.T) {
	achieved := DerivedStatus(false, 1.2, false, 0, 0.8, false)
	if achieved != StatusAchieved {
		t.Fatalf("first reading = %v, want achieved", achieved)
	}
	regressed := DerivedStatus(false, 0.9, false, 0, 0.8, false)
	if regressed != StatusOnTrack {
		t.Fatalf("after regression = %v, want on-track (no latch)", regressed)
	}
}
