package render

import (
	"time"

	"github.com/colchuck-ai/para/internal/csvfile"
)

// measurementsRenderer renders MEASUREMENTS.csv: a projection of the
// key-result's measurement events, oldest first, one row per reading (§4.4).
//
// It carries derived columns — `decimal` and `progress` — which §2.5 permits
// precisely because nothing reads them back: a spreadsheet or GitHub's CSV
// viewer will chart this file and will not compute anything.
type measurementsRenderer struct{}

// measurementsFile lives only at the key-result. Nothing aggregates
// measurements upward (§4.4).
const measurementsFile = "MEASUREMENTS.csv"

func (measurementsRenderer) Path(in In) (string, error) {
	dir, err := in.dir()
	if err != nil {
		return "", err
	}
	return join(dir, measurementsFile), nil
}

func (measurementsRenderer) Render(in In) ([]byte, error) {
	rs := readings(in)
	rows := make([]csvfile.Row, 0, len(rs))
	for _, r := range rs {
		rows = append(rows, csvfile.Row{
			// UTC, like every other timestamp in a generated file. §4.4 gives
			// this column a specific job — "a spreadsheet, a notebook, or
			// GitHub's CSV viewer will chart this file" — and a column of mixed
			// offsets does not sort or plot as one axis. The instant is exact
			// either way (§3.1).
			At:       r.Event.At.UTC().Format(time.RFC3339),
			Value:    r.Event.Value,
			Decimal:  r.Decimal,
			Progress: r.Progress,
			Note:     flatten(r.Event.Note),
		})
	}
	return csvfile.Encode(rows)
}
