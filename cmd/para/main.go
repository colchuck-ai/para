// Command para manages a PARA-method tree of projects, areas, and resources.
package main

import (
	"fmt"
	"os"

	"github.com/colchuck-ai/para/internal/cli"
	"github.com/colchuck-ai/para/internal/clock"
)

func main() {
	os.Exit(run())
}

func run() int {
	clk, err := resolveClock()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return cli.Run(clk, os.Args[1:], os.Stdout, os.Stderr)
}

// resolveClock honors PARA_NOW/PARA_TZ only under a para_testhooks build
// (§0.2); production always falls back to the real wall clock.
func resolveClock() (clock.Clock, error) {
	clk, err := clock.FromEnv()
	if err != nil {
		return nil, err
	}
	if clk != nil {
		return clk, nil
	}
	return clock.System{}, nil
}
