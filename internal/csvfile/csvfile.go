// Package csvfile implements the MEASUREMENTS.csv writer (design §4.4): a
// fixed at,value,decimal,progress,note column projection of a key-result's
// measurement events, oldest first. It is one-directional — nothing ever
// reads this file back as truth (§2.2's merge posture is merge=ours,
// wholly generated), so unlike ptoml and mdfile there is no Decode: the
// byte-stability guarantee it needs is only that two Encode calls over the
// same rows produce identical bytes.
package csvfile

import (
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// Row is one measurement reading, in §4.4's column order.
type Row struct {
	At       string
	Value    string
	Decimal  float64
	Progress float64
	Note     string
}

var header = []string{"at", "value", "decimal", "progress", "note"}

// Encode renders rows in the given order (oldest first, per §4.4) as CSV,
// with decimal and progress at four decimal places, matching §4.4's own
// worked example.
func Encode(rows []Row) ([]byte, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)

	if err := w.Write(header); err != nil {
		return nil, paraerr.Wrap(paraerr.KindInternal, err, "csvfile: writing header")
	}
	for _, r := range rows {
		record := []string{
			r.At,
			r.Value,
			strconv.FormatFloat(r.Decimal, 'f', 4, 64),
			strconv.FormatFloat(r.Progress, 'f', 4, 64),
			r.Note,
		}
		if err := w.Write(record); err != nil {
			return nil, paraerr.Wrap(paraerr.KindInternal, err, "csvfile: writing row")
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, paraerr.Wrap(paraerr.KindInternal, err, "csvfile: flushing")
	}
	return []byte(b.String()), nil
}
