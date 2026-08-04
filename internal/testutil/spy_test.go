package testutil

// fakeT is a minimal TB spy used to test Golden/AssertRoundTrip's failure
// paths without those failures bubbling up into the real test's pass/fail
// status the way a real *testing.T subtest would.
type fakeT struct {
	failed bool
}

func (f *fakeT) Helper() {}

func (f *fakeT) Errorf(format string, args ...any) {
	f.failed = true
}

func (f *fakeT) Fatalf(format string, args ...any) {
	f.failed = true
}
