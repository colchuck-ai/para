package testutil

import "bytes"

// AssertRoundTrip asserts that decode(data) then encode is a fixed point:
// re-encoding what was decoded reproduces data byte for byte. Every
// generated file's codec must satisfy this (§0.2), since doctor's
// stale-projection check re-derives every file in memory and compares it
// byte for byte against what's on disk.
func AssertRoundTrip[T any](t TB, data []byte, decode func([]byte) (T, error), encode func(T) ([]byte, error)) {
	t.Helper()

	v, err := decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
		return
	}
	out, err := encode(v)
	if err != nil {
		t.Fatalf("encode: %v", err)
		return
	}
	if !bytes.Equal(data, out) {
		t.Errorf("round-trip mismatch:\n--- input ---\n%s\n--- re-encoded ---\n%s", data, out)
	}
}
