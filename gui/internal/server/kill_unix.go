//go:build !windows

package server

import (
	"os/exec"
	"syscall"
)

// setProcAttrs puts the child in its own process group so killTree can kill
// the whole tree (love and the game may spawn children).
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
