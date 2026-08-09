package version

import "testing"

func TestStringFallsBackToDevWithoutOverrideOrModuleVersion(t *testing.T) {
	// In a `go test` binary, debug.ReadBuildInfo's Main.Version is "(devel)",
	// not a real semver — String must not surface that literally.
	saved := ldflagsVersion
	ldflagsVersion = ""
	defer func() { ldflagsVersion = saved }()

	if got := String(); got == "" || got == "(devel)" {
		t.Errorf("String() = %q, want a non-empty fallback other than (devel)", got)
	}
}

func TestStringPrefersLdflagsOverride(t *testing.T) {
	saved := ldflagsVersion
	ldflagsVersion = "1.2.3"
	defer func() { ldflagsVersion = saved }()

	if got, want := String(), "1.2.3"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
