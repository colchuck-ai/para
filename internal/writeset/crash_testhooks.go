//go:build para_testhooks

package writeset

import (
	"fmt"
	"os"
	"strconv"
	"sync"
)

// crashEnv names the write to die after. It is honoured only under the
// para_testhooks build tag, like PARA_NOW (§0.2).
const crashEnv = "PARA_CRASH_AFTER"

var (
	crashOnce   sync.Once
	crashTarget int
	crashCount  int
)

// crashPoint ends the process abruptly once it has performed PARA_CRASH_AFTER
// filesystem operations, which is how Phase 14's crash matrix kills para
// between any two writes in a writeset.
//
// It fires *after* the operation completes, so PARA_CRASH_AFTER=n means "n
// operations landed and the n+1th never started". Sweeping n from 1 upward
// therefore walks every point a crash can happen at, and §0.2's claim — that
// every one of them leaves either a clean tree or exactly stale-projection — is
// checkable rather than merely argued.
//
// os.Exit rather than a panic or an error: a crash is not something the write
// path gets to handle, and an error return would exercise the cleanup paths that
// a real kill skips. The count is per process, which is what a command is.
func crashPoint(op Op) {
	crashOnce.Do(func() {
		if raw := os.Getenv(crashEnv); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				fmt.Fprintf(os.Stderr, "writeset: invalid %s=%q\n", crashEnv, raw)
				os.Exit(CrashExitCode)
			}
			crashTarget = n
		}
	})
	if crashTarget == 0 {
		return
	}
	crashCount++
	if crashCount >= crashTarget {
		fmt.Fprintf(os.Stderr, "writeset: killed after %d operations, last was %s %s\n", crashCount, op.Kind, op.Path)
		os.Exit(CrashExitCode)
	}
}
