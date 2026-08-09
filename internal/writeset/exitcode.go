package writeset

// CrashExitCode is what a para killed by PARA_CRASH_AFTER exits with.
//
// It is declared here, in the file no build tag guards, rather than beside the
// hook that uses it: the crash harness compares against it, and a constant that
// existed only under `para_testhooks` would make `go vet ./...` — which runs
// without tags — fail to compile a test it cannot run.
//
// 99 is outside the range §0.5 assigns meaning to (0, 1, 2), so the harness can
// tell a process that was killed from one that failed. Nothing in a production
// build can produce it.
const CrashExitCode = 99
