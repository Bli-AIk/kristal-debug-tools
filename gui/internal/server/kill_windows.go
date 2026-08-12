//go:build windows

package server

import (
	"os/exec"
	"strconv"
)

func setProcAttrs(cmd *exec.Cmd) {
	// No setup needed: taskkill /T kills the whole tree.
}

// killTree terminates the process and all its children (love and the game
// may spawn children).
func killTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}
