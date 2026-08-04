// Package testutil provides shared test infrastructure: golden-file
// comparison with an -update flag, and a generic round-trip property
// assertion for the codecs that need to be byte-stable fixed points (§0.2).
package testutil

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
)

var update = flag.Bool("update", false, "update golden files instead of comparing against them")

// TB is the subset of testing.T (and testing.B) that Golden and
// AssertRoundTrip need. *testing.T satisfies it directly.
type TB interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Golden compares got against the file at path, failing the test on
// mismatch. Run `go test -update` to write got as the new golden file
// instead of comparing.
func Golden(t TB, path string, got []byte) {
	t.Helper()

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating golden directory for %s: %v", path, err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("writing golden file %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %s: %v (run go test -update to create it)", path, err)
		return
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden mismatch for %s:\n--- want ---\n%s\n--- got ---\n%s", path, want, got)
	}
}
