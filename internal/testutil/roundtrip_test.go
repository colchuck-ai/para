package testutil

import (
	"errors"
	"testing"
)

var errBoom = errors.New("boom")

func identityDecode(b []byte) (string, error) { return string(b), nil }
func identityEncode(s string) ([]byte, error) { return []byte(s), nil }

func TestAssertRoundTripPassesOnFixedPoint(t *testing.T) {
	spy := &fakeT{}
	AssertRoundTrip(spy, []byte("hello\n"), identityDecode, identityEncode)
	if spy.failed {
		t.Error("AssertRoundTrip should pass when decode -> encode is a fixed point")
	}
}

func TestAssertRoundTripFailsWhenNotAFixedPoint(t *testing.T) {
	lossyEncode := func(s string) ([]byte, error) { return []byte("mangled"), nil }

	spy := &fakeT{}
	AssertRoundTrip(spy, []byte("hello\n"), identityDecode, lossyEncode)
	if !spy.failed {
		t.Error("AssertRoundTrip should fail when re-encoding does not reproduce the input")
	}
}

func TestAssertRoundTripFailsOnDecodeError(t *testing.T) {
	failingDecode := func(b []byte) (string, error) { return "", errBoom }

	spy := &fakeT{}
	AssertRoundTrip(spy, []byte("hello\n"), failingDecode, identityEncode)
	if !spy.failed {
		t.Error("AssertRoundTrip should fail when decode returns an error")
	}
}
