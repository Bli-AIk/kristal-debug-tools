// Command kristal-run is the native Windows equivalent of bin/kristal-run:
// the same launcher, no bash required. On Linux it is also a drop-in
// replacement, kept byte-compatible with the bash script (see
// internal/launcher and KRISTAL_DEBUG_TOOLS_DRY_RUN).
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/Bli-AIk/kristal-debug-tools/gui/internal/launcher"
)

func main() {
	args, err := launcher.ParseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, launcher.ErrHelp) {
			fmt.Println(launcher.Usage())
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		var unknown *launcher.UnknownOptionError
		if errors.As(err, &unknown) {
			fmt.Fprintln(os.Stderr, launcher.Usage())
		}
		os.Exit(64)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	res, err := launcher.Resolve(cwd, os.Getenv("KRISTAL_MOD_ROOT"), os.Getenv("KRISTAL_ROOT"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Mirror bin/kristal-run: the dry run is resolved but does not touch love.
	if os.Getenv("KRISTAL_DEBUG_TOOLS_DRY_RUN") == "1" {
		fmt.Print(launcher.DryRunString(res, args))
		os.Exit(0)
	}

	cmd, err := launcher.Command(res.EngineRoot, res.ModID, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// bash `exec love` replaces the shell, so love's exit code is the
	// script's exit code; propagating it is the Go equivalent.
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
