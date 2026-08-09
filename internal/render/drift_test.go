package render

import "testing"

// TestActivityDrift covers §10's dated drift report: the earliest day two
// renderings of ACTIVITY.md disagree about.
func TestActivityDrift(t *testing.T) {
	derived := "# Activity\n" +
		"\n## 2026-01-05\n- Added objective **q1-growth**.\n" +
		"\n## 2026-01-04\n- Note: waiting on the ingest team.\n" +
		"\n## 2026-01-03\n- Measured 880/11000 (8.0%) — 24% of target.\n"

	cases := []struct {
		name     string
		existing string
		wantDay  string
		wantOK   bool
	}{
		{
			name:     "identical files have no drift",
			existing: derived,
		},
		{
			// §26's own case: someone appended a line by hand. The day it
			// landed in is the day named, not the newest day in the file.
			name: "a line appended to an older day",
			existing: "# Activity\n" +
				"\n## 2026-01-05\n- Added objective **q1-growth**.\n" +
				"\n## 2026-01-04\n- Note: waiting on the ingest team.\n" +
				"\n## 2026-01-03\n- Measured 880/11000 (8.0%) — 24% of target.\n- hand-edited\n",
			wantDay: "2026-01-03",
			wantOK:  true,
		},
		{
			name: "a whole day missing from the file",
			existing: "# Activity\n" +
				"\n## 2026-01-05\n- Added objective **q1-growth**.\n" +
				"\n## 2026-01-03\n- Measured 880/11000 (8.0%) — 24% of target.\n",
			wantDay: "2026-01-04",
			wantOK:  true,
		},
		{
			name: "a day the journal does not account for",
			existing: "# Activity\n" +
				"\n## 2026-01-06\n- Invented.\n" +
				"\n## 2026-01-05\n- Added objective **q1-growth**.\n" +
				"\n## 2026-01-04\n- Note: waiting on the ingest team.\n" +
				"\n## 2026-01-03\n- Measured 880/11000 (8.0%) — 24% of target.\n",
			wantDay: "2026-01-06",
			wantOK:  true,
		},
		{
			name:     "two days differ and the earlier one is named",
			existing: "# Activity\n\n## 2026-01-05\n- Changed.\n\n## 2026-01-04\n- Also changed.\n",
			wantDay:  "2026-01-03",
			wantOK:   true,
		},
		{
			// The drift is real but no day owns it: the file is not one this
			// parser can account for, so the caller reports it undated.
			name:     "prose before the first day heading names no day",
			existing: "# Activity\n\nsomeone wrote a paragraph here.\n\n## 2026-01-05\n- Added objective **q1-growth**.\n",
			wantOK:   false,
		},
		{
			name:     "an empty file names no day",
			existing: "",
			wantDay:  "2026-01-03",
			wantOK:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			day, ok := ActivityDrift([]byte(c.existing), []byte(derived))
			if ok != c.wantOK {
				t.Fatalf("ActivityDrift ok = %v, want %v (day %q)", ok, c.wantOK, day)
			}
			if ok && day != c.wantDay {
				t.Fatalf("ActivityDrift day = %q, want %q", day, c.wantDay)
			}
		})
	}
}
